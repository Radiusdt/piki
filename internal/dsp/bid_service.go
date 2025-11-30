package dsp

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/radiusdt/vector-dsp/internal/metrics"
	"github.com/radiusdt/vector-dsp/internal/models"
	"github.com/radiusdt/vector-dsp/internal/storage"
	"github.com/radiusdt/vector-dsp/internal/targeting"
)

// BidService implements the core business logic for building OpenRTB bid responses.
type BidService struct {
	repo       storage.CampaignRepo
	pacer      PacingEngine
	targeting  *targeting.TargetingEngine
	metrics    *metrics.Metrics
}

// NewBidService constructs a BidService with the given dependencies.
func NewBidService(repo storage.CampaignRepo, pacer PacingEngine, targetingEngine *targeting.TargetingEngine, m *metrics.Metrics) *BidService {
	return &BidService{
		repo:      repo,
		pacer:     pacer,
		targeting: targetingEngine,
		metrics:   m,
	}
}

// NoBidReason represents reasons for not bidding.
type NoBidReason string

const (
	NoBidReasonNoMatch       NoBidReason = "no_match"
	NoBidReasonBelowFloor    NoBidReason = "below_floor"
	NoBidReasonPacing        NoBidReason = "pacing"
	NoBidReasonFreqCap       NoBidReason = "freq_cap"
	NoBidReasonNoCreative    NoBidReason = "no_creative"
	NoBidReasonTargeting     NoBidReason = "targeting"
	NoBidReasonInactive      NoBidReason = "inactive"
	NoBidReasonNoCampaigns   NoBidReason = "no_campaigns"
)

// BuildBidResponse generates a bid response for the given request.
func (s *BidService) BuildBidResponse(br *models.BidRequest) (*models.BidResponse, error) {
	start := time.Now()
	
	if br == nil {
		return nil, errors.New("nil bid request")
	}

	// Record request metrics
	source := "unknown"
	if br.App != nil && br.App.Bundle != "" {
		source = "app"
	} else if br.Site != nil && br.Site.Domain != "" {
		source = "web"
	}
	deviceType := int32(0)
	if br.Device != nil {
		deviceType = br.Device.DeviceType
	}
	if s.metrics != nil {
		s.metrics.RecordBidRequest(source, deviceType)
	}

	// Fetch campaigns
	campaigns, err := s.repo.ListCampaigns()
	if err != nil {
		s.recordNoBid("error", time.Since(start))
		return nil, err
	}

	if len(campaigns) == 0 {
		s.recordNoBid(string(NoBidReasonNoCampaigns), time.Since(start))
		return nil, nil
	}

	// Extract user ID for pacing
	userID := s.extractUserID(br)

	var seatBids []models.SeatBid

	for _, imp := range br.Imp {
		bid := s.findBestBid(br, &imp, campaigns, userID)
		if bid != nil {
			seatBids = append(seatBids, models.SeatBid{Bid: []models.Bid{*bid}})
		}
	}

	if len(seatBids) == 0 {
		s.recordNoBid("no_bids", time.Since(start))
		return nil, nil
	}

	resp := &models.BidResponse{
		ID:      br.ID,
		SeatBid: seatBids,
		Cur:     "USD",
	}

	if s.metrics != nil {
		s.metrics.RecordBidResponse("bid", seatBids[0].Bid[0].CID, time.Since(start))
	}

	return resp, nil
}

