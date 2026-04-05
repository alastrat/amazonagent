package service

import (
	"context"
	"fmt"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type PipelineService struct {
	orchestrator *PipelineOrchestrator
	campaigns    port.CampaignRepo
	scoring      port.ScoringConfigRepo
	deals        *DealService
}

func NewPipelineService(orchestrator *PipelineOrchestrator, campaigns port.CampaignRepo, scoring port.ScoringConfigRepo, deals *DealService) *PipelineService {
	return &PipelineService{orchestrator: orchestrator, campaigns: campaigns, scoring: scoring, deals: deals}
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

	pipelineConfig := domain.DefaultPipelineConfig(tenantID)
	pipelineConfig.Scoring = sc.Weights

	// Override pipeline thresholds with campaign criteria if provided
	if campaign.Criteria.MinMarginPct != nil {
		pipelineConfig.Thresholds.MinMarginPct = *campaign.Criteria.MinMarginPct
	}

	// Apply brand filter from campaign criteria
	if len(campaign.Criteria.BlockedBrands) > 0 {
		pipelineConfig.Thresholds.BrandFilter.BlockList = campaign.Criteria.BlockedBrands
	}
	if len(campaign.Criteria.PreferredBrands) > 0 {
		pipelineConfig.Thresholds.BrandFilter.AllowList = campaign.Criteria.PreferredBrands
	}

	result, err := s.orchestrator.RunPipeline(ctx, campaignID, campaign.Criteria, pipelineConfig)
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
