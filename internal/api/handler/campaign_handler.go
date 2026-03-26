package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type CampaignHandler struct {
	svc *service.CampaignService
}

func NewCampaignHandler(svc *service.CampaignService) *CampaignHandler {
	return &CampaignHandler{svc: svc}
}

type createCampaignRequest struct {
	Type        domain.CampaignType `json:"type"`
	TriggerType domain.TriggerType  `json:"trigger_type"`
	Criteria    domain.Criteria     `json:"criteria"`
	SourceFile  *string             `json:"source_file,omitempty"`
}

func (h *CampaignHandler) Create(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	var req createCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	campaign, err := h.svc.Create(r.Context(), service.CreateCampaignInput{
		TenantID:    ac.TenantID,
		Type:        req.Type,
		TriggerType: req.TriggerType,
		Criteria:    req.Criteria,
		SourceFile:  req.SourceFile,
		CreatedBy:   string(ac.UserID),
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, campaign)
}

func (h *CampaignHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	id := chi.URLParam(r, "id")

	campaign, err := h.svc.GetByID(r.Context(), ac.TenantID, domain.CampaignID(id))
	if err != nil {
		response.Error(w, http.StatusNotFound, "campaign not found")
		return
	}

	response.JSON(w, http.StatusOK, campaign)
}

func (h *CampaignHandler) List(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	campaigns, err := h.svc.List(r.Context(), ac.TenantID, port.CampaignFilter{
		Limit: 50,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, campaigns)
}
