package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/radiusdt/vector-dsp/internal/models"
)

// PostgresAdvertiserRepo implements AdvertiserRepo using PostgreSQL.
type PostgresAdvertiserRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresAdvertiserRepo creates a new PostgreSQL-backed advertiser repository.
func NewPostgresAdvertiserRepo(pool *pgxpool.Pool) *PostgresAdvertiserRepo {
	return &PostgresAdvertiserRepo{pool: pool}
}

// GetAdvertiser returns an advertiser by ID.
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

// ListAdvertisers returns all advertisers.
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

// UpsertAdvertiser inserts or updates an advertiser.
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

// DeleteAdvertiser deletes an advertiser by ID.
func (r *PostgresAdvertiserRepo) DeleteAdvertiser(id string) error {
	ctx := context.Background()

	_, err := r.pool.Exec(ctx, `DELETE FROM advertisers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete advertiser: %w", err)
	}

	return nil
}
