package handler

import (
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type CreditHandler struct {
	credits *service.CreditService
}

func NewCreditHandler(credits *service.CreditService) *CreditHandler {
	return &CreditHandler{credits: credits}
}

func (h *CreditHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	account, err := h.credits.GetBalance(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to get credit balance")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"tier":           account.Tier,
		"monthly_limit":  account.MonthlyLimit,
		"used":           account.UsedThisMonth,
		"remaining":      account.Remaining(),
		"reset_at":       account.ResetAt,
	})
}

func (h *CreditHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	txs, err := h.credits.GetTransactions(r.Context(), ac.TenantID, 50)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to get transactions")
		return
	}

	response.JSON(w, http.StatusOK, txs)
}
