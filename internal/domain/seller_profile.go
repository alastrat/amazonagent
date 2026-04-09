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
	ASIN     string `json:"asin"`
	Brand    string `json:"brand"`
	Category string `json:"category"`
	Tier     string `json:"tier"` // top, mid, generic, calibration
	Eligible bool   `json:"eligible"`
	Reason   string `json:"reason,omitempty"`
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
