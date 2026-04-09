package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// CreditAccountRepo manages credit balances per tenant.
type CreditAccountRepo interface {
	Get(ctx context.Context, tenantID domain.TenantID) (*domain.CreditAccount, error)
	EnsureExists(ctx context.Context, tenantID domain.TenantID, tier domain.CreditTier) error
	Debit(ctx context.Context, tenantID domain.TenantID, amount int) error
	ResetMonthly(ctx context.Context, tenantID domain.TenantID) error
	UpdateTier(ctx context.Context, tenantID domain.TenantID, tier domain.CreditTier) error
}

// CreditTransactionRepo records the immutable credit ledger.
type CreditTransactionRepo interface {
	Record(ctx context.Context, tx *domain.CreditTransaction) error
	ListByTenant(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.CreditTransaction, error)
}
