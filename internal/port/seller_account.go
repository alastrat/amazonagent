package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// SellerAccountRepo manages per-tenant Amazon seller account credentials.
type SellerAccountRepo interface {
	Create(ctx context.Context, account *domain.AmazonSellerAccount) error
	Get(ctx context.Context, tenantID domain.TenantID) (*domain.AmazonSellerAccount, error)
	Update(ctx context.Context, account *domain.AmazonSellerAccount) error
	Delete(ctx context.Context, tenantID domain.TenantID) error
	UpdateStatus(ctx context.Context, tenantID domain.TenantID, status domain.SellerAccountStatus, errMsg string) error
}
