package postback

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PostbackHandler handles incoming postbacks from MMPs and S2S sources
type PostbackHandler struct {
	clickStore      ClickStore
	conversionStore ConversionStore
	sourceStore     SourceStore
	httpClient      *http.Client
	logger          *zap.Logger
}

// ClickStore interface for click lookups
type ClickStore interface {
	GetClick(ctx context.Context, id string) (*Click, error)
}

// Click represents stored click data
type Click struct {
	ID           string
	CampaignID   string
	LineItemID   string
	CreativeID   string
	SourceType   string
	SourceID     string
	SourceName   string
	Timestamp    time.Time
}

// ConversionStore interface for storing conversions
type ConversionStore interface {
	SaveConversion(ctx context.Context, conv *Conversion) error
}

// Conversion represents a conversion event
type Conversion struct {
	ID              string
	Timestamp       time.Time
	ClickID         string
	CampaignID      string
	LineItemID      string
	CreativeID      string
	SourceType      string
	SourceID        string
	SourceName      string
	Event           string
	EventOriginal   string
	Revenue         float64
	RevenueCurrency string
	RevenueUSD      float64
	Payout          float64
	PayoutCurrency  string
	PayoutUSD       float64
	DeviceIFA       string
	ExternalID      string
	Params          map[string]string
}

// SourceStore interface for source lookups
type SourceStore interface {
	GetS2SSource(ctx context.Context, id string) (*S2SSource, error)
}

// S2SSource represents an S2S partner configuration
type S2SSource struct {
	ID             string
	Name           string
	PostbackURL    string
	PostbackMethod string
	PostbackEvents []string
	MacrosMapping  map[string]string
}

