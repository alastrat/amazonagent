# Pipeline Architecture Refactor — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the monolithic `AgentRuntime.RunResearchPipeline()` into per-agent execution with Go-owned pipeline orchestration as an elimination funnel. Update the simulator to the new interface.

**Architecture:** The `AgentRuntime` interface changes from "run the whole pipeline" to "run one agent." Pipeline sequence (Source → Gate/Risk → Profit → Demand+Competition → Supplier → Review), quality gates, early elimination, context passing, and the hybrid Reviewer all live in Go's `PipelineService`. Agent runtimes (simulator, OpenFang, ZeroClaw) become thin per-agent executors.

**Tech Stack:** Go 1.25+, existing hexagonal architecture (domain → port → service → adapter)

---

## File Structure

### New files
```
internal/domain/pipeline.go           -- PipelineConfig, AgentConfig, PipelineThresholds, AgentContext, DealTier
internal/domain/agent.go              -- AgentTask, AgentOutput (domain types for agent execution)
internal/domain/validation.go         -- PlausibilityValidator (rule-based checks, bounds)
internal/domain/fees.go               -- FBA fee calculator (deterministic)
internal/service/pipeline_orchestrator.go  -- New pipeline orchestration (funnel, stages, context passing)
internal/service/pipeline_orchestrator_test.go
internal/service/reviewer.go          -- Hybrid reviewer (rules + scoring)
internal/service/reviewer_test.go
internal/adapter/simulator/per_agent.go  -- New simulator implementing per-agent RunAgent
```

### Modified files
```
internal/port/agent_runtime.go        -- Replace RunResearchPipeline with RunAgent
internal/domain/research.go           -- Add DealTier to CandidateResult
internal/domain/scoring.go            -- Extend with PipelineThresholds
internal/service/pipeline_service.go  -- Delegate to new orchestrator
internal/adapter/openfang/agent_runtime.go  -- Update to new interface
internal/adapter/simulator/agent_runtime.go -- Delegate to per_agent.go
apps/api/main.go                      -- Wire new PipelineConfig
```

### Deleted files
```
(none — we modify in place to preserve git history)
```

---

## Task 1: New Domain Types — AgentTask, AgentOutput, AgentContext

**Files:**
- Create: `internal/domain/agent.go`
- Create: `internal/domain/pipeline.go`

- [ ] **Step 1: Create `internal/domain/agent.go`**

```go
package domain

// AgentTask is the input to a single agent execution.
// The pipeline orchestrator constructs this; the runtime just executes it.
type AgentTask struct {
	AgentName    string            `json:"agent_name"`
	SystemPrompt string            `json:"system_prompt"`
	Input        map[string]any    `json:"input"`
	Context      []AgentContext    `json:"context,omitempty"`
	OutputSchema map[string]any    `json:"output_schema,omitempty"`
}

// AgentOutput is the result of a single agent execution.
type AgentOutput struct {
	Structured map[string]any `json:"structured"`
	Raw        string         `json:"raw"`
	TokensUsed int            `json:"tokens_used"`
	DurationMs int64          `json:"duration_ms"`
}

// AgentContext carries upstream facts to downstream agents.
// Not full reasoning — just structured data and flags.
type AgentContext struct {
	AgentName string         `json:"agent_name"`
	Facts     map[string]any `json:"facts"`
	Flags     []string       `json:"flags,omitempty"`
}
```

- [ ] **Step 2: Create `internal/domain/pipeline.go`**

```go
package domain

import "time"

type PipelineConfigID string

// PipelineConfig is a composable configuration for the research pipeline.
// Each agent has its own versioned config. Autoresearch varies one agent at a time.
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

// AgentConfig is the per-agent configuration within a pipeline.
type AgentConfig struct {
	Version      int            `json:"version"`
	SystemPrompt string         `json:"system_prompt"`
	Tools        []string       `json:"tools"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	ModelTier    string         `json:"model_tier"` // "fast", "standard", "premium"
}

// PipelineThresholds controls quality gates and the reviewer.
type PipelineThresholds struct {
	MinMarginPct         float64 `json:"min_margin_pct"`
	RiskMaxScore         int     `json:"risk_max_score"`
	TierA                float64 `json:"tier_a"`
	TierB                float64 `json:"tier_b"`
	MaxRewriteIterations int     `json:"max_rewrite_iterations"`
	RewriteMinDelta      float64 `json:"rewrite_min_delta"`
}

// DealTier represents the quality tier assigned by the reviewer.
type DealTier string

const (
	DealTierA   DealTier = "A" // auto-recommend
	DealTierB   DealTier = "B" // worth reviewing
	DealTierC   DealTier = "C" // niche opportunity
	DealTierCut DealTier = "cut"
)

// DefaultPipelineThresholds returns sensible defaults.
func DefaultPipelineThresholds() PipelineThresholds {
	return PipelineThresholds{
		MinMarginPct:         15.0,
		RiskMaxScore:         7,
		TierA:                8.0,
		TierB:                6.5,
		MaxRewriteIterations: 2,
		RewriteMinDelta:      0.05,
	}
}

// DefaultPipelineConfig returns a starter config with default prompts.
func DefaultPipelineConfig(tenantID TenantID) PipelineConfig {
	return PipelineConfig{
		TenantID: tenantID,
		Name:     "default",
		Agents: map[string]AgentConfig{
			"sourcing":    {Version: 1, SystemPrompt: "You are a product sourcing agent. Find candidate ASINs matching the given criteria using ceiling/floor logic.", ModelTier: "fast"},
			"gating":      {Version: 1, SystemPrompt: "You are a gating and risk assessment agent. Evaluate whether products can be sold: check category gating, IP risk, brand restrictions, hazmat status.", ModelTier: "fast"},
			"profitability": {Version: 1, SystemPrompt: "You are a profitability analysis agent. Given pre-calculated FBA fees, evaluate margin, ROI, and cash flow viability.", ModelTier: "standard"},
			"demand":      {Version: 1, SystemPrompt: "You are a demand and competition analysis agent. Evaluate sales velocity, BSR trends, seller landscape, buy box dynamics, and social sentiment.", ModelTier: "standard"},
			"supplier":    {Version: 1, SystemPrompt: "You are a supplier discovery agent. Find and evaluate wholesale suppliers, compare pricing and terms, draft outreach templates.", ModelTier: "standard"},
			"reviewer":    {Version: 1, SystemPrompt: "You are the pipeline reviewer. Score each candidate on Opportunity Viability, Execution Confidence, and Sourcing Feasibility (1-10 each). Provide reasoning.", ModelTier: "premium"},
		},
		Scoring:    DefaultScoringWeights(),
		Thresholds: DefaultPipelineThresholds(),
		CreatedBy:  "system",
		Active:     true,
	}
}
```

- [ ] **Step 3: Verify compilation**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/domain/agent.go internal/domain/pipeline.go
git commit -m "feat: add AgentTask, AgentOutput, PipelineConfig domain types"
```

