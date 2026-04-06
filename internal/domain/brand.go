package domain

import "time"

type BrandID string

type EligibilityStatus string

const (
	EligibilityUnknown    EligibilityStatus = "unknown"
	EligibilityEligible   EligibilityStatus = "eligible"
	EligibilityRestricted EligibilityStatus = "restricted"
)

type Brand struct {
	ID             BrandID   `json:"id"`
	Name           string    `json:"name"`
	NormalizedName string    `json:"normalized_name"`
	CreatedAt      time.Time `json:"created_at"`
}

type BrandEligibility struct {
	ID         string            `json:"id"`
	TenantID   TenantID          `json:"tenant_id"`
	BrandID    BrandID           `json:"brand_id"`
	Status     EligibilityStatus `json:"status"`
	Reason     string            `json:"reason"`
	SampleASIN string            `json:"sample_asin"`
	CheckedAt  time.Time         `json:"checked_at"`
}
