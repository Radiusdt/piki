package dsp

import (
    "github.com/radiusdt/vector-dsp/internal/storage"
)

// CampaignStats aggregates click and conversion metrics for a single
// campaign.  Revenue sums up the revenue recorded from conversions.
type CampaignStats struct {
    CampaignID  string  `json:"campaign_id"`
    Clicks      int     `json:"clicks"`
    Conversions int     `json:"conversions"`
    Revenue     float64 `json:"revenue"`
}

// StatsService computes aggregate statistics from the event store.
// It joins conversions to clicks to associate conversions with
// campaigns.  This implementation only works with the InMemoryEventStore
// which exposes list methods; for production you would perform
// aggregation in a data warehouse or analytics system.
type StatsService struct {
    store storage.EventStore
}

// NewStatsService constructs a StatsService backed by the given store.
func NewStatsService(store storage.EventStore) *StatsService {
    return &StatsService{store: store}
}

// AggregateByCampaign returns stats grouped by campaign.  The method
// iterates through all clicks and conversions in the store.  For
// conversions, it attempts to find the associated click via clickID to
// determine the campaign.  Conversions with no matching click are
// ignored.  This method may be expensive for large datasets.
func (s *StatsService) AggregateByCampaign() ([]CampaignStats, error) {
    clicks, err := s.store.ListClicks()
    if err != nil {
        return nil, err
    }
    conversions, err := s.store.ListConversions()
    if err != nil {
        return nil, err
    }
    // Build map of campaignID -> stats
    statsMap := make(map[string]*CampaignStats)
    // Count clicks
    for _, c := range clicks {
        st, ok := statsMap[c.CampaignID]
        if !ok {
            st = &CampaignStats{CampaignID: c.CampaignID}
            statsMap[c.CampaignID] = st
        }
        st.Clicks++
    }
    // Count conversions
    for _, conv := range conversions {
        if conv.ClickID == "" {
            continue
        }
        click, err := s.store.GetClick(conv.ClickID)
        if err != nil || click == nil {
            continue
        }
        st, ok := statsMap[click.CampaignID]
        if !ok {
            st = &CampaignStats{CampaignID: click.CampaignID}
            statsMap[click.CampaignID] = st
        }
        st.Conversions++
        st.Revenue += conv.Revenue
    }
    // Build slice
    res := make([]CampaignStats, 0, len(statsMap))
    for _, st := range statsMap {
        res = append(res, *st)
    }
    return res, nil
}