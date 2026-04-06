package service

import (
	"context"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type TenantSettingsRepo interface {
	Get(ctx context.Context, tenantID domain.TenantID) (*domain.TenantSettings, error)
	Upsert(ctx context.Context, s *domain.TenantSettings) error
}

type TenantSettingsService struct {
	repo TenantSettingsRepo
}

func NewTenantSettingsService(repo TenantSettingsRepo) *TenantSettingsService {
	return &TenantSettingsService{repo: repo}
}

func (s *TenantSettingsService) Get(ctx context.Context, tenantID domain.TenantID) (*domain.TenantSettings, error) {
	settings, err := s.repo.Get(ctx, tenantID)
	if err != nil {
		defaults := domain.DefaultTenantSettings(tenantID)
		return &defaults, nil
	}
	return settings, nil
}

func (s *TenantSettingsService) Update(ctx context.Context, settings *domain.TenantSettings) error {
	if err := s.repo.Upsert(ctx, settings); err != nil {
		return err
	}
	slog.Info("tenant settings updated", "tenant_id", settings.TenantID,
		"memory", settings.AgentMemoryEnabled, "telegram", settings.TelegramEnabled)
	return nil
}
