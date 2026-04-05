package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type CampaignRepo interface {
	Create(ctx context.Context, c *domain.Campaign) error
	GetByID(ctx context.Context, tenantID domain.TenantID, id domain.CampaignID) (*domain.Campaign, error)
	List(ctx context.Context, tenantID domain.TenantID, filter CampaignFilter) ([]domain.Campaign, error)
	Update(ctx context.Context, c *domain.Campaign) error
}

type CampaignFilter struct {
	Status *domain.CampaignStatus
	Type   *domain.CampaignType
	Limit  int
	Offset int
}

type DealRepo interface {
	Create(ctx context.Context, d *domain.Deal) error
	CreateBatch(ctx context.Context, deals []domain.Deal) error
	GetByID(ctx context.Context, tenantID domain.TenantID, id domain.DealID) (*domain.Deal, error)
	List(ctx context.Context, tenantID domain.TenantID, filter DealFilter) ([]domain.Deal, int, error)
	Update(ctx context.Context, d *domain.Deal) error
}

type DealFilter struct {
	CampaignID *domain.CampaignID
	Status     *domain.DealStatus
	MinScore   *float64
	Search     *string
	SortBy     string
	SortDir    string
	Limit      int
	Offset     int
}

type EventRepo interface {
	Create(ctx context.Context, e *domain.DomainEvent) error
	List(ctx context.Context, tenantID domain.TenantID, filter EventFilter) ([]domain.DomainEvent, error)
}

type EventFilter struct {
	EntityType *string
	EntityID   *string
	EventType  *string
	Limit      int
	Offset     int
}

type ScoringConfigRepo interface {
	Create(ctx context.Context, sc *domain.ScoringConfig) error
	GetActive(ctx context.Context, tenantID domain.TenantID) (*domain.ScoringConfig, error)
	GetByID(ctx context.Context, tenantID domain.TenantID, id domain.ScoringConfigID) (*domain.ScoringConfig, error)
	SetActive(ctx context.Context, tenantID domain.TenantID, id domain.ScoringConfigID) error
}

type DiscoveryConfigRepo interface {
	Get(ctx context.Context, tenantID domain.TenantID) (*domain.DiscoveryConfig, error)
	Upsert(ctx context.Context, dc *domain.DiscoveryConfig) error
}

type BrandBlocklistRepo interface {
	List(ctx context.Context, tenantID domain.TenantID) ([]domain.BlockedBrand, error)
	Add(ctx context.Context, b *domain.BlockedBrand) error
	Remove(ctx context.Context, tenantID domain.TenantID, brand string) error
	Exists(ctx context.Context, tenantID domain.TenantID, brand string) (bool, error)
}
