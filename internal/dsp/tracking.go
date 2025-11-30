package dsp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/radiusdt/vector-dsp/internal/metrics"
	"github.com/radiusdt/vector-dsp/internal/models"
	"github.com/radiusdt/vector-dsp/internal/storage"
	"github.com/radiusdt/vector-dsp/internal/targeting"
	"go.uber.org/zap"
)

// TrackingService handles click and view tracking.
type TrackingService struct {
	eventStore       storage.EventStore
	campaignRepo     storage.CampaignRepo
	targetingEngine  *targeting.TargetingEngine
	baseURL          string
	logger           *zap.Logger
	metrics          *metrics.Metrics
	httpClient       *http.Client
}

// NewTrackingService creates a new tracking service.
func NewTrackingService(
	eventStore storage.EventStore,
	campaignRepo storage.CampaignRepo,
	targetingEngine *targeting.TargetingEngine,
	baseURL string,
	logger *zap.Logger,
	m *metrics.Metrics,
) *TrackingService {
	return &TrackingService{
		eventStore:      eventStore,
		campaignRepo:    campaignRepo,
		targetingEngine: targetingEngine,
		baseURL:         baseURL,
		logger:          logger,
		metrics:         m,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		},
	}
}

// RegisterClick handles click tracking and returns MMP redirect URL.
func (s *TrackingService) RegisterClick(
	ctx context.Context,
	campaignID, creativeID, lineItemID string,
	sourceType, sourceID, impressionID string,
	gaid, idfa string,
	ip, userAgent string,
	sub1, sub2, sub3, sub4, sub5 string,
) (string, error) {
	// Generate click ID
	clickID := uuid.New().String()

	// Get campaign for MMP URL
	campaign, err := s.campaignRepo.GetByID(ctx, campaignID)
	if err != nil {
		s.logger.Error("failed to get campaign for click", zap.Error(err), zap.String("campaign_id", campaignID))
		return "", fmt.Errorf("campaign not found: %w", err)
	}

	// Detect geo from IP
	var geoCountry, geoRegion, geoCity string
	if s.targetingEngine != nil {
		geoInfo := s.targetingEngine.GetGeoInfo(ip)
		if geoInfo != nil {
			geoCountry = geoInfo.Country
			geoRegion = geoInfo.Region
			geoCity = geoInfo.City
		}
	}

	// Parse device info from User-Agent
	deviceOS, deviceType, deviceMake, deviceModel := parseUserAgent(userAgent)

	// Determine device IFA
	deviceIFA := gaid
	if deviceIFA == "" {
		deviceIFA = idfa
	}

	// Create click record
	click := &models.Click{
		ID:          clickID,
		Timestamp:   time.Now(),
		CampaignID:  campaignID,
		LineItemID:  lineItemID,
		CreativeID:  creativeID,
		SourceType:  sourceType,
		SourceID:    sourceID,
		DeviceIFA:   deviceIFA,
		IP:          ip,
		UserAgent:   userAgent,
		GeoCountry:  geoCountry,
		GeoRegion:   geoRegion,
		GeoCity:     geoCity,
		DeviceType:  deviceType,
		DeviceOS:    deviceOS,
		DeviceMake:  deviceMake,
		DeviceModel: deviceModel,
		Sub1:        sub1,
		Sub2:        sub2,
		Sub3:        sub3,
		Sub4:        sub4,
		Sub5:        sub5,
	}

	// Save click
	if err := s.eventStore.SaveClick(ctx, click); err != nil {
		s.logger.Error("failed to save click", zap.Error(err))
		return "", fmt.Errorf("failed to save click: %w", err)
	}

	s.logger.Info("click registered",
		zap.String("click_id", clickID),
		zap.String("campaign_id", campaignID),
		zap.String("source_id", sourceID),
		zap.String("device_ifa", deviceIFA),
		zap.String("geo_country", geoCountry),
	)

	// Build MMP Click URL with macro replacements
	if campaign != nil && campaign.MMP.ClickURL != "" {
		redirectURL := s.buildMMPClickURL(campaign, click, gaid, idfa)
		click.TargetURL = redirectURL
		return redirectURL, nil
	}

	// No MMP URL - return app store URL
	if campaign != nil && campaign.AppStoreURL != "" {
		return campaign.AppStoreURL, nil
	}

	return "", nil
}

