package models

import (
	"errors"
	"time"
)

type BidStrategyType string

const (
	BidStrategyFixedCPM    BidStrategyType = "fixed_cpm"
	BidStrategyDynamicCPM  BidStrategyType = "dynamic_cpm"
	BidStrategyTargetCPA   BidStrategyType = "target_cpa"
	BidStrategyMaximizeROI BidStrategyType = "maximize_roi"
)

// Targeting defines restrictions for serving an ad with enhanced geo and domain support.
type Targeting struct {
	// Geo targeting
	Countries    []string `json:"countries,omitempty"`     // ISO 3166-1 alpha-2 codes
	Regions      []string `json:"regions,omitempty"`       // Region/state names
	Cities       []string `json:"cities,omitempty"`        // City names
	PostalCodes  []string `json:"postal_codes,omitempty"`  // Postal/ZIP codes
	DMAs         []int32  `json:"dmas,omitempty"`          // DMA codes (US)
	GeoRadius    *GeoRadius `json:"geo_radius,omitempty"`  // Radius targeting

	// Domain/App targeting
	SiteDomains     []string `json:"site_domains,omitempty"`      // Domain whitelist
	DomainBlacklist []string `json:"domain_blacklist,omitempty"`  // Domain blacklist
	AppBundles      []string `json:"app_bundles,omitempty"`       // App bundle whitelist
	BundleBlacklist []string `json:"bundle_blacklist,omitempty"`  // App bundle blacklist

	// Device targeting
	DeviceTypes     []int32  `json:"device_types,omitempty"`     // 1=mobile, 2=PC, etc.
	OS              []string `json:"os,omitempty"`               // ios, android, windows, etc.
	OSVersionMin    string   `json:"os_version_min,omitempty"`   // Min OS version
	OSVersionMax    string   `json:"os_version_max,omitempty"`   // Max OS version
	DeviceMakes     []string `json:"device_makes,omitempty"`     // Apple, Samsung, etc.
	DeviceModels    []string `json:"device_models,omitempty"`    // iPhone, Galaxy, etc.
	
	// Connectivity targeting
	ConnectionTypes []int32  `json:"connection_types,omitempty"` // 1=ethernet, 2=wifi, 3=cell
	Carriers        []string `json:"carriers,omitempty"`         // Carrier names

	// Content targeting
	CatWhitelist []string `json:"cat_whitelist,omitempty"` // IAB category whitelist
	CatBlacklist []string `json:"cat_blacklist,omitempty"` // IAB category blacklist
	Languages    []string `json:"languages,omitempty"`     // Content/device languages

	// Size targeting
	MinBannerW int32 `json:"min_banner_w,omitempty"`
	MinBannerH int32 `json:"min_banner_h,omitempty"`

	// Time targeting
	DayParting *DayParting `json:"day_parting,omitempty"`

	// Audience targeting
	AudienceIDs     []string `json:"audience_ids,omitempty"`     // First-party audience IDs
	AudienceExclude []string `json:"audience_exclude,omitempty"` // Audiences to exclude

	// Publisher targeting
	PublisherIDs     []string `json:"publisher_ids,omitempty"`
	PublisherExclude []string `json:"publisher_exclude,omitempty"`
}

// GeoRadius defines radius-based geo targeting.
type GeoRadius struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RadiusKm  float64 `json:"radius_km"`
}

// DayParting defines time-of-day targeting.
type DayParting struct {
	Timezone string         `json:"timezone"` // e.g., "America/New_York"
	Schedule []DaySchedule  `json:"schedule"`
}

// DaySchedule defines hours for a specific day.
type DaySchedule struct {
	Day       int   `json:"day"`        // 0=Sunday, 1=Monday, etc.
	StartHour int   `json:"start_hour"` // 0-23
	EndHour   int   `json:"end_hour"`   // 0-23
}

type BidStrategy struct {
	Type        BidStrategyType `json:"type"`
	FixedCPM    float64         `json:"fixed_cpm,omitempty"`
	MinCPM      float64         `json:"min_cpm,omitempty"`      // Floor for dynamic bidding
	MaxCPM      float64         `json:"max_cpm,omitempty"`      // Ceiling for dynamic bidding
	TargetCPA   float64         `json:"target_cpa,omitempty"`   // Target cost per action
	TargetROAS  float64         `json:"target_roas,omitempty"`  // Target return on ad spend
	BidShading  float64         `json:"bid_shading,omitempty"`  // Bid reduction factor (0-1)
}