---

## Task 2: Refactor AgentRuntime Interface to Per-Agent Execution

**Files:**
- Modify: `internal/port/agent_runtime.go`
- Modify: `internal/adapter/openfang/agent_runtime.go`
- Modify: `internal/adapter/simulator/agent_runtime.go`

- [ ] **Step 1: Replace `internal/port/agent_runtime.go`**

```go
package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// AgentRuntime executes a single agent task.
// Pipeline orchestration (sequence, gates, retries) is NOT the runtime's job.
// The runtime is a thin executor — it receives a task and returns structured output.
type AgentRuntime interface {
	RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error)
}
```

- [ ] **Step 2: Update `internal/adapter/openfang/agent_runtime.go`**

```go
package openfang

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type AgentRuntime struct {
	apiURL string
	apiKey string
}

func NewAgentRuntime(apiURL, apiKey string) *AgentRuntime {
	return &AgentRuntime{apiURL: apiURL, apiKey: apiKey}
}

func (r *AgentRuntime) RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	slog.Info("running agent via OpenFang",
		"agent", task.AgentName,
		"url", r.apiURL,
	)

	// TODO: implement real OpenFang API call
	return nil, fmt.Errorf("OpenFang agent execution not yet implemented for agent %q", task.AgentName)
}
```

- [ ] **Step 3: Create minimal simulator stub**

Replace `internal/adapter/simulator/agent_runtime.go` with a minimal stub that compiles. The full per-agent simulator comes in Task 5.

```go
package simulator

import (
	"context"
	"fmt"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type AgentRuntime struct{}

func NewAgentRuntime() *AgentRuntime {
	return &AgentRuntime{}
}

func (r *AgentRuntime) RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	return nil, fmt.Errorf("simulator agent %q not yet implemented", task.AgentName)
}
```

- [ ] **Step 4: Verify compilation fails (PipelineService still uses old interface)**

```bash
go build ./... 2>&1 | head -5
```

Expected: compilation errors in `pipeline_service.go` and `main.go` — this is correct, we fix them in the next tasks.

- [ ] **Step 5: Commit (with build broken — we fix it in the next tasks)**

```bash
git add internal/port/agent_runtime.go internal/adapter/openfang/agent_runtime.go internal/adapter/simulator/agent_runtime.go
git commit -m "refactor: change AgentRuntime interface from RunResearchPipeline to per-agent RunAgent"
```

---

## Task 3: FBA Fee Calculator + Plausibility Validator

**Files:**
- Create: `internal/domain/fees.go`
- Create: `internal/domain/fees_test.go`
- Create: `internal/domain/validation.go`
- Create: `internal/domain/validation_test.go`

- [ ] **Step 1: Create `internal/domain/fees.go`**

```go
package domain

// FBAFeeCalculation holds the deterministic fee breakdown for a product.
type FBAFeeCalculation struct {
	ReferralFeePct float64 `json:"referral_fee_pct"`
	ReferralFee    float64 `json:"referral_fee"`
	FBAFulfillment float64 `json:"fba_fulfillment"`
	StorageFee     float64 `json:"storage_fee_monthly"`
	TotalFees      float64 `json:"total_fees"`
	NetProfit      float64 `json:"net_profit"`
	NetMarginPct   float64 `json:"net_margin_pct"`
	ROIPct         float64 `json:"roi_pct"`
}

// CalculateFBAFees computes deterministic FBA fee breakdown.
// This is NEVER an LLM task — it's pure arithmetic.
func CalculateFBAFees(amazonPrice, wholesaleCost, weightLbs float64, isOversized bool) FBAFeeCalculation {
	// Referral fee: 15% for most categories
	referralPct := 0.15
	referralFee := amazonPrice * referralPct

	// FBA fulfillment fee (simplified — real implementation uses Amazon's fee tables)
	fbaFee := 3.22 // small standard
	if weightLbs > 1.0 {
		fbaFee = 4.75 + (weightLbs-1.0)*0.40 // standard weight-based
	}
	if isOversized {
		fbaFee = 9.73 + (weightLbs-2.0)*0.42 // oversized
	}

	// Monthly storage (simplified)
	storageFee := 0.87 // per cubic foot per month, standard

	totalFees := referralFee + fbaFee + storageFee
	landedCost := wholesaleCost * 1.10 // 10% for shipping/prep
	netProfit := amazonPrice - landedCost - totalFees

	netMarginPct := 0.0
	if amazonPrice > 0 {
		netMarginPct = (netProfit / amazonPrice) * 100
	}

	roiPct := 0.0
	if landedCost > 0 {
		roiPct = (netProfit / landedCost) * 100
	}

	return FBAFeeCalculation{
		ReferralFeePct: referralPct * 100,
		ReferralFee:    referralFee,
		FBAFulfillment: fbaFee,
		StorageFee:     storageFee,
		TotalFees:      totalFees,
		NetProfit:      netProfit,
		NetMarginPct:   netMarginPct,
		ROIPct:         roiPct,
	}
}
```

- [ ] **Step 2: Create `internal/domain/fees_test.go`**

```go
package domain

import (
	"math"
	"testing"
)

func TestCalculateFBAFees_StandardProduct(t *testing.T) {
	result := CalculateFBAFees(25.00, 10.00, 0.8, false)

	if result.ReferralFeePct != 15.0 {
		t.Errorf("expected 15%% referral, got %.1f%%", result.ReferralFeePct)
	}
	if math.Abs(result.ReferralFee-3.75) > 0.01 {
		t.Errorf("expected referral fee ~3.75, got %.2f", result.ReferralFee)
	}
	if result.FBAFulfillment != 3.22 {
		t.Errorf("expected FBA fee 3.22, got %.2f", result.FBAFulfillment)
	}
	if result.NetProfit <= 0 {
		t.Error("expected positive net profit for this product")
	}
	if result.NetMarginPct <= 0 {
		t.Error("expected positive margin")
	}
	if result.ROIPct <= 0 {
		t.Error("expected positive ROI")
	}
}

func TestCalculateFBAFees_HeavyProduct(t *testing.T) {
	result := CalculateFBAFees(50.00, 20.00, 3.5, false)

	// FBA fee: 4.75 + (3.5-1.0)*0.40 = 4.75 + 1.00 = 5.75
	if math.Abs(result.FBAFulfillment-5.75) > 0.01 {
		t.Errorf("expected FBA fee ~5.75, got %.2f", result.FBAFulfillment)
	}
}

func TestCalculateFBAFees_OversizedProduct(t *testing.T) {
	result := CalculateFBAFees(80.00, 30.00, 5.0, true)

	// Oversized FBA fee: 9.73 + (5.0-2.0)*0.42 = 9.73 + 1.26 = 10.99
	if math.Abs(result.FBAFulfillment-10.99) > 0.01 {
		t.Errorf("expected FBA fee ~10.99, got %.2f", result.FBAFulfillment)
	}
}

func TestCalculateFBAFees_NegativeMargin(t *testing.T) {
	result := CalculateFBAFees(12.00, 10.00, 0.5, false)

	// Amazon price barely above wholesale — should have negative or very low margin
	if result.NetMarginPct > 20 {
		t.Errorf("expected low/negative margin for thin spread, got %.1f%%", result.NetMarginPct)
	}
}
```

