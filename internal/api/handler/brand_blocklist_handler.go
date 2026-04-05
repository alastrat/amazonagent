package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type BrandBlocklistHandler struct {
	svc *service.BrandBlocklistService
}

func NewBrandBlocklistHandler(svc *service.BrandBlocklistService) *BrandBlocklistHandler {
	return &BrandBlocklistHandler{svc: svc}
}

func (h *BrandBlocklistHandler) List(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	brands, err := h.svc.List(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, brands)
}

type addBrandRequest struct {
	Brand  string `json:"brand"`
	Reason string `json:"reason"`
}

func (h *BrandBlocklistHandler) Add(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	var req addBrandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Brand == "" {
		response.Error(w, http.StatusBadRequest, "brand is required")
		return
	}
	if err := h.svc.Add(r.Context(), ac.TenantID, req.Brand, req.Reason, domain.BlockedBrandSourceManual, ""); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusCreated, map[string]string{"status": "added", "brand": req.Brand})
}

type removeBrandRequest struct {
	Brand string `json:"brand"`
}

func (h *BrandBlocklistHandler) Remove(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	var req removeBrandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.svc.Remove(r.Context(), ac.TenantID, req.Brand); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "removed", "brand": req.Brand})
}
