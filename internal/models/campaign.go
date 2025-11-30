package models

import (
	"errors"
	"time"
)

// ===========================================
// BID STRATEGY
// ===========================================

type BidStrategyType string

const (
	BidStrategyFixedCPM    BidStrategyType = "fixed_cpm"
	BidStrategyDynamicCPM  BidStrategyType = "dynamic_cpm"
	BidStrategyTargetCPI   BidStrategyType = "target_cpi"
	BidStrategyTargetCPA   BidStrategyType = "target_cpa"
	BidStrategyMaximizeROI BidStrategyType = "maximize_roi"
)

type BidStrategy struct {
	Type       BidStrategyType `json:"type"`
	FixedCPM   float64         `json:"fixed_cpm,omitempty"`
	MinCPM     float64         `json:"min_cpm,omitempty"`
	MaxCPM     float64         `json:"max_cpm,omitempty"`
	TargetCPI  float64         `json:"target_cpi,omitempty"`
	TargetCPA  float64         `json:"target_cpa,omitempty"`
	TargetROAS float64         `json:"target_roas,omitempty"`
	BidShading float64         `json:"bid_shading,omitempty"`
}

// ===========================================
// PACING
// ===========================================

type PacingType string

const (
	PacingTypeEven        PacingType = "even"
	PacingTypeAccelerated PacingType = "accelerated"
	PacingTypeFrontLoaded PacingType = "front_loaded"
)

type PacingConfig struct {
	DailyBudget            float64    `json:"daily_budget"`
	TotalBudget            float64    `json:"total_budget,omitempty"`
	StartAt                time.Time  `json:"start_at"`
	EndAt                  time.Time  `json:"end_at,omitempty"`
	FreqCapPerUserPerDay   int32      `json:"freq_cap_per_user_per_day"`
	FreqCapPerUserPerHour  int32      `json:"freq_cap_per_user_per_hour,omitempty"`
	FreqCapPerUserLifetime int32      `json:"freq_cap_per_user_lifetime,omitempty"`
	PacingType             PacingType `json:"pacing_type,omitempty"`
	QPSLimitPerSource      int32      `json:"qps_limit_per_source,omitempty"`
	HourlyBudgetCap        float64    `json:"hourly_budget_cap,omitempty"`
}

// ===========================================
// TARGETING
// ===========================================

type Targeting struct {
	// Geo targeting
	Countries   []string   `json:"countries,omitempty"`
	Regions     []string   `json:"regions,omitempty"`
	Cities      []string   `json:"cities,omitempty"`
	PostalCodes []string   `json:"postal_codes,omitempty"`
	DMAs        []int32    `json:"dmas,omitempty"`
	GeoRadius   *GeoRadius `json:"geo_radius,omitempty"`

	// Domain/App targeting
	SiteDomains     []string `json:"site_domains,omitempty"`
	DomainBlacklist []string `json:"domain_blacklist,omitempty"`
	AppBundles      []string `json:"app_bundles,omitempty"`
	BundleBlacklist []string `json:"bundle_blacklist,omitempty"`

	// Device targeting
	DeviceTypes  []int32  `json:"device_types,omitempty"`
	OS           []string `json:"os,omitempty"`
	OSVersionMin string   `json:"os_version_min,omitempty"`
	OSVersionMax string   `json:"os_version_max,omitempty"`
	DeviceMakes  []string `json:"device_makes,omitempty"`
	DeviceModels []string `json:"device_models,omitempty"`

	// Connectivity
	ConnectionTypes []int32  `json:"connection_types,omitempty"`
	Carriers        []string `json:"carriers,omitempty"`

	// Content
	CatWhitelist []string `json:"cat_whitelist,omitempty"`
	CatBlacklist []string `json:"cat_blacklist,omitempty"`
	Languages    []string `json:"languages,omitempty"`

	// Size
	MinBannerW int32 `json:"min_banner_w,omitempty"`
	MinBannerH int32 `json:"min_banner_h,omitempty"`

	// Time
	DayParting *DayParting `json:"day_parting,omitempty"`

	// Audience
	AudienceIDs     []string `json:"audience_ids,omitempty"`
	AudienceExclude []string `json:"audience_exclude,omitempty"`

	// Publisher
	PublisherIDs     []string `json:"publisher_ids,omitempty"`
	PublisherExclude []string `json:"publisher_exclude,omitempty"`
}

