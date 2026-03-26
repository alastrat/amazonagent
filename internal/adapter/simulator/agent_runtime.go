// Package simulator provides a simulated AgentRuntime that generates realistic
// fake research results. Used for development and testing before OpenFang is connected.
package simulator

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type AgentRuntime struct{}

func NewAgentRuntime() *AgentRuntime {
	return &AgentRuntime{}
}

// product templates used to generate realistic candidates
var products = []struct {
	ASIN     string
	Title    string
	Brand    string
	Category string
}{
	{"B0CX23V5KK", "Stainless Steel Kitchen Utensil Set 12-Piece", "HomeChef Pro", "Kitchen"},
	{"B0D1FG89NM", "Silicone Baking Mat Set (3 Pack)", "BakeRight", "Kitchen"},
	{"B0BY7K3PQR", "Bamboo Cutting Board with Juice Groove", "EcoBoard", "Kitchen"},
	{"B0C2JN45ST", "Electric Milk Frother Handheld", "FrothMaster", "Kitchen"},
	{"B0DK8M12UV", "Collapsible Silicone Food Storage Containers", "FlexStore", "Kitchen"},
	{"B0BN4R67WX", "Cast Iron Skillet 10-Inch Pre-Seasoned", "IronForge", "Kitchen"},
	{"B0CR9T23YZ", "Adjustable Dumbbell Set 25lb", "FitCore", "Fitness"},
	{"B0D5HJ78AB", "Resistance Band Set with Door Anchor", "FlexFit", "Fitness"},
	{"B0CL2M90CD", "Yoga Mat Extra Thick 1/2 Inch", "ZenFlow", "Fitness"},
	{"B0DQ6N45EF", "Wireless Earbuds Noise Cancelling", "SoundPeak", "Electronics"},
	{"B0CT3P78GH", "Portable Phone Charger 20000mAh", "JuiceBox", "Electronics"},
	{"B0D8KR12IJ", "LED Desk Lamp with USB Charging Port", "BrightWork", "Office"},
	{"B0BV5S34KL", "Ergonomic Mouse Pad with Wrist Rest", "ComfortClick", "Office"},
	{"B0CW7T56MN", "Stainless Steel Water Bottle 32oz", "HydroKeep", "Sports"},
	{"B0DJ9U78OP", "Microfiber Cleaning Cloth 12-Pack", "CleanPro", "Home"},
}

