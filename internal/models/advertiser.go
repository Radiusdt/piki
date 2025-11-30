package models

import (
	"errors"
	"time"
)

// Advertiser represents an advertiser entity in the DSP platform.
type Advertiser struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	LegalName string    `json:"legal_name,omitempty"`
	TaxID     string    `json:"tax_id,omitempty"`     // ИНН
	KPP       string    `json:"kpp,omitempty"`        // КПП
	OGRN      string    `json:"ogrn,omitempty"`       // ОГРН
	Address   string    `json:"address,omitempty"`
	Website   string    `json:"website,omitempty"`
	Industry  string    `json:"industry,omitempty"`
	
	// Billing
	Balance      float64 `json:"balance"`
	CreditLimit  float64 `json:"credit_limit,omitempty"`
	Currency     string  `json:"currency,omitempty"` // USD, RUB, EUR
	
	// Bank details
	BIK           string `json:"bik,omitempty"`
	AccountNumber string `json:"account_number,omitempty"`
	BankName      string `json:"bank_name,omitempty"`
	
	// Contract
	ContractNumber string     `json:"contract_number,omitempty"`
	ContractDate   *time.Time `json:"contract_date,omitempty"`
	
	Status    string    `json:"status,omitempty"` // active, paused, suspended
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks that required fields are present.
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

// AvailableBalance returns the amount available for spending
func (a *Advertiser) AvailableBalance() float64 {
	return a.Balance + a.CreditLimit
}

// AdGroup represents a grouping of creatives within a campaign.
type AdGroup struct {
	ID          string    `json:"id"`
	CampaignID  string    `json:"campaign_id"`
	Name        string    `json:"name"`
	Budget      float64   `json:"budget"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
	Targeting   Targeting `json:"targeting"`
	CreativeIDs []string  `json:"creative_ids"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Validate ensures the AdGroup has the minimal required data.
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
