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
		// Stable UUIDs for dev mode — Postgres requires UUID format for tenant_id
		return &port.AuthContext{
			UserID:   domain.UserID("00000000-0000-0000-0000-000000000001"),
			TenantID: domain.TenantID("00000000-0000-0000-0000-000000000010"),
			Role:     domain.RoleOwner,
		}, nil
	}

	return nil, fmt.Errorf("Supabase JWT validation not yet implemented — use dev token format 'dev-<user>-<tenant>'")
}
