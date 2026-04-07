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
	productDiscovery *service.ProductDiscovery,
	brandBlocklistSvc *service.BrandBlocklistService,
	campaigns port.CampaignRepo,
	scoring port.ScoringConfigRepo,
	dealSvc *service.DealService,
	priceListScanner *service.PriceListScanner,
	funnelSvc *service.FunnelService,
	categoryScanSvc *service.CategoryScanService,
	catalogSvc *service.CatalogService,
	brandIntelRepo port.BrandIntelligenceRepo,
	productSearcher port.ProductSearcher,
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

			// Step 2: Build pipeline config (load brand blocklist from DB)
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
				// Load tenant's brand blocklist from DB and merge
				if brandBlocklistSvc != nil {
					dbFilter, err := brandBlocklistSvc.LoadBrandFilter(ctx, tenantID)
					if err != nil {
						slog.Warn("inngest: failed to load brand blocklist", "error", err)
					} else {
						config.Thresholds.BrandFilter.BlockList = append(
							config.Thresholds.BrandFilter.BlockList,
							dbFilter.BlockList...,
						)
					}
				}
				b, _ := json.Marshal(config)
				return string(b), nil
			})
			if err != nil {
				markFailed(ctx, campaigns, tenantID, campaignID)
				return nil, err
			}

			// Step 3: Discover and pre-qualify products (deterministic — no LLM)
			candidatesJSON, err := step.Run(ctx, "discover-products", func(ctx context.Context) (string, error) {
				var config domain.PipelineConfig
				json.Unmarshal([]byte(configJSON), &config)

				if productDiscovery != nil {
					products, err := productDiscovery.DiscoverAndPreQualify(ctx, tenantID, campaign.Criteria, config.Thresholds)
					if err != nil {
						return "", err
					}
					var candidates []map[string]any
					for _, p := range products {
						candidates = append(candidates, p.ToCandidate())
					}
					b, _ := json.Marshal(candidates)
					return string(b), nil
				}

				// Fallback to old sourcing if no discovery service
				if toolResolver != nil {
					sourcingData, err := toolResolver.ResolveForSourcing(ctx, campaign.Criteria)
					if err != nil {
						return "", err
					}
					candidates, err := orchestrator.RunSourcingAgent(ctx, sourcingData, config)
					if err != nil {
						return "", err
					}
					b, _ := json.Marshal(candidates)
					return string(b), nil
				}

				return "[]", nil
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
					// Auto-learn: block this brand for future campaigns
					brand, _ := enriched["brand"].(string)
					if brand != "" && brandBlocklistSvc != nil {
						brandBlocklistSvc.AutoBlock(ctx, tenantID, brand, asin,
							fmt.Sprintf("Too few sellers (%d) — likely private label", sellerCount))
					}
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

			// Step 3: Gate/Risk agent (single LLM call ~15-30s)
			gatingJSON, err := step.Run(ctx, "agent-gating", func(ctx context.Context) (string, error) {
				gatingCfg := config.Agents["gating"]
				out, err := orchestrator.RunSingleAgent(ctx, "gating", gatingCfg, enriched, nil)
				if err != nil {
					return "", err
				}
				b, _ := json.Marshal(out.Structured)
				return string(b), nil
			})
			if err != nil {
				slog.Warn("inngest: gating failed", "asin", asin, "error", err)
				return map[string]string{"asin": asin, "status": "failed-gating"}, nil
			}
			var gatingResult map[string]any
			json.Unmarshal([]byte(gatingJSON), &gatingResult)
			gatingPassed, _ := gatingResult["passed"].(bool)
			if !gatingPassed {
				return map[string]string{"asin": asin, "status": "failed-gating"}, nil
			}
			gatingCtx := domain.AgentContext{AgentName: "gating", Facts: gatingResult}

			// Step 4: Profitability agent (single LLM call ~15-30s)
			profitJSON, err := step.Run(ctx, "agent-profitability", func(ctx context.Context) (string, error) {
				profitInput := enriched
				if toolResolver != nil {
					resolved, err := toolResolver.ResolveForProfitability(ctx, enriched, "US")
					if err == nil {
						profitInput = resolved
					}
				}
				profitCfg := config.Agents["profitability"]
				out, err := orchestrator.RunSingleAgent(ctx, "profitability", profitCfg, profitInput, []domain.AgentContext{gatingCtx})
				if err != nil {
					return "", err
				}
				// Merge deterministic FBA calc
				if fbaCalc, ok := profitInput["fba_calculation"]; ok {
					if fc, ok := fbaCalc.(domain.FBAFeeCalculation); ok {
						out.Structured["net_margin_pct"] = fc.NetMarginPct
						out.Structured["roi_pct"] = fc.ROIPct
						out.Structured["net_profit"] = fc.NetProfit
						out.Structured["total_fees"] = fc.TotalFees
					}
				}
				b, _ := json.Marshal(out.Structured)
				return string(b), nil
			})
			if err != nil {
				slog.Warn("inngest: profitability failed", "asin", asin, "error", err)
				return map[string]string{"asin": asin, "status": "failed-profitability"}, nil
			}
			var profitResult map[string]any
			json.Unmarshal([]byte(profitJSON), &profitResult)
			profitCtx := domain.AgentContext{AgentName: "profitability", Facts: profitResult}

			// Step 5: Demand + Competition agent (single LLM call ~15-30s)
			demandJSON, err := step.Run(ctx, "agent-demand", func(ctx context.Context) (string, error) {
				demandInput := enriched
				if toolResolver != nil {
					resolved, err := toolResolver.ResolveForDemand(ctx, enriched, "US")
					if err == nil {
						demandInput = resolved
					}
				}
				demandCfg := config.Agents["demand"]
				out, err := orchestrator.RunSingleAgent(ctx, "demand", demandCfg, demandInput, []domain.AgentContext{gatingCtx, profitCtx})
				if err != nil {
					return "", err
				}
				b, _ := json.Marshal(out.Structured)
				return string(b), nil
			})
			if err != nil {
				slog.Warn("inngest: demand failed", "asin", asin, "error", err)
				return map[string]string{"asin": asin, "status": "failed-demand"}, nil
			}
			var demandResult map[string]any
			json.Unmarshal([]byte(demandJSON), &demandResult)
			demandCtx := domain.AgentContext{AgentName: "demand", Facts: demandResult}

			// Step 6: Supplier agent (single LLM call ~15-30s)
			supplierJSON, err := step.Run(ctx, "agent-supplier", func(ctx context.Context) (string, error) {
				supplierInput := enriched
				if toolResolver != nil {
					resolved, err := toolResolver.ResolveForSupplier(ctx, enriched)
					if err == nil {
						supplierInput = resolved
					}
				}
				supplierCfg := config.Agents["supplier"]
				out, err := orchestrator.RunSingleAgent(ctx, "supplier", supplierCfg, supplierInput, []domain.AgentContext{gatingCtx, profitCtx, demandCtx})
				if err != nil {
					return "", err
				}
				b, _ := json.Marshal(out.Structured)
				return string(b), nil
			})
			if err != nil {
				slog.Warn("inngest: supplier failed", "asin", asin, "error", err)
				return map[string]string{"asin": asin, "status": "failed-supplier"}, nil
			}
			var supplierResult map[string]any
			json.Unmarshal([]byte(supplierJSON), &supplierResult)
			supplierCtx := domain.AgentContext{AgentName: "supplier", Facts: supplierResult}

			// Step 7: Reviewer (hybrid rules + LLM ~15-30s)
			reviewJSON, err := step.Run(ctx, "agent-reviewer", func(ctx context.Context) (string, error) {
				allContexts := []domain.AgentContext{gatingCtx, profitCtx, demandCtx, supplierCtx}
				reviewInput := make(map[string]any)
				for k, v := range enriched {
					reviewInput[k] = v
				}
				for k, v := range profitResult {
					reviewInput[k] = v
				}
				for k, v := range demandResult {
					reviewInput[k] = v
				}
				for k, v := range supplierResult {
					reviewInput[k] = v
				}
				reviewerCfg := config.Agents["reviewer"]
				result, err := orchestrator.ReviewCandidate(ctx, reviewInput, allContexts, reviewerCfg, config)
				if err != nil {
					return "", err
				}
				b, _ := json.Marshal(result)
				return string(b), nil
			})
			if err != nil {
				slog.Warn("inngest: reviewer failed", "asin", asin, "error", err)
				return map[string]string{"asin": asin, "status": "failed-reviewer"}, nil
			}

			var reviewResult service.ReviewResult
			json.Unmarshal([]byte(reviewJSON), &reviewResult)

			if reviewResult.Tier == domain.DealTierCut {
				slog.Info("inngest: candidate cut by reviewer", "asin", asin)
				return map[string]string{"asin": asin, "status": "cut"}, nil
			}

			// Build the final candidate result
			title, _ := enriched["title"].(string)
			brand, _ := enriched["brand"].(string)
			category, _ := enriched["category"].(string)
			demandScore, _ := getIntFromMap(demandResult, "demand_score")
			competitionScore, _ := getIntFromMap(demandResult, "competition_score")
			marginPct, _ := getFloatFromMap(profitResult, "net_margin_pct")
			riskScore, _ := getIntFromMap(gatingResult, "risk_score")

			result := &domain.CandidateResult{
				ASIN:     asin,
				Title:    title,
				Brand:    brand,
				Category: category,
				Scores: domain.DealScores{
					Demand:              demandScore,
					Competition:         competitionScore,
					Margin:              scoreFromMarginPct(marginPct),
					Risk:                10 - riskScore,
					SourcingFeasibility: reviewResult.SourcingFeasibility,
					Overall:             reviewResult.WeightedComposite,
				},
				Evidence: domain.Evidence{
					Demand:      domain.AgentEvidence{Reasoning: strFromMap(demandResult, "reasoning"), Data: demandResult},
					Competition: domain.AgentEvidence{Reasoning: strFromMap(demandResult, "reasoning"), Data: demandResult},
					Margin:      domain.AgentEvidence{Reasoning: strFromMap(profitResult, "reasoning"), Data: profitResult},
					Risk:        domain.AgentEvidence{Reasoning: strFromMap(gatingResult, "reasoning"), Data: gatingResult},
					Sourcing:    domain.AgentEvidence{Reasoning: strFromMap(supplierResult, "reasoning"), Data: supplierResult},
				},
				ReviewerVerdict: reviewResult.Reasoning,
				Tier:            reviewResult.Tier,
				IterationCount:  1,
			}

			slog.Info("inngest: candidate passed", "asin", asin, "tier", result.Tier, "score", result.Scores.Overall)

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

	// =========================================================
	// Function 3: process-pricelist (price list upload → funnel → LLM)
	// Matches UPC/EAN to ASINs, runs funnel, fans out LLM on survivors
	// =========================================================
	type PriceListUploadedEvent struct {
		TenantID        string `json:"tenant_id"`
		CampaignID      string `json:"campaign_id"`
		ItemsJSON       string `json:"items_json"`       // JSON-serialized []PriceListItem
		ThresholdsJSON  string `json:"thresholds_json"`  // JSON-serialized PipelineThresholds
	}

	if priceListScanner != nil && funnelSvc != nil {
		inngestgo.CreateFunction(
			client,
			inngestgo.FunctionOpts{ID: "process-pricelist", Name: "Process Price List", Retries: &retries},
			inngestgo.EventTrigger("pricelist/uploaded", nil),
			func(ctx context.Context, input inngestgo.Input[PriceListUploadedEvent]) (any, error) {
				data := input.Event.Data
				tenantID := domain.TenantID(data.TenantID)
				campaignID := domain.CampaignID(data.CampaignID)

				slog.Info("inngest[process-pricelist]: started", "campaign_id", data.CampaignID)

				// Step 1: Parse items from JSON
				var items []domain.PriceListItem
				json.Unmarshal([]byte(data.ItemsJSON), &items)

				var thresholds domain.PipelineThresholds
				if err := json.Unmarshal([]byte(data.ThresholdsJSON), &thresholds); err != nil {
					thresholds = domain.DefaultPipelineThresholds()
				}

				// Step 2: Match UPC/EAN → ASINs
				funnelInputs, err := step.Run(ctx, "match-identifiers", func(ctx context.Context) (string, error) {
					matched, err := priceListScanner.MatchItemsToASINs(ctx, items, "US")
					if err != nil {
						return "", err
					}
					b, _ := json.Marshal(matched)
					return string(b), nil
				})
				if err != nil {
					markFailed(ctx, campaigns, tenantID, campaignID)
					return nil, fmt.Errorf("match identifiers: %w", err)
				}

				var matched []service.FunnelInput
				json.Unmarshal([]byte(funnelInputs), &matched)

				if len(matched) == 0 {
					step.Run(ctx, "complete-empty", func(ctx context.Context) (string, error) {
						completeCampaign(ctx, campaigns, tenantID, campaignID)
						return "no matches", nil
					})
					return map[string]any{"status": "completed", "matched": 0}, nil
				}

				// Step 3: Run funnel (T0-T3)
				survivorsJSON, err := step.Run(ctx, "run-funnel", func(ctx context.Context) (string, error) {
					survivors, stats, err := funnelSvc.ProcessBatch(ctx, tenantID, matched, thresholds)
					if err != nil {
						return "", err
					}
					result := map[string]any{"survivors": survivors, "stats": stats}
					b, _ := json.Marshal(result)
					return string(b), nil
				})
				if err != nil {
					markFailed(ctx, campaigns, tenantID, campaignID)
					return nil, fmt.Errorf("funnel: %w", err)
				}

				var funnelResult struct {
					Survivors []service.FunnelSurvivor `json:"survivors"`
					Stats     service.FunnelStats      `json:"stats"`
				}
				json.Unmarshal([]byte(survivorsJSON), &funnelResult)

				slog.Info("inngest[process-pricelist]: funnel complete",
					"input", funnelResult.Stats.InputCount,
					"survivors", funnelResult.Stats.SurvivorCount)

				if len(funnelResult.Survivors) == 0 {
					step.Run(ctx, "complete-no-survivors", func(ctx context.Context) (string, error) {
						completeCampaign(ctx, campaigns, tenantID, campaignID)
						return "no survivors", nil
					})
					return map[string]any{"status": "completed", "matched": len(matched), "survivors": 0, "stats": funnelResult.Stats}, nil
				}

				// Step 4: Build pipeline config for LLM evaluation
				configJSON, err := step.Run(ctx, "build-llm-config", func(ctx context.Context) (string, error) {
					config := domain.DefaultPipelineConfig(tenantID)
					b, _ := json.Marshal(config)
					return string(b), nil
				})
				if err != nil {
					markFailed(ctx, campaigns, tenantID, campaignID)
					return nil, err
				}

				// Step 5: Fan out LLM evaluation per survivor (reuse evaluate-candidate)
				// Cap at 200 candidates max to control cost
				maxLLM := 200
				if len(funnelResult.Survivors) < maxLLM {
					maxLLM = len(funnelResult.Survivors)
				}
				for i := 0; i < maxLLM; i++ {
					s := funnelResult.Survivors[i]
					step.Run(ctx, fmt.Sprintf("dispatch-llm-%d", i), func(ctx context.Context) (string, error) {
						candidate := s.DiscoveredProduct
						candidateMap := map[string]any{
							"asin":                 candidate.ASIN,
							"title":                candidate.Title,
							"brand":                candidate.BrandID,
							"category":             candidate.Category,
							"amazon_price":         candidate.BuyBoxPrice,
							"estimated_price":      candidate.EstimatedPrice,
							"bsr_rank":             candidate.BSRRank,
							"seller_count":         candidate.SellerCount,
							"estimated_margin_pct": candidate.EstimatedMarginPct,
							"real_margin_pct":      candidate.RealMarginPct,
							"wholesale_cost":       s.WholesaleCost,
						}
						b, _ := json.Marshal(candidateMap)
						client.Send(ctx, inngestgo.Event{
							Name: "candidate/evaluate",
							Data: map[string]any{
								"campaign_id": data.CampaignID,
								"tenant_id":   data.TenantID,
								"asin":        candidate.ASIN,
								"candidate":   string(b),
								"config_json": configJSON,
							},
						})
						return candidate.ASIN, nil
					})
				}

				// Step 6: Wait for LLM evaluations
				waitTime := time.Duration(maxLLM*30+60) * time.Second
				if waitTime > 10*time.Minute {
					waitTime = 10 * time.Minute
				}
				step.Sleep(ctx, "wait-for-llm", waitTime)

				// Step 7: Complete campaign
				step.Run(ctx, "complete-campaign", func(ctx context.Context) (string, error) {
					completeCampaign(ctx, campaigns, tenantID, campaignID)
					return "completed", nil
				})

				slog.Info("inngest[process-pricelist]: completed",
					"campaign_id", data.CampaignID,
					"matched", len(matched),
					"survivors", funnelResult.Stats.SurvivorCount,
					"llm_dispatched", maxLLM)

				return map[string]any{
					"status":    "completed",
					"matched":   len(matched),
					"survivors": funnelResult.Stats.SurvivorCount,
					"stats":     funnelResult.Stats,
				}, nil
			},
		)
	}

	// =========================================================
	// Function 4: nightly-category-scan (cron — background catalog building)
	// Picks browse nodes from rotation, searches SP-API, runs funnel
	// =========================================================
	type NightlyScanEvent struct {
		TenantID string `json:"tenant_id"`
		MaxNodes int    `json:"max_nodes"`
	}

	if categoryScanSvc != nil {
		cron := "0 2 * * *" // 2 AM UTC
		inngestgo.CreateFunction(
			client,
			inngestgo.FunctionOpts{
				ID:      "nightly-category-scan",
				Name:    "Nightly Category Scan",
				Retries: &retries,
			},
			inngestgo.CronTrigger(cron),
			func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
				// Default tenant for now — in multi-tenant future, fan out per tenant
				defaultTenantID := domain.TenantID("00000000-0000-0000-0000-000000000010")
				maxNodes := 100

				slog.Info("inngest[nightly-scan]: starting", "tenant_id", defaultTenantID, "max_nodes", maxNodes)

				thresholds := domain.DefaultPipelineThresholds()

				job, err := step.Run(ctx, "scan-nodes", func(ctx context.Context) (*domain.ScanJob, error) {
					return categoryScanSvc.ScanNextNodes(ctx, defaultTenantID, maxNodes, thresholds)
				})
				if err != nil {
					slog.Error("inngest[nightly-scan]: failed", "error", err)
					return map[string]string{"status": "failed"}, err
				}

				if job == nil {
					return map[string]string{"status": "no_nodes"}, nil
				}

				slog.Info("inngest[nightly-scan]: complete",
					"products_found", job.TotalItems,
					"qualified", job.Qualified,
					"eliminated", job.Eliminated)

				return map[string]any{
					"status":    "completed",
					"scan_id":   job.ID,
					"products":  job.TotalItems,
					"qualified": job.Qualified,
				}, nil
			},
		)

		// Also allow manual trigger
		inngestgo.CreateFunction(
			client,
			inngestgo.FunctionOpts{ID: "manual-category-scan", Name: "Manual Category Scan", Retries: &retries},
			inngestgo.EventTrigger("scan/category", nil),
			func(ctx context.Context, input inngestgo.Input[NightlyScanEvent]) (any, error) {
				data := input.Event.Data
				tenantID := domain.TenantID(data.TenantID)
				maxNodes := data.MaxNodes
				if maxNodes <= 0 {
					maxNodes = 50
				}

				thresholds := domain.DefaultPipelineThresholds()

				job, err := step.Run(ctx, "scan-nodes", func(ctx context.Context) (*domain.ScanJob, error) {
					return categoryScanSvc.ScanNextNodes(ctx, tenantID, maxNodes, thresholds)
				})
				if err != nil {
					return nil, err
				}

				if job == nil {
					return map[string]string{"status": "no_nodes"}, nil
				}

				return map[string]any{
					"status":    "completed",
					"scan_id":   job.ID,
					"products":  job.TotalItems,
					"qualified": job.Qualified,
				}, nil
			},
		)
	}

	// =========================================================
	// Function 6: catalog-refresh (cron — refresh stale pricing + brand intelligence)
	// =========================================================
	if catalogSvc != nil {
		refreshCron := "0 6 * * *" // 6 AM UTC
		inngestgo.CreateFunction(
			client,
			inngestgo.FunctionOpts{ID: "catalog-refresh", Name: "Catalog Refresh", Retries: &retries},
			inngestgo.CronTrigger(refreshCron),
			func(ctx context.Context, input inngestgo.Input[any]) (any, error) {
				defaultTenantID := domain.TenantID("00000000-0000-0000-0000-000000000010")

				slog.Info("inngest[catalog-refresh]: starting")

				// Step 1: Recompute refresh priorities
				step.Run(ctx, "recompute-priority", func(ctx context.Context) (string, error) {
					return "done", catalogSvc.UpdateRefreshPriority(ctx, defaultTenantID)
				})

				// Step 2: Refresh stale products (top 500 by priority)
				refreshed, _ := step.Run(ctx, "refresh-pricing", func(ctx context.Context) (int, error) {
					stale, err := catalogSvc.ListStale(ctx, defaultTenantID, 24*time.Hour, 500)
					if err != nil {
						return 0, err
					}
					if len(stale) == 0 {
						return 0, nil
					}

					// Batch refresh via competitive pricing
					asins := make([]string, len(stale))
					for i, p := range stale {
						asins[i] = p.ASIN
					}

					for i := 0; i < len(asins); i += 20 {
						end := i + 20
						if end > len(asins) {
							end = len(asins)
						}
						batch := asins[i:end]
						details, err := productSearcher.GetProductDetails(ctx, batch, "US")
						if err != nil {
							slog.Warn("catalog-refresh: batch pricing failed", "error", err)
							continue
						}
						for _, d := range details {
							if d.ASIN == "" || d.AmazonPrice <= 0 {
								continue
							}
							wholesaleCost := d.AmazonPrice * 0.4
							fbaCalc := domain.CalculateFBAFees(d.AmazonPrice, wholesaleCost, 1.0, false)
							catalogSvc.UpdatePricing(ctx, defaultTenantID, d.ASIN, d.AmazonPrice, d.SellerCount, d.BSRRank, fbaCalc.NetMarginPct)
						}
					}
					return len(stale), nil
				})

				// Step 3: Refresh brand intelligence materialized view
				if brandIntelRepo != nil {
					step.Run(ctx, "refresh-brand-intelligence", func(ctx context.Context) (string, error) {
						return "done", brandIntelRepo.Refresh(ctx)
					})
				}

				slog.Info("inngest[catalog-refresh]: complete", "refreshed", refreshed)
				return map[string]any{"status": "completed", "refreshed": refreshed}, nil
			},
		)
	}

	return &DurableRuntime{client: client}, nil
}

// TriggerCategoryScan sends a manual category scan event.
func (r *DurableRuntime) TriggerCategoryScan(ctx context.Context, tenantID domain.TenantID, maxNodes int) error {
	_, err := r.client.Send(ctx, inngestgo.Event{
		Name: "scan/category",
		Data: map[string]any{
			"tenant_id": string(tenantID),
			"max_nodes": maxNodes,
		},
	})
	return err
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

// Helper functions for extracting typed values from map[string]any
func getIntFromMap(m map[string]any, key string) (int, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		return val, true
	case float64:
		return int(val), true
	}
	return 0, false
}

func getFloatFromMap(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	}
	return 0, false
}

func strFromMap(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func scoreFromMarginPct(marginPct float64) int {
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
