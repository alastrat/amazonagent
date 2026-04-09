package domain

import "testing"

func TestClassifyArchetype(t *testing.T) {
	tests := []struct {
		name           string
		accountAgeDays int
		activeListings int
		statedCapital  float64
		want           SellerArchetype
	}{
		{
			name:           "greenhorn — new account, few listings, low capital",
			accountAgeDays: 30,
			activeListings: 3,
			statedCapital:  5000,
			want:           SellerArchetypeGreenhorn,
		},
		{
			name:           "greenhorn — zero day account",
			accountAgeDays: 0,
			activeListings: 0,
			statedCapital:  0,
			want:           SellerArchetypeGreenhorn,
		},
		{
			name:           "capital_rich — new account but 50K+ capital",
			accountAgeDays: 30,
			activeListings: 2,
			statedCapital:  50000,
			want:           SellerArchetypeCapitalRich,
		},
		{
			name:           "capital_rich — exactly 50K boundary",
			accountAgeDays: 89,
			activeListings: 0,
			statedCapital:  50000,
			want:           SellerArchetypeCapitalRich,
		},
		{
			name:           "capital_rich takes priority over greenhorn when capital >= 50K",
			accountAgeDays: 10,
			activeListings: 5,
			statedCapital:  100000,
			want:           SellerArchetypeCapitalRich,
		},
		{
			name:           "ra_to_wholesale — 90-365 days, 10+ listings",
			accountAgeDays: 180,
			activeListings: 15,
			statedCapital:  10000,
			want:           SellerArchetypeRAToWholesale,
		},
		{
			name:           "ra_to_wholesale — exactly 90 days, exactly 10 listings",
			accountAgeDays: 90,
			activeListings: 10,
			statedCapital:  0,
			want:           SellerArchetypeRAToWholesale,
		},
		{
			name:           "ra_to_wholesale — exactly 365 days",
			accountAgeDays: 365,
			activeListings: 10,
			statedCapital:  0,
			want:           SellerArchetypeRAToWholesale,
		},
		{
			name:           "expanding_pro — over 365 days",
			accountAgeDays: 400,
			activeListings: 20,
			statedCapital:  30000,
			want:           SellerArchetypeExpandingPro,
		},
		{
			name:           "ra_to_wholesale takes priority over expanding_pro when 90-365 days and 50+ listings",
			accountAgeDays: 200,
			activeListings: 50,
			statedCapital:  5000,
			want:           SellerArchetypeRAToWholesale,
		},
		{
			name:           "expanding_pro — exactly 366 days, few listings",
			accountAgeDays: 366,
			activeListings: 1,
			statedCapital:  0,
			want:           SellerArchetypeExpandingPro,
		},
		{
			name:           "default greenhorn — 90-365 days but few listings",
			accountAgeDays: 200,
			activeListings: 5,
			statedCapital:  10000,
			want:           SellerArchetypeGreenhorn,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyArchetype(tt.accountAgeDays, tt.activeListings, tt.statedCapital)
			if got != tt.want {
				t.Errorf("ClassifyArchetype(%d, %d, %.0f) = %q, want %q",
					tt.accountAgeDays, tt.activeListings, tt.statedCapital, got, tt.want)
			}
		})
	}
}
