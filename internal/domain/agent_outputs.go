package domain

// Typed output structs per agent stage — replaces map[string]any assertions.

// GatingOutput is returned by the gating agent.
type GatingOutput struct {
	Passed    bool     `json:"passed"`
	RiskScore float64  `json:"risk_score"`
	Flags     []string `json:"flags"`
	Reasoning string   `json:"reasoning"`
}

// ProfitabilityOutput is returned by the profitability agent.
type ProfitabilityOutput struct {
	MarginPercent    float64 `json:"margin_percent"`
	FBAFees          float64 `json:"fba_fees"`
	NetProfit        float64 `json:"net_profit"`
	QualitativeScore int     `json:"qualitative_score"` // 1-10
	Reasoning        string  `json:"reasoning"`
}

// DemandOutput is returned by the demand agent.
type DemandOutput struct {
	DemandScore      int    `json:"demand_score"`      // 1-10
	CompetitionScore int    `json:"competition_score"` // 1-10
	BuyBoxDynamics   string `json:"buy_box_dynamics"`
	Reasoning        string `json:"reasoning"`
}

// SupplierOutput is returned by the supplier agent.
type SupplierOutput struct {
	Suppliers     []SupplierInfo `json:"suppliers"`
	OutreachDraft string         `json:"outreach_draft"`
	Reasoning     string         `json:"reasoning"`
}

// SupplierInfo describes a single supplier found by the supplier agent.
type SupplierInfo struct {
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	MOQ      int     `json:"moq"`
	LeadDays int     `json:"lead_days"`
	Source   string  `json:"source"` // URL or description
}

// ReviewerOutput is returned by the reviewer agent.
type ReviewerOutput struct {
	OpportunityViability int     `json:"opportunity_viability"` // 1-10
	ExecutionConfidence  int     `json:"execution_confidence"`  // 1-10
	SourcingFeasibility  int     `json:"sourcing_feasibility"`  // 1-10
	CompositeScore       float64 `json:"composite_score"`
	Tier                 string  `json:"tier"` // A, B, C, Cut
	Reasoning            string  `json:"reasoning"`
}

// ConciergeOutput is returned by the concierge agent in chat mode.
type ConciergeOutput struct {
	Message       string              `json:"message"`
	ToolCalls     []ConciergeToolCall `json:"tool_calls,omitempty"`
	Suggestions   []string            `json:"suggestions,omitempty"` // follow-up suggestions for the user
	ActionNeeded  bool                `json:"action_needed"`         // true if agent is proposing an action
	ActionType    string              `json:"action_type,omitempty"` // suggest, critical
	Confidence    int                 `json:"confidence"`            // 1-10
}

// ConciergeToolCall records a tool invocation made by the concierge during a turn.
type ConciergeToolCall struct {
	Tool   string         `json:"tool"`
	Input  map[string]any `json:"input"`
	Result map[string]any `json:"result,omitempty"`
}
