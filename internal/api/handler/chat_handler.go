package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type ChatHandler struct {
	chat *service.ChatService
	hub  *service.ChatHub
}

func NewChatHandler(chat *service.ChatService, hub *service.ChatHub) *ChatHandler {
	return &ChatHandler{chat: chat, hub: hub}
}

// Send handles POST /chat/send — receives a user message, triggers agent response.
func (h *ChatHandler) Send(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Message == "" {
		response.Error(w, http.StatusBadRequest, "message is required")
		return
	}

	// Send message asynchronously — response streams via SSE.
	// Use background context since the HTTP request context cancels when we respond.
	tenantID := ac.TenantID
	message := req.Message
	go func() {
		ctx := context.Background()
		if _, err := h.chat.SendMessage(ctx, tenantID, message); err != nil {
			slog.Error("chat: async send failed", "tenant_id", tenantID, "error", err)
		}
	}()

	response.JSON(w, http.StatusAccepted, map[string]string{
		"status": "accepted",
	})
}

// Events handles GET /chat/events — SSE stream for chat events.
func (h *ChatHandler) Events(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	flusher, ok := w.(http.Flusher)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, unsub := h.hub.Subscribe(ac.TenantID)
	defer unsub()

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// History handles GET /chat/history — returns recent messages.
func (h *ChatHandler) History(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	messages, err := h.chat.GetHistory(r.Context(), ac.TenantID, 50)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to load history: "+err.Error())
		return
	}
	if messages == nil {
		messages = []domain.ChatMessage{}
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"messages": messages,
	})
}
