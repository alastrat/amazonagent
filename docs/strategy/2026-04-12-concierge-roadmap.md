# Concierge Roadmap — From Tool to Always-On AI

**Date:** 2026-04-12
**Status:** Active
**Branch:** feat/concierge-chat-interface

---

## Vision

Transform the FBA Agent Orchestrator from a manual tool (user clicks buttons, reviews tables) into an always-on AI concierge that proactively monitors, discovers, and acts on behalf of each seller — communicating via an embedded chat interface. The system continuously improves through the Karpathy autoresearch loop and builds compounding knowledge in tenant-isolated RAG memory.

---

## What's Done

### PR #3 (merged): Assessment v2
| Feature | Status |
|---------|--------|
| Per-tenant SP-API credentials (AES-256-GCM) | Done |
| 3-phase discovery assessment (search + enrich + eligibility) | Done |
| 3-state eligibility (eligible / ungatable / restricted) | Done |
| SSE real-time streaming during scan | Done |
| ECharts radial tree (Category → Subcategory → Brand) | Done |
| Onboarding wizard (Connect → Discover → Reveal → Commit) | Done |
| Filterable product table with status tabs | Done |
| 6 circuit breakers + brand enrichment | Done |
| 20 database migrations | Done |

### Phase B (in progress): Chat Interface + Agent Foundation
| Feature | Status |
|---------|--------|
| Typed AgentDefinition + Registry (Pattern 1) | Done |
| ReAct Agent Loop with stop conditions (Pattern 8) | Done |
| Lifecycle Hooks + Ralph Loop (Pattern 7) | Done |
| ChatMessage + ChatSession domain types | Done |
| ConversationalRuntime port (StartSession/SendMessage/EndSession) | Done |
| OpenFang + Simulator adapter implementations | Done |
| ChatHub SSE pub/sub | Done |
| ChatService with tenant context injection | Done |
| ChatRepo Postgres adapter + migration 021 | Done |
| ChatHandler (POST /chat/send, GET /chat/events, GET /chat/history) | Done |
| ChatPanel frontend (collapsible sidebar, markdown, typing indicator) | Done |
| useChat hook (SSE streaming, optimistic UI, history) | Done |
| AppShell integration (toggle button, localStorage persistence) | Done |

**Remaining for Phase B:** Wire ChatService into main.go, test end-to-end with OpenFang running.

---

## Phase C: Tenant RAG Memory (Weeks 3-4)

**Goal:** Each seller gets persistent memory so the concierge remembers context across sessions and learns from outcomes.

| Component | Details |
|-----------|---------|
| **Migration** | `022_seller_memory.sql`: pgvector extension, `seller_memory` table, HNSW index, RLS |
| **Memory types** | conversation (90d TTL), outcome (permanent), learning (permanent), market (30d TTL), preference (permanent) |
| **Port** | `SellerMemoryRepo`: Store, Search (vector similarity), ListByType, Expire |
| **Embedding** | text-embedding-3-small via Anthropic/OpenAI API |
| **Integration** | Inject top-K memories into concierge system prompt before every action |

```go
// internal/port/memory.go
type SellerMemoryRepo interface {
    Store(ctx context.Context, tenantID domain.TenantID, memory *domain.SellerMemory) error
    Search(ctx context.Context, tenantID domain.TenantID, query string, topK int) ([]domain.SellerMemory, error)
    ListByType(ctx context.Context, tenantID domain.TenantID, memoryType string, limit int) ([]domain.SellerMemory, error)
    Expire(ctx context.Context) error
}
```

---

## Phase D: Heartbeat + Daily Discovery (Weeks 5-6)

**Goal:** Concierge proactively monitors and discovers without user input.

| Component | Details |
|-----------|---------|
| **Inngest cron** | Per-tenant, every 6 hours (configurable via tenant settings) |
| **Heartbeat cycle** | Wake → load context + RAG → observe changes → reason → act/propose → sleep |
| **Observations** | New products in monitored categories, price changes, eligibility changes, approval status updates |
| **Actions** | Create suggestions, push notifications (chat + Telegram), refresh eligibility cache |
| **Tech debt** | TD-3: Parallel SP-API calls (85s → 17s) |

