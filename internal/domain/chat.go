package domain

import "time"

// ChatMessageRole defines who sent the message.
type ChatMessageRole string

const (
	ChatRoleUser      ChatMessageRole = "user"
	ChatRoleAssistant ChatMessageRole = "assistant"
	ChatRoleSystem    ChatMessageRole = "system"
)

// ChatSessionStatus tracks the lifecycle of a chat session.
type ChatSessionStatus string

const (
	ChatSessionActive ChatSessionStatus = "active"
	ChatSessionEnded  ChatSessionStatus = "ended"
)

// ChatSession represents a persistent conversation between a tenant and the concierge.
type ChatSession struct {
	ID             string            `json:"id"`
	TenantID       TenantID          `json:"tenant_id"`
	AgentSessionID string            `json:"agent_session_id"` // runtime-specific session ID (e.g. OpenFang agent ID)
	Status         ChatSessionStatus `json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
	LastMessageAt  time.Time         `json:"last_message_at"`
}

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	ID        string            `json:"id"`
	TenantID  TenantID          `json:"tenant_id"`
	SessionID string            `json:"session_id"`
	Role      ChatMessageRole   `json:"role"`
	Content   string            `json:"content"`
	Metadata  map[string]any    `json:"metadata,omitempty"` // tool calls, suggestions, etc.
	CreatedAt time.Time         `json:"created_at"`
}
