package dsp

import (
    "time"
    "github.com/radiusdt/vector-dsp/internal/models"
    "github.com/radiusdt/vector-dsp/internal/storage"
)

// AdvertiserService provides CRUD operations over advertisers.
// It wraps a repository and adds timestamp management and validation.
type AdvertiserService struct {
    repo storage.AdvertiserRepo
}

// NewAdvertiserService constructs a new AdvertiserService.
func NewAdvertiserService(repo storage.AdvertiserRepo) *AdvertiserService {
    return &AdvertiserService{repo: repo}
}

// ListAdvertisers returns all advertisers.
func (s *AdvertiserService) ListAdvertisers() ([]*models.Advertiser, error) {
    return s.repo.ListAdvertisers()
}

// GetAdvertiser returns advertiser by ID.
func (s *AdvertiserService) GetAdvertiser(id string) (*models.Advertiser, error) {
    return s.repo.GetAdvertiser(id)
}

// UpsertAdvertiser validates and saves an advertiser.  If CreatedAt is zero
// it sets it to now.  UpdatedAt is always set to now.
func (s *AdvertiserService) UpsertAdvertiser(a *models.Advertiser) error {
    now := time.Now().UTC()
    if a.CreatedAt.IsZero() {
        a.CreatedAt = now
    }
    a.UpdatedAt = now
    if err := a.Validate(); err != nil {
        return err
    }
    return s.repo.UpsertAdvertiser(a)
}