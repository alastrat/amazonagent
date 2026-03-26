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
