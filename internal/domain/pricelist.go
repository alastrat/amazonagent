package domain

import "time"

type PriceListItem struct {
	UPC           string  `json:"upc"`
	EAN           string  `json:"ean,omitempty"`
	SKU           string  `json:"sku,omitempty"`
	ProductName   string  `json:"product_name"`
	WholesaleCost float64 `json:"wholesale_cost"`
	MSRP          float64 `json:"msrp,omitempty"`
	CasePack      int     `json:"case_pack,omitempty"`
	MinOrderQty   int     `json:"min_order_qty,omitempty"`
	Brand         string  `json:"brand,omitempty"`
	Category      string  `json:"category,omitempty"`
}

type PriceListMatch struct {
	PriceListItem
	ASIN           string            `json:"asin"`
	AmazonTitle    string            `json:"amazon_title"`
	AmazonPrice    float64           `json:"amazon_price"`
	BSRRank        int               `json:"bsr_rank"`
	SellerCount    int               `json:"seller_count"`
	FBACalculation FBAFeeCalculation `json:"fba_calculation"`
	RealMarginPct  float64           `json:"real_margin_pct"`
	RealProfit     float64           `json:"real_profit"`
	RealROIPct     float64           `json:"real_roi_pct"`
	Eligible       bool              `json:"eligible"`
	EligibleReason string            `json:"eligible_reason,omitempty"`
	MatchStatus    string            `json:"match_status"`
}

type PriceListUpload struct {
	ID              string     `json:"id"`
	TenantID        TenantID   `json:"tenant_id"`
	CampaignID      CampaignID `json:"campaign_id"`
	DistributorName string     `json:"distributor_name"`
	FileName        string     `json:"file_name"`
	TotalItems      int        `json:"total_items"`
	Matched         int        `json:"matched"`
	Eligible        int        `json:"eligible"`
	Profitable      int        `json:"profitable"`
	Status          string     `json:"status"`
	CreatedAt       time.Time  `json:"created_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}
