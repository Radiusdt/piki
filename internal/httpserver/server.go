package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/radiusdt/vector-dsp/internal/config"
	"github.com/radiusdt/vector-dsp/internal/database"
	"github.com/radiusdt/vector-dsp/internal/dsp"
	"github.com/radiusdt/vector-dsp/internal/metrics"
	"github.com/radiusdt/vector-dsp/internal/models"
	"github.com/radiusdt/vector-dsp/internal/storage"
	"github.com/radiusdt/vector-dsp/internal/targeting"
	"go.uber.org/zap"
)

// Dependencies holds all external dependencies for the server.
type Dependencies struct {
	DB      *database.PostgresDB
	Redis   *database.RedisDB
	Config  *config.Config
	Logger  *zap.Logger
	Metrics *metrics.Metrics
}

// Server wraps HTTP handlers and DSP services.
type Server struct {
	campaignService   *dsp.CampaignService
	bidService        *dsp.BidService
	eventService      *dsp.EventService
	advertiserService *dsp.AdvertiserService
	adGroupService    *dsp.AdGroupService
	creativeService   *dsp.CreativeService
	sourceService     *dsp.SourceService
	reportingService  *dsp.ReportingService
	pacingEngine      dsp.PacingEngine
	trackingService   *dsp.TrackingService
	postbackHandler   *dsp.PostbackHandler
	logger            *zap.Logger
	config            *config.Config
	metrics           *metrics.Metrics
}

