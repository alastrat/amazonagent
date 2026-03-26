# FBA Agent Orchestrator — System Design Spec

**Date:** 2026-03-25
**Status:** Draft — pending review
**Scope:** Full architecture design + MVP focus on Product Sourcing & Deal Scoring

---

## 1. Product Vision

A multi-tenant SaaS platform for Amazon FBA wholesale resellers. The system uses specialized AI agents to source, analyze, score, procure, list, and monitor wholesale products on Amazon — with humans approving every critical decision.

Two interaction surfaces:
- **Dashboard** (Next.js) — visual operational interface
- **Chat** (OpenFang channels: WhatsApp, Telegram, Slack) — conversational interface

Both drive the same backend. The system learns over time via an autoresearch-inspired continuous improvement engine integrated with PostHog.

---

## 2. System Boundaries

Three layers, each owning its domain:

```
┌─────────────────────────────────────────────────────────────┐
│  RESEARCH PIPELINE (OpenFang / ZeroClaw)                    │
│  Owns: product discovery, scoring, quality gates,           │
│        listing content generation                           │
│  Tools: SP-API, Exa, Firecrawl, OpenAI                     │
│  Channels: WhatsApp, Telegram, Slack (user interaction)     │
└──────────────────────┬──────────────────────────────────────┘
                       │ structured research results
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  OPERATIONS CORE (Go backend)                               │
│  Owns: tenancy, auth, deal lifecycle, approvals, suppliers, │
│        inventory, CRM, fulfillment, disputes, ads,          │
│        listing management, domain events, audit trail,      │
│        configuration (scoring thresholds, pipeline params)  │
│  Persistence: Supabase (Postgres + Auth + RLS)              │
│  Durable execution: Inngest                                 │
│  API: REST for dashboard + agent tool calls                 │
└──────────────────────┬──────────────────────────────────────┘
                       │ domain events + metrics
                       ▼
┌─────────────────────────────────────────────────────────────┐
│  CONTINUOUS IMPROVEMENT (Autoresearch pattern + PostHog)     │
│  Owns: hypotheses, experiments, variant assignment,          │
│        shadow evaluation, outcome comparison, keep/revert   │
│  PostHog: feature flags, A/B variants, event analytics      │
│  Cadence: autonomous iteration with human review gates      │
└─────────────────────────────────────────────────────────────┘
```

### Contracts between layers

- **Research -> Operations:** Research pipeline produces `ResearchResult` (scored product candidates with evidence and agent audit trail). Operations Core consumes these as input to the deal lifecycle.
- **Operations -> Improvement:** Operations emits domain events (deal approved, margin realized, supplier responded, listing performance). PostHog captures these. Autoresearch engine reads outcomes to propose experiments.
- **Improvement -> Research:** Experiment variants modify research pipeline behavior (scoring weights, threshold parameters, agent prompt variants). These flow back as configuration, not code changes.
- **Improvement -> Operations:** Promoted experiment configs update the tenant's active scoring configuration via the Operations Core API.

---

## 3. Technology Stack

| Concern | Technology |
|---------|-----------|
| Backend | Go (hexagonal architecture) |
| Frontend | Next.js App Router + TypeScript + Tailwind + shadcn/ui |
| Auth + DB | Supabase (Postgres + Auth + RLS) |
| Agent orchestration | OpenFang (primary), ZeroClaw (comparative evaluation planned) |
| Agent tools | Amazon SP-API, Exa, Firecrawl, OpenAI |
| Durable execution | Inngest |
| Analytics + flags + experiments | PostHog |
| Continuous improvement | Autoresearch-inspired engine + PostHog |
| Chat channels | WhatsApp, Telegram, Slack (via OpenFang channel adapters) |

### Architecture principles (from CLAUDE.md)

- Business logic decoupled from orchestration runtimes
- All external systems behind interfaces
- No service directly depends on OpenFang, ZeroClaw, or Inngest — always through internal interfaces
- Agents produce suggestions, not final decisions
- All critical actions require human validation or approval
- Prefer shadow mode before rollout
- Never auto-apply risky changes

