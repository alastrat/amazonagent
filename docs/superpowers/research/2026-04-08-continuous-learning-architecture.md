# Continuous Learning Architecture — Design Notes

**Date:** 2026-04-08
**Status:** Design thinking — informs spec revision
**Context:** The account assessment should not be a static snapshot. The platform must continuously learn, adapt strategy, and evolve goals with human approval gates.

---

## The Problem With Static Assessment

The current spec treats assessment as: scan once → generate strategy → execute forever. This misses three things:

1. **The seller's eligibility changes** — they get ungated for new brands, new categories open, restrictions lift
2. **The market changes** — margins shift, competition enters/exits, seasonal patterns emerge
3. **The strategy should learn from outcomes** — which recommendations led to profitable deals? which didn't?

---

## The Continuous Learning Loop

```
                    ┌─────────────────────────┐
                    │   STRATEGY BRIEF         │
                    │   (goals + timeframe)     │
                    │   approved by user        │
                    └──────────┬──────────────┘
                               │
                    ┌──────────▼──────────────┐
                    │   DISCOVERY ENGINE       │
                    │   (directed by strategy)  │
                    │   runs daily in queue     │
                    └──────────┬──────────────┘
                               │
                    ┌──────────▼──────────────┐
                    │   OUTCOMES               │
                    │   deals approved/rejected │
                    │   margins realized        │
                    │   sell-through rates      │
                    └──────────┬──────────────┘
                               │
                    ┌──────────▼──────────────┐
                    │   AUTORESEARCH ENGINE    │
                    │   (Karpathy pattern)      │
                    │   observes → hypothesizes │
                    │   → experiments → learns  │
                    └──────────┬──────────────┘
                               │
                    ┌──────────▼──────────────┐
                    │   STRATEGY EVOLUTION     │
                    │   proposes shifts         │
                    │   user approves/rejects   │
                    └──────────┬──────────────┘
                               │
                               └──────→ back to top
```

---

## Key Design Decisions

### 1. Strategy Brief Has Goals With Timeframes

Not "find profitable products forever." Instead:

```
Strategy Brief v1 (approved 2026-04-08)
├── Goal 1: List 10 products in Home & Kitchen by April 30
│   ├── Target: $2,000/mo revenue from this category
│   └── Search params: min_margin 20%, min_sellers 3, open brands only
├── Goal 2: Get ungated in Grocery by May 15
│   ├── Action: Apply via KeHE distributor invoice
│   └── Estimated unlock: 47 new profitable ASINs
├── Goal 3: Reach $5,000/mo total revenue by June 1
│   └── Requires: goals 1+2 complete + 5 additional products
└── Review date: May 1 (auto-prompted)
```

Users MUST approve the strategy. Goals evolve — the system proposes shifts, users confirm.

### 2. Discovery Runs Daily in a Queue (Not On-Demand Only)

```
Daily Discovery Queue (Inngest cron, per tenant):
├── Check: is discovery enabled for this tenant?
├── Load: current strategy brief → approved goals → search params
├── Scan: eligible categories from strategy (not all categories)
├── Funnel: T0-T3 as built
├── Score: rank survivors by goal alignment
├── Notify: "3 new products found matching Goal 1"
└── Learn: record what was found, what was shown, what user did
```

This replaces the static "create a campaign" model. The system proactively finds products aligned with the seller's approved goals.

### 3. RAG for Contextual Learning

The system accumulates knowledge per tenant that informs future decisions:

**What to store in RAG (vector DB or structured memory):**
- Past deal outcomes: "ASIN B0XYZ had 25% predicted margin but only 12% realized → competitor entered"
- Seller preferences: "User rejected 3 beauty products in a row → deprioritize beauty suggestions"
- Category learnings: "Grocery products from KeHE consistently outperform UNFI for this seller"
- Seasonal patterns: "Q4 toys margins spike 40% — suggest pre-stocking in September"
- Brand insights: "This seller's Brand X products have 2x sell-through vs Brand Y"

**Where RAG feeds into the pipeline:**
- Strategy generation: "Based on your history, I recommend X instead of Y"
- Product ranking: "Similar products you approved had 30% higher margin than ones you rejected"
- Risk assessment: "Last time you sourced in this niche, sell-through was slow"

