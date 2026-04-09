package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// StrategyService manages versioned growth strategies per tenant.
// Every change creates a new version. Users can rollback to any previous version.
type StrategyService struct {
	versions port.StrategyVersionRepo
	idGen    port.IDGenerator
}

func NewStrategyService(versions port.StrategyVersionRepo, idGen port.IDGenerator) *StrategyService {
	return &StrategyService{versions: versions, idGen: idGen}
}

// GenerateInitialStrategy creates the first strategy version based on assessment results.
// Returns a draft — user must approve to activate.
func (s *StrategyService) GenerateInitialStrategy(
	ctx context.Context,
	tenantID domain.TenantID,
	fingerprint *domain.EligibilityFingerprint,
	archetype domain.SellerArchetype,
) (*domain.StrategyVersion, error) {
	versionNum, err := s.versions.NextVersionNumber(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Build goals based on archetype
	goals := s.generateGoals(archetype)

	// Build search params from fingerprint
	params := s.buildSearchParams(fingerprint)

	sv := &domain.StrategyVersion{
		ID:            domain.StrategyVersionID(s.idGen.New()),
		TenantID:      tenantID,
		VersionNumber: versionNum,
		Goals:         goals,
		SearchParams:  params,
		Status:        domain.StrategyStatusDraft,
		ChangeReason:  "Initial strategy from account assessment",
		CreatedBy:     domain.StrategyCreatedBySystem,
		CreatedAt:     time.Now(),
	}

	if err := s.versions.Create(ctx, sv); err != nil {
		return nil, err
	}

	slog.Info("strategy: generated initial version",
		"tenant_id", tenantID,
		"version", versionNum,
		"archetype", archetype,
		"goals", len(goals),
		"eligible_categories", len(params.EligibleCategories))

	return sv, nil
}

// ActivateVersion approves and activates a strategy version. Archives the previous active version.
func (s *StrategyService) ActivateVersion(ctx context.Context, tenantID domain.TenantID, versionID domain.StrategyVersionID) error {
	sv, err := s.versions.GetByID(ctx, tenantID, versionID)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}
	if sv.Status != domain.StrategyStatusDraft {
		return fmt.Errorf("can only activate draft versions, current status: %s", sv.Status)
	}

	if err := s.versions.Activate(ctx, tenantID, versionID); err != nil {
		return err
	}

	slog.Info("strategy: activated version",
		"tenant_id", tenantID,
		"version", sv.VersionNumber,
		"change_reason", sv.ChangeReason)

	return nil
}

