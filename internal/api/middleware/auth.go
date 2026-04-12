package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// AuthContextKey is exported for test injection of auth context.
var AuthContextKey = ctxKeyAuth{}

type ctxKeyAuth struct{}

func Auth(provider port.AuthProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			// Fallback: allow token via query param (needed for EventSource/SSE)
			if header == "" {
				if qToken := r.URL.Query().Get("token"); qToken != "" {
					header = "Bearer " + qToken
				}
			}
			if header == "" {
				response.Error(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			if token == header {
				response.Error(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}

			authCtx, err := provider.ValidateToken(r.Context(), token)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyAuth{}, authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetAuthContext(ctx context.Context) *port.AuthContext {
	ac, _ := ctx.Value(ctxKeyAuth{}).(*port.AuthContext)
	return ac
}
