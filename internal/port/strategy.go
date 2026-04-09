package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type StrategyVersionRepo interface {
	Create(ctx context.Context, sv *domain.StrategyVersion) error
	GetByID(ctx context.Context, tenantID domain.TenantID, id domain.StrategyVersionID) (*domain.StrategyVersion, error)
	GetActive(ctx context.Context, tenantID domain.TenantID) (*domain.StrategyVersion, error)
	List(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.StrategyVersion, error)
	NextVersionNumber(ctx context.Context, tenantID domain.TenantID) (int, error)
	SetStatus(ctx context.Context, tenantID domain.TenantID, id domain.StrategyVersionID, status domain.StrategyStatus) error
	Activate(ctx context.Context, tenantID domain.TenantID, id domain.StrategyVersionID) error
}