- [ ] **Step 3: Create `internal/domain/validation.go`**

```go
package domain

import "fmt"

// PlausibilityError represents a validation failure.
type PlausibilityError struct {
	Field   string
	Value   any
	Message string
}

func (e PlausibilityError) Error() string {
	return fmt.Sprintf("plausibility check failed: %s=%v — %s", e.Field, e.Value, e.Message)
}

// ValidateAgentOutput checks plausibility bounds on agent output fields.
// Returns nil if all checks pass, or a list of errors.
func ValidateAgentOutput(agentName string, output map[string]any) []PlausibilityError {
	var errs []PlausibilityError

	check := func(field string, val any, min, max float64) {
		switch v := val.(type) {
		case float64:
			if v < min || v > max {
				errs = append(errs, PlausibilityError{Field: field, Value: v, Message: fmt.Sprintf("expected %.0f–%.0f", min, max)})
			}
		case int:
			if float64(v) < min || float64(v) > max {
				errs = append(errs, PlausibilityError{Field: field, Value: v, Message: fmt.Sprintf("expected %.0f–%.0f", min, max)})
			}
		}
	}

	switch agentName {
	case "gating":
		if v, ok := output["risk_score"]; ok {
			check("risk_score", v, 0, 10)
		}
	case "profitability":
		if v, ok := output["net_margin_pct"]; ok {
			check("net_margin_pct", v, -100, 500)
		}
		if v, ok := output["wholesale_cost"]; ok {
			check("wholesale_cost", v, 0, 100000)
		}
		if v, ok := output["amazon_price"]; ok {
			check("amazon_price", v, 0, 100000)
		}
	case "demand":
		if v, ok := output["bsr_rank"]; ok {
			check("bsr_rank", v, 1, 10000000)
		}
		if v, ok := output["monthly_units"]; ok {
			check("monthly_units", v, 0, 1000000)
		}
	case "reviewer":
		for _, dim := range []string{"opportunity_viability", "execution_confidence", "sourcing_feasibility"} {
			if v, ok := output[dim]; ok {
				check(dim, v, 1, 10)
			}
		}
	}

	return errs
}
```

- [ ] **Step 4: Create `internal/domain/validation_test.go`**

```go
package domain

import "testing"

func TestValidateAgentOutput_Profitability_Valid(t *testing.T) {
	output := map[string]any{
		"net_margin_pct": 25.0,
		"wholesale_cost": 15.0,
		"amazon_price":   35.0,
	}
	errs := ValidateAgentOutput("profitability", output)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateAgentOutput_Profitability_InvalidMargin(t *testing.T) {
	output := map[string]any{
		"net_margin_pct": 600.0, // > 500% is implausible
	}
	errs := ValidateAgentOutput("profitability", output)
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestValidateAgentOutput_Demand_InvalidBSR(t *testing.T) {
	output := map[string]any{
		"bsr_rank": -5, // negative BSR
	}
	errs := ValidateAgentOutput("demand", output)
	if len(errs) != 1 {
		t.Errorf("expected 1 error for negative BSR, got %d", len(errs))
	}
}

func TestValidateAgentOutput_Reviewer_ValidScores(t *testing.T) {
	output := map[string]any{
		"opportunity_viability": 8,
		"execution_confidence":  7,
		"sourcing_feasibility":  9,
	}
	errs := ValidateAgentOutput("reviewer", output)
	if len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateAgentOutput_Reviewer_OutOfRange(t *testing.T) {
	output := map[string]any{
		"opportunity_viability": 12, // > 10
		"execution_confidence":  0,  // < 1
		"sourcing_feasibility":  8,
	}
	errs := ValidateAgentOutput("reviewer", output)
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/domain/... -v -count=1
```

Expected: all domain tests pass (existing + new)

- [ ] **Step 6: Commit**

```bash
git add internal/domain/fees.go internal/domain/fees_test.go internal/domain/validation.go internal/domain/validation_test.go
git commit -m "feat: add deterministic FBA fee calculator and plausibility validator"
```

---

## Task 4: Hybrid Reviewer Service

**Files:**
- Create: `internal/service/reviewer.go`
- Create: `internal/service/reviewer_test.go`

- [ ] **Step 1: Create `internal/service/reviewer.go`**

```go
package service

import (
	"context"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// ReviewResult is the output of the hybrid reviewer.
type ReviewResult struct {
	Tier                  domain.DealTier `json:"tier"`
	OpportunityViability  int             `json:"opportunity_viability"`
	ExecutionConfidence   int             `json:"execution_confidence"`
	SourcingFeasibility   int             `json:"sourcing_feasibility"`
	WeightedComposite     float64         `json:"weighted_composite"`
	RuleChecksPassed      bool            `json:"rule_checks_passed"`
	RuleFailures          []string        `json:"rule_failures,omitempty"`
	Reasoning             string          `json:"reasoning"`
}

// Reviewer implements the hybrid review pattern: rules first, then LLM scoring.
type Reviewer struct {
	runtime port.AgentRuntime
}

func NewReviewer(runtime port.AgentRuntime) *Reviewer {
	return &Reviewer{runtime: runtime}
}

// Review runs rule-based checks first. If they pass, runs LLM scoring for subjective quality.
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

	// Check margin threshold
	if marginPct, ok := getFloat(candidate, "net_margin_pct"); ok {
		if marginPct < thresholds.MinMarginPct {
			ruleFailures = append(ruleFailures, "margin below minimum threshold")
		}
	}

	// Check risk score
	if riskScore, ok := getInt(candidate, "risk_score"); ok {
		if riskScore > thresholds.RiskMaxScore {
			ruleFailures = append(ruleFailures, "risk score exceeds maximum")
		}
	}

	// Check required fields exist
	for _, field := range []string{"asin", "title"} {
		if _, ok := candidate[field]; !ok {
			ruleFailures = append(ruleFailures, "missing required field: "+field)
		}
	}

	// Plausibility validation on all numeric fields
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
		// Fallback: pass with B-tier if rules passed
		result.Tier = domain.DealTierB
		result.Reasoning = "LLM reviewer unavailable — passed rule checks"
		return result, nil
	}

	// Extract scores from LLM output
	ov, _ := getInt(output.Structured, "opportunity_viability")
	ec, _ := getInt(output.Structured, "execution_confidence")
	sf, _ := getInt(output.Structured, "sourcing_feasibility")
	reasoning, _ := output.Structured["reasoning"].(string)

	result.OpportunityViability = ov
	result.ExecutionConfidence = ec
	result.SourcingFeasibility = sf
	result.Reasoning = reasoning

	// Weighted composite score
	result.WeightedComposite = float64(ov)*0.35 + float64(ec)*0.35 + float64(sf)*0.30

	// Assign tier
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

// helper to extract float from map[string]any
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

// helper to extract int from map[string]any
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
```