// RollbackToVersion creates a NEW version with the params of a target version.
// Marks the current active as rolled_back. The new version is a draft requiring activation.
func (s *StrategyService) RollbackToVersion(ctx context.Context, tenantID domain.TenantID, targetVersionID domain.StrategyVersionID) (*domain.StrategyVersion, error) {
	// Get the target version to copy from
	target, err := s.versions.GetByID(ctx, tenantID, targetVersionID)
	if err != nil {
		return nil, fmt.Errorf("target version not found: %w", err)
	}

	// Mark current active as rolled back
	current, err := s.versions.GetActive(ctx, tenantID)
	if err == nil && current != nil {
		s.versions.SetStatus(ctx, tenantID, current.ID, domain.StrategyStatusRolledBack)
	}

	// Create new version with target's params
	versionNum, err := s.versions.NextVersionNumber(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	newVersion := &domain.StrategyVersion{
		ID:              domain.StrategyVersionID(s.idGen.New()),
		TenantID:        tenantID,
		VersionNumber:   versionNum,
		Goals:           target.Goals,
		SearchParams:    target.SearchParams,
		ScoringConfigID: target.ScoringConfigID,
		Status:          domain.StrategyStatusActive,
		ParentVersionID: target.ID,
		ChangeReason:    fmt.Sprintf("Rollback to v%d", target.VersionNumber),
		CreatedBy:       domain.StrategyCreatedByUser,
		CreatedAt:       time.Now(),
	}
	now := time.Now()
	newVersion.ActivatedAt = &now

	if err := s.versions.Create(ctx, newVersion); err != nil {
		return nil, err
	}

	fromVersion := 0
	if current != nil {
		fromVersion = current.VersionNumber
	}
	slog.Info("strategy: rolled back",
		"tenant_id", tenantID,
		"from_version", fromVersion,
		"to_version_source", target.VersionNumber,
		"new_version", versionNum)

	return newVersion, nil
}

// GetActive returns the current active strategy for a tenant.
func (s *StrategyService) GetActive(ctx context.Context, tenantID domain.TenantID) (*domain.StrategyVersion, error) {
	return s.versions.GetActive(ctx, tenantID)
}

// ListVersions returns the version history for a tenant.
func (s *StrategyService) ListVersions(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.StrategyVersion, error) {
	return s.versions.List(ctx, tenantID, limit)
}

// GetVersion returns a specific strategy version.
func (s *StrategyService) GetVersion(ctx context.Context, tenantID domain.TenantID, versionID domain.StrategyVersionID) (*domain.StrategyVersion, error) {
	return s.versions.GetByID(ctx, tenantID, versionID)
}

// generateGoals creates default goals based on seller archetype.
func (s *StrategyService) generateGoals(archetype domain.SellerArchetype) []domain.StrategyGoal {
	now := time.Now()

	switch archetype {
	case domain.SellerArchetypeGreenhorn:
		// 90-day timeframe — longer because ungating is needed first
		return []domain.StrategyGoal{
			{
				ID:             s.idGen.New(),
				Type:           "revenue",
				TargetAmount:   2000,
				Currency:       "USD",
				TimeframeStart: now,
				TimeframeEnd:   now.AddDate(0, 3, 0),
			},
		}
	case domain.SellerArchetypeRAToWholesale:
		// 30-day first wholesale revenue goal
		return []domain.StrategyGoal{
			{
				ID:             s.idGen.New(),
				Type:           "revenue",
				TargetAmount:   5000,
				Currency:       "USD",
				TimeframeStart: now,
				TimeframeEnd:   now.AddDate(0, 1, 0),
			},
		}
	case domain.SellerArchetypeExpandingPro:
		// 14-day incremental — they're already selling
		return []domain.StrategyGoal{
			{
				ID:             s.idGen.New(),
				Type:           "profit",
				TargetAmount:   3000,
				Currency:       "USD",
				TimeframeStart: now,
				TimeframeEnd:   now.AddDate(0, 0, 14),
			},
		}
	case domain.SellerArchetypeCapitalRich:
		// 60-day — aggressive but needs account health first
		return []domain.StrategyGoal{
			{
				ID:             s.idGen.New(),
				Type:           "revenue",
				TargetAmount:   10000,
				Currency:       "USD",
				TimeframeStart: now,
				TimeframeEnd:   now.AddDate(0, 2, 0),
			},
		}
	default:
		return []domain.StrategyGoal{
			{
				ID:             s.idGen.New(),
				Type:           "revenue",
				TargetAmount:   2000,
				Currency:       "USD",
				TimeframeStart: now,
				TimeframeEnd:   now.AddDate(0, 3, 0),
			},
		}
	}
}

// buildSearchParams creates discovery parameters from the eligibility fingerprint.
func (s *StrategyService) buildSearchParams(fp *domain.EligibilityFingerprint) domain.StrategySearchParams {
	params := domain.StrategySearchParams{
		MinMarginPct:   15.0,
		MinSellerCount: 2,
		ScoringWeights: domain.DefaultScoringWeights(),
	}

	if fp == nil {
		return params
	}

	// Only include categories where the seller has > 30% open rate
	for _, cat := range fp.Categories {
		if cat.OpenRate >= 30 {
			params.EligibleCategories = append(params.EligibleCategories, cat.Category)
		}
	}

	// Collect eligible brands
	brandSet := make(map[string]bool)
	for _, br := range fp.BrandResults {
		if br.Eligible {
			brandSet[br.Brand] = true
		}
	}
	for brand := range brandSet {
		params.EligibleBrands = append(params.EligibleBrands, brand)
	}

	return params
}
