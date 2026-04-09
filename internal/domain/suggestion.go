package domain

import "time"

type SuggestionID string

type SuggestionStatus string

const (
	SuggestionStatusPending   SuggestionStatus = "pending"
	SuggestionStatusAccepted  SuggestionStatus = "accepted"
	SuggestionStatusDismissed SuggestionStatus = "dismissed"
)

// DiscoverySuggestion is a product found by the daily discovery queue
// that matches the seller's active strategy. Requires user action
// (accept → creates deal, dismiss → ignored, no preference bias).
type DiscoverySuggestion struct {
	ID                SuggestionID        `json:"id"`
	TenantID          TenantID            `json:"tenant_id"`
	StrategyVersionID StrategyVersionID   `json:"strategy_version_id"`
	GoalID            string              `json:"goal_id,omitempty"`
	ASIN              string              `json:"asin"`
	Title             string              `json:"title"`
	Brand             string              `json:"brand"`
	Category          string              `json:"category"`
	BuyBoxPrice       float64             `json:"buy_box_price"`
	EstimatedMargin   float64             `json:"estimated_margin_pct"`
	BSRRank           int                 `json:"bsr_rank"`
	SellerCount       int                 `json:"seller_count"`
	Reason            string              `json:"reason"` // why this product matches the strategy
	Status            SuggestionStatus    `json:"status"`
	DealID            *DealID             `json:"deal_id,omitempty"` // set when accepted
	CreatedAt         time.Time           `json:"created_at"`
	ResolvedAt        *time.Time          `json:"resolved_at,omitempty"`
}
