package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type SellerAccountHandler struct {
	svc *service.SellerAccountService
}

func NewSellerAccountHandler(svc *service.SellerAccountService) *SellerAccountHandler {
	return &SellerAccountHandler{svc: svc}
}

func (h *SellerAccountHandler) Connect(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	var input service.ConnectAccountInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	account, err := h.svc.ConnectAccount(r.Context(), ac.TenantID, input)
	if err != nil {
		if strings.Contains(err.Error(), "required") {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to connect seller account")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"id":             account.ID,
		"tenant_id":      account.TenantID,
		"seller_id":      account.SellerID,
		"marketplace_id": account.MarketplaceID,
		"status":         account.Status,
		"last_verified":  account.LastVerified,
		"error_message":  account.ErrorMessage,
		"created_at":     account.CreatedAt,
	})
}

func (h *SellerAccountHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	account, err := h.svc.GetAccount(r.Context(), ac.TenantID)
	if err != nil {
		// Not found is the expected case for unconnected tenants
		if strings.Contains(err.Error(), "no rows") {
			response.JSON(w, http.StatusOK, map[string]any{
				"connected": false,
			})
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get seller account")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"connected":      true,
		"id":             account.ID,
		"tenant_id":      account.TenantID,
		"seller_id":      account.SellerID,
		"marketplace_id": account.MarketplaceID,
		"status":         account.Status,
		"last_verified":  account.LastVerified,
		"error_message":  account.ErrorMessage,
		"created_at":     account.CreatedAt,
	})
}

func (h *SellerAccountHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	if err := h.svc.DisconnectAccount(r.Context(), ac.TenantID); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to disconnect seller account")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"disconnected": true,
	})
}
