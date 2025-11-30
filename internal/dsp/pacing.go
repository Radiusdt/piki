package dsp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/radiusdt/vector-dsp/internal/config"
	"github.com/radiusdt/vector-dsp/internal/metrics"
	"github.com/radiusdt/vector-dsp/internal/models"
	"github.com/redis/go-redis/v9"
)

// PacingEngine controls spend and frequency caps for a line item.
type PacingEngine interface {
	// Allow returns true if a bid is allowed for the given line item and user.
	Allow(lineItemID, userID string, cfg models.PacingConfig, price float64) bool
	
	// GetStats returns current pacing stats for a line item.
	GetStats(lineItemID string) (*PacingStats, error)
	
	// RecordWin records an auction win for pacing calculations.
	RecordWin(lineItemID string, price float64) error
}

// PacingStats holds current pacing statistics.
type PacingStats struct {
	DailySpend       float64   `json:"daily_spend"`
	HourlySpend      float64   `json:"hourly_spend"`
	TotalImpressions int64     `json:"total_impressions"`
	BudgetRemaining  float64   `json:"budget_remaining"`
	PacingMultiplier float64   `json:"pacing_multiplier"`
	SpendVelocity    float64   `json:"spend_velocity"`     // $/hour
	ProjectedSpend   float64   `json:"projected_spend"`    // End of day projection
	LastUpdated      time.Time `json:"last_updated"`
}

// RedisPacingEngine implements PacingEngine using Redis with advanced pacing.
type RedisPacingEngine struct {
	client     *redis.Client
	globalCfg  config.PacingGlobalConfig
	metrics    *metrics.Metrics
	
	// Local cache for reducing Redis calls
	mu         sync.RWMutex
	spendCache map[string]*spendCacheEntry
}

type spendCacheEntry struct {
	spend     float64
	expiresAt time.Time
}

// NewRedisPacingEngine creates a new Redis-backed pacing engine.
func NewRedisPacingEngine(client *redis.Client, globalCfg config.PacingGlobalConfig, m *metrics.Metrics) *RedisPacingEngine {
	return &RedisPacingEngine{
		client:     client,
		globalCfg:  globalCfg,
		metrics:    m,
		spendCache: make(map[string]*spendCacheEntry),
	}
}

// Allow checks if a bid is allowed based on budget and frequency caps.
func (p *RedisPacingEngine) Allow(lineItemID, userID string, cfg models.PacingConfig, price float64) bool {
	ctx := context.Background()
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	hour := now.Hour()

	// Check schedule (start/end dates)
	if !cfg.StartAt.IsZero() && now.Before(cfg.StartAt) {
		if p.metrics != nil {
			p.metrics.RecordPacingRejection(lineItemID, "not_started")
		}
		return false
	}
	if !cfg.EndAt.IsZero() && now.After(cfg.EndAt) {
		if p.metrics != nil {
			p.metrics.RecordPacingRejection(lineItemID, "ended")
		}
		return false
	}

	// Check daily budget with smoothing
	if cfg.DailyBudget > 0 {
		allowed, reason := p.checkBudget(ctx, lineItemID, today, hour, cfg, price)
		if !allowed {
			if p.metrics != nil {
				p.metrics.RecordPacingRejection(lineItemID, reason)
			}
			return false
		}
	}

	// Check hourly budget cap
	if cfg.HourlyBudgetCap > 0 {
		hourlySpend := p.getHourlySpend(ctx, lineItemID, today, hour)
		if hourlySpend+price > cfg.HourlyBudgetCap {
			if p.metrics != nil {
				p.metrics.RecordPacingRejection(lineItemID, "hourly_cap")
			}
			return false
		}
	}

	// Check frequency caps
	if userID != "" && userID != "anonymous" {
		// Daily frequency cap
		if cfg.FreqCapPerUserPerDay > 0 {
			if !p.checkFreqCap(ctx, lineItemID, userID, today, "day", cfg.FreqCapPerUserPerDay) {
				if p.metrics != nil {
					p.metrics.RecordFreqCapRejection(lineItemID)
				}
				return false
			}
		}

		// Hourly frequency cap
		if cfg.FreqCapPerUserPerHour > 0 {
			hourKey := fmt.Sprintf("%s:%02d", today, hour)
			if !p.checkFreqCap(ctx, lineItemID, userID, hourKey, "hour", cfg.FreqCapPerUserPerHour) {
				if p.metrics != nil {
					p.metrics.RecordFreqCapRejection(lineItemID)
				}
				return false
			}
		}

		// Lifetime frequency cap
		if cfg.FreqCapPerUserLifetime > 0 {
			if !p.checkFreqCap(ctx, lineItemID, userID, "lifetime", "lifetime", cfg.FreqCapPerUserLifetime) {
				if p.metrics != nil {
					p.metrics.RecordFreqCapRejection(lineItemID)
				}
				return false
			}
		}
	}

	// Increment counters (atomic operations)
	p.incrementCounters(ctx, lineItemID, userID, today, hour, price, cfg)

	return true
}

