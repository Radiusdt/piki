package dsp

import (
    "time"
    "github.com/radiusdt/vector-dsp/internal/models"
    "github.com/radiusdt/vector-dsp/internal/storage"
)

// AdGroupService provides CRUD operations over ad groups.
// It manages timestamps and validation before delegating to the repository.
type AdGroupService struct {
    repo storage.AdGroupRepo
}

// NewAdGroupService constructs a new AdGroupService.
func NewAdGroupService(repo storage.AdGroupRepo) *AdGroupService {
    return &AdGroupService{repo: repo}
}

// ListAdGroups returns all ad groups.
func (s *AdGroupService) ListAdGroups() ([]*models.AdGroup, error) {
    return s.repo.ListAdGroups()
}

// ListAdGroupsByCampaign lists ad groups for a specific campaign.
func (s *AdGroupService) ListAdGroupsByCampaign(campaignID string) ([]*models.AdGroup, error) {
    return s.repo.ListAdGroupsByCampaign(campaignID)
}

// GetAdGroup returns an ad group by ID.
func (s *AdGroupService) GetAdGroup(id string) (*models.AdGroup, error) {
    return s.repo.GetAdGroup(id)
}

// UpsertAdGroup validates and stores an ad group with timestamp management.
func (s *AdGroupService) UpsertAdGroup(g *models.AdGroup) error {
    now := time.Now().UTC()
    if g.CreatedAt.IsZero() {
        g.CreatedAt = now
    }
    g.UpdatedAt = now
    if err := g.Validate(); err != nil {
        return err
    }
    return s.repo.UpsertAdGroup(g)
}