---

## 4. Two Sourcing Modes

### Mode 1: Continuous Discovery (always-on)

A tenant-level background process that runs on a configurable schedule (nightly, twice daily, weekly). Feeds the tenant's catalog continuously with new opportunities.

- Uses the tenant's current best-known scoring configuration
- Candidates that pass quality gates appear as "discovered" deals in the dashboard
- Autoresearch observes outcomes over time to propose config improvements
- Config improvements are tested via Campaigns before being promoted to continuous discovery

### Mode 2: Campaigns (scoped, intentional)

User-initiated or autoresearch-initiated research runs with specific criteria that may differ from baseline.

Use cases:
- Analyze a supplier's spreadsheet (bulk upload)
- Explore a new category
- Test a hypothesis (different margin threshold, new scoring weights)
- A/B experiment (variant campaign vs. control campaign)

A continuous discovery run creates a Campaign of type `discovery_run` each execution, so everything traces back to a campaign for audit purposes.

### How they relate

Continuous discovery is the **production system**. Campaigns are the **experiment lab**. Autoresearch uses campaigns to test improvements before promoting them to the always-on process.

```
Continuous Discovery (baseline config)
        │
        │ Autoresearch observes outcomes, proposes improvement
        ▼
Campaign (experiment: control config vs. variant config)
        │
        │ Variant wins with statistical significance
        ▼
Human approves promotion → Variant becomes new baseline
```

---

## 5. Research Pipeline (OpenFang / ZeroClaw)

### Campaign Input

All entry points produce the same normalized input:

```
Campaign {
  tenant_id
  trigger_type: chat | dashboard | scheduler | spreadsheet
  criteria {
    categories/keywords
    min_monthly_revenue
    min_margin_pct
    max_wholesale_cost
    max_moq
    preferred_brands (optional)
    marketplace (US/EU/UK)
  }
  scoring_config_id
  source_file (optional, for spreadsheet uploads)
}
```

### Agent Pipeline (sequential, quality-gated)

```
Campaign Input
  │
  ▼
┌─────────────────────────┐
│ 1. SOURCING AGENT       │  Tools: SP-API, Exa, Firecrawl
│    Find candidate ASINs │  Output: ~50-200 raw candidates
│    Ceiling/floor logic   │  with BSR, price, category, seller count
└────────┬────────────────┘
         ▼
┌─────────────────────────┐
│ 2. DEMAND AGENT         │  Tools: SP-API (sales estimates), Exa
│    Score demand signals  │  Output: demand score + evidence
│    Velocity, trends,     │  (monthly units, trend direction,
│    social sentiment      │   sentiment quotes)
└────────┬────────────────┘
         ▼
┌─────────────────────────┐
│ 3. COMPETITION AGENT    │  Tools: SP-API
│    Gating check          │  Output: competition score + evidence
│    FBA seller count      │  (gated?, seller count, review velocity,
│    Buy box analysis      │   buy box rotation, PPC intensity)
└────────┬────────────────┘
         ▼
┌─────────────────────────┐
│ 4. PROFITABILITY AGENT  │  Tools: SP-API (fees), calculator
│    Real margin calc      │  Output: margin score + breakdown
│    FBA fees, referral,   │  (wholesale cost, landed cost, all fees,
│    shipping, returns     │   net margin %, ROI, 90-day cash flow)
└────────┬────────────────┘
         ▼
┌─────────────────────────┐
│ 5. RISK AGENT           │  Tools: SP-API, Exa
│    IP/brand risk         │  Output: risk score + flags
│    Listing quality       │  (IP complaints, brand gating,
│    Hazmat/restrictions   │   listing suppression risk, hazmat)
└────────┬────────────────┘
         ▼
┌──────────────────────────────┐
│ 6. SUPPLIER AGENT            │  Tools: Exa, Firecrawl, supplier DBs
│    Find 3-5 wholesalers      │  Output: supplier candidates + comparison
│    per candidate             │
│    Compare: unit cost, MOQ,  │  For each supplier:
│    lead time, authorization, │    - company, contact, MOQ, lead time
│    shipping terms            │    - unit price, shipping terms
│                              │    - authorization status
│    Draft outreach template   │    - reliability signals
│    per top supplier          │  + best-price recommendation
│                              │  + personalized outreach draft
└────────┬─────────────────────┘
         │
         │  If real supplier price differs >5% from
         │  Profitability Agent estimate:
         │  ──→ Profitability Agent re-scores with real prices
         │
         ▼
┌─────────────────────────────────────────┐
│ 7. REVIEWER AGENT (the "Boss")          │
│    Scores each candidate on 3 axes:     │
│                                          │
│    - Opportunity Viability (1-10)        │
│    - Execution Confidence (1-10)         │
│    - Sourcing Feasibility (1-10)         │
│                                          │
│    All three >= 8 → PASS                │
│    Any score 6-7 → REWRITE              │
│      (sent back to relevant agent)       │
│    Any score < 6 → CUT                   │
│                                          │
│    Max 3 rewrite loops per candidate     │
└────────┬────────────────────────────────┘
         ▼
  ResearchResult {
    campaign_id
    candidates[] {
      asin, title, brand, category
      scores { demand, competition, margin, risk,
               sourcing_feasibility, overall }
      evidence { per-agent reasoning + data }
      supplier_candidates[] {
        company, contact, unit_price, moq,
        lead_time, shipping_terms, authorization
      }
      outreach_drafts[]
      reviewer_verdict + justification
      iteration_count
    }
    research_trail (full agent audit log)
    summary (top 5-10 ready-to-source products)
  }
```

