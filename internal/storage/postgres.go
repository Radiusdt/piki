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

func NewPostgresCampaignRepo(pool *pgxpool.Pool) *PostgresCampaignRepo {
	return &PostgresCampaignRepo{pool: pool}
}

func (r *PostgresCampaignRepo) GetCampaign(id string) (*models.Campaign, error) {
	ctx := context.Background()

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

	lineItems, err := r.getLineItemsByCampaign(ctx, id)
	if err != nil {
		return nil, err
	}
	c.LineItems = lineItems

	return &c, nil
}

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

		lineItems, err := r.getLineItemsByCampaign(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		c.LineItems = lineItems

		campaigns = append(campaigns, &c)
	}

	return campaigns, nil
}

func (r *PostgresCampaignRepo) UpsertCampaign(c *models.Campaign) error {
	ctx := context.Background()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

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

	_, err = tx.Exec(ctx, `DELETE FROM line_items WHERE campaign_id = $1`, c.ID)
	if err != nil {
		return fmt.Errorf("failed to delete line items: %w", err)
	}

	for _, li := range c.LineItems {
		if err := r.insertLineItem(ctx, tx, &li); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

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

		if len(targetingJSON) > 0 {
			if err := json.Unmarshal(targetingJSON, &li.Targeting); err != nil {
				return nil, fmt.Errorf("failed to parse targeting: %w", err)
			}
		}

		creatives, err := r.getCreativesByLineItem(ctx, li.ID)
		if err != nil {
			return nil, err
		}
		li.Creatives = creatives

		lineItems = append(lineItems, li)
	}

	return lineItems, nil
}

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

func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// PostgresAdvertiserRepo implements AdvertiserRepo using PostgreSQL.
type PostgresAdvertiserRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresAdvertiserRepo(pool *pgxpool.Pool) *PostgresAdvertiserRepo {
	return &PostgresAdvertiserRepo{pool: pool}
}

func (r *PostgresAdvertiserRepo) GetAdvertiser(id string) (*models.Advertiser, error) {
	ctx := context.Background()

	var a models.Advertiser
	var legalName, taxID, address *string

	err := r.pool.QueryRow(ctx, `
		SELECT id, name, legal_name, tax_id, address, created_at, updated_at
		FROM advertisers WHERE id = $1
	`, id).Scan(&a.ID, &a.Name, &legalName, &taxID, &address, &a.CreatedAt, &a.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get advertiser: %w", err)
	}

	if legalName != nil {
		a.LegalName = *legalName
	}
	if taxID != nil {
		a.TaxID = *taxID
	}
	if address != nil {
		a.Address = *address
	}

	return &a, nil
}

func (r *PostgresAdvertiserRepo) ListAdvertisers() ([]*models.Advertiser, error) {
	ctx := context.Background()

	rows, err := r.pool.Query(ctx, `
		SELECT id, name, legal_name, tax_id, address, created_at, updated_at
		FROM advertisers ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list advertisers: %w", err)
	}
	defer rows.Close()

	var advertisers []*models.Advertiser
	for rows.Next() {
		var a models.Advertiser
		var legalName, taxID, address *string

		if err := rows.Scan(&a.ID, &a.Name, &legalName, &taxID, &address, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}

		if legalName != nil {
			a.LegalName = *legalName
		}
		if taxID != nil {
			a.TaxID = *taxID
		}
		if address != nil {
			a.Address = *address
		}

		advertisers = append(advertisers, &a)
	}

	return advertisers, nil
}

func (r *PostgresAdvertiserRepo) UpsertAdvertiser(a *models.Advertiser) error {
	ctx := context.Background()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO advertisers (id, name, legal_name, tax_id, address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			legal_name = EXCLUDED.legal_name,
			tax_id = EXCLUDED.tax_id,
			address = EXCLUDED.address,
			updated_at = EXCLUDED.updated_at
	`, a.ID, a.Name, nullString(a.LegalName), nullString(a.TaxID), nullString(a.Address), a.CreatedAt, a.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert advertiser: %w", err)
	}

	return nil
}

// PostgresEventStore implements EventStore using PostgreSQL.
type PostgresEventStore struct {
	pool *pgxpool.Pool
}

func NewPostgresEventStore(pool *pgxpool.Pool) *PostgresEventStore {
	return &PostgresEventStore{pool: pool}
}

func (s *PostgresEventStore) SaveClick(click *Click) error {
	if click == nil {
		return nil
	}

	ctx := context.Background()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO clicks (id, campaign_id, line_item_id, user_id, target_url, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO NOTHING
	`, click.ID, click.CampaignID, click.LineItemID, nullString(click.UserID), click.TargetURL, click.Timestamp)

	if err != nil {
		return fmt.Errorf("failed to save click: %w", err)
	}
	return nil
}

func (s *PostgresEventStore) SaveConversion(conv *Conversion) error {
	if conv == nil {
		return nil
	}

	ctx := context.Background()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO conversions (id, click_id, external_id, event_name, revenue, currency, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO NOTHING
	`, conv.ID, nullString(conv.ClickID), nullString(conv.ExternalID), conv.EventName, conv.Revenue, conv.Currency, conv.Timestamp)

	if err != nil {
		return fmt.Errorf("failed to save conversion: %w", err)
	}
	return nil
}

func (s *PostgresEventStore) GetClick(id string) (*Click, error) {
	ctx := context.Background()

	var click Click
	var userID *string

	err := s.pool.QueryRow(ctx, `
		SELECT id, campaign_id, line_item_id, user_id, target_url, timestamp
		FROM clicks WHERE id = $1
	`, id).Scan(&click.ID, &click.CampaignID, &click.LineItemID, &userID, &click.TargetURL, &click.Timestamp)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get click: %w", err)
	}

	if userID != nil {
		click.UserID = *userID
	}

	return &click, nil
}

func (s *PostgresEventStore) ListClicks() ([]*Click, error) {
	ctx := context.Background()

	rows, err := s.pool.Query(ctx, `
		SELECT id, campaign_id, line_item_id, user_id, target_url, timestamp
		FROM clicks ORDER BY timestamp DESC LIMIT 10000
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list clicks: %w", err)
	}
	defer rows.Close()

	var clicks []*Click
	for rows.Next() {
		var click Click
		var userID *string

		if err := rows.Scan(&click.ID, &click.CampaignID, &click.LineItemID, &userID, &click.TargetURL, &click.Timestamp); err != nil {
			return nil, err
		}

		if userID != nil {
			click.UserID = *userID
		}

		clicks = append(clicks, &click)
	}

	return clicks, nil
}

func (s *PostgresEventStore) ListConversions() ([]*Conversion, error) {
	ctx := context.Background()

	rows, err := s.pool.Query(ctx, `
		SELECT id, click_id, external_id, event_name, revenue, currency, timestamp
		FROM conversions ORDER BY timestamp DESC LIMIT 10000
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversions: %w", err)
	}
	defer rows.Close()

	var conversions []*Conversion
	for rows.Next() {
		var conv Conversion
		var clickID, externalID *string

		if err := rows.Scan(&conv.ID, &clickID, &externalID, &conv.EventName, &conv.Revenue, &conv.Currency, &conv.Timestamp); err != nil {
			return nil, err
		}

		if clickID != nil {
			conv.ClickID = *clickID
		}
		if externalID != nil {
			conv.ExternalID = *externalID
		}

		conversions = append(conversions, &conv)
	}

	return conversions, nil
}