// RegisterView handles view/impression tracking.
func (s *TrackingService) RegisterView(
	ctx context.Context,
	campaignID, creativeID, lineItemID string,
	sourceType, sourceID, impressionID string,
	gaid, idfa, ip string,
) {
	// Generate impression ID if not provided
	if impressionID == "" {
		impressionID = uuid.New().String()
	}

	// Detect geo
	var geoCountry string
	if s.targetingEngine != nil {
		geoInfo := s.targetingEngine.GetGeoInfo(ip)
		if geoInfo != nil {
			geoCountry = geoInfo.Country
		}
	}

	deviceIFA := gaid
	if deviceIFA == "" {
		deviceIFA = idfa
	}

	// Create impression record
	imp := &models.Impression{
		ID:         impressionID,
		Timestamp:  time.Now(),
		CampaignID: campaignID,
		LineItemID: lineItemID,
		CreativeID: creativeID,
		SourceType: sourceType,
		SourceID:   sourceID,
		DeviceIFA:  deviceIFA,
		IP:         ip,
		GeoCountry: geoCountry,
	}

	// Save impression (async, don't block response)
	go func() {
		if err := s.eventStore.SaveImpression(context.Background(), imp); err != nil {
			s.logger.Error("failed to save impression", zap.Error(err))
		}
	}()

	// Call MMP View URL asynchronously
	go func() {
		campaign, err := s.campaignRepo.GetByID(context.Background(), campaignID)
		if err != nil || campaign == nil {
			return
		}
		if campaign.MMP.ViewURL != "" {
			viewURL := s.buildMMPViewURL(campaign, impressionID, gaid, idfa, geoCountry)
			s.callMMPURL(viewURL)
		}
	}()

	s.logger.Debug("view registered",
		zap.String("impression_id", impressionID),
		zap.String("campaign_id", campaignID),
	)
}

// RegisterWin handles win notification from RTB.
func (s *TrackingService) RegisterWin(
	ctx context.Context,
	impID, campaignID, lineItemID, creativeID string,
	winPrice float64,
) {
	// This could be stored in a separate wins table for analysis
	s.logger.Info("win registered",
		zap.String("imp_id", impID),
		zap.String("campaign_id", campaignID),
		zap.Float64("win_price", winPrice),
	)
}

// buildMMPClickURL replaces macros in MMP Click URL.
func (s *TrackingService) buildMMPClickURL(campaign *models.Campaign, click *models.Click, gaid, idfa string) string {
	if campaign.MMP.ClickURL == "" {
		return ""
	}

	result := campaign.MMP.ClickURL

	// Standard macro replacements
	replacements := map[string]string{
		"{click_id}":       click.ID,
		"{clickid}":        click.ID,
		"{campaign_id}":    campaign.ID,
		"{campaign_name}":  url.QueryEscape(campaign.Name),
		"{campaign}":       url.QueryEscape(campaign.Name),
		"{creative_id}":    click.CreativeID,
		"{source_id}":      click.SourceID,
		"{source_name}":    click.SourceID, // Could be resolved to actual name
		"{publisher_id}":   click.SourceID,
		"{gaid}":           gaid,
		"{advertising_id}": gaid,
		"{idfa}":           idfa,
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
		"{timestamp}":      fmt.Sprintf("%d", click.Timestamp.Unix()),
		"{ts}":             fmt.Sprintf("%d", click.Timestamp.Unix()),
	}

	for macro, value := range replacements {
		result = strings.ReplaceAll(result, macro, value)
	}

	// Apply custom macros mapping from campaign
	for ourMacro, theirMacro := range campaign.MMP.MacrosMapping {
		if value, ok := replacements["{"+ourMacro+"}"]; ok {
			result = strings.ReplaceAll(result, "{"+theirMacro+"}", value)
		}
	}

	return result
}

// buildMMPViewURL replaces macros in MMP View URL.
func (s *TrackingService) buildMMPViewURL(campaign *models.Campaign, impressionID, gaid, idfa, country string) string {
	if campaign.MMP.ViewURL == "" {
		return ""
	}

	result := campaign.MMP.ViewURL

	replacements := map[string]string{
		"{impression_id}":  impressionID,
		"{clickid}":        impressionID, // Some MMPs use clickid for view-through
		"{campaign_id}":    campaign.ID,
		"{campaign_name}":  url.QueryEscape(campaign.Name),
		"{campaign}":       url.QueryEscape(campaign.Name),
		"{gaid}":           gaid,
		"{advertising_id}": gaid,
		"{idfa}":           idfa,
		"{country}":        country,
		"{geo_country}":    country,
		"{timestamp}":      fmt.Sprintf("%d", time.Now().Unix()),
		"{ts}":             fmt.Sprintf("%d", time.Now().Unix()),
	}

	for macro, value := range replacements {
		result = strings.ReplaceAll(result, macro, value)
	}

	return result
}

