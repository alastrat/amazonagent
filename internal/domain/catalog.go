package domain

import "time"

// ScanType distinguishes how products enter the pipeline.
type ScanType string

const (
	ScanTypePriceList   ScanType = "pricelist"
	ScanTypeCategory    ScanType = "category"
	ScanTypeKeyword     ScanType = "keyword"
	ScanTypeAssessment  ScanType = "assessment"
)

// DataQuality bitmask flags track which data points are populated.
const (
	DataQualityPrice       = 1
	DataQualityBSR         = 2
	DataQualityFees        = 4
	DataQualityEligibility = 8
	DataQualityBuyBox      = 16
)

// DiscoveredProduct is a persistent catalog entry that accumulates data across scans.
type DiscoveredProduct struct {
	ID                 string     `json:"id"`
	TenantID           TenantID   `json:"tenant_id"`
	ASIN               string     `json:"asin"`
	Title              string     `json:"title"`
	BrandID            string     `json:"brand_id,omitempty"`
	Category           string     `json:"category"`
	BrowseNodeID       string     `json:"browse_node_id,omitempty"`
	EstimatedPrice     float64    `json:"estimated_price"`
	BuyBoxPrice        float64    `json:"buy_box_price"`
	BSRRank            int        `json:"bsr_rank"`
	SellerCount        int        `json:"seller_count"`
	EstimatedMarginPct float64    `json:"estimated_margin_pct"`
	RealMarginPct      float64    `json:"real_margin_pct"`
	EligibilityStatus  string     `json:"eligibility_status"`
	DataQuality        int        `json:"data_quality"`
	RefreshPriority    float64    `json:"refresh_priority"`
	Source             ScanType   `json:"source"`
	FirstSeenAt        time.Time  `json:"first_seen_at"`
	LastSeenAt         time.Time  `json:"last_seen_at"`
	PriceUpdatedAt     *time.Time `json:"price_updated_at,omitempty"`
}

// PriceSnapshot tracks price/rank changes over time for BI analysis.
type PriceSnapshot struct {
	ASIN        string    `json:"asin"`
	TenantID    TenantID  `json:"tenant_id"`
	RecordedAt  time.Time `json:"recorded_at"`
	AmazonPrice float64   `json:"amazon_price"`
	BSRRank     int       `json:"bsr_rank"`
	SellerCount int       `json:"seller_count"`
}

type ScanJobID string

// ScanJob tracks a background scan's progress.
type ScanJob struct {
	ID          ScanJobID      `json:"id"`
	TenantID    TenantID       `json:"tenant_id"`
	Type        ScanType       `json:"type"`
	Status      string         `json:"status"`
	TotalItems  int            `json:"total_items"`
	Processed   int            `json:"processed"`
	Qualified   int            `json:"qualified"`
	Eliminated  int            `json:"eliminated"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// BrandIntelligence is the pre-computed brand-level view from the materialized view.
type BrandIntelligence struct {
	TenantID        TenantID `json:"tenant_id"`
	BrandID         string   `json:"brand_id"`
	BrandName       string   `json:"brand_name"`
	Category        string   `json:"category"`
	ProductCount    int      `json:"product_count"`
	HighMarginCount int      `json:"high_margin_count"`
	AvgMargin       float64  `json:"avg_margin"`
	AvgSellers      float64  `json:"avg_sellers"`
	AvgBSR          float64  `json:"avg_bsr"`
}
