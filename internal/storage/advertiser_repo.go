package storage

import (
    "sync"
    "github.com/radiusdt/vector-dsp/internal/models"
)

// AdvertiserRepo defines operations for storing and retrieving advertisers.
// Implementations may be in-memory, database-backed, etc.
type AdvertiserRepo interface {
    GetAdvertiser(id string) (*models.Advertiser, error)
    ListAdvertisers() ([]*models.Advertiser, error)
    UpsertAdvertiser(a *models.Advertiser) error
}

// InMemoryAdvertiserRepo is a simple thread-safe in-memory implementation
// of AdvertiserRepo.  It is intended for demonstration and testing; replace
// with a persistent store in production.
type InMemoryAdvertiserRepo struct {
    mu         sync.RWMutex
    advertisers map[string]*models.Advertiser
}

// NewInMemoryAdvertiserRepo creates an empty in-memory advertiser repo.
func NewInMemoryAdvertiserRepo() *InMemoryAdvertiserRepo {
    return &InMemoryAdvertiserRepo{
        advertisers: make(map[string]*models.Advertiser),
    }
}

// GetAdvertiser returns the advertiser with the given ID or nil if not found.
func (r *InMemoryAdvertiserRepo) GetAdvertiser(id string) (*models.Advertiser, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    if a, ok := r.advertisers[id]; ok {
        return a, nil
    }
    return nil, nil
}

// ListAdvertisers returns a slice of all advertisers.
func (r *InMemoryAdvertiserRepo) ListAdvertisers() ([]*models.Advertiser, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    res := make([]*models.Advertiser, 0, len(r.advertisers))
    for _, a := range r.advertisers {
        res = append(res, a)
    }
    return res, nil
}

// UpsertAdvertiser inserts or updates the given advertiser.
func (r *InMemoryAdvertiserRepo) UpsertAdvertiser(a *models.Advertiser) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    cp := *a
    r.advertisers[a.ID] = &cp
    return nil
}