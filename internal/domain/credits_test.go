package domain

import (
	"testing"
	"time"
)

func TestCreditTier_MonthlyCredits(t *testing.T) {
	tests := []struct {
		tier CreditTier
		want int
	}{
		{CreditTierFree, 500},
		{CreditTierStarter, 5000},
		{CreditTierGrowth, 25000},
		{CreditTierScale, 100000},
	}
	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			if got := tt.tier.MonthlyCredits(); got != tt.want {
				t.Errorf("CreditTier(%q).MonthlyCredits() = %d, want %d", tt.tier, got, tt.want)
			}
		})
	}
}

func TestCreditTier_MonthlyCredits_UnknownTier(t *testing.T) {
	unknown := CreditTier("enterprise")
	if got := unknown.MonthlyCredits(); got != 500 {
		t.Errorf("CreditTier(%q).MonthlyCredits() = %d, want 500", unknown, got)
	}
}

func TestCreditAccount_Remaining(t *testing.T) {
	tests := []struct {
		name  string
		limit int
		used  int
		want  int
	}{
		{"used less than limit", 1000, 300, 700},
		{"used equals limit", 1000, 1000, 0},
		{"used exceeds limit", 1000, 1500, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &CreditAccount{
				MonthlyLimit:  tt.limit,
				UsedThisMonth: tt.used,
			}
			if got := a.Remaining(); got != tt.want {
				t.Errorf("Remaining() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSharedProduct_IsFresh_NilLastEnrichedAt(t *testing.T) {
	p := &SharedProduct{LastEnrichedAt: nil}
	if p.IsFresh(24 * time.Hour) {
		t.Error("IsFresh() = true for nil LastEnrichedAt, want false")
	}
}

func TestSharedProduct_IsFresh_WithinWindow(t *testing.T) {
	recent := time.Now().Add(-1 * time.Hour)
	p := &SharedProduct{LastEnrichedAt: &recent}
	if !p.IsFresh(24 * time.Hour) {
		t.Error("IsFresh() = false for product enriched 1h ago with 24h window, want true")
	}
}

func TestSharedProduct_IsFresh_OutsideWindow(t *testing.T) {
	old := time.Now().Add(-48 * time.Hour)
	p := &SharedProduct{LastEnrichedAt: &old}
	if p.IsFresh(24 * time.Hour) {
		t.Error("IsFresh() = true for product enriched 48h ago with 24h window, want false")
	}
}
