package storage

import (
    "sync"
    "time"
)

// Click represents a click event recorded by the system.  It links a
// campaign and line item to the user and the original target URL.  A
// click may later be associated with conversions.
type Click struct {
    ID         string    `json:"id"`
    CampaignID string    `json:"campaign_id"`
    LineItemID string    `json:"line_item_id"`
    UserID     string    `json:"user_id,omitempty"`
    TargetURL  string    `json:"target_url"`
    Timestamp  time.Time `json:"timestamp"`
}

// Conversion represents a server-to-server conversion event.  A
// conversion can be tied either to a click via ClickID or to some
// external identifier (e.g. app install ID).  Revenue may be
// optionally provided.
type Conversion struct {
    ID         string    `json:"id"`
    ClickID    string    `json:"click_id,omitempty"`
    ExternalID string    `json:"external_id,omitempty"`
    EventName  string    `json:"event_name"`
    Revenue    float64   `json:"revenue,omitempty"`
    Currency   string    `json:"currency,omitempty"`
    Timestamp  time.Time `json:"timestamp"`
}

// EventStore provides an abstraction for recording and retrieving
// click/conversion events.  This can be backed by a database,
// analytics pipeline or simple in-memory map.
type EventStore interface {
    SaveClick(click *Click) error
    SaveConversion(conv *Conversion) error
    GetClick(id string) (*Click, error)

    // ListClicks returns all clicks recorded in the store.  In
    // production implementations you would query a database or
    // streaming system.
    ListClicks() ([]*Click, error)
    // ListConversions returns all conversions recorded in the store.
    ListConversions() ([]*Conversion, error)
}

// InMemoryEventStore stores events in memory.  This implementation is
// not durable and resets on process restart.  It is intended for
// demonstration and testing.  For production you would use a database
// or streaming system and index events for analytics.
type InMemoryEventStore struct {
    mu          sync.RWMutex
    clicks      map[string]*Click
    conversions map[string]*Conversion
}

// NewInMemoryEventStore constructs a new empty event store.
func NewInMemoryEventStore() *InMemoryEventStore {
    return &InMemoryEventStore{
        clicks:      make(map[string]*Click),
        conversions: make(map[string]*Conversion),
    }
}

// SaveClick stores the given click event.  If an event with the same ID
// already exists it will be overwritten.
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

// SaveConversion stores the given conversion event.  If an event with
// the same ID already exists it will be overwritten.
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

// GetClick returns the click with the given ID or nil if not found.
func (s *InMemoryEventStore) GetClick(id string) (*Click, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if c, ok := s.clicks[id]; ok {
        return c, nil
    }
    return nil, nil
}

// ListClicks returns a slice of all clicks recorded in the store.
func (s *InMemoryEventStore) ListClicks() ([]*Click, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    res := make([]*Click, 0, len(s.clicks))
    for _, c := range s.clicks {
        res = append(res, c)
    }
    return res, nil
}

// ListConversions returns a slice of all conversions recorded in the store.
func (s *InMemoryEventStore) ListConversions() ([]*Conversion, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    res := make([]*Conversion, 0, len(s.conversions))
    for _, c := range s.conversions {
        res = append(res, c)
    }
    return res, nil
}