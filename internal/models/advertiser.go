package models

import (
    "errors"
    "time"
)

// Advertiser represents an advertiser entity in the DSP platform.  An advertiser
// can own multiple campaigns and contains identifying information.  Fields
// mirror many of the requirements outlined in the Ibiza DSP frontend README:
// name/legal name, tax identifiers and contact details.  CreatedAt and
// UpdatedAt are maintained by the service layer.
type Advertiser struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    LegalName string    `json:"legal_name"`
    TaxID     string    `json:"tax_id,omitempty"`
    Address   string    `json:"address,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks that required fields are present.  Only the ID and Name
// fields are mandatory; other fields are optional.
func (a *Advertiser) Validate() error {
    if a == nil {
        return errors.New("advertiser is nil")
    }
    if a.ID == "" {
        return errors.New("id is required")
    }
    if a.Name == "" {
        return errors.New("name is required")
    }
    return nil
}