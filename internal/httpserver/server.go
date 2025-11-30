package httpserver

import (
    "encoding/json"
    "io"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "fmt"

    "github.com/radiusdt/vector-dsp/internal/dsp"
    "github.com/radiusdt/vector-dsp/internal/models"
    "github.com/radiusdt/vector-dsp/internal/storage"
)

// Server wraps HTTP handlers and DSP services.  It aggregates all services
// needed to serve OpenRTB auctions, campaign management, event tracking and
// advertiser/ad group management.  For production use, replace the in-memory
// repositories with persistent implementations.
type Server struct {
    campaignService   *dsp.CampaignService
    bidService        *dsp.BidService
    eventService      *dsp.EventService
    advertiserService *dsp.AdvertiserService
    adGroupService    *dsp.AdGroupService
    creativeService   *dsp.CreativeService
    statsService      *dsp.StatsService
}

// NewServer constructs a new http.Handler with all routes registered.  It
// instantiates in-memory repos and services.
func NewServer() http.Handler {
    // Campaign and pacing repos
    cRepo := storage.NewInMemoryCampaignRepo()
    eventStore := storage.NewInMemoryEventStore()
    pacer := dsp.NewInMemoryPacingEngine()
    // Advertiser, adgroup and creative repos
    advRepo := storage.NewInMemoryAdvertiserRepo()
    agRepo := storage.NewInMemoryAdGroupRepo()
    crRepo := storage.NewInMemoryCreativeRepo()

    // Services
    cSvc := dsp.NewCampaignService(cRepo)
    bSvc := dsp.NewBidService(cRepo, pacer)
    eSvc := dsp.NewEventService(eventStore)
    advSvc := dsp.NewAdvertiserService(advRepo)
    agSvc := dsp.NewAdGroupService(agRepo)
    crSvc := dsp.NewCreativeService(crRepo)
    statsSvc := dsp.NewStatsService(eventStore)

    s := &Server{
        campaignService:   cSvc,
        bidService:        bSvc,
        eventService:      eSvc,
        advertiserService: advSvc,
        adGroupService:    agSvc,
        creativeService:   crSvc,
        statsService:      statsSvc,
    }

    mux := http.NewServeMux()
    // Bidding and campaign endpoints
    mux.HandleFunc("/openrtb2/bid", s.handleBid)
    // Win/Loss notifications capture billing and analytics signals
    mux.HandleFunc("/openrtb2/win", s.handleWinNotice)
    mux.HandleFunc("/openrtb2/loss", s.handleLossNotice)
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
    // Endpoint for uploading creative assets (images/videos)
    mux.HandleFunc("/creatives/upload", s.handleCreativeUpload)
    // Stats endpoint
    mux.HandleFunc("/stats", s.handleStats)
    // Event endpoints
    mux.HandleFunc("/events/click", s.handleClick)
    mux.HandleFunc("/events/s2s/conversion", s.handleConversion)
    // Health
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    })

    return loggingMiddleware(mux)
}

// loggingMiddleware logs each request method, path and remote address.
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
        next.ServeHTTP(w, r)
    })
}

// ---- OpenRTB Bid ----

