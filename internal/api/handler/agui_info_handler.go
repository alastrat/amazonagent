package handler

import (
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
)

// Info handles GET /api/copilotkit/info — returns available agents for CopilotKit discovery.
func (h *AGUIHandler) Info(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]any{
		"agents": []map[string]string{
			{
				"name":        "concierge",
				"description": "FBA Wholesale Concierge",
			},
		},
	})
}