### Pipeline design decisions

1. **Each agent has one job, structured output.** No agent sees another agent's reasoning — only the Reviewer sees all scores together. Prevents groupthink.
2. **Rewrite loops are bounded.** Max 3 iterations per candidate. Prevents infinite loops and controls cost.
3. **The Reviewer doesn't rewrite — it sends back.** If demand score is weak, the Demand Agent re-runs, not the Reviewer.
4. **Spreadsheet upload joins at step 2.** The Sourcing Agent is skipped — the spreadsheet provides candidate ASINs directly. Steps 2-7 run identically.
5. **Profitability re-scores when real prices arrive.** Avoids blocking on supplier data while ensuring final approval uses real numbers.
6. **The entire pipeline is auditable.** Every agent call, score, and rewrite is stored as the research trail.

### What OpenFang provides

- Agent definitions (system prompts, tool bindings per agent)
- Sequential execution orchestration with quality gate logic
- Tool calling (SP-API, Exa, Firecrawl, OpenAI)
- Memory (past research results for agent context)
- Channel routing (user gets progress updates via WhatsApp/Telegram/Slack)

### What OpenFang does NOT own

- Scoring thresholds and weights (come from Operations Core, modifiable by Improvement layer)
- Deal lifecycle after research completes
- Approval decisions
- Any persistent business state

---

## 6. Operations Core (Go Backend)

### Responsibilities

- Multi-tenancy and auth (Supabase)
- Deal lifecycle (state machine from research result to sold product)
- Approval engine (human gates for outreach, POs, listings)
- Supplier CRM (contacts, outreach tracking, PO history)
- Inventory and fulfillment (FBA inbound, stock levels, reorder)
- Listing management (SP-API publish, monitoring)
- Domain events and audit trail (every state transition, every decision)
- Configuration (scoring thresholds, pipeline parameters — the knobs the Improvement layer turns)

### Deal Lifecycle State Machine

