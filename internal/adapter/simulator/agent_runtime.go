package simulator

import (
	"context"
	"fmt"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type AgentRuntime struct{}

func NewAgentRuntime() *AgentRuntime {
	return &AgentRuntime{}
}

func (r *AgentRuntime) RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	return nil, fmt.Errorf("simulator agent %q not yet implemented", task.AgentName)
}
