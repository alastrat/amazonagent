package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// FunnelInput is a product entering the tiered elimination funnel from any source.
type FunnelInput struct {
	ASIN           string
	Title          string
	Brand          string
	Category       string
	EstimatedPrice float64        // from catalog search or CSV MSRP
	WholesaleCost  float64        // from price list (0 if unknown — estimate at 40%)
	BSRRank        int
	SellerCount    int            // may be 0 if not yet enriched
	Source         domain.ScanType
}

// FunnelSurvivor is a product that passed T0-T3 and is ready for T4 (LLM).
type FunnelSurvivor struct {
	domain.DiscoveredProduct
	WholesaleCost float64 `json:"wholesale_cost"` // carried from input (price list only)
}

// FunnelStats tracks elimination counts at each tier for observability.
type FunnelStats struct {
	InputCount     int `json:"input_count"`
	T0Deduped      int `json:"t0_deduped"`
	T1MarginKilled int `json:"t1_margin_killed"`
	T2BrandKilled  int `json:"t2_brand_killed"`
	T3EnrichKilled int `json:"t3_enrich_killed"`
	SurvivorCount  int `json:"survivor_count"`
}

// FunnelService runs the tiered elimination funnel (T0-T3) on a batch of products.
type FunnelService struct {
	catalog          *CatalogService
	brandEligibility *BrandEligibilityService
	brandBlocklist   *BrandBlocklistService
	spapi            port.ProductSearcher
}

func NewFunnelService(
	catalog *CatalogService,
	brandEligibility *BrandEligibilityService,
	brandBlocklist *BrandBlocklistService,
	spapi port.ProductSearcher,
) *FunnelService {
	return &FunnelService{
		catalog:          catalog,
		brandEligibility: brandEligibility,
		brandBlocklist:   brandBlocklist,
		spapi:            spapi,
	}
}

