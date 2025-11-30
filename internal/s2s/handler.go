package s2s

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// S2SHandler handles S2S partner ad requests
type S2SHandler struct {
	sourceStore     SourceStore
	campaignStore   CampaignStore
	trackingService TrackingService
	logger          *zap.Logger
	baseURL         string
}

// SourceStore interface for source lookups
type SourceStore interface {
	GetS2SSourceByName(ctx context.Context, internalName string) (*S2SSource, error)
	ValidateSourceToken(ctx context.Context, sourceID, token string) bool
	ValidateSourceIP(ctx context.Context, sourceID, ip string) bool
}

// S2SSource represents an S2S partner
type S2SSource struct {
	ID             string
	Name           string
	InternalName   string
	APIToken       string
	AllowedIPs     []string
	DefaultPayout  float64
	PayoutType     string
	Status         string
}

// CampaignStore interface for campaign lookups
type CampaignStore interface {
	GetActiveCampaignsForSource(ctx context.Context, sourceID string) ([]*Campaign, error)
	GetCampaign(ctx context.Context, id string) (*Campaign, error)
}

// Campaign represents campaign data
type Campaign struct {
	ID           string
	Name         string
	AppBundle    string
	MMPClickURL  string
	MMPViewURL   string
	BidAmount    float64
	PayoutAmount float64
	Targeting    *Targeting
	Creatives    []*Creative
}

// Targeting holds targeting criteria
type Targeting struct {
	Countries   []string
	OS          []string
	DeviceTypes []string
}

// Creative holds creative data
type Creative struct {
	ID          string
	Name        string
	Type        string
	BannerURL   string
	BannerW     int
	BannerH     int
	Title       string
	Description string
	IconURL     string
	ImageURL    string
	CTA         string
}

// TrackingService interface for tracking
type TrackingService interface {
	BuildOurClickURL(campaignID, creativeID, lineItemID, sourceID, sourceType string) string
	BuildOurViewURL(campaignID, creativeID, lineItemID, sourceID, sourceType, impressionID string) string
}

// NewS2SHandler creates a new S2S handler
func NewS2SHandler(
	sourceStore SourceStore,
	campaignStore CampaignStore,
	trackingService TrackingService,
	baseURL string,
	logger *zap.Logger,
) *S2SHandler {
	return &S2SHandler{
		sourceStore:     sourceStore,
		campaignStore:   campaignStore,
		trackingService: trackingService,
		baseURL:         baseURL,
		logger:          logger,
	}
}

// AdRequest represents incoming ad request from S2S partner
type AdRequest struct {
	SourceName  string
	Country     string
	OS          string
	DeviceType  string
	DeviceIFA   string
	IP          string
	UserAgent   string
	Sub1        string
	Sub2        string
	Sub3        string
	Sub4        string
	Sub5        string
	Token       string
	CampaignID  string // Optional: request specific campaign
}

// AdResponse represents ad response to S2S partner
type AdResponse struct {
	Success    bool        `json:"success"`
	CampaignID string      `json:"campaign_id,omitempty"`
	AppBundle  string      `json:"app_bundle,omitempty"`
	AppName    string      `json:"app_name,omitempty"`
	Creative   *CreativeAd `json:"creative,omitempty"`
	ClickURL   string      `json:"click_url"`
	ViewURL    string      `json:"view_url,omitempty"`
	Payout     float64     `json:"payout,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// CreativeAd represents creative in ad response
type CreativeAd struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // banner, native
	BannerURL   string `json:"banner_url,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	IconURL     string `json:"icon_url,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	CTA         string `json:"cta,omitempty"`
}

