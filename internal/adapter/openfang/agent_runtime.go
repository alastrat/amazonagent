package openfang

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type AgentRuntime struct {
	apiURL string
	apiKey string
}

func NewAgentRuntime(apiURL, apiKey string) *AgentRuntime {
	return &AgentRuntime{apiURL: apiURL, apiKey: apiKey}
}

func (r *AgentRuntime) RunResearchPipeline(ctx context.Context, input port.PipelineInput) (*domain.ResearchResult, error) {
	slog.Info("running research pipeline via OpenFang",
		"campaign_id", input.CampaignID,
		"tenant_id", input.TenantID,
		"keywords", input.Criteria.Keywords,
	)

	return nil, fmt.Errorf("OpenFang research pipeline not yet implemented — configure OPENFANG_API_URL and OPENFANG_API_KEY")
}
