package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type AuthContext struct {
	UserID   domain.UserID
	TenantID domain.TenantID
	Role     domain.Role
}

type AuthProvider interface {
	ValidateToken(ctx context.Context, token string) (*AuthContext, error)
}
