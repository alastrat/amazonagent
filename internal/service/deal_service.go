package service

import (
	"context"
	"fmt"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type DealService struct {
	repo   port.DealRepo
	events *EventService
	idGen  port.IDGenerator
}

func NewDealService(repo port.DealRepo, events *EventService, idGen port.IDGenerator) *DealService {
	return &DealService{repo: repo, events: events, idGen: idGen}
}

func (s *DealService) CreateFromResearch(ctx context.Context, tenantID domain.TenantID, result *domain.ResearchResult) ([]domain.Deal, error) {
	var deals []domain.Deal
	now := time.Now()

	for _, c := range result.Candidates {
		deal := domain.Deal{
			ID:              domain.DealID(s.idGen.New()),
			TenantID:        tenantID,
			CampaignID:      result.CampaignID,
			ASIN:            c.ASIN,
			Title:           c.Title,
			Brand:           c.Brand,
			Category:        c.Category,
			Status:          domain.DealStatusNeedsReview,
			Scores:          c.Scores,
			Evidence:        c.Evidence,
			ReviewerVerdict: c.ReviewerVerdict,
			IterationCount:  c.IterationCount,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		deals = append(deals, deal)
	}

	if err := s.repo.CreateBatch(ctx, deals); err != nil {
		return nil, fmt.Errorf("create deals batch: %w", err)
	}

	for _, d := range deals {
		_ = s.events.Emit(ctx, tenantID, "deal_created", "deal", string(d.ID), "system", map[string]any{
			"campaign_id": result.CampaignID,
			"asin":        d.ASIN,
			"overall":     d.Scores.Overall,
		})
	}

	return deals, nil
}

func (s *DealService) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.DealID) (*domain.Deal, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

func (s *DealService) List(ctx context.Context, tenantID domain.TenantID, filter port.DealFilter) ([]domain.Deal, int, error) {
	return s.repo.List(ctx, tenantID, filter)
}

func (s *DealService) Approve(ctx context.Context, tenantID domain.TenantID, id domain.DealID, userID string) (*domain.Deal, error) {
	deal, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	if err := deal.Transition(domain.DealStatusApproved); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, deal); err != nil {
		return nil, fmt.Errorf("update deal: %w", err)
	}

	_ = s.events.Emit(ctx, tenantID, "deal_approved", "deal", string(id), userID, map[string]any{
		"asin":    deal.ASIN,
		"overall": deal.Scores.Overall,
	})

	return deal, nil
}

func (s *DealService) Reject(ctx context.Context, tenantID domain.TenantID, id domain.DealID, userID string, reason string) (*domain.Deal, error) {
	deal, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	if err := deal.Transition(domain.DealStatusRejected); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, deal); err != nil {
		return nil, fmt.Errorf("update deal: %w", err)
	}

	_ = s.events.Emit(ctx, tenantID, "deal_rejected", "deal", string(id), userID, map[string]any{
		"asin":   deal.ASIN,
		"reason": reason,
	})

	return deal, nil
}
