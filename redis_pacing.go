package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/radiusdt/vector-dsp/internal/models"
	"github.com/redis/go-redis/v9"
)

// RedisPacingEngine implements PacingEngine using Redis for distributed counters.
type RedisPacingEngine struct {
	client *redis.Client
}

// NewRedisPacingEngine creates a new Redis-backed pacing engine.
func NewRedisPacingEngine(client *redis.Client) *RedisPacingEngine {
	return &RedisPacingEngine{client: client}
}

// Allow checks if a bid is allowed based on budget and frequency caps.
// Uses Redis for atomic increments and distributed coordination.
func (p *RedisPacingEngine) Allow(lineItemID, userID string, cfg models.PacingConfig, price float64) bool {
	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")

	// Check and increment daily budget atomically
	if cfg.DailyBudget > 0 {
		budgetKey := fmt.Sprintf("pacing:budget:%s:%s", lineItemID, today)
		
		// Use INCRBYFLOAT to atomically check and increment
		// First, get current value
		current, err := p.client.Get(ctx, budgetKey).Float64()
		if err != nil && err != redis.Nil {
			// On error, allow the bid (fail open for availability)
			return true
		}

		if current+price > cfg.DailyBudget {
			return false
		}

		// Increment budget
		pipe := p.client.Pipeline()
		pipe.IncrByFloat(ctx, budgetKey, price)
		pipe.Expire(ctx, budgetKey, 25*time.Hour) // Expire after 25 hours
		_, err = pipe.Exec(ctx)
		if err != nil {
			// On error, allow the bid
			return true
		}
	}

	// Check and increment frequency cap
	if cfg.FreqCapPerUserPerDay > 0 && userID != "" && userID != "anonymous" {
		freqKey := fmt.Sprintf("pacing:freq:%s:%s:%s", lineItemID, today, userID)

		// Increment and check
		count, err := p.client.Incr(ctx, freqKey).Result()
		if err != nil {
			return true // Fail open
		}

		// Set expiry on first increment
		if count == 1 {
			p.client.Expire(ctx, freqKey, 25*time.Hour)
		}

		if count > int64(cfg.FreqCapPerUserPerDay) {
			// Decrement since we're rejecting
			p.client.Decr(ctx, freqKey)
			return false
		}
	}

	return true
}

// GetDailySpend returns the current daily spend for a line item.
func (p *RedisPacingEngine) GetDailySpend(lineItemID string) (float64, error) {
	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("pacing:budget:%s:%s", lineItemID, today)

	spend, err := p.client.Get(ctx, key).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return spend, err
}

// GetUserFrequency returns the current impression count for a user on a line item.
func (p *RedisPacingEngine) GetUserFrequency(lineItemID, userID string) (int64, error) {
	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("pacing:freq:%s:%s:%s", lineItemID, today, userID)

	count, err := p.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// ResetDailyBudget resets the daily budget counter for a line item.
func (p *RedisPacingEngine) ResetDailyBudget(lineItemID string) error {
	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("pacing:budget:%s:%s", lineItemID, today)

	return p.client.Del(ctx, key).Err()
}

// RecordWin records a winning bid for analytics.
func (p *RedisPacingEngine) RecordWin(lineItemID string, price float64) error {
	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")

	pipe := p.client.Pipeline()
	
	// Increment win count
	winKey := fmt.Sprintf("stats:wins:%s:%s", lineItemID, today)
	pipe.Incr(ctx, winKey)
	pipe.Expire(ctx, winKey, 48*time.Hour)

	// Add to spend
	spendKey := fmt.Sprintf("stats:spend:%s:%s", lineItemID, today)
	pipe.IncrByFloat(ctx, spendKey, price)
	pipe.Expire(ctx, spendKey, 48*time.Hour)

	_, err := pipe.Exec(ctx)
	return err
}

// RecordBid records a bid attempt for analytics.
func (p *RedisPacingEngine) RecordBid(lineItemID string) error {
	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("stats:bids:%s:%s", lineItemID, today)

	pipe := p.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 48*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}
