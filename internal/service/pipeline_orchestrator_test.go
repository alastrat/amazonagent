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
	orchestrator := service.NewPipelineOrchestrator(runtime, nil)

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
	t.Logf("pipeline: %d candidates passed out of evaluated", len(result.Candidates))
	t.Logf("summary: %s", result.Summary)

	if len(result.ResearchTrail) == 0 {
		t.Error("expected non-empty research trail")
	}

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
	runtime := &emptySourcingRuntime{}
	orchestrator := service.NewPipelineOrchestrator(runtime, nil)

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
