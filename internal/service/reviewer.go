package service

import (
	"context"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type ReviewResult struct {
	Tier                 domain.DealTier `json:"tier"`
	OpportunityViability int             `json:"opportunity_viability"`
	ExecutionConfidence  int             `json:"execution_confidence"`
	SourcingFeasibility  int             `json:"sourcing_feasibility"`
	WeightedComposite    float64         `json:"weighted_composite"`
	RuleChecksPassed     bool            `json:"rule_checks_passed"`
	RuleFailures         []string        `json:"rule_failures,omitempty"`
	Reasoning            string          `json:"reasoning"`
}

type Reviewer struct {
	runtime port.AgentRuntime
}

func NewReviewer(runtime port.AgentRuntime) *Reviewer {
	return &Reviewer{runtime: runtime}
}

func (r *Reviewer) Review(
	ctx context.Context,
	candidate map[string]any,
	agentContexts []domain.AgentContext,
	config domain.AgentConfig,
	thresholds domain.PipelineThresholds,
	weights domain.ScoringWeights,
) (*ReviewResult, error) {
	result := &ReviewResult{RuleChecksPassed: true}

	// --- Rule-based checks (deterministic) ---
	var ruleFailures []string

	if marginPct, ok := getFloat(candidate, "net_margin_pct"); ok {
		if marginPct < thresholds.MinMarginPct {
			ruleFailures = append(ruleFailures, "margin below minimum threshold")
		}
	}

	if riskScore, ok := getInt(candidate, "risk_score"); ok {
		if riskScore > thresholds.RiskMaxScore {
			ruleFailures = append(ruleFailures, "risk score exceeds maximum")
		}
	}

	for _, field := range []string{"asin", "title"} {
		if _, ok := candidate[field]; !ok {
			ruleFailures = append(ruleFailures, "missing required field: "+field)
		}
	}

	plausibilityErrs := domain.ValidateAgentOutput("reviewer", candidate)
	for _, pe := range plausibilityErrs {
		ruleFailures = append(ruleFailures, pe.Error())
	}

	if len(ruleFailures) > 0 {
		result.RuleChecksPassed = false
		result.RuleFailures = ruleFailures
		result.Tier = domain.DealTierCut
		result.Reasoning = "Failed rule-based checks"
		return result, nil
	}

	// --- LLM scoring (subjective quality) ---
	task := domain.AgentTask{
		AgentName:    "reviewer",
		SystemPrompt: config.SystemPrompt,
		Input:        candidate,
		Context:      agentContexts,
		OutputSchema: map[string]any{
			"opportunity_viability": "int 1-10",
			"execution_confidence":  "int 1-10",
			"sourcing_feasibility":  "int 1-10",
			"reasoning":             "string",
		},
	}

	output, err := r.runtime.RunAgent(ctx, task)
	if err != nil {
		slog.Warn("reviewer LLM call failed, falling back to rule-based only", "error", err)
		result.Tier = domain.DealTierB
		result.Reasoning = "LLM reviewer unavailable — passed rule checks"
		return result, nil
	}

	ov, _ := getInt(output.Structured, "opportunity_viability")
	ec, _ := getInt(output.Structured, "execution_confidence")
	sf, _ := getInt(output.Structured, "sourcing_feasibility")
	reasoning, _ := output.Structured["reasoning"].(string)

	result.OpportunityViability = ov
	result.ExecutionConfidence = ec
	result.SourcingFeasibility = sf
	result.Reasoning = reasoning
	result.WeightedComposite = float64(ov)*0.35 + float64(ec)*0.35 + float64(sf)*0.30

	switch {
	case result.WeightedComposite >= thresholds.TierA:
		result.Tier = domain.DealTierA
	case result.WeightedComposite >= thresholds.TierB:
		result.Tier = domain.DealTierB
	case result.WeightedComposite >= 5.0:
		result.Tier = domain.DealTierC
	default:
		result.Tier = domain.DealTierCut
	}

	return result, nil
}

func getFloat(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	}
	return 0, false
}

func getInt(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		return val, true
	case float64:
		return int(val), true
	}
	return 0, false
}
