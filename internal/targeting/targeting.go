package targeting

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/radiusdt/vector-dsp/internal/metrics"
	"github.com/radiusdt/vector-dsp/internal/models"
)

// GeoInfo holds geographic information for an IP.
type GeoInfo struct {
	Country     string
	CountryCode string
	Region      string
	City        string
	PostalCode  string
	Latitude    float64
	Longitude   float64
	Timezone    string
	ISP         string
	ASN         uint
}

// GeoProvider interface for IP geolocation.
type GeoProvider interface {
	Lookup(ip string) (*GeoInfo, error)
	Close() error
}

// TargetingEngine performs advanced targeting checks.
type TargetingEngine struct {
	geoProvider GeoProvider
	geoCache    *geoCache
	metrics     *metrics.Metrics
}

// geoCache caches geo lookups.
type geoCache struct {
	mu       sync.RWMutex
	data     map[string]*geoCacheEntry
	maxSize  int
	ttl      time.Duration
}

type geoCacheEntry struct {
	info      *GeoInfo
	expiresAt time.Time
}

// NewTargetingEngine creates a new targeting engine.
func NewTargetingEngine(geoProvider GeoProvider, cacheSize int, cacheTTL time.Duration, m *metrics.Metrics) *TargetingEngine {
	return &TargetingEngine{
		geoProvider: geoProvider,
		geoCache: &geoCache{
			data:    make(map[string]*geoCacheEntry),
			maxSize: cacheSize,
			ttl:     cacheTTL,
		},
		metrics: m,
	}
}

// MatchResult contains detailed matching information.
type MatchResult struct {
	Matched        bool
	FailedCriteria string
	GeoInfo        *GeoInfo
}

