package dsp

import (
	"context"
	"fmt"
	"time"

	"github.com/radiusdt/vector-dsp/internal/storage"
	"github.com/redis/go-redis/v9"
)

// ReportingService provides comprehensive campaign reporting.
type ReportingService struct {
	eventStore storage.EventStore
	redis      *redis.Client
}

// NewReportingService creates a new reporting service.
func NewReportingService(eventStore storage.EventStore, redis *redis.Client) *ReportingService {
	return &ReportingService{
		eventStore: eventStore,
		redis:      redis,
	}
}

// CampaignStats aggregates metrics for a campaign.
type CampaignStats struct {
	CampaignID     string    `json:"campaign_id"`
	CampaignName   string    `json:"campaign_name,omitempty"`
	Date           string    `json:"date,omitempty"`
	
	// Volume metrics
	Impressions    int64     `json:"impressions"`
	Clicks         int64     `json:"clicks"`
	Conversions    int64     `json:"conversions"`
	
	// Financial metrics
	Spend          float64   `json:"spend"`
	Revenue        float64   `json:"revenue"`
	Profit         float64   `json:"profit"`
	
	// Rate metrics
	CTR            float64   `json:"ctr"`       // Click-through rate (%)
	CVR            float64   `json:"cvr"`       // Conversion rate (%)
	WinRate        float64   `json:"win_rate"`  // Auction win rate (%)
	
	// Cost metrics
	eCPM           float64   `json:"ecpm"`      // Effective CPM
	eCPC           float64   `json:"ecpc"`      // Effective CPC
	eCPA           float64   `json:"ecpa"`      // Effective CPA
	ROAS           float64   `json:"roas"`      // Return on ad spend
	
	// Pacing
	BudgetSpent    float64   `json:"budget_spent_pct"`
	
	LastUpdated    time.Time `json:"last_updated"`
}

// LineItemStats aggregates metrics for a line item.
type LineItemStats struct {
	LineItemID     string    `json:"line_item_id"`
	LineItemName   string    `json:"line_item_name,omitempty"`
	CampaignID     string    `json:"campaign_id"`
	Date           string    `json:"date,omitempty"`
	
	Impressions    int64     `json:"impressions"`
	Clicks         int64     `json:"clicks"`
	Conversions    int64     `json:"conversions"`
	Spend          float64   `json:"spend"`
	Revenue        float64   `json:"revenue"`
	
	CTR            float64   `json:"ctr"`
	CVR            float64   `json:"cvr"`
	eCPM           float64   `json:"ecpm"`
	eCPC           float64   `json:"ecpc"`
	eCPA           float64   `json:"ecpa"`
	
	// Pacing info
	DailyBudget    float64   `json:"daily_budget"`
	DailySpend     float64   `json:"daily_spend"`
	BudgetPacing   float64   `json:"budget_pacing_pct"`
	
	LastUpdated    time.Time `json:"last_updated"`
}

// CreativeStats aggregates metrics for a creative.
type CreativeStats struct {
	CreativeID     string    `json:"creative_id"`
	CreativeName   string    `json:"creative_name,omitempty"`
	LineItemID     string    `json:"line_item_id"`
	CampaignID     string    `json:"campaign_id"`
	
	Impressions    int64     `json:"impressions"`
	Clicks         int64     `json:"clicks"`
	CTR            float64   `json:"ctr"`
	Spend          float64   `json:"spend"`
	
	LastUpdated    time.Time `json:"last_updated"`
}

// GeoStats aggregates metrics by geography.
type GeoStats struct {
	Country        string    `json:"country"`
	Region         string    `json:"region,omitempty"`
	City           string    `json:"city,omitempty"`
	
	Impressions    int64     `json:"impressions"`
	Clicks         int64     `json:"clicks"`
	Conversions    int64     `json:"conversions"`
	Spend          float64   `json:"spend"`
	CTR            float64   `json:"ctr"`
}

// TimeSeriesPoint represents a single data point.
type TimeSeriesPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	Impressions    int64     `json:"impressions"`
	Clicks         int64     `json:"clicks"`
	Spend          float64   `json:"spend"`
	Conversions    int64     `json:"conversions"`
}

// ReportFilter defines filters for reports.
type ReportFilter struct {
	CampaignIDs    []string  `json:"campaign_ids,omitempty"`
	LineItemIDs    []string  `json:"line_item_ids,omitempty"`
	CreativeIDs    []string  `json:"creative_ids,omitempty"`
	AdvertiserID   string    `json:"advertiser_id,omitempty"`
	StartDate      time.Time `json:"start_date"`
	EndDate        time.Time `json:"end_date"`
	Granularity    string    `json:"granularity,omitempty"` // hourly, daily, weekly
	GroupBy        []string  `json:"group_by,omitempty"`    // campaign, line_item, creative, geo, device
}

