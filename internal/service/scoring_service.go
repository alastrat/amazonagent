package service

import (
	"context"
	"fmt"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type ScoringService struct {
	repo  port.ScoringConfigRepo
	idGen port.IDGenerator
}

func NewScoringService(repo port.ScoringConfigRepo, idGen port.IDGenerator) *ScoringService {
	return &ScoringService{repo: repo, idGen: idGen}
}

func (s *ScoringService) GetActive(ctx context.Context, tenantID domain.TenantID) (*domain.ScoringConfig, error) {
	return s.repo.GetActive(ctx, tenantID)
}

func (s *ScoringService) Update(ctx context.Context, tenantID domain.TenantID, weights domain.ScoringWeights, thresholds domain.Thresholds) (*domain.ScoringConfig, error) {
	current, err := s.repo.GetActive(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get current config: %w", err)
	}

	newConfig := &domain.ScoringConfig{
		ID:         domain.ScoringConfigID(s.idGen.New()),
		TenantID:   tenantID,
		Version:    current.Version + 1,
		Weights:    weights,
		Thresholds: thresholds,
		CreatedBy:  "user",
		Active:     true,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, newConfig); err != nil {
		return nil, fmt.Errorf("create config: %w", err)
	}

	if err := s.repo.SetActive(ctx, tenantID, newConfig.ID); err != nil {
		return nil, fmt.Errorf("set active: %w", err)
	}

	return newConfig, nil
}

func (s *ScoringService) EnsureDefault(ctx context.Context, tenantID domain.TenantID) error {
	_, err := s.repo.GetActive(ctx, tenantID)
	if err == nil {
		return nil
	}

	sc := &domain.ScoringConfig{
		ID:         domain.ScoringConfigID(s.idGen.New()),
		TenantID:   tenantID,
		Version:    1,
		Weights:    domain.DefaultScoringWeights(),
		Thresholds: domain.DefaultThresholds(),
		CreatedBy:  "system",
		Active:     true,
		CreatedAt:  time.Now(),
	}
	return s.repo.Create(ctx, sc)
}
