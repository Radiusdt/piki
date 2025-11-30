package middleware

import (
	"net/http"
	"strings"
	"sync"

	"github.com/radiusdt/vector-dsp/internal/config"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimitMiddleware implements token bucket rate limiting.
type RateLimitMiddleware struct {
	cfg        config.RateLimitConfig
	logger     *zap.Logger
	bidLimiter *rate.Limiter
	mgmtLimiter *rate.Limiter
	
	// Per-IP limiters for more granular control
	mu         sync.RWMutex
	ipLimiters map[string]*rate.Limiter
}

// NewRateLimitMiddleware creates a new rate limiting middleware.
func NewRateLimitMiddleware(cfg config.RateLimitConfig, logger *zap.Logger) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		cfg:         cfg,
		logger:      logger,
		bidLimiter:  rate.NewLimiter(rate.Limit(cfg.RPS), cfg.Burst),
		mgmtLimiter: rate.NewLimiter(rate.Limit(cfg.MgmtRPS), cfg.MgmtBurst),
		ipLimiters:  make(map[string]*rate.Limiter),
	}
}

// Handler wraps an http.Handler with rate limiting.
func (rl *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if disabled
		if !rl.cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Choose limiter based on endpoint type
		var limiter *rate.Limiter
		if rl.isBidEndpoint(r.URL.Path) {
			limiter = rl.bidLimiter
		} else {
			limiter = rl.mgmtLimiter
		}

		// Check rate limit
		if !limiter.Allow() {
			rl.logger.Warn("rate limit exceeded",
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
			)
			rl.tooManyRequests(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// HandlerPerIP applies per-IP rate limiting (more aggressive).
func (rl *RateLimitMiddleware) HandlerPerIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		ip := rl.getClientIP(r)
		limiter := rl.getIPLimiter(ip)

		if !limiter.Allow() {
			rl.logger.Warn("per-IP rate limit exceeded",
				zap.String("ip", ip),
				zap.String("path", r.URL.Path),
			)
			rl.tooManyRequests(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// getIPLimiter returns or creates a rate limiter for the given IP.
func (rl *RateLimitMiddleware) getIPLimiter(ip string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.ipLimiters[ip]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = rl.ipLimiters[ip]; exists {
		return limiter
	}

	// Create new limiter for this IP (more restrictive than global)
	limiter = rate.NewLimiter(rate.Limit(rl.cfg.RPS/10), rl.cfg.Burst/10)
	rl.ipLimiters[ip] = limiter

	return limiter
}

// getClientIP extracts the client IP from the request.
func (rl *RateLimitMiddleware) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// isBidEndpoint returns true if the path is a bidding endpoint.
func (rl *RateLimitMiddleware) isBidEndpoint(path string) bool {
	return strings.HasPrefix(path, "/openrtb2/")
}

// tooManyRequests sends a 429 response.
func (rl *RateLimitMiddleware) tooManyRequests(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"error":"rate limit exceeded"}`))
}

// CleanupIPLimiters removes old IP limiters to prevent memory leaks.
// Should be called periodically (e.g., every hour).
func (rl *RateLimitMiddleware) CleanupIPLimiters() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	// Simple cleanup: just clear the map
	// A more sophisticated approach would track last access time
	rl.ipLimiters = make(map[string]*rate.Limiter)
	rl.logger.Debug("cleaned up IP rate limiters")
}
