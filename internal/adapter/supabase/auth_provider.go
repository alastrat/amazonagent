package supabase

import (
	"context"
	"encoding/base64"
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

	// First, peek at the token to see algorithm and claims (without verification)
	parser := jwt.NewParser()
	unverified, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		slog.Warn("auth: cannot parse token", "error", err.Error())
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	alg := "unknown"
	if unverified.Header != nil {
		if a, ok := unverified.Header["alg"].(string); ok {
			alg = a
		}
	}
	slog.Info("auth: token algorithm", "alg", alg, "secret_len", len(p.jwtSecret))

	// Try HMAC verification with raw secret
	secretBytes := []byte(p.jwtSecret)
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); ok {
			return secretBytes, nil
		}
		return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
	})

	// If failed, try with base64-decoded secret
	if err != nil {
		slog.Warn("auth: HMAC raw failed", "error", err.Error())
		decoded, decErr := base64.StdEncoding.DecodeString(p.jwtSecret)
		if decErr == nil {
			parsed, err = jwt.Parse(token, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); ok {
					return decoded, nil
				}
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			})
			if err != nil {
				slog.Warn("auth: HMAC base64 failed", "error", err.Error())
			}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, fmt.Errorf("missing sub claim")
	}

	tenantID := ""
	if appMeta, ok := claims["app_metadata"].(map[string]any); ok {
		tenantID, _ = appMeta["tenant_id"].(string)
	}
	if tenantID == "" {
		tenantID = "00000000-0000-0000-0000-000000000010"
	}

	role := domain.RoleOwner
	if r, ok := claims["role"].(string); ok && r == "member" {
		role = domain.RoleMember
	}

	slog.Info("auth: token validated", "user_id", sub, "tenant_id", tenantID)

	return &port.AuthContext{
		UserID:   domain.UserID(sub),
		TenantID: domain.TenantID(tenantID),
		Role:     role,
	}, nil
}
