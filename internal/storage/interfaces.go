package storage

import (
	"context"
	"time"

	"github.com/radiusdt/vector-dsp/internal/models"
)

// =============================================
// CAMPAIGN REPOSITORY
// =============================================

// CampaignRepo defines operations for campaign storage.
type CampaignRepo interface {
	// Basic CRUD
	ListAll(ctx context.Context) ([]*models.Campaign, error)
	GetByID(ctx context.Context, id string) (*models.Campaign, error)
	Upsert(ctx context.Context, c *models.Campaign) error
	Delete(ctx context.Context, id string) error

	// Queries
	GetByAdvertiser(ctx context.Context, advertiserID string) ([]*models.Campaign, error)
	GetActive(ctx context.Context) ([]*models.Campaign, error)
	GetByStatus(ctx context.Context, status string) ([]*models.Campaign, error)
}

// =============================================
// ADVERTISER REPOSITORY
// =============================================

// AdvertiserRepo defines operations for advertiser storage.
type AdvertiserRepo interface {
	ListAll(ctx context.Context) ([]*models.Advertiser, error)
	GetByID(ctx context.Context, id string) (*models.Advertiser, error)
	Upsert(ctx context.Context, a *models.Advertiser) error
	Delete(ctx context.Context, id string) error
	UpdateBalance(ctx context.Context, id string, delta float64) error
}

// =============================================
// SOURCE REPOSITORY
// =============================================

// SourceRepo defines operations for traffic source storage.
type SourceRepo interface {
	// S2S Sources
	ListS2SSources(ctx context.Context) ([]*models.S2SSource, error)
	GetS2SSource(ctx context.Context, id string) (*models.S2SSource, error)
	GetS2SSourceByName(ctx context.Context, internalName string) (*models.S2SSource, error)
	UpsertS2SSource(ctx context.Context, src *models.S2SSource) error
	DeleteS2SSource(ctx context.Context, id string) error

	// RTB Sources
	ListRTBSources(ctx context.Context) ([]*models.RTBSource, error)
	GetRTBSource(ctx context.Context, id string) (*models.RTBSource, error)
	UpsertRTBSource(ctx context.Context, src *models.RTBSource) error
	DeleteRTBSource(ctx context.Context, id string) error

	// Campaign-Source Links
	GetCampaignSources(ctx context.Context, campaignID string) ([]*models.CampaignSource, error)
	GetCampaignsForSource(ctx context.Context, sourceType, sourceID string) ([]*models.Campaign, error)
	UpsertCampaignSource(ctx context.Context, link *models.CampaignSource) error
	DeleteCampaignSource(ctx context.Context, id string) error
}

// =============================================
// EVENT STORE
// =============================================

// EventStore defines operations for event storage (clicks, impressions, conversions).
type EventStore interface {
	// Clicks
	SaveClick(ctx context.Context, click *models.Click) error
	GetClick(ctx context.Context, id string) (*models.Click, error)
	GetClicksByDevice(ctx context.Context, deviceIFA string, since time.Time) ([]*models.Click, error)

	// Impressions
	SaveImpression(ctx context.Context, imp *models.Impression) error
	GetImpression(ctx context.Context, id string) (*models.Impression, error)

	// Conversions
	SaveConversion(ctx context.Context, conv *models.Conversion) error
	GetConversion(ctx context.Context, id string) (*models.Conversion, error)
	GetConversionsByClick(ctx context.Context, clickID string) ([]*models.Conversion, error)

	// Wins
	SaveWin(ctx context.Context, win *models.Win) error

	// Aggregations
	GetClickCount(ctx context.Context, campaignID string, since time.Time) (int64, error)
	GetImpressionCount(ctx context.Context, campaignID string, since time.Time) (int64, error)
	GetConversionCount(ctx context.Context, campaignID string, event string, since time.Time) (int64, error)
}

// =============================================
// AD GROUP REPOSITORY
// =============================================

// AdGroupRepo defines operations for ad group storage.
type AdGroupRepo interface {
	ListAll(ctx context.Context) ([]*models.AdGroup, error)
	ListByCampaign(ctx context.Context, campaignID string) ([]*models.AdGroup, error)
	GetByID(ctx context.Context, id string) (*models.AdGroup, error)
	Upsert(ctx context.Context, ag *models.AdGroup) error
	Delete(ctx context.Context, id string) error
}

// =============================================
// CREATIVE REPOSITORY
// =============================================

// CreativeRepo defines operations for creative storage.
type CreativeRepo interface {
	ListAll(ctx context.Context) ([]*models.Creative, error)
	ListByAdvertiser(ctx context.Context, advertiserID string) ([]*models.Creative, error)
	GetByID(ctx context.Context, id string) (*models.Creative, error)
	Upsert(ctx context.Context, cr *models.Creative) error
	Delete(ctx context.Context, id string) error
	UpdateStatus(ctx context.Context, id string, status string) error
}

// =============================================
// STATS REPOSITORY
// =============================================

// StatsRepo defines operations for aggregated statistics.
type StatsRepo interface {
	// Daily stats
	GetDailyStats(ctx context.Context, filter StatsFilter) ([]*DailyStats, error)
	UpsertDailyStats(ctx context.Context, stats *DailyStats) error

	// Campaign stats
	GetCampaignStats(ctx context.Context, campaignID string, startDate, endDate time.Time) (*CampaignStatsAgg, error)

	// Source stats
	GetSourceStats(ctx context.Context, sourceType, sourceID string, startDate, endDate time.Time) (*SourceStatsAgg, error)
}

// StatsFilter for querying stats.
type StatsFilter struct {
	CampaignID  string
	SourceType  string
	SourceID    string
	Country     string
	StartDate   time.Time
	EndDate     time.Time
	GroupBy     []string // date, campaign_id, source_id, country
}

// DailyStats represents aggregated daily statistics.
type DailyStats struct {
	Date        time.Time
	CampaignID  string
	LineItemID  string
	CreativeID  string
	SourceType  string
	SourceID    string
	GeoCountry  string
	DeviceOS    string
	Impressions int64
	Clicks      int64
	Installs    int64
	Events      int64
	Spend       float64
	Revenue     float64
	Payout      float64
}

// CampaignStatsAgg represents aggregated campaign statistics.
type CampaignStatsAgg struct {
	CampaignID  string
	Impressions int64
	Clicks      int64
	Installs    int64
	Spend       float64
	Revenue     float64
	CTR         float64
	CVR         float64
	CPI         float64
	ROAS        float64
}

// SourceStatsAgg represents aggregated source statistics.
type SourceStatsAgg struct {
	SourceType  string
	SourceID    string
	Clicks      int64
	Conversions int64
	Payout      float64
	CVR         float64
}
