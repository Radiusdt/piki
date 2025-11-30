package models

import (
	"time"
)

// ===========================================
// CLICK EVENT
// ===========================================

type Click struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	
	// Campaign info
	CampaignID string `json:"campaign_id"`
	LineItemID string `json:"line_item_id"`
	CreativeID string `json:"creative_id"`
	
	// Source info
	SourceType string `json:"source_type"` // "s2s" or "rtb"
	SourceID   string `json:"source_id"`
	SourceName string `json:"source_name"`
	
	// Device info
	DeviceIFA string `json:"device_ifa,omitempty"` // GAID or IDFA
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	
	// Geo info
	GeoCountry string `json:"geo_country"`
	GeoRegion  string `json:"geo_region,omitempty"`
	GeoCity    string `json:"geo_city,omitempty"`
	
	// Device info
	DeviceType    string `json:"device_type,omitempty"`
	DeviceOS      string `json:"device_os,omitempty"`
	DeviceOSV     string `json:"device_os_version,omitempty"`
	DeviceMake    string `json:"device_make,omitempty"`
	DeviceModel   string `json:"device_model,omitempty"`
	
	// From RTB
	BidRequestID string  `json:"bid_request_id,omitempty"`
	BidPrice     float64 `json:"bid_price,omitempty"`
	WinPrice     float64 `json:"win_price,omitempty"`
	
	// Sub IDs from source
	Sub1 string `json:"sub1,omitempty"`
	Sub2 string `json:"sub2,omitempty"`
	Sub3 string `json:"sub3,omitempty"`
	Sub4 string `json:"sub4,omitempty"`
	Sub5 string `json:"sub5,omitempty"`
	
	// Target URL
	TargetURL string `json:"target_url"`
	
	// Additional params
	Params map[string]string `json:"params,omitempty"`
}

// ===========================================
// IMPRESSION (VIEW) EVENT
// ===========================================

type Impression struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	
	// Campaign info
	CampaignID string `json:"campaign_id"`
	LineItemID string `json:"line_item_id"`
	CreativeID string `json:"creative_id"`
	
	// Source info
	SourceType string `json:"source_type"`
	SourceID   string `json:"source_id"`
	SourceName string `json:"source_name"`
	
	// From RTB
	BidRequestID string  `json:"bid_request_id,omitempty"`
	BidPrice     float64 `json:"bid_price,omitempty"`
	WinPrice     float64 `json:"win_price,omitempty"`
	
	// Device info
	DeviceIFA  string `json:"device_ifa,omitempty"`
	IP         string `json:"ip"`
	GeoCountry string `json:"geo_country,omitempty"`
	DeviceOS   string `json:"device_os,omitempty"`
	DeviceType string `json:"device_type,omitempty"`
	
	// App/Site info
	AppBundle   string `json:"app_bundle,omitempty"`
	AppName     string `json:"app_name,omitempty"`
	SiteDomain  string `json:"site_domain,omitempty"`
	PublisherID string `json:"publisher_id,omitempty"`
}

// ===========================================
// CONVERSION EVENT
// ===========================================

type Conversion struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	
	// Link to click
	ClickID string `json:"click_id,omitempty"`
	
	// Source identification
	SourceType string `json:"source_type"`
	SourceID   string `json:"source_id"`
	SourceName string `json:"source_name"`
	
	// Campaign info (from click or external)
	AdvertiserID string `json:"advertiser_id,omitempty"`
	CampaignID   string `json:"campaign_id"`
	LineItemID   string `json:"line_item_id,omitempty"`
	CreativeID   string `json:"creative_id,omitempty"`
	
	// Event info
	Event         string `json:"event"`          // install, registration, purchase
	EventOriginal string `json:"event_original"` // Original event name from MMP (af_purchase)
	
	// Revenue
	Revenue         float64 `json:"revenue,omitempty"`
	RevenueCurrency string  `json:"revenue_currency,omitempty"`
	RevenueUSD      float64 `json:"revenue_usd,omitempty"`
	
	// Payout (to source)
	Payout         float64 `json:"payout,omitempty"`
	PayoutCurrency string  `json:"payout_currency,omitempty"`
	PayoutUSD      float64 `json:"payout_usd,omitempty"`
	
	// Device info
	DeviceIFA string `json:"device_ifa,omitempty"`
	
	// Attribution
	ClickTime     *time.Time `json:"click_time,omitempty"`
	InstallTime   *time.Time `json:"install_time,omitempty"`
	TimeToInstall int64      `json:"time_to_install,omitempty"` // Seconds
	
	// External IDs
	ExternalID string `json:"external_id,omitempty"` // From MMP
	
	// Additional params from postback
	Params map[string]string `json:"params,omitempty"`
}

