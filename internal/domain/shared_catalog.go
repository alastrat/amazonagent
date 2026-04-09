package domain

import "time"

// SharedProduct is a platform-wide product entry enriched by all tenants' scans.
// Product data (ASIN, title, BSR, price) is shared. Tenant-specific data (eligibility,
// margins) lives in separate per-tenant tables.
type SharedProduct struct {
	ASIN             string     `json:"asin"`
	Title            string     `json:"title"`
	Brand            string     `json:"brand"`
	Category         string     `json:"category"`
	BSRRank          int        `json:"bsr_rank"`
	SellerCount      int        `json:"seller_count"`
	BuyBoxPrice      float64    `json:"buy_box_price"`
	EstimatedMargin  float64    `json:"estimated_margin_pct"`
	ImageURL         string     `json:"image_url,omitempty"`
	LastEnrichedAt   *time.Time `json:"last_enriched_at,omitempty"`
	EnrichmentCount  int        `json:"enrichment_count"`
	CreatedAt        time.Time  `json:"created_at"`
}

// IsFresh returns true if the product was enriched within the given duration.
func (p *SharedProduct) IsFresh(maxAge time.Duration) bool {
	if p.LastEnrichedAt == nil {
		return false
	}
	return time.Since(*p.LastEnrichedAt) < maxAge
}

// SharedBrand is a platform-wide brand entry with gating metadata.
type SharedBrand struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	NormalizedName string    `json:"normalized_name"`
	TypicalGating  string    `json:"typical_gating"` // open, brand_gated, category_gated, unknown
	Categories     []string  `json:"categories"`
	ProductCount   int       `json:"product_count"`
	CreatedAt      time.Time `json:"created_at"`
}

// TenantEligibility is a per-tenant, per-ASIN eligibility record (private).
type TenantEligibility struct {
	TenantID  TenantID  `json:"tenant_id"`
	ASIN      string    `json:"asin"`
	Eligible  bool      `json:"eligible"`
	Reason    string    `json:"reason"`
	CheckedAt time.Time `json:"checked_at"`
}

// TenantMargin is a per-tenant, per-ASIN margin record from price lists (private).
type TenantMargin struct {
	TenantID      TenantID  `json:"tenant_id"`
	ASIN          string    `json:"asin"`
	WholesaleCost float64   `json:"wholesale_cost"`
	RealMarginPct float64   `json:"real_margin_pct"`
	Source        string    `json:"source"` // pricelist, manual
	UpdatedAt     time.Time `json:"updated_at"`
}