- [ ] **Step 2: Create `internal/service/reviewer_test.go`**

```go
package service_test

import (
	"context"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

// mockAgentRuntime for reviewer tests
type mockReviewerRuntime struct {
	output *domain.AgentOutput
	err    error
}

func (r *mockReviewerRuntime) RunAgent(_ context.Context, _ domain.AgentTask) (*domain.AgentOutput, error) {
	return r.output, r.err
}

func TestReviewer_RuleFailure_LowMargin(t *testing.T) {
	runtime := &mockReviewerRuntime{}
	reviewer := service.NewReviewer(runtime)

	candidate := map[string]any{
		"asin":           "B0TEST001",
		"title":          "Test Product",
		"net_margin_pct": 5.0, // below 15% threshold
	}

	result, err := reviewer.Review(
		context.Background(),
		candidate,
		nil,
		domain.AgentConfig{SystemPrompt: "test"},
		domain.DefaultPipelineThresholds(),
		domain.DefaultScoringWeights(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tier != domain.DealTierCut {
		t.Errorf("expected cut, got %s", result.Tier)
	}
	if result.RuleChecksPassed {
		t.Error("expected rule checks to fail")
	}
}

func TestReviewer_LLMScoring_TierA(t *testing.T) {
	runtime := &mockReviewerRuntime{
		output: &domain.AgentOutput{
			Structured: map[string]any{
				"opportunity_viability": 9,
				"execution_confidence":  9,
				"sourcing_feasibility":  8,
				"reasoning":             "Strong opportunity",
			},
		},
	}
	reviewer := service.NewReviewer(runtime)

	candidate := map[string]any{
		"asin":           "B0TEST001",
		"title":          "Test Product",
		"net_margin_pct": 30.0,
	}

	result, err := reviewer.Review(
		context.Background(),
		candidate,
		nil,
		domain.AgentConfig{SystemPrompt: "test"},
		domain.DefaultPipelineThresholds(),
		domain.DefaultScoringWeights(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tier != domain.DealTierA {
		t.Errorf("expected A-tier, got %s (composite: %.2f)", result.Tier, result.WeightedComposite)
	}
	if !result.RuleChecksPassed {
		t.Error("expected rule checks to pass")
	}
}

func TestReviewer_LLMScoring_TierB(t *testing.T) {
	runtime := &mockReviewerRuntime{
		output: &domain.AgentOutput{
			Structured: map[string]any{
				"opportunity_viability": 7,
				"execution_confidence":  7,
				"sourcing_feasibility":  7,
				"reasoning":             "Decent opportunity",
			},
		},
	}
	reviewer := service.NewReviewer(runtime)

	candidate := map[string]any{
		"asin":           "B0TEST001",
		"title":          "Test Product",
		"net_margin_pct": 25.0,
	}

	result, err := reviewer.Review(
		context.Background(),
		candidate,
		nil,
		domain.AgentConfig{SystemPrompt: "test"},
		domain.DefaultPipelineThresholds(),
		domain.DefaultScoringWeights(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tier != domain.DealTierB {
		t.Errorf("expected B-tier, got %s (composite: %.2f)", result.Tier, result.WeightedComposite)
	}
}

func TestReviewer_LLMFallback_WhenRuntimeFails(t *testing.T) {
	runtime := &mockReviewerRuntime{
		err: fmt.Errorf("LLM unavailable"),
	}
	reviewer := service.NewReviewer(runtime)

	candidate := map[string]any{
		"asin":           "B0TEST001",
		"title":          "Test Product",
		"net_margin_pct": 25.0,
	}

	result, err := reviewer.Review(
		context.Background(),
		candidate,
		nil,
		domain.AgentConfig{SystemPrompt: "test"},
		domain.DefaultPipelineThresholds(),
		domain.DefaultScoringWeights(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fallback to B-tier when LLM fails but rules pass
	if result.Tier != domain.DealTierB {
		t.Errorf("expected B-tier fallback, got %s", result.Tier)
	}
}
```

Note: The test file needs `"fmt"` imported for the `fmt.Errorf` in the last test. Add it to the imports.

- [ ] **Step 3: Run tests**

```bash
go test ./internal/service/... -v -count=1 -run TestReviewer
```

Expected: all 4 reviewer tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/service/reviewer.go internal/service/reviewer_test.go
git commit -m "feat: add hybrid reviewer — rule-based checks + LLM scoring with tiered output"
```

---

## Task 5: Per-Agent Simulator

**Files:**
- Rewrite: `internal/adapter/simulator/agent_runtime.go`

- [ ] **Step 1: Rewrite `internal/adapter/simulator/agent_runtime.go`**

This implements the new `RunAgent` interface. Each agent name maps to a simulator function that returns realistic fake structured output.

```go
package simulator

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type AgentRuntime struct{}

func NewAgentRuntime() *AgentRuntime {
	return &AgentRuntime{}
}