// HandleAdRequest handles incoming ad request from S2S partner
// GET /s2s/{source}/ad?country=US&os=android&device_type=phone&gaid=xxx&sub1=yyy&...
func (h *S2SHandler) HandleAdRequest(ctx context.Context, r *http.Request) (*AdResponse, error) {
	// Extract source name from URL path
	// Expected: /s2s/{source_internal_name}/ad
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		return &AdResponse{Success: false, Error: "invalid path"}, nil
	}
	sourceName := parts[1]

	// Look up source
	source, err := h.sourceStore.GetS2SSourceByName(ctx, sourceName)
	if err != nil || source == nil {
		return &AdResponse{Success: false, Error: "source not found"}, nil
	}

	if source.Status != "active" {
		return &AdResponse{Success: false, Error: "source inactive"}, nil
	}

	// Parse request parameters
	q := r.URL.Query()
	req := &AdRequest{
		SourceName: sourceName,
		Country:    strings.ToUpper(q.Get("country")),
		OS:         strings.ToLower(q.Get("os")),
		DeviceType: strings.ToLower(q.Get("device_type")),
		DeviceIFA:  q.Get("gaid"),
		IP:         getClientIP(r),
		UserAgent:  r.UserAgent(),
		Sub1:       q.Get("sub1"),
		Sub2:       q.Get("sub2"),
		Sub3:       q.Get("sub3"),
		Sub4:       q.Get("sub4"),
		Sub5:       q.Get("sub5"),
		Token:      q.Get("token"),
		CampaignID: q.Get("campaign_id"),
	}

	// Get IDFA if no GAID
	if req.DeviceIFA == "" {
		req.DeviceIFA = q.Get("idfa")
	}

	// Validate token if required
	if source.APIToken != "" {
		if !h.sourceStore.ValidateSourceToken(ctx, source.ID, req.Token) {
			return &AdResponse{Success: false, Error: "invalid token"}, nil
		}
	}

	// Validate IP if required
	if len(source.AllowedIPs) > 0 {
		if !h.sourceStore.ValidateSourceIP(ctx, source.ID, req.IP) {
			return &AdResponse{Success: false, Error: "ip not allowed"}, nil
		}
	}

	// Find matching campaign
	campaign, creative, err := h.findMatchingCampaign(ctx, source, req)
	if err != nil || campaign == nil {
		return &AdResponse{Success: false, Error: "no ad available"}, nil
	}

	// Generate tracking URLs
	impressionID := uuid.New().String()
	clickURL := h.buildClickURL(campaign, creative, source, req, impressionID)
	viewURL := h.buildViewURL(campaign, creative, source, req, impressionID)

	// Calculate payout
	payout := h.calculatePayout(source, campaign)

	h.logger.Info("s2s ad served",
		zap.String("source", source.Name),
		zap.String("campaign_id", campaign.ID),
		zap.String("creative_id", creative.ID),
		zap.Float64("payout", payout),
	)

	return &AdResponse{
		Success:    true,
		CampaignID: campaign.ID,
		AppBundle:  campaign.AppBundle,
		Creative: &CreativeAd{
			ID:          creative.ID,
			Type:        creative.Type,
			BannerURL:   creative.BannerURL,
			Width:       creative.BannerW,
			Height:      creative.BannerH,
			Title:       creative.Title,
			Description: creative.Description,
			IconURL:     creative.IconURL,
			ImageURL:    creative.ImageURL,
			CTA:         creative.CTA,
		},
		ClickURL: clickURL,
		ViewURL:  viewURL,
		Payout:   payout,
	}, nil
}

// findMatchingCampaign finds a campaign that matches the request
func (h *S2SHandler) findMatchingCampaign(ctx context.Context, source *S2SSource, req *AdRequest) (*Campaign, *Creative, error) {
	// If specific campaign requested
	if req.CampaignID != "" {
		campaign, err := h.campaignStore.GetCampaign(ctx, req.CampaignID)
		if err != nil || campaign == nil {
			return nil, nil, err
		}
		if !h.matchesTargeting(campaign, req) {
			return nil, nil, nil
		}
		if len(campaign.Creatives) == 0 {
			return nil, nil, nil
		}
		return campaign, campaign.Creatives[0], nil
	}

	// Get all active campaigns for this source
	campaigns, err := h.campaignStore.GetActiveCampaignsForSource(ctx, source.ID)
	if err != nil {
		return nil, nil, err
	}

	// Find first matching campaign
	for _, campaign := range campaigns {
		if h.matchesTargeting(campaign, req) && len(campaign.Creatives) > 0 {
			return campaign, campaign.Creatives[0], nil
		}
	}

	return nil, nil, nil
}

