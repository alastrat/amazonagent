package domain

import "fmt"

type PlausibilityError struct {
	Field   string
	Value   any
	Message string
}

func (e PlausibilityError) Error() string {
	return fmt.Sprintf("plausibility check failed: %s=%v — %s", e.Field, e.Value, e.Message)
}

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
