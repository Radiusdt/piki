package models

import (
    "errors"
    "time"
)

// AdGroup represents a grouping of creatives within a campaign.  It maps
// closely to the concept of an "Ad Group" in the Ibiza DSP frontend.  Each
// AdGroup belongs to a campaign and can define its own budget, schedule and
// targeting criteria.  The creatives referenced by CreativeIDs must exist in
// the parent campaign's line items/creatives list or an external creative
// repository.
type AdGroup struct {
    ID         string     `json:"id"`
    CampaignID string     `json:"campaign_id"`
    Name       string     `json:"name"`
    Budget     float64    `json:"budget"`
    StartAt    time.Time  `json:"start_at"`
    EndAt      time.Time  `json:"end_at"`
    Targeting  Targeting  `json:"targeting"`
    CreativeIDs []string  `json:"creative_ids"`
    IsActive   bool       `json:"is_active"`
    CreatedAt  time.Time  `json:"created_at"`
    UpdatedAt  time.Time  `json:"updated_at"`
}

// Validate ensures the AdGroup has the minimal required data.  A valid
// AdGroup requires an ID, CampaignID and Name.  Budget must be positive.
func (ag *AdGroup) Validate() error {
    if ag == nil {
        return errors.New("adgroup is nil")
    }
    if ag.ID == "" {
        return errors.New("id is required")
    }
    if ag.CampaignID == "" {
        return errors.New("campaign_id is required")
    }
    if ag.Name == "" {
        return errors.New("name is required")
    }
    if ag.Budget <= 0 {
        return errors.New("budget must be > 0")
    }
    return nil
}