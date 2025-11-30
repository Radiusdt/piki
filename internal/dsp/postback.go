package dsp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/radiusdt/vector-dsp/internal/metrics"
	"github.com/radiusdt/vector-dsp/internal/models"
	"github.com/radiusdt/vector-dsp/internal/storage"
	"go.uber.org/zap"
)

// PostbackHandler handles postbacks from MMPs (AppsFlyer, Adjust, Singular, etc.)
type PostbackHandler struct {
	eventStore   storage.EventStore
	sourceRepo   storage.SourceRepo
	campaignRepo storage.CampaignRepo
	logger       *zap.Logger
	metrics      *metrics.Metrics
	httpClient   *http.Client
}

// PostbackResult represents the result of processing a postback.
type PostbackResult struct {
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	ConversionID string `json:"conversion_id,omitempty"`
	Error        string `json:"error,omitempty"`
}

// NewPostbackHandler creates a new postback handler.
func NewPostbackHandler(
	eventStore storage.EventStore,
	sourceRepo storage.SourceRepo,
	campaignRepo storage.CampaignRepo,
	logger *zap.Logger,
	m *metrics.Metrics,
) *PostbackHandler {
	return &PostbackHandler{
		eventStore:   eventStore,
		sourceRepo:   sourceRepo,
		campaignRepo: campaignRepo,
		logger:       logger,
		metrics:      m,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// HandleAppsFlyer processes AppsFlyer postbacks.
// Expected URL: /postback/appsflyer?click_id={clickid}&event={event_name}&revenue={event_revenue}&currency={currency}&idfa={idfa}&gaid={advertising_id}
func (h *PostbackHandler) HandleAppsFlyer(ctx context.Context, r *http.Request) (*PostbackResult, error) {
	q := r.URL.Query()

	clickID := q.Get("click_id")
	if clickID == "" {
		clickID = q.Get("clickid")
	}
	if clickID == "" {
		return &PostbackResult{Success: false, Error: "missing click_id"}, nil
	}

	eventName := q.Get("event")
	if eventName == "" {
		eventName = q.Get("event_name")
	}
	if eventName == "" {
		eventName = "install" // Default
	}

	// Parse revenue
	revenue := 0.0
	if revStr := q.Get("revenue"); revStr != "" {
		revenue, _ = strconv.ParseFloat(revStr, 64)
	} else if revStr := q.Get("event_revenue"); revStr != "" {
		revenue, _ = strconv.ParseFloat(revStr, 64)
	}

	currency := q.Get("currency")
	if currency == "" {
		currency = "USD"
	}

	// Device IDs
	idfa := q.Get("idfa")
	gaid := q.Get("gaid")
	if gaid == "" {
		gaid = q.Get("advertising_id")
	}

	// External ID (AppsFlyer's conversion ID)
	externalID := q.Get("install_id")
	if externalID == "" {
		externalID = q.Get("appsflyer_id")
	}

	// Map AppsFlyer event to internal event
	internalEvent := mapAppsFlyerEvent(eventName)

	return h.processPostback(ctx, clickID, internalEvent, eventName, revenue, currency, gaid, idfa, externalID)
}

// HandleAdjust processes Adjust postbacks.
// Expected URL: /postback/adjust?click_id={click_id}&event_token={event_token}&revenue={revenue}&currency={currency}&gps_adid={gps_adid}
func (h *PostbackHandler) HandleAdjust(ctx context.Context, r *http.Request) (*PostbackResult, error) {
	q := r.URL.Query()

	clickID := q.Get("click_id")
	if clickID == "" {
		return &PostbackResult{Success: false, Error: "missing click_id"}, nil
	}

	eventToken := q.Get("event_token")
	if eventToken == "" {
		eventToken = q.Get("event")
	}
	if eventToken == "" {
		eventToken = "install"
	}

	revenue := 0.0
	if revStr := q.Get("revenue"); revStr != "" {
		revenue, _ = strconv.ParseFloat(revStr, 64)
	}

	currency := q.Get("currency")
	if currency == "" {
		currency = "USD"
	}

	gaid := q.Get("gps_adid")
	if gaid == "" {
		gaid = q.Get("gaid")
	}
	idfa := q.Get("idfa")

	externalID := q.Get("adid") // Adjust ID

	internalEvent := mapAdjustEvent(eventToken)

	return h.processPostback(ctx, clickID, internalEvent, eventToken, revenue, currency, gaid, idfa, externalID)
}

// HandleSingular processes Singular postbacks.
// Expected URL: /postback/singular?click_id={click_id}&event={event}&revenue={revenue}&aifa={aifa}
func (h *PostbackHandler) HandleSingular(ctx context.Context, r *http.Request) (*PostbackResult, error) {
	q := r.URL.Query()

	clickID := q.Get("click_id")
	if clickID == "" {
		return &PostbackResult{Success: false, Error: "missing click_id"}, nil
	}

	eventName := q.Get("event")
	if eventName == "" {
		eventName = "__INSTALL__"
	}

	revenue := 0.0
	if revStr := q.Get("revenue"); revStr != "" {
		revenue, _ = strconv.ParseFloat(revStr, 64)
	}

	currency := q.Get("currency")
	if currency == "" {
		currency = "USD"
	}

	gaid := q.Get("aifa") // Android ID for attribution
	if gaid == "" {
		gaid = q.Get("gaid")
	}
	idfa := q.Get("idfa")

	externalID := q.Get("singular_id")

	internalEvent := mapSingularEvent(eventName)

	return h.processPostback(ctx, clickID, internalEvent, eventName, revenue, currency, gaid, idfa, externalID)
}

// HandleGeneric processes generic/custom postbacks.
// Expected URL: /postback?click_id={click_id}&event={event}&revenue={revenue}
func (h *PostbackHandler) HandleGeneric(ctx context.Context, r *http.Request) (*PostbackResult, error) {
	q := r.URL.Query()

	clickID := q.Get("click_id")
	if clickID == "" {
		clickID = q.Get("clickid")
	}
	if clickID == "" {
		return &PostbackResult{Success: false, Error: "missing click_id"}, nil
	}

	eventName := q.Get("event")
	if eventName == "" {
		eventName = "install"
	}

	revenue := 0.0
	if revStr := q.Get("revenue"); revStr != "" {
		revenue, _ = strconv.ParseFloat(revStr, 64)
	}

	currency := q.Get("currency")
	if currency == "" {
		currency = "USD"
	}

	gaid := q.Get("gaid")
	idfa := q.Get("idfa")
	externalID := q.Get("external_id")

	return h.processPostback(ctx, clickID, eventName, eventName, revenue, currency, gaid, idfa, externalID)
}

// processPostback is the common logic for all MMP postbacks.
func (h *PostbackHandler) processPostback(
	ctx context.Context,
	clickID, internalEvent, originalEvent string,
	revenue float64, currency string,
	gaid, idfa, externalID string,
) (*PostbackResult, error) {
	// Look up the original click
	click, err := h.eventStore.GetClick(ctx, clickID)
	if err != nil {
		h.logger.Warn("click not found for postback",
			zap.String("click_id", clickID),
			zap.String("event", internalEvent),
			zap.Error(err),
		)
		return &PostbackResult{Success: false, Error: "click not found"}, nil
	}

	// Generate conversion ID
	conversionID := uuid.New().String()

	// Calculate time to install
	timeToInstall := int32(0)
	if click != nil {
		timeToInstall = int32(time.Since(click.Timestamp).Seconds())
	}

	// Determine device IFA
	deviceIFA := gaid
	if deviceIFA == "" {
		deviceIFA = idfa
	}
	if deviceIFA == "" && click != nil {
		deviceIFA = click.DeviceIFA
	}

	// Get geo from click
	geoCountry := ""
	if click != nil {
		geoCountry = click.GeoCountry
	}

	// Calculate payout (simplified - real implementation would check campaign settings)
	payout := 0.0
	if click != nil {
		campaign, err := h.campaignRepo.GetByID(ctx, click.CampaignID)
		if err == nil && campaign != nil {
			if campaign.PayoutEvent == internalEvent || campaign.PayoutEvent == "" {
				payout = campaign.PayoutAmount
			}
		}
	}

	// Convert revenue to USD (simplified - real implementation would use exchange rates)
	revenueUSD := revenue
	if currency != "USD" && revenue > 0 {
		// Apply exchange rate conversion here
		revenueUSD = revenue // Placeholder
	}

	// Create conversion record
	conversion := &models.Conversion{
		ID:            conversionID,
		Timestamp:     time.Now(),
		ClickID:       clickID,
		Event:         internalEvent,
		EventOriginal: originalEvent,
		Revenue:       revenue,
		RevenueCurrency: currency,
		RevenueUSD:    revenueUSD,
		Payout:        payout,
		PayoutCurrency: "USD",
		PayoutUSD:     payout,
		DeviceIFA:     deviceIFA,
		GeoCountry:    geoCountry,
		TimeToInstall: timeToInstall,
		ExternalID:    externalID,
	}

	if click != nil {
		conversion.CampaignID = click.CampaignID
		conversion.LineItemID = click.LineItemID
		conversion.CreativeID = click.CreativeID
		conversion.SourceType = click.SourceType
		conversion.SourceID = click.SourceID
		conversion.ClickTimestamp = click.Timestamp
	}

	// Save conversion
	if err := h.eventStore.SaveConversion(ctx, conversion); err != nil {
		h.logger.Error("failed to save conversion", zap.Error(err))
		return &PostbackResult{Success: false, Error: "failed to save conversion"}, err
	}

	h.logger.Info("conversion recorded",
		zap.String("conversion_id", conversionID),
		zap.String("click_id", clickID),
		zap.String("event", internalEvent),
		zap.Float64("revenue", revenue),
		zap.Float64("payout", payout),
	)

	// Record metrics
	if h.metrics != nil && click != nil {
		h.metrics.RecordConversion(click.CampaignID, click.LineItemID, internalEvent, payout)
	}

	// Send postback to S2S source if configured
	if click != nil && click.SourceType == "s2s" {
		go h.sendPostbackToSource(ctx, conversion, click)
	}

	return &PostbackResult{
		Success:      true,
		Message:      "conversion recorded",
		ConversionID: conversionID,
	}, nil
}

// sendPostbackToSource sends conversion postback to S2S source.
func (h *PostbackHandler) sendPostbackToSource(ctx context.Context, conv *models.Conversion, click *models.Click) {
	source, err := h.sourceRepo.GetS2SSource(ctx, click.SourceID)
	if err != nil || source == nil {
		return
	}

	// Check if this event should be sent
	if !shouldSendEvent(source.PostbackEvents, conv.Event) {
		return
	}

	if source.PostbackURL == "" {
		return
	}

	// Build postback URL with macro replacements
	postbackURL := source.PostbackURL

	replacements := map[string]string{
		"{click_id}":      click.ID,
		"{clickid}":       click.ID,
		"{conversion_id}": conv.ID,
		"{event}":         conv.Event,
		"{revenue}":       fmt.Sprintf("%.4f", conv.Revenue),
		"{currency}":      conv.RevenueCurrency,
		"{payout}":        fmt.Sprintf("%.4f", conv.Payout),
		"{timestamp}":     fmt.Sprintf("%d", conv.Timestamp.Unix()),
		"{sub1}":          click.Sub1,
		"{sub2}":          click.Sub2,
		"{sub3}":          click.Sub3,
		"{sub4}":          click.Sub4,
		"{sub5}":          click.Sub5,
	}

	for macro, value := range replacements {
		postbackURL = strings.ReplaceAll(postbackURL, macro, url.QueryEscape(value))
	}

	// Apply source-specific macro mapping
	for ourMacro, theirMacro := range source.MacrosMapping {
		if value, ok := replacements["{"+ourMacro+"}"]; ok {
			postbackURL = strings.ReplaceAll(postbackURL, "{"+theirMacro+"}", url.QueryEscape(value))
		}
	}

	// Send postback
	var resp *http.Response
	if source.PostbackMethod == "POST" {
		resp, err = h.httpClient.Post(postbackURL, "application/x-www-form-urlencoded", nil)
	} else {
		resp, err = h.httpClient.Get(postbackURL)
	}

	if err != nil {
		h.logger.Warn("failed to send postback to source",
			zap.String("source_id", source.ID),
			zap.String("url", postbackURL),
			zap.Error(err),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		h.logger.Warn("source postback returned error",
			zap.String("source_id", source.ID),
			zap.Int("status", resp.StatusCode),
		)
	} else {
		h.logger.Debug("postback sent to source",
			zap.String("source_id", source.ID),
			zap.String("event", conv.Event),
			zap.Int("status", resp.StatusCode),
		)
	}
}

// shouldSendEvent checks if event should be sent based on source configuration.
func shouldSendEvent(configuredEvents []string, event string) bool {
	if len(configuredEvents) == 0 {
		return true // Send all events if not configured
	}
	for _, e := range configuredEvents {
		if e == event || e == "*" {
			return true
		}
	}
	return false
}

// =============================================
// Event Mapping Functions
// =============================================

func mapAppsFlyerEvent(event string) string {
	mapping := map[string]string{
		"install":                    "install",
		"af_app_install":             "install",
		"af_complete_registration":   "registration",
		"af_purchase":                "purchase",
		"af_first_purchase":          "first_purchase",
		"af_subscribe":               "subscribe",
		"af_add_to_cart":             "add_to_cart",
		"af_initiated_checkout":      "checkout",
		"af_level_achieved":          "level_achieved",
		"af_tutorial_completion":     "tutorial_complete",
		"af_achievement_unlocked":    "achievement",
		"af_content_view":            "content_view",
		"af_search":                  "search",
		"af_rate":                    "rate",
		"af_start_trial":             "start_trial",
	}

	if mapped, ok := mapping[event]; ok {
		return mapped
	}
	return event
}

func mapAdjustEvent(event string) string {
	// Adjust uses event tokens, mapping should be configured per campaign
	mapping := map[string]string{
		"install":      "install",
		"session":      "session",
		"registration": "registration",
		"purchase":     "purchase",
		"revenue":      "revenue",
	}

	if mapped, ok := mapping[event]; ok {
		return mapped
	}
	return event
}

func mapSingularEvent(event string) string {
	mapping := map[string]string{
		"__INSTALL__":      "install",
		"__SESSION__":      "session",
		"__CUSTOM_EVENT__": "custom",
		"__REVENUE__":      "revenue",
	}

	if mapped, ok := mapping[event]; ok {
		return mapped
	}
	return event
}

// =============================================
// Helper Types
// =============================================

// PostbackRequest represents raw postback data (for logging/debugging)
type PostbackRequest struct {
	Timestamp   time.Time         `json:"timestamp"`
	MMP         string            `json:"mmp"`
	RawParams   map[string]string `json:"raw_params"`
	Headers     map[string]string `json:"headers"`
	SourceIP    string            `json:"source_ip"`
	ProcessedOK bool              `json:"processed_ok"`
}

// MarshalJSON for logging
func (p *PostbackRequest) MarshalJSON() ([]byte, error) {
	type Alias PostbackRequest
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	})
}