func (s *Server) handleBid(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    var br models.BidRequest
    if err := json.NewDecoder(r.Body).Decode(&br); err != nil {
        http.Error(w, "invalid json", http.StatusBadRequest)
        return
    }
    if err := br.Validate(); err != nil {
        http.Error(w, "invalid bidrequest: "+err.Error(), http.StatusBadRequest)
        return
    }
    resp, err := s.bidService.BuildBidResponse(&br)
    if err != nil {
        log.Printf("bid error: %v", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    if resp == nil {
        w.WriteHeader(http.StatusNoContent)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}

// ---- Campaigns CRUD ----

func (s *Server) handleCampaigns(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        list, err := s.campaignService.ListCampaigns()
        if err != nil {
            http.Error(w, "failed to list", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(list)
    case http.MethodPost:
        var c models.Campaign
        if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
            http.Error(w, "invalid json", http.StatusBadRequest)
            return
        }
        if err := s.campaignService.UpsertCampaign(&c); err != nil {
            http.Error(w, "failed to save: "+err.Error(), http.StatusBadRequest)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(c)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

func (s *Server) handleCampaignByID(w http.ResponseWriter, r *http.Request) {
    if !strings.HasPrefix(r.URL.Path, "/campaigns/") {
        http.NotFound(w, r)
        return
    }
    id := strings.TrimPrefix(r.URL.Path, "/campaigns/")
    switch r.Method {
    case http.MethodGet:
        c, err := s.campaignService.GetCampaign(id)
        if err != nil {
            http.Error(w, "error: "+err.Error(), http.StatusInternalServerError)
            return
        }
        if c == nil {
            http.NotFound(w, r)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(c)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

// ---- Advertisers CRUD ----

func (s *Server) handleAdvertisers(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        list, err := s.advertiserService.ListAdvertisers()
        if err != nil {
            http.Error(w, "failed to list", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(list)
    case http.MethodPost:
        var a models.Advertiser
        if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
            http.Error(w, "invalid json", http.StatusBadRequest)
            return
        }
        if err := s.advertiserService.UpsertAdvertiser(&a); err != nil {
            http.Error(w, "failed to save: "+err.Error(), http.StatusBadRequest)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(a)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

func (s *Server) handleAdvertiserByID(w http.ResponseWriter, r *http.Request) {
    if !strings.HasPrefix(r.URL.Path, "/advertisers/") {
        http.NotFound(w, r)
        return
    }
    id := strings.TrimPrefix(r.URL.Path, "/advertisers/")
    switch r.Method {
    case http.MethodGet:
        adv, err := s.advertiserService.GetAdvertiser(id)
        if err != nil {
            http.Error(w, "error: "+err.Error(), http.StatusInternalServerError)
            return
        }
        if adv == nil {
            http.NotFound(w, r)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(adv)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

// ---- Ad Groups CRUD ----

func (s *Server) handleAdGroups(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        // Optional query parameter campaign_id to filter ad groups by campaign.
        campaignID := r.URL.Query().Get("campaign_id")
        var list []*models.AdGroup
        var err error
        if campaignID != "" {
            list, err = s.adGroupService.ListAdGroupsByCampaign(campaignID)
        } else {
            list, err = s.adGroupService.ListAdGroups()
        }
        if err != nil {
            http.Error(w, "failed to list", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(list)
    case http.MethodPost:
        var g models.AdGroup
        if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
            http.Error(w, "invalid json", http.StatusBadRequest)
            return
        }
        if err := s.adGroupService.UpsertAdGroup(&g); err != nil {
            http.Error(w, "failed to save: "+err.Error(), http.StatusBadRequest)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(g)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

func (s *Server) handleAdGroupByID(w http.ResponseWriter, r *http.Request) {
    if !strings.HasPrefix(r.URL.Path, "/adgroups/") {
        http.NotFound(w, r)
        return
    }
    id := strings.TrimPrefix(r.URL.Path, "/adgroups/")
    switch r.Method {
    case http.MethodGet:
        g, err := s.adGroupService.GetAdGroup(id)
        if err != nil {
            http.Error(w, "error: "+err.Error(), http.StatusInternalServerError)
            return
        }
        if g == nil {
            http.NotFound(w, r)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(g)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

// ---- Creatives CRUD ----

// handleCreatives responds to GET and POST requests for creative resources.
// A GET request returns all creatives or those filtered by advertiser_id.
// A POST request inserts or updates a creative.  Creative IDs must be
// unique and supplied by the caller.
func (s *Server) handleCreatives(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        advertiserID := r.URL.Query().Get("advertiser_id")
        list, err := s.creativeService.ListCreatives(advertiserID)
        if err != nil {
            http.Error(w, "failed to list", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(list)
    case http.MethodPost:
        var cr models.Creative
        if err := json.NewDecoder(r.Body).Decode(&cr); err != nil {
            http.Error(w, "invalid json", http.StatusBadRequest)
            return
        }
        if cr.ID == "" {
            http.Error(w, "id is required", http.StatusBadRequest)
            return
        }
        if err := s.creativeService.UpsertCreative(&cr); err != nil {
            http.Error(w, "failed to save: "+err.Error(), http.StatusBadRequest)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(cr)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

// handleCreativeByID responds to GET requests for a single creative by ID.
func (s *Server) handleCreativeByID(w http.ResponseWriter, r *http.Request) {
    if !strings.HasPrefix(r.URL.Path, "/creatives/") {
        http.NotFound(w, r)
        return
    }
    id := strings.TrimPrefix(r.URL.Path, "/creatives/")
    switch r.Method {
    case http.MethodGet:
        cr, err := s.creativeService.GetCreative(id)
        if err != nil {
            http.Error(w, "error: "+err.Error(), http.StatusInternalServerError)
            return
        }
        if cr == nil {
            http.NotFound(w, r)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(cr)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

// ---- Statistics ----

// handleStats aggregates statistics across campaigns.  It returns a JSON
// array where each element contains a campaign ID and counts of
// clicks, conversions and revenue.  Optional query parameter
// `campaign_id` may filter the result to a single campaign.  Only GET
// method is supported.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    stats, err := s.statsService.AggregateByCampaign()
    if err != nil {
        http.Error(w, "failed to compute stats", http.StatusInternalServerError)
        return
    }
    // Optional filter by campaign_id
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
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(stats)
}

// ---- Upload Creative Assets ----

// handleCreativeUpload handles uploading of image or video files used by
// creatives.  It expects a multipart/form-data POST request with the
// file field named "file".  The uploaded file is saved into the
// ./static/uploads directory relative to the working directory.  A JSON
// response is returned with the relative path of the uploaded file.
func (s *Server) handleCreativeUpload(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    // Parse up to 32MB of uploaded data in memory.  Larger files will
    // spill to disk automatically.
    if err := r.ParseMultipartForm(32 << 20); err != nil {
        http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
        return
    }
    file, header, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "file field missing: "+err.Error(), http.StatusBadRequest)
        return
    }
    defer file.Close()
    // Create uploads directory if necessary
    uploadDir := "static/uploads"
    if err := os.MkdirAll(uploadDir, 0755); err != nil {
        http.Error(w, "failed to create upload dir", http.StatusInternalServerError)
        return
    }
    // Create destination file with a timestamp-based name to avoid collisions
    baseName := header.Filename
    destPath := filepath.Join(uploadDir, baseName)
    // If file exists, append a numeric suffix
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
        http.Error(w, "failed to save file", http.StatusInternalServerError)
        return
    }
    defer out.Close()
    if _, err := io.Copy(out, file); err != nil {
        http.Error(w, "failed to write file", http.StatusInternalServerError)
        return
    }
    // Return relative path for client to reference
    relPath := "/" + destPath
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]string{"path": relPath})
}

// ---- Events / S2S ----

func (s *Server) handleClick(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    q := r.URL.Query()
    campaignID := q.Get("campaign_id")
    lineItemID := q.Get("line_item_id")
    targetURL := q.Get("target_url")
    userID := q.Get("user_id")
    if campaignID == "" || lineItemID == "" || targetURL == "" {
        http.Error(w, "missing required params", http.StatusBadRequest)
        return
    }
    clickID, redirectURL, err := s.eventService.RegisterClick(campaignID, lineItemID, userID, targetURL)
    if err != nil {
        http.Error(w, "failed to register click", http.StatusInternalServerError)
        return
    }
    log.Printf("registered click %s for campaign=%s lineItem=%s", clickID, campaignID, lineItemID)
    http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *Server) handleConversion(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost && r.Method != http.MethodGet {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
        http.Error(w, "click_id or external_id required", http.StatusBadRequest)
        return
    }
    if err := s.eventService.RegisterConversion(clickID, externalID, eventName, revenueStr, currency); err != nil {
        log.Printf("conversion error: %v", err)
    }
    w.Header().Set("Content-Type", "text/plain")
    _, _ = w.Write([]byte("OK"))
}

// ---- Win/Loss notifications ----

// handleWinNotice processes a win notification from the exchange.  A real
// DSP would use this to record the winning bid, decrement budgets and
// trigger billing.  This implementation simply logs the event.
func (s *Server) handleWinNotice(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet && r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    q := r.URL.Query()
    impID := q.Get("imp_id")
    price := q.Get("price")
    bidID := q.Get("bid_id")
    campaignID := q.Get("campaign_id")
    lineItemID := q.Get("line_item_id")
    log.Printf("win notice: imp_id=%s price=%s bid_id=%s campaign_id=%s line_item_id=%s", impID, price, bidID, campaignID, lineItemID)
    w.Header().Set("Content-Type", "text/plain")
    _, _ = w.Write([]byte("OK"))
}

// handleLossNotice processes a loss notification from the exchange.  It
// logs the event.  Additional logic could be added to track lost
// auctions or adjust strategies.
func (s *Server) handleLossNotice(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet && r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    q := r.URL.Query()
    impID := q.Get("imp_id")
    price := q.Get("price")
    bidID := q.Get("bid_id")
    campaignID := q.Get("campaign_id")
    lineItemID := q.Get("line_item_id")
    log.Printf("loss notice: imp_id=%s price=%s bid_id=%s campaign_id=%s line_item_id=%s", impID, price, bidID, campaignID, lineItemID)
    w.Header().Set("Content-Type", "text/plain")
    _, _ = w.Write([]byte("OK"))
}