package service

import (
	"context"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

func TestFunnelService_T1MarginFilter(t *testing.T) {
	// No catalog, no brand service, no SP-API — tests T1 only
	funnel := NewFunnelService(nil, nil, nil, nil)
	thresholds := domain.DefaultPipelineThresholds()
	thresholds.MinMarginPct = 10.0

	products := []FunnelInput{
		{ASIN: "B001", Title: "Good product", EstimatedPrice: 50.0, WholesaleCost: 20.0, Source: domain.ScanTypePriceList},
		{ASIN: "B002", Title: "Cheap product", EstimatedPrice: 5.0, WholesaleCost: 4.0, Source: domain.ScanTypePriceList},   // price too low (<$10)
		{ASIN: "B003", Title: "Expensive product", EstimatedPrice: 300.0, WholesaleCost: 100.0, Source: domain.ScanTypePriceList}, // price too high (>$200)
		{ASIN: "B004", Title: "Low margin product", EstimatedPrice: 20.0, WholesaleCost: 19.0, Source: domain.ScanTypePriceList}, // margin too low
		{ASIN: "B005", Title: "Another good product", EstimatedPrice: 30.0, WholesaleCost: 10.0, Source: domain.ScanTypePriceList},
	}

	survivors, stats, err := funnel.ProcessBatch(context.Background(), "tenant1", products, thresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.InputCount != 5 {
		t.Errorf("expected input count 5, got %d", stats.InputCount)
	}
	if stats.T1MarginKilled != 3 {
		t.Errorf("expected T1 killed 3, got %d", stats.T1MarginKilled)
	}
	if len(survivors) != 2 {
		t.Errorf("expected 2 survivors, got %d", len(survivors))
	}

	// Verify survivors are the right products
	asinSet := make(map[string]bool)
	for _, s := range survivors {
		asinSet[s.ASIN] = true
	}
	if !asinSet["B001"] || !asinSet["B005"] {
		t.Errorf("expected B001 and B005 to survive, got %v", asinSet)
	}
}

func TestFunnelService_T2BrandFilter(t *testing.T) {
	funnel := NewFunnelService(nil, nil, nil, nil)
	thresholds := domain.DefaultPipelineThresholds()
	thresholds.MinMarginPct = 0 // disable margin filter for this test
	thresholds.BrandFilter = domain.BrandFilter{
		BlockList: []string{"BlockedBrand"},
	}

	products := []FunnelInput{
		{ASIN: "B001", Title: "Good brand", Brand: "GoodBrand", EstimatedPrice: 30.0, WholesaleCost: 10.0},
		{ASIN: "B002", Title: "Blocked brand", Brand: "BlockedBrand", EstimatedPrice: 30.0, WholesaleCost: 10.0},
		{ASIN: "B003", Title: "No brand", Brand: "", EstimatedPrice: 30.0, WholesaleCost: 10.0},
	}

	survivors, stats, err := funnel.ProcessBatch(context.Background(), "tenant1", products, thresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.T2BrandKilled != 1 {
		t.Errorf("expected T2 killed 1, got %d", stats.T2BrandKilled)
	}
	if len(survivors) != 2 {
		t.Errorf("expected 2 survivors, got %d", len(survivors))
	}
}

func TestFunnelService_StatsAccuracy(t *testing.T) {
	funnel := NewFunnelService(nil, nil, nil, nil)
	thresholds := domain.DefaultPipelineThresholds()
	thresholds.MinMarginPct = 15.0
	thresholds.BrandFilter = domain.BrandFilter{
		BlockList: []string{"BadBrand"},
	}

	products := []FunnelInput{
		{ASIN: "B001", Title: "Winner", Brand: "Good", EstimatedPrice: 50.0, WholesaleCost: 15.0},
		{ASIN: "B002", Title: "Too cheap", Brand: "Good", EstimatedPrice: 5.0, WholesaleCost: 2.0},
		{ASIN: "B003", Title: "Bad brand", Brand: "BadBrand", EstimatedPrice: 50.0, WholesaleCost: 15.0},
		{ASIN: "B004", Title: "Low margin", Brand: "Good", EstimatedPrice: 20.0, WholesaleCost: 18.0},
	}

	_, stats, err := funnel.ProcessBatch(context.Background(), "tenant1", products, thresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// T1 kills B002 (price < $10) and B004 (margin too low) = 2
	// T2 kills B003 (blocklisted) = 1
	// Survivor: B001 only
	total := stats.T0Deduped + stats.T1MarginKilled + stats.T2BrandKilled + stats.T3EnrichKilled + stats.SurvivorCount
	if total != stats.InputCount {
		t.Errorf("stats don't add up: %d + %d + %d + %d + %d = %d, want %d",
			stats.T0Deduped, stats.T1MarginKilled, stats.T2BrandKilled, stats.T3EnrichKilled, stats.SurvivorCount,
			total, stats.InputCount)
	}
	if stats.SurvivorCount != 1 {
		t.Errorf("expected 1 survivor, got %d", stats.SurvivorCount)
	}
}

func TestFunnelService_PreservesBothPrices(t *testing.T) {
	funnel := NewFunnelService(nil, nil, nil, nil)
	thresholds := domain.DefaultPipelineThresholds()
	thresholds.MinMarginPct = 0

	products := []FunnelInput{
		{ASIN: "B001", Title: "Test", EstimatedPrice: 49.99, WholesaleCost: 15.0, Source: domain.ScanTypePriceList},
	}

	survivors, _, err := funnel.ProcessBatch(context.Background(), "tenant1", products, thresholds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(survivors) != 1 {
		t.Fatalf("expected 1 survivor, got %d", len(survivors))
	}

	s := survivors[0]
	if s.EstimatedPrice != 49.99 {
		t.Errorf("expected estimated_price 49.99, got %f", s.EstimatedPrice)
	}
	if s.EstimatedMarginPct <= 0 {
		t.Errorf("expected positive estimated_margin_pct, got %f", s.EstimatedMarginPct)
	}
	if s.WholesaleCost != 15.0 {
		t.Errorf("expected wholesale_cost 15.0, got %f", s.WholesaleCost)
	}
}