---

## Phase E: Action Execution + Approval Flow (Weeks 6-7)

**Goal:** Agent can execute actions with appropriate approval gates.

| Tier | Examples | Approval |
|------|----------|----------|
| **Auto** | Refresh eligibility cache, update product data, log insights | No approval needed |
| **Suggest** | New product opportunities, strategy adjustments, category recs | User approves in chat |
| **Critical** | Submit approval applications, create listings, set prices | User approves + confirms |

```go
// internal/domain/agent_action.go
type AgentAction struct {
    ID         string            `json:"id"`
    TenantID   TenantID          `json:"tenant_id"`
    Type       string            `json:"type"`       // auto, suggest, critical
    Action     string            `json:"action"`      // description of what to do
    Status     string            `json:"status"`      // proposed, approved, executed, rejected
    Metadata   map[string]any    `json:"metadata"`
    ProposedAt time.Time         `json:"proposed_at"`
    ResolvedAt *time.Time        `json:"resolved_at"`
}
```

---

## Phase F: Autoresearch — Karpathy Loop (Weeks 8-10)

**Goal:** System continuously improves its own recommendations.

```
OBSERVE → HYPOTHESIZE → EXPERIMENT → EVALUATE → PROMOTE
   ↑                                                │
   └────────────────────────────────────────────────┘
```

| Phase | What | Example |
|-------|------|---------|
| **Observe** | PostHog outcome tracking | "Tools category: 28% realized vs 35% predicted margin" |
| **Hypothesize** | Agent proposes parameter change | "Adjust FBA fee multiplier 1.0 → 1.15 for Tools" |
| **Experiment** | A/B test via PostHog flag | 50/50 split, 1 experiment max per tenant |
| **Evaluate** | 2-week window, compare metrics | "Adjusted: 30% predicted/29% realized. Old: 35% predicted/24% realized" |
| **Promote** | New StrategyVersion if winner | Learnings stored in tenant RAG |

---

## Phase G: Cohort Learning (Weeks 11-12)

**Goal:** Anonymized cross-tenant insights improve all sellers.

- Platform-wide embedding table (no tenant_id)
- Category-level insights: FBA fee accuracy, seasonal trends, brand gating changes
- Concierge queries both personal RAG + cohort RAG
- Privacy: only aggregate statistics, never individual seller data

---

## Tech Debt

| ID | Item | Priority | Phase |
|----|------|----------|-------|
| TD-2 | Batch INSERT (300 round-trips → 1) | **High** | B |
| TD-3 | Parallel SP-API calls (85s → 17s) | **Medium** | D |
| TD-7 | Remove dual Eligible bool | Medium | C |
| TD-8 | Move graph aggregation to service | Low | B |
| TD-1 | Unify Brand/SharedBrand types | Low | C |

---

## Timeline

| Phase | Dates | Duration |
|-------|-------|----------|
| **B** Chat + Agent Foundation | Apr 12 – Apr 25 | 2 weeks |
| **C** Tenant RAG Memory | Apr 26 – May 9 | 2 weeks |
| **D** Heartbeat + Discovery | May 10 – May 23 | 2 weeks |
| **E** Action Execution | May 17 – May 30 | 2 weeks (overlaps D) |
| **F** Autoresearch | May 31 – Jun 20 | 3 weeks |
| **G** Cohort Learning | Jun 21 – Jul 4 | 2 weeks |

---

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| OpenFang latency (2-10s per turn) | Chat feels slow | Stream partial responses; typing indicator; runtime port allows switching |
| pgvector at scale | Slow similarity search | HNSW index; partition by tenant if >1M vectors |
| Agent overreach | Executes unwanted actions | 3-tier approval gates; critical actions always need confirmation |
| Cost per chat turn ($0.01-0.05) | Uncontrolled spend | Rate limit per tenant; track credits per message |
| Context window overflow | Agent loses coherence | Ralph Loop: summarize + continue in fresh context |
| OpenFang vendor lock-in | Can't switch runtimes | ConversationalRuntime port; adapter pattern |
