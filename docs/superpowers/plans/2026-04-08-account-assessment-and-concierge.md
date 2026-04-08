# Account Assessment + Concierge — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform the platform from a product scanner into an AI concierge that assesses each seller's situation, generates a versioned growth strategy, and continuously discovers products aligned with approved goals.

**Spec:** [Account Assessment Service](../specs/2026-04-08-account-assessment-service.md)
**Research:** [AI Concierge Brainstorm](../research/2026-04-08-ai-concierge-expert-brainstorm.md), [Continuous Learning Architecture](../research/2026-04-08-continuous-learning-architecture.md)

**Architecture:** Assessment bootstraps a learning loop. Strategy Brief has versioned goals (revenue/profit + timeframe). Daily discovery queue runs automatically directed by active strategy. Autoresearch proposes parameter changes. User approves every strategy shift. Full rollback to any previous version.

**Tech Stack:** Go 1.23+, Postgres (pgvector for RAG), Supabase, Inngest, PostHog, SP-API, existing hexagonal architecture

---

## Dependency Graph

```
Phase 1: Seller Profile + Eligibility Scan (foundation)
    │
    ├── Phase 2: Strategy Brief + Versioning (depends on 1)
    │       │
    │       ├── Phase 3: Daily Discovery Queue (depends on 2)
    │       │
    │       └── Phase 4: Onboarding Frontend (depends on 2)
    │
    └── Phase 5: RAG + Autoresearch (depends on 1, 2, 3)
            │
            └── Phase 6: Multi-Tenant Cohort Learning (depends on 5)
```

Phases 3 and 4 are independent and can be parallelized after Phase 2.
Phases 5 and 6 are the continuous improvement layer — can ship after Phases 1-4 are live.

---

## Phase 1: Seller Profile + Eligibility Scan

**Delivers:** When a seller connects their SP-API credentials, the system runs a 300-ASIN eligibility scan and builds their profile (archetype, category scores, brand map).

### Task 1.1: Domain Types

**Files:** Create `internal/domain/seller_profile.go`

- [ ] SellerProfile (tenant_id, archetype, stage, account_age_days, assessment_status)
- [ ] SellerArchetype enum (greenhorn, ra_to_wholesale, expanding_pro, capital_rich)
- [ ] EligibilityFingerprint (tenant_id, category_scores[], brand_results[], probe_results[], confidence, assessed_at)
- [ ] CategoryEligibility (category, open_count, gated_count, open_rate, sample_brands[])
- [ ] BrandEligibilityResult (brand, category, status, sample_asin, reason)
- [ ] AssessmentSampleASIN (asin, category, brand, tier, expected_gating)
- [ ] Verify build

### Task 1.2: Assessment Sample Database

**Files:** Create `internal/domain/assessment_samples.go`

The 300 probe ASINs need to be curated — real ASINs from real categories with known brand gating patterns. This is a static dataset that ships with the code.

- [ ] Define the category × brand × tier matrix (30 categories × 10 ASINs)
- [ ] Tier 1: 10 high-volume categories (Home & Kitchen, Office, Sports, etc.) × 4 samples
- [ ] Tier 2: 10 commonly gated categories (Grocery, Beauty, Health, Toys, etc.) × 3 samples
- [ ] Tier 3: 10 niche categories × 2 samples
- [ ] Brand probes: top 25 wholesale brands × 2 ASINs
- [ ] Calibration: 10 known-open ASINs
- [ ] Each sample has: ASIN, category, brand_name, tier (top/mid/generic), expected_gating

**Note:** This requires manual research to populate with real ASINs. Can be seeded with SP-API catalog searches per category during development.

### Task 1.3: Database Migrations

**Files:** Create migrations

- [ ] `010_seller_profiles.sql` — seller_profiles table
- [ ] `011_eligibility_fingerprints.sql` — eligibility_fingerprints + category_eligibilities + brand_eligibility_results tables
- [ ] `012_assessment_probes.sql` — assessment_probe_results table (raw scan results)

### Task 1.4: Port Interfaces

**Files:** Create `internal/port/assessment.go`

- [ ] SellerProfileRepo interface (Create, Get, Update)
- [ ] EligibilityFingerprintRepo interface (Create, Get, GetByTenant)
- [ ] EligibilitySampler interface (RunScan(ctx, tenantID, samples) → results)

