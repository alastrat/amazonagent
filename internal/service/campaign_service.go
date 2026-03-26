package service

import (
	"context"
	"fmt"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type CampaignService struct {
	repo    port.CampaignRepo
	scoring port.ScoringConfigRepo
	events  *EventService
	durable port.DurableRuntime
	idGen   port.IDGenerator
}

func NewCampaignService(repo port.CampaignRepo, scoring port.ScoringConfigRepo, events *EventService, durable port.DurableRuntime, idGen port.IDGenerator) *CampaignService {
	return &CampaignService{repo: repo, scoring: scoring, events: events, durable: durable, idGen: idGen}
}

type CreateCampaignInput struct {
	TenantID    domain.TenantID
	Type        domain.CampaignType
	TriggerType domain.TriggerType
	Criteria    domain.Criteria
	SourceFile  *string
	CreatedBy   string
}

func (s *CampaignService) Create(ctx context.Context, input CreateCampaignInput) (*domain.Campaign, error) {
	sc, err := s.scoring.GetActive(ctx, input.TenantID)
	if err != nil {
		return nil, fmt.Errorf("get active scoring config: %w", err)
	}

	campaign := &domain.Campaign{
		ID:              domain.CampaignID(s.idGen.New()),
		TenantID:        input.TenantID,
		Type:            input.Type,
		Criteria:        input.Criteria,
		ScoringConfigID: sc.ID,
		SourceFile:      input.SourceFile,
		Status:          domain.CampaignStatusPending,
		CreatedBy:       input.CreatedBy,
		TriggerType:     input.TriggerType,
		CreatedAt:       time.Now(),
	}

	if err := s.repo.Create(ctx, campaign); err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}

	_ = s.events.Emit(ctx, input.TenantID, "campaign_created", "campaign", string(campaign.ID), input.CreatedBy, map[string]any{
		"type":         input.Type,
		"trigger_type": input.TriggerType,
	})

	if s.durable != nil {
		if err := s.durable.TriggerCampaignProcessing(ctx, campaign.ID, input.TenantID); err != nil {
			return nil, fmt.Errorf("trigger campaign processing: %w", err)
		}
	}

	return campaign, nil
}

func (s *CampaignService) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.CampaignID) (*domain.Campaign, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

func (s *CampaignService) List(ctx context.Context, tenantID domain.TenantID, filter port.CampaignFilter) ([]domain.Campaign, error) {
	return s.repo.List(ctx, tenantID, filter)
}