type GeoRadius struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RadiusKm  float64 `json:"radius_km"`
}

type DayParting struct {
	Timezone string        `json:"timezone"`
	Schedule []DaySchedule `json:"schedule"`
}

type DaySchedule struct {
	Day       int `json:"day"`
	StartHour int `json:"start_hour"`
	EndHour   int `json:"end_hour"`
}

// ===========================================
// MMP CONFIGURATION
// ===========================================

type MMPType string

const (
	MMPTypeNone     MMPType = "none"
	MMPTypeAppsFlyer MMPType = "appsflyer"
	MMPTypeAdjust   MMPType = "adjust"
	MMPTypeSingular MMPType = "singular"
	MMPTypeBranch   MMPType = "branch"
	MMPTypeKochava  MMPType = "kochava"
)

// MMPConfig holds MMP tracking URLs and configuration.
// These URLs are provided by the client when they add Vector-DSP as a source.
type MMPConfig struct {
	// MMP type
	Type MMPType `json:"type"`

	// Click URL from MMP (client provides this)
	// Example: https://app.appsflyer.com/com.app?pid=vector_dsp&c={campaign}&clickid={click_id}&...
	ClickURL string `json:"click_url"`

	// View/Impression URL from MMP (for view-through attribution)
	// Example: https://impression.appsflyer.com/com.app?pid=vector_dsp&...
	ViewURL string `json:"view_url"`

	// Custom macros mapping for this MMP
	// {"click_id": "clickid", "campaign": "c", "gaid": "advertising_id"}
	MacrosMapping map[string]string `json:"macros_mapping,omitempty"`

	// Postback configuration
	PostbackEvents []string `json:"postback_events,omitempty"` // ["install", "registration", "purchase"]
}

// ===========================================
// CREATIVE
// ===========================================

