package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// ---------------------------------------------------------------------------
// In-memory mock: ChatRepo
// ---------------------------------------------------------------------------

type memChatRepo struct {
	mu       sync.Mutex
	sessions map[domain.TenantID]*domain.ChatSession
	messages []domain.ChatMessage
}

func newMemChatRepo() *memChatRepo {
	return &memChatRepo{
		sessions: make(map[domain.TenantID]*domain.ChatSession),
	}
}

func (m *memChatRepo) CreateSession(_ context.Context, session *domain.ChatSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *session
	m.sessions[session.TenantID] = &cp
	return nil
}

func (m *memChatRepo) GetSession(_ context.Context, tenantID domain.TenantID) (*domain.ChatSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[tenantID]
	if !ok {
		return nil, fmt.Errorf("session not found for tenant %s", tenantID)
	}
	cp := *s
	return &cp, nil
}

func (m *memChatRepo) UpdateSession(_ context.Context, session *domain.ChatSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *session
	m.sessions[session.TenantID] = &cp
	return nil
}

func (m *memChatRepo) SaveMessage(_ context.Context, msg *domain.ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, *msg)
	return nil
}

func (m *memChatRepo) ListMessages(_ context.Context, tenantID domain.TenantID, limit int) ([]domain.ChatMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []domain.ChatMessage
	for _, msg := range m.messages {
		if msg.TenantID == tenantID {
			result = append(result, msg)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// In-memory mock: ConversationalRuntime
// ---------------------------------------------------------------------------

type memConversationalRuntime struct {
	mu            sync.Mutex
	sessionID     string
	response      *domain.AgentOutput
	sendErr       error
	startCalls    int
	sendCalls     int
}

func newMemConversationalRuntime() *memConversationalRuntime {
	return &memConversationalRuntime{
		sessionID: "agent-session-001",
		response: &domain.AgentOutput{
			Raw:        "Hello! How can I help?",
			Structured: map[string]any{"message": "Hello! How can I help?"},
			TokensUsed: 100,
			DurationMs: 500,
		},
	}
}

func (m *memConversationalRuntime) RunAgent(_ context.Context, _ domain.AgentTask) (*domain.AgentOutput, error) {
	return m.response, nil
}

func (m *memConversationalRuntime) StartSession(_ context.Context, _ domain.TenantID, _ port.SessionConfig) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalls++
	return m.sessionID, nil
}

func (m *memConversationalRuntime) SendMessage(_ context.Context, _ string, _ string) (*domain.AgentOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCalls++
	if m.sendErr != nil {
		return nil, m.sendErr
	}
	return m.response, nil
}

func (m *memConversationalRuntime) EndSession(_ context.Context, _ string) error {
	return nil
}

// ---------------------------------------------------------------------------
// Sequential ID generator for chat tests
// ---------------------------------------------------------------------------

type chatIDGen struct {
	mu      sync.Mutex
	counter int
}

func (g *chatIDGen) New() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return fmt.Sprintf("chat-id-%d", g.counter)
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

func newChatTestHarness() (*ChatService, *memChatRepo, *memConversationalRuntime, *ChatHub) {
	repo := newMemChatRepo()
	runtime := newMemConversationalRuntime()
	hub := NewChatHub()
	profiles := newMemSellerProfileRepo()
	fingerprints := newMemFingerprintRepo()
	idGen := &chatIDGen{}

	svc := NewChatService(repo, runtime, hub, idGen, profiles, fingerprints)
	return svc, repo, runtime, hub
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestGetOrCreateSession_CreatesNew(t *testing.T) {
	svc, repo, _, _ := newChatTestHarness()
	ctx := context.Background()
	tenantID := domain.TenantID("tenant-chat-1")

	session, err := svc.GetOrCreateSession(ctx, tenantID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if session.TenantID != tenantID {
		t.Errorf("TenantID = %q, want %q", session.TenantID, tenantID)
	}
	if session.Status != domain.ChatSessionActive {
		t.Errorf("Status = %q, want %q", session.Status, domain.ChatSessionActive)
	}
	if session.AgentSessionID == "" {
		t.Error("AgentSessionID should not be empty")
	}

	// Verify persisted
	stored, err := repo.GetSession(ctx, tenantID)
	if err != nil {
		t.Fatalf("session not persisted: %v", err)
	}
	if stored.ID != session.ID {
		t.Errorf("stored ID = %q, want %q", stored.ID, session.ID)
	}
}

func TestGetOrCreateSession_ReturnsExistingActive(t *testing.T) {
	svc, _, runtime, _ := newChatTestHarness()
	ctx := context.Background()
	tenantID := domain.TenantID("tenant-chat-2")

	first, err := svc.GetOrCreateSession(ctx, tenantID)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	second, err := svc.GetOrCreateSession(ctx, tenantID)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("expected same session ID, got %q and %q", first.ID, second.ID)
	}

	// StartSession is now called on every GetOrCreateSession (for restart recovery),
	// but the DB session should be reused (same ID returned both times).
	runtime.mu.Lock()
	startCalls := runtime.startCalls
	runtime.mu.Unlock()
	if startCalls != 2 {
		t.Errorf("StartSession called %d times, want 2 (called each time for recovery)", startCalls)
	}
}

func TestSendMessage_HappyPath(t *testing.T) {
	svc, repo, _, hub := newChatTestHarness()
	ctx := context.Background()
	tenantID := domain.TenantID("tenant-chat-3")

	// Subscribe to SSE events
	ch, unsub := hub.Subscribe(tenantID)
	defer unsub()

	msg, err := svc.SendMessage(ctx, tenantID, "Find me profitable products")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected non-nil assistant message")
	}
	if msg.Role != domain.ChatRoleAssistant {
		t.Errorf("Role = %q, want %q", msg.Role, domain.ChatRoleAssistant)
	}
	if msg.Content == "" {
		t.Error("assistant message content should not be empty")
	}

	// Verify both user and assistant messages were saved
	repo.mu.Lock()
	msgCount := len(repo.messages)
	repo.mu.Unlock()
	if msgCount != 2 {
		t.Errorf("saved %d messages, want 2 (user + assistant)", msgCount)
	}

	// Verify SSE events were published (typing, message, done)
	var events []ChatEvent
	timeout := time.After(time.Second)
	for {
		select {
		case evt := <-ch:
			events = append(events, evt)
			if evt.Type == ChatEventDone {
				goto checkEvents
			}
		case <-timeout:
			goto checkEvents
		}
	}
checkEvents:
	if len(events) < 3 {
		t.Errorf("expected at least 3 SSE events (typing, message, done), got %d", len(events))
	}
}

func TestSendMessage_RuntimeError(t *testing.T) {
	svc, _, runtime, hub := newChatTestHarness()
	ctx := context.Background()
	tenantID := domain.TenantID("tenant-chat-4")

	runtime.mu.Lock()
	runtime.sendErr = errors.New("model overloaded")
	runtime.mu.Unlock()

	// Subscribe to catch the error event
	ch, unsub := hub.Subscribe(tenantID)
	defer unsub()

	_, err := svc.SendMessage(ctx, tenantID, "hello")
	if err == nil {
		t.Fatal("expected error from runtime failure")
	}

	// Verify error event was published
	select {
	case evt := <-ch:
		// First event is typing, skip to find error
		if evt.Type == ChatEventTyping {
			select {
			case evt = <-ch:
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for error event")
			}
		}
		if evt.Type != ChatEventError {
			t.Errorf("expected error event, got %q", evt.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SSE events")
	}
}

func TestSendMessage_FallsBackToStructuredMessage(t *testing.T) {
	svc, _, runtime, _ := newChatTestHarness()
	ctx := context.Background()
	tenantID := domain.TenantID("tenant-chat-5")

	runtime.mu.Lock()
	runtime.response = &domain.AgentOutput{
		Raw:        "", // empty Raw
		Structured: map[string]any{"message": "structured fallback"},
		TokensUsed: 50,
		DurationMs: 200,
	}
	runtime.mu.Unlock()

	msg, err := svc.SendMessage(ctx, tenantID, "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "structured fallback" {
		t.Errorf("Content = %q, want %q", msg.Content, "structured fallback")
	}
}

func TestGetHistory_DefaultsTo50WhenLimitZero(t *testing.T) {
	svc, repo, _, _ := newChatTestHarness()
	ctx := context.Background()
	tenantID := domain.TenantID("tenant-chat-6")

	// Insert 60 messages directly into the repo
	repo.mu.Lock()
	for i := 0; i < 60; i++ {
		repo.messages = append(repo.messages, domain.ChatMessage{
			ID:        fmt.Sprintf("msg-%d", i),
			TenantID:  tenantID,
			SessionID: "session-1",
			Role:      domain.ChatRoleUser,
			Content:   fmt.Sprintf("message %d", i),
			CreatedAt: time.Now(),
		})
	}
	repo.mu.Unlock()

	messages, err := svc.GetHistory(ctx, tenantID, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 50 {
		t.Errorf("got %d messages, want 50 (default limit)", len(messages))
	}
}

func TestGetHistory_ReturnsMessagesInOrder(t *testing.T) {
	svc, repo, _, _ := newChatTestHarness()
	ctx := context.Background()
	tenantID := domain.TenantID("tenant-chat-7")

	repo.mu.Lock()
	for i := 0; i < 5; i++ {
		repo.messages = append(repo.messages, domain.ChatMessage{
			ID:        fmt.Sprintf("msg-%d", i),
			TenantID:  tenantID,
			SessionID: "session-1",
			Role:      domain.ChatRoleUser,
			Content:   fmt.Sprintf("message %d", i),
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		})
	}
	repo.mu.Unlock()

	messages, err := svc.GetHistory(ctx, tenantID, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(messages) != 5 {
		t.Fatalf("got %d messages, want 5", len(messages))
	}
	for i, msg := range messages {
		expected := fmt.Sprintf("message %d", i)
		if msg.Content != expected {
			t.Errorf("messages[%d].Content = %q, want %q", i, msg.Content, expected)
		}
	}
}
