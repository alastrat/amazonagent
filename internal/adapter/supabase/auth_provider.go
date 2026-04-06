package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type AuthProvider struct {
	jwtSecret string
	isDev     bool
	jwks      keyfunc.Keyfunc
}

func NewAuthProvider(jwtSecret string, isDev bool) *AuthProvider {
	return &AuthProvider{jwtSecret: jwtSecret, isDev: isDev}
}

func NewAuthProviderWithURL(jwtSecret, supabaseURL, supabaseAnonKey string, isDev bool) *AuthProvider {
	p := &AuthProvider{jwtSecret: jwtSecret, isDev: isDev}

	if supabaseURL != "" && supabaseAnonKey != "" {
		jwksURL := strings.TrimSuffix(supabaseURL, "/") + "/auth/v1/.well-known/jwks"
		jwksJSON, err := fetchJWKS(jwksURL, supabaseAnonKey)
		if err != nil {
			slog.Warn("auth: failed to fetch JWKS", "error", err.Error())
		} else {
			jwks, err := keyfunc.NewJWKSetJSON(jwksJSON)
			if err != nil {
				slog.Warn("auth: failed to parse JWKS", "error", err.Error())
			} else {
				p.jwks = jwks
				slog.Info("auth: JWKS initialized for ES256 verification")
			}
		}
	}

	return p
}

func fetchJWKS(url, apiKey string) (json.RawMessage, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS fetch failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(body), nil
}

func (p *AuthProvider) ValidateToken(ctx context.Context, token string) (*port.AuthContext, error) {
	if p.isDev && len(token) > 4 && token[:4] == "dev-" {
		slog.Debug("using dev auth mode")
		return &port.AuthContext{
			UserID:   domain.UserID("00000000-0000-0000-0000-000000000001"),
			TenantID: domain.TenantID("00000000-0000-0000-0000-000000000010"),
			Role:     domain.RoleOwner,
		}, nil
	}

	// Try JWKS verification first (ES256)
	if p.jwks != nil {
		parsed, err := jwt.Parse(token, p.jwks.KeyfuncCtx(ctx))
		if err == nil {
			return p.extractClaims(parsed)
		}
		slog.Debug("auth: JWKS verification failed", "error", err.Error())
	}

	// Fallback: HMAC with legacy secret
	if p.jwtSecret != "" {
		parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); ok {
				return []byte(p.jwtSecret), nil
			}
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		})
		if err == nil {
			return p.extractClaims(parsed)
		}
	}

	return nil, fmt.Errorf("invalid token")
}

func (p *AuthProvider) extractClaims(parsed *jwt.Token) (*port.AuthContext, error) {
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

	slog.Info("auth: token validated", "user_id", sub)

	return &port.AuthContext{
		UserID:   domain.UserID(sub),
		TenantID: domain.TenantID(tenantID),
		Role:     role,
	}, nil
}
