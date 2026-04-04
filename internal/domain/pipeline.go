package domain

import "time"

type PipelineConfigID string

type PipelineConfig struct {
	ID         PipelineConfigID       `json:"id"`
	TenantID   TenantID               `json:"tenant_id"`
	Name       string                 `json:"name"`
	Agents     map[string]AgentConfig `json:"agents"`
	Scoring    ScoringWeights         `json:"scoring"`
	Thresholds PipelineThresholds     `json:"thresholds"`
	CreatedBy  string                 `json:"created_by"`
	Active     bool                   `json:"active"`
	CreatedAt  time.Time              `json:"created_at"`
}

type AgentConfig struct {
	Version      int            `json:"version"`
	SystemPrompt string         `json:"system_prompt"`
	Tools        []string       `json:"tools"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	ModelTier    string         `json:"model_tier"`
}

type PipelineThresholds struct {
	MinMarginPct         float64 `json:"min_margin_pct"`
	MinSellerCount       int     `json:"min_seller_count"`
	RiskMaxScore         int     `json:"risk_max_score"`
	TierA                float64 `json:"tier_a"`
	TierB                float64 `json:"tier_b"`
	MaxRewriteIterations int     `json:"max_rewrite_iterations"`
	RewriteMinDelta      float64 `json:"rewrite_min_delta"`
}

type DealTier string

const (
	DealTierA   DealTier = "A"
	DealTierB   DealTier = "B"
	DealTierC   DealTier = "C"
	DealTierCut DealTier = "cut"
)

func DefaultPipelineThresholds() PipelineThresholds {
	return PipelineThresholds{
		MinMarginPct:         15.0,
		MinSellerCount:       3,
		RiskMaxScore:         7,
		TierA:                8.0,
		TierB:                6.5,
		MaxRewriteIterations: 2,
		RewriteMinDelta:      0.05,
	}
}

func DefaultPipelineConfig(tenantID TenantID) PipelineConfig {
	return PipelineConfig{
		TenantID: tenantID,
		Name:     "default",
		Agents: map[string]AgentConfig{
			"sourcing":      {Version: 1, SystemPrompt: "You are a product sourcing agent. Find candidate ASINs matching the given criteria using ceiling/floor logic.", ModelTier: "fast"},
			"gating":        {Version: 1, SystemPrompt: "You are a gating and risk assessment agent. Evaluate whether products can be sold: check category gating, IP risk, brand restrictions, hazmat status.", ModelTier: "fast"},
			"profitability": {Version: 1, SystemPrompt: "You are a profitability analysis agent. Given pre-calculated FBA fees, evaluate margin, ROI, and cash flow viability.", ModelTier: "standard"},
			"demand":        {Version: 1, SystemPrompt: "You are a demand and competition analysis agent. Evaluate sales velocity, BSR trends, seller landscape, buy box dynamics, and social sentiment.", ModelTier: "standard"},
			"supplier":      {Version: 1, SystemPrompt: "You are a supplier discovery agent. Find and evaluate wholesale suppliers, compare pricing and terms, draft outreach templates.", ModelTier: "standard"},
			"reviewer":      {Version: 1, SystemPrompt: "You are the pipeline reviewer. Score each candidate on Opportunity Viability, Execution Confidence, and Sourcing Feasibility (1-10 each). Provide reasoning.", ModelTier: "premium"},
		},
		Scoring:    DefaultScoringWeights(),
		Thresholds: DefaultPipelineThresholds(),
		CreatedBy:  "system",
		Active:     true,
	}
}