### Task 1.5: Postgres Repos

**Files:** Create repos for profile, fingerprint, probes

- [ ] seller_profile_repo.go
- [ ] eligibility_fingerprint_repo.go
- [ ] Verify build

### Task 1.6: Assessment Service

**Files:** Create `internal/service/assessment_service.go`

- [ ] `RunAssessment(ctx, tenantID)` — orchestrates the full scan
  1. Load sample ASINs (static dataset)
  2. Batch SP-API CheckListingEligibility (300 calls, rate limited)
  3. Aggregate results → CategoryEligibility scores
  4. Classify archetype (decision tree on account age + open rate + listing count)
  5. Build EligibilityFingerprint
  6. Create/update SellerProfile
- [ ] Unit tests for archetype classification
- [ ] Unit tests for category score aggregation

### Task 1.7: Inngest Assessment Workflow

**Files:** Modify `internal/adapter/inngest/client.go`

- [ ] `assessment/requested` event trigger
- [ ] Steps: create-profile → scan-eligibility (batched) → aggregate-scores → classify-archetype → save-fingerprint
- [ ] Progress events emitted per batch (for frontend progress bar)

### Task 1.8: API Endpoints

**Files:** Create `internal/api/handler/assessment_handler.go`, modify router

- [ ] `POST /assessment/start` — triggers assessment for current tenant
- [ ] `GET /assessment/status` — returns assessment progress
- [ ] `GET /assessment/profile` — returns SellerProfile + EligibilityFingerprint
- [ ] Wire into main.go + router

### Task 1.9: Wire + Test

- [ ] Wire all new services into main.go
- [ ] Run `go build ./...` + `go test ./...`
- [ ] Deploy to Railway
- [ ] Test: trigger assessment via API → verify fingerprint is built

**CHECKPOINT 1:** A seller can trigger an assessment, the system scans 300 ASINs, and returns a profile with archetype + category scores + brand eligibility map.

---

## Phase 2: Strategy Brief + Versioning

**Delivers:** System generates a personalized strategy with revenue/profit goals and timeframes. Strategies are versioned — every change creates a new version. Full rollback to any previous version.

### Task 2.1: Domain Types

**Files:** Create `internal/domain/strategy.go`

- [ ] StrategyVersion (id, tenant_id, version_number, goals[], search_params, scoring_config_id, status, parent_version_id, promoted_from_experiment_id, change_reason, created_by, created_at, activated_at, rolled_back_at)
- [ ] StrategyStatus enum (draft, active, rolled_back, archived)
- [ ] StrategyGoal (type: revenue|profit, target_amount, currency, timeframe_start, timeframe_end, target_categories[], current_progress)
- [ ] StrategySearchParams (per-goal: min_margin, min_sellers, eligible_categories[], eligible_brands[], scoring_weights)
- [ ] CategoryRecommendation (category, priority_score, rationale, estimated_monthly_revenue, ungating_required)
- [ ] UngatingRecommendation (brand_or_category, difficulty, estimated_unlock_value, suggested_distributor, action_steps[])

### Task 2.2: Database Migration

- [ ] `013_strategy_versions.sql` — strategy_versions table + strategy_goals table
- [ ] Indexes on tenant_id + status, version_number

### Task 2.3: Port Interfaces

**Files:** Create `internal/port/strategy.go`

- [ ] StrategyVersionRepo (Create, GetActive, GetByID, List, Activate, Rollback)
- [ ] StrategyEngine interface (GenerateInitialStrategy, ProposeEvolution)

### Task 2.4: Strategy Service

**Files:** Create `internal/service/strategy_service.go`

- [ ] `GenerateInitialStrategy(ctx, tenantID, fingerprint, archetype)` → StrategyVersion v1
  - Uses category prioritization formula from research
  - Sets goals based on archetype (Greenhorn: 90-day, Pro: 14-day timeframe)
  - Goals are revenue/profit targets ONLY
  - Returns draft → user must approve to activate
