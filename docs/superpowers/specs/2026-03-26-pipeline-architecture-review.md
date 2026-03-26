# Pipeline Architecture Review — Expert Synthesis

**Date:** 2026-03-26
**Status:** Approved — proceeding to implementation
**Reviewers:** Systems Architect, ML/AI Engineer, FBA Domain Expert

---

## Decision: Go owns pipeline orchestration, agent runtimes are thin executors

Unanimously validated by all three reviewers. The `AgentRuntime` interface becomes per-agent execution. Pipeline sequence, quality gates, retry logic, and config selection stay in Go.

---

## Critical Changes From Review

### 1. Reorder pipeline as elimination funnel

```
Old:  Source → Demand → Competition → Profit → Risk → Supplier → Review
New:  Source → Gate/Risk → Profit → Demand+Competition → Supplier → Review
```

Kill losers early. Gate/Risk eliminates restricted/risky products before any expensive analysis. Profitability kills low-margin products next. Saves 60-70% compute.

### 2. Per-agent composable config versioning

Each agent has its own versioned config (prompt, tools, parameters). A pipeline config is a named composition: `{sourcing: v3, gating: v1, profit: v5, ...}`. Autoresearch varies one agent at a time for single-variable experiments.

### 3. Hybrid Reviewer (rules + LLM)

- Rule-based: required fields present, math correct, BSR in valid range, margin above threshold
- LLM-based: subjective quality only (analysis coherence, risk factor coverage)
- Tiered output: A-tier (recommend), B-tier (worth reviewing), C-tier (only if already in category)
- Track rewrite deltas — terminate early if <5% structured field change
- Max 2 rewrites (not 3) — data shows iteration 3 is noise

### 4. Go owns schema validation

Don't trust runtime's output parsing. Go validates every agent output. Plausibility bounds on all numeric fields.

### 5. Pre-resolved tools (not runtime callbacks)

Go calls external APIs (SP-API, Exa, Firecrawl), passes results to agents as structured context. Agents do reasoning + output generation only. This makes the runtime truly stateless and swappable.

### 6. Deterministic FBA fee calculator

Go function computes FBA fees from product dimensions, weight, category. Never an LLM task. Passed to Profitability agent as input.

### 7. Structured context sharing between agents

Downstream agents receive a lightweight facts envelope from upstream agents — not full reasoning, just structured data (BSR rank, velocity, gating status, flags). Configurable per pipeline config so autoresearch can test whether sharing helps.

---

## Revised Agent Pipeline

```
Campaign Input + PipelineConfig
  │
  ▼
┌──────────────────────────────────────────────────────────────┐
│ 1. SOURCING AGENT                                            │
│    Go pre-resolves: SP-API product search, Exa web search    │
│    Agent reasons: apply ceiling/floor logic, select candidates│
│    Output: 50-200 raw candidate ASINs                        │
│    Config: search parameters, ceiling/floor ratios           │
└────────┬─────────────────────────────────────────────────────┘
         ▼
┌──────────────────────────────────────────────────────────────┐
│ 2. GATE/RISK AGENT (merged — early elimination)              │
│    Go pre-resolves: SP-API category/gating data              │
│    Agent reasons: IP risk, brand restrictions, hazmat, gating│
│    Output: pass/fail + risk flags                            │
│    ~60% of candidates eliminated here                        │
│    Config: risk thresholds, gating rules                     │
└────────┬─────────────────────────────────────────────────────┘
         ▼ (only survivors)
┌──────────────────────────────────────────────────────────────┐
│ 3. PROFITABILITY AGENT                                       │
│    Go pre-resolves: SP-API fees (deterministic calculator),  │
│                     product dimensions, referral rates       │
│    Agent reasons: margin analysis, ROI, cash flow projection │
│    Output: margin score + breakdown                          │
│    ~50% of remaining eliminated (low margin)                 │
│    Config: min margin threshold, cost assumptions            │
└────────┬─────────────────────────────────────────────────────┘
         ▼ (only profitable survivors)
┌──────────────────────────────────────────────────────────────┐
│ 4. DEMAND + COMPETITION AGENT (parallel or merged)           │
│    Go pre-resolves: SP-API BSR/sales data, Exa social data  │
│    Agent reasons: demand signals, competition analysis,      │
│                   buy box dynamics, seller landscape         │
│    Receives context: risk flags, margin data from upstream   │
│    Output: demand score + competition score + evidence       │
│    Config: demand weights, competition thresholds            │
└────────┬─────────────────────────────────────────────────────┘
         ▼
┌──────────────────────────────────────────────────────────────┐
│ 5. SUPPLIER AGENT                                            │
│    Go pre-resolves: Exa search, Firecrawl scrape             │
│    Agent reasons: evaluate supplier candidates, compare,     │
│                   draft outreach                             │
│    Receives context: margin data, risk flags                 │
│    Output: supplier candidates + outreach drafts             │
│    Config: supplier search parameters                        │
└────────┬─────────────────────────────────────────────────────┘
         ▼
┌──────────────────────────────────────────────────────────────┐
│ 6. REVIEWER (hybrid: rules + LLM)                            │
│                                                              │
│    Rule-based checks (deterministic):                        │
│    - All required fields present                             │
│    - Math verification (margin calc matches fee calc)        │
│    - Plausibility bounds (BSR, price, margin ranges)         │
│    - Hard disqualifiers (margin < min, no suppliers found)   │
│                                                              │
│    LLM scoring (subjective):                                 │
│    - Opportunity Viability (1-10)                            │
│    - Execution Confidence (1-10)                             │
│    - Sourcing Feasibility (1-10)                             │
│                                                              │
│    Tiered output:                                            │
│    - A-tier: weighted composite >= 8.0 → auto-recommend      │
│    - B-tier: weighted composite >= 6.5 → worth reviewing     │
│    - C-tier: weighted composite >= 5.0 → niche opportunity   │
│    - Cut: below 5.0 or failed rule checks                    │
│                                                              │
│    Rewrite: if B-tier and iteration < 2, send back           │
│    Track delta: if <5% structured change, terminate early    │
│                                                              │
│    Config: score weights, tier thresholds, rewrite rules     │
└──────────────────────────────────────────────────────────────┘
```