// ProcessBatch runs T0-T3 on a batch of products and returns survivors for T4 (LLM).
func (s *FunnelService) ProcessBatch(
	ctx context.Context,
	tenantID domain.TenantID,
	products []FunnelInput,
	thresholds domain.PipelineThresholds,
) ([]FunnelSurvivor, FunnelStats, error) {
	stats := FunnelStats{InputCount: len(products)}

	if len(products) == 0 {
		return nil, stats, nil
	}

	slog.Info("funnel: starting", "input", len(products), "tenant_id", tenantID)

	// ─── T0: Dedup — skip ASINs already in catalog with fresh data ───
	var t0Survivors []FunnelInput
	if s.catalog != nil {
		asins := make([]string, len(products))
		for i, p := range products {
			asins[i] = p.ASIN
		}
		cached, _ := s.catalog.GetByASINs(ctx, tenantID, asins)
		cachedMap := make(map[string]domain.DiscoveredProduct)
		for _, c := range cached {
			if c.PriceUpdatedAt != nil && time.Since(*c.PriceUpdatedAt) < 24*time.Hour {
				cachedMap[c.ASIN] = c
			}
		}
		for _, p := range products {
			if _, fresh := cachedMap[p.ASIN]; fresh {
				stats.T0Deduped++
			} else {
				t0Survivors = append(t0Survivors, p)
			}
		}
	} else {
		t0Survivors = products
	}

	slog.Info("funnel: T0 dedup complete", "deduped", stats.T0Deduped, "remaining", len(t0Survivors))

	// ─── T1: Local math — margin filter using estimated or real wholesale cost ───
	var t1Survivors []FunnelInput
	for _, p := range t0Survivors {
		// Skip products with no price data
		if p.EstimatedPrice <= 0 {
			t1Survivors = append(t1Survivors, p)
			continue
		}

		// Price range filter ($10-$200 default)
		if p.EstimatedPrice < 10.0 || p.EstimatedPrice > 200.0 {
			slog.Debug("funnel: T1 eliminated (price range)", "asin", p.ASIN, "price", p.EstimatedPrice)
			stats.T1MarginKilled++
			continue
		}

		// Calculate margin
		wholesaleCost := p.WholesaleCost
		if wholesaleCost <= 0 {
			wholesaleCost = p.EstimatedPrice * 0.4 // estimate at 40% of retail
		}
		fbaCalc := domain.CalculateFBAFees(p.EstimatedPrice, wholesaleCost, 1.0, false)

		if thresholds.MinMarginPct > 0 && fbaCalc.NetMarginPct < thresholds.MinMarginPct {
			slog.Debug("funnel: T1 eliminated (margin)", "asin", p.ASIN, "margin", fbaCalc.NetMarginPct, "min", thresholds.MinMarginPct)
			stats.T1MarginKilled++
			continue
		}

		t1Survivors = append(t1Survivors, p)
	}

	slog.Info("funnel: T1 margin complete", "killed", stats.T1MarginKilled, "remaining", len(t1Survivors))

	// ─── T2: Brand gate — cached brand eligibility + blocklist ───
	var t2Survivors []FunnelInput
	if s.brandEligibility != nil && len(t1Survivors) > 0 {
		// Build batch for brand check
		type brandProduct struct {
			ASIN  string
			Brand string
		}
		var bps []struct {
			ASIN  string
			Brand string
		}
		for _, p := range t1Survivors {
			bps = append(bps, struct {
				ASIN  string
				Brand string
			}{p.ASIN, p.Brand})
		}

		eligibilityMap := s.brandEligibility.BatchCheckBrands(ctx, tenantID, bps)

		for _, p := range t1Survivors {
			// Check blocklist
			if !thresholds.BrandFilter.IsBrandAllowed(p.Brand) {
				slog.Debug("funnel: T2 eliminated (blocklist)", "asin", p.ASIN, "brand", p.Brand)
				stats.T2BrandKilled++
				continue
			}

			// Check eligibility
			if isEligible, ok := eligibilityMap[p.ASIN]; ok && !isEligible {
				slog.Debug("funnel: T2 eliminated (restricted)", "asin", p.ASIN, "brand", p.Brand)
				stats.T2BrandKilled++
				continue
			}

			t2Survivors = append(t2Survivors, p)
		}
	} else {
		// No brand service — just apply blocklist filter
		for _, p := range t1Survivors {
			if !thresholds.BrandFilter.IsBrandAllowed(p.Brand) {
				stats.T2BrandKilled++
				continue
			}
			t2Survivors = append(t2Survivors, p)
		}
	}

	slog.Info("funnel: T2 brand gate complete", "killed", stats.T2BrandKilled, "remaining", len(t2Survivors))

	// ─── T3: Competitive pricing enrichment — real Buy Box price + seller count ───
	var survivors []FunnelSurvivor
	if s.spapi != nil && len(t2Survivors) > 0 {
		// Batch ASINs in groups of 20 for competitive pricing
		asins := make([]string, len(t2Survivors))
		inputMap := make(map[string]FunnelInput)
		for i, p := range t2Survivors {
			asins[i] = p.ASIN
			inputMap[p.ASIN] = p
		}

		// Call SP-API GetProductDetails (competitive pricing, batched at 20)
		for i := 0; i < len(asins); i += 20 {
			end := i + 20
			if end > len(asins) {
				end = len(asins)
			}
			batch := asins[i:end]

			details, err := s.spapi.GetProductDetails(ctx, batch, "US")
			if err != nil {
				slog.Warn("funnel: T3 competitive pricing failed, keeping batch", "error", err)
				// On failure, keep all products with estimated data
				for _, asin := range batch {
					p := inputMap[asin]
					survivors = append(survivors, s.buildSurvivor(p, 0, 0, 0))
				}
				continue
			}

			// Build enrichment map
			enrichMap := make(map[string]port.ProductSearchResult)
			for _, d := range details {
				enrichMap[d.ASIN] = d
			}

			for _, asin := range batch {
				p := inputMap[asin]
				enriched, hasEnrichment := enrichMap[asin]

				buyBoxPrice := p.EstimatedPrice
				sellerCount := p.SellerCount
				bsr := p.BSRRank

				if hasEnrichment {
					if enriched.AmazonPrice > 0 {
						buyBoxPrice = enriched.AmazonPrice
					}
					if enriched.SellerCount > 0 {
						sellerCount = enriched.SellerCount
					}
					if enriched.BSRRank > 0 {
						bsr = enriched.BSRRank
					}
				}

				// Apply seller count filter
				if thresholds.MinSellerCount > 0 && sellerCount > 0 && sellerCount < thresholds.MinSellerCount {
					slog.Debug("funnel: T3 eliminated (sellers)", "asin", asin, "sellers", sellerCount, "min", thresholds.MinSellerCount)
					stats.T3EnrichKilled++
					continue
				}

				// Recalculate margin with real Buy Box price
				wholesaleCost := p.WholesaleCost
				if wholesaleCost <= 0 {
					wholesaleCost = buyBoxPrice * 0.4
				}
				fbaCalc := domain.CalculateFBAFees(buyBoxPrice, wholesaleCost, 1.0, false)

				if thresholds.MinMarginPct > 0 && fbaCalc.NetMarginPct < thresholds.MinMarginPct {
					slog.Debug("funnel: T3 eliminated (real margin)", "asin", asin, "margin", fbaCalc.NetMarginPct, "buy_box", buyBoxPrice)
					stats.T3EnrichKilled++
					continue
				}

				survivor := s.buildSurvivor(p, buyBoxPrice, sellerCount, bsr)
				survivor.RealMarginPct = fbaCalc.NetMarginPct
				survivor.DataQuality |= domain.DataQualityBuyBox
				now := time.Now()
				survivor.PriceUpdatedAt = &now
				survivors = append(survivors, survivor)
			}
		}
	} else {
		// No SP-API — pass all through with estimated data
		for _, p := range t2Survivors {
			survivors = append(survivors, s.buildSurvivor(p, 0, 0, 0))
		}
	}

	stats.SurvivorCount = len(survivors)

	slog.Info("funnel: T3 enrich complete", "killed", stats.T3EnrichKilled, "survivors", len(survivors))
	slog.Info("funnel: complete",
		"input", stats.InputCount,
		"t0_deduped", stats.T0Deduped,
		"t1_margin", stats.T1MarginKilled,
		"t2_brand", stats.T2BrandKilled,
		"t3_enrich", stats.T3EnrichKilled,
		"survivors", stats.SurvivorCount,
	)

	// Write survivors to persistent catalog
	if s.catalog != nil && len(survivors) > 0 {
		catalogProducts := make([]domain.DiscoveredProduct, len(survivors))
		for i, s := range survivors {
			catalogProducts[i] = s.DiscoveredProduct
		}
		if err := s.catalog.UpsertProducts(ctx, tenantID, catalogProducts); err != nil {
			slog.Warn("funnel: failed to persist catalog", "error", err)
		}
	}

	return survivors, stats, nil
}

