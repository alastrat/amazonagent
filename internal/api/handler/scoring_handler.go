package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type ScoringHandler struct {
	svc *service.ScoringService
}

func NewScoringHandler(svc *service.ScoringService) *ScoringHandler {
	return &ScoringHandler{svc: svc}
}

func (h *ScoringHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	sc, err := h.svc.GetActive(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "no scoring config found")
		return
	}

	response.JSON(w, http.StatusOK, sc)
}

type updateScoringRequest struct {
	Weights    domain.ScoringWeights `json:"weights"`
	Thresholds domain.Thresholds     `json:"thresholds"`
}

func (h *ScoringHandler) Update(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	var req updateScoringRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sc, err := h.svc.Update(r.Context(), ac.TenantID, req.Weights, req.Thresholds)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, sc)
}
