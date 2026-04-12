package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// ChatService manages concierge conversations.
type ChatService struct {
	repo    port.ChatRepo
	runtime port.ConversationalRuntime
	hub     *ChatHub
	idGen   port.IDGenerator

	// Context providers — injected to build the concierge system prompt
	profiles     port.SellerProfileRepo
	fingerprints port.EligibilityFingerprintRepo
}

func NewChatService(
	repo port.ChatRepo,
	runtime port.ConversationalRuntime,
	hub *ChatHub,
	idGen port.IDGenerator,
	profiles port.SellerProfileRepo,
	fingerprints port.EligibilityFingerprintRepo,
) *ChatService {
	return &ChatService{
		repo:         repo,
		runtime:      runtime,
		hub:          hub,
		idGen:        idGen,
		profiles:     profiles,
		fingerprints: fingerprints,
	}
}

// GetOrCreateSession returns the active session for a tenant, creating one if needed.
// If the runtime lost the session (e.g. API restart), it re-creates the runtime session
// and updates the DB record.
func (s *ChatService) GetOrCreateSession(ctx context.Context, tenantID domain.TenantID) (*domain.ChatSession, error) {
	// Build concierge system prompt with tenant context
	systemPrompt := s.buildSystemPrompt(ctx, tenantID)

	// Always ensure the runtime session exists (survives API restarts)
	agentSessionID, err := s.runtime.StartSession(ctx, tenantID, port.SessionConfig{
		AgentName:    "concierge",
		SystemPrompt: systemPrompt,
		Definition:   domain.GetAgentDefinition("concierge"),
	})
	if err != nil {
		return nil, fmt.Errorf("start agent session: %w", err)
	}

	// Check for existing DB session
	session, _ := s.repo.GetSession(ctx, tenantID)
	if session != nil && session.Status == domain.ChatSessionActive {
		// Update the runtime session ID in case it changed (API restart)
		if session.AgentSessionID != agentSessionID {
			session.AgentSessionID = agentSessionID
			_ = s.repo.UpdateSession(ctx, session)
		}
		return session, nil
	}

	// Create new DB session
	session = &domain.ChatSession{
		ID:             s.idGen.New(),
		TenantID:       tenantID,
		AgentSessionID: agentSessionID,
		Status:         domain.ChatSessionActive,
		CreatedAt:      time.Now(),
		LastMessageAt:  time.Now(),
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	slog.Info("chat: session created", "tenant_id", tenantID, "session_id", session.ID)
	return session, nil
}

// SendMessage sends a user message and gets the concierge response.
func (s *ChatService) SendMessage(ctx context.Context, tenantID domain.TenantID, content string) (*domain.ChatMessage, error) {
	session, err := s.GetOrCreateSession(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Persist user message
	userMsg := &domain.ChatMessage{
		ID:        s.idGen.New(),
		TenantID:  tenantID,
		SessionID: session.ID,
		Role:      domain.ChatRoleUser,
		Content:   content,
		CreatedAt: time.Now(),
	}
	if err := s.repo.SaveMessage(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	// Notify SSE: typing indicator
	s.hub.Publish(tenantID, ChatEvent{
		Type:      ChatEventTyping,
		Timestamp: time.Now(),
		Data:      map[string]any{"session_id": session.ID},
	})

	// Call agent runtime
	output, err := s.runtime.SendMessage(ctx, session.AgentSessionID, content)
	if err != nil {
		s.hub.Publish(tenantID, ChatEvent{
			Type:      ChatEventError,
			Timestamp: time.Now(),
			Data:      map[string]any{"error": err.Error()},
		})
		return nil, fmt.Errorf("agent response: %w", err)
	}

	// Extract response text
	responseText := output.Raw
	if responseText == "" {
		if msg, ok := output.Structured["message"].(string); ok {
			responseText = msg
		}
	}

	// Persist assistant message
	assistantMsg := &domain.ChatMessage{
		ID:        s.idGen.New(),
		TenantID:  tenantID,
		SessionID: session.ID,
		Role:      domain.ChatRoleAssistant,
		Content:   responseText,
		Metadata: map[string]any{
			"tokens_used": output.TokensUsed,
			"duration_ms": output.DurationMs,
		},
		CreatedAt: time.Now(),
	}
	if err := s.repo.SaveMessage(ctx, assistantMsg); err != nil {
		slog.Warn("chat: failed to save assistant message", "error", err)
	}

	// Update session last message time
	session.LastMessageAt = time.Now()
	_ = s.repo.UpdateSession(ctx, session)

	// Publish SSE: full message
	s.hub.Publish(tenantID, ChatEvent{
		Type:      ChatEventMessage,
		Timestamp: time.Now(),
		Data: map[string]any{
			"id":      assistantMsg.ID,
			"role":    "assistant",
			"content": responseText,
		},
	})
	s.hub.Publish(tenantID, ChatEvent{
		Type:      ChatEventDone,
		Timestamp: time.Now(),
		Data:      map[string]any{"session_id": session.ID},
	})

	return assistantMsg, nil
}

// GetHistory returns recent chat messages for a tenant.
func (s *ChatService) GetHistory(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListMessages(ctx, tenantID, limit)
}

// buildSystemPrompt creates the concierge system prompt with tenant-specific context.
func (s *ChatService) buildSystemPrompt(ctx context.Context, tenantID domain.TenantID) string {
	prompt := `You are an FBA wholesale concierge for an Amazon seller. You help them find profitable products they can list and sell on Amazon via wholesale/arbitrage.

IMPORTANT: You have tools that query REAL data from the seller's account. Use them — don't guess.
- ALWAYS call get_assessment_summary first to understand the seller's current situation
- Use get_eligible_products to show products they can list NOW
- Use get_ungatable_products to show products they can apply for approval
- Use search_products to find new products on Amazon
- Use check_eligibility to verify if a specific ASIN can be listed

Guidelines:
- Be concise and actionable — this is a seller who wants to make money, not read essays
- When discussing products, always reference ASIN, price, estimated margin, and eligibility status
- For "ungatable" products (status: can apply), tell them they can request approval via Seller Central
- Provide the Seller Central approval URL when available
- Prioritize high-margin, low-competition products
- Never auto-execute critical actions — always ask for confirmation
- Base your answers on the seller's ACTUAL DATA below, not general knowledge
`

	// Inject seller profile
	profile, err := s.profiles.Get(ctx, tenantID)
	if err == nil && profile != nil {
		prompt += fmt.Sprintf("\n## Seller Profile\nArchetype: %s\n", profile.Archetype)
	}

	// Don't inject product data into system prompt — tools provide it on demand.
	// This keeps the system prompt small for faster responses.
	prompt += "\nUse your tools to get real-time data. Don't rely on cached information.\n"

	return prompt
}
