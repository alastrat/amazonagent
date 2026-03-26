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
	output := map[string]any{"net_margin_pct": 600.0}
	errs := ValidateAgentOutput("profitability", output)
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestValidateAgentOutput_Demand_InvalidBSR(t *testing.T) {
	output := map[string]any{"bsr_rank": -5}
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
		"opportunity_viability": 12,
		"execution_confidence":  0,
		"sourcing_feasibility":  8,
	}
	errs := ValidateAgentOutput("reviewer", output)
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}
