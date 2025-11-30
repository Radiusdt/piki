package storage

import (
	"sync"
	"time"

	"github.com/radiusdt/vector-dsp/internal/models"
)

// CampaignRepo defines CRUD operations for campaigns.
type CampaignRepo interface {
	GetCampaign(id string) (*models.Campaign, error)
	ListCampaigns() ([]*models.Campaign, error)
	UpsertCampaign(c *models.Campaign) error
}

// AdvertiserRepo defines operations for advertisers.
type AdvertiserRepo interface {
	GetAdvertiser(id string) (*models.Advertiser, error)
	ListAdvertisers() ([]*models.Advertiser, error)
	UpsertAdvertiser(a *models.Advertiser) error
}

// AdGroupRepo defines operations for ad groups.
type AdGroupRepo interface {
	GetAdGroup(id string) (*models.AdGroup, error)
	ListAdGroups() ([]*models.AdGroup, error)
	UpsertAdGroup(g *models.AdGroup) error
	ListAdGroupsByCampaign(campaignID string) ([]*models.AdGroup, error)
}

// CreativeRepo defines CRUD operations for creatives.
type CreativeRepo interface {
	GetCreative(id string) (*models.Creative, error)
	ListCreatives() ([]*models.Creative, error)
	UpsertCreative(c *models.Creative) error
	ListCreativesByAdvertiser(advertiserID string) ([]*models.Creative, error)
}

// Click represents a click event.
type Click struct {
	ID         string    `json:"id"`
	CampaignID string    `json:"campaign_id"`
	LineItemID string    `json:"line_item_id"`
	UserID     string    `json:"user_id,omitempty"`
	TargetURL  string    `json:"target_url"`
	Timestamp  time.Time `json:"timestamp"`
}

// Conversion represents a conversion event.
type Conversion struct {
	ID         string    `json:"id"`
	ClickID    string    `json:"click_id,omitempty"`
	ExternalID string    `json:"external_id,omitempty"`
	EventName  string    `json:"event_name"`
	Revenue    float64   `json:"revenue,omitempty"`
	Currency   string    `json:"currency,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// EventStore provides an abstraction for click/conversion events.
type EventStore interface {
	SaveClick(click *Click) error
	SaveConversion(conv *Conversion) error
	GetClick(id string) (*Click, error)
	ListClicks() ([]*Click, error)
	ListConversions() ([]*Conversion, error)
}

// In-memory implementations

// InMemoryCampaignRepo stores campaigns in memory.
type InMemoryCampaignRepo struct {
	mu        sync.RWMutex
	campaigns map[string]*models.Campaign
}

func NewInMemoryCampaignRepo() *InMemoryCampaignRepo {
	return &InMemoryCampaignRepo{
		campaigns: make(map[string]*models.Campaign),
	}
}

func (r *InMemoryCampaignRepo) GetCampaign(id string) (*models.Campaign, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if c, ok := r.campaigns[id]; ok {
		return c, nil
	}
	return nil, nil
}

func (r *InMemoryCampaignRepo) ListCampaigns() ([]*models.Campaign, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]*models.Campaign, 0, len(r.campaigns))
	for _, c := range r.campaigns {
		res = append(res, c)
	}
	return res, nil
}

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

// InMemoryAdvertiserRepo stores advertisers in memory.
type InMemoryAdvertiserRepo struct {
	mu          sync.RWMutex
	advertisers map[string]*models.Advertiser
}

func NewInMemoryAdvertiserRepo() *InMemoryAdvertiserRepo {
	return &InMemoryAdvertiserRepo{
		advertisers: make(map[string]*models.Advertiser),
	}
}

func (r *InMemoryAdvertiserRepo) GetAdvertiser(id string) (*models.Advertiser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if a, ok := r.advertisers[id]; ok {
		return a, nil
	}
	return nil, nil
}

func (r *InMemoryAdvertiserRepo) ListAdvertisers() ([]*models.Advertiser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]*models.Advertiser, 0, len(r.advertisers))
	for _, a := range r.advertisers {
		res = append(res, a)
	}
	return res, nil
}

func (r *InMemoryAdvertiserRepo) UpsertAdvertiser(a *models.Advertiser) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *a
	r.advertisers[a.ID] = &cp
	return nil
}

// InMemoryAdGroupRepo stores ad groups in memory.
type InMemoryAdGroupRepo struct {
	mu     sync.RWMutex
	groups map[string]*models.AdGroup
}

func NewInMemoryAdGroupRepo() *InMemoryAdGroupRepo {
	return &InMemoryAdGroupRepo{
		groups: make(map[string]*models.AdGroup),
	}
}

func (r *InMemoryAdGroupRepo) GetAdGroup(id string) (*models.AdGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if g, ok := r.groups[id]; ok {
		return g, nil
	}
	return nil, nil
}

func (r *InMemoryAdGroupRepo) ListAdGroups() ([]*models.AdGroup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]*models.AdGroup, 0, len(r.groups))
	for _, g := range r.groups {
		res = append(res, g)
	}
	return res, nil
}

func (r *InMemoryAdGroupRepo) UpsertAdGroup(g *models.AdGroup) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *g
	r.groups[g.ID] = &cp
	return nil
}

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

// InMemoryCreativeRepo stores creatives in memory.
type InMemoryCreativeRepo struct {
	mu        sync.RWMutex
	creatives map[string]*models.Creative
}

func NewInMemoryCreativeRepo() *InMemoryCreativeRepo {
	return &InMemoryCreativeRepo{
		creatives: make(map[string]*models.Creative),
	}
}

func (r *InMemoryCreativeRepo) GetCreative(id string) (*models.Creative, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if c, ok := r.creatives[id]; ok {
		return c, nil
	}
	return nil, nil
}

func (r *InMemoryCreativeRepo) ListCreatives() ([]*models.Creative, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]*models.Creative, 0, len(r.creatives))
	for _, c := range r.creatives {
		res = append(res, c)
	}
	return res, nil
}

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

// InMemoryEventStore stores events in memory.
type InMemoryEventStore struct {
	mu          sync.RWMutex
	clicks      map[string]*Click
	conversions map[string]*Conversion
}

func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		clicks:      make(map[string]*Click),
		conversions: make(map[string]*Conversion),
	}
}

func (s *InMemoryEventStore) SaveClick(click *Click) error {
	if click == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *click
	s.clicks[click.ID] = &cp
	return nil
}

func (s *InMemoryEventStore) SaveConversion(conv *Conversion) error {
	if conv == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *conv
	s.conversions[conv.ID] = &cp
	return nil
}

func (s *InMemoryEventStore) GetClick(id string) (*Click, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if c, ok := s.clicks[id]; ok {
		return c, nil
	}
	return nil, nil
}

func (s *InMemoryEventStore) ListClicks() ([]*Click, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]*Click, 0, len(s.clicks))
	for _, c := range s.clicks {
		res = append(res, c)
	}
	return res, nil
}

func (s *InMemoryEventStore) ListConversions() ([]*Conversion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]*Conversion, 0, len(s.conversions))
	for _, c := range s.conversions {
		res = append(res, c)
	}
	return res, nil
}
