package dsp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/radiusdt/vector-dsp/internal/models"
	"github.com/radiusdt/vector-dsp/internal/storage"
)

// SourceService manages traffic sources (S2S partners and RTB exchanges).
type SourceService struct {
	repo storage.SourceRepo
}

// NewSourceService creates a new source service.
func NewSourceService(repo storage.SourceRepo) *SourceService {
	return &SourceService{repo: repo}
}

// =============================================
// S2S Sources
// =============================================

// ListS2SSources returns all S2S sources.
func (s *SourceService) ListS2SSources(ctx context.Context) ([]*models.S2SSource, error) {
	return s.repo.ListS2SSources(ctx)
}

// GetS2SSource returns a single S2S source by ID.
func (s *SourceService) GetS2SSource(ctx context.Context, id string) (*models.S2SSource, error) {
	return s.repo.GetS2SSource(ctx, id)
}

// GetS2SSourceByName returns a single S2S source by internal name.
func (s *SourceService) GetS2SSourceByName(ctx context.Context, internalName string) (*models.S2SSource, error) {
	return s.repo.GetS2SSourceByName(ctx, internalName)
}

// UpsertS2SSource creates or updates an S2S source.
func (s *SourceService) UpsertS2SSource(ctx context.Context, src *models.S2SSource) error {
	if src.ID == "" {
		src.ID = uuid.New().String()
	}
	if src.InternalName == "" {
		return errors.New("internal_name is required")
	}
	if src.Name == "" {
		return errors.New("name is required")
	}
	if src.Status == "" {
		src.Status = "active"
	}
	if src.CreatedAt.IsZero() {
		src.CreatedAt = time.Now()
	}
	src.UpdatedAt = time.Now()

	return s.repo.UpsertS2SSource(ctx, src)
}

// DeleteS2SSource deletes an S2S source.
func (s *SourceService) DeleteS2SSource(ctx context.Context, id string) error {
	return s.repo.DeleteS2SSource(ctx, id)
}

// ValidateS2SToken validates API token for S2S source.
func (s *SourceService) ValidateS2SToken(ctx context.Context, sourceID, token string) (bool, error) {
	src, err := s.repo.GetS2SSource(ctx, sourceID)
	if err != nil {
		return false, err
	}
	if src == nil {
		return false, nil
	}
	if src.APIToken == "" {
		return true, nil // No token required
	}
	return src.APIToken == token, nil
}

// ValidateS2SIP validates IP address for S2S source.
func (s *SourceService) ValidateS2SIP(ctx context.Context, sourceID, ip string) (bool, error) {
	src, err := s.repo.GetS2SSource(ctx, sourceID)
	if err != nil {
		return false, err
	}
	if src == nil {
		return false, nil
	}
	if len(src.AllowedIPs) == 0 {
		return true, nil // No IP restriction
	}
	for _, allowed := range src.AllowedIPs {
		if allowed == ip || allowed == "*" {
			return true, nil
		}
	}
	return false, nil
}

// =============================================
// RTB Sources
// =============================================

// ListRTBSources returns all RTB sources.
func (s *SourceService) ListRTBSources(ctx context.Context) ([]*models.RTBSource, error) {
	return s.repo.ListRTBSources(ctx)
}

// GetRTBSource returns a single RTB source by ID.
func (s *SourceService) GetRTBSource(ctx context.Context, id string) (*models.RTBSource, error) {
	return s.repo.GetRTBSource(ctx, id)
}

// UpsertRTBSource creates or updates an RTB source.
func (s *SourceService) UpsertRTBSource(ctx context.Context, src *models.RTBSource) error {
	if src.ID == "" {
		src.ID = uuid.New().String()
	}
	if src.Name == "" {
		return errors.New("name is required")
	}
	if src.Status == "" {
		src.Status = "active"
	}
	if src.ProtocolVersion == "" {
		src.ProtocolVersion = "2.5"
	}
	if src.BidAdjustment == 0 {
		src.BidAdjustment = 1.0
	}
	if src.MaxQPS == 0 {
		src.MaxQPS = 1000
	}
	if src.TimeoutMs == 0 {
		src.TimeoutMs = 100
	}
	if src.CreatedAt.IsZero() {
		src.CreatedAt = time.Now()
	}
	src.UpdatedAt = time.Now()

	return s.repo.UpsertRTBSource(ctx, src)
}

// DeleteRTBSource deletes an RTB source.
func (s *SourceService) DeleteRTBSource(ctx context.Context, id string) error {
	return s.repo.DeleteRTBSource(ctx, id)
}

// =============================================
// Campaign-Source Linking
// =============================================

// LinkCampaignSource links a campaign to a source.
func (s *SourceService) LinkCampaignSource(ctx context.Context, link *models.CampaignSource) error {
	if link.ID == "" {
		link.ID = uuid.New().String()
	}
	if link.CampaignID == "" {
		return errors.New("campaign_id is required")
	}
	if link.SourceType == "" {
		return errors.New("source_type is required")
	}
	if link.SourceID == "" {
		return errors.New("source_id is required")
	}
	if link.Status == "" {
		link.Status = "active"
	}
	link.CreatedAt = time.Now()
	link.UpdatedAt = time.Now()

	return s.repo.UpsertCampaignSource(ctx, link)
}

// GetCampaignSources returns all sources linked to a campaign.
func (s *SourceService) GetCampaignSources(ctx context.Context, campaignID string) ([]*models.CampaignSource, error) {
	return s.repo.GetCampaignSources(ctx, campaignID)
}

// FindCampaignForSource finds a matching campaign for an S2S source request.
func (s *SourceService) FindCampaignForSource(
	ctx context.Context,
	sourceID string,
	country string,
	os string,
) (*models.Campaign, *models.Creative, error) {
	// Get campaigns linked to this source
	campaigns, err := s.repo.GetCampaignsForSource(ctx, "s2s", sourceID)
	if err != nil {
		return nil, nil, err
	}

	// Find first matching campaign
	for _, campaign := range campaigns {
		if !matchesTargeting(campaign, country, os) {
			continue
		}

		// Get creative for campaign
		creative := getFirstCreative(campaign)
		if creative == nil {
			continue
		}

		return campaign, creative, nil
	}

	return nil, nil, fmt.Errorf("no matching campaign found")
}

// =============================================
// Helper Functions
// =============================================

func matchesTargeting(campaign *models.Campaign, country, os string) bool {
	// Check if campaign has any line items
	if len(campaign.LineItems) == 0 {
		return false
	}

	// Check status
	if campaign.Status != "active" {
		return false
	}

	// Get first line item's targeting (simplified)
	li := campaign.LineItems[0]
	if li.Status != "active" {
		return false
	}

	// Check geo targeting
	if len(li.Targeting.Countries) > 0 {
		found := false
		for _, c := range li.Targeting.Countries {
			if c == country {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check OS targeting
	if len(li.Targeting.OS) > 0 {
		found := false
		for _, o := range li.Targeting.OS {
			if o == os {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func getFirstCreative(campaign *models.Campaign) *models.Creative {
	if len(campaign.LineItems) == 0 {
		return nil
	}
	li := campaign.LineItems[0]
	if len(li.Creatives) == 0 {
		return nil
	}
	return &li.Creatives[0]
}
