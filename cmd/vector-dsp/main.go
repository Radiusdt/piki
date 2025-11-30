package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/radiusdt/vector-dsp/internal/config"
	"github.com/radiusdt/vector-dsp/internal/database"
	"github.com/radiusdt/vector-dsp/internal/httpserver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger := setupLogger(cfg)
	defer logger.Sync()

	logger.Info("starting Vector-DSP",
		zap.String("env", cfg.Server.Env),
		zap.String("addr", cfg.Server.Addr),
	)

	// Initialize database connections
	var db *database.PostgresDB
	var redis *database.RedisDB

	// Try to connect to PostgreSQL
	db, err = database.NewPostgresDB(cfg.Database.DSN(), cfg.Database.MaxConns, cfg.Database.MinConns)
	if err != nil {
		logger.Warn("PostgreSQL not available, using in-memory storage", zap.Error(err))
		db = nil
	} else {
		defer db.Close()
		logger.Info("connected to PostgreSQL")
	}

	// Try to connect to Redis
	redis, err = database.NewRedisDB(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logger.Warn("Redis not available, pacing/caching disabled", zap.Error(err))
		redis = nil
	} else {
		defer redis.Close()
		logger.Info("connected to Redis")
	}

	// Create HTTP server
	deps := &httpserver.Dependencies{
		DB:     db,
		Redis:  redis,
		Config: cfg,
		Logger: logger,
	}

	handler := httpserver.NewServer(deps)

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("HTTP server listening", zap.String("addr", cfg.Server.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server stopped")
}

func setupLogger(cfg *config.Config) *zap.Logger {
	var zapCfg zap.Config

	if cfg.IsDevelopment() {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		zapCfg = zap.NewProductionConfig()
	}

	// Set log level
	switch cfg.Log.Level {
	case "debug":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	}

	logger, err := zapCfg.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}

	return logger
}
