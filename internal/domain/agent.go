package domain

// AgentTask is the input to a single agent execution.
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
type AgentContext struct {
	AgentName string         `json:"agent_name"`
	Facts     map[string]any `json:"facts"`
	Flags     []string       `json:"flags,omitempty"`
}
