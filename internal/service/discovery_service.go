package service

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type DiscoveryService struct {
	repo port.DiscoveryConfigRepo
}

func NewDiscoveryService(repo port.DiscoveryConfigRepo) *DiscoveryService {
	return &DiscoveryService{repo: repo}
}

func (s *DiscoveryService) Get(ctx context.Context, tenantID domain.TenantID) (*domain.DiscoveryConfig, error) {
	return s.repo.Get(ctx, tenantID)
}

func (s *DiscoveryService) Update(ctx context.Context, dc *domain.DiscoveryConfig) error {
	return s.repo.Upsert(ctx, dc)
}