// matchesTargeting checks if request matches campaign targeting
func (h *S2SHandler) matchesTargeting(campaign *Campaign, req *AdRequest) bool {
	if campaign.Targeting == nil {
		return true
	}

	// Check country
	if len(campaign.Targeting.Countries) > 0 {
		matched := false
		for _, c := range campaign.Targeting.Countries {
			if strings.EqualFold(c, req.Country) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check OS
	if len(campaign.Targeting.OS) > 0 {
		matched := false
		for _, os := range campaign.Targeting.OS {
			if strings.EqualFold(os, req.OS) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check device type
	if len(campaign.Targeting.DeviceTypes) > 0 {
		matched := false
		for _, dt := range campaign.Targeting.DeviceTypes {
			if strings.EqualFold(dt, req.DeviceType) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// buildClickURL builds our tracking click URL
func (h *S2SHandler) buildClickURL(campaign *Campaign, creative *Creative, source *S2SSource, req *AdRequest, impID string) string {
	// Build URL with all parameters for tracking
	params := fmt.Sprintf(
		"cid=%s&cr=%s&src=%s&st=s2s&imp=%s&gaid=%s&sub1=%s&sub2=%s&sub3=%s&sub4=%s&sub5=%s",
		campaign.ID, creative.ID, source.ID, impID,
		req.DeviceIFA, req.Sub1, req.Sub2, req.Sub3, req.Sub4, req.Sub5,
	)
	return fmt.Sprintf("%s/track/click?%s", h.baseURL, params)
}

// buildViewURL builds our tracking view URL
func (h *S2SHandler) buildViewURL(campaign *Campaign, creative *Creative, source *S2SSource, req *AdRequest, impID string) string {
	params := fmt.Sprintf(
		"cid=%s&cr=%s&src=%s&st=s2s&imp=%s&gaid=%s",
		campaign.ID, creative.ID, source.ID, impID, req.DeviceIFA,
	)
	return fmt.Sprintf("%s/track/view?%s", h.baseURL, params)
}

// calculatePayout calculates payout for this impression
func (h *S2SHandler) calculatePayout(source *S2SSource, campaign *Campaign) float64 {
	// Campaign-specific payout takes priority
	if campaign.PayoutAmount > 0 {
		if source.PayoutType == "percent" {
			return campaign.BidAmount * (campaign.PayoutAmount / 100)
		}
		return campaign.PayoutAmount
	}

	// Fall back to source default
	if source.DefaultPayout > 0 {
		if source.PayoutType == "percent" {
			return campaign.BidAmount * (source.DefaultPayout / 100)
		}
		return source.DefaultPayout
	}

	return 0
}

// HandleClick handles click redirect from S2S
// GET /s2s/{source}/click?cid={campaign_id}&...
func (h *S2SHandler) HandleClick(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	campaignID := q.Get("cid")
	if campaignID == "" {
		http.Error(w, "missing campaign_id", http.StatusBadRequest)
		return
	}

	// Get campaign to get MMP click URL
	campaign, err := h.campaignStore.GetCampaign(ctx, campaignID)
	if err != nil || campaign == nil {
		http.Error(w, "campaign not found", http.StatusNotFound)
		return
	}

	// Build redirect URL (MMP click URL with macros)
	redirectURL := campaign.MMPClickURL
	if redirectURL == "" {
		http.Error(w, "no redirect URL configured", http.StatusInternalServerError)
		return
	}

	// Replace macros in MMP URL
	redirectURL = h.replaceMacros(redirectURL, r)

	// Redirect to MMP
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// replaceMacros replaces macros in URL
func (h *S2SHandler) replaceMacros(urlStr string, r *http.Request) string {
	q := r.URL.Query()

	replacements := map[string]string{
		"{click_id}":       q.Get("click_id"),
		"{clickid}":        q.Get("click_id"),
		"{campaign_id}":    q.Get("cid"),
		"{creative_id}":    q.Get("cr"),
		"{source_id}":      q.Get("src"),
		"{gaid}":           q.Get("gaid"),
		"{advertising_id}": q.Get("gaid"),
		"{idfa}":           q.Get("idfa"),
		"{sub1}":           q.Get("sub1"),
		"{sub2}":           q.Get("sub2"),
		"{sub3}":           q.Get("sub3"),
		"{sub4}":           q.Get("sub4"),
		"{sub5}":           q.Get("sub5"),
		"{ip}":             getClientIP(r),
		"{ua}":             r.UserAgent(),
		"{timestamp}":      fmt.Sprintf("%d", time.Now().Unix()),
	}

	for macro, value := range replacements {
		urlStr = strings.ReplaceAll(urlStr, macro, value)
	}

	return urlStr
}

// getClientIP extracts client IP from request
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// ServeJSON writes JSON response
func ServeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
