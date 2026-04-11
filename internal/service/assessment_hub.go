package service

import (
	"log/slog"
	"sync"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// AssessmentEventType enumerates SSE event types.
type AssessmentEventType string

const (
	EventCategoryStart      AssessmentEventType = "category_start"
	EventProductFound       AssessmentEventType = "product_found"
	EventCategoryComplete   AssessmentEventType = "category_complete"
	EventPhaseChange        AssessmentEventType = "phase_change"
	EventAssessmentComplete AssessmentEventType = "assessment_complete"
)

// AssessmentEvent is the unit published to subscribers.
type AssessmentEvent struct {
	Type      AssessmentEventType `json:"type"`
	Timestamp time.Time           `json:"timestamp"`
	Data      map[string]any      `json:"data"`
}

const maxHistory = 500

// tenantStream holds per-tenant state: subscribers + history for late-join.
type tenantStream struct {
	mu          sync.RWMutex
	subscribers map[chan AssessmentEvent]struct{}
	history     []AssessmentEvent
	done        bool
}

// AssessmentHub is a process-global pub/sub for assessment progress events.
type AssessmentHub struct {
	mu      sync.RWMutex
	streams map[domain.TenantID]*tenantStream
}

func NewAssessmentHub() *AssessmentHub {
	return &AssessmentHub{
		streams: make(map[domain.TenantID]*tenantStream),
	}
}

// StartStream initialises a tenant's event stream. Called at the start of an assessment.
func (h *AssessmentHub) StartStream(tenantID domain.TenantID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If a previous stream exists, close it first
	if old, ok := h.streams[tenantID]; ok {
		old.mu.Lock()
		old.done = true
		for ch := range old.subscribers {
			close(ch)
		}
		old.subscribers = nil
		old.mu.Unlock()
	}

	h.streams[tenantID] = &tenantStream{
		subscribers: make(map[chan AssessmentEvent]struct{}),
		history:     make([]AssessmentEvent, 0, 64),
	}
	slog.Debug("assessment-hub: stream started", "tenant_id", tenantID)
}

// EndStream marks the stream as done, closes all subscriber channels,
// and schedules cleanup after a grace period.
func (h *AssessmentHub) EndStream(tenantID domain.TenantID) {
	h.mu.RLock()
	ts, ok := h.streams[tenantID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	ts.mu.Lock()
	ts.done = true
	for ch := range ts.subscribers {
		close(ch)
	}
	ts.subscribers = nil
	ts.mu.Unlock()

	// Grace period for late SSE reconnects to get catch-up history.
	// Guard: only delete if the stream is still the same one (a rescan could have started a new one).
	staleStream := ts
	go func() {
		time.Sleep(30 * time.Second)
		h.mu.Lock()
		if h.streams[tenantID] == staleStream {
			delete(h.streams, tenantID)
			slog.Debug("assessment-hub: stream cleaned up", "tenant_id", tenantID)
		}
		h.mu.Unlock()
	}()
}

// Publish sends an event to all subscribers and appends to history.
// Non-blocking: if a subscriber's channel is full, the event is dropped for that subscriber.
func (h *AssessmentHub) Publish(tenantID domain.TenantID, evt AssessmentEvent) {
	h.mu.RLock()
	ts, ok := h.streams[tenantID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.done {
		return
	}

	// Append to history (bounded)
	if len(ts.history) < maxHistory {
		ts.history = append(ts.history, evt)
	}

	// Fan out to subscribers (non-blocking)
	for ch := range ts.subscribers {
		select {
		case ch <- evt:
		default:
			// Subscriber too slow, drop event
		}
	}
}

// Subscribe returns a channel for receiving events, the current history (for catch-up),
// and an unsubscribe function. If no stream exists, returns a closed channel and nil history.
func (h *AssessmentHub) Subscribe(tenantID domain.TenantID) (ch chan AssessmentEvent, history []AssessmentEvent, unsub func()) {
	h.mu.RLock()
	ts, ok := h.streams[tenantID]
	h.mu.RUnlock()

	if !ok {
		// No active stream — return closed channel
		closed := make(chan AssessmentEvent)
		close(closed)
		return closed, nil, func() {}
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.done {
		closed := make(chan AssessmentEvent)
		close(closed)
		// Return history even if done (for catch-up on completed assessments)
		snapshot := make([]AssessmentEvent, len(ts.history))
		copy(snapshot, ts.history)
		return closed, snapshot, func() {}
	}

	sub := make(chan AssessmentEvent, 64)
	ts.subscribers[sub] = struct{}{}

	// Snapshot history for catch-up
	snapshot := make([]AssessmentEvent, len(ts.history))
	copy(snapshot, ts.history)

	unsub = func() {
		ts.mu.Lock()
		defer ts.mu.Unlock()
		delete(ts.subscribers, sub)
	}

	return sub, snapshot, unsub
}
