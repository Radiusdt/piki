package tracking

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TrackingService handles click and view tracking with MMP URL generation
type TrackingService struct {
	geoDetect    GeoDetector
	deviceDetect DeviceDetector
	clickStore   ClickStore
	impStore     ImpressionStore
	httpClient   *http.Client
	logger       *zap.Logger
	baseURL      string
}

// GeoDetector interface for geo detection
type GeoDetector interface {
	Detect(ip string) *GeoInfo
}

// GeoInfo holds geo detection results
type GeoInfo struct {
	Country string
	Region  string
	City    string
}

// DeviceDetector interface for device detection
type DeviceDetector interface {
	Parse(userAgent string) *DeviceInfo
}

// DeviceInfo holds device detection results
type DeviceInfo struct {
	Type    string // phone, tablet, desktop
	OS      string // android, ios, windows
	OSV     string // version
	Make    string // Apple, Samsung
	Model   string
	Browser string
}

// ClickStore interface for storing clicks
type ClickStore interface {
	SaveClick(ctx context.Context, click *Click) error
	GetClick(ctx context.Context, id string) (*Click, error)
}

// ImpressionStore interface for storing impressions
type ImpressionStore interface {
	SaveImpression(ctx context.Context, imp *Impression) error
}

// Click represents a click event
type Click struct {
	ID           string
	Timestamp    time.Time
	CampaignID   string
	CreativeID   string
	LineItemID   string
	SourceType   string
	SourceID     string
	SourceName   string
	DeviceIFA    string
	IP           string
	UserAgent    string
	GeoCountry   string
	GeoRegion    string
	GeoCity      string
	DeviceType   string
	DeviceOS     string
	DeviceOSV    string
	DeviceMake   string
	DeviceModel  string
	Sub1         string
	Sub2         string
	Sub3         string
	Sub4         string
	Sub5         string
	BidRequestID string
	ImpressionID string
	TargetURL    string
	Params       map[string]string
}

// Impression represents an impression/view event
type Impression struct {
	ID           string
	Timestamp    time.Time
	CampaignID   string
	CreativeID   string
	LineItemID   string
	SourceType   string
	SourceID     string
	SourceName   string
	DeviceIFA    string
	IP           string
	GeoCountry   string
	DeviceOS     string
	BidRequestID string
	WinPrice     float64
}

// Campaign holds campaign info needed for tracking
type Campaign struct {
	ID            string
	Name          string
	MMPType       string
	MMPClickURL   string
	MMPViewURL    string
	MMPMacros     map[string]string
}

// NewTrackingService creates a new tracking service
func NewTrackingService(
	geoDetect GeoDetector,
	deviceDetect DeviceDetector,
	clickStore ClickStore,
	impStore ImpressionStore,
	baseURL string,
	logger *zap.Logger,
) *TrackingService {
	return &TrackingService{
		geoDetect:    geoDetect,
		deviceDetect: deviceDetect,
		clickStore:   clickStore,
		impStore:     impStore,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger:  logger,
		baseURL: baseURL,
	}
}

// ClickParams holds parameters for click registration
type ClickParams struct {
	CampaignID   string
	CreativeID   string
	LineItemID   string
	SourceType   string
	SourceID     string
	SourceName   string
	GAID         string
	IDFA         string
	IP           string
	UserAgent    string
	Sub1         string
	Sub2         string
	Sub3         string
	Sub4         string
	Sub5         string
	BidRequestID string
	ImpressionID string
	ExtraParams  map[string]string
}

// ClickResult holds the result of click registration
type ClickResult struct {
	ClickID     string
	RedirectURL string
}

