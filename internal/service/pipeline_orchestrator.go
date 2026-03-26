package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type PipelineOrchestrator struct {
	runtime  port.AgentRuntime
	reviewer *Reviewer
}

func NewPipelineOrchestrator(runtime port.AgentRuntime) *PipelineOrchestrator {
	return &PipelineOrchestrator{
		runtime:  runtime,
		reviewer: NewReviewer(runtime),
	}
}

func (o *PipelineOrchestrator) RunPipeline(ctx context.Context, campaignID domain.CampaignID, criteria domain.Criteria, config domain.PipelineConfig) (*domain.ResearchResult, error) {
	slog.Info("pipeline: starting", "campaign_id", campaignID)

	// Stage 1: Sourcing
	sourcingCfg := config.Agents["sourcing"]
	sourcingOut, err := o.runtime.RunAgent(ctx, domain.AgentTask{
		AgentName:    "sourcing",
		SystemPrompt: sourcingCfg.SystemPrompt,
		Input:        map[string]any{"criteria": criteria},
	})
	if err != nil {
		return nil, fmt.Errorf("sourcing agent failed: %w", err)
	}

	candidatesRaw := extractCandidateList(sourcingOut.Structured["candidates"])
	if len(candidatesRaw) == 0 {
		return &domain.ResearchResult{
			CampaignID: campaignID,
			Summary:    "No candidates found by sourcing agent",
		}, nil
	}

	slog.Info("pipeline: sourcing complete", "candidates", len(candidatesRaw))

	var trail []domain.AgentTrailEntry
	trail = append(trail, domain.AgentTrailEntry{AgentName: "sourcing", DurationMs: sourcingOut.DurationMs})

	// Process each candidate through the funnel
	var results []domain.CandidateResult
	for _, candidateMap := range candidatesRaw {
		asin, _ := candidateMap["asin"].(string)
		title, _ := candidateMap["title"].(string)
		brand, _ := candidateMap["brand"].(string)
		category, _ := candidateMap["category"].(string)
		if asin == "" {
			continue
		}

		var agentContexts []domain.AgentContext

		// Stage 2: Gate/Risk
		gatingCfg := config.Agents["gating"]
		gatingOut, err := o.runtime.RunAgent(ctx, domain.AgentTask{
			AgentName:    "gating",
			SystemPrompt: gatingCfg.SystemPrompt,
			Input:        candidateMap,
		})
		if err != nil {
			slog.Warn("pipeline: gating failed, skipping", "asin", asin, "error", err)
			continue
		}
		trail = append(trail, domain.AgentTrailEntry{AgentName: "gating", ASIN: asin, DurationMs: gatingOut.DurationMs})

		passed, _ := gatingOut.Structured["passed"].(bool)
		if !passed {
			slog.Debug("pipeline: eliminated at gating", "asin", asin)
			continue
		}

		agentContexts = append(agentContexts, domain.AgentContext{
			AgentName: "gating",
			Facts:     gatingOut.Structured,
			Flags:     toStringSlice(gatingOut.Structured["flags"]),
		})

		// Stage 3: Profitability
		profitCfg := config.Agents["profitability"]
		profitOut, err := o.runtime.RunAgent(ctx, domain.AgentTask{
			AgentName:    "profitability",
			SystemPrompt: profitCfg.SystemPrompt,
			Input:        candidateMap,
			Context:      agentContexts,
		})
		if err != nil {
			slog.Warn("pipeline: profitability failed, skipping", "asin", asin, "error", err)
			continue
		}
		trail = append(trail, domain.AgentTrailEntry{AgentName: "profitability", ASIN: asin, DurationMs: profitOut.DurationMs})

		marginPct, _ := getFloat(profitOut.Structured, "net_margin_pct")
		if marginPct < config.Thresholds.MinMarginPct {
			slog.Debug("pipeline: eliminated at profitability", "asin", asin, "margin", marginPct)
			continue
		}

		if errs := domain.ValidateAgentOutput("profitability", profitOut.Structured); len(errs) > 0 {
			slog.Warn("pipeline: profitability validation failed", "asin", asin, "errors", errs)
			continue
		}

		agentContexts = append(agentContexts, domain.AgentContext{
			AgentName: "profitability",
			Facts:     profitOut.Structured,
		})

		// Stage 4: Demand + Competition
		demandCfg := config.Agents["demand"]
		demandOut, err := o.runtime.RunAgent(ctx, domain.AgentTask{
			AgentName:    "demand",
			SystemPrompt: demandCfg.SystemPrompt,
			Input:        candidateMap,
			Context:      agentContexts,
		})
		if err != nil {
			slog.Warn("pipeline: demand failed, skipping", "asin", asin, "error", err)
			continue
		}
		trail = append(trail, domain.AgentTrailEntry{AgentName: "demand", ASIN: asin, DurationMs: demandOut.DurationMs})

		agentContexts = append(agentContexts, domain.AgentContext{
			AgentName: "demand",
			Facts:     demandOut.Structured,
		})

		// Stage 5: Supplier
		supplierCfg := config.Agents["supplier"]
		supplierOut, err := o.runtime.RunAgent(ctx, domain.AgentTask{
			AgentName:    "supplier",
			SystemPrompt: supplierCfg.SystemPrompt,
			Input:        candidateMap,
			Context:      agentContexts,
		})
		if err != nil {
			slog.Warn("pipeline: supplier failed, skipping", "asin", asin, "error", err)
			continue
		}
		trail = append(trail, domain.AgentTrailEntry{AgentName: "supplier", ASIN: asin, DurationMs: supplierOut.DurationMs})

		agentContexts = append(agentContexts, domain.AgentContext{
			AgentName: "supplier",
			Facts:     supplierOut.Structured,
		})

		// Stage 6: Review (hybrid)
		reviewInput := mergeMaps(candidateMap, profitOut.Structured, demandOut.Structured, supplierOut.Structured)
		reviewerCfg := config.Agents["reviewer"]
		reviewResult, err := o.reviewer.Review(ctx, reviewInput, agentContexts, reviewerCfg, config.Thresholds, config.Scoring)
		if err != nil {
			slog.Warn("pipeline: reviewer failed", "asin", asin, "error", err)
			continue
		}
		trail = append(trail, domain.AgentTrailEntry{AgentName: "reviewer", ASIN: asin})

		if reviewResult.Tier == domain.DealTierCut {
			slog.Debug("pipeline: cut by reviewer", "asin", asin)
			continue
		}

		// Build scores
		demandScore, _ := getInt(demandOut.Structured, "demand_score")
		competitionScore, _ := getInt(demandOut.Structured, "competition_score")
		marginScore := scoreFromMargin(marginPct)
		riskScore, _ := getInt(gatingOut.Structured, "risk_score")
		sourcingScore := reviewResult.SourcingFeasibility

		overall := float64(demandScore)*config.Scoring.Demand +
			float64(competitionScore)*config.Scoring.Competition +
			float64(marginScore)*config.Scoring.Margin +
			float64(10-riskScore)*config.Scoring.Risk +
			float64(sourcingScore)*config.Scoring.Sourcing

		// Build supplier candidates from structured output
		supplierCandidates := extractSupplierCandidates(supplierOut.Structured)

		outreachDraft, _ := supplierOut.Structured["outreach_draft"].(string)
		var outreachDrafts []string
		if outreachDraft != "" {
			outreachDrafts = []string{outreachDraft}
		}

		result := domain.CandidateResult{
			ASIN:     asin,
			Title:    title,
			Brand:    brand,
			Category: category,
			Scores: domain.DealScores{
				Demand:              demandScore,
				Competition:         competitionScore,
				Margin:              marginScore,
				Risk:                10 - riskScore,
				SourcingFeasibility: sourcingScore,
				Overall:             overall,
			},
			Evidence: domain.Evidence{
				Demand:      domain.AgentEvidence{Reasoning: strVal(demandOut.Structured, "reasoning"), Data: demandOut.Structured},
				Competition: domain.AgentEvidence{Reasoning: strVal(demandOut.Structured, "reasoning"), Data: demandOut.Structured},
				Margin:      domain.AgentEvidence{Reasoning: strVal(profitOut.Structured, "reasoning"), Data: profitOut.Structured},
				Risk:        domain.AgentEvidence{Reasoning: strVal(gatingOut.Structured, "reasoning"), Data: gatingOut.Structured},
				Sourcing:    domain.AgentEvidence{Reasoning: strVal(supplierOut.Structured, "reasoning"), Data: supplierOut.Structured},
			},
			SupplierCandidates: supplierCandidates,
			OutreachDrafts:     outreachDrafts,
			ReviewerVerdict:    reviewResult.Reasoning,
			Tier:               reviewResult.Tier,
			IterationCount:     1,
		}

		results = append(results, result)
		slog.Info("pipeline: candidate passed", "asin", asin, "tier", reviewResult.Tier, "overall", overall)
	}

	slog.Info("pipeline: complete", "campaign_id", campaignID, "evaluated", len(candidatesRaw), "passed", len(results))

	return &domain.ResearchResult{
		CampaignID:    campaignID,
		Candidates:    results,
		ResearchTrail: trail,
		Summary:       fmt.Sprintf("Evaluated %d products, %d passed quality gates", len(candidatesRaw), len(results)),
	}, nil
}

