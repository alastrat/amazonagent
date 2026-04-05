package service

import (
	"context"
	"log/slog"
	"sort"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// ProductDiscovery performs deterministic product discovery and pre-qualification.
// It replaces the LLM sourcing agent's role in finding candidates.
// All filtering happens BEFORE any LLM calls.
type ProductDiscovery struct {
	products port.ProductSearcher
}

func NewProductDiscovery(products port.ProductSearcher) *ProductDiscovery {
	return &ProductDiscovery{products: products}
}

// DiscoveredProduct is a pre-qualified product with all data resolved.
type DiscoveredProduct struct {
	ASIN               string                  `json:"asin"`
	Title              string                  `json:"title"`
	Brand              string                  `json:"brand"`
	Category           string                  `json:"category"`
	AmazonPrice        float64                 `json:"amazon_price"`
	BSRRank            int                     `json:"bsr_rank"`
	BSRCategory        string                  `json:"bsr_category"`
	SellerCount        int                     `json:"seller_count"`
	ReviewCount        int                     `json:"review_count"`
	EstimatedMarginPct float64                 `json:"estimated_margin_pct"`
	FBACalculation     domain.FBAFeeCalculation `json:"fba_calculation"`
}

// DiscoverAndPreQualify performs the full deterministic discovery:
// 1. Search SP-API by keywords
// 2. Batch enrich with competitive pricing (price + seller count)
// 3. Pre-filter: seller count, brand blocklist, margin, BSR range
// 4. Sort by opportunity score
func (d *ProductDiscovery) DiscoverAndPreQualify(
	ctx context.Context,
	criteria domain.Criteria,
	thresholds domain.PipelineThresholds,
) ([]DiscoveredProduct, error) {
	if d.products == nil {
		slog.Warn("product-discovery: no product searcher configured")
		return nil, nil
	}

	slog.Info("product-discovery: searching", "keywords", criteria.Keywords, "marketplace", criteria.Marketplace)
	rawProducts, err := d.products.SearchProducts(ctx, criteria.Keywords, criteria.Marketplace)
	if err != nil {
		return nil, err
	}
	slog.Info("product-discovery: found raw products", "count", len(rawProducts))

	if len(rawProducts) == 0 {
		return nil, nil
	}

	// Batch enrich with competitive pricing
	asins := make([]string, len(rawProducts))
	for i, p := range rawProducts {
		asins[i] = p.ASIN
	}
	enriched, err := d.products.GetProductDetails(ctx, asins, criteria.Marketplace)
	if err != nil {
		slog.Warn("product-discovery: batch enrichment failed, using raw data", "error", err)
		enriched = rawProducts
	}

	// Merge enriched data back
	productMap := make(map[string]port.ProductSearchResult)
	for _, p := range rawProducts {
		productMap[p.ASIN] = p
	}
	for _, e := range enriched {
		if existing, ok := productMap[e.ASIN]; ok {
			if e.AmazonPrice > 0 {
				existing.AmazonPrice = e.AmazonPrice
			}
			if e.SellerCount > 0 {
				existing.SellerCount = e.SellerCount
			}
			productMap[e.ASIN] = existing
		}
	}

	// Deterministic pre-filter
	var qualified []DiscoveredProduct
	var eliminated int

	for _, p := range productMap {
		if p.ASIN == "" {
			continue
		}

		if thresholds.MinSellerCount > 0 && p.SellerCount > 0 && p.SellerCount < thresholds.MinSellerCount {
			eliminated++
			continue
		}

		if !thresholds.BrandFilter.IsBrandAllowed(p.Brand) {
			eliminated++
			continue
		}

		var marginPct float64
		var fbaCalc domain.FBAFeeCalculation
		if p.AmazonPrice > 0 {
			wholesaleCost := p.AmazonPrice * 0.4
			fbaCalc = domain.CalculateFBAFees(p.AmazonPrice, wholesaleCost, 1.0, false)
			marginPct = fbaCalc.NetMarginPct

			if thresholds.MinMarginPct > 0 && marginPct < thresholds.MinMarginPct {
				eliminated++
				continue
			}
		}

		qualified = append(qualified, DiscoveredProduct{
			ASIN:               p.ASIN,
			Title:              p.Title,
			Brand:              p.Brand,
			Category:           p.Category,
			AmazonPrice:        p.AmazonPrice,
			BSRRank:            p.BSRRank,
			BSRCategory:        p.BSRCategory,
			SellerCount:        p.SellerCount,
			ReviewCount:        p.ReviewCount,
			EstimatedMarginPct: marginPct,
			FBACalculation:     fbaCalc,
		})
	}

	// Sort by BSR (lower = better)
	sort.Slice(qualified, func(i, j int) bool {
		if qualified[i].BSRRank > 0 && qualified[j].BSRRank > 0 {
			return qualified[i].BSRRank < qualified[j].BSRRank
		}
		if qualified[i].BSRRank > 0 {
			return true
		}
		return qualified[i].EstimatedMarginPct > qualified[j].EstimatedMarginPct
	})

	if len(qualified) > 15 {
		qualified = qualified[:15]
	}

	slog.Info("product-discovery: pre-qualification complete",
		"raw", len(rawProducts), "eliminated", eliminated, "qualified", len(qualified))

	return qualified, nil
}

// ToCandidate converts a DiscoveredProduct to a map[string]any for the pipeline.
func (p DiscoveredProduct) ToCandidate() map[string]any {
	return map[string]any{
		"asin":                 p.ASIN,
		"title":                p.Title,
		"brand":                p.Brand,
		"category":             p.Category,
		"amazon_price":         p.AmazonPrice,
		"bsr_rank":             p.BSRRank,
		"bsr_category":         p.BSRCategory,
		"seller_count":         p.SellerCount,
		"review_count":         p.ReviewCount,
		"estimated_margin_pct": p.EstimatedMarginPct,
		"fba_calculation":      p.FBACalculation,
	}
}
