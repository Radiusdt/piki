package storage

import (
	"context"
	"sync"
	"time"

	"github.com/radiusdt/vector-dsp/internal/models"
)

// InMemoryEventStore provides in-memory storage for events.
type InMemoryEventStore struct {
	mu          sync.RWMutex
	clicks      map[string]*models.Click
	impressions map[string]*models.Impression
	conversions map[string]*models.Conversion
	wins        map[string]*models.Win

	// Indexes for faster lookups
	clicksByDevice     map[string][]string // device_ifa -> []click_id
	conversionsByClick map[string][]string // click_id -> []conversion_id
}

// NewInMemoryEventStore creates a new in-memory event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		clicks:             make(map[string]*models.Click),
		impressions:        make(map[string]*models.Impression),
		conversions:        make(map[string]*models.Conversion),
		wins:               make(map[string]*models.Win),
		clicksByDevice:     make(map[string][]string),
		conversionsByClick: make(map[string][]string),
	}
}

// =============================================
// Clicks
// =============================================

func (s *InMemoryEventStore) SaveClick(ctx context.Context, click *models.Click) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clicks[click.ID] = click

	// Update device index
	if click.DeviceIFA != "" {
		s.clicksByDevice[click.DeviceIFA] = append(s.clicksByDevice[click.DeviceIFA], click.ID)
	}

	return nil
}

func (s *InMemoryEventStore) GetClick(ctx context.Context, id string) (*models.Click, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	click, ok := s.clicks[id]
	if !ok {
		return nil, nil
	}
	return click, nil
}

func (s *InMemoryEventStore) GetClicksByDevice(ctx context.Context, deviceIFA string, since time.Time) ([]*models.Click, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clickIDs, ok := s.clicksByDevice[deviceIFA]
	if !ok {
		return nil, nil
	}

	result := make([]*models.Click, 0)
	for _, id := range clickIDs {
		click := s.clicks[id]
		if click != nil && click.Timestamp.After(since) {
			result = append(result, click)
		}
	}
	return result, nil
}

// =============================================
// Impressions
// =============================================

func (s *InMemoryEventStore) SaveImpression(ctx context.Context, imp *models.Impression) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.impressions[imp.ID] = imp
	return nil
}

func (s *InMemoryEventStore) GetImpression(ctx context.Context, id string) (*models.Impression, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	imp, ok := s.impressions[id]
	if !ok {
		return nil, nil
	}
	return imp, nil
}

// =============================================
// Conversions
// =============================================

func (s *InMemoryEventStore) SaveConversion(ctx context.Context, conv *models.Conversion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conversions[conv.ID] = conv

	// Update click index
	if conv.ClickID != "" {
		s.conversionsByClick[conv.ClickID] = append(s.conversionsByClick[conv.ClickID], conv.ID)
	}

	return nil
}

func (s *InMemoryEventStore) GetConversion(ctx context.Context, id string) (*models.Conversion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conv, ok := s.conversions[id]
	if !ok {
		return nil, nil
	}
	return conv, nil
}

func (s *InMemoryEventStore) GetConversionsByClick(ctx context.Context, clickID string) ([]*models.Conversion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	convIDs, ok := s.conversionsByClick[clickID]
	if !ok {
		return nil, nil
	}

	result := make([]*models.Conversion, 0, len(convIDs))
	for _, id := range convIDs {
		if conv, ok := s.conversions[id]; ok {
			result = append(result, conv)
		}
	}
	return result, nil
}

// =============================================
// Wins
// =============================================

func (s *InMemoryEventStore) SaveWin(ctx context.Context, win *models.Win) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.wins[win.ID] = win
	return nil
}

// =============================================
// Aggregations
// =============================================

func (s *InMemoryEventStore) GetClickCount(ctx context.Context, campaignID string, since time.Time) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, click := range s.clicks {
		if click.CampaignID == campaignID && click.Timestamp.After(since) {
			count++
		}
	}
	return count, nil
}

func (s *InMemoryEventStore) GetImpressionCount(ctx context.Context, campaignID string, since time.Time) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, imp := range s.impressions {
		if imp.CampaignID == campaignID && imp.Timestamp.After(since) {
			count++
		}
	}
	return count, nil
}

func (s *InMemoryEventStore) GetConversionCount(ctx context.Context, campaignID string, event string, since time.Time) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, conv := range s.conversions {
		if conv.CampaignID == campaignID && conv.Timestamp.After(since) {
			if event == "" || conv.Event == event {
				count++
			}
		}
	}
	return count, nil
}

// =============================================
// Cleanup (for TTL)
// =============================================

func (s *InMemoryEventStore) CleanupOldClicks(ctx context.Context, before time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int64
	toDelete := make([]string, 0)

	for id, click := range s.clicks {
		if click.Timestamp.Before(before) {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		click := s.clicks[id]
		delete(s.clicks, id)

		// Clean up device index
		if click.DeviceIFA != "" {
			if ids, ok := s.clicksByDevice[click.DeviceIFA]; ok {
				newIDs := make([]string, 0)
				for _, cid := range ids {
					if cid != id {
						newIDs = append(newIDs, cid)
					}
				}
				if len(newIDs) > 0 {
					s.clicksByDevice[click.DeviceIFA] = newIDs
				} else {
					delete(s.clicksByDevice, click.DeviceIFA)
				}
			}
		}
		count++
	}

	return count, nil
}

// =============================================
// PostgresEventStore (stub)
// =============================================

// PostgresEventStore provides PostgreSQL storage for events.
type PostgresEventStore struct {
	// pool *pgxpool.Pool
}

// NewPostgresEventStore creates a new PostgreSQL event store.
func NewPostgresEventStore(pool interface{}) *InMemoryEventStore {
	// TODO: Implement PostgreSQL storage
	// For now, return in-memory
	return NewInMemoryEventStore()
}