func scoreFromMargin(marginPct float64) int {
	switch {
	case marginPct >= 50:
		return 10
	case marginPct >= 40:
		return 9
	case marginPct >= 30:
		return 8
	case marginPct >= 25:
		return 7
	case marginPct >= 20:
		return 6
	case marginPct >= 15:
		return 5
	case marginPct >= 10:
		return 4
	default:
		return 3
	}
}

func extractCandidateList(v any) []map[string]any {
	var result []map[string]any
	switch typed := v.(type) {
	case []any:
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
			}
		}
	case []map[string]any:
		result = typed
	}
	return result
}

func extractSupplierCandidates(structured map[string]any) []domain.SupplierCandidate {
	var candidates []domain.SupplierCandidate
	rawSuppliers, ok := structured["suppliers"].([]any)
	if !ok {
		return candidates
	}
	for _, rs := range rawSuppliers {
		sm, ok := rs.(map[string]any)
		if !ok {
			continue
		}
		sc := domain.SupplierCandidate{
			Company: fmt.Sprintf("%v", sm["company"]),
		}
		if up, ok := getFloat(sm, "unit_price"); ok {
			sc.UnitPrice = up
		}
		if moq, ok := getInt(sm, "moq"); ok {
			sc.MOQ = moq
		}
		if lt, ok := getInt(sm, "lead_time_days"); ok {
			sc.LeadTimeDays = lt
		}
		if auth, ok := sm["authorized"].(bool); ok {
			sc.Authorized = auth
		}
		candidates = append(candidates, sc)
	}
	return candidates
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func toStringSlice(v any) []string {
	switch arr := v.(type) {
	case []any:
		var result []string
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return arr
	}
	return nil
}

func mergeMaps(maps ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
