package dsp

import (
	"errors"
	"math/rand"
	"strconv"
	"time"

	"github.com/radiusdt/vector-dsp/internal/models"
	"github.com/radiusdt/vector-dsp/internal/storage"
)

// CampaignService provides CRUD operations over campaigns.
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

// AdvertiserService provides CRUD operations over advertisers.
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

// UpsertAdvertiser validates and saves an advertiser.
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

// AdGroupService provides CRUD operations over ad groups.
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

// UpsertAdGroup validates and stores an ad group.
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

// CreativeService provides CRUD operations over creatives.
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

// UpsertCreative validates the creative and saves it.
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

// EventService encapsulates click and conversion registration.
type EventService struct {
	store storage.EventStore
}

// NewEventService constructs an EventService backed by the given store.
func NewEventService(store storage.EventStore) *EventService {
	return &EventService{store: store}
}

// generateID produces a pseudo-random ID.
func generateID(prefix string) string {
	ts := time.Now().UnixNano()
	return prefix + strconv.FormatInt(ts, 36) + strconv.FormatInt(int64(rand.Int31()), 36)
}

// RegisterClick records a click event and returns the click ID and redirect URL.
func (s *EventService) RegisterClick(campaignID, lineItemID, userID, targetURL string) (string, string, error) {
	if campaignID == "" || lineItemID == "" || targetURL == "" {
		return "", "", errors.New("missing campaignID, lineItemID or targetURL")
	}
	id := generateID("clk_")
	click := &storage.Click{
		ID:         id,
		CampaignID: campaignID,
		LineItemID: lineItemID,
		UserID:     userID,
		TargetURL:  targetURL,
		Timestamp:  time.Now().UTC(),
	}
	if err := s.store.SaveClick(click); err != nil {
		return "", "", err
	}
	return id, targetURL, nil
}

// RegisterConversion records a conversion event.
func (s *EventService) RegisterConversion(clickID, externalID, eventName, revenueStr, currency string) error {
	if clickID == "" && externalID == "" {
		return errors.New("clickID or externalID required")
	}
	revenue := 0.0
	if revenueStr != "" {
		if val, err := strconv.ParseFloat(revenueStr, 64); err == nil {
			revenue = val
		}
	}
	conv := &storage.Conversion{
		ID:         generateID("cnv_"),
		ClickID:    clickID,
		ExternalID: externalID,
		EventName:  eventName,
		Revenue:    revenue,
		Currency:   currency,
		Timestamp:  time.Now().UTC(),
	}
	return s.store.SaveConversion(conv)
}
