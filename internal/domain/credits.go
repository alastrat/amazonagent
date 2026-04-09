package domain

import "time"

type CreditTier string

const (
	CreditTierFree    CreditTier = "free"
	CreditTierStarter CreditTier = "starter"
	CreditTierGrowth  CreditTier = "growth"
	CreditTierScale   CreditTier = "scale"
)

// MonthlyCredits returns the credit allocation for a tier.
func (t CreditTier) MonthlyCredits() int {
	switch t {
	case CreditTierFree:
		return 500
	case CreditTierStarter:
		return 5000
	case CreditTierGrowth:
		return 25000
	case CreditTierScale:
		return 100000
	default:
		return 500
	}
}

// CreditAccount tracks a tenant's credit balance and tier.
type CreditAccount struct {
	TenantID      TenantID   `json:"tenant_id"`
	Tier          CreditTier `json:"tier"`
	MonthlyLimit  int        `json:"monthly_limit"`
	UsedThisMonth int        `json:"used_this_month"`
	ResetAt       time.Time  `json:"reset_at"`
}

// Remaining returns the number of credits left this month.
func (a *CreditAccount) Remaining() int {
	r := a.MonthlyLimit - a.UsedThisMonth
	if r < 0 {
		return 0
	}
	return r
}

// CreditAction describes what consumed the credit.
type CreditAction string

const (
	CreditActionEligibilityCheck CreditAction = "eligibility_check"
	CreditActionEnrichment       CreditAction = "enrichment"
	CreditActionAssessment       CreditAction = "assessment"
	CreditActionDiscoveryScan    CreditAction = "discovery_scan"
	CreditActionMonthlyGrant     CreditAction = "monthly_grant"
)

// CreditTransaction is an immutable ledger entry for credit usage.
type CreditTransaction struct {
	ID        string       `json:"id"`
	TenantID  TenantID     `json:"tenant_id"`
	Amount    int          `json:"amount"` // negative = spent, positive = granted
	Action    CreditAction `json:"action"`
	Reference string       `json:"reference"` // ASIN, scan job ID, etc.
	CreatedAt time.Time    `json:"created_at"`
}