type Creative struct {
	ID           string   `json:"id"`
	AdvertiserID string   `json:"advertiser_id,omitempty"`
	Name         string   `json:"name,omitempty"`
	AdmTemplate  string   `json:"adm_template"`
	W            int32    `json:"w"`
	H            int32    `json:"h"`
	ADomain      []string `json:"adomain"`
	ClickURL     string   `json:"click_url"`

	// Creative type
	Format   string `json:"format,omitempty"` // banner, video, native, audio
	VideoURL string `json:"video_url,omitempty"`
	VASTTag  string `json:"vast_tag,omitempty"`

	// Native fields
	NativeAssets *NativeAssets `json:"native_assets,omitempty"`

	// Audit status
	AuditStatus string `json:"audit_status,omitempty"`

	// Tracking
	ImpressionTrackers []string `json:"impression_trackers,omitempty"`
	ClickTrackers      []string `json:"click_trackers,omitempty"`

	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type NativeAssets struct {
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	IconURL     string  `json:"icon_url,omitempty"`
	ImageURL    string  `json:"image_url,omitempty"`
	CTAText     string  `json:"cta_text,omitempty"`
	Rating      float64 `json:"rating,omitempty"`
	Likes       int32   `json:"likes,omitempty"`
	Downloads   int32   `json:"downloads,omitempty"`
	Price       string  `json:"price,omitempty"`
	SalePrice   string  `json:"sale_price,omitempty"`
}

// ===========================================
// LINE ITEM
// ===========================================

type OptimizationGoal string

const (
	OptimizeClicks      OptimizationGoal = "clicks"
	OptimizeConversions OptimizationGoal = "conversions"
	OptimizeImpressions OptimizationGoal = "impressions"
	OptimizeInstalls    OptimizationGoal = "installs"
	OptimizeVideoViews  OptimizationGoal = "video_views"
)

type LineItem struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	CampaignID string      `json:"campaign_id"`
	Targeting  Targeting   `json:"targeting"`
	BidStrategy BidStrategy `json:"bid_strategy"`
	Pacing     PacingConfig `json:"pacing"`
	Creatives  []Creative  `json:"creatives"`
	IsActive   bool        `json:"is_active"`
	Priority   int32       `json:"priority"`

	// Delivery optimization
	OptimizationGoal OptimizationGoal `json:"optimization_goal,omitempty"`

	// Attribution
	AttributionWindow int32  `json:"attribution_window,omitempty"`
	AttributionModel  string `json:"attribution_model,omitempty"`

	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func (li *LineItem) Validate() error {
	if li.ID == "" {
		return errors.New("line_item id is required")
	}
	if li.CampaignID == "" {
		return errors.New("line_item campaign_id is required")
	}
	if li.BidStrategy.Type == BidStrategyFixedCPM && li.BidStrategy.FixedCPM <= 0 {
		return errors.New("fixed_cpm must be > 0")
	}
	if li.Pacing.DailyBudget <= 0 {
		return errors.New("daily_budget must be > 0")
	}
	if len(li.Creatives) == 0 {
		return errors.New("at least one creative required")
	}
	return nil
}

// ===========================================
// CAMPAIGN
// ===========================================

type CampaignStatus string

const (
	CampaignStatusDraft    CampaignStatus = "draft"
	CampaignStatusActive   CampaignStatus = "active"
	CampaignStatusPaused   CampaignStatus = "paused"
	CampaignStatusEnded    CampaignStatus = "ended"
	CampaignStatusArchived CampaignStatus = "archived"
)

type CampaignObjective string

const (
	ObjectiveBrandAwareness CampaignObjective = "brand_awareness"
	ObjectiveTraffic        CampaignObjective = "traffic"
	ObjectiveConversions    CampaignObjective = "conversions"
	ObjectiveAppInstalls    CampaignObjective = "app_installs"
	ObjectiveVideoViews     CampaignObjective = "video_views"
	ObjectiveLeadGeneration CampaignObjective = "lead_generation"
)

type Campaign struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	AdvertiserID string            `json:"advertiser_id"`
	Status       CampaignStatus    `json:"status"`
	Objective    CampaignObjective `json:"objective,omitempty"`

	// App info (for mobile campaigns)
	AppBundle   string `json:"app_bundle,omitempty"`   // com.client.app
	AppName     string `json:"app_name,omitempty"`
	AppStoreURL string `json:"app_store_url,omitempty"`

	// Budget
	TotalBudget float64   `json:"total_budget,omitempty"`
	DailyBudget float64   `json:"daily_budget,omitempty"`
	StartDate   time.Time `json:"start_date,omitempty"`
	EndDate     time.Time `json:"end_date,omitempty"`

	// MMP Configuration (provided by client)
	MMP MMPConfig `json:"mmp"`

	// Line Items
	LineItems []LineItem `json:"line_items"`

	// Payout to sources
	PayoutType   string  `json:"payout_type,omitempty"`   // fixed, percent, dynamic
	PayoutAmount float64 `json:"payout_amount,omitempty"` // Amount or percentage
	PayoutEvent  string  `json:"payout_event,omitempty"`  // install, registration, purchase

	// Reporting
	ConversionPixels []string `json:"conversion_pixels,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *Campaign) Validate() error {
	if c.ID == "" {
		return errors.New("id is required")
	}
	if c.Name == "" {
		return errors.New("name is required")
	}
	if c.AdvertiserID == "" {
		return errors.New("advertiser_id is required")
	}
	for i := range c.LineItems {
		if err := c.LineItems[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// HasMMPTracking returns true if MMP tracking is configured
func (c *Campaign) HasMMPTracking() bool {
	return c.MMP.Type != MMPTypeNone && c.MMP.Type != "" && c.MMP.ClickURL != ""
}
