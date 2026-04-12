package domain

import (
	"testing"
)

func TestParseTypedOutput_GatingOutput(t *testing.T) {
	output := &AgentOutput{
		Structured: map[string]any{
			"passed":     true,
			"risk_score": 0.3,
			"flags":      []any{"hazmat"},
			"reasoning":  "Low risk product",
		},
	}

	result, err := ParseTypedOutput[GatingOutput](output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if result.RiskScore != 0.3 {
		t.Errorf("RiskScore = %v, want 0.3", result.RiskScore)
	}
	if len(result.Flags) != 1 || result.Flags[0] != "hazmat" {
		t.Errorf("Flags = %v, want [hazmat]", result.Flags)
	}
	if result.Reasoning != "Low risk product" {
		t.Errorf("Reasoning = %q, want %q", result.Reasoning, "Low risk product")
	}
}

func TestParseTypedOutput_NilOutput_ReturnsZeroValue(t *testing.T) {
	result, err := ParseTypedOutput[GatingOutput](nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Passed {
		t.Error("expected Passed=false for zero value")
	}
	if result.RiskScore != 0 {
		t.Errorf("RiskScore = %v, want 0", result.RiskScore)
	}
	if result.Reasoning != "" {
		t.Errorf("Reasoning = %q, want empty", result.Reasoning)
	}
}

func TestParseTypedOutput_ConciergeOutput_NestedToolCalls(t *testing.T) {
	output := &AgentOutput{
		Structured: map[string]any{
			"message":    "Here are the results",
			"confidence": 8.0,
			"tool_calls": []any{
				map[string]any{
					"tool":  "search_products",
					"input": map[string]any{"keywords": "widget"},
					"result": map[string]any{
						"count": 5.0,
					},
				},
			},
			"suggestions":  []any{"Try narrowing by category"},
			"action_needed": false,
		},
	}

	result, err := ParseTypedOutput[ConciergeOutput](output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Message != "Here are the results" {
		t.Errorf("Message = %q, want %q", result.Message, "Here are the results")
	}
	if result.Confidence != 8 {
		t.Errorf("Confidence = %d, want 8", result.Confidence)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("ToolCalls length = %d, want 1", len(result.ToolCalls))
	}
	if result.ToolCalls[0].Tool != "search_products" {
		t.Errorf("ToolCalls[0].Tool = %q, want %q", result.ToolCalls[0].Tool, "search_products")
	}
	if len(result.Suggestions) != 1 || result.Suggestions[0] != "Try narrowing by category" {
		t.Errorf("Suggestions = %v, want [Try narrowing by category]", result.Suggestions)
	}
}

func TestParseTypedOutput_IgnoresUnknownFields(t *testing.T) {
	output := &AgentOutput{
		Structured: map[string]any{
			"passed":        true,
			"risk_score":    0.1,
			"flags":         []any{},
			"reasoning":     "Clean",
			"unknown_field": "should be ignored",
			"extra_number":  42.0,
		},
	}

	result, err := ParseTypedOutput[GatingOutput](output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Passed {
		t.Error("expected Passed=true")
	}
	if result.Reasoning != "Clean" {
		t.Errorf("Reasoning = %q, want %q", result.Reasoning, "Clean")
	}
}

func TestGetAgentDefinition_Concierge(t *testing.T) {
	def := GetAgentDefinition("concierge")
	if def == nil {
		t.Fatal("expected non-nil definition for concierge")
	}
	if def.Name != "concierge" {
		t.Errorf("Name = %q, want %q", def.Name, "concierge")
	}
	if def.ModelTier != ModelTierReasoning {
		t.Errorf("ModelTier = %q, want %q", def.ModelTier, ModelTierReasoning)
	}
	if def.MaxTurns != 5 {
		t.Errorf("MaxTurns = %d, want 5", def.MaxTurns)
	}
	if !def.CanSelfRefine {
		t.Error("expected CanSelfRefine=true for concierge")
	}
	if len(def.Tools) == 0 {
		t.Error("expected concierge to have tools")
	}
}

func TestGetAgentDefinition_Nonexistent(t *testing.T) {
	def := GetAgentDefinition("nonexistent")
	if def != nil {
		t.Errorf("expected nil for nonexistent agent, got %+v", def)
	}
}
