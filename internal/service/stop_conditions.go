package service

import "github.com/pluriza/fba-agent-orchestrator/internal/domain"

// SingleShot always stops after one turn (default for most pipeline agents).
func SingleShot(_ *domain.AgentOutput) bool {
	return true
}

// ConfidenceThreshold stops when the agent reports confidence above a threshold.
func ConfidenceThreshold(threshold int) func(*domain.AgentOutput) bool {
	return func(output *domain.AgentOutput) bool {
		confidence, ok := output.Structured["confidence"].(float64)
		if !ok {
			return true // stop if no confidence field (backwards compat)
		}
		return int(confidence) >= threshold
	}
}

// AllFieldsPresent stops when all required output fields are non-zero/non-empty.
func AllFieldsPresent(fields []string) func(*domain.AgentOutput) bool {
	return func(output *domain.AgentOutput) bool {
		for _, field := range fields {
			val, ok := output.Structured[field]
			if !ok || val == nil {
				return false
			}
			// Check for zero values
			switch v := val.(type) {
			case string:
				if v == "" {
					return false
				}
			case float64:
				if v == 0 {
					return false
				}
			case bool:
				// bools are always "present"
			}
		}
		return true
	}
}
