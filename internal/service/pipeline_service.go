package service

import (
	"context"
	"fmt"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type PipelineService struct {
	agentRuntime port.AgentRuntime
	campaigns    port.CampaignRepo
	scoring      port.ScoringConfigRepo
	deals        *DealService
}

func NewPipelineService(agentRuntime port.AgentRuntime, campaigns port.CampaignRepo, scoring port.ScoringConfigRepo, deals *DealService) *PipelineService {
	return &PipelineService{agentRuntime: agentRuntime, campaigns: campaigns, scoring: scoring, deals: deals}
}

func (s *PipelineService) RunCampaign(ctx context.Context, campaignID domain.CampaignID, tenantID domain.TenantID) error {
	campaign, err := s.campaigns.GetByID(ctx, tenantID, campaignID)
	if err != nil {
		return fmt.Errorf("get campaign: %w", err)
	}

	if err := campaign.Transition(domain.CampaignStatusRunning); err != nil {
		return err
	}
	if err := s.campaigns.Update(ctx, campaign); err != nil {
		return fmt.Errorf("update campaign to running: %w", err)
	}

	sc, err := s.scoring.GetByID(ctx, tenantID, campaign.ScoringConfigID)
	if err != nil {
		return fmt.Errorf("get scoring config: %w", err)
	}

	input := port.PipelineInput{
		CampaignID:    campaignID,
		TenantID:      tenantID,
		Criteria:      campaign.Criteria,
		ScoringConfig: *sc,
	}

	result, err := s.agentRuntime.RunResearchPipeline(ctx, input)
	if err != nil {
		_ = campaign.Transition(domain.CampaignStatusFailed)
		_ = s.campaigns.Update(ctx, campaign)
		return fmt.Errorf("run research pipeline: %w", err)
	}

	_, err = s.deals.CreateFromResearch(ctx, tenantID, result)
	if err != nil {
		_ = campaign.Transition(domain.CampaignStatusFailed)
		_ = s.campaigns.Update(ctx, campaign)
		return fmt.Errorf("create deals from research: %w", err)
	}

	if err := campaign.Transition(domain.CampaignStatusCompleted); err != nil {
		return err
	}
	return s.campaigns.Update(ctx, campaign)
}