// NewPostbackHandler creates a new postback handler
func NewPostbackHandler(
	clickStore ClickStore,
	conversionStore ConversionStore,
	sourceStore SourceStore,
	logger *zap.Logger,
) *PostbackHandler {
	return &PostbackHandler{
		clickStore:      clickStore,
		conversionStore: conversionStore,
		sourceStore:     sourceStore,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// PostbackParams represents parameters from incoming postback
type PostbackParams struct {
	ClickID         string
	Event           string
	Revenue         string
	Currency        string
	IDFA            string
	GAID            string
	ExternalID      string
	TransactionID   string
	SubParams       map[string]string
}

// PostbackResult represents the result of postback processing
type PostbackResult struct {
	ConversionID string
	Status       string
	Message      string
}

// HandleAppsFlyer processes AppsFlyer postback
// Expected URL: /postback/appsflyer?click_id={clickid}&event={event_name}&revenue={event_revenue}&currency={currency}&idfa={idfa}&gaid={advertising_id}
func (h *PostbackHandler) HandleAppsFlyer(ctx context.Context, r *http.Request) (*PostbackResult, error) {
	q := r.URL.Query()

	params := &PostbackParams{
		ClickID:    q.Get("click_id"),
		Event:      q.Get("event"),
		Revenue:    q.Get("revenue"),
		Currency:   q.Get("currency"),
		IDFA:       q.Get("idfa"),
		GAID:       q.Get("gaid"),
		ExternalID: q.Get("appsflyer_id"),
	}

	// Map AppsFlyer event to internal event
	internalEvent := mapAppsFlyerEvent(params.Event)

	return h.processPostback(ctx, "appsflyer", params, internalEvent)
}

// HandleAdjust processes Adjust postback
func (h *PostbackHandler) HandleAdjust(ctx context.Context, r *http.Request) (*PostbackResult, error) {
	q := r.URL.Query()

	params := &PostbackParams{
		ClickID:    q.Get("click_id"),
		Event:      q.Get("event_token"),
		Revenue:    q.Get("revenue"),
		Currency:   q.Get("currency"),
		IDFA:       q.Get("idfa"),
		GAID:       q.Get("gps_adid"),
		ExternalID: q.Get("adid"),
	}

	internalEvent := mapAdjustEvent(params.Event)

	return h.processPostback(ctx, "adjust", params, internalEvent)
}

// HandleSingular processes Singular postback
func (h *PostbackHandler) HandleSingular(ctx context.Context, r *http.Request) (*PostbackResult, error) {
	q := r.URL.Query()

	params := &PostbackParams{
		ClickID:    q.Get("click_id"),
		Event:      q.Get("event"),
		Revenue:    q.Get("revenue"),
		Currency:   q.Get("currency"),
		IDFA:       q.Get("idfa"),
		GAID:       q.Get("aifa"),
		ExternalID: q.Get("singular_id"),
	}

	internalEvent := mapSingularEvent(params.Event)

	return h.processPostback(ctx, "singular", params, internalEvent)
}

// HandleGeneric processes generic postback (for S2S sources)
func (h *PostbackHandler) HandleGeneric(ctx context.Context, r *http.Request) (*PostbackResult, error) {
	q := r.URL.Query()

	params := &PostbackParams{
		ClickID:       q.Get("click_id"),
		Event:         q.Get("event"),
		Revenue:       q.Get("revenue"),
		Currency:      q.Get("currency"),
		IDFA:          q.Get("idfa"),
		GAID:          q.Get("gaid"),
		ExternalID:    q.Get("external_id"),
		TransactionID: q.Get("transaction_id"),
		SubParams:     extractSubParams(q),
	}

	// Default to "install" if no event specified
	if params.Event == "" {
		params.Event = "install"
	}

	return h.processPostback(ctx, "generic", params, params.Event)
}

// processPostback is the main postback processing logic
func (h *PostbackHandler) processPostback(ctx context.Context, mmpType string, params *PostbackParams, internalEvent string) (*PostbackResult, error) {
	// Validate required params
	if params.ClickID == "" {
		return &PostbackResult{
			Status:  "error",
			Message: "click_id required",
		}, nil
	}

	// Look up click
	click, err := h.clickStore.GetClick(ctx, params.ClickID)
	if err != nil {
		h.logger.Error("failed to get click", zap.Error(err), zap.String("click_id", params.ClickID))
		return &PostbackResult{
			Status:  "error",
			Message: "click lookup failed",
		}, err
	}

	if click == nil {
		h.logger.Warn("click not found", zap.String("click_id", params.ClickID))
		return &PostbackResult{
			Status:  "error",
			Message: "click not found",
		}, nil
	}

	// Parse revenue
	revenue := 0.0
	if params.Revenue != "" {
		if val, err := strconv.ParseFloat(params.Revenue, 64); err == nil {
			revenue = val
		}
	}

	// Get device IFA
	deviceIFA := params.GAID
	if deviceIFA == "" {
		deviceIFA = params.IDFA
	}

	// Create conversion
	conversionID := uuid.New().String()
	conv := &Conversion{
		ID:              conversionID,
		Timestamp:       time.Now().UTC(),
		ClickID:         params.ClickID,
		CampaignID:      click.CampaignID,
		LineItemID:      click.LineItemID,
		CreativeID:      click.CreativeID,
		SourceType:      click.SourceType,
		SourceID:        click.SourceID,
		SourceName:      click.SourceName,
		Event:           internalEvent,
		EventOriginal:   params.Event,
		Revenue:         revenue,
		RevenueCurrency: params.Currency,
		RevenueUSD:      revenue, // TODO: currency conversion
		DeviceIFA:       deviceIFA,
		ExternalID:      params.ExternalID,
		Params:          params.SubParams,
	}

	// Save conversion
	if err := h.conversionStore.SaveConversion(ctx, conv); err != nil {
		h.logger.Error("failed to save conversion", zap.Error(err))
		return &PostbackResult{
			Status:  "error",
			Message: "save failed",
		}, err
	}

	h.logger.Info("conversion registered",
		zap.String("conversion_id", conversionID),
		zap.String("click_id", params.ClickID),
		zap.String("campaign_id", click.CampaignID),
		zap.String("event", internalEvent),
		zap.Float64("revenue", revenue),
	)

	// Send postback to S2S source if needed
	if click.SourceType == "s2s" {
		go h.sendPostbackToSource(ctx, click, conv)
	}

	return &PostbackResult{
		ConversionID: conversionID,
		Status:       "ok",
		Message:      "conversion registered",
	}, nil
}

// sendPostbackToSource sends postback to S2S partner
func (h *PostbackHandler) sendPostbackToSource(ctx context.Context, click *Click, conv *Conversion) {
	source, err := h.sourceStore.GetS2SSource(ctx, click.SourceID)
	if err != nil || source == nil {
		return
	}

	// Check if this event should be sent
	if !h.shouldSendEvent(source.PostbackEvents, conv.Event) {
		return
	}

	if source.PostbackURL == "" {
		return
	}

	// Build postback URL with macros
	postbackURL := h.buildSourcePostbackURL(source, click, conv)
	if postbackURL == "" {
		return
	}

	// Send postback
	method := source.PostbackMethod
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequestWithContext(ctx, method, postbackURL, nil)
	if err != nil {
		h.logger.Error("failed to create source postback request", zap.Error(err))
		return
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.logger.Error("source postback failed",
			zap.String("source_id", source.ID),
			zap.Error(err),
		)
		return
	}
	defer resp.Body.Close()

	h.logger.Info("source postback sent",
		zap.String("source_id", source.ID),
		zap.String("conversion_id", conv.ID),
		zap.Int("status", resp.StatusCode),
	)
}

// buildSourcePostbackURL builds the postback URL for S2S source
func (h *PostbackHandler) buildSourcePostbackURL(source *S2SSource, click *Click, conv *Conversion) string {
	urlStr := source.PostbackURL

	// Standard macros
	replacements := map[string]string{
		"{click_id}":      click.ID,
		"{conversion_id}": conv.ID,
		"{event}":         conv.Event,
		"{revenue}":       fmt.Sprintf("%.2f", conv.Revenue),
		"{currency}":      conv.RevenueCurrency,
		"{payout}":        fmt.Sprintf("%.2f", conv.Payout),
		"{timestamp}":     fmt.Sprintf("%d", time.Now().Unix()),
	}

	// Apply source-specific macros mapping
	if source.MacrosMapping != nil {
		for ourMacro, theirMacro := range source.MacrosMapping {
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

// shouldSendEvent checks if event should be sent to source
func (h *PostbackHandler) shouldSendEvent(allowedEvents []string, event string) bool {
	if len(allowedEvents) == 0 {
		return true // Send all if not specified
	}
	for _, e := range allowedEvents {
		if strings.EqualFold(e, event) {
			return true
		}
	}
	return false
}

// Event mapping functions
func mapAppsFlyerEvent(event string) string {
	mappings := map[string]string{
		"install":                  "install",
		"af_app_install":           "install",
		"af_complete_registration": "registration",
		"af_purchase":              "purchase",
		"af_first_purchase":        "first_purchase",
		"af_subscribe":             "subscribe",
		"af_add_to_cart":           "add_to_cart",
		"af_initiated_checkout":    "checkout",
		"af_level_achieved":        "level_achieved",
		"af_tutorial_completion":   "tutorial_complete",
	}
	if mapped, ok := mappings[event]; ok {
		return mapped
	}
	return event
}

func mapAdjustEvent(event string) string {
	mappings := map[string]string{
		"install":      "install",
		"session":      "session",
		"registration": "registration",
		"purchase":     "purchase",
	}
	if mapped, ok := mappings[event]; ok {
		return mapped
	}
	return event
}

func mapSingularEvent(event string) string {
	mappings := map[string]string{
		"__INSTALL__":      "install",
		"__SESSION__":      "session",
		"__CUSTOM_EVENT__": "custom",
		"registration":     "registration",
		"purchase":         "purchase",
	}
	if mapped, ok := mappings[event]; ok {
		return mapped
	}
	return event
}

func extractSubParams(q url.Values) map[string]string {
	params := make(map[string]string)
	for key, values := range q {
		if strings.HasPrefix(key, "sub") || strings.HasPrefix(key, "aff_") {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}
	}
	return params
}
