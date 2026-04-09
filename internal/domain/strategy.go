package domain

import "time"

type StrategyVersionID string

type StrategyStatus string

const (
	StrategyStatusDraft      StrategyStatus = "draft"
	StrategyStatusActive     StrategyStatus = "active"
	StrategyStatusRolledBack StrategyStatus = "rolled_back"
	StrategyStatusArchived   StrategyStatus = "archived"
)

type StrategyCreatedBy string

const (
	StrategyCreatedByUser         StrategyCreatedBy = "user"
	StrategyCreatedBySystem       StrategyCreatedBy = "system"
	StrategyCreatedByAutoresearch StrategyCreatedBy = "autoresearch"
)

// StrategyVersion is an immutable snapshot of a seller's growth strategy.
// Every change creates a new version. Rollback creates a copy of an old version.
type StrategyVersion struct {
	ID                      StrategyVersionID `json:"id"`
	TenantID                TenantID          `json:"tenant_id"`
	VersionNumber           int               `json:"version_number"`
	Goals                   []StrategyGoal    `json:"goals"`
	SearchParams            StrategySearchParams `json:"search_params"`
	ScoringConfigID         ScoringConfigID   `json:"scoring_config_id,omitempty"`
	Status                  StrategyStatus    `json:"status"`
	ParentVersionID         StrategyVersionID `json:"parent_version_id,omitempty"`
	PromotedFromExperimentID string           `json:"promoted_from_experiment_id,omitempty"`
	ChangeReason            string            `json:"change_reason"`
	CreatedBy               StrategyCreatedBy `json:"created_by"`
	CreatedAt               time.Time         `json:"created_at"`
	ActivatedAt             *time.Time        `json:"activated_at,omitempty"`
	RolledBackAt            *time.Time        `json:"rolled_back_at,omitempty"`
}

// StrategyGoal is a measurable, time-bound objective.
// Goals are revenue/profit targets ONLY — not tactical ("list 10 products").
type StrategyGoal struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"` // "revenue" or "profit"
	TargetAmount     float64   `json:"target_amount"`
	Currency         string    `json:"currency"` // "USD"
	TimeframeStart   time.Time `json:"timeframe_start"`
	TimeframeEnd     time.Time `json:"timeframe_end"`
	TargetCategories []string  `json:"target_categories,omitempty"`
	CurrentProgress  float64   `json:"current_progress"`
}

// DaysRemaining returns the number of days until the goal's timeframe ends.
func (g *StrategyGoal) DaysRemaining() int {
	d := int(time.Until(g.TimeframeEnd).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

// ProgressPct returns goal completion as a percentage (0-100).
func (g *StrategyGoal) ProgressPct() float64 {
	if g.TargetAmount <= 0 {
		return 0
	}
	pct := (g.CurrentProgress / g.TargetAmount) * 100
	if pct > 100 {
		return 100
	}
	return pct
}

// StrategySearchParams defines per-goal discovery parameters.
type StrategySearchParams struct {
	MinMarginPct       float64  `json:"min_margin_pct"`
	MinSellerCount     int      `json:"min_seller_count"`
	EligibleCategories []string `json:"eligible_categories,omitempty"`
	EligibleBrands     []string `json:"eligible_brands,omitempty"`
	ScoringWeights     ScoringWeights `json:"scoring_weights"`
}

// CategoryRecommendation is a strategy-generated category suggestion.
type CategoryRecommendation struct {
	Category               string  `json:"category"`
	PriorityScore          float64 `json:"priority_score"`
	Rationale              string  `json:"rationale"`
	EstimatedMonthlyRevenue float64 `json:"estimated_monthly_revenue"`
	UngatingRequired       bool    `json:"ungating_required"`
}

// UngatingRecommendation suggests a brand/category to unlock.
type UngatingRecommendation struct {
	BrandOrCategory    string   `json:"brand_or_category"`
	Difficulty         int      `json:"difficulty"` // 1-4
	EstimatedUnlockValue float64 `json:"estimated_unlock_value"`
	SuggestedDistributor string `json:"suggested_distributor,omitempty"`
	ActionSteps        []string `json:"action_steps"`
}
