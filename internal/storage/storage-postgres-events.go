package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresEventStore implements EventStore using PostgreSQL.
type PostgresEventStore struct {
	pool *pgxpool.Pool
}

// NewPostgresEventStore creates a new PostgreSQL-backed event store.
func NewPostgresEventStore(pool *pgxpool.Pool) *PostgresEventStore {
	return &PostgresEventStore{pool: pool}
}

// SaveClick stores a click event.
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

// SaveConversion stores a conversion event.
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

// GetClick retrieves a click by ID.
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

// ListClicks returns all clicks (with pagination for production).
func (s *PostgresEventStore) ListClicks() ([]*Click, error) {
	ctx := context.Background()

	// In production, add LIMIT and pagination
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

// ListConversions returns all conversions.
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

// SaveImpression stores an impression (win) event.
func (s *PostgresEventStore) SaveImpression(imp *Impression) error {
	if imp == nil {
		return nil
	}

	ctx := context.Background()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO impressions (id, campaign_id, line_item_id, creative_id, bid_id, imp_id, price, currency, user_id, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO NOTHING
	`, imp.ID, imp.CampaignID, imp.LineItemID, nullString(imp.CreativeID), nullString(imp.BidID), 
	   nullString(imp.ImpID), imp.Price, imp.Currency, nullString(imp.UserID), imp.Timestamp)

	if err != nil {
		return fmt.Errorf("failed to save impression: %w", err)
	}
	return nil
}

// Impression represents a win notification event.
type Impression struct {
	ID         string
	CampaignID string
	LineItemID string
	CreativeID string
	BidID      string
	ImpID      string
	Price      float64
	Currency   string
	UserID     string
	Timestamp  interface{}
}
