package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// ToolResolver pre-resolves external API data for pipeline agents.
// Agents receive structured data — they never call external APIs directly.
type ToolResolver struct {
	products port.ProductSearcher
	search   port.WebSearcher
	scraper  port.WebScraper
}

func NewToolResolver(products port.ProductSearcher, search port.WebSearcher, scraper port.WebScraper) *ToolResolver {
	return &ToolResolver{products: products, search: search, scraper: scraper}
}

// ResolveForSourcing gets product search results and web context.
func (r *ToolResolver) ResolveForSourcing(ctx context.Context, criteria domain.Criteria) (map[string]any, error) {
	slog.Info("tool-resolver: resolving sourcing data", "keywords", criteria.Keywords)

	resolved := map[string]any{"criteria": criteria}

	if r.products != nil {
		products, err := r.products.SearchProducts(ctx, criteria.Keywords, criteria.Marketplace)
		if err != nil {
			slog.Warn("tool-resolver: product search failed", "error", err)
		} else {
			resolved["amazon_products"] = products
			slog.Info("tool-resolver: found products", "count", len(products))
		}
	}

	if r.search != nil {
		query := fmt.Sprintf("%s amazon wholesale FBA", strings.Join(criteria.Keywords, " "))
		webResults, err := r.search.Search(ctx, query, 5)
		if err != nil {
			slog.Warn("tool-resolver: web search failed", "error", err)
		} else {
			resolved["web_research"] = webResults
		}
	}

	return resolved, nil
}

// ResolveForGating gets category gating and restriction data.
func (r *ToolResolver) ResolveForGating(ctx context.Context, candidate map[string]any, marketplace string) (map[string]any, error) {
	resolved := make(map[string]any)
	for k, v := range candidate {
		resolved[k] = v
	}
	return resolved, nil
}

// ResolveForProfitability gets fee estimates and pricing data.
func (r *ToolResolver) ResolveForProfitability(ctx context.Context, candidate map[string]any, marketplace string) (map[string]any, error) {
	resolved := make(map[string]any)
	for k, v := range candidate {
		resolved[k] = v
	}

	asin, _ := candidate["asin"].(string)
	price, _ := candidate["amazon_price"].(float64)

	if r.products != nil && asin != "" && price > 0 {
		fees, err := r.products.EstimateFees(ctx, asin, price, marketplace)
		if err != nil {
			slog.Warn("tool-resolver: fee estimate failed", "asin", asin, "error", err)
		} else {
			resolved["fee_estimate"] = fees
		}
	}

	if price > 0 {
		wholesaleCost, _ := candidate["estimated_wholesale"].(float64)
		if wholesaleCost <= 0 {
			wholesaleCost = price * 0.4
		}
		fbaCalc := domain.CalculateFBAFees(price, wholesaleCost, 1.0, false)
		resolved["fba_calculation"] = fbaCalc
	}

	return resolved, nil
}

// ResolveForDemand gets demand signals — BSR trends, social sentiment.
func (r *ToolResolver) ResolveForDemand(ctx context.Context, candidate map[string]any, marketplace string) (map[string]any, error) {
	resolved := make(map[string]any)
	for k, v := range candidate {
		resolved[k] = v
	}

	title, _ := candidate["title"].(string)
	brand, _ := candidate["brand"].(string)

	if r.search != nil && title != "" {
		query := fmt.Sprintf("%s %s reviews demand amazon", title, brand)
		results, err := r.search.Search(ctx, query, 3)
		if err != nil {
			slog.Warn("tool-resolver: demand search failed", "error", err)
		} else {
			resolved["demand_research"] = results
		}

		compQuery := fmt.Sprintf("%s amazon FBA sellers competition", title)
		compResults, err := r.search.Search(ctx, compQuery, 3)
		if err != nil {
			slog.Warn("tool-resolver: competition search failed", "error", err)
		} else {
			resolved["competition_research"] = compResults
		}
	}

	return resolved, nil
}

// ResolveForSupplier gets supplier discovery data.
func (r *ToolResolver) ResolveForSupplier(ctx context.Context, candidate map[string]any) (map[string]any, error) {
	resolved := make(map[string]any)
	for k, v := range candidate {
		resolved[k] = v
	}

	brand, _ := candidate["brand"].(string)
	title, _ := candidate["title"].(string)

	if r.search != nil && brand != "" {
		query := fmt.Sprintf("%s wholesale distributor authorized", brand)
		results, err := r.search.Search(ctx, query, 5)
		if err != nil {
			slog.Warn("tool-resolver: supplier search failed", "error", err)
		} else {
			resolved["supplier_search_results"] = results

			if r.scraper != nil && len(results) > 0 {
				var urls []string
				for i, result := range results {
					if i >= 3 {
						break
					}
					urls = append(urls, result.URL)
				}
				pages, err := r.scraper.ScrapeMultiple(ctx, urls)
				if err != nil {
					slog.Warn("tool-resolver: supplier scrape failed", "error", err)
				} else {
					resolved["supplier_page_contents"] = pages
				}
			}
		}
	}

	if title != "" {
		query := fmt.Sprintf("%s wholesale supplier MOQ", title)
		results, err := r.search.Search(ctx, query, 3)
		if err == nil {
			resolved["product_supplier_search"] = results
		}
	}

	return resolved, nil
}