// GetCampaignStats returns aggregated stats for campaigns.
func (r *ReportingService) GetCampaignStats(ctx context.Context, filter ReportFilter) ([]CampaignStats, error) {
	// Get clicks from event store
	clicks, err := r.eventStore.ListClicks()
	if err != nil {
		return nil, err
	}

	// Get conversions
	conversions, err := r.eventStore.ListConversions()
	if err != nil {
		return nil, err
	}

	// Build campaign stats map
	statsMap := make(map[string]*CampaignStats)

	// Aggregate clicks
	for _, c := range clicks {
		if !r.matchesFilter(c.CampaignID, filter) {
			continue
		}
		
		st, ok := statsMap[c.CampaignID]
		if !ok {
			st = &CampaignStats{
				CampaignID:  c.CampaignID,
				LastUpdated: time.Now(),
			}
			statsMap[c.CampaignID] = st
		}
		st.Clicks++
	}

	// Aggregate conversions
	for _, conv := range conversions {
		if conv.ClickID == "" {
			continue
		}
		click, err := r.eventStore.GetClick(conv.ClickID)
		if err != nil || click == nil {
			continue
		}
		
		if !r.matchesFilter(click.CampaignID, filter) {
			continue
		}

		st, ok := statsMap[click.CampaignID]
		if !ok {
			st = &CampaignStats{
				CampaignID:  click.CampaignID,
				LastUpdated: time.Now(),
			}
			statsMap[click.CampaignID] = st
		}
		st.Conversions++
		st.Revenue += conv.Revenue
	}

	// Get impressions and spend from Redis
	for campaignID, st := range statsMap {
		imps, spend := r.getImpressionData(ctx, campaignID, filter.StartDate, filter.EndDate)
		st.Impressions = imps
		st.Spend = spend
		
		// Calculate derived metrics
		r.calculateDerivedMetrics(st)
	}

	// Convert to slice
	result := make([]CampaignStats, 0, len(statsMap))
	for _, st := range statsMap {
		result = append(result, *st)
	}

	return result, nil
}

// GetLineItemStats returns aggregated stats for line items.
func (r *ReportingService) GetLineItemStats(ctx context.Context, filter ReportFilter) ([]LineItemStats, error) {
	// Similar implementation to GetCampaignStats but grouped by line item
	statsMap := make(map[string]*LineItemStats)

	clicks, err := r.eventStore.ListClicks()
	if err != nil {
		return nil, err
	}

	for _, c := range clicks {
		if !r.matchesLineItemFilter(c.LineItemID, filter) {
			continue
		}

		st, ok := statsMap[c.LineItemID]
		if !ok {
			st = &LineItemStats{
				LineItemID:  c.LineItemID,
				CampaignID:  c.CampaignID,
				LastUpdated: time.Now(),
			}
			statsMap[c.LineItemID] = st
		}
		st.Clicks++
	}

	// Get conversions
	conversions, _ := r.eventStore.ListConversions()
	for _, conv := range conversions {
		if conv.ClickID == "" {
			continue
		}
		click, _ := r.eventStore.GetClick(conv.ClickID)
		if click == nil {
			continue
		}

		st, ok := statsMap[click.LineItemID]
		if !ok {
			continue
		}
		st.Conversions++
		st.Revenue += conv.Revenue
	}

	// Get impressions from Redis
	for lineItemID, st := range statsMap {
		imps, spend := r.getLineItemImpressionData(ctx, lineItemID, filter.StartDate, filter.EndDate)
		st.Impressions = imps
		st.Spend = spend
		r.calculateLineItemDerivedMetrics(st)
	}

	result := make([]LineItemStats, 0, len(statsMap))
	for _, st := range statsMap {
		result = append(result, *st)
	}

	return result, nil
}

// GetTimeSeries returns time series data for a campaign.
func (r *ReportingService) GetTimeSeries(ctx context.Context, campaignID string, filter ReportFilter) ([]TimeSeriesPoint, error) {
	// This would typically query a time-series database
	// For now, return aggregated daily data from Redis
	
	var points []TimeSeriesPoint
	
	start := filter.StartDate
	if start.IsZero() {
		start = time.Now().AddDate(0, 0, -7)
	}
	end := filter.EndDate
	if end.IsZero() {
		end = time.Now()
	}

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		
		// Get daily stats from Redis
		impsKey := fmt.Sprintf("stats:imps:%s:%s", campaignID, dateStr)
		spendKey := fmt.Sprintf("stats:spend:%s:%s", campaignID, dateStr)
		clicksKey := fmt.Sprintf("stats:clicks:%s:%s", campaignID, dateStr)
		
		imps, _ := r.redis.Get(ctx, impsKey).Int64()
		spend, _ := r.redis.Get(ctx, spendKey).Float64()
		clicks, _ := r.redis.Get(ctx, clicksKey).Int64()

		points = append(points, TimeSeriesPoint{
			Timestamp:   d,
			Impressions: imps,
			Clicks:      clicks,
			Spend:       spend,
		})
	}

	return points, nil
}

