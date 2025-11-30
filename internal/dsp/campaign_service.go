package dsp

import (
    "time"

    "github.com/radiusdt/vector-dsp/internal/models"
    "github.com/radiusdt/vector-dsp/internal/storage"
)

// CampaignService provides CRUD operations over campaigns.  It
// encapsulates validation and timestamp management, delegating
// persistence to the underlying repository.  CampaignService is
// intentionally thin; any cross-cutting logic such as audits or
// authorization should be implemented at a higher layer.
type CampaignService struct {
    repo storage.CampaignRepo
}

// NewCampaignService constructs a CampaignService backed by the given repo.
func NewCampaignService(repo storage.CampaignRepo) *CampaignService {
    return &CampaignService{repo: repo}
}

// ListCampaigns returns all campaigns.
func (s *CampaignService) ListCampaigns() ([]*models.Campaign, error) {
    return s.repo.ListCampaigns()
}

// GetCampaign returns a campaign by ID.
func (s *CampaignService) GetCampaign(id string) (*models.Campaign, error) {
    return s.repo.GetCampaign(id)
}

// UpsertCampaign validates the campaign, populates timestamps and saves it.
// If CreatedAt is zero it is set to now.  UpdatedAt is always set to now.
func (s *CampaignService) UpsertCampaign(c *models.Campaign) error {
    now := time.Now().UTC()
    if c.CreatedAt.IsZero() {
        c.CreatedAt = now
    }
    c.UpdatedAt = now
    if err := c.Validate(); err != nil {
        return err
    }
    return s.repo.UpsertCampaign(c)
}