- [ ] `ActivateVersion(ctx, tenantID, versionID)` — sets as active, archives previous
- [ ] `RollbackToVersion(ctx, tenantID, targetVersionID)` — creates NEW version with old params, marks current as rolled_back
- [ ] `GetActiveStrategy(ctx, tenantID)` → current active version
- [ ] `ListVersions(ctx, tenantID)` → full version history
- [ ] Tests for versioning logic (activate, rollback, version numbering)

### Task 2.5: Strategy Generation (LLM-assisted)

**Files:** Create `internal/service/strategy_generator.go`

- [ ] Takes: EligibilityFingerprint + SellerArchetype + category scores
- [ ] Produces: recommended goals (revenue targets), category priorities, ungating roadmap
- [ ] LLM used for: narrative rationale ("here's why Grocery should be your first focus")
- [ ] Deterministic: scoring formula, goal timeframes, category ranking
- [ ] NOT learning from seller preferences (only from objective data)

### Task 2.6: API Endpoints

- [ ] `GET /strategy` — get active strategy version
- [ ] `GET /strategy/versions` — list all versions
- [ ] `GET /strategy/versions/:id` — specific version detail
- [ ] `POST /strategy/versions/:id/activate` — approve and activate
- [ ] `POST /strategy/versions/:id/rollback` — rollback to this version
- [ ] Wire into main.go + router

### Task 2.7: Connect Assessment → Strategy

- [ ] After assessment completes, auto-generate initial strategy (draft)
- [ ] Notify user: "Your strategy is ready for review"

**CHECKPOINT 2:** Assessment generates a strategy brief with revenue goals. User can approve, view history, and rollback. Strategy versions are fully auditable.

---

## Phase 3: Daily Discovery Queue

**Delivers:** The discovery engine runs daily per tenant, directed by the active strategy. Products found are presented as suggestions aligned with goals. No more manual campaign creation needed.

### Task 3.1: Domain Types

- [ ] DiscoverySuggestion (id, tenant_id, strategy_version_id, goal_id, asin, title, brand, category, estimated_margin, reason, status: pending|accepted|dismissed, created_at)

### Task 3.2: Discovery Queue Service

**Files:** Create `internal/service/discovery_queue_service.go`

- [ ] `RunDailyDiscovery(ctx, tenantID)`:
  1. Load active strategy → goals → search params
  2. For each goal's target categories: run funnel (T0-T3) with goal-specific params
  3. Rank survivors by goal alignment
  4. Create DiscoverySuggestion for each (not deals — suggestions need approval)
  5. Cap at 20 suggestions per day per tenant

### Task 3.3: Inngest Daily Cron

- [ ] `discovery/daily` cron (runs per active tenant, e.g., 3 AM UTC)
- [ ] Loads active strategy, runs discovery queue, creates suggestions
- [ ] Replaces manual campaign creation for ongoing discovery

### Task 3.4: Suggestion API

- [ ] `GET /suggestions` — list pending suggestions
- [ ] `POST /suggestions/:id/accept` — creates a deal from the suggestion
- [ ] `POST /suggestions/:id/dismiss` — marks as dismissed (does NOT train preferences)

### Task 3.5: Connect to Existing Pipeline

- [ ] Accepted suggestions trigger the existing evaluate-candidate pipeline (LLM agents)
- [ ] Or skip LLM if strategy confidence is high enough (configurable)

**CHECKPOINT 3:** The system proactively finds products daily, aligned with the seller's approved goals. No manual campaigns needed.

---

## Phase 4: Onboarding Frontend

**Delivers:** The "Wealthfront moment" — connect account, see strategy in < 5 minutes.

### Task 4.1: Onboarding Flow (4 screens)

- [ ] **Connect screen**: SP-API OAuth button, "Connect your Amazon account"
- [ ] **Discover screen**: Live progress — categories being scanned, brands being checked, animated progress bar
- [ ] **Reveal screen**: Strategy Brief — top categories, quick-win brands, ungating roadmap, revenue goals
- [ ] **Commit screen**: "Approve this strategy" button → activates v1

### Task 4.2: Strategy Dashboard

- [ ] Strategy overview card on dashboard (active version, goals with progress bars)
- [ ] "View strategy history" → version list with rollback buttons
- [ ] Goal progress tracking (revenue/profit vs target)

### Task 4.3: Suggestions Feed

