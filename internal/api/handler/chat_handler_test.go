package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

// --- test mocks ---

type memChatRepo struct {
	sessions []domain.ChatSession
	messages []domain.ChatMessage
}

func (r *memChatRepo) CreateSession(_ context.Context, s *domain.ChatSession) error {
	r.sessions = append(r.sessions, *s)
	return nil
}
func (r *memChatRepo) GetSession(_ context.Context, tenantID domain.TenantID) (*domain.ChatSession, error) {
	for i := len(r.sessions) - 1; i >= 0; i-- {
		if r.sessions[i].TenantID == tenantID && r.sessions[i].Status == domain.ChatSessionActive {
			return &r.sessions[i], nil
		}
	}
	return nil, nil
}
func (r *memChatRepo) UpdateSession(_ context.Context, s *domain.ChatSession) error {
	for i, existing := range r.sessions {
		if existing.ID == s.ID {
			r.sessions[i] = *s
			return nil
		}
	}
	return nil
}
func (r *memChatRepo) SaveMessage(_ context.Context, msg *domain.ChatMessage) error {
	r.messages = append(r.messages, *msg)
	return nil
}
func (r *memChatRepo) ListMessages(_ context.Context, tenantID domain.TenantID, limit int) ([]domain.ChatMessage, error) {
	var msgs []domain.ChatMessage
	for _, m := range r.messages {
		if m.TenantID == tenantID {
			msgs = append(msgs, m)
		}
	}
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	return msgs, nil
}

type mockConvRuntime struct {
	response string
}

func (r *mockConvRuntime) RunAgent(_ context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	return &domain.AgentOutput{Raw: "mock", Structured: map[string]any{"message": "mock"}}, nil
}
func (r *mockConvRuntime) StartSession(_ context.Context, _ domain.TenantID, _ port.SessionConfig) (string, error) {
	return "test-session-id", nil
}
func (r *mockConvRuntime) SendMessage(_ context.Context, _ string, msg string) (*domain.AgentOutput, error) {
	return &domain.AgentOutput{
		Raw:        r.response,
		Structured: map[string]any{"message": r.response},
		TokensUsed: 100,
		DurationMs: 50,
	}, nil
}
func (r *mockConvRuntime) EndSession(_ context.Context, _ string) error { return nil }

type seqIDGen struct{ n int }

func (g *seqIDGen) New() string {
	g.n++
	return fmt.Sprintf("id-%d", g.n)
}

type memProfileRepo struct{}

func (r *memProfileRepo) Create(_ context.Context, _ *domain.SellerProfile) error { return nil }
func (r *memProfileRepo) Get(_ context.Context, _ domain.TenantID) (*domain.SellerProfile, error) {
	return nil, fmt.Errorf("not found")
}
func (r *memProfileRepo) Update(_ context.Context, _ *domain.SellerProfile) error { return nil }
func (r *memProfileRepo) Delete(_ context.Context, _ domain.TenantID) error      { return nil }

type memFPRepo struct{}

func (r *memFPRepo) Create(_ context.Context, _ *domain.EligibilityFingerprint) error { return nil }
func (r *memFPRepo) Get(_ context.Context, _ domain.TenantID) (*domain.EligibilityFingerprint, error) {
	return nil, fmt.Errorf("not found")
}
func (r *memFPRepo) SaveProbeResults(_ context.Context, _ string, _ domain.TenantID, _ []domain.BrandProbeResult) error {
	return nil
}
func (r *memFPRepo) SaveCategoryEligibilities(_ context.Context, _ string, _ domain.TenantID, _ []domain.CategoryEligibility) error {
	return nil
}
func (r *memFPRepo) Delete(_ context.Context, _ domain.TenantID) error { return nil }

// --- helpers ---

func withAuth(r *http.Request, tenantID string) *http.Request {
	ac := &port.AuthContext{
		UserID:   "test-user",
		TenantID: domain.TenantID(tenantID),
	}
	ctx := context.WithValue(r.Context(), middleware.AuthContextKey, ac)
	return r.WithContext(ctx)
}

func newTestChatHandler() (*ChatHandler, *memChatRepo) {
	repo := &memChatRepo{}
	hub := service.NewChatHub()
	runtime := &mockConvRuntime{response: "Hello! I'm your concierge."}
	idGen := &seqIDGen{}
	chatSvc := service.NewChatService(repo, runtime, hub, idGen, &memProfileRepo{}, &memFPRepo{})
	h := NewChatHandler(chatSvc, hub)
	return h, repo
}

// --- tests ---

func TestChatHandler_Send_ReturnsAccepted(t *testing.T) {
	h, _ := newTestChatHandler()

	body := bytes.NewBufferString(`{"message":"hello"}`)
	req := httptest.NewRequest("POST", "/chat/send", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, "tenant-1")

	rr := httptest.NewRecorder()
	h.Send(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["status"] != "accepted" {
		t.Errorf("expected status=accepted, got %s", resp["status"])
	}
}

func TestChatHandler_Send_EmptyMessage(t *testing.T) {
	h, _ := newTestChatHandler()

	body := bytes.NewBufferString(`{"message":""}`)
	req := httptest.NewRequest("POST", "/chat/send", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, "tenant-1")

	rr := httptest.NewRecorder()
	h.Send(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty message, got %d", rr.Code)
	}
}

func TestChatHandler_Send_InvalidJSON(t *testing.T) {
	h, _ := newTestChatHandler()

	body := bytes.NewBufferString(`not json`)
	req := httptest.NewRequest("POST", "/chat/send", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, "tenant-1")

	rr := httptest.NewRecorder()
	h.Send(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rr.Code)
	}
}

func TestChatHandler_Send_MessageActuallyProcessed(t *testing.T) {
	h, repo := newTestChatHandler()

	body := bytes.NewBufferString(`{"message":"what can I sell?"}`)
	req := httptest.NewRequest("POST", "/chat/send", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, "tenant-1")

	rr := httptest.NewRecorder()
	h.Send(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}

	// Wait for async goroutine to complete
	time.Sleep(500 * time.Millisecond)

	// Verify messages were persisted (user + assistant)
	if len(repo.messages) < 2 {
		t.Fatalf("expected at least 2 messages (user+assistant), got %d", len(repo.messages))
	}

	if repo.messages[0].Role != domain.ChatRoleUser {
		t.Errorf("first message should be user, got %s", repo.messages[0].Role)
	}
	if repo.messages[0].Content != "what can I sell?" {
		t.Errorf("user message content wrong: %s", repo.messages[0].Content)
	}
	if repo.messages[1].Role != domain.ChatRoleAssistant {
		t.Errorf("second message should be assistant, got %s", repo.messages[1].Role)
	}
}

func TestChatHandler_History_EmptyReturnsEmptyArray(t *testing.T) {
	h, _ := newTestChatHandler()

	req := httptest.NewRequest("GET", "/chat/history", nil)
	req = withAuth(req, "tenant-1")

	rr := httptest.NewRecorder()
	h.History(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	// Should be [] not null
	body := rr.Body.String()
	if !strings.Contains(body, `"messages":[]`) && !strings.Contains(body, `"messages": []`) {
		t.Errorf("expected empty messages array, got: %s", body)
	}
}

func TestChatHandler_Events_SSEHeaders(t *testing.T) {
	h, _ := newTestChatHandler()

	req := httptest.NewRequest("GET", "/chat/events", nil)
	req = withAuth(req, "tenant-1")

	// Cancel context after 100ms to avoid blocking forever
	ctx, cancel := context.WithTimeout(req.Context(), 100*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	h.Events(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}
	cc := rr.Header().Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", cc)
	}
}
