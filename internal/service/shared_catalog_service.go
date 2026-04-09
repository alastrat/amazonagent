package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

const (
	// Products enriched within this window are considered fresh (0 credits).
	CatalogFreshnessWindow = 24 * time.Hour
)

// SharedCatalogService manages the platform-wide product catalog with credit-aware
// enrichment. Cached products are free. Fresh SP-API calls cost credits.
type SharedCatalogService struct {
	catalog     port.SharedCatalogRepo
	brands      port.BrandCatalogRepo
	eligibility port.TenantEligibilityRepo
	margins     port.TenantMarginRepo
	spapi       port.ProductSearcher
	credits     *CreditService
}

func NewSharedCatalogService(
	catalog port.SharedCatalogRepo,
	brands port.BrandCatalogRepo,
	eligibility port.TenantEligibilityRepo,
	margins port.TenantMarginRepo,
	spapi port.ProductSearcher,
	credits *CreditService,
) *SharedCatalogService {
	return &SharedCatalogService{
		catalog:     catalog,
		brands:      brands,
		eligibility: eligibility,
		margins:     margins,
		spapi:       spapi,
		credits:     credits,
	}
}

// EnrichProduct ensures a product has fresh data. If cached and fresh: free (0 credits).
// If stale or missing: calls SP-API (costs 1 credit), updates shared catalog.
// Returns the product data and whether credits were spent.
func (s *SharedCatalogService) EnrichProduct(ctx context.Context, tenantID domain.TenantID, asin string) (*domain.SharedProduct, bool, error) {
	// Check cache first
	cached, err := s.catalog.GetByASIN(ctx, asin)
	if err == nil && cached != nil && cached.IsFresh(CatalogFreshnessWindow) {
		return cached, false, nil // Free — cached and fresh
	}

	// Can't enrich without SP-API — return stale data
	if s.spapi == nil {
		return cached, false, nil
	}

	// Need fresh data — spend credit before calling SP-API
	if s.credits != nil {
		if !s.credits.SpendIfAvailable(ctx, tenantID, 1, domain.CreditActionEnrichment, asin) {
			if cached != nil {
				slog.Info("shared-catalog: returning stale data (no credits)", "asin", asin)
				return cached, false, nil
			}
			return nil, false, nil
		}
	}

	details, err := s.spapi.GetProductDetails(ctx, []string{asin}, "US")
	if err != nil {
		slog.Warn("shared-catalog: enrichment failed", "asin", asin, "error", err)
		return cached, false, err
	}

	if len(details) == 0 {
		return cached, true, nil
	}

	d := details[0]
	now := time.Now()
	product := &domain.SharedProduct{
		ASIN:           asin,
		Title:          d.Title,
		Brand:          d.Brand,
		Category:       d.Category,
		BSRRank:        d.BSRRank,
		SellerCount:    d.SellerCount,
		BuyBoxPrice:    d.AmazonPrice,
		LastEnrichedAt: &now,
		CreatedAt:      now,
	}

	// Calculate estimated margin using default wholesale ratio
	product.EstimatedMargin = domain.EstimateMarginPct(d.AmazonPrice)

	s.catalog.UpsertProduct(ctx, product)
	s.catalog.IncrementEnrichment(ctx, asin)

	// Update brand catalog
	if d.Brand != "" {
		s.upsertBrand(ctx, d.Brand, d.Category)
	}

	return product, true, nil
}

