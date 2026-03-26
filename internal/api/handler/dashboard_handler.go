package handler

import (
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type DashboardHandler struct {
	campaigns *service.CampaignService
	deals     *service.DealService
}

func NewDashboardHandler(campaigns *service.CampaignService, deals *service.DealService) *DashboardHandler {
	return &DashboardHandler{campaigns: campaigns, deals: deals}
}

func (h *DashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	needsReview := domain.DealStatusNeedsReview
	_, reviewCount, _ := h.deals.List(r.Context(), ac.TenantID, port.DealFilter{Status: &needsReview, Limit: 0})

	approved := domain.DealStatusApproved
	_, approvedCount, _ := h.deals.List(r.Context(), ac.TenantID, port.DealFilter{Status: &approved, Limit: 0})

	running := domain.CampaignStatusRunning
	activeCampaigns, _ := h.campaigns.List(r.Context(), ac.TenantID, port.CampaignFilter{Status: &running})

	recentDeals, _, _ := h.deals.List(r.Context(), ac.TenantID, port.DealFilter{Limit: 5, SortBy: "created_at", SortDir: "desc"})

	response.JSON(w, http.StatusOK, map[string]any{
		"deals_pending_review": reviewCount,
		"deals_approved":       approvedCount,
		"active_campaigns":     len(activeCampaigns),
		"recent_deals":         recentDeals,
	})
}
