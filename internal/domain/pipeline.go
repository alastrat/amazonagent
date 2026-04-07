package domain

import (
	"strings"
	"time"
)

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
	MinMarginPct         float64     `json:"min_margin_pct"`
	MinSellerCount       int         `json:"min_seller_count"`
	RiskMaxScore         int         `json:"risk_max_score"`
	TierA                float64     `json:"tier_a"`
	TierB                float64     `json:"tier_b"`
	MaxRewriteIterations int         `json:"max_rewrite_iterations"`
	RewriteMinDelta      float64     `json:"rewrite_min_delta"`
	BrandFilter          BrandFilter `json:"brand_filter"`
}

// BrandFilter controls which brands the pipeline accepts or rejects.
// If AllowList is non-empty, ONLY those brands pass (whitelist mode).
// If BlockList is non-empty, those brands are rejected (blacklist mode).
// If both are empty, all brands pass.
type BrandFilter struct {
	AllowList []string `json:"allow_list,omitempty"`
	BlockList []string `json:"block_list,omitempty"`
}

// IsBrandAllowed checks if a brand passes the filter.
func (f BrandFilter) IsBrandAllowed(brand string) bool {
	if brand == "" {
		return true // can't filter without brand info
	}
	lower := strings.ToLower(brand)

	// Allowlist mode: only listed brands pass
	if len(f.AllowList) > 0 {
		for _, allowed := range f.AllowList {
			if strings.ToLower(allowed) == lower {
				return true
			}
		}
		return false
	}

	// Blocklist mode: listed brands are rejected
	for _, blocked := range f.BlockList {
		if strings.ToLower(blocked) == lower {
			return false
		}
	}
	return true
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
		MinMarginPct:         10.0,
		MinSellerCount:       1,
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
