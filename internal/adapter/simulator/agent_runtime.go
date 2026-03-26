package simulator

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type AgentRuntime struct{}

func NewAgentRuntime() *AgentRuntime {
	return &AgentRuntime{}
}

func (r *AgentRuntime) RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error) {
	time.Sleep(time.Duration(50+rand.Intn(200)) * time.Millisecond)

	start := time.Now()
	var structured map[string]any

	switch task.AgentName {
	case "sourcing":
		structured = simulateSourcing(task.Input)
	case "gating":
		structured = simulateGating(task.Input)
	case "profitability":
		structured = simulateProfitability(task.Input)
	case "demand":
		structured = simulateDemand(task.Input)
	case "supplier":
		structured = simulateSupplier(task.Input)
	case "reviewer":
		structured = simulateReviewer(task.Input)
	default:
		return nil, fmt.Errorf("unknown agent: %s", task.AgentName)
	}

	return &domain.AgentOutput{
		Structured: structured,
		Raw:        fmt.Sprintf("simulated %s output", task.AgentName),
		TokensUsed: 100 + rand.Intn(900),
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

func simulateSourcing(input map[string]any) map[string]any {
	candidates := []any{}
	products := []struct{ ASIN, Title, Brand, Category string }{
		{"B0CX23V5KK", "Stainless Steel Kitchen Utensil Set", "HomeChef Pro", "Kitchen"},
		{"B0D1FG89NM", "Silicone Baking Mat Set (3 Pack)", "BakeRight", "Kitchen"},
		{"B0BY7K3PQR", "Bamboo Cutting Board with Juice Groove", "EcoBoard", "Kitchen"},
		{"B0CR9T23YZ", "Adjustable Dumbbell Set 25lb", "FitCore", "Fitness"},
		{"B0D5HJ78AB", "Resistance Band Set with Door Anchor", "FlexFit", "Fitness"},
		{"B0DQ6N45EF", "Wireless Earbuds Noise Cancelling", "SoundPeak", "Electronics"},
		{"B0CT3P78GH", "Portable Phone Charger 20000mAh", "JuiceBox", "Electronics"},
		{"B0D8KR12IJ", "LED Desk Lamp with USB Charging", "BrightWork", "Office"},
		{"B0CW7T56MN", "Stainless Steel Water Bottle 32oz", "HydroKeep", "Sports"},
		{"B0DJ9U78OP", "Microfiber Cleaning Cloth 12-Pack", "CleanPro", "Home"},
	}

	n := 5 + rand.Intn(6)
	if n > len(products) {
		n = len(products)
	}
	perm := rand.Perm(len(products))
	for i := 0; i < n; i++ {
		p := products[perm[i]]
		candidates = append(candidates, map[string]any{
			"asin":         p.ASIN,
			"title":        p.Title,
			"brand":        p.Brand,
			"category":     p.Category,
			"amazon_price": 15.0 + rand.Float64()*60.0,
			"bsr_rank":     1000 + rand.Intn(50000),
			"seller_count": 2 + rand.Intn(20),
		})
	}
	return map[string]any{"candidates": candidates}
}

func simulateGating(input map[string]any) map[string]any {
	passed := rand.Float64() > 0.40
	riskScore := 1 + rand.Intn(10)
	flags := []any{}
	if !passed {
		flags = append(flags, []string{"category_gated", "ip_risk", "brand_restricted", "hazmat"}[rand.Intn(4)])
	}
	return map[string]any{
		"passed":     passed,
		"risk_score": riskScore,
		"flags":      flags,
		"reasoning":  fmt.Sprintf("Gating assessment: passed=%v, risk_score=%d", passed, riskScore),
	}
}

func simulateProfitability(input map[string]any) map[string]any {
	amazonPrice := 20.0 + rand.Float64()*50.0
	wholesaleCost := amazonPrice * (0.3 + rand.Float64()*0.4)
	fees := domain.CalculateFBAFees(amazonPrice, wholesaleCost, 0.5+rand.Float64()*3.0, false)

	return map[string]any{
		"amazon_price":   amazonPrice,
		"wholesale_cost": wholesaleCost,
		"net_margin_pct": fees.NetMarginPct,
		"roi_pct":        fees.ROIPct,
		"net_profit":     fees.NetProfit,
		"total_fees":     fees.TotalFees,
		"referral_fee":   fees.ReferralFee,
		"fba_fee":        fees.FBAFulfillment,
		"reasoning":      fmt.Sprintf("Margin: %.1f%%, ROI: %.0f%%", fees.NetMarginPct, fees.ROIPct),
	}
}

func simulateDemand(input map[string]any) map[string]any {
	return map[string]any{
		"demand_score":      6 + rand.Intn(5),
		"competition_score": 5 + rand.Intn(6),
		"bsr_rank":          1000 + rand.Intn(50000),
		"monthly_units":     100 + rand.Intn(5000),
		"fba_sellers":       2 + rand.Intn(20),
		"buy_box_share":     0.1 + rand.Float64()*0.5,
		"trend":             []string{"improving", "stable", "declining"}[rand.Intn(3)],
		"reasoning":         "Demand analysis based on BSR trends, velocity, and competition landscape.",
	}
}

func simulateSupplier(input map[string]any) map[string]any {
	wholesaleCost := 10.0 + rand.Float64()*30.0
	numSuppliers := 1 + rand.Intn(4)
	suppliers := []any{}
	for i := 0; i < numSuppliers; i++ {
		suppliers = append(suppliers, map[string]any{
			"company":        fmt.Sprintf("Supplier %c Distribution", rune('A'+i)),
			"unit_price":     wholesaleCost * (0.9 + rand.Float64()*0.2),
			"moq":            24 + rand.Intn(200),
			"lead_time_days": 5 + rand.Intn(25),
			"authorized":     rand.Float64() > 0.3,
		})
	}
	return map[string]any{
		"suppliers":      suppliers,
		"best_price":     wholesaleCost * 0.95,
		"outreach_draft": "Hi, I'm interested in wholesale pricing for this product. We're an established Amazon FBA seller...",
		"reasoning":      fmt.Sprintf("Found %d suppliers, best price: $%.2f", numSuppliers, wholesaleCost*0.95),
	}
}

func simulateReviewer(input map[string]any) map[string]any {
	ov := 5 + rand.Intn(6)
	ec := 5 + rand.Intn(6)
	sf := 5 + rand.Intn(6)
	return map[string]any{
		"opportunity_viability": ov,
		"execution_confidence":  ec,
		"sourcing_feasibility":  sf,
		"reasoning":             fmt.Sprintf("Scores: OV=%d, EC=%d, SF=%d", ov, ec, sf),
	}
}