// Match checks if a bid request matches line item targeting.
func (e *TargetingEngine) Match(br *models.BidRequest, imp *models.Imp, li *models.LineItem) *MatchResult {
	result := &MatchResult{Matched: true}
	targeting := &li.Targeting

	// Extract IP for geo targeting
	var ip string
	if br.Device != nil {
		if br.Device.IP != "" {
			ip = br.Device.IP
		} else if br.Device.IPv6 != "" {
			ip = br.Device.IPv6
		}
	}

	// Geo targeting (country)
	if len(targeting.Countries) > 0 {
		geoInfo := e.lookupGeo(ip)
		result.GeoInfo = geoInfo
		if geoInfo == nil || !e.matchCountry(geoInfo.CountryCode, targeting.Countries) {
			result.Matched = false
			result.FailedCriteria = "geo_country"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "geo_country")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "geo_country")
		}
	}

	// Geo targeting (region)
	if len(targeting.Regions) > 0 {
		geoInfo := result.GeoInfo
		if geoInfo == nil {
			geoInfo = e.lookupGeo(ip)
			result.GeoInfo = geoInfo
		}
		if geoInfo == nil || !e.matchRegion(geoInfo.Region, targeting.Regions) {
			result.Matched = false
			result.FailedCriteria = "geo_region"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "geo_region")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "geo_region")
		}
	}

	// Geo targeting (city)
	if len(targeting.Cities) > 0 {
		geoInfo := result.GeoInfo
		if geoInfo == nil {
			geoInfo = e.lookupGeo(ip)
			result.GeoInfo = geoInfo
		}
		if geoInfo == nil || !e.matchCity(geoInfo.City, targeting.Cities) {
			result.Matched = false
			result.FailedCriteria = "geo_city"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "geo_city")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "geo_city")
		}
	}

	// Domain targeting (whitelist)
	if len(targeting.SiteDomains) > 0 {
		var domain string
		if br.Site != nil {
			domain = br.Site.Domain
		}
		if !e.matchDomain(domain, targeting.SiteDomains) {
			result.Matched = false
			result.FailedCriteria = "domain_whitelist"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "domain_whitelist")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "domain_whitelist")
		}
	}

	// Domain blacklist
	if len(targeting.DomainBlacklist) > 0 {
		var domain string
		if br.Site != nil {
			domain = br.Site.Domain
		}
		if e.matchDomain(domain, targeting.DomainBlacklist) {
			result.Matched = false
			result.FailedCriteria = "domain_blacklist"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "domain_blacklist")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "domain_blacklist")
		}
	}

	// App bundle targeting
	if len(targeting.AppBundles) > 0 {
		var bundle string
		if br.App != nil {
			bundle = br.App.Bundle
		}
		if !e.matchBundle(bundle, targeting.AppBundles) {
			result.Matched = false
			result.FailedCriteria = "app_bundle"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "app_bundle")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "app_bundle")
		}
	}

	// App bundle blacklist
	if len(targeting.BundleBlacklist) > 0 {
		var bundle string
		if br.App != nil {
			bundle = br.App.Bundle
		}
		if e.matchBundle(bundle, targeting.BundleBlacklist) {
			result.Matched = false
			result.FailedCriteria = "bundle_blacklist"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "bundle_blacklist")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "bundle_blacklist")
		}
	}

	// Device type targeting
	if len(targeting.DeviceTypes) > 0 {
		dt := int32(0)
		if br.Device != nil {
			dt = br.Device.DeviceType
		}
		if !e.matchDeviceType(dt, targeting.DeviceTypes) {
			result.Matched = false
			result.FailedCriteria = "device_type"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "device_type")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "device_type")
		}
	}

	// OS targeting
	if len(targeting.OS) > 0 {
		osVal := ""
		if br.Device != nil {
			osVal = br.Device.OS
		}
		if !e.matchOS(osVal, targeting.OS) {
			result.Matched = false
			result.FailedCriteria = "os"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "os")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "os")
		}
	}

	// OS version targeting
	if targeting.OSVersionMin != "" || targeting.OSVersionMax != "" {
		osv := ""
		if br.Device != nil {
			osv = br.Device.OSV
		}
		if !e.matchOSVersion(osv, targeting.OSVersionMin, targeting.OSVersionMax) {
			result.Matched = false
			result.FailedCriteria = "os_version"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "os_version")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "os_version")
		}
	}

	// Category whitelist (IAB categories)
	if len(targeting.CatWhitelist) > 0 {
		var cats []string
		if br.Site != nil {
			cats = br.Site.Cat
		} else if br.App != nil {
			cats = br.App.Cat
		}
		if !e.matchCategories(cats, targeting.CatWhitelist) {
			result.Matched = false
			result.FailedCriteria = "category_whitelist"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "category_whitelist")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "category_whitelist")
		}
	}

	// Category blacklist
	if len(targeting.CatBlacklist) > 0 {
		var cats []string
		if br.Site != nil {
			cats = br.Site.Cat
		} else if br.App != nil {
			cats = br.App.Cat
		}
		if e.matchCategories(cats, targeting.CatBlacklist) {
			result.Matched = false
			result.FailedCriteria = "category_blacklist"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "category_blacklist")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "category_blacklist")
		}
	}

	// Banner size targeting
	if targeting.MinBannerW > 0 || targeting.MinBannerH > 0 {
		if imp.Banner != nil {
			if targeting.MinBannerW > 0 && imp.Banner.W < targeting.MinBannerW {
				result.Matched = false
				result.FailedCriteria = "banner_width"
				if e.metrics != nil {
					e.metrics.RecordTargetingMiss(li.ID, "banner_size")
				}
				return result
			}
			if targeting.MinBannerH > 0 && imp.Banner.H < targeting.MinBannerH {
				result.Matched = false
				result.FailedCriteria = "banner_height"
				if e.metrics != nil {
					e.metrics.RecordTargetingMiss(li.ID, "banner_size")
				}
				return result
			}
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "banner_size")
		}
	}

	// Connection type targeting
	if len(targeting.ConnectionTypes) > 0 {
		connType := int32(0)
		if br.Device != nil {
			connType = br.Device.ConnectionType
		}
		if !e.matchConnectionType(connType, targeting.ConnectionTypes) {
			result.Matched = false
			result.FailedCriteria = "connection_type"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "connection_type")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "connection_type")
		}
	}

	// Carrier targeting
	if len(targeting.Carriers) > 0 {
		carrier := ""
		if br.Device != nil {
			carrier = br.Device.Carrier
		}
		if !e.matchCarrier(carrier, targeting.Carriers) {
			result.Matched = false
			result.FailedCriteria = "carrier"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "carrier")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "carrier")
		}
	}

	// Device make/model targeting
	if len(targeting.DeviceMakes) > 0 {
		make := ""
		if br.Device != nil {
			make = br.Device.Make
		}
		if !e.matchMake(make, targeting.DeviceMakes) {
			result.Matched = false
			result.FailedCriteria = "device_make"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "device_make")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "device_make")
		}
	}

	// Language targeting
	if len(targeting.Languages) > 0 {
		lang := ""
		if br.Device != nil {
			lang = br.Device.Language
		}
		if !e.matchLanguage(lang, targeting.Languages) {
			result.Matched = false
			result.FailedCriteria = "language"
			if e.metrics != nil {
				e.metrics.RecordTargetingMiss(li.ID, "language")
			}
			return result
		}
		if e.metrics != nil {
			e.metrics.RecordTargetingMatch(li.ID, "language")
		}
	}

	return result
}

