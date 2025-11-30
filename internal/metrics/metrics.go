package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the DSP.
type Metrics struct {
	// Bid metrics
	BidRequests      *prometheus.CounterVec
	BidResponses     *prometheus.CounterVec
	BidLatency       *prometheus.HistogramVec
	BidPrice         *prometheus.HistogramVec
	NoBidReasons     *prometheus.CounterVec

	// Impression/Win metrics
	Impressions      *prometheus.CounterVec
	Wins             *prometheus.CounterVec
	Losses           *prometheus.CounterVec
	WinRate          *prometheus.GaugeVec

	// Spend metrics
	Spend            *prometheus.CounterVec
	DailyBudgetUsage *prometheus.GaugeVec

	// Click/Conversion metrics
	Clicks           *prometheus.CounterVec
	Conversions      *prometheus.CounterVec
	Revenue          *prometheus.CounterVec

	// System metrics
	ActiveCampaigns  prometheus.Gauge
	ActiveLineItems  prometheus.Gauge
	DBConnections    *prometheus.GaugeVec
	RedisLatency     *prometheus.HistogramVec

	// Rate limiting metrics
	RateLimitHits    *prometheus.CounterVec

	// Pacing metrics
	PacingRejections *prometheus.CounterVec
	FreqCapRejections *prometheus.CounterVec

	// Targeting metrics
	TargetingMatches   *prometheus.CounterVec
	TargetingMisses    *prometheus.CounterVec
	GeoLookupLatency   *prometheus.HistogramVec
}

var (
	// DefaultMetrics is the global metrics instance
	DefaultMetrics *Metrics
)

// NewMetrics creates and registers all Prometheus metrics.
func NewMetrics(namespace string) *Metrics {
	m := &Metrics{
		// Bid metrics
		BidRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "bid_requests_total",
				Help:      "Total number of bid requests received",
			},
			[]string{"source", "device_type"},
		),
		BidResponses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "bid_responses_total",
				Help:      "Total number of bid responses sent",
			},
			[]string{"status", "campaign_id"},
		),
		BidLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "bid_latency_seconds",
				Help:      "Bid processing latency in seconds",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
			},
			[]string{"status"},
		),
		BidPrice: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "bid_price_dollars",
				Help:      "Bid prices in dollars",
				Buckets:   []float64{0.0001, 0.001, 0.01, 0.1, 0.5, 1, 5, 10},
			},
			[]string{"campaign_id", "line_item_id"},
		),
		NoBidReasons: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "nobid_reasons_total",
				Help:      "Reasons for not bidding",
			},
			[]string{"reason"},
		),

		// Impression/Win metrics
		Impressions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "impressions_total",
				Help:      "Total impressions (wins)",
			},
			[]string{"campaign_id", "line_item_id"},
		),
		Wins: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "wins_total",
				Help:      "Total auction wins",
			},
			[]string{"campaign_id"},
		),
		Losses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "losses_total",
				Help:      "Total auction losses",
			},
			[]string{"campaign_id", "reason"},
		),
		WinRate: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "win_rate",
				Help:      "Current win rate",
			},
			[]string{"campaign_id"},
		),

		// Spend metrics
		Spend: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "spend_dollars_total",
				Help:      "Total spend in dollars",
			},
			[]string{"campaign_id", "line_item_id"},
		),
		DailyBudgetUsage: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "daily_budget_usage_percent",
				Help:      "Daily budget usage percentage",
			},
			[]string{"line_item_id"},
		),

		// Click/Conversion metrics
		Clicks: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "clicks_total",
				Help:      "Total clicks",
			},
			[]string{"campaign_id", "line_item_id"},
		),
		Conversions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "conversions_total",
				Help:      "Total conversions",
			},
			[]string{"campaign_id", "event_name"},
		),
		Revenue: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "revenue_dollars_total",
				Help:      "Total revenue from conversions",
			},
			[]string{"campaign_id"},
		),

		// System metrics
		ActiveCampaigns: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_campaigns",
				Help:      "Number of active campaigns",
			},
		),
		ActiveLineItems: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "active_line_items",
				Help:      "Number of active line items",
			},
		),
		DBConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "db_connections",
				Help:      "Database connection pool stats",
			},
			[]string{"state"}, // idle, in_use, total
		),
		RedisLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "redis_latency_seconds",
				Help:      "Redis operation latency",
				Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05},
			},
			[]string{"operation"},
		),

		// Rate limiting metrics
		RateLimitHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "rate_limit_hits_total",
				Help:      "Rate limit rejections",
			},
			[]string{"endpoint", "ip"},
		),

		// Pacing metrics
		PacingRejections: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "pacing_rejections_total",
				Help:      "Bid rejections due to pacing/budget",
			},
			[]string{"line_item_id", "reason"},
		),
		FreqCapRejections: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "freq_cap_rejections_total",
				Help:      "Bid rejections due to frequency cap",
			},
			[]string{"line_item_id"},
		),

		// Targeting metrics
		TargetingMatches: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "targeting_matches_total",
				Help:      "Successful targeting matches",
			},
			[]string{"line_item_id", "targeting_type"},
		),
		TargetingMisses: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "targeting_misses_total",
				Help:      "Targeting misses by type",
			},
			[]string{"line_item_id", "targeting_type"},
		),
		GeoLookupLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "geo_lookup_latency_seconds",
				Help:      "GeoIP lookup latency",
				Buckets:   []float64{0.00001, 0.0001, 0.001, 0.01},
			},
			[]string{"cache_hit"},
		),
	}

	DefaultMetrics = m
	return m
}