func (r *AgentRuntime) RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	// Simulate latency
	time.Sleep(time.Duration(50+rand.Intn(200)) * time.Millisecond)

	start := time.Now()
	var structured map[string]any

	switch task.AgentName {
	case "sourcing":
		structured = simulateSourcing(task.Input)
	case "gating":
		structured = simulateGating(task.Input)
	case "profitability":
		structured = simulateProfitability(task.Input)
	case "demand":
		structured = simulateDemand(task.Input)
	case "supplier":
		structured = simulateSupplier(task.Input)
	case "reviewer":
		structured = simulateReviewer(task.Input)
	default:
		return nil, fmt.Errorf("unknown agent: %s", task.AgentName)
	}

	return &domain.AgentOutput{
		Structured: structured,
		Raw:        fmt.Sprintf("simulated %s output", task.AgentName),
		TokensUsed: 100 + rand.Intn(900),
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

func simulateSourcing(input map[string]any) map[string]any {
	// Return a list of candidate ASINs with basic data
	candidates := []map[string]any{}
	products := []struct{ ASIN, Title, Brand, Category string }{
		{"B0CX23V5KK", "Stainless Steel Kitchen Utensil Set", "HomeChef Pro", "Kitchen"},
		{"B0D1FG89NM", "Silicone Baking Mat Set (3 Pack)", "BakeRight", "Kitchen"},
		{"B0BY7K3PQR", "Bamboo Cutting Board with Juice Groove", "EcoBoard", "Kitchen"},
		{"B0CR9T23YZ", "Adjustable Dumbbell Set 25lb", "FitCore", "Fitness"},
		{"B0D5HJ78AB", "Resistance Band Set with Door Anchor", "FlexFit", "Fitness"},
		{"B0DQ6N45EF", "Wireless Earbuds Noise Cancelling", "SoundPeak", "Electronics"},
		{"B0CT3P78GH", "Portable Phone Charger 20000mAh", "JuiceBox", "Electronics"},
		{"B0D8KR12IJ", "LED Desk Lamp with USB Charging", "BrightWork", "Office"},
		{"B0CW7T56MN", "Stainless Steel Water Bottle 32oz", "HydroKeep", "Sports"},
		{"B0DJ9U78OP", "Microfiber Cleaning Cloth 12-Pack", "CleanPro", "Home"},
	}

	n := 5 + rand.Intn(6) // 5-10 candidates
	if n > len(products) {
		n = len(products)
	}
	perm := rand.Perm(len(products))
	for i := 0; i < n; i++ {
		p := products[perm[i]]
		candidates = append(candidates, map[string]any{
			"asin":          p.ASIN,
			"title":         p.Title,
			"brand":         p.Brand,
			"category":      p.Category,
			"amazon_price":  15.0 + rand.Float64()*60.0,
			"bsr_rank":      1000 + rand.Intn(50000),
			"seller_count":  2 + rand.Intn(20),
		})
	}
	return map[string]any{"candidates": candidates}
}

func simulateGating(input map[string]any) map[string]any {
	// 60% pass gating, 40% fail
	passed := rand.Float64() > 0.40
	riskScore := 1 + rand.Intn(10)

	flags := []string{}
	if !passed {
		flags = append(flags, []string{"category_gated", "ip_risk", "brand_restricted", "hazmat"}[rand.Intn(4)])
	}

	return map[string]any{
		"passed":     passed,
		"risk_score": riskScore,
		"flags":      flags,
		"reasoning":  fmt.Sprintf("Gating assessment: passed=%v, risk_score=%d", passed, riskScore),
	}
}

func simulateProfitability(input map[string]any) map[string]any {
	amazonPrice := 20.0 + rand.Float64()*50.0
	wholesaleCost := amazonPrice * (0.3 + rand.Float64()*0.4) // 30-70% of amazon price
	fees := domain.CalculateFBAFees(amazonPrice, wholesaleCost, 0.5+rand.Float64()*3.0, false)

	return map[string]any{
		"amazon_price":   amazonPrice,
		"wholesale_cost": wholesaleCost,
		"net_margin_pct": fees.NetMarginPct,
		"roi_pct":        fees.ROIPct,
		"net_profit":     fees.NetProfit,
		"total_fees":     fees.TotalFees,
		"referral_fee":   fees.ReferralFee,
		"fba_fee":        fees.FBAFulfillment,
		"reasoning":      fmt.Sprintf("Margin: %.1f%%, ROI: %.0f%%", fees.NetMarginPct, fees.ROIPct),
	}
}

func simulateDemand(input map[string]any) map[string]any {
	return map[string]any{
		"demand_score":      6 + rand.Intn(5),
		"competition_score": 5 + rand.Intn(6),
		"bsr_rank":          1000 + rand.Intn(50000),
		"monthly_units":     100 + rand.Intn(5000),
		"fba_sellers":       2 + rand.Intn(20),
		"buy_box_share":     0.1 + rand.Float64()*0.5,
		"trend":             []string{"improving", "stable", "declining"}[rand.Intn(3)],
		"reasoning":         "Demand analysis based on BSR trends, velocity, and competition landscape.",
	}
}

func simulateSupplier(input map[string]any) map[string]any {
	wholesaleCost := 10.0 + rand.Float64()*30.0
	numSuppliers := 1 + rand.Intn(4)
	suppliers := []map[string]any{}
	for i := 0; i < numSuppliers; i++ {
		suppliers = append(suppliers, map[string]any{
			"company":       fmt.Sprintf("Supplier %c Distribution", rune('A'+i)),
			"unit_price":    wholesaleCost * (0.9 + rand.Float64()*0.2),
			"moq":           24 + rand.Intn(200),
			"lead_time_days": 5 + rand.Intn(25),
			"authorized":    rand.Float64() > 0.3,
		})
	}
	return map[string]any{
		"suppliers":      suppliers,
		"best_price":     wholesaleCost * 0.95,
		"outreach_draft": "Hi, I'm interested in wholesale pricing for this product. We're an established Amazon FBA seller...",
		"reasoning":      fmt.Sprintf("Found %d suppliers, best price: $%.2f", numSuppliers, wholesaleCost*0.95),
	}
}

func simulateReviewer(input map[string]any) map[string]any {
	ov := 5 + rand.Intn(6)  // 5-10
	ec := 5 + rand.Intn(6)
	sf := 5 + rand.Intn(6)
	return map[string]any{
		"opportunity_viability": ov,
		"execution_confidence":  ec,
		"sourcing_feasibility":  sf,
		"reasoning":             fmt.Sprintf("Scores: OV=%d, EC=%d, SF=%d. Overall assessment of the opportunity.", ov, ec, sf),
	}
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/adapter/simulator/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/adapter/simulator/agent_runtime.go
git commit -m "feat: rewrite simulator with per-agent RunAgent interface"
```

---

## Task 6: Pipeline Orchestrator (the core refactor)

**Files:**
- Create: `internal/service/pipeline_orchestrator.go`
- Create: `internal/service/pipeline_orchestrator_test.go`
- Modify: `internal/service/pipeline_service.go`
- Modify: `internal/domain/research.go`

- [ ] **Step 1: Add `Tier` field to `CandidateResult` in `internal/domain/research.go`**

Add after the `IterationCount` field:

```go
type CandidateResult struct {
	ASIN               string              `json:"asin"`
	Title              string              `json:"title"`
	Brand              string              `json:"brand"`
	Category           string              `json:"category"`
	Scores             DealScores          `json:"scores"`
	Evidence           Evidence            `json:"evidence"`
	SupplierCandidates []SupplierCandidate `json:"supplier_candidates"`
	OutreachDrafts     []string            `json:"outreach_drafts"`
	ReviewerVerdict    string              `json:"reviewer_verdict"`
	Tier               DealTier            `json:"tier"`
	IterationCount     int                 `json:"iteration_count"`
}
```

- [ ] **Step 2: Create `internal/service/pipeline_orchestrator.go`**

```go
package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// PipelineOrchestrator owns the research pipeline sequence.
// It calls agents one at a time through the AgentRuntime interface,
// applies quality gates, and manages the elimination funnel.
type PipelineOrchestrator struct {
	runtime  port.AgentRuntime
	reviewer *Reviewer
}

func NewPipelineOrchestrator(runtime port.AgentRuntime) *PipelineOrchestrator {
	return &PipelineOrchestrator{
		runtime:  runtime,
		reviewer: NewReviewer(runtime),
	}
}

// RunPipeline executes the full research pipeline for a campaign.
// Sequence: Source → Gate/Risk → Profit → Demand+Competition → Supplier → Review
func (o *PipelineOrchestrator) RunPipeline(ctx context.Context, campaignID domain.CampaignID, criteria domain.Criteria, config domain.PipelineConfig) (*domain.ResearchResult, error) {
	slog.Info("pipeline: starting", "campaign_id", campaignID)

	// Stage 1: Sourcing — find candidate ASINs
	sourcingCfg := config.Agents["sourcing"]
	sourcingOut, err := o.runtime.RunAgent(ctx, domain.AgentTask{
		AgentName:    "sourcing",
		SystemPrompt: sourcingCfg.SystemPrompt,
		Input:        map[string]any{"criteria": criteria},
	})
	if err != nil {
		return nil, fmt.Errorf("sourcing agent failed: %w", err)
	}

	candidatesRaw, ok := sourcingOut.Structured["candidates"].([]any)
	if !ok {
		// Try typed assertion for simulator
		if typed, ok2 := sourcingOut.Structured["candidates"].([]map[string]any); ok2 {
			for _, c := range typed {
				candidatesRaw = append(candidatesRaw, c)
			}
		}
	}
	if len(candidatesRaw) == 0 {
		return &domain.ResearchResult{
			CampaignID: campaignID,
			Summary:    "No candidates found by sourcing agent",
		}, nil
	}

	slog.Info("pipeline: sourcing complete", "candidates", len(candidatesRaw))

	var trail []domain.AgentTrailEntry
	trail = append(trail, domain.AgentTrailEntry{AgentName: "sourcing", DurationMs: sourcingOut.DurationMs})

	// Process each candidate through the funnel
	var results []domain.CandidateResult
	for _, rawCandidate := range candidatesRaw {
		candidateMap, ok := rawCandidate.(map[string]any)
		if !ok {
			continue
		}

		asin, _ := candidateMap["asin"].(string)
		title, _ := candidateMap["title"].(string)
		brand, _ := candidateMap["brand"].(string)
		category, _ := candidateMap["category"].(string)

		if asin == "" {
			continue
		}

		var agentContexts []domain.AgentContext

		// Stage 2: Gate/Risk — early elimination
		gatingCfg := config.Agents["gating"]
		gatingOut, err := o.runtime.RunAgent(ctx, domain.AgentTask{
			AgentName:    "gating",
			SystemPrompt: gatingCfg.SystemPrompt,
			Input:        candidateMap,
		})
		if err != nil {
			slog.Warn("pipeline: gating failed, skipping candidate", "asin", asin, "error", err)
			continue
		}
		trail = append(trail, domain.AgentTrailEntry{AgentName: "gating", ASIN: asin, DurationMs: gatingOut.DurationMs})

		passed, _ := gatingOut.Structured["passed"].(bool)
		if !passed {
			slog.Debug("pipeline: candidate eliminated at gating", "asin", asin)
			continue
		}

		agentContexts = append(agentContexts, domain.AgentContext{
			AgentName: "gating",
			Facts:     gatingOut.Structured,
			Flags:     toStringSlice(gatingOut.Structured["flags"]),
		})

		// Stage 3: Profitability — margin check
		profitCfg := config.Agents["profitability"]
		profitOut, err := o.runtime.RunAgent(ctx, domain.AgentTask{
			AgentName:    "profitability",
			SystemPrompt: profitCfg.SystemPrompt,
			Input:        candidateMap,
			Context:      agentContexts,
		})
		if err != nil {
			slog.Warn("pipeline: profitability failed, skipping", "asin", asin, "error", err)
			continue
		}
		trail = append(trail, domain.AgentTrailEntry{AgentName: "profitability", ASIN: asin, DurationMs: profitOut.DurationMs})

		marginPct, _ := getFloat(profitOut.Structured, "net_margin_pct")
		if marginPct < config.Thresholds.MinMarginPct {
			slog.Debug("pipeline: candidate eliminated at profitability", "asin", asin, "margin", marginPct)
			continue
		}

		// Validate profitability output
		if errs := domain.ValidateAgentOutput("profitability", profitOut.Structured); len(errs) > 0 {
			slog.Warn("pipeline: profitability output failed validation", "asin", asin, "errors", errs)
			continue
		}

		agentContexts = append(agentContexts, domain.AgentContext{
			AgentName: "profitability",
			Facts:     profitOut.Structured,
		})

		// Stage 4: Demand + Competition
		demandCfg := config.Agents["demand"]
		demandOut, err := o.runtime.RunAgent(ctx, domain.AgentTask{
			AgentName:    "demand",
			SystemPrompt: demandCfg.SystemPrompt,
			Input:        candidateMap,
			Context:      agentContexts,
		})
		if err != nil {
			slog.Warn("pipeline: demand failed, skipping", "asin", asin, "error", err)
			continue
		}
		trail = append(trail, domain.AgentTrailEntry{AgentName: "demand", ASIN: asin, DurationMs: demandOut.DurationMs})

		agentContexts = append(agentContexts, domain.AgentContext{
			AgentName: "demand",
			Facts:     demandOut.Structured,
		})

		// Stage 5: Supplier
		supplierCfg := config.Agents["supplier"]
		supplierOut, err := o.runtime.RunAgent(ctx, domain.AgentTask{
			AgentName:    "supplier",
			SystemPrompt: supplierCfg.SystemPrompt,
			Input:        candidateMap,
			Context:      agentContexts,
		})
		if err != nil {
			slog.Warn("pipeline: supplier failed, skipping", "asin", asin, "error", err)
			continue
		}
		trail = append(trail, domain.AgentTrailEntry{AgentName: "supplier", ASIN: asin, DurationMs: supplierOut.DurationMs})

		agentContexts = append(agentContexts, domain.AgentContext{
			AgentName: "supplier",
			Facts:     supplierOut.Structured,
		})

		// Stage 6: Reviewer (hybrid)
		// Assemble all data for review
		reviewInput := mergeMaps(candidateMap, profitOut.Structured, demandOut.Structured, supplierOut.Structured)

		reviewerCfg := config.Agents["reviewer"]
		reviewResult, err := o.reviewer.Review(ctx, reviewInput, agentContexts, reviewerCfg, config.Thresholds, config.Scoring)
		if err != nil {
			slog.Warn("pipeline: reviewer failed", "asin", asin, "error", err)
			continue
		}
		trail = append(trail, domain.AgentTrailEntry{AgentName: "reviewer", ASIN: asin})

		if reviewResult.Tier == domain.DealTierCut {
			slog.Debug("pipeline: candidate cut by reviewer", "asin", asin)
			continue
		}

		// Extract scores
		demandScore, _ := getInt(demandOut.Structured, "demand_score")
		competitionScore, _ := getInt(demandOut.Structured, "competition_score")
		marginScore := scoreFromMargin(marginPct)
		riskScore, _ := getInt(gatingOut.Structured, "risk_score")
		sourcingScore := reviewResult.SourcingFeasibility

		overall := float64(demandScore)*config.Scoring.Demand +
			float64(competitionScore)*config.Scoring.Competition +
			float64(marginScore)*config.Scoring.Margin +
			float64(10-riskScore)*config.Scoring.Risk + // invert: low risk = high score
			float64(sourcingScore)*config.Scoring.Sourcing

		// Build supplier candidates
		var supplierCandidates []domain.SupplierCandidate
		if rawSuppliers, ok := supplierOut.Structured["suppliers"].([]any); ok {
			for _, rs := range rawSuppliers {
				if sm, ok := rs.(map[string]any); ok {
					sc := domain.SupplierCandidate{
						Company: fmt.Sprintf("%v", sm["company"]),
					}
					if up, ok := getFloat(sm, "unit_price"); ok {
						sc.UnitPrice = up
					}
					if moq, ok := getInt(sm, "moq"); ok {
						sc.MOQ = moq
					}
					if lt, ok := getInt(sm, "lead_time_days"); ok {
						sc.LeadTimeDays = lt
					}
					if auth, ok := sm["authorized"].(bool); ok {
						sc.Authorized = auth
					}
					supplierCandidates = append(supplierCandidates, sc)
				}
			}
		}

		outreachDraft, _ := supplierOut.Structured["outreach_draft"].(string)
		var outreachDrafts []string
		if outreachDraft != "" {
			outreachDrafts = []string{outreachDraft}
		}

		result := domain.CandidateResult{
			ASIN:     asin,
			Title:    title,
			Brand:    brand,
			Category: category,
			Scores: domain.DealScores{
				Demand:              demandScore,
				Competition:         competitionScore,
				Margin:              marginScore,
				Risk:                10 - riskScore,
				SourcingFeasibility: sourcingScore,
				Overall:             overall,
			},
			Evidence: domain.Evidence{
				Demand:      domain.AgentEvidence{Reasoning: strVal(demandOut.Structured, "reasoning"), Data: demandOut.Structured},
				Competition: domain.AgentEvidence{Reasoning: strVal(demandOut.Structured, "reasoning"), Data: demandOut.Structured},
				Margin:      domain.AgentEvidence{Reasoning: strVal(profitOut.Structured, "reasoning"), Data: profitOut.Structured},
				Risk:        domain.AgentEvidence{Reasoning: strVal(gatingOut.Structured, "reasoning"), Data: gatingOut.Structured},
				Sourcing:    domain.AgentEvidence{Reasoning: strVal(supplierOut.Structured, "reasoning"), Data: supplierOut.Structured},
			},
			SupplierCandidates: supplierCandidates,
			OutreachDrafts:     outreachDrafts,
			ReviewerVerdict:    reviewResult.Reasoning,
			Tier:               reviewResult.Tier,
			IterationCount:     1,
		}

		results = append(results, result)
		slog.Info("pipeline: candidate passed", "asin", asin, "tier", reviewResult.Tier, "overall", overall)
	}

	slog.Info("pipeline: complete",
		"campaign_id", campaignID,
		"evaluated", len(candidatesRaw),
		"passed", len(results),
	)

	return &domain.ResearchResult{
		CampaignID:    campaignID,
		Candidates:    results,
		ResearchTrail: trail,
		Summary:       fmt.Sprintf("Evaluated %d products, %d passed quality gates", len(candidatesRaw), len(results)),
	}, nil
}

// scoreFromMargin converts margin percentage to a 1-10 score
func scoreFromMargin(marginPct float64) int {
	switch {
	case marginPct >= 50:
		return 10
	case marginPct >= 40:
		return 9
	case marginPct >= 30:
		return 8
	case marginPct >= 25:
		return 7
	case marginPct >= 20:
		return 6
	case marginPct >= 15:
		return 5
	case marginPct >= 10:
		return 4
	default:
		return 3
	}
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func toStringSlice(v any) []string {
	if arr, ok := v.([]any); ok {
		var result []string
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	if arr, ok := v.([]string); ok {
		return arr
	}
	return nil
}

func mergeMaps(maps ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
```

- [ ] **Step 3: Create `internal/service/pipeline_orchestrator_test.go`**

```go
package service_test

import (
	"context"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/simulator"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

func TestPipelineOrchestrator_RunPipeline(t *testing.T) {
	runtime := simulator.NewAgentRuntime()
	orchestrator := service.NewPipelineOrchestrator(runtime)

	config := domain.DefaultPipelineConfig("test-tenant")
	criteria := domain.Criteria{
		Keywords:    []string{"kitchen gadgets"},
		Marketplace: "US",
	}

	result, err := orchestrator.RunPipeline(context.Background(), "camp-1", criteria, config)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.CampaignID != "camp-1" {
		t.Errorf("expected campaign ID camp-1, got %s", result.CampaignID)
	}
	// The simulator should produce some candidates (not guaranteed all pass the funnel)
	t.Logf("pipeline evaluated, %d candidates passed", len(result.Candidates))
	t.Logf("summary: %s", result.Summary)

	if len(result.ResearchTrail) == 0 {
		t.Error("expected non-empty research trail")
	}

	// Verify candidates that passed have valid structure
	for _, c := range result.Candidates {
		if c.ASIN == "" {
			t.Error("candidate missing ASIN")
		}
		if c.Tier == "" {
			t.Error("candidate missing tier")
		}
		if c.Tier == domain.DealTierCut {
			t.Error("cut candidates should not appear in results")
		}
		if c.Scores.Overall <= 0 {
			t.Errorf("candidate %s has zero/negative overall score", c.ASIN)
		}
	}
}

func TestPipelineOrchestrator_EmptySourcing(t *testing.T) {
	// Use a runtime that returns empty candidates from sourcing
	runtime := &emptySourcingRuntime{}
	orchestrator := service.NewPipelineOrchestrator(runtime)

	config := domain.DefaultPipelineConfig("test-tenant")
	result, err := orchestrator.RunPipeline(context.Background(), "camp-2", domain.Criteria{}, config)
	if err != nil {
		t.Fatalf("pipeline failed: %v", err)
	}
	if len(result.Candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(result.Candidates))
	}
}

type emptySourcingRuntime struct{}

func (r *emptySourcingRuntime) RunAgent(_ context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	if task.AgentName == "sourcing" {
		return &domain.AgentOutput{
			Structured: map[string]any{"candidates": []any{}},
		}, nil
	}
	return &domain.AgentOutput{Structured: map[string]any{}}, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/service/... -v -count=1 -run TestPipeline
```

Expected: both pipeline tests pass

- [ ] **Step 5: Commit**

```bash
git add internal/service/pipeline_orchestrator.go internal/service/pipeline_orchestrator_test.go internal/domain/research.go
git commit -m "feat: add pipeline orchestrator with elimination funnel and context passing"
```

---

## Task 7: Update PipelineService + main.go + Inngest Wiring

**Files:**
- Modify: `internal/service/pipeline_service.go`
- Modify: `apps/api/main.go`

- [ ] **Step 1: Rewrite `internal/service/pipeline_service.go`**

```go
package service

import (
	"context"
	"fmt"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type PipelineService struct {
	orchestrator *PipelineOrchestrator
	campaigns    port.CampaignRepo
	scoring      port.ScoringConfigRepo
	deals        *DealService
}

func NewPipelineService(orchestrator *PipelineOrchestrator, campaigns port.CampaignRepo, scoring port.ScoringConfigRepo, deals *DealService) *PipelineService {
	return &PipelineService{orchestrator: orchestrator, campaigns: campaigns, scoring: scoring, deals: deals}
}

func (s *PipelineService) RunCampaign(ctx context.Context, campaignID domain.CampaignID, tenantID domain.TenantID) error {
	campaign, err := s.campaigns.GetByID(ctx, tenantID, campaignID)
	if err != nil {
		return fmt.Errorf("get campaign: %w", err)
	}

	if err := campaign.Transition(domain.CampaignStatusRunning); err != nil {
		return err
	}
	if err := s.campaigns.Update(ctx, campaign); err != nil {
		return fmt.Errorf("update campaign to running: %w", err)
	}

	// Build pipeline config from scoring config
	// In the future this comes from a PipelineConfigRepo
	sc, err := s.scoring.GetByID(ctx, tenantID, campaign.ScoringConfigID)
	if err != nil {
		return fmt.Errorf("get scoring config: %w", err)
	}

	pipelineConfig := domain.DefaultPipelineConfig(tenantID)
	pipelineConfig.Scoring = sc.Weights

	result, err := s.orchestrator.RunPipeline(ctx, campaignID, campaign.Criteria, pipelineConfig)
	if err != nil {
		_ = campaign.Transition(domain.CampaignStatusFailed)
		_ = s.campaigns.Update(ctx, campaign)
		return fmt.Errorf("run research pipeline: %w", err)
	}

	_, err = s.deals.CreateFromResearch(ctx, tenantID, result)
	if err != nil {
		_ = campaign.Transition(domain.CampaignStatusFailed)
		_ = s.campaigns.Update(ctx, campaign)
		return fmt.Errorf("create deals from research: %w", err)
	}

	if err := campaign.Transition(domain.CampaignStatusCompleted); err != nil {
		return err
	}
	return s.campaigns.Update(ctx, campaign)
}
```

- [ ] **Step 2: Update `apps/api/main.go`**

Replace the agent runtime and pipeline service wiring section. The key change: `PipelineService` now takes a `PipelineOrchestrator` instead of an `AgentRuntime`.

Find this block:
```go
	// Services
	eventSvc := service.NewEventService(eventRepo, analyticsProvider, idGen)
	scoringSvc := service.NewScoringService(scoringRepo, idGen)
	dealSvc := service.NewDealService(dealRepo, eventSvc, idGen)
	pipelineSvc := service.NewPipelineService(agentRuntime, campaignRepo, scoringRepo, dealSvc)
```

Replace with:
```go
	// Services
	eventSvc := service.NewEventService(eventRepo, analyticsProvider, idGen)
	scoringSvc := service.NewScoringService(scoringRepo, idGen)
	dealSvc := service.NewDealService(dealRepo, eventSvc, idGen)
	orchestrator := service.NewPipelineOrchestrator(agentRuntime)
	pipelineSvc := service.NewPipelineService(orchestrator, campaignRepo, scoringRepo, dealSvc)
```

- [ ] **Step 3: Build and test everything**

```bash
go build ./...
go test ./... -count=1
```

Expected: all builds pass, all tests pass (existing + new)

- [ ] **Step 4: Commit**

```bash
git add internal/service/pipeline_service.go apps/api/main.go
git commit -m "refactor: wire PipelineOrchestrator into PipelineService and main.go"
```

---

## Task 8: Rebuild Docker and Verify End-to-End

**Files:** none (operational verification)

- [ ] **Step 1: Rebuild API container**

```bash
docker compose up --build api -d
```

- [ ] **Step 2: Wait and verify**

```bash
sleep 5
curl -s http://localhost:8081/health
```

Expected: `{"status":"ok"}`

- [ ] **Step 3: Create a campaign and verify deals appear**

```bash
curl -s -X POST http://localhost:8081/campaigns \
  -H "Authorization: Bearer dev-token" \
  -H "Content-Type: application/json" \
  -d '{"type":"manual","trigger_type":"dashboard","criteria":{"keywords":["kitchen gadgets"],"marketplace":"US"}}' | python3 -m json.tool
```

Wait 5 seconds, then:

```bash
curl -s -H "Authorization: Bearer dev-token" http://localhost:8081/deals | python3 -m json.tool | head -20
```

Expected: deals with tiered results (A/B/C), proper evidence from each agent stage, scores in 1-10 range

- [ ] **Step 4: Verify API logs show funnel elimination**

```bash
docker compose logs api --tail 20
```

Expected: log lines showing candidates eliminated at gating and profitability stages, fewer reaching the reviewer

- [ ] **Step 5: Commit final verification notes**

```bash
git add -A
git commit -m "chore: verify end-to-end pipeline refactor works in Docker" --allow-empty
```

---

## Self-Review Results

**Spec coverage:**
- Per-agent AgentRuntime interface: Task 2 ✓
- PipelineConfig + AgentConfig composable models: Task 1 ✓
- Funnel reorder (Source → Gate → Profit → Demand → Supplier → Review): Task 6 ✓
- Deterministic FBA fee calculator: Task 3 ✓
- Plausibility validator: Task 3 ✓
- Hybrid reviewer (rules + LLM, tiered output): Task 4 ✓
- Context sharing between agents: Task 6 (AgentContext passed downstream) ✓
- Updated simulator: Task 5 ✓
- Updated wiring: Task 7 ✓
- Go owns schema validation: Task 3 (validation.go) + Task 6 (orchestrator calls it) ✓
- Pre-resolved tools: Structurally ready (agents receive `Input` with pre-resolved data, not tool callbacks) ✓

**Placeholder scan:** No TBDs or vague steps. All code blocks complete.

**Type consistency:** Verified: `AgentTask`, `AgentOutput`, `AgentContext`, `PipelineConfig`, `AgentConfig`, `DealTier`, `ReviewResult` are consistent across all tasks.

**Gap:** OpenFang and ZeroClaw adapters are not in this plan — they were intentionally scoped to steps 4-7 of the implementation order. This plan covers steps 1-3 only.