// checkBudget checks daily budget with optional smoothing.
func (p *RedisPacingEngine) checkBudget(ctx context.Context, lineItemID, today string, hour int, cfg models.PacingConfig, price float64) (bool, string) {
	budgetKey := fmt.Sprintf("pacing:budget:%s:%s", lineItemID, today)
	
	// Get current spend
	currentSpend, err := p.client.Get(ctx, budgetKey).Float64()
	if err != nil && err != redis.Nil {
		return true, "" // Fail open
	}

	// Check total daily budget
	if currentSpend+price > cfg.DailyBudget {
		return false, "daily_budget"
	}

	// Apply smoothing if enabled
	if p.globalCfg.SmoothingEnabled && cfg.PacingType != models.PacingTypeAccelerated {
		maxHourlyPct := p.globalCfg.HourlyBudgetPct / 100.0
		if cfg.HourlyBudgetCap > 0 {
			// Use explicit hourly cap if set
			maxHourlyPct = cfg.HourlyBudgetCap / cfg.DailyBudget
		}

		// Calculate ideal spend by this hour
		hoursElapsed := float64(hour) + float64(time.Now().Minute())/60.0
		
		var idealSpend float64
		switch cfg.PacingType {
		case models.PacingTypeFrontLoaded:
			// More aggressive early in the day
			idealSpend = cfg.DailyBudget * (1 - (1-hoursElapsed/24)*(1-hoursElapsed/24))
		default: // Even pacing
			idealSpend = cfg.DailyBudget * (hoursElapsed / 24.0)
		}

		// Allow some buffer (20% ahead of pace)
		maxSpend := idealSpend * 1.2
		if currentSpend+price > maxSpend {
			return false, "pacing_ahead"
		}

		// Check hourly spend limit
		hourlySpend := p.getHourlySpend(ctx, lineItemID, today, hour)
		maxHourlySpend := cfg.DailyBudget * maxHourlyPct
		if hourlySpend+price > maxHourlySpend {
			return false, "hourly_pacing"
		}
	}

	return true, ""
}

// getHourlySpend returns spend for a specific hour.
func (p *RedisPacingEngine) getHourlySpend(ctx context.Context, lineItemID, today string, hour int) float64 {
	key := fmt.Sprintf("pacing:hourly:%s:%s:%02d", lineItemID, today, hour)
	spend, err := p.client.Get(ctx, key).Float64()
	if err != nil {
		return 0
	}
	return spend
}

// checkFreqCap checks and increments frequency cap.
func (p *RedisPacingEngine) checkFreqCap(ctx context.Context, lineItemID, userID, period, capType string, limit int32) bool {
	key := fmt.Sprintf("pacing:freq:%s:%s:%s", lineItemID, userID, period)
	
	count, err := p.client.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		return true // Fail open
	}

	return count < int64(limit)
}

// incrementCounters increments all relevant pacing counters.
func (p *RedisPacingEngine) incrementCounters(ctx context.Context, lineItemID, userID, today string, hour int, price float64, cfg models.PacingConfig) {
	pipe := p.client.Pipeline()

	// Daily spend
	budgetKey := fmt.Sprintf("pacing:budget:%s:%s", lineItemID, today)
	pipe.IncrByFloat(ctx, budgetKey, price)
	pipe.Expire(ctx, budgetKey, 25*time.Hour)

	// Hourly spend
	hourlyKey := fmt.Sprintf("pacing:hourly:%s:%s:%02d", lineItemID, today, hour)
	pipe.IncrByFloat(ctx, hourlyKey, price)
	pipe.Expire(ctx, hourlyKey, 2*time.Hour)

	// Impression count
	impKey := fmt.Sprintf("pacing:imps:%s:%s", lineItemID, today)
	pipe.Incr(ctx, impKey)
	pipe.Expire(ctx, impKey, 25*time.Hour)

	// Frequency caps
	if userID != "" && userID != "anonymous" {
		if cfg.FreqCapPerUserPerDay > 0 {
			dayKey := fmt.Sprintf("pacing:freq:%s:%s:%s", lineItemID, userID, today)
			pipe.Incr(ctx, dayKey)
			pipe.Expire(ctx, dayKey, 25*time.Hour)
		}

		if cfg.FreqCapPerUserPerHour > 0 {
			hourFreqKey := fmt.Sprintf("pacing:freq:%s:%s:%s:%02d", lineItemID, userID, today, hour)
			pipe.Incr(ctx, hourFreqKey)
			pipe.Expire(ctx, hourFreqKey, 2*time.Hour)
		}

		if cfg.FreqCapPerUserLifetime > 0 {
			lifetimeKey := fmt.Sprintf("pacing:freq:%s:%s:lifetime", lineItemID, userID)
			pipe.Incr(ctx, lifetimeKey)
			// Keep lifetime caps for 30 days
			pipe.Expire(ctx, lifetimeKey, 30*24*time.Hour)
		}
	}

	pipe.Exec(ctx)
}

