package models

import (
	"errors"
	"time"
)

// ===========================================
// S2S SOURCE (Direct Partners)
// ===========================================

type S2SSource struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	InternalName string    `json:"internal_name"` // For URL: /s2s/{internal_name}/...
	
	// Authentication
	APIToken   string   `json:"api_token,omitempty"`
	AllowedIPs []string `json:"allowed_ips,omitempty"`
	
	// Request configuration
	RequestMethod string `json:"request_method,omitempty"` // GET, POST
	
	// Postback configuration (to send back to source)
	PostbackURL    string            `json:"postback_url,omitempty"`
	PostbackMethod string            `json:"postback_method,omitempty"` // GET, POST
	PostbackEvents []string          `json:"postback_events,omitempty"` // ["install", "purchase"]
	MacrosMapping  map[string]string `json:"macros_mapping,omitempty"`  // {"click_id": "clickid", "sub1": "aff_sub"}
	
	// Default payout
	DefaultPayout     float64 `json:"default_payout,omitempty"`
	DefaultPayoutType string  `json:"default_payout_type,omitempty"` // fixed, percent
	
	// Limits
	DailyBudgetCap float64 `json:"daily_budget_cap,omitempty"`
	DailyClickCap  int64   `json:"daily_click_cap,omitempty"`
	
	Status    string    `json:"status"` // active, paused, blocked
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *S2SSource) Validate() error {
	if s.ID == "" {
		return errors.New("id is required")
	}
	if s.Name == "" {
		return errors.New("name is required")
	}
	if s.InternalName == "" {
		return errors.New("internal_name is required")
	}
	return nil
}

// ===========================================
// RTB SOURCE (OpenRTB SSPs)
// ===========================================

type RTBSource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	
	// Our endpoint that SSP calls
	OurEndpoint string `json:"our_endpoint"` // Our URL for their requests
	
	// Protocol
	ProtocolVersion  string   `json:"protocol_version"` // "2.5", "2.6"
	SupportedFormats []string `json:"supported_formats,omitempty"` // ["banner", "native", "video"]
	SupportedSizes   [][]int  `json:"supported_sizes,omitempty"`   // [[320,50], [300,250]]
	
	// Bidding configuration
	BidAdjustment float64 `json:"bid_adjustment,omitempty"` // Multiplier for bids
	MaxQPS        int32   `json:"max_qps,omitempty"`        // Max queries per second
	TimeoutMs     int32   `json:"timeout_ms,omitempty"`     // Response timeout
	
	// Win notice URL template
	WinNoticeURL string `json:"win_notice_url,omitempty"`
	
	Status    string    `json:"status"` // active, paused
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (r *RTBSource) Validate() error {
	if r.ID == "" {
		return errors.New("id is required")
	}
	if r.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

// ===========================================
// CAMPAIGN-SOURCE LINK
// ===========================================

type CampaignSource struct {
	ID         string `json:"id"`
	CampaignID string `json:"campaign_id"`
	SourceType string `json:"source_type"` // "s2s" or "rtb"
	SourceID   string `json:"source_id"`
	
	// Custom settings for this campaign-source pair
	CustomPayout     *float64 `json:"custom_payout,omitempty"`
	CustomPayoutType string   `json:"custom_payout_type,omitempty"`
	CustomBid        *float64 `json:"custom_bid,omitempty"`
	
	// Caps
	DailyCap int64 `json:"daily_cap,omitempty"`
	TotalCap int64 `json:"total_cap,omitempty"`
	
	Status    string    `json:"status"` // active, paused
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
