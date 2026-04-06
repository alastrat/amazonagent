package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// BrandEligibilityService caches brand-level eligibility to avoid redundant
// SP-API calls. One check per brand covers all products from that brand.
type BrandEligibilityService struct {
	brandRepo BrandRepo
	products  port.ProductSearcher
	maxAge    time.Duration
}

// BrandRepo defines the interface for the brand repo (to avoid import cycle).
type BrandRepo interface {
	GetOrCreateBrand(ctx context.Context, brandName string) (*domain.Brand, error)
	GetEligibility(ctx context.Context, tenantID domain.TenantID, brandID domain.BrandID) (*domain.BrandEligibility, error)
	SetEligibility(ctx context.Context, be *domain.BrandEligibility) error
}

func NewBrandEligibilityService(brandRepo BrandRepo, products port.ProductSearcher, maxAge time.Duration) *BrandEligibilityService {
	if maxAge == 0 {
		maxAge = 7 * 24 * time.Hour // default: 7 days
	}
	return &BrandEligibilityService{
		brandRepo: brandRepo,
		products:  products,
		maxAge:    maxAge,
	}
}

// CheckBrandEligibility checks if a brand is eligible for a tenant.
// Uses cache if fresh, otherwise checks SP-API and caches the result.
// Returns: eligible (bool), reason (string)
func (s *BrandEligibilityService) CheckBrandEligibility(ctx context.Context, tenantID domain.TenantID, brandName string, sampleASIN string) (bool, string) {
	if brandName == "" {
		return true, "" // can't check without brand name
	}

	// Get or create the brand record
	brand, err := s.brandRepo.GetOrCreateBrand(ctx, brandName)
	if err != nil {
		slog.Warn("brand-eligibility: failed to get/create brand", "brand", brandName, "error", err)
		return true, "" // fail open
	}

	// Check cache
	cached, err := s.brandRepo.GetEligibility(ctx, tenantID, brand.ID)
	if err == nil && time.Since(cached.CheckedAt) < s.maxAge {
		// Cache hit — use cached result
		slog.Debug("brand-eligibility: cache hit", "brand", brandName, "status", cached.Status)
		return cached.Status == domain.EligibilityEligible, cached.Reason
	}

	// Cache miss or stale — check SP-API
	if s.products == nil || sampleASIN == "" {
		return true, "" // can't check without product searcher or sample ASIN
	}

	restrictions, err := s.products.CheckListingEligibility(ctx, []string{sampleASIN}, "US")
	if err != nil {
		slog.Warn("brand-eligibility: SP-API check failed", "brand", brandName, "asin", sampleASIN, "error", err)
		return true, "" // fail open on API error
	}

	// Determine status from restriction result
	status := domain.EligibilityEligible
	reason := ""
	if len(restrictions) > 0 && !restrictions[0].Allowed {
		status = domain.EligibilityRestricted
		reason = restrictions[0].Reason
	}

	// Cache the result
	be := &domain.BrandEligibility{
		TenantID:   tenantID,
		BrandID:    brand.ID,
		Status:     status,
		Reason:     reason,
		SampleASIN: sampleASIN,
		CheckedAt:  time.Now(),
	}
	if err := s.brandRepo.SetEligibility(ctx, be); err != nil {
		slog.Warn("brand-eligibility: failed to cache", "brand", brandName, "error", err)
	}

	if status == domain.EligibilityRestricted {
		slog.Info("brand-eligibility: restricted (cached)", "brand", brandName, "reason", reason)
	} else {
		slog.Info("brand-eligibility: eligible (cached)", "brand", brandName)
	}

	return status == domain.EligibilityEligible, reason
}

// BatchCheckBrands checks eligibility for multiple products, grouped by brand.
// Returns a map of ASIN → eligible (bool).
// This is the key optimization: N products with K brands → K API calls, not N.
func (s *BrandEligibilityService) BatchCheckBrands(ctx context.Context, tenantID domain.TenantID, products []struct {
	ASIN  string
	Brand string
}) map[string]bool {
	result := make(map[string]bool)

	// Group products by brand, pick one sample ASIN per brand
	brandSamples := make(map[string]string) // brand → sample ASIN
	productBrands := make(map[string]string) // ASIN → brand

	for _, p := range products {
		if p.Brand == "" {
			result[p.ASIN] = true // can't check, assume eligible
			continue
		}
		productBrands[p.ASIN] = p.Brand
		if _, exists := brandSamples[p.Brand]; !exists {
			brandSamples[p.Brand] = p.ASIN
		}
	}

	slog.Info("brand-eligibility: batch check",
		"products", len(products),
		"unique_brands", len(brandSamples),
		"saved_api_calls", len(products)-len(brandSamples),
	)

	// Check each unique brand (using cache when possible)
	brandEligible := make(map[string]bool)
	for brand, sampleASIN := range brandSamples {
		eligible, _ := s.CheckBrandEligibility(ctx, tenantID, brand, sampleASIN)
		brandEligible[brand] = eligible
	}

	// Map back to ASINs
	for asin, brand := range productBrands {
		result[asin] = brandEligible[brand]
	}

	return result
}