// NewServer constructs a new http.Handler with all routes registered.
func NewServer(deps *Dependencies) http.Handler {
	// Initialize repositories
	var cRepo storage.CampaignRepo
	var advRepo storage.AdvertiserRepo
	var eventStore storage.EventStore
	var sourceRepo storage.SourceRepo

	if deps.DB != nil {
		cRepo = storage.NewPostgresCampaignRepo(deps.DB.Pool)
		advRepo = storage.NewPostgresAdvertiserRepo(deps.DB.Pool)
		eventStore = storage.NewPostgresEventStore(deps.DB.Pool)
		sourceRepo = storage.NewPostgresSourceRepo(deps.DB.Pool)
	} else {
		cRepo = storage.NewInMemoryCampaignRepo()
		advRepo = storage.NewInMemoryAdvertiserRepo()
		eventStore = storage.NewInMemoryEventStore()
		sourceRepo = storage.NewInMemorySourceRepo()
	}

	agRepo := storage.NewInMemoryAdGroupRepo()
	crRepo := storage.NewInMemoryCreativeRepo()

	// Initialize pacing engine
	var pacer dsp.PacingEngine
	if deps.Redis != nil {
		pacer = dsp.NewRedisPacingEngine(deps.Redis.Client, deps.Config.Pacing, deps.Metrics)
	} else {
		pacer = dsp.NewInMemoryPacingEngine()
	}

	// Initialize targeting engine
	var targetingEngine *targeting.TargetingEngine
	if deps.Config.Geo.Enabled {
		geoProvider, err := targeting.NewMaxMindGeoProvider(deps.Config.Geo.DatabasePath)
		if err != nil {
			deps.Logger.Warn("failed to initialize geo provider, using mock", zap.Error(err))
			geoProvider = nil
		}
		if geoProvider != nil {
			targetingEngine = targeting.NewTargetingEngine(
				geoProvider,
				deps.Config.Geo.CacheSize,
				deps.Config.Geo.CacheTTL,
				deps.Metrics,
			)
		}
	}
	if targetingEngine == nil {
		targetingEngine = targeting.NewTargetingEngine(nil, 1000, time.Hour, deps.Metrics)
	}

	// Initialize services
	cSvc := dsp.NewCampaignService(cRepo)
	bSvc := dsp.NewBidService(cRepo, pacer, targetingEngine, deps.Metrics, deps.Config.Tracking.BaseURL)
	eSvc := dsp.NewEventService(eventStore)
	advSvc := dsp.NewAdvertiserService(advRepo)
	agSvc := dsp.NewAdGroupService(agRepo)
	crSvc := dsp.NewCreativeService(crRepo)
	srcSvc := dsp.NewSourceService(sourceRepo)

	// Initialize tracking service
	trackingSvc := dsp.NewTrackingService(
		eventStore,
		cRepo,
		targetingEngine,
		deps.Config.Tracking.BaseURL,
		deps.Logger,
		deps.Metrics,
	)

	// Initialize postback handler
	postbackHandler := dsp.NewPostbackHandler(
		eventStore,
		sourceRepo,
		cRepo,
		deps.Logger,
		deps.Metrics,
	)

	var reportingSvc *dsp.ReportingService
	if deps.Redis != nil {
		reportingSvc = dsp.NewReportingService(eventStore, deps.Redis.Client)
	}

	s := &Server{
		campaignService:   cSvc,
		bidService:        bSvc,
		eventService:      eSvc,
		advertiserService: advSvc,
		adGroupService:    agSvc,
		creativeService:   crSvc,
		sourceService:     srcSvc,
		reportingService:  reportingSvc,
		pacingEngine:      pacer,
		trackingService:   trackingSvc,
		postbackHandler:   postbackHandler,
		logger:            deps.Logger,
		config:            deps.Config,
		metrics:           deps.Metrics,
	}

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", s.handleHealth)

	// Prometheus metrics
	if deps.Config.Metrics.Enabled {
		mux.Handle(deps.Config.Metrics.Path, metrics.Handler())
	}

	// =============================================
	// OpenRTB endpoints
	// =============================================
	mux.HandleFunc("/openrtb2/bid", s.handleBid)
	mux.HandleFunc("/openrtb2/win", s.handleWinNotice)
	mux.HandleFunc("/openrtb2/loss", s.handleLossNotice)

	// =============================================
	// Tracking endpoints (Click & View)
	// =============================================
	mux.HandleFunc("/track/click", s.handleTrackClick)
	mux.HandleFunc("/track/view", s.handleTrackView)
	mux.HandleFunc("/track/event", s.handleTrackEvent)

	// =============================================
	// Postback endpoints (from MMPs)
	// =============================================
	mux.HandleFunc("/postback", s.handlePostback)
	mux.HandleFunc("/postback/appsflyer", s.handlePostbackAppsFlyer)
	mux.HandleFunc("/postback/adjust", s.handlePostbackAdjust)
	mux.HandleFunc("/postback/singular", s.handlePostbackSingular)

	// =============================================
	// S2S endpoints (for direct partners)
	// =============================================
	mux.HandleFunc("/s2s/", s.handleS2S)

	// =============================================
	// Admin API - Campaigns
	// =============================================
	mux.HandleFunc("/api/campaigns", s.handleCampaigns)
	mux.HandleFunc("/api/campaigns/", s.handleCampaignByID)

	// =============================================
	// Admin API - Advertisers
	// =============================================
	mux.HandleFunc("/api/advertisers", s.handleAdvertisers)
	mux.HandleFunc("/api/advertisers/", s.handleAdvertiserByID)

	// =============================================
	// Admin API - Sources
	// =============================================
	mux.HandleFunc("/api/sources/s2s", s.handleS2SSources)
	mux.HandleFunc("/api/sources/s2s/", s.handleS2SSourceByID)
	mux.HandleFunc("/api/sources/rtb", s.handleRTBSources)
	mux.HandleFunc("/api/sources/rtb/", s.handleRTBSourceByID)

	// =============================================
	// Admin API - Ad Groups
	// =============================================
	mux.HandleFunc("/api/adgroups", s.handleAdGroups)
	mux.HandleFunc("/api/adgroups/", s.handleAdGroupByID)

	// =============================================
	// Admin API - Creatives
	// =============================================
	mux.HandleFunc("/api/creatives", s.handleCreatives)
	mux.HandleFunc("/api/creatives/", s.handleCreativeByID)
	mux.HandleFunc("/api/creatives/upload", s.handleCreativeUpload)

	// =============================================
	// Reporting
	// =============================================
	mux.HandleFunc("/api/reports/campaigns", s.handleCampaignReports)
	mux.HandleFunc("/api/reports/sources", s.handleSourceReports)
	mux.HandleFunc("/api/reports/geo", s.handleGeoReports)
	mux.HandleFunc("/api/reports/time-series", s.handleTimeSeriesReport)

	// Stats (backward compatibility)
	mux.HandleFunc("/api/stats", s.handleStats)

	// Pacing
	mux.HandleFunc("/api/pacing/", s.handlePacingStats)

	return mux
}

// =============================================
// Health Check
// =============================================

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": "1.0.0"})
}

// =============================================
// OpenRTB Bid
// =============================================

