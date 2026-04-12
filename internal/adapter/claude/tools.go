package claude

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// ConciergeTool defines a tool the concierge can call.
type ConciergeTool struct {
	Name        string
	Description string
	InputSchema map[string]any // JSON Schema for the tool input
	Execute     func(ctx context.Context, input json.RawMessage) (string, error)
}

// ToolDeps holds port-based dependencies for the concierge tools.
// main.go injects these — no concrete types leak outside the adapter.
type ToolDeps struct {
	ProductSearcher port.ProductSearcher
	Profiles        port.SellerProfileRepo
	Fingerprints    port.EligibilityFingerprintRepo
}

// ConciergeToolkit holds all available tools and their dependencies.
type ConciergeToolkit struct {
	tools    map[string]*ConciergeTool
	spapi    port.ProductSearcher
	profiles port.SellerProfileRepo
	fps      port.EligibilityFingerprintRepo
}

// NewConciergeToolkit creates the toolkit from port-based dependencies.
func NewConciergeToolkit(deps ToolDeps) *ConciergeToolkit {
	tk := &ConciergeToolkit{
		tools:    make(map[string]*ConciergeTool),
		spapi:    deps.ProductSearcher,
		profiles: deps.Profiles,
		fps:      deps.Fingerprints,
	}
	tk.register()
	return tk
}

func (tk *ConciergeToolkit) register() {
	tk.tools["get_assessment_summary"] = &ConciergeTool{
		Name:        "get_assessment_summary",
		Description: "Get a summary of the seller's assessment results — the same data shown on the onboarding page. Shows deduplicated counts of eligible, ungatable, and restricted products, category breakdown, and open brands. Always call this first to understand the seller's situation.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: tk.getAssessmentSummary,
	}

	tk.tools["get_eligible_products"] = &ConciergeTool{
		Name:        "get_eligible_products",
		Description: "Get the list of products the seller can list immediately on Amazon. Returns ASINs, titles, prices, margins, brands, and categories.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: tk.getEligibleProducts,
	}

	tk.tools["get_ungatable_products"] = &ConciergeTool{
		Name:        "get_ungatable_products",
		Description: "Get products that require approval but the seller CAN apply. Returns ASINs, brands, approval URLs, prices, and margins.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: tk.getUngatableProducts,
	}

	tk.tools["get_category_breakdown"] = &ConciergeTool{
		Name:        "get_category_breakdown",
		Description: "Get eligibility breakdown by Amazon category. Shows which categories are open, how many products were probed, and open rates.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: tk.getCategoryBreakdown,
	}

	tk.tools["search_products"] = &ConciergeTool{
		Name:        "search_products",
		Description: "Search Amazon for products by keywords. Returns ASINs, titles, prices, BSR ranks, and brands.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keywords": map[string]any{"type": "string", "description": "Search keywords"},
			},
			"required": []string{"keywords"},
		},
		Execute: tk.searchProducts,
	}

	tk.tools["check_eligibility"] = &ConciergeTool{
		Name:        "check_eligibility",
		Description: "Check if the seller can list a specific ASIN on Amazon. Returns eligibility status (eligible, ungatable, restricted) and approval URL if applicable.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"asin": map[string]any{"type": "string", "description": "The Amazon ASIN to check"},
			},
			"required": []string{"asin"},
		},
		Execute: tk.checkEligibility,
	}

	tk.tools["get_product_details"] = &ConciergeTool{
		Name:        "get_product_details",
		Description: "Get pricing and seller details for specific ASINs. Returns buy box price, seller count, and FBA fee estimates.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"asins": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "List of ASINs to look up"},
			},
			"required": []string{"asins"},
		},
		Execute: tk.getProductDetails,
	}

	tk.tools["get_seller_profile"] = &ConciergeTool{
		Name:        "get_seller_profile",
		Description: "Get the seller's profile including archetype classification, assessment status, and account details.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: tk.getSellerProfile,
	}
}

// GetTools returns all registered tools.
func (tk *ConciergeToolkit) GetTools() []*ConciergeTool {
	var tools []*ConciergeTool
	for _, t := range tk.tools {
		tools = append(tools, t)
	}
	return tools
}

// Execute runs a tool by name with the given input.
func (tk *ConciergeToolkit) Execute(ctx context.Context, name string, input json.RawMessage) (string, error) {
	tool, ok := tk.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, input)
}

