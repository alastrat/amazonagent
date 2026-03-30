package spapi

import (
	"context"
	"testing"
)

func TestClient_SearchProducts_MockFallback(t *testing.T) {
	client := NewClient("", "", "", "US")
	if client.IsConfigured() {
		t.Error("expected unconfigured client")
	}

	products, err := client.SearchProducts(context.Background(), []string{"kitchen gadgets"}, "US")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(products) == 0 {
		t.Error("expected mock products")
	}
	for _, p := range products {
		if p.ASIN == "" {
			t.Error("product missing ASIN")
		}
		if p.AmazonPrice <= 0 {
			t.Error("product missing price")
		}
	}
}

func TestClient_EstimateFees_MockFallback(t *testing.T) {
	client := NewClient("", "", "", "US")
	fees, err := client.EstimateFees(context.Background(), "B0TEST001", 25.00, "US")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fees.ReferralFee <= 0 {
		t.Error("expected positive referral fee")
	}
	if fees.TotalFees <= 0 {
		t.Error("expected positive total fees")
	}
}

func TestMarketplaceID(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"US", "ATVPDKIKX0DER"},
		{"UK", "A1F83G8C2ARO7P"},
		{"EU", "A1PA6795UKMFR9"},
		{"", "ATVPDKIKX0DER"},
	}
	for _, tt := range tests {
		if got := marketplaceID(tt.input); got != tt.expected {
			t.Errorf("marketplaceID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
