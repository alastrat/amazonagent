package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

const MaxDailySuggestions = 20

// DiscoveryQueueService runs daily per tenant, directed by the active strategy.
// Products found are presented as suggestions (not deals) — user must accept.
type DiscoveryQueueService struct {
	strategy    *StrategyService
	funnel      *FunnelService
	suggestions port.SuggestionRepo
	spapi       port.ProductSearcher
	idGen       port.IDGenerator
}

func NewDiscoveryQueueService(
	strategy *StrategyService,
	funnel *FunnelService,
	suggestions port.SuggestionRepo,
	spapi port.ProductSearcher,
	idGen port.IDGenerator,
) *DiscoveryQueueService {
	return &DiscoveryQueueService{
		strategy:    strategy,
		funnel:      funnel,
		suggestions: suggestions,
		spapi:       spapi,
		idGen:       idGen,
	}
}

// RunDailyDiscovery executes the discovery queue for a tenant.
// Loads active strategy → searches eligible categories → runs funnel → creates suggestions.
func (s *DiscoveryQueueService) RunDailyDiscovery(ctx context.Context, tenantID domain.TenantID) ([]domain.DiscoverySuggestion, error) {
	// Check daily cap
	todayCount, err := s.suggestions.CountToday(ctx, tenantID)
	if err != nil {
		slog.Warn("discovery-queue: failed to count today's suggestions", "error", err)
	}
	if todayCount >= MaxDailySuggestions {
		slog.Info("discovery-queue: daily cap reached", "tenant_id", tenantID, "count", todayCount)
		return nil, nil
	}
	remaining := MaxDailySuggestions - todayCount

	// Load active strategy
	strategy, err := s.strategy.GetActive(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("no active strategy: %w", err)
	}

	slog.Info("discovery-queue: starting",
		"tenant_id", tenantID,
		"strategy_version", strategy.VersionNumber,
		"eligible_categories", len(strategy.SearchParams.EligibleCategories),
		"remaining_cap", remaining)

	// Search eligible categories from strategy
	var allInputs []FunnelInput
	for _, category := range strategy.SearchParams.EligibleCategories {
		if s.spapi == nil {
			break
		}
		products, err := s.spapi.SearchProducts(ctx, []string{category}, "US")
		if err != nil {
			slog.Warn("discovery-queue: search failed", "category", category, "error", err)
			continue
		}
		for _, p := range products {
			allInputs = append(allInputs, FunnelInput{
				ASIN:           p.ASIN,
				Title:          p.Title,
				Brand:          p.Brand,
				Category:       p.Category,
				EstimatedPrice: p.AmazonPrice,
				BSRRank:        p.BSRRank,
				SellerCount:    p.SellerCount,
				Source:         domain.ScanTypeCategory,
			})
		}
	}

	if len(allInputs) == 0 {
		slog.Info("discovery-queue: no products found", "tenant_id", tenantID)
		return nil, nil
	}

	// Run through funnel with strategy's params
	thresholds := domain.PipelineThresholds{
		MinMarginPct:   strategy.SearchParams.MinMarginPct,
		MinSellerCount: strategy.SearchParams.MinSellerCount,
	}
	survivors, stats, err := s.funnel.ProcessBatch(ctx, tenantID, allInputs, thresholds)
	if err != nil {
		return nil, fmt.Errorf("funnel processing: %w", err)
	}

	slog.Info("discovery-queue: funnel complete",
		"tenant_id", tenantID,
		"input", stats.InputCount,
		"survivors", stats.SurvivorCount)

	// Cap survivors to remaining daily allowance
	if len(survivors) > remaining {
		survivors = survivors[:remaining]
	}

	// Create suggestions
	now := time.Now()
	var suggestions []domain.DiscoverySuggestion
	for _, surv := range survivors {
		suggestion := domain.DiscoverySuggestion{
			ID:                domain.SuggestionID(s.idGen.New()),
			TenantID:          tenantID,
			StrategyVersionID: strategy.ID,
			ASIN:              surv.ASIN,
			Title:             surv.Title,
			Brand:             surv.DiscoveredProduct.Category,
			Category:          surv.Category,
			BuyBoxPrice:       surv.BuyBoxPrice,
			EstimatedMargin:   surv.EstimatedMarginPct,
			BSRRank:           surv.BSRRank,
			SellerCount:       surv.SellerCount,
			Reason:            fmt.Sprintf("Matches strategy v%d — %s category, %.1f%% estimated margin", strategy.VersionNumber, surv.Category, surv.EstimatedMarginPct),
			Status:            domain.SuggestionStatusPending,
			CreatedAt:         now,
		}
		suggestions = append(suggestions, suggestion)
	}

	if len(suggestions) > 0 {
		if err := s.suggestions.CreateBatch(ctx, suggestions); err != nil {
			return nil, fmt.Errorf("save suggestions: %w", err)
		}
	}

	slog.Info("discovery-queue: suggestions created",
		"tenant_id", tenantID,
		"count", len(suggestions))

	return suggestions, nil
}

// AcceptSuggestion marks a suggestion as accepted. The caller should create a deal
// from the suggestion data and pass the deal ID.
func (s *DiscoveryQueueService) AcceptSuggestion(ctx context.Context, tenantID domain.TenantID, suggestionID domain.SuggestionID, dealID domain.DealID) error {
	return s.suggestions.Accept(ctx, suggestionID, dealID)
}

// DismissSuggestion marks a suggestion as dismissed.
// NOTE: This does NOT train preferences — dismissals are not used to bias future suggestions.
func (s *DiscoveryQueueService) DismissSuggestion(ctx context.Context, suggestionID domain.SuggestionID) error {
	return s.suggestions.Dismiss(ctx, suggestionID)
}

// ListPending returns pending suggestions for a tenant.
func (s *DiscoveryQueueService) ListPending(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoverySuggestion, error) {
	return s.suggestions.ListPending(ctx, tenantID, limit)
}

// ListAll returns all suggestions for a tenant.
func (s *DiscoveryQueueService) ListAll(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoverySuggestion, error) {
	return s.suggestions.ListAll(ctx, tenantID, limit)
}