// --- Tool implementations ---

// dedup returns products deduplicated by ASIN, sorted by margin descending.
func dedup(results []map[string]any) []map[string]any {
	seen := make(map[string]bool)
	var out []map[string]any
	for _, r := range results {
		asin, _ := r["asin"].(string)
		if asin == "" || seen[asin] {
			continue
		}
		seen[asin] = true
		out = append(out, r)
	}
	return out
}

func (tk *ConciergeToolkit) getAssessmentSummary(ctx context.Context, _ json.RawMessage) (string, error) {
	fp, err := tk.fps.Get(ctx, tk.tenantID(ctx))
	if err != nil {
		return "No assessment data available. Please complete onboarding first.", nil
	}

	// Deduplicate by ASIN — same logic as the onboarding page
	seen := make(map[string]bool)
	eligible, ungatable, restricted := 0, 0, 0
	brands := make(map[string]bool)

	for _, br := range fp.BrandResults {
		if seen[br.ASIN] {
			continue
		}
		seen[br.ASIN] = true
		switch br.EligibilityStatus {
		case "eligible":
			eligible++
		case "ungatable":
			ungatable++
		default:
			restricted++
		}
		if br.EligibilityStatus == "eligible" || br.EligibilityStatus == "ungatable" {
			brands[br.Brand] = true
		}
	}

	var cats []map[string]any
	for _, cat := range fp.Categories {
		cats = append(cats, map[string]any{
			"category":  cat.Category,
			"probed":    cat.ProbeCount,
			"open":      cat.OpenCount,
			"open_rate": cat.OpenRate,
		})
	}

	b, _ := json.Marshal(map[string]any{
		"total_products":      len(seen),
		"eligible_products":   eligible,
		"ungatable_products":  ungatable,
		"restricted_products": restricted,
		"open_brands":         len(brands),
		"categories_scanned":  len(fp.Categories),
		"overall_open_rate":   fp.OverallOpenRate,
		"categories":          cats,
	})
	return string(b), nil
}

func (tk *ConciergeToolkit) getEligibleProducts(ctx context.Context, _ json.RawMessage) (string, error) {
	fp, err := tk.fps.Get(ctx, tk.tenantID(ctx))
	if err != nil {
		return "No assessment data available. Please run an assessment first.", nil
	}
	var results []map[string]any
	for _, br := range fp.BrandResults {
		if br.EligibilityStatus == "eligible" {
			results = append(results, map[string]any{
				"asin":        br.ASIN,
				"title":       br.Title,
				"brand":       br.Brand,
				"category":    br.Category,
				"subcategory": br.Subcategory,
				"price":       br.Price,
				"margin_pct":  br.EstMarginPct,
				"sellers":     br.SellerCount,
			})
		}
	}
	results = dedup(results)
	b, _ := json.Marshal(map[string]any{"eligible_products": results, "count": len(results)})
	return string(b), nil
}

func (tk *ConciergeToolkit) getUngatableProducts(ctx context.Context, _ json.RawMessage) (string, error) {
	fp, err := tk.fps.Get(ctx, tk.tenantID(ctx))
	if err != nil {
		return "No assessment data available. Please run an assessment first.", nil
	}
	var results []map[string]any
	for _, br := range fp.BrandResults {
		if br.EligibilityStatus == "ungatable" {
			results = append(results, map[string]any{
				"asin":         br.ASIN,
				"title":        br.Title,
				"brand":        br.Brand,
				"category":     br.Category,
				"subcategory":  br.Subcategory,
				"price":        br.Price,
				"margin_pct":   br.EstMarginPct,
				"approval_url": br.ApprovalURL,
			})
		}
	}
	results = dedup(results)
	// Limit to top 20 to keep response fast — Claude can ask for more
	if len(results) > 20 {
		results = results[:20]
	}
	b, _ := json.Marshal(map[string]any{
		"ungatable_products": results,
		"count":             len(results),
		"note":              "Showing top 20 by margin. Ask for more if needed.",
	})
	return string(b), nil
}

