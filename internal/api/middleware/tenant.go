package middleware

import (
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
)

func RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac := GetAuthContext(r.Context())
		if ac == nil || ac.TenantID == "" {
			response.Error(w, http.StatusForbidden, "no tenant context")
			return
		}
		next.ServeHTTP(w, r)
	})
}