- [ ] Suggestions list on dashboard (replaces empty deals view for new sellers)
- [ ] Accept/dismiss buttons per suggestion
- [ ] Funnel stats: "Scanned 500 products → 12 match your strategy"

**CHECKPOINT 4:** Complete onboarding experience. New sellers connect, get assessed, see strategy, approve, and start receiving daily suggestions.

---

## Phase 5: RAG + Autoresearch (Continuous Learning)

**Delivers:** System learns from outcomes and proposes strategy improvements. A/B testing via PostHog. pgvector for contextual memory.

### Task 5.1: pgvector Setup

- [ ] Enable pgvector extension in Supabase
- [ ] `014_seller_memory.sql` — seller_memory table with vector(1536) column
- [ ] Embedding service (calls OpenAI ada-002 or similar)
- [ ] Memory write: after deal outcomes (margin realized, sell-through)
- [ ] Memory read: strategy engine queries similar past outcomes

### Task 5.2: Outcome Events

- [ ] Extend PostHog event capture: deal_approved, margin_realized, sell_through_rate, suggestion_accepted, goal_progress
- [ ] Structured outcome records in Postgres (not just PostHog)

### Task 5.3: Autoresearch Engine

**Files:** Create `internal/service/autoresearch_service.go`

- [ ] `AnalyzeOutcomes(ctx, tenantID)` — weekly analysis of PostHog data
- [ ] `GenerateHypothesis(ctx, tenantID, outcomes)` — propose parameter changes
- [ ] `CreateExperiment(ctx, tenantID, hypothesis)` — A/B test setup (1 max per tenant)
- [ ] `EvaluateExperiment(ctx, experimentID)` — compare control vs variant
- [ ] `ProposePromotion(ctx, experimentID)` — suggest new strategy version if variant wins

### Task 5.4: Experiment Workflow

- [ ] Inngest weekly cron: analyze → hypothesize → propose
- [ ] PostHog feature flags for A/B variant assignment
- [ ] Evaluation after configurable window (default: 2 weeks)
- [ ] User approval gate for promotion or rollback

### Task 5.5: RAG-Enhanced Strategy Generation

- [ ] Strategy generator queries pgvector for: similar sellers' outcomes, category performance history, seasonal patterns
- [ ] Context window: last 90 days of outcome data
- [ ] NOT querying: seller accept/reject signals (prevents bias)

**CHECKPOINT 5:** System learns from real outcomes, proposes improvements, A/B tests changes, and promotes winners with human approval.

---

## Phase 6: Multi-Tenant Cohort Learning

**Delivers:** Anonymized insights from all tenants improve individual recommendations.

### Task 6.1: Cohort Aggregation

- [ ] Nightly Inngest job: compute anonymized stats across tenants
- [ ] Cohort dimensions: seller archetype, account age bucket, primary category
- [ ] Metrics: avg margin by category, ungating success rate, revenue growth rate

### Task 6.2: Cohort Insights

- [ ] `CohortInsight` domain type (insight_text, confidence, applicable_archetypes, created_at)
- [ ] Surface in strategy generation: "Sellers at your stage who ungated Grocery saw 40% faster growth"
- [ ] Surface in suggestions: "This brand performs well for sellers like you"

### Task 6.3: Privacy Controls

- [ ] Only aggregate stats (min 10 sellers per cohort before surfacing)
- [ ] No individual seller data exposed
- [ ] Tenant can opt out of cohort contribution

**CHECKPOINT 6:** Platform intelligence compounds across tenants while preserving privacy.

---

## Implementation Priority

| Phase | What | Effort | Priority |
|-------|------|--------|----------|
| **1** | Seller Profile + Eligibility Scan | 1-2 weeks | **NOW** — solves the immediate "everything restricted" problem |
| **2** | Strategy Brief + Versioning | 1-2 weeks | **NOW** — the concierge value proposition |
| **3** | Daily Discovery Queue | 1 week | **NEXT** — replaces manual campaigns |
| **4** | Onboarding Frontend | 1-2 weeks | **NEXT** — the "Wealthfront moment" |
| **5** | RAG + Autoresearch | 2-3 weeks | **LATER** — continuous improvement |
| **6** | Multi-Tenant Cohort | 1 week | **LATER** — compounds over time |

Phases 1-4 ship the concierge MVP. Phases 5-6 add the learning loop.