// findBestBid finds the best bid for an impression.
func (s *BidService) findBestBid(br *models.BidRequest, imp *models.Imp, campaigns []*models.Campaign, userID string) *models.Bid {
	var bestBid *models.Bid
	var bestPriority int32 = -1 << 31
	var bestPrice float64

	for _, c := range campaigns {
		if c.Status != models.CampaignStatusActive {
			continue
		}

		for _, li := range c.LineItems {
			if !li.IsActive {
				continue
			}

			// Check targeting
			if s.targeting != nil {
				result := s.targeting.Match(br, imp, &li)
				if !result.Matched {
					if s.metrics != nil {
						s.metrics.RecordNoBid("targeting_" + result.FailedCriteria)
					}
					continue
				}
			} else {
				// Fallback to basic targeting
				if !s.matchesBasicTargeting(br, imp, &li) {
					continue
				}
			}

			// Calculate price
			price := s.calculateBidPrice(br, imp, &li)
			if price <= 0 {
				continue
			}

			// Check bid floor
			if imp.BidFloor > 0 && price < imp.BidFloor {
				if s.metrics != nil {
					s.metrics.RecordNoBid(string(NoBidReasonBelowFloor))
				}
				continue
			}

			// Check pacing
			if !s.pacer.Allow(li.ID, userID, li.Pacing, price) {
				if s.metrics != nil {
					s.metrics.RecordNoBid(string(NoBidReasonPacing))
				}
				continue
			}

			// Select creative
			cr := s.selectCreative(imp, &li)
			if cr == nil {
				if s.metrics != nil {
					s.metrics.RecordNoBid(string(NoBidReasonNoCreative))
				}
				continue
			}

			// Compare with current best
			if li.Priority > bestPriority || (li.Priority == bestPriority && price > bestPrice) {
				bestPriority = li.Priority
				bestPrice = price

				// Build ad markup
				adm := s.buildAdMarkup(imp, cr)

				// Build notification URLs
				nurl := s.buildNotificationURL("win", c.ID, li.ID, cr.ID, imp.ID, "${AUCTION_PRICE}")
				lurl := s.buildNotificationURL("loss", c.ID, li.ID, cr.ID, imp.ID, "${AUCTION_PRICE}")

				bestBid = &models.Bid{
					ID:      fmt.Sprintf("%s/%s", li.ID, imp.ID),
					ImpID:   imp.ID,
					Price:   price,
					CrID:    cr.ID,
					AdM:     adm,
					NURL:    nurl,
					LURL:    lurl,
					ADomain: cr.ADomain,
					CID:     c.ID,
					CrtrID:  cr.ID,
					W:       cr.W,
					H:       cr.H,
				}

				// Record bid metrics
				if s.metrics != nil {
					s.metrics.RecordBid(c.ID, li.ID, price)
				}
			}
		}
	}

	return bestBid
}

// calculateBidPrice calculates the bid price based on strategy.
func (s *BidService) calculateBidPrice(br *models.BidRequest, imp *models.Imp, li *models.LineItem) float64 {
	switch li.BidStrategy.Type {
	case models.BidStrategyFixedCPM:
		return li.BidStrategy.FixedCPM / 1000.0

	case models.BidStrategyDynamicCPM:
		// Start with max CPM and apply bid shading
		price := li.BidStrategy.MaxCPM / 1000.0
		
		// Apply bid shading if configured
		if li.BidStrategy.BidShading > 0 {
			price *= (1 - li.BidStrategy.BidShading)
		}

		// Ensure we're above minimum
		minPrice := li.BidStrategy.MinCPM / 1000.0
		if price < minPrice {
			price = minPrice
		}

		// Ensure we're above floor
		if imp.BidFloor > 0 && price < imp.BidFloor*1.01 {
			price = imp.BidFloor * 1.01
		}

		return price

	case models.BidStrategyTargetCPA:
		// This would require historical conversion data
		// For now, fall back to a reasonable CPM
		if li.BidStrategy.MaxCPM > 0 {
			return li.BidStrategy.MaxCPM / 1000.0
		}
		return 1.0 / 1000.0 // $1 CPM default

	default:
		if li.BidStrategy.FixedCPM > 0 {
			return li.BidStrategy.FixedCPM / 1000.0
		}
		return 0
	}
}

