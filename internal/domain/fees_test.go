package domain

import (
	"math"
	"testing"
)

func TestCalculateFBAFees_StandardProduct(t *testing.T) {
	result := CalculateFBAFees(25.00, 10.00, 0.8, false)
	if result.ReferralFeePct != 15.0 {
		t.Errorf("expected 15%% referral, got %.1f%%", result.ReferralFeePct)
	}
	if math.Abs(result.ReferralFee-3.75) > 0.01 {
		t.Errorf("expected referral fee ~3.75, got %.2f", result.ReferralFee)
	}
	if result.FBAFulfillment != 3.22 {
		t.Errorf("expected FBA fee 3.22, got %.2f", result.FBAFulfillment)
	}
	if result.NetProfit <= 0 {
		t.Error("expected positive net profit")
	}
	if result.NetMarginPct <= 0 {
		t.Error("expected positive margin")
	}
}

func TestCalculateFBAFees_HeavyProduct(t *testing.T) {
	result := CalculateFBAFees(50.00, 20.00, 3.5, false)
	expected := 4.75 + (3.5-1.0)*0.40
	if math.Abs(result.FBAFulfillment-expected) > 0.01 {
		t.Errorf("expected FBA fee ~%.2f, got %.2f", expected, result.FBAFulfillment)
	}
}

func TestCalculateFBAFees_OversizedProduct(t *testing.T) {
	result := CalculateFBAFees(80.00, 30.00, 5.0, true)
	expected := 9.73 + (5.0-2.0)*0.42
	if math.Abs(result.FBAFulfillment-expected) > 0.01 {
		t.Errorf("expected FBA fee ~%.2f, got %.2f", expected, result.FBAFulfillment)
	}
}

func TestCalculateFBAFees_NegativeMargin(t *testing.T) {
	result := CalculateFBAFees(12.00, 10.00, 0.5, false)
	if result.NetMarginPct > 20 {
		t.Errorf("expected low margin for thin spread, got %.1f%%", result.NetMarginPct)
	}
}