### 4. Autoresearch (Karpathy Pattern) for Search Parameter Optimization

The original spec (Phase 6) designed this. Now it connects to the per-tenant strategy:

```
Autoresearch Loop (weekly, per tenant):
├── OBSERVE: collect outcome data from PostHog
│   ├── deal_approved / deal_rejected rates
│   ├── margin_realized vs margin_predicted
│   ├── sell-through rates by category
│   └── user engagement with suggestions
│
├── HYPOTHESIZE: generate experiments
│   ├── "Demand weight should increase from 0.20 to 0.30 — rejected deals had high margin but low demand"
│   ├── "Min seller count should decrease from 3 to 2 for Office — this seller's Office products all have 2 sellers"
│   └── "Try expanding to Health category — similar sellers who ungated Health saw 40% revenue increase"
│
├── PROPOSE (human gate): surface to user
│   ├── "I'd like to test different scoring weights for your next scan batch"
│   └── User approves / rejects / modifies
│
├── EXPERIMENT: A/B test via PostHog feature flags
│   ├── Control: current search params
│   ├── Variant: proposed search params
│   └── Split: 50/50 on next 2 scan batches
│
├── EVALUATE: compare outcomes after evaluation window
│   ├── Variant had 20% more approved deals → promote
│   └── Variant had same results → revert
│
└── PROMOTE (human gate): "Variant B found better products. Make this your new default?"
```

### 5. Strategy Evolution With Approval Gates

```
Month 1: Initial strategy (user approved)
  └── Goals: 10 products in Home & Kitchen, ungate Grocery

Month 2: System proposes shift
  └── "You've listed 8/10 Home & Kitchen products. Grocery ungating
       succeeded. I recommend:
       - Goal 1: Expand Home & Kitchen to 20 products (stretch)
       - Goal 2: List 5 Grocery products (new opportunity)
       - Goal 3: Apply for Health ungating (data shows 40% more margin)
       Approve this revised strategy?"

Month 3: Autoresearch finding
  └── "A/B test showed that lowering min_margin from 20% to 15% in
       Grocery found 3x more products with similar sell-through.
       Want me to update your Grocery search parameters?"
```

Every strategy shift requires explicit user approval. The system proposes, the human decides.

---

## How This Changes the Architecture

### New Concepts
- **StrategyGoal** — specific, measurable, time-bound target within a StrategyBrief
- **DiscoveryQueue** — per-tenant daily scan job directed by active goals
- **OutcomeEvent** — structured record of what happened after a recommendation
- **Hypothesis** — autoresearch-generated proposal for parameter change
- **Experiment** — A/B test with control/variant configs via PostHog

### What Stays
- Discovery engine (funnel, catalog, brand intelligence) — becomes the execution layer
- Account assessment — becomes the bootstrap that initializes the learning loop
- OpenFang agents — run the same pipeline, directed by strategy-optimized parameters

### What Changes
- Campaigns become implicit (generated by the discovery queue, not manually created)
- Search parameters are per-goal, not global defaults
- The scoring config is a living document that autoresearch evolves
- PostHog integration becomes critical (not optional) for the feedback loop

---

## Decisions (2026-04-08)

### 1. RAG: pgvector extension in Postgres
Keep everything in Supabase. No separate vector DB. pgvector handles embeddings for seller history, deal outcomes, and strategy context. Embeddings stored alongside structured data in the same DB.

### 2. Experiments: cap at 1 per tenant (for now)
Architecture supports multiple, but limit to 1 concurrent A/B test per tenant to keep costs predictable. Can increase later.

### 3. Goals: revenue/profit within timeframe only
No "list 10 products" goals — too tactical. Goals are outcome-based:
- "Reach $3,000/mo profit by June 1"
- "Achieve 25% average margin across portfolio by July 1"

New accounts (Greenhorn archetype) get longer timeframes because ungating creates a natural delay before revenue is possible. The system should set realistic expectations:
- Greenhorn: 90-day first revenue goal
- RA-to-Wholesale: 30-day first wholesale revenue goal
- Expanding Pro: 14-day incremental revenue goal

### 4. Multi-tenant anonymized learning: approved
Cohort insights feed into strategy suggestions:
- "Sellers at your stage who ungated Grocery first saw 40% faster revenue growth"
- "In your top category, the average margin is 22% — yours is 18%, here's why"
- Privacy: only aggregate stats, never individual seller data

