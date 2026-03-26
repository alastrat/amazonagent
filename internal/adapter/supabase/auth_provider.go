package supabase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type AuthProvider struct {
	jwtSecret string
	isDev     bool
}

func NewAuthProvider(jwtSecret string, isDev bool) *AuthProvider {
	return &AuthProvider{jwtSecret: jwtSecret, isDev: isDev}
}

func (p *AuthProvider) ValidateToken(ctx context.Context, token string) (*port.AuthContext, error) {
	if p.isDev && len(token) > 4 && token[:4] == "dev-" {
		slog.Warn("using dev auth mode — do not use in production")
		return &port.AuthContext{
			UserID:   domain.UserID("dev-user"),
			TenantID: domain.TenantID("dev-tenant"),
			Role:     domain.RoleOwner,
		}, nil
	}

	return nil, fmt.Errorf("Supabase JWT validation not yet implemented — use dev token format 'dev-<user>-<tenant>'")
}
