package inngest

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

// Event types
type CampaignRequestedEvent struct {
	CampaignID string `json:"campaign_id"`
	TenantID   string `json:"tenant_id"`
}

type CandidateEvaluateEvent struct {
	CampaignID string `json:"campaign_id"`
	TenantID   string `json:"tenant_id"`
	ASIN       string `json:"asin"`
	Candidate  string `json:"candidate"`  // JSON-serialized candidate map
	ConfigJSON string `json:"config_json"` // JSON-serialized PipelineConfig
}

type DurableRuntime struct {
	client inngestgo.Client
}

func NewDurableRuntime(
	pipelineSvc *service.PipelineService,
	orchestrator *service.PipelineOrchestrator,
	toolResolver *service.ToolResolver,
	campaigns port.CampaignRepo,
	scoring port.ScoringConfigRepo,
	dealSvc *service.DealService,
) (*DurableRuntime, error) {
	client, err := inngestgo.NewClient(inngestgo.ClientOpts{
		AppID: "fba-orchestrator",
	})
	if err != nil {
		return nil, fmt.Errorf("create inngest client: %w", err)
	}

	retries := 2

	// =========================================================
	// Function 1: process-campaign (parent)
	// Resolves data, selects candidates, fans out, waits, saves
	// =========================================================
	inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "process-campaign", Name: "Process Campaign", Retries: &retries},
		inngestgo.EventTrigger("campaign/requested", nil),
		func(ctx context.Context, input inngestgo.Input[CampaignRequestedEvent]) (any, error) {
			data := input.Event.Data
			campaignID := domain.CampaignID(data.CampaignID)
			tenantID := domain.TenantID(data.TenantID)

			slog.Info("inngest[process-campaign]: started", "campaign_id", data.CampaignID)

			// Step 1: Start campaign
			campaign, err := step.Run(ctx, "start-campaign", func(ctx context.Context) (*domain.Campaign, error) {
				c, err := campaigns.GetByID(ctx, tenantID, campaignID)
				if err != nil {
					return nil, err
				}
				if err := c.Transition(domain.CampaignStatusRunning); err != nil {
					return nil, err
				}
				return c, campaigns.Update(ctx, c)
			})
			if err != nil {
				return nil, fmt.Errorf("start campaign: %w", err)
			}

			// Step 2: Build pipeline config
			configJSON, err := step.Run(ctx, "build-config", func(ctx context.Context) (string, error) {
				sc, err := scoring.GetByID(ctx, tenantID, campaign.ScoringConfigID)
				if err != nil {
					return "", err
				}
				config := domain.DefaultPipelineConfig(tenantID)
				config.Scoring = sc.Weights
				if campaign.Criteria.MinMarginPct != nil {
					config.Thresholds.MinMarginPct = *campaign.Criteria.MinMarginPct
				}
				if len(campaign.Criteria.BlockedBrands) > 0 {
					config.Thresholds.BrandFilter.BlockList = campaign.Criteria.BlockedBrands
				}
				if len(campaign.Criteria.PreferredBrands) > 0 {
					config.Thresholds.BrandFilter.AllowList = campaign.Criteria.PreferredBrands
				}
				b, _ := json.Marshal(config)
				return string(b), nil
			})
			if err != nil {
				markFailed(ctx, campaigns, tenantID, campaignID)
				return nil, err
			}

			// Step 3: Resolve sourcing data (SP-API + Exa — no LLM)
			sourcingData, err := step.Run(ctx, "resolve-sourcing", func(ctx context.Context) (map[string]any, error) {
				if toolResolver == nil {
					return map[string]any{"criteria": campaign.Criteria}, nil
				}
				return toolResolver.ResolveForSourcing(ctx, campaign.Criteria)
			})
			if err != nil {
				markFailed(ctx, campaigns, tenantID, campaignID)
				return nil, err
			}

			// Step 4: Select candidates via sourcing agent (LLM)
			candidatesJSON, err := step.Run(ctx, "select-candidates", func(ctx context.Context) (string, error) {
				var config domain.PipelineConfig
				json.Unmarshal([]byte(configJSON), &config)
				candidates, err := orchestrator.RunSourcingAgent(ctx, sourcingData, config)
				if err != nil {
					return "", err
				}
				b, _ := json.Marshal(candidates)
				return string(b), nil
			})
			if err != nil {
				markFailed(ctx, campaigns, tenantID, campaignID)
				return nil, err
			}

			var candidates []map[string]any
			json.Unmarshal([]byte(candidatesJSON), &candidates)

			if len(candidates) == 0 {
				step.Run(ctx, "complete-empty", func(ctx context.Context) (string, error) {
					completeCampaign(ctx, campaigns, tenantID, campaignID)
					return "no candidates", nil
				})
				return map[string]string{"status": "completed", "candidates": "0"}, nil
			}

			// Step 5: Fan out — one event per candidate
			for i, c := range candidates {
				candidateCopy := c
				step.Run(ctx, fmt.Sprintf("dispatch-%d", i), func(ctx context.Context) (string, error) {
					b, _ := json.Marshal(candidateCopy)
					asin, _ := candidateCopy["asin"].(string)
					client.Send(ctx, inngestgo.Event{
						Name: "candidate/evaluate",
						Data: map[string]any{
							"campaign_id": data.CampaignID,
							"tenant_id":   data.TenantID,
							"asin":        asin,
							"candidate":   string(b),
							"config_json": configJSON,
						},
					})
					return asin, nil
				})
			}

			// Step 6: Wait for candidates to process
			// Each child function takes ~60-90s, running in parallel
			waitTime := time.Duration(len(candidates)*30+60) * time.Second
			if waitTime > 5*time.Minute {
				waitTime = 5 * time.Minute
			}
			step.Sleep(ctx, "wait-for-evaluations", waitTime)

			// Step 7: Complete campaign
			step.Run(ctx, "complete-campaign", func(ctx context.Context) (string, error) {
				completeCampaign(ctx, campaigns, tenantID, campaignID)
				return "completed", nil
			})

			slog.Info("inngest[process-campaign]: completed", "campaign_id", data.CampaignID)
			return map[string]string{"status": "completed"}, nil
		},
	)

	// =========================================================
	// Function 2: evaluate-candidate (child — one per ASIN)
	// Runs full pipeline for a single candidate, saves deal if passes
	// =========================================================
	inngestgo.CreateFunction(
		client,
		inngestgo.FunctionOpts{ID: "evaluate-candidate", Name: "Evaluate Candidate", Retries: &retries},
		inngestgo.EventTrigger("candidate/evaluate", nil),
		func(ctx context.Context, input inngestgo.Input[CandidateEvaluateEvent]) (any, error) {
			data := input.Event.Data
			tenantID := domain.TenantID(data.TenantID)
			campaignID := domain.CampaignID(data.CampaignID)

			var candidate map[string]any
			json.Unmarshal([]byte(data.Candidate), &candidate)

			var config domain.PipelineConfig
			json.Unmarshal([]byte(data.ConfigJSON), &config)

			asin := data.ASIN
			slog.Info("inngest[evaluate-candidate]: started", "campaign_id", data.CampaignID, "asin", asin)

			// Step 1: Enrich with competitive pricing (no LLM, fast)
			enriched, err := step.Run(ctx, "enrich", func(ctx context.Context) (map[string]any, error) {
				if toolResolver == nil {
					return candidate, nil
				}
				return toolResolver.ResolveForGating(ctx, candidate, "US")
			})
			if err != nil {
				enriched = candidate
			}

			// Step 2: Pre-gate — deterministic filters (no LLM, instant)
			passed, _ := step.Run(ctx, "pre-gate", func(ctx context.Context) (bool, error) {
				// Seller count
				sellerCount := 0
				if sc, ok := enriched["seller_count"].(float64); ok {
					sellerCount = int(sc)
				} else if sc, ok := enriched["seller_count"].(int); ok {
					sellerCount = sc
				}
				if config.Thresholds.MinSellerCount > 0 && sellerCount > 0 && sellerCount < config.Thresholds.MinSellerCount {
					slog.Info("inngest: pre-gate eliminated (sellers)", "asin", asin, "sellers", sellerCount)
					return false, nil
				}

				// Brand filter
				brand, _ := enriched["brand"].(string)
				if !config.Thresholds.BrandFilter.IsBrandAllowed(brand) {
					slog.Info("inngest: pre-gate eliminated (brand)", "asin", asin, "brand", brand)
					return false, nil
				}

				// Margin check
				price, _ := enriched["amazon_price"].(float64)
				if price > 0 {
					fbaCalc := domain.CalculateFBAFees(price, price*0.4, 1.0, false)
					if fbaCalc.NetMarginPct < config.Thresholds.MinMarginPct {
						slog.Info("inngest: pre-gate eliminated (margin)", "asin", asin, "margin", fbaCalc.NetMarginPct)
						return false, nil
					}
				}

				return true, nil
			})

			if !passed {
				return map[string]string{"asin": asin, "status": "eliminated"}, nil
			}

			// Step 3: Full pipeline evaluation (LLM calls — each ~15-30s)
			result, err := step.Run(ctx, "evaluate", func(ctx context.Context) (*domain.CandidateResult, error) {
				return orchestrator.EvaluateCandidate(ctx, enriched, config)
			})
			if err != nil {
				slog.Warn("inngest: evaluation failed", "asin", asin, "error", err)
				return map[string]string{"asin": asin, "status": "failed"}, nil
			}

			if result.Tier == domain.DealTierCut {
				slog.Info("inngest: candidate cut by reviewer", "asin", asin)
				return map[string]string{"asin": asin, "status": "cut"}, nil
			}

			// Step 4: Save deal to database
			step.Run(ctx, "save-deal", func(ctx context.Context) (string, error) {
				researchResult := &domain.ResearchResult{
					CampaignID: campaignID,
					Candidates: []domain.CandidateResult{*result},
				}
				deals, err := dealSvc.CreateFromResearch(ctx, tenantID, researchResult)
				if err != nil {
					return "", err
				}
				slog.Info("inngest: deal saved", "asin", asin, "deal_id", deals[0].ID)
				return string(deals[0].ID), nil
			})

			slog.Info("inngest[evaluate-candidate]: passed", "asin", asin, "tier", result.Tier, "score", result.Scores.Overall)
			return map[string]string{"asin": asin, "status": "passed", "tier": string(result.Tier)}, nil
		},
	)

	return &DurableRuntime{client: client}, nil
}

