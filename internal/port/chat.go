package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// ChatRepo persists chat sessions and messages.
type ChatRepo interface {
	CreateSession(ctx context.Context, session *domain.ChatSession) error
	GetSession(ctx context.Context, tenantID domain.TenantID) (*domain.ChatSession, error)
	UpdateSession(ctx context.Context, session *domain.ChatSession) error

	SaveMessage(ctx context.Context, msg *domain.ChatMessage) error
	ListMessages(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.ChatMessage, error)
}
