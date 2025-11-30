package storage

import (
    "sync"
    "github.com/radiusdt/vector-dsp/internal/models"
)

// AdGroupRepo defines operations for storing and retrieving ad groups.
// An AdGroup belongs to a campaign.
type AdGroupRepo interface {
    GetAdGroup(id string) (*models.AdGroup, error)
    ListAdGroups() ([]*models.AdGroup, error)
    UpsertAdGroup(g *models.AdGroup) error
    ListAdGroupsByCampaign(campaignID string) ([]*models.AdGroup, error)
}

// InMemoryAdGroupRepo is a simple thread-safe in-memory implementation
// of AdGroupRepo.
type InMemoryAdGroupRepo struct {
    mu      sync.RWMutex
    groups  map[string]*models.AdGroup
}

// NewInMemoryAdGroupRepo creates an empty in-memory ad group repo.
func NewInMemoryAdGroupRepo() *InMemoryAdGroupRepo {
    return &InMemoryAdGroupRepo{
        groups: make(map[string]*models.AdGroup),
    }
}

// GetAdGroup returns an ad group by ID.
func (r *InMemoryAdGroupRepo) GetAdGroup(id string) (*models.AdGroup, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    if g, ok := r.groups[id]; ok {
        return g, nil
    }
    return nil, nil
}

// ListAdGroups returns all ad groups.
func (r *InMemoryAdGroupRepo) ListAdGroups() ([]*models.AdGroup, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    res := make([]*models.AdGroup, 0, len(r.groups))
    for _, g := range r.groups {
        res = append(res, g)
    }
    return res, nil
}

// UpsertAdGroup inserts or updates the given ad group.
func (r *InMemoryAdGroupRepo) UpsertAdGroup(g *models.AdGroup) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    cp := *g
    r.groups[g.ID] = &cp
    return nil
}

// ListAdGroupsByCampaign returns all ad groups for a given campaign.
func (r *InMemoryAdGroupRepo) ListAdGroupsByCampaign(campaignID string) ([]*models.AdGroup, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var res []*models.AdGroup
    for _, g := range r.groups {
        if g.CampaignID == campaignID {
            res = append(res, g)
        }
    }
    return res, nil
}