// CheckEligibility checks if a tenant can list a product. Cached checks are free.
// Fresh SP-API calls cost 1 credit. Stores result in tenant_product_eligibility.
func (s *SharedCatalogService) CheckEligibility(ctx context.Context, tenantID domain.TenantID, asin string) (*domain.TenantEligibility, bool, error) {
	// Check tenant-specific cache
	cached, err := s.eligibility.Get(ctx, tenantID, asin)
	if err == nil && cached != nil && time.Since(cached.CheckedAt) < 14*24*time.Hour {
		return cached, false, nil // Free — cached within 14 days
	}

	// Need fresh check — spend credit
	// Can't check without SP-API
	if s.spapi == nil {
		return &domain.TenantEligibility{TenantID: tenantID, ASIN: asin, Eligible: true, CheckedAt: time.Now()}, false, nil
	}

	// Spend credit before calling SP-API
	if s.credits != nil {
		if !s.credits.SpendIfAvailable(ctx, tenantID, 1, domain.CreditActionEligibilityCheck, asin) {
			if cached != nil {
				return cached, false, nil
			}
			return nil, false, nil
		}
	}

	restrictions, err := s.spapi.CheckListingEligibility(ctx, []string{asin}, "US")
	if err != nil {
		slog.Warn("shared-catalog: eligibility check failed", "asin", asin, "error", err)
		return cached, false, err
	}

	eligible := true
	reason := ""
	if len(restrictions) > 0 && !restrictions[0].Allowed {
		eligible = false
		reason = restrictions[0].Reason
	}

	result := &domain.TenantEligibility{
		TenantID:  tenantID,
		ASIN:      asin,
		Eligible:  eligible,
		Reason:    reason,
		CheckedAt: time.Now(),
	}
	s.eligibility.Set(ctx, result)

	// Update brand gating info in shared catalog
	product, _ := s.catalog.GetByASIN(ctx, asin)
	if product != nil && product.Brand != "" && !eligible {
		s.brands.UpdateGating(ctx, domain.NormalizeBrandName(product.Brand), "brand_gated")
	}

	return result, true, nil
}

// RecordFromScan writes products discovered during any tenant's scan into the shared catalog.
// This is how the catalog grows — every scan enriches the platform.
func (s *SharedCatalogService) RecordFromScan(ctx context.Context, products []port.ProductSearchResult) error {
	now := time.Now()
	shared := make([]domain.SharedProduct, 0, len(products))
	// Collect unique brands to upsert once per brand instead of per product.
	uniqueBrands := make(map[string]string) // normalized -> category (last seen)
	for _, p := range products {
		if p.ASIN == "" {
			continue
		}
		sp := domain.SharedProduct{
			ASIN:        p.ASIN,
			Title:       p.Title,
			Brand:       p.Brand,
			Category:    p.Category,
			BSRRank:     p.BSRRank,
			SellerCount: p.SellerCount,
			BuyBoxPrice: p.AmazonPrice,
			CreatedAt:   now,
		}
		sp.EstimatedMargin = domain.EstimateMarginPct(p.AmazonPrice)
		if p.AmazonPrice > 0 || p.BSRRank > 0 {
			sp.LastEnrichedAt = &now
		}
		shared = append(shared, sp)

		if p.Brand != "" {
			normalized := domain.NormalizeBrandName(p.Brand)
			if _, seen := uniqueBrands[normalized]; !seen {
				uniqueBrands[normalized] = p.Category
			}
		}
	}

	// Upsert each brand once.
	for normalized, category := range uniqueBrands {
		s.upsertBrand(ctx, normalized, category)
	}

	if len(shared) > 0 {
		if err := s.catalog.UpsertProductBatch(ctx, shared); err != nil {
			return err
		}
		slog.Info("shared-catalog: recorded from scan", "products", len(shared))
	}
	return nil
}

// GetCachedProducts returns products from the shared catalog without spending credits.
func (s *SharedCatalogService) GetCachedProducts(ctx context.Context, asins []string) ([]domain.SharedProduct, error) {
	return s.catalog.GetByASINs(ctx, asins)
}

// GetTenantEligibility returns cached eligibility for a tenant without spending credits.
func (s *SharedCatalogService) GetTenantEligibility(ctx context.Context, tenantID domain.TenantID, asins []string) ([]domain.TenantEligibility, error) {
	return s.eligibility.GetByASINs(ctx, tenantID, asins)
}

// RecordEligibility persists a tenant eligibility result in the shared catalog.
func (s *SharedCatalogService) RecordEligibility(ctx context.Context, te *domain.TenantEligibility) error {
	return s.eligibility.Set(ctx, te)
}

func (s *SharedCatalogService) upsertBrand(ctx context.Context, brandName, category string) {
	normalized := domain.NormalizeBrandName(brandName)
	if normalized == "" {
		return
	}
	brand := &domain.SharedBrand{
		Name:           brandName,
		NormalizedName: normalized,
		TypicalGating:  "unknown",
		Categories:     []string{category},
	}
	s.brands.Upsert(ctx, brand)
	s.brands.IncrementProductCount(ctx, normalized)
}