---

## Revised Interface Design

```go
// Per-agent execution — the thin runtime contract
type AgentRuntime interface {
    RunAgent(ctx context.Context, task AgentTask) (*AgentOutput, error)
}

type AgentTask struct {
    AgentName    string            // "sourcing", "gating", "profitability", etc.
    SystemPrompt string            // from per-agent config
    Input        map[string]any    // pre-resolved tool data + candidate info
    Context      []AgentContext    // upstream agent facts (optional, configurable)
    OutputSchema map[string]any    // expected JSON schema for validation
}

type AgentOutput struct {
    Structured map[string]any     // parsed structured output
    Raw        string             // raw LLM response for debugging
    TokensUsed int                // for cost tracking
    DurationMs int64              // for latency tracking
}

// Upstream facts shared between agents (not full reasoning)
type AgentContext struct {
    AgentName string
    Facts     map[string]any     // e.g., {"bsr_rank": 4500, "gated": false}
    Flags     []string           // e.g., ["ip_risk", "oversized"]
}
```

---

## PipelineConfig Model

```go
type PipelineConfig struct {
    ID        string
    TenantID  string
    Name      string                          // "baseline-v3", "experiment-demand-v2"
    Agents    map[string]AgentConfig           // per-agent configs, keyed by agent name
    Scoring   ScoringWeights
    Thresholds PipelineThresholds
    CreatedBy string                           // "user" | "autoresearch"
    CreatedAt time.Time
}

type AgentConfig struct {
    Version      int
    SystemPrompt string
    Tools        []string                      // which pre-resolved tools to use
    Parameters   map[string]any                // agent-specific params
    ModelTier    string                        // "fast" | "standard" | "premium"
}

type PipelineThresholds struct {
    MinMarginPct       float64                 // hard floor for profitability
    RiskMaxScore       int                     // max acceptable risk score
    TierA              float64                 // weighted composite >= this → A-tier
    TierB              float64                 // weighted composite >= this → B-tier
    MaxRewriteIterations int                   // default 2
    RewriteMinDelta    float64                 // min % change to justify rewrite
}
```

---

## Infrastructure Decisions

| Concern | Decision | Rationale |
|---------|----------|-----------|
| Execution state | Inngest steps | Durable, resumable, handles retries natively |
| Business state | Supabase | Queryable, auditable, experiment analysis |
| Schema validation | Go (authoritative) | Don't trust runtime's parsing |
| Tool execution | Pre-resolved by Go | Runtime-agnostic, cacheable, verifiable |
| Rate limiting | Global token bucket per LLM provider | Prevents experiment interference |
| Cost tracking | Per-campaign, per-agent | Budget caps halt execution |
| FBA fee calculation | Deterministic Go function | Never an LLM task |

---

## Implementation Order

| Step | What |
|------|------|
| 1 | Refactor AgentRuntime to per-agent + PipelineConfig + AgentConfig models |
| 2 | Pipeline orchestration in Go (funnel sequence, early elimination, context passing) |
| 3 | Update simulator to new interface |
| 4 | Deterministic FBA fee calculator |
| 5 | Hybrid Reviewer (rules + LLM, tiered output) |
| 6 | OpenFang in docker-compose + adapter |
| 7 | ZeroClaw adapter stub |
| 8 | Autoresearch with per-agent experiments + PostHog |

---

## Future Product Enhancements (from FBA expert)

Not blocking current work, but important for roadmap:

- Reverse sourcing from distributor catalogs (higher value than ASIN discovery)
- Reorder management (70% of the work, 90% of the profit)
- Seasonality, return rates, storage fees in scoring
- Curated distributor database instead of web-scraped suppliers
- Portfolio-level optimization (capital allocation across SKUs)
- Weekly discovery cadence (not nightly) + daily monitoring for active products