func (r *AgentRuntime) RunResearchPipeline(ctx context.Context, input port.PipelineInput) (*domain.ResearchResult, error) {
	slog.Info("running simulated research pipeline",
		"campaign_id", input.CampaignID,
		"keywords", input.Criteria.Keywords,
	)

	// Simulate processing time (1-3 seconds)
	time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond)

	// Pick 3-7 random products as candidates
	numCandidates := 3 + rand.Intn(5)
	if numCandidates > len(products) {
		numCandidates = len(products)
	}

	// Shuffle and pick
	shuffled := make([]int, len(products))
	for i := range shuffled {
		shuffled[i] = i
	}
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	var candidates []domain.CandidateResult
	var trail []domain.AgentTrailEntry

	for i := 0; i < numCandidates; i++ {
		p := products[shuffled[i]]

		// Generate scores with some variance — most pass, some borderline
		demand := 6 + rand.Intn(5)         // 6-10
		competition := 5 + rand.Intn(6)    // 5-10
		margin := 6 + rand.Intn(5)         // 6-10
		risk := 5 + rand.Intn(6)           // 5-10
		sourcing := 6 + rand.Intn(5)       // 6-10

		w := input.ScoringConfig.Weights
		overall := float64(demand)*w.Demand +
			float64(competition)*w.Competition +
			float64(margin)*w.Margin +
			float64(risk)*w.Risk +
			float64(sourcing)*w.Sourcing

		// Scale to 1-10 range
		overall = overall * 10

		verdict := "PASS"
		iterations := 1
		if overall < 6.0 {
			verdict = "CUT"
		} else if overall < 7.5 {
			verdict = "PASS (borderline)"
			iterations = 2
		}

		wholesalePrice := 5.0 + rand.Float64()*25.0
		amazonPrice := wholesalePrice * (1.5 + rand.Float64()*1.0)
		netMargin := ((amazonPrice - wholesalePrice*1.15) / amazonPrice) * 100

		candidate := domain.CandidateResult{
			ASIN:     p.ASIN,
			Title:    p.Title,
			Brand:    p.Brand,
			Category: p.Category,
			Scores: domain.DealScores{
				Demand:              demand,
				Competition:         competition,
				Margin:              margin,
				Risk:                risk,
				SourcingFeasibility: sourcing,
				Overall:             overall,
			},
			Evidence: domain.Evidence{
				Demand: domain.AgentEvidence{
					Reasoning: fmt.Sprintf("Monthly estimated sales: %d units. BSR trend: improving over 90 days. Social sentiment: positive across Reddit and X.", 500+rand.Intn(9500)),
					Data: map[string]any{
						"monthly_units":  500 + rand.Intn(9500),
						"bsr_rank":       1000 + rand.Intn(50000),
						"trend":          "improving",
						"sentiment_score": 0.6 + rand.Float64()*0.4,
					},
				},
				Competition: domain.AgentEvidence{
					Reasoning: fmt.Sprintf("FBA seller count: %d. Buy box rotation: %s. Review velocity: %d/month. Category is %s.",
						3+rand.Intn(15),
						[]string{"low", "moderate", "high"}[rand.Intn(3)],
						10+rand.Intn(200),
						[]string{"ungated", "gated (easy approval)", "gated"}[rand.Intn(3)],
					),
					Data: map[string]any{
						"fba_sellers":    3 + rand.Intn(15),
						"buy_box_share":  0.1 + rand.Float64()*0.5,
						"review_count":   50 + rand.Intn(5000),
						"gated":          rand.Intn(3) == 0,
					},
				},
				Margin: domain.AgentEvidence{
					Reasoning: fmt.Sprintf("Wholesale: $%.2f → Amazon: $%.2f. After FBA fees + referral + shipping: net margin %.1f%%. ROI: %.0f%%. Break-even: %d units.",
						wholesalePrice, amazonPrice, netMargin, netMargin*2.5, 10+rand.Intn(50)),
					Data: map[string]any{
						"wholesale_cost": wholesalePrice,
						"amazon_price":   amazonPrice,
						"fba_fee":        amazonPrice * 0.15,
						"referral_fee":   amazonPrice * 0.15,
						"net_margin_pct": netMargin,
						"roi_pct":        netMargin * 2.5,
						"break_even":     10 + rand.Intn(50),
					},
				},
				Risk: domain.AgentEvidence{
					Reasoning: fmt.Sprintf("IP risk: %s. Brand authorization: %s. Listing quality: %s. No hazmat flags.",
						[]string{"low", "low", "moderate"}[rand.Intn(3)],
						[]string{"verified", "unverified", "pending"}[rand.Intn(3)],
						[]string{"good", "excellent", "needs improvement"}[rand.Intn(3)],
					),
					Data: map[string]any{
						"ip_risk":       []string{"low", "low", "moderate"}[rand.Intn(3)],
						"hazmat":        false,
						"listing_score": 7 + rand.Intn(4),
					},
				},
				Sourcing: domain.AgentEvidence{
					Reasoning: fmt.Sprintf("Found %d suppliers. Best price: $%.2f (MOQ: %d). Lead time: %d days. Authorization: %s.",
						2+rand.Intn(4), wholesalePrice, 24+rand.Intn(200), 5+rand.Intn(25),
						[]string{"authorized", "pending verification", "direct from manufacturer"}[rand.Intn(3)]),
					Data: map[string]any{
						"supplier_count": 2 + rand.Intn(4),
						"best_price":     wholesalePrice,
						"moq":            24 + rand.Intn(200),
						"lead_time_days": 5 + rand.Intn(25),
					},
				},
			},
			SupplierCandidates: []domain.SupplierCandidate{
				{
					Company:       fmt.Sprintf("%s Distribution LLC", p.Brand),
					Contact:       fmt.Sprintf("sales@%sdist.com", p.Brand),
					UnitPrice:     wholesalePrice,
					MOQ:           24 + rand.Intn(200),
					LeadTimeDays:  5 + rand.Intn(25),
					ShippingTerms: "FOB",
					Authorized:    rand.Intn(3) != 0,
				},
			},
			OutreachDrafts:  []string{fmt.Sprintf("Hi, I'm interested in wholesale pricing for %s. We're an established Amazon FBA seller...", p.Title)},
			ReviewerVerdict: verdict,
			IterationCount:  iterations,
		}

		// Only include candidates that passed the reviewer
		if verdict != "CUT" {
			candidates = append(candidates, candidate)
		}

		// Add trail entries for each agent
		for _, agentName := range []string{"sourcing", "demand", "competition", "profitability", "risk", "supplier", "reviewer"} {
			trail = append(trail, domain.AgentTrailEntry{
				AgentName:  agentName,
				ASIN:       p.ASIN,
				Iteration:  1,
				DurationMs: int64(200 + rand.Intn(2000)),
			})
		}
	}

	slog.Info("simulated pipeline complete",
		"campaign_id", input.CampaignID,
		"total_evaluated", numCandidates,
		"passed", len(candidates),
	)

	return &domain.ResearchResult{
		CampaignID:    input.CampaignID,
		Candidates:    candidates,
		ResearchTrail: trail,
		Summary:       fmt.Sprintf("Evaluated %d products, %d passed quality gates", numCandidates, len(candidates)),
	}, nil
}
