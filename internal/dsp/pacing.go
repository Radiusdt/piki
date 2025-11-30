package dsp

import (
    "sync"
    "time"

    "github.com/radiusdt/vector-dsp/internal/models"
)

// PacingEngine controls spend and frequency caps for a line item.  It
// determines whether a bid is allowed based on the configuration and
// historical counters.  After allowing a bid, the engine records the
// impression cost and the user exposure.
//
// Implementations should be thread-safe.  In production you would
// typically back this with Redis or another distributed counter store.
type PacingEngine interface {
    // Allow returns true if a bid is allowed for the given line item and
    // user.  The implementation should examine the provided pacing
    // configuration along with the current spend and frequency counts.
    // The price parameter indicates the cost of the impression in the
    // currency of the campaign (e.g. USD).  Implementations are
    // responsible for converting this into budget units (e.g. dollars).
    Allow(lineItemID, userID string, cfg models.PacingConfig, price float64) bool
}

// InMemoryPacingEngine is a simple thread-safe pacing implementation.
// It maintains per-day spend and per-day per-user impression counts.
// Counters reset at midnight UTC.  This implementation is suitable for
// single-instance testing and demonstration.  For production you
// should use a distributed counter store such as Redis or Memcached.
type InMemoryPacingEngine struct {
    mu        sync.Mutex
    dailySpend map[string]map[string]float64       // lineItemID -> date -> spend
    dailyFreq  map[string]map[string]map[string]int32 // lineItemID -> date -> userID -> count
}

// NewInMemoryPacingEngine constructs a new pacing engine with empty counters.
func NewInMemoryPacingEngine() *InMemoryPacingEngine {
    return &InMemoryPacingEngine{
        dailySpend: make(map[string]map[string]float64),
        dailyFreq:  make(map[string]map[string]map[string]int32),
    }
}

// dateKey returns a string representing the current date in UTC (YYYY-MM-DD).
func dateKey() string {
    return time.Now().UTC().Format("2006-01-02")
}

// Allow checks the current spend and frequency counters against the
// configuration.  If the daily budget or per-user frequency cap would be
// exceeded, it returns false.  Otherwise it increments the spend and
// frequency counters and returns true.
func (p *InMemoryPacingEngine) Allow(lineItemID, userID string, cfg models.PacingConfig, price float64) bool {
    // Determine keys
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
    // Check frequency cap per user
    currentCount := p.dailyFreq[lineItemID][d][userID]
    if cfg.FreqCapPerUserPerDay > 0 && currentCount >= cfg.FreqCapPerUserPerDay {
        return false
    }
    // Allowed: increment counters
    p.dailySpend[lineItemID][d] = currentSpend + price
    p.dailyFreq[lineItemID][d][userID] = currentCount + 1
    return true
}