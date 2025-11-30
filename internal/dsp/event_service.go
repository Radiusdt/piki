package dsp

import (
    "errors"
    "math/rand"
    "strconv"
    "time"

    "github.com/radiusdt/vector-dsp/internal/storage"
)

// EventService encapsulates click and conversion registration.  It
// generates identifiers, stores events and performs minimal validation.
type EventService struct {
    store storage.EventStore
}

// NewEventService constructs an EventService backed by the given store.
func NewEventService(store storage.EventStore) *EventService {
    return &EventService{store: store}
}

// generateID produces a pseudo-random ID suitable for clicks and
// conversions.  It combines the current timestamp with a random
// component.  This implementation is not cryptographically secure but
// suffices for logging and analytics.
func generateID(prefix string) string {
    ts := time.Now().UnixNano()
    return prefix + strconv.FormatInt(ts, 36) + strconv.FormatInt(int64(rand.Int31()), 36)
}

// RegisterClick records a click event and returns a unique click ID
// along with the URL to which the user should be redirected.  The
// target URL is not modified; in a real DSP you would append
// macros/parameters for click tracking.  If campaignID or lineItemID
// are empty an error is returned.
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
    // In a production system you might add click macros here
    return id, targetURL, nil
}

// RegisterConversion records a conversion event.  A conversion may be
// associated with a click via clickID or by an externalID.  Revenue
// should be provided as a string to preserve decimal precision and is
// parsed into a float64.  If neither clickID nor externalID are
// provided, an error is returned.
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
        ID:         generateID("cnv_") ,
        ClickID:    clickID,
        ExternalID: externalID,
        EventName:  eventName,
        Revenue:    revenue,
        Currency:   currency,
        Timestamp:  time.Now().UTC(),
    }
    return s.store.SaveConversion(conv)
}