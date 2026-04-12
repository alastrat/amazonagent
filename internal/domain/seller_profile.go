package domain

import "time"

type SellerArchetype string

const (
	SellerArchetypeGreenhorn      SellerArchetype = "greenhorn"       // New account, <90 days
	SellerArchetypeRAToWholesale  SellerArchetype = "ra_to_wholesale" // 6-12 mo RA experience, transitioning
	SellerArchetypeExpandingPro   SellerArchetype = "expanding_pro"   // 1yr+, $10-50K/mo, wants more brands
	SellerArchetypeCapitalRich    SellerArchetype = "capital_rich"    // New to Amazon, has $50K+ to deploy
)

type AssessmentStatus string

const (
	AssessmentStatusPending    AssessmentStatus = "pending"
	AssessmentStatusRunning    AssessmentStatus = "running"
	AssessmentStatusCompleted  AssessmentStatus = "completed"
	AssessmentStatusFailed     AssessmentStatus = "failed"
)

// SellerProfile captures a tenant's assessed situation on the platform.
type SellerProfile struct {
	ID               string           `json:"id"`
	TenantID         TenantID         `json:"tenant_id"`
	Archetype        SellerArchetype  `json:"archetype"`
	AccountAgeDays   int              `json:"account_age_days"`
	ActiveListings   int              `json:"active_listings"`
	StatedCapital    float64          `json:"stated_capital,omitempty"`
	AssessmentStatus AssessmentStatus `json:"assessment_status"`
	AssessedAt       *time.Time       `json:"assessed_at,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// ClassifyArchetype determines the seller archetype from profile data.
// Decision tree:
//   account_age < 90 days AND active_listings < 10 → greenhorn
//   account_age < 90 days AND stated_capital >= 50000 → capital_rich
//   account_age 90-365 days AND active_listings >= 10 → ra_to_wholesale
//   account_age > 365 days OR active_listings >= 50 → expanding_pro
//   default → greenhorn
func ClassifyArchetype(accountAgeDays int, activeListings int, statedCapital float64) SellerArchetype {
	if accountAgeDays < 90 {
		if statedCapital >= 50000 {
			return SellerArchetypeCapitalRich
		}
		return SellerArchetypeGreenhorn
	}
	if accountAgeDays <= 365 && activeListings >= 10 {
		return SellerArchetypeRAToWholesale
	}
	if accountAgeDays > 365 || activeListings >= 50 {
		return SellerArchetypeExpandingPro
	}
	return SellerArchetypeGreenhorn
}

// EligibilityFingerprint is the result of the 300-ASIN assessment scan.
// Maps a seller's eligibility surface across categories and brands.
type EligibilityFingerprint struct {
	ID               string               `json:"id"`
	TenantID         TenantID             `json:"tenant_id"`
	Categories       []CategoryEligibility `json:"categories"`
	BrandResults     []BrandProbeResult    `json:"brand_results"`
	TotalProbes      int                  `json:"total_probes"`
	TotalEligible    int                  `json:"total_eligible"`
	TotalRestricted  int                  `json:"total_restricted"`
	OverallOpenRate  float64              `json:"overall_open_rate"`
	Confidence       float64              `json:"confidence"` // 0-1 based on probe coverage
	AssessedAt       time.Time            `json:"assessed_at"`
}

// CategoryEligibility summarizes eligibility within a single Amazon category.
type CategoryEligibility struct {
	Category    string  `json:"category"`
	ProbeCount  int     `json:"probe_count"`
	OpenCount   int     `json:"open_count"`
	GatedCount  int     `json:"gated_count"`
	OpenRate    float64 `json:"open_rate"` // 0-100%
}

// BrandProbeResult records the eligibility check result for a specific brand ASIN.
type BrandProbeResult struct {
	ASIN              string  `json:"asin"`
	Brand             string  `json:"brand"`
	Category          string  `json:"category"`
	Subcategory       string  `json:"subcategory"`
	Tier              string  `json:"tier"` // top, mid, generic, calibration
	Eligible          bool    `json:"eligible"`
	EligibilityStatus string  `json:"eligibility_status"` // eligible, ungatable, restricted
	Reason            string  `json:"reason,omitempty"`
	ApprovalURL       string  `json:"approval_url,omitempty"`
	Title             string  `json:"title"`
	Price             float64 `json:"price"`
	EstMarginPct      float64 `json:"est_margin_pct"`
	SellerCount       int     `json:"seller_count"`
}

// AssessmentProbe defines a single ASIN to check during the assessment scan.
// These are curated and ship with the code as a static dataset.
type AssessmentProbe struct {
	ASIN           string `json:"asin"`
	Category       string `json:"category"`
	Brand          string `json:"brand"`
	Tier           string `json:"tier"` // top, mid, generic, calibration
	ExpectedGating string `json:"expected_gating"` // open, brand_gated, category_gated
}

// ---------------------------------------------------------------------------
// Discovery Assessment types (Phase B — broad category search)
// ---------------------------------------------------------------------------

// AssessmentSearchResult records a per-ASIN result during the discovery assessment.
type AssessmentSearchResult struct {
	ASIN              string  `json:"asin"`
	Title             string  `json:"title"`
	Brand             string  `json:"brand"`
	Category          string  `json:"category"`
	Subcategory       string  `json:"subcategory"`
	AmazonPrice       float64 `json:"amazon_price"`
	BSRRank           int     `json:"bsr_rank"`
	SellerCount       int     `json:"seller_count"`
	Eligible          bool    `json:"eligible"`
	EligibilityStatus string  `json:"eligibility_status"` // eligible, ungatable, restricted
	RestrictionReason string  `json:"restriction_reason,omitempty"`
	ApprovalURL       string  `json:"approval_url,omitempty"`
}

// AssessmentOutcome wraps either an OpportunityResult (products found) or
// an UngatingResult (nothing found, seller needs ungating).
type AssessmentOutcome struct {
	HasOpportunities bool               `json:"has_opportunities"`
	Opportunity      *OpportunityResult `json:"opportunity,omitempty"`
	Ungating         *UngatingResult    `json:"ungating,omitempty"`

	// Metadata about the assessment run
	TotalSearched     int     `json:"total_searched"`
	TotalEligible     int     `json:"total_eligible"`
	TotalUngatable    int     `json:"total_ungatable"`
	TotalRestricted   int     `json:"total_restricted"`
	TotalQualified    int     `json:"total_qualified"`
	APICallsUsed      int     `json:"api_calls_used"`
	DurationSeconds   float64 `json:"duration_seconds"`
	CircuitBreakers   []string `json:"circuit_breakers,omitempty"` // which breakers fired
}

// OpportunityResult is returned when the assessment finds sellable products.
type OpportunityResult struct {
	QualifiedProducts  []ProductRecommendation `json:"qualified_products"`
	EligibleCategories []CategorySummary       `json:"eligible_categories"`
	OpenBrands         []string                `json:"open_brands"`
	TopRecommendations []ProductRecommendation `json:"top_recommendations"` // top 10 by margin
	EstimatedMonthlyRev float64               `json:"estimated_monthly_rev"`
}

// UngatingResult is returned when the assessment finds no sellable products.
type UngatingResult struct {
	RestrictedCategories []RestrictedCategory `json:"restricted_categories"`
	RecommendedPath      []UngatingStep       `json:"recommended_path"`
	EstimatedTimeline    string               `json:"estimated_timeline"`
}

// CategorySummary summarises assessment results for one category.
type CategorySummary struct {
	Category       string  `json:"category"`
	BrowseNodeID   string  `json:"browse_node_id"`
	EligibleCount  int     `json:"eligible_count"`
	QualifiedCount int     `json:"qualified_count"`
	AvgMarginPct   float64 `json:"avg_margin_pct"`
	OpenRate       float64 `json:"open_rate"`
}

// ProductRecommendation is a single product recommendation from the assessment.
type ProductRecommendation struct {
	ASIN         string  `json:"asin"`
	Title        string  `json:"title"`
	Brand        string  `json:"brand"`
	Category     string  `json:"category"`
	BuyBoxPrice  float64 `json:"buy_box_price"`
	EstMarginPct float64 `json:"est_margin_pct"`
	SellerCount  int     `json:"seller_count"`
	BSRRank      int     `json:"bsr_rank"`
}

// RestrictedCategory records gating info for the ungating roadmap.
type RestrictedCategory struct {
	Category   string  `json:"category"`
	OpenRate   float64 `json:"open_rate"`
	Difficulty string  `json:"difficulty"` // easy | medium | hard
}

// UngatingStep is one step in the ungating roadmap.
type UngatingStep struct {
	Order      int    `json:"order"`
	Category   string `json:"category"`
	Action     string `json:"action"`
	Difficulty string `json:"difficulty"`
	EstDays    int    `json:"est_days"`
	Impact     string `json:"impact"`
}
