package database

import (
	"context"
	"fmt"

	"github.com/radiusdt/vector-dsp/internal/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisDB wraps a Redis client with convenience methods.
type RedisDB struct {
	Client *redis.Client
	logger *zap.Logger
}

// NewRedisDB creates a new Redis client connection.
func NewRedisDB(ctx context.Context, cfg config.RedisConfig, logger *zap.Logger) (*RedisDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: 100,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("connected to Redis",
		zap.String("addr", cfg.Addr),
		zap.Int("db", cfg.DB),
	)

	return &RedisDB{
		Client: client,
		logger: logger,
	}, nil
}

// Close closes the Redis connection.
func (r *RedisDB) Close() error {
	if r.Client != nil {
		r.logger.Info("Redis connection closed")
		return r.Client.Close()
	}
	return nil
}

// Health checks if Redis is reachable.
func (r *RedisDB) Health(ctx context.Context) error {
	return r.Client.Ping(ctx).Err()
}
