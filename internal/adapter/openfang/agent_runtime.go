package openfang

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type AgentRuntime struct {
	apiURL string
	apiKey string
}

func NewAgentRuntime(apiURL, apiKey string) *AgentRuntime {
	return &AgentRuntime{apiURL: apiURL, apiKey: apiKey}
}

func (r *AgentRuntime) RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	slog.Info("running agent via OpenFang",
		"agent", task.AgentName,
		"url", r.apiURL,
	)
	return nil, fmt.Errorf("OpenFang agent execution not yet implemented for agent %q", task.AgentName)
}