### 5. Strategy versioning with rollback
Strategies are versioned like deployments. Every change creates a new version. Users can rollback to any previous version.

```
strategy_versions
├── id, tenant_id, version_number
├── goals[] (revenue/profit targets + timeframes)
├── search_params (per-goal scoring weights, thresholds)
├── scoring_config_id (links to the ScoringConfig used)
├── status: draft | active | rolled_back | archived
├── promoted_from_experiment_id (nullable — if this came from an A/B win)
├── parent_version_id (what this was derived from)
├── created_at, activated_at, rolled_back_at
├── created_by: system | user | autoresearch
└── change_reason: "Initial strategy" | "Autoresearch promoted variant" | "User adjusted goals" | "Manual rollback"
```

The rollback flow:
```
v1 (active) → autoresearch proposes change → user approves → v2 (active), v1 (archived)
  → v2 performs worse over 2 weeks → user clicks "Rollback to v1"
  → v3 created (copy of v1 params), v2 (rolled_back)
  → v3 is now active, system logs why rollback happened
```

This gives full auditability: why was each strategy version created, what changed, and what happened. The autoresearch engine can learn from rollbacks too — "this type of change tends to get rolled back, reduce confidence in similar hypotheses."

---

## Revised Architecture With Decisions

```
┌─────────────────────────────────────────────────────────────┐
│  ACCOUNT ASSESSMENT (bootstrap — runs once on connect)       │
│  300-ASIN eligibility scan → archetype → initial strategy    │
│  Creates: strategy_versions v1 (user approves)               │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  DAILY DISCOVERY QUEUE (Inngest cron, per tenant)            │
│  Reads: active strategy version → goals → search params      │
│  Runs: funnel T0-T3 on eligible categories/brands only       │
│  Writes: suggestions (products matching goals)                │
│  Uses: pgvector RAG for seller-specific context               │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  OUTCOMES (PostHog events)                                    │
│  deal_approved, deal_rejected, margin_realized,               │
│  suggestion_accepted, suggestion_dismissed,                   │
│  sell_through_rate, revenue_milestone_hit                     │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  AUTORESEARCH ENGINE (weekly, 1 experiment max per tenant)    │
│  Observes: PostHog outcomes                                   │
│  Hypothesizes: parameter changes                              │
│  Proposes: new strategy version (user must approve)           │
│  Experiments: A/B via PostHog flags (1 at a time)             │
│  Learns: from outcomes AND from rollbacks                     │
│  Multi-tenant: anonymized cohort insights                     │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  STRATEGY EVOLUTION (human gate always)                       │
│  System proposes: "Adjust Grocery margin threshold 20→15%"    │
│  User: approves → new version activated                       │
│        rejects → hypothesis archived with reason              │
│        rollbacks → previous version restored, system learns   │
│  Goals evolve: "You hit $3K/mo — new target: $5K by Aug 1"   │
└─────────────────────────────────────────────────────────────┘
```

### RAG Context (pgvector)

What gets embedded and stored for per-tenant retrieval:
- Deal outcomes with reasoning ("B0XYZ rejected: too many sellers entered")
- Seller preference signals ("user rejected 5 beauty products consecutively")
- Category performance history ("Grocery ROI 35% avg, Office ROI 18% avg for this seller")
- Seasonal patterns ("Q4 toys margin was 45%, Q1 dropped to 15%")
- Autoresearch learnings ("lowering margin threshold in Grocery increased approved deals by 30%")

The strategy engine queries RAG when:
- Generating daily discovery params ("what has this seller liked/disliked?")
- Proposing strategy shifts ("what worked for similar sellers?")
- Ranking suggestions ("which products align with this seller's history?")

### pgvector Schema Addition

```sql
-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Seller memory embeddings (RAG)
CREATE TABLE seller_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    content TEXT NOT NULL,
    embedding vector(1536),  -- OpenAI ada-002 or similar
    memory_type TEXT NOT NULL,  -- outcome | preference | learning | seasonal
    entity_type TEXT,  -- deal | brand | category | experiment
    entity_id TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON seller_memory USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
CREATE INDEX idx_sm_tenant ON seller_memory(tenant_id, memory_type);
```