// GetStats returns current pacing statistics.
func (p *RedisPacingEngine) GetStats(lineItemID string) (*PacingStats, error) {
	ctx := context.Background()
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	hour := now.Hour()

	stats := &PacingStats{
		LastUpdated: now,
	}

	// Get daily spend
	budgetKey := fmt.Sprintf("pacing:budget:%s:%s", lineItemID, today)
	dailySpend, _ := p.client.Get(ctx, budgetKey).Float64()
	stats.DailySpend = dailySpend

	// Get hourly spend
	hourlyKey := fmt.Sprintf("pacing:hourly:%s:%s:%02d", lineItemID, today, hour)
	hourlySpend, _ := p.client.Get(ctx, hourlyKey).Float64()
	stats.HourlySpend = hourlySpend

	// Get impression count
	impKey := fmt.Sprintf("pacing:imps:%s:%s", lineItemID, today)
	imps, _ := p.client.Get(ctx, impKey).Int64()
	stats.TotalImpressions = imps

	// Calculate velocity (simple moving average over last 3 hours)
	var totalSpend float64
	hoursBack := 3
	if hour < hoursBack {
		hoursBack = hour
	}
	for h := hour - hoursBack; h <= hour; h++ {
		hKey := fmt.Sprintf("pacing:hourly:%s:%s:%02d", lineItemID, today, h)
		spend, _ := p.client.Get(ctx, hKey).Float64()
		totalSpend += spend
	}
	if hoursBack > 0 {
		stats.SpendVelocity = totalSpend / float64(hoursBack+1)
	}

	// Project end of day spend
	hoursRemaining := 24 - hour
	stats.ProjectedSpend = stats.DailySpend + (stats.SpendVelocity * float64(hoursRemaining))

	return stats, nil
}

// RecordWin records an auction win.
func (p *RedisPacingEngine) RecordWin(lineItemID string, price float64) error {
	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")

	pipe := p.client.Pipeline()

	// Win count
	winKey := fmt.Sprintf("stats:wins:%s:%s", lineItemID, today)
	pipe.Incr(ctx, winKey)
	pipe.Expire(ctx, winKey, 48*time.Hour)

	// Spend tracking
	spendKey := fmt.Sprintf("stats:spend:%s:%s", lineItemID, today)
	pipe.IncrByFloat(ctx, spendKey, price)
	pipe.Expire(ctx, spendKey, 48*time.Hour)

	_, err := pipe.Exec(ctx)
	return err
}

// InMemoryPacingEngine is a simple in-memory pacing implementation for testing.
type InMemoryPacingEngine struct {
	mu         sync.Mutex
	dailySpend map[string]map[string]float64          // lineItemID -> date -> spend
	dailyFreq  map[string]map[string]map[string]int32 // lineItemID -> date -> userID -> count
}

// NewInMemoryPacingEngine constructs a new pacing engine with empty counters.
func NewInMemoryPacingEngine() *InMemoryPacingEngine {
	return &InMemoryPacingEngine{
		dailySpend: make(map[string]map[string]float64),
		dailyFreq:  make(map[string]map[string]map[string]int32),
	}
}

// dateKey returns the current date string.
func dateKey() string {
	return time.Now().UTC().Format("2006-01-02")
}

// Allow checks and increments pacing counters.
func (p *InMemoryPacingEngine) Allow(lineItemID, userID string, cfg models.PacingConfig, price float64) bool {
	d := dateKey()
	p.mu.Lock()
	defer p.mu.Unlock()

	// Initialize maps
	if _, ok := p.dailySpend[lineItemID]; !ok {
		p.dailySpend[lineItemID] = make(map[string]float64)
	}
	if _, ok := p.dailyFreq[lineItemID]; !ok {
		p.dailyFreq[lineItemID] = make(map[string]map[string]int32)
	}
	if _, ok := p.dailyFreq[lineItemID][d]; !ok {
		p.dailyFreq[lineItemID][d] = make(map[string]int32)
	}

	// Check budget
	currentSpend := p.dailySpend[lineItemID][d]
	if cfg.DailyBudget > 0 && currentSpend+price > cfg.DailyBudget {
		return false
	}

	// Check frequency cap
	currentCount := p.dailyFreq[lineItemID][d][userID]
	if cfg.FreqCapPerUserPerDay > 0 && currentCount >= cfg.FreqCapPerUserPerDay {
		return false
	}

	// Increment counters
	p.dailySpend[lineItemID][d] = currentSpend + price
	p.dailyFreq[lineItemID][d][userID] = currentCount + 1
	return true
}

// GetStats returns current pacing stats.
func (p *InMemoryPacingEngine) GetStats(lineItemID string) (*PacingStats, error) {
	d := dateKey()
	p.mu.Lock()
	defer p.mu.Unlock()

	spend := 0.0
	if spends, ok := p.dailySpend[lineItemID]; ok {
		spend = spends[d]
	}

	return &PacingStats{
		DailySpend:  spend,
		LastUpdated: time.Now(),
	}, nil
}

// RecordWin records an auction win.
func (p *InMemoryPacingEngine) RecordWin(lineItemID string, price float64) error {
	return nil
}
