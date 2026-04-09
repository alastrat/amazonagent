package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/inngest"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type AssessmentHandler struct {
	assessment     *service.AssessmentService
	durableRuntime *inngest.DurableRuntime
}

func NewAssessmentHandler(assessment *service.AssessmentService, durableRuntime *inngest.DurableRuntime) *AssessmentHandler {
	return &AssessmentHandler{assessment: assessment, durableRuntime: durableRuntime}
}

func (h *AssessmentHandler) Start(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	var req struct {
		AccountAgeDays int     `json:"account_age_days"`
		ActiveListings int     `json:"active_listings"`
		StatedCapital  float64 `json:"stated_capital"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	// Create profile synchronously
	profile, err := h.assessment.StartAssessment(r.Context(), ac.TenantID, req.AccountAgeDays, req.ActiveListings, req.StatedCapital)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to start assessment: "+err.Error())
		return
	}

	// Trigger Inngest workflow for the async scan + strategy generation
	if h.durableRuntime != nil {
		slog.Info("assessment: triggering inngest workflow", "tenant_id", ac.TenantID)
		if err := h.durableRuntime.TriggerAssessment(r.Context(), ac.TenantID, req.AccountAgeDays, req.ActiveListings, req.StatedCapital); err != nil {
			slog.Error("assessment: failed to trigger inngest", "tenant_id", ac.TenantID, "error", err)
			response.Error(w, http.StatusInternalServerError, "failed to trigger assessment workflow: "+err.Error())
			return
		}
		slog.Info("assessment: inngest event sent", "tenant_id", ac.TenantID)
	} else {
		slog.Warn("assessment: no durable runtime available, scan will not run")
	}

	response.JSON(w, http.StatusOK, profile)
}

func (h *AssessmentHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	profile, err := h.assessment.GetProfile(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "no assessment found")
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"status":    profile.AssessmentStatus,
		"archetype": profile.Archetype,
	})
}

func (h *AssessmentHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	profile, err := h.assessment.GetProfile(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "no profile found")
		return
	}

	fingerprint, _ := h.assessment.GetFingerprint(r.Context(), ac.TenantID)

	response.JSON(w, http.StatusOK, map[string]any{
		"profile":     profile,
		"fingerprint": fingerprint,
	})
}
