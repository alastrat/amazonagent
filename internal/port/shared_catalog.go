package port

import (
	"context"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// SharedCatalogRepo manages the platform-wide product catalog.
type SharedCatalogRepo interface {
	UpsertProduct(ctx context.Context, product *domain.SharedProduct) error
	UpsertProductBatch(ctx context.Context, products []domain.SharedProduct) error
	GetByASIN(ctx context.Context, asin string) (*domain.SharedProduct, error)
	GetByASINs(ctx context.Context, asins []string) ([]domain.SharedProduct, error)
	GetStale(ctx context.Context, olderThan time.Time, limit int) ([]domain.SharedProduct, error)
	SearchByCategory(ctx context.Context, category string, limit int) ([]domain.SharedProduct, error)
	IncrementEnrichment(ctx context.Context, asin string) error
}

// BrandCatalogRepo manages the platform-wide brand catalog.
type BrandCatalogRepo interface {
	Upsert(ctx context.Context, brand *domain.SharedBrand) error
	GetByName(ctx context.Context, name string) (*domain.SharedBrand, error)
	ListByCategory(ctx context.Context, category string) ([]domain.SharedBrand, error)
	UpdateGating(ctx context.Context, normalizedName string, gating string) error
	IncrementProductCount(ctx context.Context, normalizedName string) error
}

// TenantEligibilityRepo manages per-tenant, per-ASIN eligibility (private).
type TenantEligibilityRepo interface {
	Set(ctx context.Context, e *domain.TenantEligibility) error
	SetBatch(ctx context.Context, eligibilities []domain.TenantEligibility) error
	Get(ctx context.Context, tenantID domain.TenantID, asin string) (*domain.TenantEligibility, error)
	GetByASINs(ctx context.Context, tenantID domain.TenantID, asins []string) ([]domain.TenantEligibility, error)
	ListEligible(ctx context.Context, tenantID domain.TenantID, category string, limit int) ([]domain.TenantEligibility, error)
}

// TenantMarginRepo manages per-tenant margin data from price lists (private).
type TenantMarginRepo interface {
	Set(ctx context.Context, m *domain.TenantMargin) error
	GetByASIN(ctx context.Context, tenantID domain.TenantID, asin string) (*domain.TenantMargin, error)
}