// ===========================================
// EVENT MAPPING (MMP events to internal)
// ===========================================

type EventMapping struct {
	ID            string `json:"id"`
	SourceType    string `json:"source_type"` // "s2s", "rtb", "mmp"
	SourceID      string `json:"source_id"`
	ExternalEvent string `json:"external_event"` // af_purchase, registration_success
	InternalEvent string `json:"internal_event"` // purchase, registration
	IsConversion  bool   `json:"is_conversion"`
	IsRevenue     bool   `json:"is_revenue"`
}

// DefaultEventMappings returns standard event mappings for MMPs
var DefaultEventMappings = map[string]map[string]string{
	"appsflyer": {
		"install":                  "install",
		"af_app_install":           "install",
		"af_complete_registration": "registration",
		"af_purchase":              "purchase",
		"af_first_purchase":        "first_purchase",
		"af_subscribe":             "subscribe",
		"af_add_to_cart":           "add_to_cart",
		"af_initiated_checkout":    "checkout",
	},
	"adjust": {
		"install":      "install",
		"registration": "registration",
		"purchase":     "purchase",
		"session":      "session",
	},
	"singular": {
		"__INSTALL__":        "install",
		"__SESSION__":        "session",
		"__CUSTOM_EVENT__":   "custom",
		"registration":       "registration",
		"purchase":           "purchase",
	},
}

// MapEvent converts external event name to internal
func MapEvent(mmpType, externalEvent string) string {
	if mappings, ok := DefaultEventMappings[mmpType]; ok {
		if internal, ok := mappings[externalEvent]; ok {
			return internal
		}
	}
	// Return as-is if no mapping found
	return externalEvent
}

// ===========================================
// WIN EVENT (RTB win notification)
// ===========================================

type Win struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// IDs
	BidRequestID string `json:"bid_request_id"`
	ImpID        string `json:"imp_id"`

	// Campaign
	CampaignID string `json:"campaign_id"`
	LineItemID string `json:"line_item_id,omitempty"`
	CreativeID string `json:"creative_id,omitempty"`

	// Source
	SourceID string `json:"source_id"`

	// Pricing
	BidPrice float64 `json:"bid_price"`
	WinPrice float64 `json:"win_price"`

	// Device
	DeviceIFA  string `json:"device_ifa,omitempty"`
	GeoCountry string `json:"geo_country,omitempty"`
}

// ===========================================
// STANDARD MACROS
// ===========================================

// StandardMacros contains all supported macros for URL templates.
var StandardMacros = []string{
	"{click_id}",
	"{clickid}",
	"{impression_id}",
	"{campaign_id}",
	"{campaign_name}",
	"{campaign}",
	"{creative_id}",
	"{line_item_id}",
	"{source_id}",
	"{source_name}",
	"{publisher_id}",
	"{gaid}",
	"{advertising_id}",
	"{idfa}",
	"{device_ifa}",
	"{ip}",
	"{user_agent}",
	"{ua}",
	"{country}",
	"{geo_country}",
	"{city}",
	"{geo_city}",
	"{region}",
	"{geo_region}",
	"{device_os}",
	"{os}",
	"{device_osv}",
	"{osv}",
	"{device_type}",
	"{device_make}",
	"{device_model}",
	"{sub1}",
	"{sub2}",
	"{sub3}",
	"{sub4}",
	"{sub5}",
	"{timestamp}",
	"{ts}",
	"{event}",
	"{revenue}",
	"{currency}",
	"{payout}",
	"{conversion_id}",
}
