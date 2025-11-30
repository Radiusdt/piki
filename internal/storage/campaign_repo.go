package storage

import (
    "sync"

    "github.com/radiusdt/vector-dsp/internal/models"
)

// CampaignRepo defines the minimal CRUD operations for campaigns.  A
// campaign groups one or more line items which contain creatives and
// targeting.  Implementations should provide thread safety.  In
// production you would replace this with a database-backed store.
type CampaignRepo interface {
    // GetCampaign returns a single campaign by ID or nil if not found.
    GetCampaign(id string) (*models.Campaign, error)
    // ListCampaigns returns all campaigns.  It does not filter by
    // advertiser or status; such filtering should be performed by the
    // caller or via additional methods on the repository.
    ListCampaigns() ([]*models.Campaign, error)
    // UpsertCampaign inserts or updates the given campaign.  The
    // implementation should overwrite any existing campaign with the
    // same ID.  No validation is performed at this layer.
    UpsertCampaign(c *models.Campaign) error
}

// InMemoryCampaignRepo is a simple in-memory implementation of
// CampaignRepo.  It stores campaigns in a map keyed by campaign ID.  It
// is intended for demonstration and testing only.  For production
// deployments you should persist campaigns in a database or other
// durable store and add indexes for efficient filtering.
type InMemoryCampaignRepo struct {
    mu        sync.RWMutex
    campaigns map[string]*models.Campaign
}

// NewInMemoryCampaignRepo creates a new empty in-memory campaign repo.
func NewInMemoryCampaignRepo() *InMemoryCampaignRepo {
    return &InMemoryCampaignRepo{
        campaigns: make(map[string]*models.Campaign),
    }
}

// GetCampaign returns the campaign with the given ID or nil if not found.
func (r *InMemoryCampaignRepo) GetCampaign(id string) (*models.Campaign, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    if c, ok := r.campaigns[id]; ok {
        return c, nil
    }
    return nil, nil
}

// ListCampaigns returns a slice containing all campaigns.
func (r *InMemoryCampaignRepo) ListCampaigns() ([]*models.Campaign, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    res := make([]*models.Campaign, 0, len(r.campaigns))
    for _, c := range r.campaigns {
        res = append(res, c)
    }
    return res, nil
}

// UpsertCampaign inserts or updates the given campaign.  It makes a
// shallow copy of the provided value to avoid external mutation of the
// stored object.
func (r *InMemoryCampaignRepo) UpsertCampaign(c *models.Campaign) error {
    if c == nil {
        return nil
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    cp := *c
    r.campaigns[c.ID] = &cp
    return nil
}