package storage

import (
	"context"
	"fmt"
	"sync"

	"github.com/radiusdt/vector-dsp/internal/models"
)

// InMemorySourceRepo provides in-memory storage for sources.
type InMemorySourceRepo struct {
	mu              sync.RWMutex
	s2sSources      map[string]*models.S2SSource
	s2sNameIndex    map[string]string // internal_name -> id
	rtbSources      map[string]*models.RTBSource
	campaignSources map[string]*models.CampaignSource
	
	// Reference to campaign repo for GetCampaignsForSource
	campaignRepo CampaignRepo
}

// NewInMemorySourceRepo creates a new in-memory source repository.
func NewInMemorySourceRepo() *InMemorySourceRepo {
	return &InMemorySourceRepo{
		s2sSources:      make(map[string]*models.S2SSource),
		s2sNameIndex:    make(map[string]string),
		rtbSources:      make(map[string]*models.RTBSource),
		campaignSources: make(map[string]*models.CampaignSource),
	}
}

// SetCampaignRepo sets the campaign repository for lookups.
func (r *InMemorySourceRepo) SetCampaignRepo(repo CampaignRepo) {
	r.campaignRepo = repo
}

// =============================================
// S2S Sources
// =============================================

func (r *InMemorySourceRepo) ListS2SSources(ctx context.Context) ([]*models.S2SSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*models.S2SSource, 0, len(r.s2sSources))
	for _, src := range r.s2sSources {
		result = append(result, src)
	}
	return result, nil
}

func (r *InMemorySourceRepo) GetS2SSource(ctx context.Context, id string) (*models.S2SSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	src, ok := r.s2sSources[id]
	if !ok {
		return nil, nil
	}
	return src, nil
}

func (r *InMemorySourceRepo) GetS2SSourceByName(ctx context.Context, internalName string) (*models.S2SSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.s2sNameIndex[internalName]
	if !ok {
		return nil, nil
	}
	return r.s2sSources[id], nil
}

func (r *InMemorySourceRepo) UpsertS2SSource(ctx context.Context, src *models.S2SSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate internal_name
	if existingID, ok := r.s2sNameIndex[src.InternalName]; ok && existingID != src.ID {
		return fmt.Errorf("internal_name '%s' already exists", src.InternalName)
	}

	// Remove old name index if updating
	if existing, ok := r.s2sSources[src.ID]; ok {
		delete(r.s2sNameIndex, existing.InternalName)
	}

	r.s2sSources[src.ID] = src
	r.s2sNameIndex[src.InternalName] = src.ID
	return nil
}

func (r *InMemorySourceRepo) DeleteS2SSource(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if src, ok := r.s2sSources[id]; ok {
		delete(r.s2sNameIndex, src.InternalName)
	}
	delete(r.s2sSources, id)
	return nil
}

// =============================================
// RTB Sources
// =============================================

func (r *InMemorySourceRepo) ListRTBSources(ctx context.Context) ([]*models.RTBSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*models.RTBSource, 0, len(r.rtbSources))
	for _, src := range r.rtbSources {
		result = append(result, src)
	}
	return result, nil
}

func (r *InMemorySourceRepo) GetRTBSource(ctx context.Context, id string) (*models.RTBSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	src, ok := r.rtbSources[id]
	if !ok {
		return nil, nil
	}
	return src, nil
}

func (r *InMemorySourceRepo) UpsertRTBSource(ctx context.Context, src *models.RTBSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.rtbSources[src.ID] = src
	return nil
}

func (r *InMemorySourceRepo) DeleteRTBSource(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.rtbSources, id)
	return nil
}

// =============================================
// Campaign-Source Links
// =============================================

func (r *InMemorySourceRepo) GetCampaignSources(ctx context.Context, campaignID string) ([]*models.CampaignSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*models.CampaignSource, 0)
	for _, link := range r.campaignSources {
		if link.CampaignID == campaignID {
			result = append(result, link)
		}
	}
	return result, nil
}

func (r *InMemorySourceRepo) GetCampaignsForSource(ctx context.Context, sourceType, sourceID string) ([]*models.Campaign, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.campaignRepo == nil {
		return nil, fmt.Errorf("campaign repo not set")
	}

	// Find all campaign IDs linked to this source
	campaignIDs := make(map[string]bool)
	for _, link := range r.campaignSources {
		if link.SourceType == sourceType && link.SourceID == sourceID && link.Status == "active" {
			campaignIDs[link.CampaignID] = true
		}
	}

	// Get campaigns
	result := make([]*models.Campaign, 0)
	for cid := range campaignIDs {
		campaign, err := r.campaignRepo.GetByID(ctx, cid)
		if err == nil && campaign != nil && campaign.Status == "active" {
			result = append(result, campaign)
		}
	}

	return result, nil
}

func (r *InMemorySourceRepo) UpsertCampaignSource(ctx context.Context, link *models.CampaignSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for existing link with same campaign + source
	for id, existing := range r.campaignSources {
		if existing.CampaignID == link.CampaignID &&
			existing.SourceType == link.SourceType &&
			existing.SourceID == link.SourceID &&
			id != link.ID {
			// Update existing
			link.ID = id
			break
		}
	}

	r.campaignSources[link.ID] = link
	return nil
}

func (r *InMemorySourceRepo) DeleteCampaignSource(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.campaignSources, id)
	return nil
}

// =============================================
// PostgresSourceRepo (stub)
// =============================================

// PostgresSourceRepo provides PostgreSQL storage for sources.
type PostgresSourceRepo struct {
	// pool *pgxpool.Pool
}

// NewPostgresSourceRepo creates a new PostgreSQL source repository.
func NewPostgresSourceRepo(pool interface{}) *InMemorySourceRepo {
	// TODO: Implement PostgreSQL storage
	// For now, return in-memory
	return NewInMemorySourceRepo()
}
