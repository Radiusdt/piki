package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/radiusdt/vector-dsp/internal/config"
	"go.uber.org/zap"
)

// PostgresDB wraps a pgx connection pool with convenience methods.
type PostgresDB struct {
	Pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewPostgresDB creates a new PostgreSQL connection pool.
func NewPostgresDB(ctx context.Context, cfg config.DatabaseConfig, logger *zap.Logger) (*PostgresDB, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxConns)
	poolConfig.MinConns = int32(cfg.MinConns)
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("connected to PostgreSQL",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.DBName),
		zap.Int("max_conns", cfg.MaxConns),
	)

	return &PostgresDB{
		Pool:   pool,
		logger: logger,
	}, nil
}

// Close closes the database connection pool.
func (db *PostgresDB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		db.logger.Info("PostgreSQL connection pool closed")
	}
}

// Health checks if the database is reachable.
func (db *PostgresDB) Health(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}

// Stats returns connection pool statistics.
func (db *PostgresDB) Stats() *pgxpool.Stat {
	return db.Pool.Stat()
}