// selectCreative selects the best matching creative for an impression.
func (s *BidService) selectCreative(imp *models.Imp, li *models.LineItem) *models.Creative {
	if imp.Video != nil {
		// Video request
		for i := range li.Creatives {
			cr := &li.Creatives[i]
			if strings.ToLower(cr.Format) == "video" {
				return cr
			}
		}
		return nil
	}

	if imp.Native != nil {
		// Native request
		for i := range li.Creatives {
			cr := &li.Creatives[i]
			if strings.ToLower(cr.Format) == "native" {
				return cr
			}
		}
		return nil
	}

	if imp.Audio != nil {
		// Audio request
		for i := range li.Creatives {
			cr := &li.Creatives[i]
			if strings.ToLower(cr.Format) == "audio" {
				return cr
			}
		}
		return nil
	}

	// Banner request
	for i := range li.Creatives {
		cr := &li.Creatives[i]
		format := strings.ToLower(cr.Format)
		if format != "" && format != "banner" {
			continue
		}

		// Check size if banner has dimensions
		if imp.Banner != nil {
			// Exact match
			if imp.Banner.W > 0 && imp.Banner.H > 0 {
				if cr.W == imp.Banner.W && cr.H == imp.Banner.H {
					return cr
				}
			}

			// Check format array
			for _, f := range imp.Banner.Format {
				if cr.W == f.W && cr.H == f.H {
					return cr
				}
			}

			// Flexible match (creative fits within banner)
			if cr.W >= imp.Banner.W && cr.H >= imp.Banner.H {
				return cr
			}
		} else {
			// No size specified, return first banner creative
			return cr
		}
	}

	return nil
}

// buildAdMarkup builds the ad markup based on creative type.
func (s *BidService) buildAdMarkup(imp *models.Imp, cr *models.Creative) string {
	if imp.Video != nil {
		if cr.VASTTag != "" {
			return cr.VASTTag
		}
		if cr.VideoURL != "" {
			return fmt.Sprintf(`<VAST version="4.0"><Ad><InLine><Creatives><Creative><Linear><MediaFiles><MediaFile><![CDATA[%s]]></MediaFile></MediaFiles></Linear></Creative></Creatives></InLine></Ad></VAST>`, cr.VideoURL)
		}
	}

	return cr.AdmTemplate
}

// buildNotificationURL builds win/loss notification URL.
func (s *BidService) buildNotificationURL(notifType, campaignID, lineItemID, creativeID, impID, priceMacro string) string {
	// In production, this would be your actual notification endpoint
	return fmt.Sprintf("/openrtb2/%s?campaign_id=%s&line_item_id=%s&creative_id=%s&imp_id=%s&price=%s",
		notifType, campaignID, lineItemID, creativeID, impID, priceMacro)
}

// extractUserID extracts user ID from bid request.
func (s *BidService) extractUserID(br *models.BidRequest) string {
	if br.User != nil {
		if br.User.ID != "" {
			return br.User.ID
		}
		if br.User.BuyerUID != "" {
			return br.User.BuyerUID
		}
	}
	if br.Device != nil && br.Device.Ifa != "" {
		return br.Device.Ifa
	}
	return "anonymous"
}

// matchesBasicTargeting performs basic targeting checks (fallback).
func (s *BidService) matchesBasicTargeting(br *models.BidRequest, imp *models.Imp, li *models.LineItem) bool {
	// Device type
	if len(li.Targeting.DeviceTypes) > 0 {
		dt := int32(0)
		if br.Device != nil {
			dt = br.Device.DeviceType
		}
		matched := false
		for _, allowed := range li.Targeting.DeviceTypes {
			if dt == allowed {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// OS
	if len(li.Targeting.OS) > 0 {
		osVal := ""
		if br.Device != nil {
			osVal = strings.ToLower(br.Device.OS)
		}
		matched := false
		for _, allowed := range li.Targeting.OS {
			if osVal == strings.ToLower(allowed) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Min banner size
	if li.Targeting.MinBannerW > 0 || li.Targeting.MinBannerH > 0 {
		if imp.Banner != nil {
			if li.Targeting.MinBannerW > 0 && imp.Banner.W < li.Targeting.MinBannerW {
				return false
			}
			if li.Targeting.MinBannerH > 0 && imp.Banner.H < li.Targeting.MinBannerH {
				return false
			}
		}
	}

	return true
}

// recordNoBid records a no-bid event.
func (s *BidService) recordNoBid(reason string, latency time.Duration) {
	if s.metrics != nil {
		s.metrics.RecordNoBid(reason)
		s.metrics.RecordBidResponse("nobid", "", latency)
	}
}
