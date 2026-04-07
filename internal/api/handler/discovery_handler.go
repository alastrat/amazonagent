package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type DiscoveryHandler struct {
	svc *service.DiscoveryService
}

func NewDiscoveryHandler(svc *service.DiscoveryService) *DiscoveryHandler {
	return &DiscoveryHandler{svc: svc}
}

func (h *DiscoveryHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	dc, err := h.svc.Get(r.Context(), ac.TenantID)
	if err != nil {
		// Return default config instead of 404
		defaults := domain.DefaultDiscoveryConfig(ac.TenantID)
		response.JSON(w, http.StatusOK, defaults)
		return
	}

	response.JSON(w, http.StatusOK, dc)
}

func (h *DiscoveryHandler) Update(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	var dc domain.DiscoveryConfig
	if err := json.NewDecoder(r.Body).Decode(&dc); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	dc.TenantID = ac.TenantID

	if err := h.svc.Update(r.Context(), &dc); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, dc)
}
