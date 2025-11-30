package storage

import (
    "sync"
    "github.com/radiusdt/vector-dsp/internal/models"
)

// CreativeRepo defines CRUD operations for creatives.  In addition to
// storing creatives globally, a separate repository allows campaigns to
// reference shared creatives by ID.  In a real system creatives are
// often stored in a CDN or asset store and metadata kept in a
// database.
type CreativeRepo interface {
    GetCreative(id string) (*models.Creative, error)
    ListCreatives() ([]*models.Creative, error)
    UpsertCreative(c *models.Creative) error
    ListCreativesByAdvertiser(advertiserID string) ([]*models.Creative, error)
}

// InMemoryCreativeRepo stores creatives in memory keyed by their ID.
// An optional index by advertiser ID helps listing creatives per
// advertiser.  For production you should persist creatives in a
// database and upload assets to cloud storage.
type InMemoryCreativeRepo struct {
    mu         sync.RWMutex
    creatives  map[string]*models.Creative
}

// NewInMemoryCreativeRepo constructs an empty creative repository.
func NewInMemoryCreativeRepo() *InMemoryCreativeRepo {
    return &InMemoryCreativeRepo{
        creatives: make(map[string]*models.Creative),
    }
}

// GetCreative returns a creative by ID or nil if not found.
func (r *InMemoryCreativeRepo) GetCreative(id string) (*models.Creative, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    if c, ok := r.creatives[id]; ok {
        return c, nil
    }
    return nil, nil
}

// ListCreatives returns all creatives.
func (r *InMemoryCreativeRepo) ListCreatives() ([]*models.Creative, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    res := make([]*models.Creative, 0, len(r.creatives))
    for _, c := range r.creatives {
        res = append(res, c)
    }
    return res, nil
}

// UpsertCreative inserts or updates the given creative.  A shallow
// copy is stored to prevent external mutation.
func (r *InMemoryCreativeRepo) UpsertCreative(c *models.Creative) error {
    if c == nil {
        return nil
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    cp := *c
    r.creatives[c.ID] = &cp
    return nil
}

// ListCreativesByAdvertiser returns all creatives belonging to a given
// advertiser.  When AdvertiserID is empty the entire collection is
// returned.  For efficiency you should maintain an index per
// advertiser; this implementation simply filters the in-memory map.
func (r *InMemoryCreativeRepo) ListCreativesByAdvertiser(advertiserID string) ([]*models.Creative, error) {
    if advertiserID == "" {
        return r.ListCreatives()
    }
    r.mu.RLock()
    defer r.mu.RUnlock()
    var res []*models.Creative
    for _, c := range r.creatives {
        if c.AdvertiserID == advertiserID {
            res = append(res, c)
        }
    }
    return res, nil
}