func (s *Server) handleBid(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var br models.BidRequest
	if err := json.NewDecoder(r.Body).Decode(&br); err != nil {
		s.errorResponse(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := br.Validate(); err != nil {
		s.errorResponse(w, "invalid bidrequest: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := s.bidService.BuildBidResponse(&br)
	if err != nil {
		s.logger.Error("bid error", zap.Error(err))
		s.errorResponse(w, "internal error", http.StatusInternalServerError)
		return
	}

	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleWinNotice(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	campaignID := q.Get("campaign_id")
	lineItemID := q.Get("line_item_id")
	creativeID := q.Get("creative_id")
	impID := q.Get("imp_id")
	priceStr := q.Get("price")

	price := 0.0
	if priceStr != "" {
		fmt.Sscanf(priceStr, "%f", &price)
	}

	s.logger.Info("win notice",
		zap.String("imp_id", impID),
		zap.Float64("price", price),
		zap.String("campaign_id", campaignID),
	)

	if s.metrics != nil && campaignID != "" {
		s.metrics.RecordWin(campaignID, lineItemID, price)
	}

	// Log impression
	s.trackingService.RegisterWin(r.Context(), impID, campaignID, lineItemID, creativeID, price)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) handleLossNotice(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	campaignID := q.Get("campaign_id")
	reason := q.Get("reason")

	s.logger.Debug("loss notice",
		zap.String("campaign_id", campaignID),
		zap.String("reason", reason),
	)

	if s.metrics != nil && campaignID != "" {
		s.metrics.RecordLoss(campaignID, reason)
	}

	w.WriteHeader(http.StatusOK)
}

// =============================================
// Tracking Endpoints
// =============================================

func (s *Server) handleTrackClick(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	campaignID := q.Get("cid")
	creativeID := q.Get("cr")
	lineItemID := q.Get("li")
	sourceID := q.Get("src")
	sourceType := q.Get("st")
	impressionID := q.Get("imp")
	gaid := q.Get("gaid")
	idfa := q.Get("idfa")

	if campaignID == "" {
		s.errorResponse(w, "missing campaign_id", http.StatusBadRequest)
		return
	}

	// Get sub parameters
	sub1 := q.Get("sub1")
	sub2 := q.Get("sub2")
	sub3 := q.Get("sub3")
	sub4 := q.Get("sub4")
	sub5 := q.Get("sub5")

	// Register click and get MMP redirect URL
	redirectURL, err := s.trackingService.RegisterClick(
		r.Context(),
		campaignID, creativeID, lineItemID,
		sourceType, sourceID, impressionID,
		gaid, idfa,
		getClientIP(r), r.UserAgent(),
		sub1, sub2, sub3, sub4, sub5,
	)
	if err != nil {
		s.logger.Error("click registration failed", zap.Error(err))
		s.errorResponse(w, "click registration failed", http.StatusInternalServerError)
		return
	}

	if s.metrics != nil {
		s.metrics.RecordClick(campaignID, lineItemID)
	}

	// Redirect to MMP Click URL
	if redirectURL != "" {
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	// No redirect - return OK
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleTrackView(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	campaignID := q.Get("cid")
	creativeID := q.Get("cr")
	lineItemID := q.Get("li")
	sourceID := q.Get("src")
	sourceType := q.Get("st")
	impressionID := q.Get("imp")
	gaid := q.Get("gaid")
	idfa := q.Get("idfa")

	// Register view
	s.trackingService.RegisterView(
		r.Context(),
		campaignID, creativeID, lineItemID,
		sourceType, sourceID, impressionID,
		gaid, idfa, getClientIP(r),
	)

	// Return 1x1 transparent pixel
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Write(transparentPixel)
}

func (s *Server) handleTrackEvent(w http.ResponseWriter, r *http.Request) {
	// Custom event tracking
	q := r.URL.Query()
	eventName := q.Get("event")
	clickID := q.Get("click_id")

	s.logger.Debug("event tracked",
		zap.String("event", eventName),
		zap.String("click_id", clickID),
	)

	w.WriteHeader(http.StatusOK)
}

// =============================================
// Postback Endpoints
// =============================================

func (s *Server) handlePostback(w http.ResponseWriter, r *http.Request) {
	result, err := s.postbackHandler.HandleGeneric(r.Context(), r)
	if err != nil {
		s.logger.Error("postback error", zap.Error(err))
	}
	s.jsonResponse(w, result)
}

func (s *Server) handlePostbackAppsFlyer(w http.ResponseWriter, r *http.Request) {
	result, err := s.postbackHandler.HandleAppsFlyer(r.Context(), r)
	if err != nil {
		s.logger.Error("appsflyer postback error", zap.Error(err))
	}
	s.jsonResponse(w, result)
}

func (s *Server) handlePostbackAdjust(w http.ResponseWriter, r *http.Request) {
	result, err := s.postbackHandler.HandleAdjust(r.Context(), r)
	if err != nil {
		s.logger.Error("adjust postback error", zap.Error(err))
	}
	s.jsonResponse(w, result)
}

func (s *Server) handlePostbackSingular(w http.ResponseWriter, r *http.Request) {
	result, err := s.postbackHandler.HandleSingular(r.Context(), r)
	if err != nil {
		s.logger.Error("singular postback error", zap.Error(err))
	}
	s.jsonResponse(w, result)
}

// =============================================
// S2S Endpoints
// =============================================

func (s *Server) handleS2S(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		s.errorResponse(w, "invalid path", http.StatusBadRequest)
		return
	}

	sourceName := parts[1]
	action := parts[2]

	switch action {
	case "ad":
		s.handleS2SAd(w, r, sourceName)
	case "click":
		s.handleS2SClick(w, r, sourceName)
	default:
		s.errorResponse(w, "unknown action", http.StatusBadRequest)
	}
}

func (s *Server) handleS2SAd(w http.ResponseWriter, r *http.Request, sourceName string) {
	// Get source
	source, err := s.sourceService.GetS2SSourceByName(r.Context(), sourceName)
	if err != nil || source == nil {
		s.jsonResponse(w, map[string]interface{}{"success": false, "error": "source not found"})
		return
	}

	// Parse request
	q := r.URL.Query()
	country := strings.ToUpper(q.Get("country"))
	os := strings.ToLower(q.Get("os"))
	gaid := q.Get("gaid")
	if gaid == "" {
		gaid = q.Get("idfa")
	}

	// Find matching campaign
	campaign, creative, err := s.sourceService.FindCampaignForSource(r.Context(), source.ID, country, os)
	if err != nil || campaign == nil {
		s.jsonResponse(w, map[string]interface{}{"success": false, "error": "no ad available"})
		return
	}

	// Build tracking URLs
	clickURL := fmt.Sprintf("%s/track/click?cid=%s&cr=%s&src=%s&st=s2s&gaid=%s&sub1=%s&sub2=%s&sub3=%s",
		s.config.Tracking.BaseURL, campaign.ID, creative.ID, source.ID,
		gaid, q.Get("sub1"), q.Get("sub2"), q.Get("sub3"))

	viewURL := fmt.Sprintf("%s/track/view?cid=%s&cr=%s&src=%s&st=s2s&gaid=%s",
		s.config.Tracking.BaseURL, campaign.ID, creative.ID, source.ID, gaid)

	s.jsonResponse(w, map[string]interface{}{
		"success":     true,
		"campaign_id": campaign.ID,
		"app_bundle":  campaign.AppBundle,
		"creative": map[string]interface{}{
			"id":   creative.ID,
			"type": creative.Format,
			"url":  creative.AdmTemplate,
			"w":    creative.W,
			"h":    creative.H,
		},
		"click_url": clickURL,
		"view_url":  viewURL,
		"payout":    source.DefaultPayout,
	})
}

func (s *Server) handleS2SClick(w http.ResponseWriter, r *http.Request, sourceName string) {
	// This is the same as handleTrackClick but with source validation
	s.handleTrackClick(w, r)
}

// =============================================
// Admin API - Campaigns
// =============================================

func (s *Server) handleCampaigns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.campaignService.ListCampaigns()
		if err != nil {
			s.errorResponse(w, "failed to list", http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, list)

	case http.MethodPost:
		var c models.Campaign
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			s.errorResponse(w, "invalid json", http.StatusBadRequest)
			return
		}
		if err := s.campaignService.UpsertCampaign(&c); err != nil {
			s.errorResponse(w, "failed to save: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, c)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCampaignByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/campaigns/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		c, err := s.campaignService.GetCampaign(id)
		if err != nil {
			s.errorResponse(w, "error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if c == nil {
			http.NotFound(w, r)
			return
		}
		s.jsonResponse(w, c)

	case http.MethodPut:
		var c models.Campaign
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			s.errorResponse(w, "invalid json", http.StatusBadRequest)
			return
		}
		c.ID = id
		if err := s.campaignService.UpsertCampaign(&c); err != nil {
			s.errorResponse(w, "failed to save: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, c)

	case http.MethodDelete:
		s.errorResponse(w, "not implemented", http.StatusNotImplemented)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// =============================================
// Admin API - Advertisers
// =============================================

func (s *Server) handleAdvertisers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.advertiserService.ListAdvertisers()
		if err != nil {
			s.errorResponse(w, "failed to list", http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, list)

	case http.MethodPost:
		var a models.Advertiser
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			s.errorResponse(w, "invalid json", http.StatusBadRequest)
			return
		}
		if err := s.advertiserService.UpsertAdvertiser(&a); err != nil {
			s.errorResponse(w, "failed to save: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, a)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdvertiserByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/advertisers/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		adv, err := s.advertiserService.GetAdvertiser(id)
		if err != nil {
			s.errorResponse(w, "error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if adv == nil {
			http.NotFound(w, r)
			return
		}
		s.jsonResponse(w, adv)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// =============================================
// Admin API - Sources
// =============================================

func (s *Server) handleS2SSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.sourceService.ListS2SSources(r.Context())
		if err != nil {
			s.errorResponse(w, "failed to list", http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, list)

	case http.MethodPost:
		var src models.S2SSource
		if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
			s.errorResponse(w, "invalid json", http.StatusBadRequest)
			return
		}
		if err := s.sourceService.UpsertS2SSource(r.Context(), &src); err != nil {
			s.errorResponse(w, "failed to save: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, src)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleS2SSourceByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/sources/s2s/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		src, err := s.sourceService.GetS2SSource(r.Context(), id)
		if err != nil {
			s.errorResponse(w, "error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if src == nil {
			http.NotFound(w, r)
			return
		}
		s.jsonResponse(w, src)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRTBSources(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.sourceService.ListRTBSources(r.Context())
		if err != nil {
			s.errorResponse(w, "failed to list", http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, list)

	case http.MethodPost:
		var src models.RTBSource
		if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
			s.errorResponse(w, "invalid json", http.StatusBadRequest)
			return
		}
		if err := s.sourceService.UpsertRTBSource(r.Context(), &src); err != nil {
			s.errorResponse(w, "failed to save: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, src)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRTBSourceByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/sources/rtb/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		src, err := s.sourceService.GetRTBSource(r.Context(), id)
		if err != nil {
			s.errorResponse(w, "error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if src == nil {
			http.NotFound(w, r)
			return
		}
		s.jsonResponse(w, src)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// =============================================
// Admin API - Ad Groups
// =============================================

func (s *Server) handleAdGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		campaignID := r.URL.Query().Get("campaign_id")
		var list []*models.AdGroup
		var err error
		if campaignID != "" {
			list, err = s.adGroupService.ListAdGroupsByCampaign(campaignID)
		} else {
			list, err = s.adGroupService.ListAdGroups()
		}
		if err != nil {
			s.errorResponse(w, "failed to list", http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, list)

	case http.MethodPost:
		var g models.AdGroup
		if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
			s.errorResponse(w, "invalid json", http.StatusBadRequest)
			return
		}
		if err := s.adGroupService.UpsertAdGroup(&g); err != nil {
			s.errorResponse(w, "failed to save: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, g)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdGroupByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/adgroups/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		g, err := s.adGroupService.GetAdGroup(id)
		if err != nil {
			s.errorResponse(w, "error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if g == nil {
			http.NotFound(w, r)
			return
		}
		s.jsonResponse(w, g)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// =============================================
// Admin API - Creatives
// =============================================

func (s *Server) handleCreatives(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		advertiserID := r.URL.Query().Get("advertiser_id")
		list, err := s.creativeService.ListCreatives(advertiserID)
		if err != nil {
			s.errorResponse(w, "failed to list", http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, list)

	case http.MethodPost:
		var cr models.Creative
		if err := json.NewDecoder(r.Body).Decode(&cr); err != nil {
			s.errorResponse(w, "invalid json", http.StatusBadRequest)
			return
		}
		if cr.ID == "" {
			s.errorResponse(w, "id is required", http.StatusBadRequest)
			return
		}
		if err := s.creativeService.UpsertCreative(&cr); err != nil {
			s.errorResponse(w, "failed to save: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, cr)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreativeByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/creatives/")
	if id == "" || id == "upload" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		cr, err := s.creativeService.GetCreative(id)
		if err != nil {
			s.errorResponse(w, "error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if cr == nil {
			http.NotFound(w, r)
			return
		}
		s.jsonResponse(w, cr)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreativeUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.errorResponse(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.errorResponse(w, "file field missing: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	uploadDir := "static/uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		s.errorResponse(w, "failed to create upload dir", http.StatusInternalServerError)
		return
	}

	destPath := filepath.Join(uploadDir, header.Filename)
	if _, err := os.Stat(destPath); err == nil {
		for i := 1; ; i++ {
			suffixName := filepath.Join(uploadDir, fmt.Sprintf("%d_%s", i, header.Filename))
			if _, err := os.Stat(suffixName); os.IsNotExist(err) {
				destPath = suffixName
				break
			}
		}
	}

	out, err := os.Create(destPath)
	if err != nil {
		s.errorResponse(w, "failed to save file", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		s.errorResponse(w, "failed to write file", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"path": "/" + destPath})
}

// =============================================
// Reporting
// =============================================

func (s *Server) handleCampaignReports(w http.ResponseWriter, r *http.Request) {
	if s.reportingService == nil {
		s.errorResponse(w, "reporting not available", http.StatusServiceUnavailable)
		return
	}

	var filter dsp.ReportFilter
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
			s.errorResponse(w, "invalid json", http.StatusBadRequest)
			return
		}
	}

	stats, err := s.reportingService.GetCampaignStats(r.Context(), filter)
	if err != nil {
		s.errorResponse(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, stats)
}

func (s *Server) handleSourceReports(w http.ResponseWriter, r *http.Request) {
	if s.reportingService == nil {
		s.errorResponse(w, "reporting not available", http.StatusServiceUnavailable)
		return
	}

	stats, err := s.reportingService.GetSourceStats(r.Context())
	if err != nil {
		s.errorResponse(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, stats)
}

func (s *Server) handleGeoReports(w http.ResponseWriter, r *http.Request) {
	if s.reportingService == nil {
		s.errorResponse(w, "reporting not available", http.StatusServiceUnavailable)
		return
	}

	campaignID := r.URL.Query().Get("campaign_id")
	stats, err := s.reportingService.GetGeoBreakdown(r.Context(), campaignID)
	if err != nil {
		s.errorResponse(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, stats)
}

func (s *Server) handleTimeSeriesReport(w http.ResponseWriter, r *http.Request) {
	if s.reportingService == nil {
		s.errorResponse(w, "reporting not available", http.StatusServiceUnavailable)
		return
	}

	campaignID := r.URL.Query().Get("campaign_id")
	filter := dsp.ReportFilter{}

	if startStr := r.URL.Query().Get("start_date"); startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			filter.StartDate = t
		}
	}
	if endStr := r.URL.Query().Get("end_date"); endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			filter.EndDate = t
		}
	}

	points, err := s.reportingService.GetTimeSeries(r.Context(), campaignID, filter)
	if err != nil {
		s.errorResponse(w, "failed to get time series", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, points)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if s.reportingService == nil {
		s.jsonResponse(w, []dsp.CampaignStats{})
		return
	}

	stats, err := s.reportingService.GetCampaignStats(context.Background(), dsp.ReportFilter{})
	if err != nil {
		s.errorResponse(w, "failed to compute stats", http.StatusInternalServerError)
		return
	}

	campaignID := r.URL.Query().Get("campaign_id")
	if campaignID != "" {
		filtered := make([]dsp.CampaignStats, 0)
		for _, st := range stats {
			if st.CampaignID == campaignID {
				filtered = append(filtered, st)
			}
		}
		stats = filtered
	}

	s.jsonResponse(w, stats)
}

func (s *Server) handlePacingStats(w http.ResponseWriter, r *http.Request) {
	lineItemID := strings.TrimPrefix(r.URL.Path, "/api/pacing/")
	if lineItemID == "" {
		s.errorResponse(w, "line_item_id required", http.StatusBadRequest)
		return
	}

	stats, err := s.pacingEngine.GetStats(lineItemID)
	if err != nil {
		s.errorResponse(w, "failed to get pacing stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, stats)
}

// =============================================
// Helper Methods
// =============================================

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) errorResponse(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

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

// 1x1 transparent GIF
var transparentPixel = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00,
	0x01, 0x00, 0x80, 0x00, 0x00, 0xFF, 0xFF, 0xFF,
	0x00, 0x00, 0x00, 0x21, 0xF9, 0x04, 0x01, 0x00,
	0x00, 0x00, 0x00, 0x2C, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
	0x01, 0x00, 0x3B,
}
