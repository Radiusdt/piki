package targeting

import (
	"fmt"
	"net"

	"github.com/oschwald/geoip2-golang"
)

// MaxMindGeoProvider implements GeoProvider using MaxMind GeoLite2 database.
type MaxMindGeoProvider struct {
	reader *geoip2.Reader
}

// NewMaxMindGeoProvider creates a new MaxMind geo provider.
func NewMaxMindGeoProvider(dbPath string) (*MaxMindGeoProvider, error) {
	reader, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GeoIP database: %w", err)
	}

	return &MaxMindGeoProvider{reader: reader}, nil
}

// Lookup returns geo information for an IP address.
func (m *MaxMindGeoProvider) Lookup(ip string) (*GeoInfo, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	record, err := m.reader.City(parsedIP)
	if err != nil {
		return nil, err
	}

	info := &GeoInfo{
		Country:     record.Country.Names["en"],
		CountryCode: record.Country.IsoCode,
		Latitude:    record.Location.Latitude,
		Longitude:   record.Location.Longitude,
		Timezone:    record.Location.TimeZone,
	}

	if len(record.Subdivisions) > 0 {
		info.Region = record.Subdivisions[0].Names["en"]
	}

	if record.City.Names["en"] != "" {
		info.City = record.City.Names["en"]
	}

	if record.Postal.Code != "" {
		info.PostalCode = record.Postal.Code
	}

	return info, nil
}

// Close closes the GeoIP database.
func (m *MaxMindGeoProvider) Close() error {
	if m.reader != nil {
		return m.reader.Close()
	}
	return nil
}
