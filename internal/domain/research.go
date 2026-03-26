package domain

type ResearchResult struct {
	CampaignID    CampaignID        `json:"campaign_id"`
	Candidates    []CandidateResult `json:"candidates"`
	ResearchTrail []AgentTrailEntry `json:"research_trail"`
	Summary       string            `json:"summary"`
}

type CandidateResult struct {
	ASIN               string              `json:"asin"`
	Title              string              `json:"title"`
	Brand              string              `json:"brand"`
	Category           string              `json:"category"`
	Scores             DealScores          `json:"scores"`
	Evidence           Evidence            `json:"evidence"`
	SupplierCandidates []SupplierCandidate `json:"supplier_candidates"`
	OutreachDrafts     []string            `json:"outreach_drafts"`
	ReviewerVerdict    string              `json:"reviewer_verdict"`
	IterationCount     int                 `json:"iteration_count"`
}

type SupplierCandidate struct {
	Company       string  `json:"company"`
	Contact       string  `json:"contact"`
	UnitPrice     float64 `json:"unit_price"`
	MOQ           int     `json:"moq"`
	LeadTimeDays  int     `json:"lead_time_days"`
	ShippingTerms string  `json:"shipping_terms"`
	Authorized    bool    `json:"authorized"`
}

type AgentTrailEntry struct {
	AgentName  string `json:"agent_name"`
	ASIN       string `json:"asin"`
	Iteration  int    `json:"iteration"`
	Input      any    `json:"input"`
	Output     any    `json:"output"`
	DurationMs int64  `json:"duration_ms"`
}
