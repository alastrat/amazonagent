package domain

// AgentRegistry maps agent names to their typed definitions.
// All pipeline agents + the concierge are registered here.
var AgentRegistry = map[string]AgentDefinition{
	"gating": {
		Name:        "gating",
		Description: "Evaluates IP risk, brand restrictions, category gating, hazmat flags",
		ModelTier:   ModelTierFast,
		Tools:       []string{"sp_api_restrictions", "brand_database"},
		MaxRetries:  1,
		TimeoutSec:  30,
		MaxTurns:    1, // single-shot
	},
	"profitability": {
		Name:        "profitability",
		Description: "Evaluates margin viability with qualitative LLM assessment",
		ModelTier:   ModelTierStandard,
		Tools:       []string{"sp_api_fees", "price_history"},
		MaxRetries:  1,
		TimeoutSec:  45,
		MaxTurns:    1,
	},
	"demand": {
		Name:        "demand",
		Description: "Scores sales velocity, BSR trends, buy box dynamics",
		ModelTier:   ModelTierStandard,
		Tools:       []string{"keepa_data", "price_history"},
		MaxRetries:  1,
		TimeoutSec:  45,
		MaxTurns:    1,
	},
	"supplier": {
		Name:        "supplier",
		Description: "Discovers wholesale suppliers and pricing via web search",
		ModelTier:   ModelTierStandard,
		Tools:       []string{"web_search", "firecrawl"},
		MaxRetries:  1,
		TimeoutSec:  60,
		MaxTurns:    1,
	},
	"reviewer": {
		Name:          "reviewer",
		Description:   "Final scoring, tiering (A/B/C/Cut), and recommendation",
		ModelTier:     ModelTierReasoning,
		Tools:         []string{},
		MaxRetries:    1,
		TimeoutSec:    60,
		MaxTurns:      2,           // can self-refine once
		CanSelfRefine: true,
	},
	"concierge": {
		Name:        "concierge",
		Description: "Always-on AI concierge for FBA wholesale sellers",
		ModelTier:   ModelTierReasoning,
		Tools: []string{
			"search_products", "check_eligibility", "get_pricing",
			"get_strategy", "query_catalog", "create_suggestion",
		},
		MaxRetries:    0,  // chat doesn't retry — user can re-ask
		TimeoutSec:    120,
		MaxTurns:      5,  // ReAct: reason → tool → observe → repeat
		CanSelfRefine: true,
	},
}

// GetAgentDefinition returns the definition for an agent by name, or nil if not found.
func GetAgentDefinition(name string) *AgentDefinition {
	def, ok := AgentRegistry[name]
	if !ok {
		return nil
	}
	return &def
}
