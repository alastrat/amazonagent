package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

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
		"net_margin_pct": 5.0,
	}

	result, err := reviewer.Review(
		context.Background(), candidate, nil,
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
		context.Background(), candidate, nil,
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
		context.Background(), candidate, nil,
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
		context.Background(), candidate, nil,
		domain.AgentConfig{SystemPrompt: "test"},
		domain.DefaultPipelineThresholds(),
		domain.DefaultScoringWeights(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tier != domain.DealTierB {
		t.Errorf("expected B-tier fallback, got %s", result.Tier)
	}
}
