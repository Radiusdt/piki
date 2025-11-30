package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/radiusdt/vector-dsp/internal/config"
	"github.com/radiusdt/vector-dsp/internal/database"
	"github.com/radiusdt/vector-dsp/internal/httpserver"
	"github.com/radiusdt/vector-dsp/internal/middleware"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		// Can't use logger yet, fall back to standard log
		panic("failed to load config: " + err.Error())
	}

	// Initialize logger
	logger, err := middleware.NewLogger(cfg.Log.Level, cfg.Log.Format)
	if err != nil {
		panic("failed to create logger: " + err.Error())
	}
	defer logger.Sync()

	logger.Info("starting Vector-DSP",
		zap.String("env", cfg.Server.Env),
		zap.String("addr", cfg.Server.Addr),
	)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize PostgreSQL
	db, err := database.NewPostgresDB(ctx, cfg.Database, logger)
	if err != nil {
		logger.Fatal("failed to connect to PostgreSQL", zap.Error(err))
	}
	defer db.Close()

	// Initialize Redis
	redis, err := database.NewRedisDB(ctx, cfg.Redis, logger)
	if err != nil {
		logger.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer redis.Close()

	// Build dependencies
	deps := &httpserver.Dependencies{
		DB:     db,
		Redis:  redis,
		Config: cfg,
		Logger: logger,
	}

	// Create HTTP server with all middlewares
	handler := httpserver.NewServer(deps)

	// Apply middleware chain (order matters: outermost first)
	// Recovery -> Logging -> RateLimit -> Auth -> Handler
	recoveryMW := middleware.NewRecoveryMiddleware(logger)
	loggingMW := middleware.NewLoggingMiddleware(logger)
	rateLimitMW := middleware.NewRateLimitMiddleware(cfg.RateLimit, logger)
	authMW := middleware.NewAuthMiddleware(cfg.Auth, logger)

	finalHandler := recoveryMW.Handler(
		loggingMW.Handler(
			rateLimitMW.Handler(
				authMW.Handler(handler),
			),
		),
	)

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           finalHandler,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// Start server in goroutine
	go func() {
		logger.Info("HTTP server starting", zap.String("addr", cfg.Server.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// Start rate limiter cleanup goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rateLimitMW.CleanupIPLimiters()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}

	// Cancel main context to stop background goroutines
	cancel()

	logger.Info("server stopped")
}
