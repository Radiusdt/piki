package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the Vector-DSP application.
type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Auth      AuthConfig
	RateLimit RateLimitConfig
	Log       LogConfig
	Metrics   MetricsConfig
	Geo       GeoConfig
	Pacing    PacingGlobalConfig
	Tracking  TrackingConfig
}

type ServerConfig struct {
	Addr            string
	Env             string
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	MaxConns int
	MinConns int
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.DBName, d.SSLMode,
	)
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AuthConfig struct {
	Enabled   bool
	MasterKey string
	SkipPaths []string
}

type RateLimitConfig struct {
	Enabled   bool
	RPS       float64
	Burst     int
	MgmtRPS   float64
	MgmtBurst int
}

type LogConfig struct {
	Level  string
	Format string
}

type MetricsConfig struct {
	Enabled bool
	Path    string
	Port    string
}

type GeoConfig struct {
	Enabled      bool
	DatabasePath string
	CacheSize    int
	CacheTTL     time.Duration
}

type PacingGlobalConfig struct {
	SmoothingEnabled bool
	HourlyBudgetPct  float64
	FreqCapLookback  time.Duration
}

// TrackingConfig holds tracking-related configuration
type TrackingConfig struct {
	// BaseURL is the public URL for tracking endpoints
	// Example: https://track.vector-dsp.com
	BaseURL string

	// ClickTTL is how long to keep clicks in cache for attribution
	ClickTTL time.Duration

	// MMPCallTimeout is the timeout for calling MMP view URLs
	MMPCallTimeout time.Duration

	// EnableViewTracking enables view/impression tracking
	EnableViewTracking bool

	// EnableClickDedup enables click deduplication
	EnableClickDedup bool
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Addr:            getEnv("VECTOR_DSP_HTTP_ADDR", ":8080"),
			Env:             getEnv("VECTOR_DSP_ENV", "development"),
			ShutdownTimeout: getDurationEnv("VECTOR_DSP_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			Host:     getEnv("VECTOR_DSP_DB_HOST", "localhost"),
			Port:     getIntEnv("VECTOR_DSP_DB_PORT", 5432),
			User:     getEnv("VECTOR_DSP_DB_USER", "vectordsp"),
			Password: getEnv("VECTOR_DSP_DB_PASSWORD", "vectordsp_secret"),
			DBName:   getEnv("VECTOR_DSP_DB_NAME", "vectordsp"),
			SSLMode:  getEnv("VECTOR_DSP_DB_SSLMODE", "disable"),
			MaxConns: getIntEnv("VECTOR_DSP_DB_MAX_CONNS", 25),
			MinConns: getIntEnv("VECTOR_DSP_DB_MIN_CONNS", 5),
		},
		Redis: RedisConfig{
			Addr:     getEnv("VECTOR_DSP_REDIS_ADDR", "localhost:6379"),
			Password: getEnv("VECTOR_DSP_REDIS_PASSWORD", ""),
			DB:       getIntEnv("VECTOR_DSP_REDIS_DB", 0),
		},
		Auth: AuthConfig{
			Enabled:   getBoolEnv("VECTOR_DSP_AUTH_ENABLED", true),
			MasterKey: getEnv("VECTOR_DSP_API_KEY_MASTER", ""),
			SkipPaths: getSliceEnv("VECTOR_DSP_AUTH_SKIP_PATHS", []string{
				"/health",
				"/metrics",
				"/openrtb2/bid",
				"/openrtb2/win",
				"/openrtb2/loss",
				"/track/",
				"/postback",
				"/s2s/",
			}),
		},
		RateLimit: RateLimitConfig{
			Enabled:   getBoolEnv("VECTOR_DSP_RATE_LIMIT_ENABLED", true),
			RPS:       getFloatEnv("VECTOR_DSP_RATE_LIMIT_RPS", 1000),
			Burst:     getIntEnv("VECTOR_DSP_RATE_LIMIT_BURST", 100),
			MgmtRPS:   getFloatEnv("VECTOR_DSP_RATE_LIMIT_MGMT_RPS", 100),
			MgmtBurst: getIntEnv("VECTOR_DSP_RATE_LIMIT_MGMT_BURST", 20),
		},
		Log: LogConfig{
			Level:  getEnv("VECTOR_DSP_LOG_LEVEL", "info"),
			Format: getEnv("VECTOR_DSP_LOG_FORMAT", "json"),
		},
		Metrics: MetricsConfig{
			Enabled: getBoolEnv("VECTOR_DSP_METRICS_ENABLED", true),
			Path:    getEnv("VECTOR_DSP_METRICS_PATH", "/metrics"),
			Port:    getEnv("VECTOR_DSP_METRICS_PORT", "9090"),
		},
		Geo: GeoConfig{
			Enabled:      getBoolEnv("VECTOR_DSP_GEO_ENABLED", false),
			DatabasePath: getEnv("VECTOR_DSP_GEO_DB_PATH", "/app/data/GeoLite2-City.mmdb"),
			CacheSize:    getIntEnv("VECTOR_DSP_GEO_CACHE_SIZE", 10000),
			CacheTTL:     getDurationEnv("VECTOR_DSP_GEO_CACHE_TTL", 1*time.Hour),
		},
		Pacing: PacingGlobalConfig{
			SmoothingEnabled: getBoolEnv("VECTOR_DSP_PACING_SMOOTHING", true),
			HourlyBudgetPct:  getFloatEnv("VECTOR_DSP_PACING_HOURLY_PCT", 8.0),
			FreqCapLookback:  getDurationEnv("VECTOR_DSP_PACING_FREQ_LOOKBACK", 24*time.Hour),
		},
		Tracking: TrackingConfig{
			BaseURL:            getEnv("VECTOR_DSP_TRACKING_BASE_URL", "https://track.vector-dsp.com"),
			ClickTTL:           getDurationEnv("VECTOR_DSP_TRACKING_CLICK_TTL", 30*24*time.Hour),
			MMPCallTimeout:     getDurationEnv("VECTOR_DSP_TRACKING_MMP_TIMEOUT", 5*time.Second),
			EnableViewTracking: getBoolEnv("VECTOR_DSP_TRACKING_VIEW_ENABLED", true),
			EnableClickDedup:   getBoolEnv("VECTOR_DSP_TRACKING_CLICK_DEDUP", true),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks that required configuration is present.
func (c *Config) Validate() error {
	if c.Auth.Enabled && c.Auth.MasterKey == "" {
		return fmt.Errorf("VECTOR_DSP_API_KEY_MASTER is required when auth is enabled")
	}
	return nil
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "development"
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}

// Helper functions

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getIntEnv(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getFloatEnv(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getBoolEnv(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getDurationEnv(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getSliceEnv(key string, def []string) []string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	}
	return def
}
