package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	reportingService  *dsp.ReportingService
	pacingEngine      dsp.PacingEngine
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

	if deps.DB != nil {
		cRepo = storage.NewPostgresCampaignRepo(deps.DB.Pool)
		advRepo = storage.NewPostgresAdvertiserRepo(deps.DB.Pool)
		eventStore = storage.NewPostgresEventStore(deps.DB.Pool)
	} else {
		cRepo = storage.NewInMemoryCampaignRepo()
		advRepo = storage.NewInMemoryAdvertiserRepo()
		eventStore = storage.NewInMemoryEventStore()
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
	bSvc := dsp.NewBidService(cRepo, pacer, targetingEngine, deps.Metrics)
	eSvc := dsp.NewEventService(eventStore)
	advSvc := dsp.NewAdvertiserService(advRepo)
	agSvc := dsp.NewAdGroupService(agRepo)
	crSvc := dsp.NewCreativeService(crRepo)

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
		reportingService:  reportingSvc,
		pacingEngine:      pacer,
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

	// OpenRTB endpoints
	mux.HandleFunc("/openrtb2/bid", s.handleBid)
	mux.HandleFunc("/openrtb2/win", s.handleWinNotice)
	mux.HandleFunc("/openrtb2/loss", s.handleLossNotice)

	// Campaign management
	mux.HandleFunc("/campaigns", s.handleCampaigns)
	mux.HandleFunc("/campaigns/", s.handleCampaignByID)

	// Advertisers
	mux.HandleFunc("/advertisers", s.handleAdvertisers)
	mux.HandleFunc("/advertisers/", s.handleAdvertiserByID)

	// Ad groups
	mux.HandleFunc("/adgroups", s.handleAdGroups)
	mux.HandleFunc("/adgroups/", s.handleAdGroupByID)

	// Creatives
	mux.HandleFunc("/creatives", s.handleCreatives)
	mux.HandleFunc("/creatives/", s.handleCreativeByID)
	mux.HandleFunc("/creatives/upload", s.handleCreativeUpload)

	// Reporting
	mux.HandleFunc("/reports/campaigns", s.handleCampaignReports)
	mux.HandleFunc("/reports/line-items", s.handleLineItemReports)
	mux.HandleFunc("/reports/time-series", s.handleTimeSeriesReport)

	// Stats (backward compatibility)
	mux.HandleFunc("/stats", s.handleStats)

	// Pacing
	mux.HandleFunc("/pacing/", s.handlePacingStats)

	// Events
	mux.HandleFunc("/events/click", s.handleClick)
	mux.HandleFunc("/events/s2s/conversion", s.handleConversion)

	return mux
}

// ---- Health Check ----

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ---- OpenRTB Bid ----

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
	_ = json.NewEncoder(w).Encode(resp)
}

// ---- Win/Loss Notifications ----

func (s *Server) handleWinNotice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	campaignID := q.Get("campaign_id")
	lineItemID := q.Get("line_item_id")
	priceStr := q.Get("price")

	s.logger.Info("win notice",
		zap.String("imp_id", q.Get("imp_id")),
		zap.String("price", priceStr),
		zap.String("bid_id", q.Get("bid_id")),
		zap.String("campaign_id", campaignID),
		zap.String("line_item_id", lineItemID),
	)

	// Record win in metrics
	if s.metrics != nil && campaignID != "" {
		price := 0.0
		fmt.Sscanf(priceStr, "%f", &price)
		s.metrics.RecordWin(campaignID, lineItemID, price)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK"))
}

func (s *Server) handleLossNotice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	campaignID := q.Get("campaign_id")
	reason := q.Get("reason")

	s.logger.Debug("loss notice",
		zap.String("imp_id", q.Get("imp_id")),
		zap.String("price", q.Get("price")),
		zap.String("campaign_id", campaignID),
		zap.String("reason", reason),
	)

	if s.metrics != nil && campaignID != "" {
		s.metrics.RecordLoss(campaignID, reason)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK"))
}

// ---- Campaigns CRUD ----

