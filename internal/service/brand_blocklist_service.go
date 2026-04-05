package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type BrandBlocklistService struct {
	repo  port.BrandBlocklistRepo
	idGen port.IDGenerator
}

func NewBrandBlocklistService(repo port.BrandBlocklistRepo, idGen port.IDGenerator) *BrandBlocklistService {
	return &BrandBlocklistService{repo: repo, idGen: idGen}
}

func (s *BrandBlocklistService) List(ctx context.Context, tenantID domain.TenantID) ([]domain.BlockedBrand, error) {
	return s.repo.List(ctx, tenantID)
}

func (s *BrandBlocklistService) Add(ctx context.Context, tenantID domain.TenantID, brand, reason string, source domain.BlockedBrandSource, asin string) error {
	b := &domain.BlockedBrand{
		ID:        domain.BlockedBrandID(s.idGen.New()),
		TenantID:  tenantID,
		Brand:     brand,
		Reason:    reason,
		Source:    source,
		ASIN:      asin,
		CreatedAt: time.Now(),
	}
	return s.repo.Add(ctx, b)
}

func (s *BrandBlocklistService) Remove(ctx context.Context, tenantID domain.TenantID, brand string) error {
	return s.repo.Remove(ctx, tenantID, brand)
}

// LoadBrandFilter loads the tenant's blocklist from DB into a BrandFilter.
func (s *BrandBlocklistService) LoadBrandFilter(ctx context.Context, tenantID domain.TenantID) (domain.BrandFilter, error) {
	brands, err := s.repo.List(ctx, tenantID)
	if err != nil {
		return domain.BrandFilter{}, err
	}
	var blocklist []string
	for _, b := range brands {
		blocklist = append(blocklist, b.Brand)
	}
	return domain.BrandFilter{BlockList: blocklist}, nil
}

// AutoBlock adds a brand to the blocklist when the pipeline discovers it can't be sold.
func (s *BrandBlocklistService) AutoBlock(ctx context.Context, tenantID domain.TenantID, brand, asin, reason string) {
	exists, err := s.repo.Exists(ctx, tenantID, brand)
	if err != nil || exists {
		return
	}
	if err := s.Add(ctx, tenantID, brand, reason, domain.BlockedBrandSourcePipeline, asin); err != nil {
		slog.Warn("auto-block brand failed", "brand", brand, "error", err)
	} else {
		slog.Info("auto-blocked brand", "brand", brand, "reason", reason, "asin", asin)
	}
}
