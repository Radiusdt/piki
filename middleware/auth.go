package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/radiusdt/vector-dsp/internal/config"
	"go.uber.org/zap"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// APIKeyContextKey is the context key for storing the authenticated API key.
	APIKeyContextKey contextKey = "api_key"
	
	// AuthHeaderName is the HTTP header name for the API key.
	AuthHeaderName = "X-API-Key"
	
	// AuthQueryParam is the query parameter name for the API key (fallback).
	AuthQueryParam = "api_key"
)

// AuthMiddleware validates API key authentication.
type AuthMiddleware struct {
	cfg    config.AuthConfig
	logger *zap.Logger
}

// NewAuthMiddleware creates a new authentication middleware.
func NewAuthMiddleware(cfg config.AuthConfig, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		cfg:    cfg,
		logger: logger,
	}
}

// Handler wraps an http.Handler with authentication.
func (a *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth if disabled
		if !a.cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for whitelisted paths
		if a.shouldSkip(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Extract API key from header or query param
		apiKey := r.Header.Get(AuthHeaderName)
		if apiKey == "" {
			apiKey = r.URL.Query().Get(AuthQueryParam)
		}

		// Validate API key
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

		// Store API key in context for downstream handlers
		ctx := context.WithValue(r.Context(), APIKeyContextKey, apiKey)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// shouldSkip checks if the path should bypass authentication.
func (a *AuthMiddleware) shouldSkip(path string) bool {
	for _, skip := range a.cfg.SkipPaths {
		if strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

// validateKey checks if the provided key is valid.
// Uses constant-time comparison to prevent timing attacks.
func (a *AuthMiddleware) validateKey(key string) bool {
	// Check against master key
	if subtle.ConstantTimeCompare([]byte(key), []byte(a.cfg.MasterKey)) == 1 {
		return true
	}
	
	// TODO: Add support for per-advertiser API keys from database
	// This would involve a database lookup here
	
	return false
}

// unauthorized sends a 401 response.
func (a *AuthMiddleware) unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "ApiKey")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + message + `"}`))
}

// GetAPIKey retrieves the API key from the request context.
func GetAPIKey(ctx context.Context) string {
	if key, ok := ctx.Value(APIKeyContextKey).(string); ok {
		return key
	}
	return ""
}
