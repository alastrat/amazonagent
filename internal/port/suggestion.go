package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type SuggestionRepo interface {
	Create(ctx context.Context, s *domain.DiscoverySuggestion) error
	CreateBatch(ctx context.Context, suggestions []domain.DiscoverySuggestion) error
	GetByID(ctx context.Context, tenantID domain.TenantID, id domain.SuggestionID) (*domain.DiscoverySuggestion, error)
	ListPending(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoverySuggestion, error)
	ListAll(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoverySuggestion, error)
	Accept(ctx context.Context, id domain.SuggestionID, dealID domain.DealID) error
	Dismiss(ctx context.Context, id domain.SuggestionID) error
	CountToday(ctx context.Context, tenantID domain.TenantID) (int, error)
}
