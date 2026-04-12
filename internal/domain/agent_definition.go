package domain

import "encoding/json"

// ModelTier controls which model quality/speed tradeoff an agent uses.
type ModelTier string

const (
	ModelTierFast      ModelTier = "fast"      // gating, pre-filters — cheapest, fastest
	ModelTierStandard  ModelTier = "standard"  // demand, supplier, profitability — balanced
	ModelTierReasoning ModelTier = "reasoning" // reviewer, coordinator, concierge — highest quality
)

// AgentDefinition is a typed schema for an agent — replaces the dead-field AgentConfig.
// Every agent in the registry has an explicit definition with model tier, tools, timeout,
// output schema, hooks, and loop configuration.
type AgentDefinition struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	SystemPrompt string    `json:"system_prompt"`
	ModelTier    ModelTier `json:"model_tier"`
	Tools        []string  `json:"tools"`
	MaxRetries   int       `json:"max_retries"`
	TimeoutSec   int       `json:"timeout_sec"`

	// ReAct loop configuration
	MaxTurns      int  `json:"max_turns"`       // 1 = single-shot (default), >1 = ReAct loop
	CanSelfRefine bool `json:"can_self_refine"` // agent can critique and retry its own output

	// Hooks — set at registration, not serialized
	Hooks *AgentHooks `json:"-"`

	// StopCondition — checked after each turn in the ReAct loop
	StopCondition func(output *AgentOutput) bool `json:"-"`
}

// ParseTypedOutput parses an untyped agent output into a typed struct.
// Returns the typed output and any error. This replaces unsafe map[string]any assertions.
func ParseTypedOutput[T any](output *AgentOutput) (T, error) {
	var result T
	if output == nil {
		return result, nil
	}
	// Marshal the structured map to JSON, then unmarshal into the typed struct
	b, err := json.Marshal(output.Structured)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return result, err
	}
	return result, nil
}