func (s *Server) handleCampaigns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := s.campaignService.ListCampaigns()
		if err != nil {
			s.logger.Error("failed to list campaigns", zap.Error(err))
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
	id := strings.TrimPrefix(r.URL.Path, "/campaigns/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		c, err := s.campaignService.GetCampaign(id)
		if err != nil {
			s.logger.Error("failed to get campaign", zap.Error(err))
			s.errorResponse(w, "error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if c == nil {
			http.NotFound(w, r)
			return
		}
		s.jsonResponse(w, c)

	case http.MethodDelete:
		s.errorResponse(w, "not implemented", http.StatusNotImplemented)

	default:
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ---- Advertisers CRUD ----

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
	id := strings.TrimPrefix(r.URL.Path, "/advertisers/")
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

// ---- Ad Groups CRUD ----

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
	id := strings.TrimPrefix(r.URL.Path, "/adgroups/")
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

// ---- Creatives CRUD ----

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
	id := strings.TrimPrefix(r.URL.Path, "/creatives/")
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

	baseName := header.Filename
	destPath := filepath.Join(uploadDir, baseName)

	if _, err := os.Stat(destPath); err == nil {
		for i := 1; ; i++ {
			suffixName := filepath.Join(uploadDir, fmt.Sprintf("%d_%s", i, baseName))
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

// ---- Reporting ----

func (s *Server) handleCampaignReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
		s.logger.Error("failed to get campaign stats", zap.Error(err))
		s.errorResponse(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, stats)
}

func (s *Server) handleLineItemReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	stats, err := s.reportingService.GetLineItemStats(r.Context(), filter)
	if err != nil {
		s.logger.Error("failed to get line item stats", zap.Error(err))
		s.errorResponse(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, stats)
}

func (s *Server) handleTimeSeriesReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.reportingService == nil {
		s.errorResponse(w, "reporting not available", http.StatusServiceUnavailable)
		return
	}

	campaignID := r.URL.Query().Get("campaign_id")
	if campaignID == "" {
		s.errorResponse(w, "campaign_id required", http.StatusBadRequest)
		return
	}

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
		s.logger.Error("failed to get time series", zap.Error(err))
		s.errorResponse(w, "failed to get time series", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, points)
}

// ---- Stats (backward compatibility) ----

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

// ---- Pacing ----

func (s *Server) handlePacingStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	lineItemID := strings.TrimPrefix(r.URL.Path, "/pacing/")
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

// ---- Events ----

func (s *Server) handleClick(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	campaignID := q.Get("campaign_id")
	lineItemID := q.Get("line_item_id")
	targetURL := q.Get("target_url")
	userID := q.Get("user_id")

	if campaignID == "" || lineItemID == "" || targetURL == "" {
		s.errorResponse(w, "missing required params", http.StatusBadRequest)
		return
	}

	clickID, redirectURL, err := s.eventService.RegisterClick(campaignID, lineItemID, userID, targetURL)
	if err != nil {
		s.errorResponse(w, "failed to register click", http.StatusInternalServerError)
		return
	}

	s.logger.Info("click registered",
		zap.String("click_id", clickID),
		zap.String("campaign_id", campaignID),
	)

	if s.metrics != nil {
		s.metrics.RecordClick(campaignID, lineItemID)
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *Server) handleConversion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		s.errorResponse(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	clickID := q.Get("click_id")
	externalID := q.Get("external_id")
	eventName := q.Get("event_name")
	if eventName == "" {
		eventName = "install"
	}
	revenueStr := q.Get("revenue")
	currency := q.Get("currency")
	if currency == "" {
		currency = "USD"
	}

	if clickID == "" && externalID == "" {
		s.errorResponse(w, "click_id or external_id required", http.StatusBadRequest)
		return
	}

	if err := s.eventService.RegisterConversion(clickID, externalID, eventName, revenueStr, currency); err != nil {
		s.logger.Error("conversion error", zap.Error(err))
	}

	// Record conversion metric
	if s.metrics != nil {
		revenue := 0.0
		fmt.Sscanf(revenueStr, "%f", &revenue)
		// Would need to look up campaign from click
		s.metrics.RecordConversion("", eventName, revenue)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK"))
}

// ---- Helper Methods ----

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) errorResponse(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
