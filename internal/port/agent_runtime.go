package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// AgentRuntime executes a single agent task.
// Pipeline orchestration (sequence, gates, retries) is NOT the runtime's job.
type AgentRuntime interface {
	RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error)
}

// SessionConfig configures a persistent agent session.
type SessionConfig struct {
	AgentName    string            `json:"agent_name"`
	SystemPrompt string            `json:"system_prompt"`
	Definition   *domain.AgentDefinition `json:"-"`
}

// ConversationalRuntime extends AgentRuntime with session-based methods for chat.
// This is a separate interface so existing pipeline callers don't need to change.
// Implementations: OpenFang adapter (first), direct Claude API (future).
type ConversationalRuntime interface {
	AgentRuntime

	// StartSession creates a persistent agent session with memory enabled.
	StartSession(ctx context.Context, tenantID domain.TenantID, config SessionConfig) (sessionID string, err error)

	// SendMessage sends a user message within an existing session and returns the agent response.
	SendMessage(ctx context.Context, sessionID string, message string) (*domain.AgentOutput, error)

	// EndSession closes a session and releases resources.
	EndSession(ctx context.Context, sessionID string) error
}
