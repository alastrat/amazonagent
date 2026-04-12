package service

import (
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

func TestSingleShot_AlwaysReturnsTrue(t *testing.T) {
	output := &domain.AgentOutput{
		Structured: map[string]any{"confidence": 50.0},
		Raw:        "some output",
	}
	if !SingleShot(output) {
		t.Error("SingleShot should always return true")
	}
	if !SingleShot(nil) {
		t.Error("SingleShot should return true even for nil output")
	}
}

func TestConfidenceThreshold_AboveThreshold(t *testing.T) {
	stop := ConfidenceThreshold(80)
	output := &domain.AgentOutput{
		Structured: map[string]any{"confidence": 90.0},
	}
	if !stop(output) {
		t.Error("expected stop=true when confidence (90) >= threshold (80)")
	}
}

func TestConfidenceThreshold_BelowThreshold(t *testing.T) {
	stop := ConfidenceThreshold(80)
	output := &domain.AgentOutput{
		Structured: map[string]any{"confidence": 70.0},
	}
	if stop(output) {
		t.Error("expected stop=false when confidence (70) < threshold (80)")
	}
}

func TestConfidenceThreshold_MissingField_ReturnsTrue(t *testing.T) {
	stop := ConfidenceThreshold(80)
	output := &domain.AgentOutput{
		Structured: map[string]any{"other_field": "value"},
	}
	if !stop(output) {
		t.Error("expected stop=true when confidence field is missing (backwards compat)")
	}
}

func TestAllFieldsPresent_AllPresent(t *testing.T) {
	stop := AllFieldsPresent([]string{"name", "score"})
	output := &domain.AgentOutput{
		Structured: map[string]any{
			"name":  "Widget",
			"score": 85.0,
		},
	}
	if !stop(output) {
		t.Error("expected stop=true when all required fields are present and non-zero")
	}
}

func TestAllFieldsPresent_MissingField(t *testing.T) {
	stop := AllFieldsPresent([]string{"name", "score"})
	output := &domain.AgentOutput{
		Structured: map[string]any{
			"name": "Widget",
		},
	}
	if stop(output) {
		t.Error("expected stop=false when a required field is missing")
	}
}

func TestAllFieldsPresent_EmptyString(t *testing.T) {
	stop := AllFieldsPresent([]string{"name"})
	output := &domain.AgentOutput{
		Structured: map[string]any{
			"name": "",
		},
	}
	if stop(output) {
		t.Error("expected stop=false when a string field is empty")
	}
}

func TestAllFieldsPresent_ZeroNumber(t *testing.T) {
	stop := AllFieldsPresent([]string{"score"})
	output := &domain.AgentOutput{
		Structured: map[string]any{
			"score": 0.0,
		},
	}
	if stop(output) {
		t.Error("expected stop=false when a numeric field is zero")
	}
}

func TestAllFieldsPresent_BoolField_AlwaysPresent(t *testing.T) {
	stop := AllFieldsPresent([]string{"active"})

	outputTrue := &domain.AgentOutput{
		Structured: map[string]any{"active": true},
	}
	if !stop(outputTrue) {
		t.Error("expected stop=true when bool field is true")
	}

	outputFalse := &domain.AgentOutput{
		Structured: map[string]any{"active": false},
	}
	if !stop(outputFalse) {
		t.Error("expected stop=true when bool field is false (bools are always present)")
	}
}
