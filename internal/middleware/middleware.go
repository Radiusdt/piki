package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/radiusdt/vector-dsp/internal/config"
	"github.com/radiusdt/vector-dsp/internal/metrics"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// contextKey is a custom type for context keys.
type contextKey string

const (
	APIKeyContextKey contextKey = "api_key"
	AuthHeaderName              = "X-API-Key"
	AuthQueryParam              = "api_key"
)

// NewLogger creates a new zap logger based on configuration.
func NewLogger(level, format string) (*zap.Logger, error) {
	var cfg zap.Config

	if format == "console" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	switch level {
	case "debug":
		cfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		cfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	return cfg.Build()
}

// RecoveryMiddleware recovers from panics.
type RecoveryMiddleware struct {
	logger *zap.Logger
}

func NewRecoveryMiddleware(logger *zap.Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{logger: logger}
}

func (rm *RecoveryMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				rm.logger.Error("panic recovered",
					zap.Any("error", err),
					zap.String("path", r.URL.Path),
					zap.String("method", r.Method),
					zap.String("stack", string(debug.Stack())),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs HTTP requests.
type LoggingMiddleware struct {
	logger *zap.Logger
}

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

func NewLoggingMiddleware(logger *zap.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{logger: logger}
}

func (l *LoggingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)

		fields := []zap.Field{
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", rw.status),
			zap.Int("size", rw.size),
			zap.Duration("duration", duration),
			zap.String("remote_addr", r.RemoteAddr),
		}

		switch {
		case rw.status >= 500:
			l.logger.Error("request completed", fields...)
		case rw.status >= 400:
			l.logger.Warn("request completed", fields...)
		case r.URL.Path == "/health" || r.URL.Path == "/metrics":
			l.logger.Debug("request completed", fields...)
		default:
			l.logger.Info("request completed", fields...)
		}
	})
}

// AuthMiddleware validates API key authentication.
type AuthMiddleware struct {
	cfg    config.AuthConfig
	logger *zap.Logger
}

func NewAuthMiddleware(cfg config.AuthConfig, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{cfg: cfg, logger: logger}
}

func (a *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		if a.shouldSkip(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		apiKey := r.Header.Get(AuthHeaderName)
		if apiKey == "" {
			apiKey = r.URL.Query().Get(AuthQueryParam)
		}

		if apiKey == "" {
			a.unauthorized(w, "missing API key")
			return
		}

		if !a.validateKey(apiKey) {
			a.logger.Warn("invalid API key attempt",
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
			)
			a.unauthorized(w, "invalid API key")
			return
		}

		ctx := context.WithValue(r.Context(), APIKeyContextKey, apiKey)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *AuthMiddleware) shouldSkip(path string) bool {
	for _, skip := range a.cfg.SkipPaths {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

func (a *AuthMiddleware) validateKey(key string) bool {
	return subtle.ConstantTimeCompare([]byte(key), []byte(a.cfg.MasterKey)) == 1
}

func (a *AuthMiddleware) unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "ApiKey")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + message + `"}`))
}

// RateLimitMiddleware implements rate limiting.
type RateLimitMiddleware struct {
	cfg         config.RateLimitConfig
	logger      *zap.Logger
	metrics     *metrics.Metrics
	bidLimiter  *rate.Limiter
	mgmtLimiter *rate.Limiter
	mu          sync.RWMutex
	ipLimiters  map[string]*rate.Limiter
}

func NewRateLimitMiddleware(cfg config.RateLimitConfig, logger *zap.Logger) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		cfg:         cfg,
		logger:      logger,
		bidLimiter:  rate.NewLimiter(rate.Limit(cfg.RPS), cfg.Burst),
		mgmtLimiter: rate.NewLimiter(rate.Limit(cfg.MgmtRPS), cfg.MgmtBurst),
		ipLimiters:  make(map[string]*rate.Limiter),
	}
}

func (rl *RateLimitMiddleware) SetMetrics(m *metrics.Metrics) {
	rl.metrics = m
}

func (rl *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		var limiter *rate.Limiter
		if rl.isBidEndpoint(r.URL.Path) {
			limiter = rl.bidLimiter
		} else {
			limiter = rl.mgmtLimiter
		}

		if !limiter.Allow() {
			rl.logger.Warn("rate limit exceeded",
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
			)
			if rl.metrics != nil {
				rl.metrics.RecordRateLimitHit(r.URL.Path, rl.getClientIP(r))
			}
			rl.tooManyRequests(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimitMiddleware) isBidEndpoint(path string) bool {
	return strings.HasPrefix(path, "/openrtb2/")
}

func (rl *RateLimitMiddleware) getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func (rl *RateLimitMiddleware) tooManyRequests(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"error":"rate limit exceeded"}`))
}

func (rl *RateLimitMiddleware) CleanupIPLimiters() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.ipLimiters = make(map[string]*rate.Limiter)
	rl.logger.Debug("cleaned up IP rate limiters")
}

// MetricsMiddleware adds metrics instrumentation.
type MetricsMiddleware struct {
	metrics *metrics.Metrics
}

func NewMetricsMiddleware(m *metrics.Metrics) *MetricsMiddleware {
	return &MetricsMiddleware{metrics: m}
}

func (m *MetricsMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Metrics are recorded in individual handlers for more detail
		next.ServeHTTP(w, r)
	})
}
