package handler

import (
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type EventHandler struct {
	svc *service.EventService
}

func NewEventHandler(svc *service.EventService) *EventHandler {
	return &EventHandler{svc: svc}
}

func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	q := r.URL.Query()

	filter := port.EventFilter{Limit: 100}
	if v := q.Get("entity_type"); v != "" {
		filter.EntityType = &v
	}
	if v := q.Get("entity_id"); v != "" {
		filter.EntityID = &v
	}

	events, err := h.svc.List(r.Context(), ac.TenantID, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, events)
}
