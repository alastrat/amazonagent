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

	// Inject assessment results with real product data
	fp, err := s.fingerprints.Get(ctx, tenantID)
	if err == nil && fp != nil {
		prompt += fmt.Sprintf("\n## Assessment Summary\n")
		prompt += fmt.Sprintf("- Products scanned: %d\n", fp.TotalProbes)
		prompt += fmt.Sprintf("- Overall open rate: %.0f%%\n", fp.OverallOpenRate)
		prompt += fmt.Sprintf("- Total eligible: %d\n", fp.TotalEligible)
		prompt += fmt.Sprintf("- Total restricted: %d\n", fp.TotalRestricted)

		// Category breakdown
		if len(fp.Categories) > 0 {
			prompt += "\n## Categories Scanned\n"
			for _, cat := range fp.Categories {
				prompt += fmt.Sprintf("- %s: %d probed, %d open, %.0f%% open rate\n",
					cat.Category, cat.ProbeCount, cat.OpenCount, cat.OpenRate)
			}
		}

		// Product details — eligible and ungatable
		if len(fp.BrandResults) > 0 {
			eligible := []domain.BrandProbeResult{}
			ungatable := []domain.BrandProbeResult{}
			restricted := []domain.BrandProbeResult{}

			for _, br := range fp.BrandResults {
				switch br.EligibilityStatus {
				case "eligible":
					eligible = append(eligible, br)
				case "ungatable":
					ungatable = append(ungatable, br)
				default:
					restricted = append(restricted, br)
				}
			}

			if len(eligible) > 0 {
				prompt += fmt.Sprintf("\n## Eligible Products (%d) — Can list immediately\n", len(eligible))
				for _, p := range eligible {
					prompt += fmt.Sprintf("- ASIN: %s | %s | Brand: %s | Category: %s > %s | Price: $%.2f | Est. Margin: %.1f%% | Sellers: %d\n",
						p.ASIN, truncate(p.Title, 50), p.Brand, p.Category, p.Subcategory, p.Price, p.EstMarginPct, p.SellerCount)
				}
			}

			if len(ungatable) > 0 {
				prompt += fmt.Sprintf("\n## Ungatable Products (%d) — Can apply for approval\n", len(ungatable))
				for _, p := range ungatable {
					approvalNote := ""
					if p.ApprovalURL != "" {
						approvalNote = fmt.Sprintf(" | Approval URL: %s", p.ApprovalURL)
					}
					prompt += fmt.Sprintf("- ASIN: %s | %s | Brand: %s | Category: %s > %s | Price: $%.2f | Est. Margin: %.1f%%%s\n",
						p.ASIN, truncate(p.Title, 50), p.Brand, p.Category, p.Subcategory, p.Price, p.EstMarginPct, approvalNote)
				}
			}

			prompt += fmt.Sprintf("\n## Restricted Products: %d (truly blocked, no approval path)\n", len(restricted))
		}
	}

	return prompt
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
