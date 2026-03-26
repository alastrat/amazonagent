package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type AgentRuntime interface {
	RunResearchPipeline(ctx context.Context, input PipelineInput) (*domain.ResearchResult, error)
}

type PipelineInput struct {
	CampaignID    domain.CampaignID    `json:"campaign_id"`
	TenantID      domain.TenantID      `json:"tenant_id"`
	Criteria      domain.Criteria      `json:"criteria"`
	ScoringConfig domain.ScoringConfig `json:"scoring_config"`
	SourceASINs   []string             `json:"source_asins,omitempty"`
}
