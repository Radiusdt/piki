package dsp

import (
    "time"

    "github.com/radiusdt/vector-dsp/internal/models"
    "github.com/radiusdt/vector-dsp/internal/storage"
)

// CreativeService provides CRUD operations over creatives.  It sets
// timestamps and delegates persistence to the repository.  When used
// together with campaigns, creatives can be referenced by line items
// through their ID.  This service does not enforce any specific
// relations between advertisers and campaigns.
type CreativeService struct {
    repo storage.CreativeRepo
}

// NewCreativeService constructs a CreativeService backed by the given repo.
func NewCreativeService(repo storage.CreativeRepo) *CreativeService {
    return &CreativeService{repo: repo}
}

// ListCreatives returns all creatives, optionally filtered by advertiser ID.
func (s *CreativeService) ListCreatives(advertiserID string) ([]*models.Creative, error) {
    return s.repo.ListCreativesByAdvertiser(advertiserID)
}

// GetCreative returns a creative by ID.
func (s *CreativeService) GetCreative(id string) (*models.Creative, error) {
    return s.repo.GetCreative(id)
}

// UpsertCreative validates the creative and saves it.  It populates
// createdAt/updatedAt timestamps if the creative implements those
// fields.  The Creative struct in models defines no validation so
// additional checks (e.g. max size, valid URL) should be implemented
// at the API layer.
func (s *CreativeService) UpsertCreative(c *models.Creative) error {
    if c == nil {
        return nil
    }
    now := time.Now().UTC()
    if c.CreatedAt.IsZero() {
        c.CreatedAt = now
    }
    c.UpdatedAt = now
    return s.repo.UpsertCreative(c)
}