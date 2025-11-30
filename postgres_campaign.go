package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/radiusdt/vector-dsp/internal/models"
)

// PostgresCampaignRepo implements CampaignRepo using PostgreSQL.
type PostgresCampaignRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresCampaignRepo creates a new PostgreSQL-backed campaign repository.
func NewPostgresCampaignRepo(pool *pgxpool.Pool) *PostgresCampaignRepo {
	return &PostgresCampaignRepo{pool: pool}
}

// GetCampaign returns a campaign by ID with all its line items and creatives.
func (r *PostgresCampaignRepo) GetCampaign(id string) (*models.Campaign, error) {
	ctx := context.Background()

	// Get campaign
	var c models.Campaign
	err := r.pool.QueryRow(ctx, `
		SELECT id, advertiser_id, name, status, created_at, updated_at
		FROM campaigns WHERE id = $1
	`, id).Scan(&c.ID, &c.AdvertiserID, &c.Name, &c.Status, &c.CreatedAt, &c.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}

	// Get line items
	lineItems, err := r.getLineItemsByCampaign(ctx, id)
	if err != nil {
		return nil, err
	}
	c.LineItems = lineItems

	return &c, nil
}

// ListCampaigns returns all campaigns.
func (r *PostgresCampaignRepo) ListCampaigns() ([]*models.Campaign, error) {
	ctx := context.Background()

	rows, err := r.pool.Query(ctx, `
		SELECT id, advertiser_id, name, status, created_at, updated_at
		FROM campaigns ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*models.Campaign
	for rows.Next() {
		var c models.Campaign
		if err := rows.Scan(&c.ID, &c.AdvertiserID, &c.Name, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}

		// Get line items for each campaign
		lineItems, err := r.getLineItemsByCampaign(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		c.LineItems = lineItems

		campaigns = append(campaigns, &c)
	}

	return campaigns, nil
}

// ListActiveCampaigns returns only active campaigns (optimized for bidding).
func (r *PostgresCampaignRepo) ListActiveCampaigns() ([]*models.Campaign, error) {
	ctx := context.Background()

	rows, err := r.pool.Query(ctx, `
		SELECT id, advertiser_id, name, status, created_at, updated_at
		FROM campaigns WHERE status = 'active'
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list active campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*models.Campaign
	for rows.Next() {
		var c models.Campaign
		if err := rows.Scan(&c.ID, &c.AdvertiserID, &c.Name, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}

		lineItems, err := r.getLineItemsByCampaign(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		c.LineItems = lineItems

		campaigns = append(campaigns, &c)
	}

	return campaigns, nil
}

// UpsertCampaign inserts or updates a campaign.
func (r *PostgresCampaignRepo) UpsertCampaign(c *models.Campaign) error {
	ctx := context.Background()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Upsert campaign
	_, err = tx.Exec(ctx, `
		INSERT INTO campaigns (id, advertiser_id, name, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			advertiser_id = EXCLUDED.advertiser_id,
			name = EXCLUDED.name,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at
	`, c.ID, c.AdvertiserID, c.Name, c.Status, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert campaign: %w", err)
	}

	// Delete existing line items and creatives (will be re-inserted)
	_, err = tx.Exec(ctx, `DELETE FROM line_items WHERE campaign_id = $1`, c.ID)
	if err != nil {
		return fmt.Errorf("failed to delete line items: %w", err)
	}

	// Insert line items
	for _, li := range c.LineItems {
		if err := r.insertLineItem(ctx, tx, &li); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// getLineItemsByCampaign fetches line items for a campaign.
func (r *PostgresCampaignRepo) getLineItemsByCampaign(ctx context.Context, campaignID string) ([]models.LineItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, campaign_id, name, is_active, priority,
			   bid_strategy_type, fixed_cpm, targeting,
			   daily_budget, total_budget, start_at, end_at,
			   freq_cap_per_user_per_day, qps_limit_per_source
		FROM line_items WHERE campaign_id = $1
	`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get line items: %w", err)
	}
	defer rows.Close()

	var lineItems []models.LineItem
	for rows.Next() {
		var li models.LineItem
		var targetingJSON []byte
		var fixedCPM *float64
		var totalBudget *float64

		if err := rows.Scan(
			&li.ID, &li.CampaignID, &li.Name, &li.IsActive, &li.Priority,
			&li.BidStrategy.Type, &fixedCPM, &targetingJSON,
			&li.Pacing.DailyBudget, &totalBudget, &li.Pacing.StartAt, &li.Pacing.EndAt,
			&li.Pacing.FreqCapPerUserPerDay, &li.Pacing.QPSLimitPerSource,
		); err != nil {
			return nil, err
		}

		if fixedCPM != nil {
			li.BidStrategy.FixedCPM = *fixedCPM
		}
		if totalBudget != nil {
			li.Pacing.TotalBudget = *totalBudget
		}

		// Parse targeting JSON
		if len(targetingJSON) > 0 {
			if err := json.Unmarshal(targetingJSON, &li.Targeting); err != nil {
				return nil, fmt.Errorf("failed to parse targeting: %w", err)
			}
		}

		// Get creatives for this line item
		creatives, err := r.getCreativesByLineItem(ctx, li.ID)
		if err != nil {
			return nil, err
		}
		li.Creatives = creatives

		lineItems = append(lineItems, li)
	}

	return lineItems, nil
}

// getCreativesByLineItem fetches creatives for a line item.
func (r *PostgresCampaignRepo) getCreativesByLineItem(ctx context.Context, lineItemID string) ([]models.Creative, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, advertiser_id, format, adm_template, width, height,
			   adomain, click_url, video_url, vast_tag, created_at, updated_at
		FROM creatives WHERE line_item_id = $1
	`, lineItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get creatives: %w", err)
	}
	defer rows.Close()

	var creatives []models.Creative
	for rows.Next() {
		var cr models.Creative
		var advertiserID *string

		if err := rows.Scan(
			&cr.ID, &advertiserID, &cr.Format, &cr.AdmTemplate, &cr.W, &cr.H,
			&cr.ADomain, &cr.ClickURL, &cr.VideoURL, &cr.VASTTag, &cr.CreatedAt, &cr.UpdatedAt,
		); err != nil {
			return nil, err
		}

		if advertiserID != nil {
			cr.AdvertiserID = *advertiserID
		}

		creatives = append(creatives, cr)
	}

	return creatives, nil
}

// insertLineItem inserts a line item and its creatives.
func (r *PostgresCampaignRepo) insertLineItem(ctx context.Context, tx pgx.Tx, li *models.LineItem) error {
	targetingJSON, err := json.Marshal(li.Targeting)
	if err != nil {
		return fmt.Errorf("failed to marshal targeting: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO line_items (
			id, campaign_id, name, is_active, priority,
			bid_strategy_type, fixed_cpm, targeting,
			daily_budget, total_budget, start_at, end_at,
			freq_cap_per_user_per_day, qps_limit_per_source
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`,
		li.ID, li.CampaignID, li.Name, li.IsActive, li.Priority,
		li.BidStrategy.Type, li.BidStrategy.FixedCPM, targetingJSON,
		li.Pacing.DailyBudget, li.Pacing.TotalBudget, li.Pacing.StartAt, li.Pacing.EndAt,
		li.Pacing.FreqCapPerUserPerDay, li.Pacing.QPSLimitPerSource,
	)
	if err != nil {
		return fmt.Errorf("failed to insert line item: %w", err)
	}

	// Insert creatives
	for _, cr := range li.Creatives {
		_, err = tx.Exec(ctx, `
			INSERT INTO creatives (
				id, advertiser_id, line_item_id, format, adm_template,
				width, height, adomain, click_url, video_url, vast_tag
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`,
			cr.ID, nullString(cr.AdvertiserID), li.ID, cr.Format, cr.AdmTemplate,
			cr.W, cr.H, cr.ADomain, cr.ClickURL, cr.VideoURL, cr.VASTTag,
		)
		if err != nil {
			return fmt.Errorf("failed to insert creative: %w", err)
		}
	}

	return nil
}

// nullString returns nil if s is empty, otherwise returns s.
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
