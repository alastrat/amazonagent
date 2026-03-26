The core insight

  Autoresearch needs to modify what the agents do and measure if it's better. That only works if the things autoresearch can change are parameters we control, not logic buried
   inside OpenFang/ZeroClaw.

  What autoresearch changes          Where it lives
  ─────────────────────────          ──────────────
  Scoring weights                    ScoringConfig (already exists)
  Score thresholds                   ScoringConfig (already exists)
  Agent prompts                      NEW: AgentPromptConfig
  Quality gate parameters            NEW: GateConfig
  Ceiling/floor logic                NEW: ResearchConfig
  Tool selection per agent           NEW: AgentToolConfig

  The architecture that makes all three work together

  ┌─────────────────────────────────────────────────────────┐
  │  PIPELINE CONFIG (what autoresearch experiments on)      │
  │                                                          │
  │  ScoringConfig:    weights, thresholds                   │
  │  AgentConfig:      per-agent system prompts, tools       │
  │  GateConfig:       min scores, max retries, pass/fail    │
  │  ResearchConfig:   ceiling/floor ratios, candidate count │
  │                                                          │
  │  All versioned. Experiments = two configs side by side.  │
  └──────────────────────┬──────────────────────────────────┘
                         │ config drives behavior
                         ▼
  ┌─────────────────────────────────────────────────────────┐
  │  PIPELINE SERVICE (Go — we own the orchestration)        │
  │                                                          │
  │  for each candidate:                                     │
  │    1. RunAgent("sourcing", prompt, tools) → candidates   │
  │    2. RunAgent("demand", prompt, tools) → score          │
  │    3. RunAgent("competition", ...) → score               │
  │    4. RunAgent("profitability", ...) → score             │
  │    5. RunAgent("risk", ...) → score                      │
  │    6. RunAgent("supplier", ...) → suppliers              │
  │    7. RunAgent("reviewer", all scores) → verdict         │
  │    8. if REWRITE && iterations < max → go back to step N │
  │    9. if PASS → add to results                           │
  │                                                          │
  │  This logic is IDENTICAL regardless of runtime.          │
  │  What changes between experiments is the CONFIG.         │
  └──────────────────────┬──────────────────────────────────┘
                         │ executes via
                         ▼
  ┌─────────────────────────────────────────────────────────┐
  │  AGENT RUNTIME (thin executor — swappable)               │
  │                                                          │
  │  interface AgentRuntime {                                 │
  │    RunAgent(ctx, AgentTask) → AgentOutput                │
  │  }                                                       │
  │                                                          │
  │  AgentTask = {                                           │
  │    agent_name, system_prompt, tools[], input, schema     │
  │  }                                                       │
  │                                                          │
  │  OpenFang adapter: POST to OpenFang API                  │
  │  ZeroClaw adapter: POST to ZeroClaw API                  │
  │  Simulator adapter: return fake data                     │
  │  (same interface, swap with one line in config)          │
  └─────────────────────────────────────────────────────────┘

                         ▲ outcomes feed back
                         │
  ┌─────────────────────────────────────────────────────────┐
  │  CONTINUOUS IMPROVEMENT (autoresearch + PostHog)          │
  │                                                          │
  │  PostHog captures:                                       │
  │    deal_approved, deal_rejected, margin_realized, etc.   │
  │    Each event tagged with config_id + campaign_id        │
  │                                                          │
  │  Autoresearch (weekly):                                  │
  │    1. Query PostHog for outcome patterns                 │
  │    2. Propose variant config (e.g. "increase demand      │
  │       weight, make profitability prompt stricter")        │
  │    3. Human approves experiment                          │
  │    4. System runs two campaigns: control vs variant      │
  │    5. Compare outcomes after evaluation window           │
  │    6. Human promotes winner to baseline                  │
  │                                                          │
  │  The experiment is just: two PipelineConfigs,            │
  │  two campaigns, same pipeline code, PostHog compares.    │
  └─────────────────────────────────────────────────────────┘

  Why this is the right approach

  - OpenFang/ZeroClaw are dumb executors. They receive a prompt + tools + input, return structured output. They don't know about scoring, quality gates, or experiments.
  - Our Go code owns the intelligence. Pipeline sequence, quality gates, retry logic, config selection — all in Go, all tested, all framework-independent.
  - Autoresearch experiments on config, not code. Changing a prompt variant or a scoring weight doesn't require redeploying. It's a new row in pipeline_configs.
  - Swapping runtimes is trivial. Change AGENT_RUNTIME=openfang to AGENT_RUNTIME=zeroclaw in env. Pipeline logic doesn't change.
  - Comparison is built-in. Run the same campaign on both runtimes with the same config → compare agent quality, latency, cost.

  Implementation order

  ┌──────┬───────────────────────────────────────────────────────────┬────────────────────────┐
  │ Step │                           What                            │         Impact         │
  ├──────┼───────────────────────────────────────────────────────────┼────────────────────────┤
  │ 1    │ Refactor AgentRuntime to per-agent + PipelineConfig model │ Foundation             │
  ├──────┼───────────────────────────────────────────────────────────┼────────────────────────┤
  │ 2    │ Pipeline orchestration in Go (sequence, gates, retries)   │ Core logic             │
  ├──────┼───────────────────────────────────────────────────────────┼────────────────────────┤
  │ 3    │ Update simulator to new interface                         │ Keep dev working       │
  ├──────┼───────────────────────────────────────────────────────────┼────────────────────────┤
  │ 4    │ Add OpenFang to docker-compose + implement adapter        │ Real agents            │
  ├──────┼───────────────────────────────────────────────────────────┼────────────────────────┤
  │ 5    │ Stub ZeroClaw adapter                                     │ Ready for comparison   │
  ├──────┼───────────────────────────────────────────────────────────┼────────────────────────┤
  │ 6    │ PostHog event tagging with config_id                      │ Measurement foundation │
  ├──────┼───────────────────────────────────────────────────────────┼────────────────────────┤
  │ 7    │ Autoresearch engine (propose → experiment → evaluate)     │ Continuous improvement │
  ├──────┼───────────────────────────────────────────────────────────┼────────────────────────┤
  │ 8    │ Runtime comparison framework                              │ OpenFang vs ZeroClaw   │
  └──────┴───────────────────────────────────────────────────────────┴────────────────────────┘

  Steps 1-3 can happen now with no external dependencies. Steps 4-5 need OpenFang/ZeroClaw running. Steps 6-8 build on top.

  Want me to start with steps 1-3? That's the architectural refactor that makes everything else possible.


  Consolidated Expert Review                                                                                                                                                 
                                                                                                                                                                               
  Where all three agree                                                                                                                                                      
                                                                                                                                                                               
  ┌────────────────────────────────────────────────────────┬─────────────────┬─────────────────────────────────────────────────┬───────────────────────────────────────────┐   
  │                        Finding                         │    Systems      │                 ML/AI Engineer                  │                FBA Expert                 │   
  │                                                        │    Architect    │                                                 │                                           │   
  ├────────────────────────────────────────────────────────┼─────────────────┼─────────────────────────────────────────────────┼───────────────────────────────────────────┤ 
  │ Pipeline orchestration in Go, runtimes as thin         │       Yes       │                       Yes                       │                     —                     │
  │ executors = correct                                    │                 │                                                 │                                           │   
  ├────────────────────────────────────────────────────────┼─────────────────┼─────────────────────────────────────────────────┼───────────────────────────────────────────┤   
  │ Per-agent composable config versioning (not            │       Yes       │                       Yes                       │                     —                     │   
  │ monolithic)                                            │                 │                                                 │                                           │   
  ├────────────────────────────────────────────────────────┼─────────────────┼─────────────────────────────────────────────────┼───────────────────────────────────────────┤ 
  │ Kill losers early — pipeline should be a funnel, not   │        —        │    Yes (early termination saves 40-60% cost)    │ Yes (reorder agents: gate first, profit   │   
  │ flat sequence                                          │                 │                                                 │                  second)                  │ 
  ├────────────────────────────────────────────────────────┼─────────────────┼─────────────────────────────────────────────────┼───────────────────────────────────────────┤
  │ Reviewer rewrite loop is the riskiest design choice    │        —        │      Yes (score drift, cosmetic rewrites,       │ Yes (re-prompting same data is wasteful)  │   
  │                                                        │                 │              diminishing returns)               │                                           │
  ├────────────────────────────────────────────────────────┼─────────────────┼─────────────────────────────────────────────────┼───────────────────────────────────────────┤   
  │ Ground truth verification — don't trust LLM numbers    │        —        │        Yes (verify BSR/fees via SP-API)         │  Yes (web-scraped suppliers are mostly    │   
  │                                                        │                 │                                                 │                 garbage)                  │
  ├────────────────────────────────────────────────────────┼─────────────────┼─────────────────────────────────────────────────┼───────────────────────────────────────────┤   
  │ 8/10 threshold is too strict                           │        —        │                        —                        │   Yes (<2-5% pass rate, wholesale is a    │   
  │                                                        │                 │                                                 │               volume game)                │
  └────────────────────────────────────────────────────────┴─────────────────┴─────────────────────────────────────────────────┴───────────────────────────────────────────┘   
                                                                                                                                                                             
  Critical architecture changes (before building)                                                                                                                              
                                         
  1. Reorder the agent pipeline (FBA expert + ML engineer agree)                                                                                                               
                                                                                                                                                                             
  Current:  Source → Demand → Competition → Profit → Risk → Supplier → Review                                                                                                  
  Better:   Source → Gate/Risk → Profit → Demand+Competition → Supplier → Review                                                                                               
                                                  
  Kill 70% of candidates at steps 2-3 before expensive analysis. Saves compute and cost.                                                                                       
                                                                                                                                                                             
  2. Composable per-agent config from day one (Systems architect)                                                                                                              
                                                                                                                                                                             
  Don't version the whole pipeline config atomically. Each agent has its own versioned config. A pipeline config is a composition: {sourcing: v3, gating: v1, profit: v5, ...}.
   Autoresearch varies one agent at a time. Expensive to retrofit later.                                                                                                     
                                                                                                                                                                               
  3. Hybrid scoring for the Reviewer (ML engineer)                                                                                                                           
                                                  
  - Rule-based for objective checks: required fields present, math correct, BSR in valid range, margin above threshold                                                         
  - LLM-based only for subjective quality: is the analysis coherent, are the right risk factors addressed
  - Track rewrite deltas — if iteration N changes <5% of structured fields, terminate early                                                                                    
  - Consider tiered output (A/B/C) instead of binary pass/fail (FBA expert)                                                                                                    
                                                                                                                                                                               
  4. Go owns schema validation, not the runtime (Systems architect)                                                                                                            
                                                                                                                                                                               
  Don't trust OpenFang/ZeroClaw's output parsing. Go validates every agent output against the expected schema. Plausibility bounds on all numeric fields (BSR 0-10M, margin    
  -100% to +500%, etc.).                                                                                                                                                     
                                                                                                                                                                               
  5. Tool execution protocol (Systems architect)                                                                                                                               
                                                  
  Define explicitly: either all tools are HTTP callbacks to a Go-hosted tool server (runtime-agnostic), or tools are pre-resolved (Go calls SP-API, passes results to agent as 
  context). The second is simpler and more reliable.                                                                                                                         
                                                                                                                                                                               
  6. FBA fee calculation must be deterministic (ML engineer)                                                                                                                 
                                                  
  Don't let an LLM calculate FBA fees. Write a Go function that computes fees from product dimensions, weight, and category via SP-API data. Pass the result to the            
  Profitability agent as input.
                                                                                                                                                                               
  Important but not blocking                                                                                                                                                   
                                                  
  ┌────────────────────────────────────────────────────────────────────────────┬─────────────┬─────────────────────────────┐                                                   
  │                                    Item                                    │   Source    │            When             │                                                 
  ├────────────────────────────────────────────────────────────────────────────┼─────────────┼─────────────────────────────┤
  │ Global rate limiter with per-campaign fairness                             │ Architect   │ Before experiments          │
  ├────────────────────────────────────────────────────────────────────────────┼─────────────┼─────────────────────────────┤
  │ Same-product pairing for valid A/B tests                                   │ ML engineer │ Before autoresearch         │                                                   
  ├────────────────────────────────────────────────────────────────────────────┼─────────────┼─────────────────────────────┤
  │ Circuit breaker on AgentRuntime adapter                                    │ Architect   │ Before production           │                                                   
  ├────────────────────────────────────────────────────────────────────────────┼─────────────┼─────────────────────────────┤                                                   
  │ Idempotency keys on all RunAgent calls                                     │ Architect   │ Before production           │
  ├────────────────────────────────────────────────────────────────────────────┼─────────────┼─────────────────────────────┤                                                   
  │ Structured context sharing between agents (not full reasoning, just facts) │ ML engineer │ During pipeline refactor    │                                                 
  ├────────────────────────────────────────────────────────────────────────────┼─────────────┼─────────────────────────────┤                                                   
  │ Add seasonality, return rate, storage fees to scoring                      │ FBA expert  │ Phase 2+                    │                                                 
  ├────────────────────────────────────────────────────────────────────────────┼─────────────┼─────────────────────────────┤                                                   
  │ Model tiering (cheap models for early stages, expensive for Review)        │ ML engineer │ During OpenFang integration │                                                 
  ├────────────────────────────────────────────────────────────────────────────┼─────────────┼─────────────────────────────┤                                                   
  │ Caching agent outputs by (product, agent, config_version)                  │ ML engineer │ Before production           │                                                 
  └────────────────────────────────────────────────────────────────────────────┴─────────────┴─────────────────────────────┘                                                   
                                                                                                                                                                             
  Product-level feedback (FBA expert — important for roadmap)                                                                                                                  
                                                                                                                                                                             
  - Reverse sourcing from distributor catalogs is more valuable than ASIN discovery — it's the daily workflow for established resellers                                        
  - Reorder management is 70% of the work and 90% of the profit — discovery alone is table stakes                                                                            
  - Supplier discovery via web scraping is weak — curated distributor database + catalog upload is more realistic                                                              
  - Nightly discovery is overkill — weekly for new ASINs, daily only for price/BSR monitoring on active products                                                               
  - At $99/month, competes with Keepa ($20) + a VA ($500). Needs to do things a VA can't do (portfolio optimization, automated margin modeling across scenarios)               
                                                                                                                                                                               
  ---                                                                                                                                                                          
  My recommendation: revised implementation order                                                                                                                              
                                                                                                                                                                               
  Based on all three reviews:                     
                                                                                                                                                                               
  ┌──────┬────────────────────────────────────────────────────────────────┬──────────────────────────────────────────────────┐                                                 
  │ Step │                              What                              │                       Why                        │
  ├──────┼────────────────────────────────────────────────────────────────┼──────────────────────────────────────────────────┤                                                 
  │ 1    │ Refactor AgentRuntime to per-agent + composable config         │ Foundation — all three experts say this is right │                                               
  ├──────┼────────────────────────────────────────────────────────────────┼──────────────────────────────────────────────────┤
  │ 2    │ Reorder pipeline as funnel (gate → profit → demand → supplier) │ 60-70% cost savings, better UX                   │                                                 
  ├──────┼────────────────────────────────────────────────────────────────┼──────────────────────────────────────────────────┤                                                 
  │ 3    │ Deterministic fee calculator + plausibility bounds             │ Ground truth, not LLM math                       │                                                 
  ├──────┼────────────────────────────────────────────────────────────────┼──────────────────────────────────────────────────┤                                                 
  │ 4    │ Hybrid Reviewer (rules + LLM, tiered output A/B/C)             │ Reduces highest-risk component                   │                                               
  ├──────┼────────────────────────────────────────────────────────────────┼──────────────────────────────────────────────────┤                                                 
  │ 5    │ OpenFang adapter with pre-resolved tools                       │ Real agents, runtime-agnostic tool protocol      │                                               
  ├──────┼────────────────────────────────────────────────────────────────┼──────────────────────────────────────────────────┤                                                 
  │ 6    │ ZeroClaw adapter stub                                          │ Ready for comparison                             │                                               
  ├──────┼────────────────────────────────────────────────────────────────┼──────────────────────────────────────────────────┤                                                 
  │ 7    │ Autoresearch with per-agent experiments                        │ Continuous improvement with valid measurement    │                                               
  └──────┴────────────────────────────────────────────────────────────────┴──────────────────────────────────────────────────┘        