// callMMPURL makes async HTTP call to MMP URL.
func (s *TrackingService) callMMPURL(mmpURL string) {
	if mmpURL == "" {
		return
	}

	req, err := http.NewRequest(http.MethodGet, mmpURL, nil)
	if err != nil {
		s.logger.Error("failed to create MMP request", zap.Error(err))
		return
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		s.logger.Warn("MMP call failed", zap.Error(err), zap.String("url", mmpURL))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		s.logger.Warn("MMP call returned error", zap.Int("status", resp.StatusCode), zap.String("url", mmpURL))
	}
}

// BuildOurClickURL builds tracking click URL for ad responses.
func (s *TrackingService) BuildOurClickURL(
	campaignID, creativeID, lineItemID string,
	sourceType, sourceID string,
	impressionID, gaid, idfa string,
	sub1, sub2, sub3, sub4, sub5 string,
) string {
	params := url.Values{}
	params.Set("cid", campaignID)
	params.Set("cr", creativeID)
	if lineItemID != "" {
		params.Set("li", lineItemID)
	}
	params.Set("src", sourceID)
	params.Set("st", sourceType)
	if impressionID != "" {
		params.Set("imp", impressionID)
	}
	if gaid != "" {
		params.Set("gaid", gaid)
	}
	if idfa != "" {
		params.Set("idfa", idfa)
	}
	if sub1 != "" {
		params.Set("sub1", sub1)
	}
	if sub2 != "" {
		params.Set("sub2", sub2)
	}
	if sub3 != "" {
		params.Set("sub3", sub3)
	}
	if sub4 != "" {
		params.Set("sub4", sub4)
	}
	if sub5 != "" {
		params.Set("sub5", sub5)
	}

	return fmt.Sprintf("%s/track/click?%s", s.baseURL, params.Encode())
}

// BuildOurViewURL builds tracking view URL for ad responses.
func (s *TrackingService) BuildOurViewURL(
	campaignID, creativeID, lineItemID string,
	sourceType, sourceID string,
	impressionID, gaid, idfa string,
) string {
	params := url.Values{}
	params.Set("cid", campaignID)
	params.Set("cr", creativeID)
	if lineItemID != "" {
		params.Set("li", lineItemID)
	}
	params.Set("src", sourceID)
	params.Set("st", sourceType)
	if impressionID != "" {
		params.Set("imp", impressionID)
	}
	if gaid != "" {
		params.Set("gaid", gaid)
	}
	if idfa != "" {
		params.Set("idfa", idfa)
	}

	return fmt.Sprintf("%s/track/view?%s", s.baseURL, params.Encode())
}

// parseUserAgent extracts device info from User-Agent string.
func parseUserAgent(ua string) (os, deviceType, make, model string) {
	ua = strings.ToLower(ua)

	// Detect OS
	switch {
	case strings.Contains(ua, "android"):
		os = "android"
	case strings.Contains(ua, "iphone"):
		os = "ios"
	case strings.Contains(ua, "ipad"):
		os = "ios"
	case strings.Contains(ua, "windows"):
		os = "windows"
	case strings.Contains(ua, "mac"):
		os = "macos"
	default:
		os = "unknown"
	}

	// Detect device type
	switch {
	case strings.Contains(ua, "mobile"):
		deviceType = "phone"
	case strings.Contains(ua, "tablet"):
		deviceType = "tablet"
	case strings.Contains(ua, "ipad"):
		deviceType = "tablet"
	case strings.Contains(ua, "iphone"):
		deviceType = "phone"
	case strings.Contains(ua, "android"):
		if strings.Contains(ua, "mobile") {
			deviceType = "phone"
		} else {
			deviceType = "tablet"
		}
	default:
		deviceType = "desktop"
	}

	// Basic make detection
	switch {
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		make = "Apple"
	case strings.Contains(ua, "samsung"):
		make = "Samsung"
	case strings.Contains(ua, "huawei"):
		make = "Huawei"
	case strings.Contains(ua, "xiaomi"):
		make = "Xiaomi"
	case strings.Contains(ua, "pixel"):
		make = "Google"
	default:
		make = "unknown"
	}

	return os, deviceType, make, model
}
