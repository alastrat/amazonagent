package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/inngest"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
)

type ScanHandler struct {
	durableRuntime *inngest.DurableRuntime
}

func NewScanHandler(durableRuntime *inngest.DurableRuntime) *ScanHandler {
	return &ScanHandler{durableRuntime: durableRuntime}
}

func (h *ScanHandler) TriggerCategoryScan(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	if h.durableRuntime == nil {
		response.Error(w, http.StatusServiceUnavailable, "durable runtime not available")
		return
	}

	var req struct {
		MaxNodes int `json:"max_nodes"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.MaxNodes <= 0 {
		req.MaxNodes = 50
	}

	if err := h.durableRuntime.TriggerCategoryScan(r.Context(), ac.TenantID, req.MaxNodes); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to trigger scan: "+err.Error())
		return
	}

	response.JSON(w, http.StatusAccepted, map[string]any{
		"status":    "triggered",
		"max_nodes": req.MaxNodes,
	})
}