// Handler returns the Prometheus metrics HTTP handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordBidRequest records a bid request.
func (m *Metrics) RecordBidRequest(source string, deviceType int32) {
	m.BidRequests.WithLabelValues(source, strconv.Itoa(int(deviceType))).Inc()
}

// RecordBidResponse records a bid response.
func (m *Metrics) RecordBidResponse(status string, campaignID string, latency time.Duration) {
	m.BidResponses.WithLabelValues(status, campaignID).Inc()
	m.BidLatency.WithLabelValues(status).Observe(latency.Seconds())
}

// RecordBid records a bid price.
func (m *Metrics) RecordBid(campaignID, lineItemID string, price float64) {
	m.BidPrice.WithLabelValues(campaignID, lineItemID).Observe(price)
}

// RecordNoBid records a no-bid reason.
func (m *Metrics) RecordNoBid(reason string) {
	m.NoBidReasons.WithLabelValues(reason).Inc()
}

// RecordWin records an auction win.
func (m *Metrics) RecordWin(campaignID, lineItemID string, price float64) {
	m.Wins.WithLabelValues(campaignID).Inc()
	m.Impressions.WithLabelValues(campaignID, lineItemID).Inc()
	m.Spend.WithLabelValues(campaignID, lineItemID).Add(price)
}

// RecordLoss records an auction loss.
func (m *Metrics) RecordLoss(campaignID, reason string) {
	m.Losses.WithLabelValues(campaignID, reason).Inc()
}

// RecordClick records a click.
func (m *Metrics) RecordClick(campaignID, lineItemID string) {
	m.Clicks.WithLabelValues(campaignID, lineItemID).Inc()
}

// RecordConversion records a conversion.
func (m *Metrics) RecordConversion(campaignID, eventName string, revenue float64) {
	m.Conversions.WithLabelValues(campaignID, eventName).Inc()
	if revenue > 0 {
		m.Revenue.WithLabelValues(campaignID).Add(revenue)
	}
}

// RecordPacingRejection records a pacing rejection.
func (m *Metrics) RecordPacingRejection(lineItemID, reason string) {
	m.PacingRejections.WithLabelValues(lineItemID, reason).Inc()
}

// RecordFreqCapRejection records a frequency cap rejection.
func (m *Metrics) RecordFreqCapRejection(lineItemID string) {
	m.FreqCapRejections.WithLabelValues(lineItemID).Inc()
}

// RecordTargetingMatch records a targeting match.
func (m *Metrics) RecordTargetingMatch(lineItemID, targetingType string) {
	m.TargetingMatches.WithLabelValues(lineItemID, targetingType).Inc()
}

// RecordTargetingMiss records a targeting miss.
func (m *Metrics) RecordTargetingMiss(lineItemID, targetingType string) {
	m.TargetingMisses.WithLabelValues(lineItemID, targetingType).Inc()
}

// RecordGeoLookup records a geo lookup.
func (m *Metrics) RecordGeoLookup(cacheHit bool, latency time.Duration) {
	hit := "false"
	if cacheHit {
		hit = "true"
	}
	m.GeoLookupLatency.WithLabelValues(hit).Observe(latency.Seconds())
}

// UpdateDBStats updates database connection metrics.
func (m *Metrics) UpdateDBStats(idle, inUse, total int) {
	m.DBConnections.WithLabelValues("idle").Set(float64(idle))
	m.DBConnections.WithLabelValues("in_use").Set(float64(inUse))
	m.DBConnections.WithLabelValues("total").Set(float64(total))
}

// UpdateActiveCounts updates active campaign/line item counts.
func (m *Metrics) UpdateActiveCounts(campaigns, lineItems int) {
	m.ActiveCampaigns.Set(float64(campaigns))
	m.ActiveLineItems.Set(float64(lineItems))
}

// RecordRateLimitHit records a rate limit hit.
func (m *Metrics) RecordRateLimitHit(endpoint, ip string) {
	m.RateLimitHits.WithLabelValues(endpoint, ip).Inc()
}