// GetGeoBreakdown returns geographic breakdown of stats.
func (r *ReportingService) GetGeoBreakdown(ctx context.Context, campaignID string) ([]GeoStats, error) {
	// This would typically query aggregated geo data
	// For now, return empty slice as this requires additional tracking
	return []GeoStats{}, nil
}

// Helper methods

func (r *ReportingService) matchesFilter(campaignID string, filter ReportFilter) bool {
	if len(filter.CampaignIDs) == 0 {
		return true
	}
	for _, id := range filter.CampaignIDs {
		if id == campaignID {
			return true
		}
	}
	return false
}

func (r *ReportingService) matchesLineItemFilter(lineItemID string, filter ReportFilter) bool {
	if len(filter.LineItemIDs) == 0 {
		return true
	}
	for _, id := range filter.LineItemIDs {
		if id == lineItemID {
			return true
		}
	}
	return false
}

func (r *ReportingService) getImpressionData(ctx context.Context, campaignID string, start, end time.Time) (int64, float64) {
	var totalImps int64
	var totalSpend float64

	if start.IsZero() {
		start = time.Now().AddDate(0, 0, -30)
	}
	if end.IsZero() {
		end = time.Now()
	}

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		
		impsKey := fmt.Sprintf("stats:imps:%s:%s", campaignID, dateStr)
		spendKey := fmt.Sprintf("stats:spend:%s:%s", campaignID, dateStr)
		
		imps, _ := r.redis.Get(ctx, impsKey).Int64()
		spend, _ := r.redis.Get(ctx, spendKey).Float64()
		
		totalImps += imps
		totalSpend += spend
	}

	return totalImps, totalSpend
}

func (r *ReportingService) getLineItemImpressionData(ctx context.Context, lineItemID string, start, end time.Time) (int64, float64) {
	var totalImps int64
	var totalSpend float64

	if start.IsZero() {
		start = time.Now().AddDate(0, 0, -30)
	}
	if end.IsZero() {
		end = time.Now()
	}

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		
		impsKey := fmt.Sprintf("pacing:imps:%s:%s", lineItemID, dateStr)
		spendKey := fmt.Sprintf("pacing:budget:%s:%s", lineItemID, dateStr)
		
		imps, _ := r.redis.Get(ctx, impsKey).Int64()
		spend, _ := r.redis.Get(ctx, spendKey).Float64()
		
		totalImps += imps
		totalSpend += spend
	}

	return totalImps, totalSpend
}

func (r *ReportingService) calculateDerivedMetrics(st *CampaignStats) {
	// CTR
	if st.Impressions > 0 {
		st.CTR = float64(st.Clicks) / float64(st.Impressions) * 100
	}
	
	// CVR
	if st.Clicks > 0 {
		st.CVR = float64(st.Conversions) / float64(st.Clicks) * 100
	}
	
	// eCPM
	if st.Impressions > 0 {
		st.eCPM = st.Spend / float64(st.Impressions) * 1000
	}
	
	// eCPC
	if st.Clicks > 0 {
		st.eCPC = st.Spend / float64(st.Clicks)
	}
	
	// eCPA
	if st.Conversions > 0 {
		st.eCPA = st.Spend / float64(st.Conversions)
	}
	
	// ROAS
	if st.Spend > 0 {
		st.ROAS = st.Revenue / st.Spend
	}
	
	// Profit
	st.Profit = st.Revenue - st.Spend
}

func (r *ReportingService) calculateLineItemDerivedMetrics(st *LineItemStats) {
	if st.Impressions > 0 {
		st.CTR = float64(st.Clicks) / float64(st.Impressions) * 100
		st.eCPM = st.Spend / float64(st.Impressions) * 1000
	}
	if st.Clicks > 0 {
		st.CVR = float64(st.Conversions) / float64(st.Clicks) * 100
		st.eCPC = st.Spend / float64(st.Clicks)
	}
	if st.Conversions > 0 {
		st.eCPA = st.Spend / float64(st.Conversions)
	}
	if st.DailyBudget > 0 {
		st.BudgetPacing = st.DailySpend / st.DailyBudget * 100
	}
}

// StatsService provides backward compatibility with existing API.
type StatsService struct {
	reporting *ReportingService
}

// NewStatsService creates a new stats service.
func NewStatsService(eventStore storage.EventStore, redis *redis.Client) *StatsService {
	return &StatsService{
		reporting: NewReportingService(eventStore, redis),
	}
}

// AggregateByCampaign returns stats grouped by campaign.
func (s *StatsService) AggregateByCampaign() ([]CampaignStats, error) {
	return s.reporting.GetCampaignStats(context.Background(), ReportFilter{})
}