```
                    ┌──────────────┐
                    │  discovered  │  <- Research pipeline drops candidates here
                    └──────┬───────┘
                           ▼
                    ┌──────────────┐
                    │  analyzing   │  <- Agents scoring in progress
                    └──────┬───────┘
                           ▼
                  ┌────────────────┐
                  │  needs_review  │  <- Passed quality gate, awaiting human
                  └───┬────────┬───┘
                      │        │
              approve │        │ reject
                      ▼        ▼
            ┌──────────┐  ┌──────────┐
            │ approved  │  │ rejected │
            └────┬─────┘  └──────────┘
                 ▼
          ┌─────────────┐
          │  sourcing    │  <- Supplier outreach in progress
          └──────┬──────┘
                 ▼
          ┌─────────────┐
          │  procuring   │  <- PO placed, awaiting delivery
          └──────┬──────┘
                 ▼
          ┌─────────────┐
          │  listing     │  <- Agents generating listing content
          └──────┬──────┘
                 ▼
          ┌─────────────┐
          │  live        │  <- Published on Amazon, selling
          └──────┬──────┘
                 ▼
          ┌─────────────┐
          │  monitoring  │  <- Ongoing performance tracking
          └──────┬──────┘
                 │
        ┌────────┴────────┐
        ▼                 ▼
 ┌────────────┐   ┌────────────┐
 │  reorder   │   │  archived  │
 └────────────┘   └────────────┘
```

Every state transition emits a domain event. Every event is persisted and forwarded to PostHog.

### Multi-Tenancy Model

- **Row-level security via Supabase RLS.** Every table has a `tenant_id` column. RLS policies enforce isolation at the database level.
- **Tenant context propagation:** Auth middleware extracts tenant from JWT -> injects into Go `context.Context` -> every query includes tenant filter -> RLS as safety net, not sole mechanism.
- **For Inngest workflows:** Tenant ID is a required workflow input parameter, carried through all steps. Workflows never run without tenant context.
- **For OpenFang agents:** Tenant ID is passed as part of the campaign input and included in every tool call to the Operations Core API.

### Domain Models

```
DiscoveryConfig {
  id, tenant_id,
  categories[], baseline_criteria,
  scoring_config_id (current active config),
  cadence, enabled,
  last_run_at, next_run_at
}

Campaign {
  id, tenant_id,
  type: discovery_run | manual | experiment,
  criteria,
  scoring_config_id (may differ from baseline if experiment),
  experiment_id (nullable — links to A/B test),
  source_file (nullable — for spreadsheet uploads),
  status: pending | running | completed | failed,
  created_by (user | system | autoresearch),
  created_at, completed_at
}

Deal {
  id, tenant_id, campaign_id, asin, title, brand, category,
  status (state machine above),
  scores { demand, competition, margin, risk,
           sourcing_feasibility, overall },
  evidence (JSON — per-agent reasoning),
  reviewer_verdict, iteration_count,
  supplier_id (once assigned),
  listing_id (once created),
  created_at, updated_at
}

Supplier {
  id, tenant_id, name, website, authorization_status,
  contacts[], notes[], reliability_score
}

SupplierQuote {
  id, supplier_id, deal_id, unit_price, moq, lead_time,
  shipping_terms, valid_until, status
}

Outreach {
  id, deal_id, supplier_id, template_content,
  status (draft/approved/sent/responded),
  approved_by, sent_at, response_at
}

Listing {
  id, deal_id, asin, title, bullets, description,
  backend_keywords, a_plus_content,
  status (draft/reviewing/published),
  sp_api_listing_id, published_at
}

PurchaseOrder {
  id, tenant_id, supplier_id, deals[],
  total_cost, status (draft/confirmed/shipped/received),
  approved_by, confirmed_at
}

Approval {
  id, tenant_id, entity_type, entity_id,
  requested_by, decided_by, decision, reason,
  created_at, decided_at
}

DomainEvent {
  id, tenant_id, event_type, entity_type, entity_id,
  payload (JSON), correlation_id, actor_id, timestamp
}

ScoringConfig {
  id, tenant_id, version,
  weights { demand, competition, margin, risk, sourcing },
  thresholds { min_overall, min_per_dimension },
  agent_prompt_variants (JSON — per-agent prompt overrides),
  created_by (user | autoresearch),
  active (boolean — is this the current baseline?),
  created_at
}
```

### Durable Execution (Inngest)

Long-running orchestration between Operations Core and agent pipelines:

- **CampaignProcessingWorkflow** — triggers research pipeline, waits for results, creates deals
- **DealApprovalWorkflow** — waits for human decision, routes to next stage
- **SupplierOutreachWorkflow** — sends approved outreach, waits for response, schedules follow-ups
- **ProcurementWorkflow** — tracks PO from confirmation through FBA inbound
- **ListingWorkflow** — triggers listing agents, waits for human review, publishes via SP-API
- **MonitoringWorkflow** — periodic scan of live products, emits alerts
- **DiscoverySchedulerWorkflow** — triggers continuous discovery campaigns on cadence

Each workflow is resumable — pauses at human approval points and resumes when the decision arrives.

### API Surface

REST API consumed by dashboard and invoked by OpenFang agents as tools:

```
POST   /campaigns                     Create campaign
GET    /campaigns/:id                 Campaign status + results

GET    /deals                         List, filter, sort, paginate
GET    /deals/:id                     Full detail + evidence + timeline
POST   /deals/:id/approve             Human approval
POST   /deals/:id/reject              Human rejection

GET    /suppliers                     List suppliers
POST   /suppliers/:id/outreach        Trigger outreach
GET    /suppliers/:id/quotes          Supplier quotes

POST   /listings/:id/review           Human review of agent-generated listing
POST   /listings/:id/publish          Trigger SP-API publish

GET    /approvals                     Pending approval queue
POST   /approvals/:id/decide          Approve/reject/request changes

GET    /events                        Audit timeline
GET    /dashboard/summary             KPIs for dashboard

GET    /config/scoring                Current scoring thresholds
PUT    /config/scoring                Update (used by Improvement layer)

GET    /discovery                     Continuous discovery config
PUT    /discovery                     Update discovery settings

GET    /experiments                   List experiments
GET    /experiments/:id               Experiment detail + results
POST   /experiments/:id/promote       Promote winning variant
POST   /experiments/:id/revert        Revert experiment
```

---

## 7. Continuous Improvement Layer (Autoresearch + PostHog)

Applies Karpathy's autoresearch pattern to FBA wholesale: autonomous experimentation within fixed budgets, human reviews results.

### The Loop

#### Step 1: Observe

PostHog collects outcome events from Operations Core:
- `deal_approved` — human liked the recommendation
- `deal_rejected` — human didn't (includes reason)
- `margin_realized` — actual vs. predicted margin
- `supplier_responded` — outreach worked
- `listing_conversion` — sessions to sales ratio
- `buy_box_won` / `buy_box_lost`
- `reorder_triggered` — product is a repeat winner
- `deal_archived` — product failed

These are the ground truth metrics — the equivalent of autoresearch's `val_bpb`.

#### Step 2: Hypothesize

Autoresearch engine runs on schedule (e.g., weekly). Analyzes outcome patterns and proposes hypotheses:

- "Deals rejected by humans have high margin but low demand scores. Demand weight should increase from 0.20 to 0.30"
- "Supplier outreach template B gets 2x response rate in electronics but template A wins in grocery"
- "Risk threshold of 6 is too aggressive — 40% of products flagged 'safe' had listing issues"

Each hypothesis becomes an Experiment proposal.

#### Step 3: Propose (human gate)

Experiment appears in dashboard/chat with evidence and recommendation. Human approves, rejects, or modifies.

#### Step 4: Experiment (campaign-scoped)

System creates two campaigns with identical criteria:
- Campaign A (control): current scoring config
- Campaign B (variant): proposed scoring config

Both run on the same categories, same time window. PostHog assigns variant via feature flag. Fixed budget: N candidates per arm (configurable per tenant).

#### Step 5: Evaluate

After both campaigns complete and enough time passes for downstream outcomes (configurable evaluation window):

