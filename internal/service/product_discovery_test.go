package service_test

import (
	"context"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type mockDiscoverySearcher struct{}

func (m *mockDiscoverySearcher) SearchProducts(_ context.Context, _ []string, _ string) ([]port.ProductSearchResult, error) {
	return []port.ProductSearchResult{
		{ASIN: "B001", Title: "Wholesale Product", Brand: "OpenBrand", AmazonPrice: 25.00, BSRRank: 100, SellerCount: 8},
		{ASIN: "B002", Title: "Private Label", Brand: "PLBrand", AmazonPrice: 15.00, BSRRank: 50, SellerCount: 1},
		{ASIN: "B003", Title: "Blocked Brand Item", Brand: "BlockedCo", AmazonPrice: 30.00, BSRRank: 200, SellerCount: 5},
		{ASIN: "B004", Title: "Low Margin Item", Brand: "CheapBrand", AmazonPrice: 5.00, BSRRank: 300, SellerCount: 6},
		{ASIN: "B005", Title: "Great Wholesale", Brand: "GoodBrand", AmazonPrice: 40.00, BSRRank: 150, SellerCount: 12},
	}, nil
}

func (m *mockDiscoverySearcher) GetProductDetails(_ context.Context, _ []string, _ string) ([]port.ProductSearchResult, error) {
	return nil, nil
}

func (m *mockDiscoverySearcher) EstimateFees(_ context.Context, _ string, _ float64, _ string) (*port.ProductFeeEstimate, error) {
	return nil, nil
}

func TestProductDiscovery_PreQualification(t *testing.T) {
	discovery := service.NewProductDiscovery(&mockDiscoverySearcher{}, nil)

	thresholds := domain.DefaultPipelineThresholds()
	thresholds.MinSellerCount = 3
	thresholds.MinMarginPct = 10
	thresholds.BrandFilter = domain.BrandFilter{
		BlockList: []string{"BlockedCo"},
	}

	results, err := discovery.DiscoverAndPreQualify(context.Background(), "test-tenant", domain.Criteria{
		Keywords: []string{"test"}, Marketplace: "US",
	}, thresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// B001 (8 sellers, $25, open brand) → pass
	// B002 (1 seller) → eliminated
	// B003 (blocked brand) → eliminated
	// B004 ($5, low margin) → eliminated
	// B005 (12 sellers, $40, good brand) → pass
	if len(results) != 2 {
		t.Fatalf("expected 2 qualified products, got %d", len(results))
	}

	if results[0].ASIN != "B001" {
		t.Errorf("expected B001 first (BSR 100), got %s (BSR %d)", results[0].ASIN, results[0].BSRRank)
	}
	if results[1].ASIN != "B005" {
		t.Errorf("expected B005 second (BSR 150), got %s", results[1].ASIN)
	}
	if results[0].EstimatedMarginPct <= 0 {
		t.Error("expected positive margin for B001")
	}
}

func TestProductDiscovery_NilSearcher(t *testing.T) {
	discovery := service.NewProductDiscovery(nil, nil)
	results, err := discovery.DiscoverAndPreQualify(context.Background(), "test-tenant", domain.Criteria{}, domain.DefaultPipelineThresholds())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Error("expected nil results with nil searcher")
	}
}

func (m *mockDiscoverySearcher) LookupByIdentifier(_ context.Context, _ []string, _ string, _ string) ([]port.ProductSearchResult, error) {
	return nil, nil
}

func (m *mockDiscoverySearcher) CheckListingEligibility(_ context.Context, asins []string, _ string) ([]port.ListingRestriction, error) {
	var results []port.ListingRestriction
	for _, asin := range asins {
		results = append(results, port.ListingRestriction{ASIN: asin, Allowed: true})
	}
	return results, nil
}
