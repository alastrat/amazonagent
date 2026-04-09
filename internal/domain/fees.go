package domain

// EstimatedWholesaleRatio is the default wholesale-to-retail cost ratio used
// when actual cost data is unavailable (e.g., pre-price-list margin estimates).
const EstimatedWholesaleRatio = 0.4

// EstimateMarginPct returns an estimated net margin percentage for a given
// Amazon price, using the default wholesale ratio and standard-size FBA fees.
func EstimateMarginPct(amazonPrice float64) float64 {
	if amazonPrice <= 0 {
		return 0
	}
	calc := CalculateFBAFees(amazonPrice, amazonPrice*EstimatedWholesaleRatio, 1.0, false)
	return calc.NetMarginPct
}

type FBAFeeCalculation struct {
	ReferralFeePct float64 `json:"referral_fee_pct"`
	ReferralFee    float64 `json:"referral_fee"`
	FBAFulfillment float64 `json:"fba_fulfillment"`
	StorageFee     float64 `json:"storage_fee_monthly"`
	TotalFees      float64 `json:"total_fees"`
	NetProfit      float64 `json:"net_profit"`
	NetMarginPct   float64 `json:"net_margin_pct"`
	ROIPct         float64 `json:"roi_pct"`
}

func CalculateFBAFees(amazonPrice, wholesaleCost, weightLbs float64, isOversized bool) FBAFeeCalculation {
	referralPct := 0.15
	referralFee := amazonPrice * referralPct

	fbaFee := 3.22
	if weightLbs > 1.0 {
		fbaFee = 4.75 + (weightLbs-1.0)*0.40
	}
	if isOversized {
		fbaFee = 9.73 + (weightLbs-2.0)*0.42
	}

	storageFee := 0.87

	totalFees := referralFee + fbaFee + storageFee
	landedCost := wholesaleCost * 1.10
	netProfit := amazonPrice - landedCost - totalFees

	netMarginPct := 0.0
	if amazonPrice > 0 {
		netMarginPct = (netProfit / amazonPrice) * 100
	}

	roiPct := 0.0
	if landedCost > 0 {
		roiPct = (netProfit / landedCost) * 100
	}

	return FBAFeeCalculation{
		ReferralFeePct: referralPct * 100,
		ReferralFee:    referralFee,
		FBAFulfillment: fbaFee,
		StorageFee:     storageFee,
		TotalFees:      totalFees,
		NetProfit:      netProfit,
		NetMarginPct:   netMarginPct,
		ROIPct:         roiPct,
	}
}
