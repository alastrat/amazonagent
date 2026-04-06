package service_test

import (
	"context"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type mockProductSearcher struct{}

func (m *mockProductSearcher) SearchProducts(_ context.Context, _ []string, _ string) ([]port.ProductSearchResult, error) {
	return []port.ProductSearchResult{
		{ASIN: "B0TEST001", Title: "Test Product", Brand: "TestBrand", AmazonPrice: 25.00, BSRRank: 5000},
	}, nil
}
func (m *mockProductSearcher) GetProductDetails(_ context.Context, _ []string, _ string) ([]port.ProductSearchResult, error) {
	return nil, nil
}
func (m *mockProductSearcher) EstimateFees(_ context.Context, _ string, price float64, _ string) (*port.ProductFeeEstimate, error) {
	return &port.ProductFeeEstimate{ReferralFee: price * 0.15, FBAFee: 3.50, TotalFees: price*0.15 + 3.50}, nil
}

type mockWebSearcher2 struct{}

func (m *mockWebSearcher2) Search(_ context.Context, _ string, _ int) ([]port.WebSearchResult, error) {
	return []port.WebSearchResult{
		{Title: "Test Result", URL: "https://example.com", Snippet: "Test snippet", Score: 0.9},
	}, nil
}

type mockWebScraper2 struct{}

func (m *mockWebScraper2) Scrape(_ context.Context, url string) (*port.ScrapedPage, error) {
	return &port.ScrapedPage{URL: url, Title: "Test", Content: "Test content"}, nil
}
func (m *mockWebScraper2) ScrapeMultiple(_ context.Context, urls []string) ([]port.ScrapedPage, error) {
	var pages []port.ScrapedPage
	for _, u := range urls {
		pages = append(pages, port.ScrapedPage{URL: u, Title: "Test", Content: "Content"})
	}
	return pages, nil
}

func TestToolResolver_ResolveForSourcing(t *testing.T) {
	resolver := service.NewToolResolver(&mockProductSearcher{}, &mockWebSearcher2{}, &mockWebScraper2{})
	data, err := resolver.ResolveForSourcing(context.Background(), domain.Criteria{
		Keywords: []string{"kitchen gadgets"}, Marketplace: "US",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["amazon_products"] == nil {
		t.Error("expected amazon_products")
	}
	if data["web_research"] == nil {
		t.Error("expected web_research")
	}
}

func TestToolResolver_ResolveForProfitability(t *testing.T) {
	resolver := service.NewToolResolver(&mockProductSearcher{}, nil, nil)
	data, err := resolver.ResolveForProfitability(context.Background(), map[string]any{
		"asin": "B0TEST001", "amazon_price": 25.00,
	}, "US")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["fee_estimate"] == nil {
		t.Error("expected fee_estimate")
	}
	if data["fba_calculation"] == nil {
		t.Error("expected fba_calculation")
	}
}

func TestToolResolver_ResolveForSupplier(t *testing.T) {
	resolver := service.NewToolResolver(nil, &mockWebSearcher2{}, &mockWebScraper2{})
	data, err := resolver.ResolveForSupplier(context.Background(), map[string]any{
		"brand": "TestBrand", "title": "Test Product",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["supplier_search_results"] == nil {
		t.Error("expected supplier_search_results")
	}
}

func TestToolResolver_NilProviders(t *testing.T) {
	resolver := service.NewToolResolver(nil, nil, nil)
	data, err := resolver.ResolveForSourcing(context.Background(), domain.Criteria{Keywords: []string{"test"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["criteria"] == nil {
		t.Error("expected criteria")
	}
}

func (m *mockProductSearcher) CheckListingEligibility(_ context.Context, asins []string, _ string) ([]port.ListingRestriction, error) {
	var results []port.ListingRestriction
	for _, asin := range asins {
		results = append(results, port.ListingRestriction{ASIN: asin, Allowed: true})
	}
	return results, nil
}