func (tk *ConciergeToolkit) getCategoryBreakdown(ctx context.Context, _ json.RawMessage) (string, error) {
	fp, err := tk.fps.Get(ctx, tk.tenantID(ctx))
	if err != nil {
		return "No assessment data available.", nil
	}
	var cats []map[string]any
	for _, cat := range fp.Categories {
		cats = append(cats, map[string]any{
			"category":   cat.Category,
			"probed":     cat.ProbeCount,
			"open":       cat.OpenCount,
			"gated":      cat.GatedCount,
			"open_rate":  cat.OpenRate,
		})
	}
	b, _ := json.Marshal(map[string]any{
		"categories":       cats,
		"total_eligible":   fp.TotalEligible,
		"total_restricted": fp.TotalRestricted,
		"overall_open_rate": fp.OverallOpenRate,
	})
	return string(b), nil
}

func (tk *ConciergeToolkit) searchProducts(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Keywords string `json:"keywords"`
	}
	json.Unmarshal(input, &params)
	if params.Keywords == "" {
		return "Please provide keywords to search.", nil
	}
	if tk.spapi == nil {
		return "Product search is not available — SP-API not configured.", nil
	}
	products, err := tk.spapi.SearchProducts(ctx, []string{params.Keywords}, "US")
	if err != nil {
		return fmt.Sprintf("Search failed: %s", err), nil
	}
	var results []map[string]any
	for _, p := range products {
		results = append(results, map[string]any{
			"asin":     p.ASIN,
			"title":    p.Title,
			"brand":    p.Brand,
			"price":    p.AmazonPrice,
			"bsr_rank": p.BSRRank,
			"sellers":  p.SellerCount,
		})
	}
	b, _ := json.Marshal(map[string]any{"products": results, "count": len(results)})
	return string(b), nil
}

func (tk *ConciergeToolkit) checkEligibility(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ASIN string `json:"asin"`
	}
	json.Unmarshal(input, &params)
	if params.ASIN == "" {
		return "Please provide an ASIN.", nil
	}
	if tk.spapi == nil {
		return "Eligibility check not available — SP-API not configured.", nil
	}
	restrictions, err := tk.spapi.CheckListingEligibility(ctx, []string{params.ASIN}, "US")
	if err != nil {
		return fmt.Sprintf("Eligibility check failed: %s", err), nil
	}
	if len(restrictions) == 0 {
		return `{"status": "eligible", "message": "You can list this product immediately."}`, nil
	}
	r := restrictions[0]
	b, _ := json.Marshal(map[string]any{
		"asin":         r.ASIN,
		"status":       string(r.Status),
		"reason":       r.Reason,
		"reason_code":  r.ReasonCode,
		"approval_url": r.ApprovalURL,
	})
	return string(b), nil
}

func (tk *ConciergeToolkit) getProductDetails(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		ASINs []string `json:"asins"`
	}
	json.Unmarshal(input, &params)
	if len(params.ASINs) == 0 {
		return "Please provide at least one ASIN.", nil
	}
	if tk.spapi == nil {
		return "Product details not available — SP-API not configured.", nil
	}
	products, err := tk.spapi.GetProductDetails(ctx, params.ASINs, "US")
	if err != nil {
		return fmt.Sprintf("Product details failed: %s", err), nil
	}
	var results []map[string]any
	for _, p := range products {
		results = append(results, map[string]any{
			"asin":    p.ASIN,
			"title":   p.Title,
			"price":   p.AmazonPrice,
			"sellers": p.SellerCount,
			"brand":   p.Brand,
		})
	}
	b, _ := json.Marshal(map[string]any{"products": results})
	return string(b), nil
}

func (tk *ConciergeToolkit) getSellerProfile(ctx context.Context, _ json.RawMessage) (string, error) {
	profile, err := tk.profiles.Get(ctx, tk.tenantID(ctx))
	if err != nil {
		return "No seller profile found. Please complete onboarding first.", nil
	}
	b, _ := json.Marshal(map[string]any{
		"archetype":         string(profile.Archetype),
		"assessment_status": string(profile.AssessmentStatus),
		"assessed_at":       profile.AssessedAt,
	})
	return string(b), nil
}

// tenantID extracts the tenant ID from context (set by the chat service).
func (tk *ConciergeToolkit) tenantID(ctx context.Context) domain.TenantID {
	if id, ok := ctx.Value(tenantIDKey).(domain.TenantID); ok {
		return id
	}
	return ""
}

type contextKey string

const tenantIDKey contextKey = "tenant_id"

// WithTenantID injects the tenant ID into the context for tool execution.
func WithTenantID(ctx context.Context, tenantID domain.TenantID) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}
