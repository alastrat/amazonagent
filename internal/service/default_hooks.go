package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// DefaultHooks returns the standard hook set applied to all agents.
func DefaultHooks() *domain.AgentHooks {
	return &domain.AgentHooks{
		PostRun: []domain.PostRunHookFunc{
			ValidateOutputHook,
			MetricsHook,
		},
		OnError: []domain.ErrorHookFunc{
			ErrorLogHook,
		},
	}
}

// ConciergeHooks returns hooks for the concierge agent, including the Ralph Loop.
func ConciergeHooks() *domain.AgentHooks {
	hooks := DefaultHooks()
	hooks.OnContextOverflow = []domain.ContextOverflowFunc{
		RalphLoopSummarizer,
	}
	return hooks
}

// ValidateOutputHook checks that agent output has the expected structure.
func ValidateOutputHook(_ context.Context, agentName string, output *domain.AgentOutput, _ int64) error {
	if output == nil {
		slog.Warn("hook: nil output", "agent", agentName)
		return nil
	}
	if output.Structured == nil && output.Raw == "" {
		slog.Warn("hook: empty output", "agent", agentName)
	}
	return nil
}

// MetricsHook logs agent execution metrics.
func MetricsHook(_ context.Context, agentName string, output *domain.AgentOutput, durationMs int64) error {
	tokens := 0
	if output != nil {
		tokens = output.TokensUsed
	}
	slog.Info("hook: agent metrics",
		"agent", agentName,
		"tokens", tokens,
		"duration_ms", durationMs,
	)
	return nil
}

// ErrorLogHook logs agent execution errors.
func ErrorLogHook(_ context.Context, agentName string, err error) error {
	slog.Error("hook: agent error", "agent", agentName, "error", err)
	return nil
}

// RalphLoopSummarizer creates a context summary when tokens approach the limit.
// This implements the Ralph Loop pattern: summarize progress → reset context → continue.
func RalphLoopSummarizer(_ context.Context, agentName string, tokensUsed int) (string, error) {
	slog.Info("hook: Ralph Loop triggered",
		"agent", agentName,
		"tokens_used", tokensUsed,
	)
	// The summary template — the caller injects this into the next turn's context.
	// In practice, this would call an LLM to summarize the conversation,
	// but for now we return a structured template that the agent loop fills.
	return fmt.Sprintf(
		"[CONTEXT CONTINUATION — Ralph Loop]\n"+
			"Agent: %s\n"+
			"Previous context was %d tokens and approaching the limit.\n"+
			"The conversation has been summarized. Continue from where you left off.\n"+
			"Focus on completing the remaining objectives.",
		agentName, tokensUsed,
	), nil
}

// InitRegistryHooks applies default hooks to all registered agents.
func InitRegistryHooks() {
	defaults := DefaultHooks()
	conciergeHooks := ConciergeHooks()

	for name, def := range domain.AgentRegistry {
		if name == "concierge" {
			def.Hooks = conciergeHooks
		} else {
			def.Hooks = defaults
		}
		// Set default stop condition for single-shot agents
		if def.MaxTurns <= 1 && def.StopCondition == nil {
			def.StopCondition = SingleShot
		}
		domain.AgentRegistry[name] = def
	}
}
