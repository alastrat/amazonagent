package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type SettingsHandler struct {
	svc *service.TenantSettingsService
}

func NewSettingsHandler(svc *service.TenantSettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	settings, err := h.svc.Get(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, settings)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	var settings domain.TenantSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	settings.TenantID = ac.TenantID
	if err := h.svc.Update(r.Context(), &settings); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, settings)
}
