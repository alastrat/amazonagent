package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// RunAgentLoop executes an agent with iterative refinement (ReAct pattern).
// For single-shot agents (MaxTurns <= 1), this is equivalent to a single RunAgent call.
// For multi-turn agents, the loop runs up to MaxTurns iterations, checking the stop
// condition after each turn and injecting the previous output as refinement context.
func RunAgentLoop(
	ctx context.Context,
	runtime port.AgentRuntime,
	def *domain.AgentDefinition,
	task domain.AgentTask,
) (*domain.AgentOutput, error) {
	maxTurns := def.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 1
	}

	// Execute pre-run hooks
	if def.Hooks != nil {
		tenantID := domain.TenantID(task.Input["_tenant_id"].(string))
		for _, hook := range def.Hooks.PreRun {
			if err := hook(ctx, def.Name, tenantID); err != nil {
				return nil, fmt.Errorf("pre-run hook failed for %s: %w", def.Name, err)
			}
		}
	}

	var lastOutput *domain.AgentOutput
	var lastErr error
	startTime := time.Now()

	for turn := 0; turn < maxTurns; turn++ {
		currentTask := task

		// On subsequent turns, inject refinement context from previous output
		if turn > 0 && lastOutput != nil && def.CanSelfRefine {
			if currentTask.Input == nil {
				currentTask.Input = make(map[string]any)
			}
			currentTask.Input["_turn"] = turn
			currentTask.Input["_refinement_context"] = SelfCritiqueRefinement(turn, lastOutput)
		}

		output, err := runtime.RunAgent(ctx, currentTask)
		if err != nil {
			lastErr = err
			slog.Warn("agent-loop: turn failed",
				"agent", def.Name, "turn", turn, "error", err)

			// Execute error hooks
			if def.Hooks != nil {
				for _, hook := range def.Hooks.OnError {
					_ = hook(ctx, def.Name, err)
				}
			}

			// If this is the last allowed retry, return the error
			if turn >= maxTurns-1 {
				return lastOutput, lastErr
			}
			continue
		}

		lastOutput = output
		lastErr = nil

		slog.Debug("agent-loop: turn complete",
			"agent", def.Name, "turn", turn, "tokens", output.TokensUsed)

		// Check stop condition
		if def.StopCondition != nil && def.StopCondition(output) {
			break
		}

		// If agent can't self-refine, single-shot only
		if !def.CanSelfRefine {
			break
		}

		// Check for context overflow (Ralph Loop trigger)
		if def.Hooks != nil && len(def.Hooks.OnContextOverflow) > 0 && output.TokensUsed > 0 {
			// Heuristic: if tokens used exceeds 80% of a typical context window
			// trigger the Ralph Loop to summarize and continue
			const contextThreshold = 80000 // ~80% of 100K context
			if output.TokensUsed > contextThreshold {
				for _, hook := range def.Hooks.OnContextOverflow {
					summary, err := hook(ctx, def.Name, output.TokensUsed)
					if err != nil {
						slog.Warn("agent-loop: context overflow hook failed", "error", err)
						break
					}
					// Inject summary as new context for next turn
					currentTask.Input["_ralph_loop_summary"] = summary
					slog.Info("agent-loop: Ralph Loop activated",
						"agent", def.Name, "turn", turn, "tokens", output.TokensUsed)
				}
			}
		}
	}

	durationMs := time.Since(startTime).Milliseconds()

	// Execute post-run hooks
	if def.Hooks != nil && lastOutput != nil {
		for _, hook := range def.Hooks.PostRun {
			if err := hook(ctx, def.Name, lastOutput, durationMs); err != nil {
				slog.Warn("agent-loop: post-run hook failed", "agent", def.Name, "error", err)
			}
		}
	}

	return lastOutput, lastErr
}

// SelfCritiqueRefinement asks the agent to critique and improve its previous output.
func SelfCritiqueRefinement(turn int, prev *domain.AgentOutput) string {
	return fmt.Sprintf(
		"This is turn %d. Your previous output was:\n%s\n\n"+
			"Review your reasoning for gaps, incorrect assumptions, or missing data. "+
			"If you find issues, provide a corrected output. "+
			"If your previous answer was sound, return it unchanged with higher confidence.",
		turn, prev.Raw,
	)
}
