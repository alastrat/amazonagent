package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type StrategyHandler struct {
	strategy *service.StrategyService
}

func NewStrategyHandler(strategy *service.StrategyService) *StrategyHandler {
	return &StrategyHandler{strategy: strategy}
}

func (h *StrategyHandler) GetActive(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	sv, err := h.strategy.GetActive(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "no active strategy")
		return
	}
	response.JSON(w, http.StatusOK, sv)
}

func (h *StrategyHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	versions, err := h.strategy.ListVersions(r.Context(), ac.TenantID, 20)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list versions")
		return
	}
	response.JSON(w, http.StatusOK, versions)
}

func (h *StrategyHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	versionID := chi.URLParam(r, "id")

	sv, err := h.strategy.GetVersion(r.Context(), ac.TenantID, domain.StrategyVersionID(versionID))
	if err != nil {
		response.Error(w, http.StatusNotFound, "version not found")
		return
	}
	response.JSON(w, http.StatusOK, sv)
}

func (h *StrategyHandler) ActivateVersion(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	versionID := chi.URLParam(r, "id")

	if err := h.strategy.ActivateVersion(r.Context(), ac.TenantID, domain.StrategyVersionID(versionID)); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "activated"})
}

func (h *StrategyHandler) RollbackToVersion(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	versionID := chi.URLParam(r, "id")

	newVersion, err := h.strategy.RollbackToVersion(r.Context(), ac.TenantID, domain.StrategyVersionID(versionID))
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, newVersion)
}
