package dsp

import (
    "errors"
    "fmt"
    "strings"

    "github.com/radiusdt/vector-dsp/internal/models"
    "github.com/radiusdt/vector-dsp/internal/storage"
)

// BidService implements the core business logic for building
// OpenRTB bid responses.  It consults the campaign repository to find
// active campaigns and line items that match the incoming bid request,
// applies pacing rules and constructs a BidResponse when a suitable
// creative is found.
//
// Note: this implementation is intentionally simple.  It does not
// handle second-price auctions, prioritisation across seat bids or
// advanced targeting fields.  Production DSPs typically implement
// complex bidding strategies, performance optimisation and machine
// learning models.
type BidService struct {
    repo  storage.CampaignRepo
    pacer PacingEngine
}

// NewBidService constructs a BidService with the given campaign repo
// and pacing engine.
func NewBidService(repo storage.CampaignRepo, pacer PacingEngine) *BidService {
    return &BidService{repo: repo, pacer: pacer}
}

// BuildBidResponse generates a bid response for the given request.  It
// iterates through impressions and campaigns, selecting the first
// qualifying line item that matches targeting and pacing.  Returns nil
// when no bid should be made.
func (s *BidService) BuildBidResponse(br *models.BidRequest) (*models.BidResponse, error) {
    if br == nil {
        return nil, errors.New("nil bid request")
    }
    // Fetch campaigns once per request
    campaigns, err := s.repo.ListCampaigns()
    if err != nil {
        return nil, err
    }
    var seatBids []models.SeatBid
    // Determine userID for pacing.  Use user.id, buyeruid or device.ifa if present.
    userID := "anonymous"
    if br.User != nil {
        if br.User.ID != "" {
            userID = br.User.ID
        } else if br.User.BuyerUID != "" {
            userID = br.User.BuyerUID
        }
    } else if br.Device != nil && br.Device.Ifa != "" {
        userID = br.Device.Ifa
    }
    for _, imp := range br.Imp {
        bestBid := models.Bid{}
        var found bool
        // Start with the lowest possible priority value to ensure that any
        // line item's priority (which should be >=0) will be higher.
        var bestPriority int32 = -1 << 31
        var bestPrice float64
        for _, c := range campaigns {
            // Only active campaigns participate in auctions
            if c.Status != models.CampaignStatusActive {
                continue
            }
            for _, li := range c.LineItems {
                if !li.IsActive {
                    continue
                }
                // Basic targeting match
                if !matchesTargeting(br, &imp, &li) {
                    continue
                }
                // Determine price in dollars
                price := 0.0
                if li.BidStrategy.Type == models.BidStrategyFixedCPM {
                    price = li.BidStrategy.FixedCPM / 1000.0
                }
                if price <= 0 {
                    continue
                }
                // Enforce bid floor
                if imp.BidFloor > 0 && price < imp.BidFloor {
                    continue
                }
                // Check pacing: ensure we don't exceed budgets or frequency caps
                if !s.pacer.Allow(li.ID, userID, li.Pacing, price) {
                    continue
                }
                // Select a creative that fits the impression.  For banner
                // impressions choose a creative whose dimensions meet or
                // exceed the banner size.  For video impressions choose
                // the first creative of format "video".  If no creative
                // fits, skip.
                var cr *models.Creative
                if imp.Video != nil {
                    // Video request: pick first creative with Format "video"
                    for i := range li.Creatives {
                        c := &li.Creatives[i]
                        if strings.ToLower(c.Format) == "video" {
                            cr = c
                            break
                        }
                    }
                } else {
                    // Banner request
                    for i := range li.Creatives {
                        c := &li.Creatives[i]
                        // If imp banner has size, ensure creative fits
                        if imp.Banner != nil {
                            impW := imp.Banner.W
                            impH := imp.Banner.H
                            if c.W >= impW && c.H >= impH {
                                cr = c
                                break
                            }
                        } else {
                            // If no size specified choose first creative
                            cr = c
                            break
                        }
                    }
                }
                if cr == nil {
                    continue
                }
                // Determine adm depending on creative type.  For banners use
                // the AdmTemplate.  For video use either VASTTag (if
                // provided) or construct a minimal VAST wrapper around
                // VideoURL.
                adm := cr.AdmTemplate
                if imp.Video != nil {
                    if cr.VASTTag != "" {
                        adm = cr.VASTTag
                    } else if cr.VideoURL != "" {
                        adm = fmt.Sprintf(`<VAST version="4.0"><Ad><InLine><Creatives><Creative><Linear><MediaFiles><MediaFile><![CDATA[%s]]></MediaFile></MediaFiles></Linear></Creative></Creatives></InLine></Ad></VAST>`, cr.VideoURL)
                    }
                }
                // Choose this line item if it has higher priority or
                // greater price than the current best.  Priority takes
                // precedence over price.
                if !found || li.Priority > bestPriority || (li.Priority == bestPriority && price > bestPrice) {
                    bestPriority = li.Priority
                    bestPrice = price
                    found = true
                    bestBid = models.Bid{
                        ID:     li.ID + "/" + imp.ID,
                        ImpID:  imp.ID,
                        Price:  price,
                        CrID:   cr.ID,
                        AdM:    adm,
                        ADomain: cr.ADomain,
                        CID:    c.ID,
                        CrtrID: cr.ID,
                    }
                }
            }
        }
        if found {
            seatBids = append(seatBids, models.SeatBid{Bid: []models.Bid{bestBid}})
        }
    }
    if len(seatBids) == 0 {
        return nil, nil
    }
    // Build final response.  Currency defaults to USD; real implementation
    // would support multiple currencies from request.
    resp := &models.BidResponse{
        ID:      br.ID,
        SeatBid: seatBids,
        Cur:     "USD",
    }
    return resp, nil
}

// matchesTargeting performs a minimal set of targeting checks between the
// bid request, impression and line item.  This function can be
// extended to support additional fields such as geo, app bundles,
// domains, operating system versions and category whitelists.  If a
// targeting field is non-empty in the line item, the bid request must
// match it.  Empty fields are treated as wildcards.
func matchesTargeting(br *models.BidRequest, imp *models.Imp, li *models.LineItem) bool {
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
    // Operating system
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
    // Minimum banner size
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