package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type SuggestionHandler struct {
	queue *service.DiscoveryQueueService
}

func NewSuggestionHandler(queue *service.DiscoveryQueueService) *SuggestionHandler {
	return &SuggestionHandler{queue: queue}
}

func (h *SuggestionHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	suggestions, err := h.queue.ListPending(r.Context(), ac.TenantID, 50)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list suggestions")
		return
	}
	response.JSON(w, http.StatusOK, suggestions)
}

func (h *SuggestionHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	suggestions, err := h.queue.ListAll(r.Context(), ac.TenantID, 50)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list suggestions")
		return
	}
	response.JSON(w, http.StatusOK, suggestions)
}

func (h *SuggestionHandler) Accept(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	id := chi.URLParam(r, "id")

	// TODO: create deal from suggestion, pass deal ID
	dealID := domain.DealID("pending")
	if err := h.queue.AcceptSuggestion(r.Context(), ac.TenantID, domain.SuggestionID(id), dealID); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to accept suggestion")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (h *SuggestionHandler) Dismiss(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.queue.DismissSuggestion(r.Context(), ac.TenantID, domain.SuggestionID(id)); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to dismiss suggestion")
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}
