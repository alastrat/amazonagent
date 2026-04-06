package supabase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/golang-jwt/jwt/v5"
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
	// Dev mode: accept dev-* tokens for local development
	if p.isDev && len(token) > 4 && token[:4] == "dev-" {
		slog.Debug("using dev auth mode")
		return &port.AuthContext{
			UserID:   domain.UserID("00000000-0000-0000-0000-000000000001"),
			TenantID: domain.TenantID("00000000-0000-0000-0000-000000000010"),
			Role:     domain.RoleOwner,
		}, nil
	}

	if p.jwtSecret == "" {
		return nil, fmt.Errorf("JWT secret not configured")
	}

	// Parse and validate the Supabase JWT
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(p.jwtSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Extract user ID from "sub" claim (standard Supabase JWT)
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, fmt.Errorf("missing sub claim")
	}

	// Supabase stores metadata in app_metadata or user_metadata
	// For multi-tenant, tenant_id should be in app_metadata
	tenantID := ""
	if appMeta, ok := claims["app_metadata"].(map[string]any); ok {
		tenantID, _ = appMeta["tenant_id"].(string)
	}

	// Fallback: single-tenant mode uses a default tenant ID
	if tenantID == "" {
		tenantID = "00000000-0000-0000-0000-000000000010"
	}

	role := domain.RoleOwner
	if r, ok := claims["role"].(string); ok && r == "member" {
		role = domain.RoleMember
	}

	return &port.AuthContext{
		UserID:   domain.UserID(sub),
		TenantID: domain.TenantID(tenantID),
		Role:     role,
	}, nil
}