func (s *FunnelService) buildSurvivor(p FunnelInput, buyBoxPrice float64, sellerCount int, bsr int) FunnelSurvivor {
	now := time.Now()
	dp := domain.DiscoveredProduct{
		TenantID:       "",
		ASIN:           p.ASIN,
		Title:          p.Title,
		Category:       p.Category,
		EstimatedPrice: p.EstimatedPrice,
		BuyBoxPrice:    buyBoxPrice,
		BSRRank:        bsr,
		SellerCount:    sellerCount,
		Source:         p.Source,
		FirstSeenAt:    now,
		LastSeenAt:     now,
	}

	// Calculate estimated margin
	if p.EstimatedPrice > 0 {
		wc := p.WholesaleCost
		if wc <= 0 {
			wc = p.EstimatedPrice * 0.4
		}
		fbaCalc := domain.CalculateFBAFees(p.EstimatedPrice, wc, 1.0, false)
		dp.EstimatedMarginPct = fbaCalc.NetMarginPct
		dp.DataQuality |= domain.DataQualityPrice | domain.DataQualityFees
	}

	if bsr > 0 {
		dp.DataQuality |= domain.DataQualityBSR
	}

	return FunnelSurvivor{
		DiscoveredProduct: dp,
		WholesaleCost:     p.WholesaleCost,
	}
}
