package domain

import "context"

// AgentHooks defines lifecycle callbacks for agent execution.
type AgentHooks struct {
	PreRun            []HookFunc
	PostRun           []PostRunHookFunc
	OnError           []ErrorHookFunc
	OnContextOverflow []ContextOverflowFunc // Ralph Loop trigger
}

// HookFunc runs before agent execution. Return error to abort.
type HookFunc func(ctx context.Context, agentName string, tenantID TenantID) error

// PostRunHookFunc runs after successful agent execution.
type PostRunHookFunc func(ctx context.Context, agentName string, output *AgentOutput, durationMs int64) error

// ErrorHookFunc runs when agent execution fails.
type ErrorHookFunc func(ctx context.Context, agentName string, err error) error

// ContextOverflowFunc runs when the agent's context is approaching limits.
// Returns a summarized context string for the Ralph Loop continuation.
type ContextOverflowFunc func(ctx context.Context, agentName string, tokensUsed int) (string, error)