// RegisterClick handles click tracking and returns MMP redirect URL
func (s *TrackingService) RegisterClick(ctx context.Context, campaign *Campaign, params *ClickParams) (*ClickResult, error) {
	// Generate unique click ID
	clickID := uuid.New().String()

	// Detect geo
	var geo *GeoInfo
	if s.geoDetect != nil && params.IP != "" {
		geo = s.geoDetect.Detect(params.IP)
	}
	if geo == nil {
		geo = &GeoInfo{}
	}

	// Detect device
	var device *DeviceInfo
	if s.deviceDetect != nil && params.UserAgent != "" {
		device = s.deviceDetect.Parse(params.UserAgent)
	}
	if device == nil {
		device = &DeviceInfo{}
	}

	// Get device IFA
	deviceIFA := params.GAID
	if deviceIFA == "" {
		deviceIFA = params.IDFA
	}

	// Create click record
	click := &Click{
		ID:           clickID,
		Timestamp:    time.Now().UTC(),
		CampaignID:   params.CampaignID,
		CreativeID:   params.CreativeID,
		LineItemID:   params.LineItemID,
		SourceType:   params.SourceType,
		SourceID:     params.SourceID,
		SourceName:   params.SourceName,
		DeviceIFA:    deviceIFA,
		IP:           params.IP,
		UserAgent:    params.UserAgent,
		GeoCountry:   geo.Country,
		GeoRegion:    geo.Region,
		GeoCity:      geo.City,
		DeviceType:   device.Type,
		DeviceOS:     device.OS,
		DeviceOSV:    device.OSV,
		DeviceMake:   device.Make,
		DeviceModel:  device.Model,
		Sub1:         params.Sub1,
		Sub2:         params.Sub2,
		Sub3:         params.Sub3,
		Sub4:         params.Sub4,
		Sub5:         params.Sub5,
		BidRequestID: params.BidRequestID,
		ImpressionID: params.ImpressionID,
		Params:       params.ExtraParams,
	}

	// Store click
	if s.clickStore != nil {
		if err := s.clickStore.SaveClick(ctx, click); err != nil {
			s.logger.Error("failed to save click", zap.Error(err), zap.String("click_id", clickID))
			// Continue anyway - don't block redirect
		}
	}

	// Build MMP Click URL with macros replaced
	redirectURL := s.buildMMPClickURL(campaign, click)
	click.TargetURL = redirectURL

	s.logger.Info("click registered",
		zap.String("click_id", clickID),
		zap.String("campaign_id", params.CampaignID),
		zap.String("source_id", params.SourceID),
		zap.String("redirect_url", redirectURL),
	)

	return &ClickResult{
		ClickID:     clickID,
		RedirectURL: redirectURL,
	}, nil
}

// buildMMPClickURL builds the MMP click URL with macros replaced
func (s *TrackingService) buildMMPClickURL(campaign *Campaign, click *Click) string {
	if campaign.MMPClickURL == "" {
		return ""
	}

	urlStr := campaign.MMPClickURL

	// Standard macro replacements
	replacements := map[string]string{
		"{click_id}":       click.ID,
		"{clickid}":        click.ID,
		"{campaign_id}":    click.CampaignID,
		"{campaign_name}":  campaign.Name,
		"{campaign}":       campaign.Name,
		"{creative_id}":    click.CreativeID,
		"{source_id}":      click.SourceID,
		"{source_name}":    click.SourceName,
		"{publisher_id}":   click.SourceID,
		"{gaid}":           click.DeviceIFA,
		"{advertising_id}": click.DeviceIFA,
		"{idfa}":           click.DeviceIFA,
		"{ip}":             click.IP,
		"{user_agent}":     url.QueryEscape(click.UserAgent),
		"{ua}":             url.QueryEscape(click.UserAgent),
		"{country}":        click.GeoCountry,
		"{geo_country}":    click.GeoCountry,
		"{city}":           click.GeoCity,
		"{geo_city}":       click.GeoCity,
		"{region}":         click.GeoRegion,
		"{geo_region}":     click.GeoRegion,
		"{device_os}":      click.DeviceOS,
		"{os}":             click.DeviceOS,
		"{device_type}":    click.DeviceType,
		"{device_make}":    click.DeviceMake,
		"{device_model}":   click.DeviceModel,
		"{sub1}":           click.Sub1,
		"{sub2}":           click.Sub2,
		"{sub3}":           click.Sub3,
		"{sub4}":           click.Sub4,
		"{sub5}":           click.Sub5,
		"{timestamp}":      fmt.Sprintf("%d", time.Now().Unix()),
		"{ts}":             fmt.Sprintf("%d", time.Now().Unix()),
	}

	// Apply custom macros mapping from campaign
	if campaign.MMPMacros != nil {
		for ourMacro, theirMacro := range campaign.MMPMacros {
			if val, ok := replacements["{"+ourMacro+"}"]; ok {
				urlStr = strings.ReplaceAll(urlStr, "{"+theirMacro+"}", val)
			}
		}
	}

	// Apply standard replacements
	for macro, value := range replacements {
		urlStr = strings.ReplaceAll(urlStr, macro, value)
	}

	return urlStr
}

// ViewParams holds parameters for view/impression registration
type ViewParams struct {
	CampaignID   string
	CreativeID   string
	LineItemID   string
	SourceType   string
	SourceID     string
	SourceName   string
	GAID         string
	IDFA         string
	IP           string
	BidRequestID string
	WinPrice     float64
}