func completeCampaign(ctx context.Context, campaigns port.CampaignRepo, tenantID domain.TenantID, campaignID domain.CampaignID) {
	c, err := campaigns.GetByID(ctx, tenantID, campaignID)
	if err != nil {
		return
	}
	c.Transition(domain.CampaignStatusCompleted)
	campaigns.Update(ctx, c)
}

func markFailed(ctx context.Context, campaigns port.CampaignRepo, tenantID domain.TenantID, campaignID domain.CampaignID) {
	c, err := campaigns.GetByID(ctx, tenantID, campaignID)
	if err != nil {
		return
	}
	c.Transition(domain.CampaignStatusFailed)
	campaigns.Update(ctx, c)
}

func (r *DurableRuntime) TriggerCampaignProcessing(ctx context.Context, campaignID domain.CampaignID, tenantID domain.TenantID) error {
	_, err := r.client.Send(ctx, inngestgo.Event{
		Name: "campaign/requested",
		Data: map[string]any{
			"campaign_id": string(campaignID),
			"tenant_id":   string(tenantID),
		},
	})
	if err != nil {
		return fmt.Errorf("send inngest event: %w", err)
	}
	slog.Info("inngest: campaign event sent", "campaign_id", campaignID)
	return nil
}

func (r *DurableRuntime) TriggerDiscoveryRun(ctx context.Context, tenantID domain.TenantID) error {
	return nil
}

func (r *DurableRuntime) Handler() http.Handler {
	return r.client.Serve()
}