Compare:
- Approval rate (did humans like variant B's picks?)
- Realized margin (were variant B deals more profitable?)
- Sourcing success (did suppliers actually respond?)
- Listing performance (did variant B products sell?)

Statistical significance check before declaring winner.

#### Step 6: Promote (human gate)

Results surface in dashboard/chat with metrics and recommendation. Human approves promotion to baseline, rejects (revert), or extends experiment.

### PostHog vs. Autoresearch responsibilities

| Concern | PostHog | Autoresearch engine |
|---------|---------|---------------------|
| Event capture | Receives domain events from Go backend | -- |
| Feature flags | Assigns control/variant per campaign | -- |
| Metrics | Stores + aggregates outcome data | Reads aggregated metrics |
| Hypothesis generation | -- | Analyzes patterns, proposes experiments |
| Experiment creation | -- | Creates experiment + variant campaigns |
| Statistical evaluation | Can compute significance | Interprets results, makes recommendation |
| Decision | -- | Proposes keep/revert (human decides) |

### Experiment domain models

```
Experiment {
  id, tenant_id,
  hypothesis,
  parameter_type: scoring_weights | thresholds | prompt_variant
                  | outreach_template | listing_strategy,
  control_config_id -> ScoringConfig,
  variant_config_id -> ScoringConfig,
  control_campaign_id -> Campaign,
  variant_campaign_id -> Campaign,
  posthog_feature_flag_key,
  status: proposed | approved | running | evaluating | completed,
  budget (max candidates per arm),
  evaluation_window_days,
  created_at, approved_at, completed_at
}

ExperimentResult {
  id, experiment_id,
  winner: control | variant | inconclusive,
  confidence,
  metrics_comparison (JSON),
  recommendation: keep | revert | extend,
  reasoning,
  decision: promoted | reverted | extended (after human review),
  decided_by, decided_at
}

ExperimentMemory {
  id, tenant_id,
  experiment_id,
  parameter_type,
  what_changed,
  outcome,
  impact_summary,
  created_at
}
```

`ExperimentMemory` accumulates over time so the autoresearch engine doesn't re-propose failed experiments and can reference what has worked historically.

### Experimentable parameters (MVP)

| Parameter | Example experiment |
|-----------|-------------------|
| Scoring weights | Increase demand weight from 0.20 to 0.30 |
| Score thresholds | Lower overall pass threshold from 8 to 7 |
| Agent prompts | "Be more conservative on competition analysis" |
| Ceiling/floor logic | Change floor cutoff from 10% of ceiling to 15% |

### Experimentable parameters (future domains)

| Parameter | Example experiment |
|-----------|-------------------|
| Outreach templates | Formal vs. casual supplier email style |
| Listing copy strategy | Keyword-dense vs. benefit-led bullets |
| Reorder timing | 14-day vs. 21-day reorder trigger |
| Risk tolerance | Tighter vs. looser gating risk threshold |

---

## 8. Human Gates

Every critical action requires human approval. No exceptions.

| Action | Gate |
|--------|------|
| Deal approval | Human reviews scored candidate, approves or rejects |
| Supplier outreach | Agent drafts, human approves before send |
| Purchase order | Human confirms PO terms and spend |
| Listing publish | Human reviews agent-generated content before SP-API publish |
| Experiment approval | Human reviews hypothesis before experiment runs |
| Config promotion | Human reviews experiment results before variant becomes baseline |

---

## 9. Implementation Roadmap

Full product decomposed into 8 domains. Each domain gets its own design -> plan -> implementation cycle.

| Phase | Domain | Description |
|-------|--------|-------------|
| **1 (MVP)** | Product Sourcing + Deal Scoring | Research pipeline, campaign model, deal lifecycle through `needs_review`, continuous discovery, dashboard for reviewing deals |
| 2 | Supplier Management + Outreach | Supplier CRM, outreach pipeline, response tracking |
| 3 | Procurement + Fulfillment | PO management, FBA inbound tracking, inventory |
| 4 | Listing + Publishing | Listing agent pipeline, SP-API publish, A+ content |
| 5 | Monitoring + Alerts | BSR/margin/competition tracking, reorder alerts |
| 6 | Continuous Improvement | Autoresearch engine, PostHog experiment integration |
| 7 | Advertising | PPC keyword suggestions, bid optimization, budget management |
| 8 | Disputes | Case drafts, evidence gathering, filing management |

### MVP (Phase 1) delivers

- Continuous discovery (nightly automated sourcing)
- Manual campaigns (user-initiated with custom criteria)
- Spreadsheet upload (bulk catalog analysis)
- 7-agent quality-gated research pipeline
- Deal scoring with evidence and audit trail
- Dashboard: campaign list, deal explorer, deal detail with score breakdown
- Chat: trigger campaigns and review results via WhatsApp/Telegram
- Tenant setup with Supabase auth

### MVP does NOT include

- Supplier outreach send (agent drafts outreach but no send mechanism yet)
- Purchase orders
- Listing generation and publish
- Post-launch monitoring
- Autoresearch experiments (infrastructure laid but engine runs from Phase 6)
- Ads, disputes

---

## 10. Listing Pipeline (Post-MVP, Phase 4)

Included here for completeness since it was discussed in the design.

After a human approves a deal and procurement completes:

```
┌─────────────────────────────────────────────────────────────┐
│  LISTING PIPELINE (OpenFang agents)                          │
│                                                              │
│  1. LISTING AGENT                                            │
│     Tools: SP-API, OpenAI                                    │
│     Generates: title, bullets, description, backend keywords │
│     Uses research ammunition from the original campaign      │
│                                                              │
│  2. A+ CONTENT AGENT (if brand registered)                   │
│     Generates: A+ module text, image alt-text,               │
│     comparison chart content                                 │
│                                                              │
│  3. LISTING REVIEWER (Boss)                                  │
│     Scores on:                                               │
│     - SEO Strength (1-10)                                    │
│     - Conversion Copy (1-10)                                 │
│     Same rewrite loop (max 3 iterations)                     │
│                                                              │
│  4. Human reviews final listing                              │
│  5. PUBLISH via SP-API                                       │
└─────────────────────────────────────────────────────────────┘
```

---

## 11. Post-Launch Monitoring (Post-MVP, Phase 5)

```
┌─────────────────────────────────────────────────────────────┐
│  POST-LAUNCH MONITORING (Operations Core)                    │
│                                                              │
│  - Track BSR, sessions, conversion, buy box %                │
│  - Inventory velocity -> reorder alerts                      │
│  - Competitor price monitoring                               │
│  - Feed metrics to Improvement layer (PostHog)               │
│  - Autoresearch proposes optimizations                       │
│    (listing tweaks, price adjustments, ad keywords)          │
└─────────────────────────────────────────────────────────────┘
```

---

## 12. Open Design Questions

Decisions deferred to implementation planning:

1. **OpenFang vs. ZeroClaw evaluation criteria** — What metrics determine which orchestration runtime wins? Latency, cost per pipeline run, channel coverage, memory quality? Need a concrete comparison framework before Phase 1 completes.
2. **SP-API rate limits and throttling strategy** — Amazon SP-API has strict rate limits per seller account. How does the Sourcing Agent handle 50-200 candidate lookups without hitting throttle? Likely needs batching + caching layer.
3. **Cost control per campaign** — Each agent call costs money (LLM tokens + API calls). Need per-campaign budget caps and per-tenant usage limits. Not designed yet.
4. **Spreadsheet format specification** — What columns are expected? How are malformed rows handled? Need a concrete schema.
5. **Autoresearch engine implementation** — Where does this run? Is it a scheduled Go service? An OpenFang agent? A standalone process? Deferred to Phase 6 design.
6. **Failure modes** — What happens when SP-API is down mid-pipeline? When an agent returns garbage? When Inngest workflow crashes? Need retry/circuit-breaker design per integration.

---

## 13. End-to-End Flow

```
Source -> Score -> Gate -> Approve -> Procure -> List -> Sell -> Monitor -> Improve
  ^                                                                          |
  └──────── Autoresearch proposes new campaigns based on learnings ──────────┘
```

Two sourcing modes feed the same pipeline:
- Continuous Discovery (always-on, baseline config)
- Campaigns (scoped, intentional, experiment vehicle)

The system learns from every cycle — predicted vs. actual margin, approval patterns, supplier response rates — and uses that data to improve future sourcing decisions.