// PacingConfig defines spend and frequency caps for a line item.
type PacingConfig struct {
	DailyBudget          float64   `json:"daily_budget"`
	TotalBudget          float64   `json:"total_budget,omitempty"`
	StartAt              time.Time `json:"start_at"`
	EndAt                time.Time `json:"end_at,omitempty"`
	
	// Frequency caps
	FreqCapPerUserPerDay   int32 `json:"freq_cap_per_user_per_day"`
	FreqCapPerUserPerHour  int32 `json:"freq_cap_per_user_per_hour,omitempty"`
	FreqCapPerUserLifetime int32 `json:"freq_cap_per_user_lifetime,omitempty"`
	
	// Advanced pacing
	PacingType          PacingType `json:"pacing_type,omitempty"`           // even, accelerated, front-loaded
	QPSLimitPerSource   int32      `json:"qps_limit_per_source,omitempty"`
	HourlyBudgetCap     float64    `json:"hourly_budget_cap,omitempty"`     // Max spend per hour
	SpendVelocity       float64    `json:"spend_velocity,omitempty"`        // Target spend rate
}

type PacingType string

const (
	PacingTypeEven        PacingType = "even"         // Spread evenly
	PacingTypeAccelerated PacingType = "accelerated"  // Spend as fast as possible
	PacingTypeFrontLoaded PacingType = "front_loaded" // Spend more early
)

// Creative represents a creative variant with enhanced fields.
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
	Format   string `json:"format,omitempty"`   // banner, video, native, audio
	VideoURL string `json:"video_url,omitempty"`
	VASTTag  string `json:"vast_tag,omitempty"`

	// Native fields
	NativeAssets *NativeAssets `json:"native_assets,omitempty"`

	// Audit status
	AuditStatus string `json:"audit_status,omitempty"` // pending, approved, rejected
	
	// Tracking
	ImpressionTrackers []string `json:"impression_trackers,omitempty"`
	ClickTrackers      []string `json:"click_trackers,omitempty"`

	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// NativeAssets holds native ad creative assets.
type NativeAssets struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"icon_url,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	CTAText     string `json:"cta_text,omitempty"`
	Rating      float64 `json:"rating,omitempty"`
	Likes       int32  `json:"likes,omitempty"`
	Downloads   int32  `json:"downloads,omitempty"`
	Price       string `json:"price,omitempty"`
	SalePrice   string `json:"sale_price,omitempty"`
}

// LineItem groups creatives under a single targeting and bidding strategy.
type LineItem struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	CampaignID  string      `json:"campaign_id"`
	Targeting   Targeting   `json:"targeting"`
	BidStrategy BidStrategy `json:"bid_strategy"`
	Pacing      PacingConfig `json:"pacing"`
	Creatives   []Creative  `json:"creatives"`
	IsActive    bool        `json:"is_active"`
	Priority    int32       `json:"priority"`
	
	// Delivery optimization
	OptimizationGoal OptimizationGoal `json:"optimization_goal,omitempty"`
	
	// Attribution
	AttributionWindow int32  `json:"attribution_window,omitempty"` // Days
	AttributionModel  string `json:"attribution_model,omitempty"`  // last_click, linear, time_decay
	
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type OptimizationGoal string

const (
	OptimizeClicks      OptimizationGoal = "clicks"
	OptimizeConversions OptimizationGoal = "conversions"
	OptimizeImpressions OptimizationGoal = "impressions"
	OptimizeVideoViews  OptimizationGoal = "video_views"
)

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
	ObjectiveBrandAwareness   CampaignObjective = "brand_awareness"
	ObjectiveTraffic          CampaignObjective = "traffic"
	ObjectiveConversions      CampaignObjective = "conversions"
	ObjectiveAppInstalls      CampaignObjective = "app_installs"
	ObjectiveVideoViews       CampaignObjective = "video_views"
	ObjectiveLeadGeneration   CampaignObjective = "lead_generation"
)

type Campaign struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	AdvertiserID string            `json:"advertiser_id"`
	Status       CampaignStatus    `json:"status"`
	Objective    CampaignObjective `json:"objective,omitempty"`
	
	// Budget
	TotalBudget  float64   `json:"total_budget,omitempty"`
	DailyBudget  float64   `json:"daily_budget,omitempty"`
	StartDate    time.Time `json:"start_date,omitempty"`
	EndDate      time.Time `json:"end_date,omitempty"`
	
	LineItems    []LineItem `json:"line_items"`
	
	// Reporting
	ConversionPixels []string `json:"conversion_pixels,omitempty"`
	
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