// lookupGeo performs a cached geo lookup.
func (e *TargetingEngine) lookupGeo(ip string) *GeoInfo {
	if ip == "" || e.geoProvider == nil {
		return nil
	}

	// Check cache
	start := time.Now()
	if info, ok := e.geoCache.get(ip); ok {
		if e.metrics != nil {
			e.metrics.RecordGeoLookup(true, time.Since(start))
		}
		return info
	}

	// Lookup
	info, err := e.geoProvider.Lookup(ip)
	if err != nil {
		return nil
	}

	// Cache result
	e.geoCache.set(ip, info)
	if e.metrics != nil {
		e.metrics.RecordGeoLookup(false, time.Since(start))
	}

	return info
}

func (c *geoCache) get(ip string) (*GeoInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[ip]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.info, true
}

func (c *geoCache) set(ip string, info *GeoInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity (simple FIFO)
	if len(c.data) >= c.maxSize {
		for k := range c.data {
			delete(c.data, k)
			break
		}
	}

	c.data[ip] = &geoCacheEntry{
		info:      info,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Matching helper functions

func (e *TargetingEngine) matchCountry(code string, allowed []string) bool {
	code = strings.ToUpper(code)
	for _, c := range allowed {
		if strings.ToUpper(c) == code {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchRegion(region string, allowed []string) bool {
	region = strings.ToLower(region)
	for _, r := range allowed {
		if strings.ToLower(r) == region {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchCity(city string, allowed []string) bool {
	city = strings.ToLower(city)
	for _, c := range allowed {
		if strings.ToLower(c) == city {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchDomain(domain string, list []string) bool {
	domain = strings.ToLower(domain)
	for _, d := range list {
		d = strings.ToLower(d)
		if domain == d || strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchBundle(bundle string, list []string) bool {
	bundle = strings.ToLower(bundle)
	for _, b := range list {
		if strings.ToLower(b) == bundle {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchDeviceType(dt int32, allowed []int32) bool {
	for _, a := range allowed {
		if dt == a {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchOS(os string, allowed []string) bool {
	os = strings.ToLower(os)
	for _, a := range allowed {
		if strings.ToLower(a) == os {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchOSVersion(osv, minV, maxV string) bool {
	if osv == "" {
		return true // No version info, allow
	}
	if minV != "" && compareVersions(osv, minV) < 0 {
		return false
	}
	if maxV != "" && compareVersions(osv, maxV) > 0 {
		return false
	}
	return true
}

func (e *TargetingEngine) matchCategories(cats, target []string) bool {
	if len(cats) == 0 {
		return false
	}
	catSet := make(map[string]bool)
	for _, c := range cats {
		catSet[strings.ToUpper(c)] = true
	}
	for _, t := range target {
		if catSet[strings.ToUpper(t)] {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchConnectionType(ct int32, allowed []int32) bool {
	for _, a := range allowed {
		if ct == a {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchCarrier(carrier string, allowed []string) bool {
	carrier = strings.ToLower(carrier)
	for _, a := range allowed {
		if strings.ToLower(a) == carrier {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchMake(make string, allowed []string) bool {
	make = strings.ToLower(make)
	for _, a := range allowed {
		if strings.ToLower(a) == make {
			return true
		}
	}
	return false
}

func (e *TargetingEngine) matchLanguage(lang string, allowed []string) bool {
	lang = strings.ToLower(lang)
	for _, a := range allowed {
		if strings.ToLower(a) == lang || strings.HasPrefix(lang, strings.ToLower(a)+"-") {
			return true
		}
	}
	return false
}

// compareVersions compares version strings (simple implementation).
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int
		if i < len(parts1) {
			p1 = parseIntSafe(parts1[i])
		}
		if i < len(parts2) {
			p2 = parseIntSafe(parts2[i])
		}

		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}
	return 0
}

func parseIntSafe(s string) int {
	// Extract numeric prefix
	var num int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			num = num*10 + int(c-'0')
		} else {
			break
		}
	}
	return num
}

// MockGeoProvider is a simple geo provider for testing.
type MockGeoProvider struct {
	data map[string]*GeoInfo
}

func NewMockGeoProvider() *MockGeoProvider {
	return &MockGeoProvider{
		data: make(map[string]*GeoInfo),
	}
}

func (m *MockGeoProvider) AddEntry(ip string, info *GeoInfo) {
	m.data[ip] = info
}

func (m *MockGeoProvider) Lookup(ip string) (*GeoInfo, error) {
	// Parse IP to handle IPv4 vs IPv6
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, nil
	}

	if info, ok := m.data[ip]; ok {
		return info, nil
	}
	return nil, nil
}

func (m *MockGeoProvider) Close() error {
	return nil
}
