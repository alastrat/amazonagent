package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type DealHandler struct {
	svc *service.DealService
}

func NewDealHandler(svc *service.DealService) *DealHandler {
	return &DealHandler{svc: svc}
}

func (h *DealHandler) List(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	q := r.URL.Query()

	filter := port.DealFilter{
		SortBy:  q.Get("sort_by"),
		SortDir: q.Get("sort_dir"),
		Limit:   50,
	}

	if v := q.Get("campaign_id"); v != "" {
		cid := domain.CampaignID(v)
		filter.CampaignID = &cid
	}
	if v := q.Get("status"); v != "" {
		s := domain.DealStatus(v)
		filter.Status = &s
	}
	if v := q.Get("search"); v != "" {
		filter.Search = &v
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	deals, total, err := h.svc.List(r.Context(), ac.TenantID, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"deals": deals,
		"total": total,
	})
}

func (h *DealHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	id := chi.URLParam(r, "id")

	deal, err := h.svc.GetByID(r.Context(), ac.TenantID, domain.DealID(id))
	if err != nil {
		response.Error(w, http.StatusNotFound, "deal not found")
		return
	}

	response.JSON(w, http.StatusOK, deal)
}

type dealDecisionRequest struct {
	Reason string `json:"reason,omitempty"`
}

func (h *DealHandler) Approve(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	id := chi.URLParam(r, "id")

	deal, err := h.svc.Approve(r.Context(), ac.TenantID, domain.DealID(id), string(ac.UserID))
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, deal)
}

func (h *DealHandler) Reject(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	id := chi.URLParam(r, "id")

	var req dealDecisionRequest
	json.NewDecoder(r.Body).Decode(&req)

	deal, err := h.svc.Reject(r.Context(), ac.TenantID, domain.DealID(id), string(ac.UserID), req.Reason)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, deal)
}
