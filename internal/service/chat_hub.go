package service

import (
	"log/slog"
	"sync"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// ChatEventType enumerates SSE event types for the chat stream.
type ChatEventType string

const (
	ChatEventMessage  ChatEventType = "message"   // assistant message (partial or full)
	ChatEventTyping   ChatEventType = "typing"     // assistant is thinking
	ChatEventDone     ChatEventType = "done"        // response complete
	ChatEventError    ChatEventType = "error"       // error occurred
	ChatEventToolCall ChatEventType = "tool_call"   // agent called a tool
)

// ChatEvent is streamed to the frontend via SSE.
type ChatEvent struct {
	Type      ChatEventType  `json:"type"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// chatStream holds per-tenant state for chat SSE.
type chatStream struct {
	mu          sync.RWMutex
	subscribers map[chan ChatEvent]struct{}
}

// ChatHub is a process-global pub/sub for chat events, keyed by tenant.
type ChatHub struct {
	mu      sync.RWMutex
	streams map[domain.TenantID]*chatStream
}

func NewChatHub() *ChatHub {
	return &ChatHub{
		streams: make(map[domain.TenantID]*chatStream),
	}
}

// Publish sends an event to all subscribers for a tenant.
func (h *ChatHub) Publish(tenantID domain.TenantID, evt ChatEvent) {
	h.mu.RLock()
	cs, ok := h.streams[tenantID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	cs.mu.RLock()
	defer cs.mu.RUnlock()
	for ch := range cs.subscribers {
		select {
		case ch <- evt:
		default:
			// subscriber too slow, drop event
		}
	}
}

// Subscribe returns a channel for receiving events and an unsubscribe function.
func (h *ChatHub) Subscribe(tenantID domain.TenantID) (chan ChatEvent, func()) {
	h.mu.Lock()
	cs, ok := h.streams[tenantID]
	if !ok {
		cs = &chatStream{
			subscribers: make(map[chan ChatEvent]struct{}),
		}
		h.streams[tenantID] = cs
	}
	h.mu.Unlock()

	sub := make(chan ChatEvent, 32)
	cs.mu.Lock()
	cs.subscribers[sub] = struct{}{}
	cs.mu.Unlock()

	unsub := func() {
		cs.mu.Lock()
		delete(cs.subscribers, sub)
		cs.mu.Unlock()
	}

	slog.Debug("chat-hub: subscriber added", "tenant_id", tenantID)
	return sub, unsub
}
