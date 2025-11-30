package models

import (
    "errors"
    "time"
)

type BidStrategyType string

const (
    BidStrategyFixedCPM BidStrategyType = "fixed_cpm"
)

// Targeting defines restrictions for serving an ad.  Many of these fields
// mirror options described in the Ibiza DSP frontend, such as country,
// bundle/domain, device type, OS and category whitelists/blacklists.  Min
// banner size ensures creatives match the minimum requirements.
type Targeting struct {
    Countries    []string `json:"countries,omitempty"`
    AppBundles   []string `json:"app_bundles,omitempty"`
    SiteDomains  []string `json:"site_domains,omitempty"`
    DeviceTypes  []int32  `json:"device_types,omitempty"`
    OS           []string `json:"os,omitempty"`
    CatWhitelist []string `json:"cat_whitelist,omitempty"`
    CatBlacklist []string `json:"cat_blacklist,omitempty"`
    MinBannerW   int32    `json:"min_banner_w,omitempty"`
    MinBannerH   int32    `json:"min_banner_h,omitempty"`
}

type BidStrategy struct {
    Type     BidStrategyType `json:"type"`
    FixedCPM float64         `json:"fixed_cpm,omitempty"`
}

// PacingConfig defines spend and frequency caps for a line item.
type PacingConfig struct {
    DailyBudget          float64   `json:"daily_budget"`
    TotalBudget          float64   `json:"total_budget,omitempty"`
    StartAt              time.Time `json:"start_at"`
    EndAt                time.Time `json:"end_at,omitempty"`
    FreqCapPerUserPerDay int32     `json:"freq_cap_per_user_per_day"`
    QPSLimitPerSource    int32     `json:"qps_limit_per_source,omitempty"`
}

// Creative represents a creative variant.  It includes the template markup,
// dimensions, approved domains and the click-through URL.
type Creative struct {
    ID          string   `json:"id"`
    // AdvertiserID associates a creative with the advertiser that owns
    // it.  This is optional for backward compatibility but is useful
    // when storing creatives in a global repository.
    AdvertiserID string   `json:"advertiser_id,omitempty"`
    AdmTemplate string   `json:"adm_template"`
    W           int32    `json:"w"`
    H           int32    `json:"h"`
    ADomain     []string `json:"adomain"`
    ClickURL    string   `json:"click_url"`

    // Format defines the creative type (e.g. "banner", "video").  If empty,
    // it defaults to "banner".  Creatives with format "video" may
    // include a VideoURL or VASTTag specifying the creative resource.
    Format string `json:"format,omitempty"`
    // VideoURL points to a hosted video asset.  This should be an
    // absolute URL accessible to the SSP.  Only used when Format is
    // "video".
    VideoURL string `json:"video_url,omitempty"`
    // VASTTag contains an inline VAST XML or a URL to a VAST tag.  When
    // provided, this will be returned in the adm field for video
    // responses.  Only used when Format is "video".
    VASTTag string `json:"vast_tag,omitempty"`

    // CreatedAt and UpdatedAt store timestamps when the creative was
    // initially created and last modified.  These fields mirror those
    // present on other top-level entities and are populated by the
    // CreativeService.
    CreatedAt time.Time `json:"created_at,omitempty"`
    UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// LineItem groups creatives under a single targeting and bidding strategy.
type LineItem struct {
    ID         string       `json:"id"`
    Name       string       `json:"name"`
    CampaignID string       `json:"campaign_id"`
    Targeting  Targeting    `json:"targeting"`
    BidStrategy BidStrategy `json:"bid_strategy"`
    Pacing     PacingConfig `json:"pacing"`
    Creatives  []Creative   `json:"creatives"`
    IsActive   bool         `json:"is_active"`
    Priority   int32        `json:"priority"`
}

type CampaignStatus string

const (
    CampaignStatusActive CampaignStatus = "active"
    CampaignStatusPaused CampaignStatus = "paused"
    CampaignStatusEnded  CampaignStatus = "ended"
)

type Campaign struct {
    ID           string         `json:"id"`
    Name         string         `json:"name"`
    AdvertiserID string         `json:"advertiser_id"`
    Status       CampaignStatus `json:"status"`
    LineItems    []LineItem     `json:"line_items"`
    CreatedAt    time.Time      `json:"created_at"`
    UpdatedAt    time.Time      `json:"updated_at"`
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