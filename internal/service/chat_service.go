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
func (s *ChatService) GetOrCreateSession(ctx context.Context, tenantID domain.TenantID) (*domain.ChatSession, error) {
	session, err := s.repo.GetSession(ctx, tenantID)
	if err == nil && session != nil && session.Status == domain.ChatSessionActive {
		return session, nil
	}

	// Build concierge system prompt with tenant context
	systemPrompt := s.buildSystemPrompt(ctx, tenantID)

	// Start agent session
	agentSessionID, err := s.runtime.StartSession(ctx, tenantID, port.SessionConfig{
		AgentName:    "concierge",
		SystemPrompt: systemPrompt,
		Definition:   domain.GetAgentDefinition("concierge"),
	})
	if err != nil {
		return nil, fmt.Errorf("start agent session: %w", err)
	}

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
	prompt := `You are an FBA wholesale concierge. You help Amazon sellers find profitable products they can list and sell.

You have access to the following tools:
- search_products(keywords, category): Search Amazon for products
- check_eligibility(asin): Check if the seller can list this product
- get_pricing(asin): Get current pricing and seller count
- get_strategy(): Get the seller's current strategy and goals
- query_catalog(filters): Search the shared product catalog
- create_suggestion(asin, reason): Propose a product for the seller to consider

Guidelines:
- Be concise and actionable
- When suggesting products, include: ASIN, price, estimated margin, seller count, eligibility status
- For ungatable products, mention they can request approval via Seller Central
- Never auto-execute critical actions (listings, pricing) — always ask for confirmation
- If asked about something outside FBA wholesale, politely redirect
`

	// Inject tenant context
	profile, err := s.profiles.Get(ctx, tenantID)
	if err == nil && profile != nil {
		prompt += fmt.Sprintf("\nSeller archetype: %s\n", profile.Archetype)
	}

	fp, err := s.fingerprints.Get(ctx, tenantID)
	if err == nil && fp != nil {
		prompt += fmt.Sprintf("Eligible categories: %d scanned, %.0f%% open rate\n",
			fp.TotalProbes, fp.OverallOpenRate)
		prompt += fmt.Sprintf("Total eligible: %d, Total restricted: %d\n",
			fp.TotalEligible, fp.TotalRestricted)
	}

	return prompt
}
