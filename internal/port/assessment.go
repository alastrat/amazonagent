package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type SellerProfileRepo interface {
	Create(ctx context.Context, profile *domain.SellerProfile) error
	Get(ctx context.Context, tenantID domain.TenantID) (*domain.SellerProfile, error)
	Update(ctx context.Context, profile *domain.SellerProfile) error
	Delete(ctx context.Context, tenantID domain.TenantID) error
}

type EligibilityFingerprintRepo interface {
	Create(ctx context.Context, fp *domain.EligibilityFingerprint) error
	Get(ctx context.Context, tenantID domain.TenantID) (*domain.EligibilityFingerprint, error)
	SaveProbeResults(ctx context.Context, fingerprintID string, tenantID domain.TenantID, results []domain.BrandProbeResult) error
	SaveCategoryEligibilities(ctx context.Context, fingerprintID string, tenantID domain.TenantID, categories []domain.CategoryEligibility) error
	Delete(ctx context.Context, tenantID domain.TenantID) error
}