// RegisterView handles view/impression tracking and calls MMP view URL
func (s *TrackingService) RegisterView(ctx context.Context, campaign *Campaign, params *ViewParams) (string, error) {
	// Generate unique impression ID
	impressionID := uuid.New().String()

	// Detect geo
	var geo *GeoInfo
	if s.geoDetect != nil && params.IP != "" {
		geo = s.geoDetect.Detect(params.IP)
	}
	if geo == nil {
		geo = &GeoInfo{}
	}

	// Get device IFA
	deviceIFA := params.GAID
	if deviceIFA == "" {
		deviceIFA = params.IDFA
	}

	// Create impression record
	imp := &Impression{
		ID:           impressionID,
		Timestamp:    time.Now().UTC(),
		CampaignID:   params.CampaignID,
		CreativeID:   params.CreativeID,
		LineItemID:   params.LineItemID,
		SourceType:   params.SourceType,
		SourceID:     params.SourceID,
		SourceName:   params.SourceName,
		DeviceIFA:    deviceIFA,
		IP:           params.IP,
		GeoCountry:   geo.Country,
		BidRequestID: params.BidRequestID,
		WinPrice:     params.WinPrice,
	}

	// Store impression
	if s.impStore != nil {
		if err := s.impStore.SaveImpression(ctx, imp); err != nil {
			s.logger.Error("failed to save impression", zap.Error(err), zap.String("impression_id", impressionID))
		}
	}

	// Call MMP View URL asynchronously (don't block response)
	if campaign.MMPViewURL != "" {
		go s.callMMPViewURL(campaign, imp)
	}

	s.logger.Debug("view registered",
		zap.String("impression_id", impressionID),
		zap.String("campaign_id", params.CampaignID),
	)

	return impressionID, nil
}

// callMMPViewURL calls the MMP view URL asynchronously
func (s *TrackingService) callMMPViewURL(campaign *Campaign, imp *Impression) {
	mmpViewURL := s.buildMMPViewURL(campaign, imp)
	if mmpViewURL == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", mmpViewURL, nil)
	if err != nil {
		s.logger.Error("failed to create MMP view request", zap.Error(err))
		return
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Error("MMP view call failed",
			zap.String("campaign_id", campaign.ID),
			zap.Error(err),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn("MMP view non-200",
			zap.String("campaign_id", campaign.ID),
			zap.Int("status", resp.StatusCode),
		)
	}
}

// buildMMPViewURL builds the MMP view URL with macros replaced
func (s *TrackingService) buildMMPViewURL(campaign *Campaign, imp *Impression) string {
	if campaign.MMPViewURL == "" {
		return ""
	}

	urlStr := campaign.MMPViewURL

	replacements := map[string]string{
		"{impression_id}":  imp.ID,
		"{clickid}":        imp.ID, // AppsFlyer uses clickid for view too
		"{campaign_id}":    imp.CampaignID,
		"{campaign_name}":  campaign.Name,
		"{campaign}":       campaign.Name,
		"{creative_id}":    imp.CreativeID,
		"{source_id}":      imp.SourceID,
		"{gaid}":           imp.DeviceIFA,
		"{advertising_id}": imp.DeviceIFA,
		"{idfa}":           imp.DeviceIFA,
		"{ip}":             imp.IP,
		"{timestamp}":      fmt.Sprintf("%d", time.Now().Unix()),
	}

	for macro, value := range replacements {
		urlStr = strings.ReplaceAll(urlStr, macro, value)
	}

	return urlStr
}

// BuildOurClickURL builds our tracking click URL for use in ad markup
func (s *TrackingService) BuildOurClickURL(campaignID, creativeID, lineItemID, sourceID, sourceType string) string {
	return fmt.Sprintf("%s/track/click?cid=%s&cr=%s&li=%s&src=%s&st=%s&gaid=${ADVERTISING_ID}",
		s.baseURL, campaignID, creativeID, lineItemID, sourceID, sourceType)
}

// BuildOurViewURL builds our tracking view URL for use in ad markup
func (s *TrackingService) BuildOurViewURL(campaignID, creativeID, lineItemID, sourceID, sourceType, impressionID string) string {
	return fmt.Sprintf("%s/track/view?cid=%s&cr=%s&li=%s&src=%s&st=%s&imp=%s&gaid=${ADVERTISING_ID}",
		s.baseURL, campaignID, creativeID, lineItemID, sourceID, sourceType, impressionID)
}

// TransparentPixel is a 1x1 transparent GIF
var TransparentPixel = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
	0x01, 0x00, 0x80, 0x00, 0x00, 0xFF, 0xFF, 0xFF,
	0x00, 0x00, 0x00, 0x21, 0xF9, 0x04, 0x01, 0x00,
	0x00, 0x00, 0x00, 0x2C, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
	0x01, 0x00, 0x3B,
}
