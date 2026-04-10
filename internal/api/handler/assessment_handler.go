package handler

import (
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

	// No longer collecting account age / listings / capital — inferred post-assessment
	if r.Body != nil {
		// Drain body for compatibility with old clients sending JSON
		r.Body.Close()
	}

	// Create profile synchronously
	profile, err := h.assessment.StartAssessment(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to start assessment: "+err.Error())
		return
	}

	// Trigger Inngest workflow for the async discovery assessment
	if h.durableRuntime != nil {
		slog.Info("assessment: triggering inngest workflow", "tenant_id", ac.TenantID)
		if err := h.durableRuntime.TriggerAssessment(r.Context(), ac.TenantID); err != nil {
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

func (h *AssessmentHandler) GetGraph(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	profile, _ := h.assessment.GetProfile(r.Context(), ac.TenantID)
	fingerprint, _ := h.assessment.GetFingerprint(r.Context(), ac.TenantID)

	status := "pending"
	if profile != nil {
		status = string(profile.AssessmentStatus)
	}

	// Build graph nodes from fingerprint
	nodes := []map[string]any{
		{"id": "root", "type": "root", "label": "Amazon US", "status": "scanned"},
	}
	edges := []map[string]any{}

	if fingerprint != nil {
		for _, cat := range fingerprint.Categories {
			catID := "cat-" + cat.Category
			catStatus := "scanned"
			if cat.OpenCount > 0 {
				catStatus = "eligible"
			} else {
				catStatus = "restricted"
			}
			nodes = append(nodes, map[string]any{
				"id": catID, "type": "category", "label": cat.Category,
				"status": catStatus, "open_rate": cat.OpenRate,
			})
			edges = append(edges, map[string]any{"source": "root", "target": catID})
		}

		for _, br := range fingerprint.BrandResults {
			brandID := "brand-" + br.Brand + "-" + br.ASIN
			brandStatus := "restricted"
			if br.Eligible {
				brandStatus = "eligible"
			}
			nodes = append(nodes, map[string]any{
				"id": brandID, "type": "product", "label": br.ASIN,
				"status": brandStatus, "eligible": br.Eligible,
			})
			catID := "cat-" + br.Category
			edges = append(edges, map[string]any{"source": catID, "target": brandID})
		}
	}

	stats := map[string]any{
		"categories_scanned": 0, "categories_total": 20,
		"eligible_products": 0, "restricted_products": 0,
	}
	if fingerprint != nil {
		stats["categories_scanned"] = len(fingerprint.Categories)
		stats["eligible_products"] = fingerprint.TotalEligible
		stats["restricted_products"] = fingerprint.TotalRestricted
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"status": status,
		"graph":  map[string]any{"nodes": nodes, "edges": edges, "stats": stats},
	})
}

func (h *AssessmentHandler) Reset(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	if err := h.assessment.ResetAssessment(r.Context(), ac.TenantID); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to reset assessment: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{"status": "reset"})
}
