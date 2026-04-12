# ADR: Agent Framework — Continuous Concierge Architecture

**Date:** 2026-04-11
**Status:** Proposed
**Supersedes:** Previous analysis recommending "direct Claude API for now"
**Context:** The platform should be an always-on AI concierge that continuously evaluates situations and takes actions on behalf of the seller — not a tool the user manually operates.

## Design Principles

1. **Runtime-agnostic** — The agent runtime (OpenFang, OpenClaw, ZeroClaw, or custom) is behind a port interface. Switching runtimes is a config change, not a rewrite.
2. **Continuous improvement** — Autoresearch (Karpathy method): observe outcomes → hypothesize improvements → experiment → evaluate → promote winners. The system gets smarter with every deal.
3. **Tenant-isolated RAG** — Each seller gets their own memory store (pgvector) with conversation history, deal outcomes, market intelligence, and strategy learnings. No cross-tenant leakage.

---

## The Shift: From Tool to Concierge

The current system requires the user to initiate every action: click Rescan, review products, click "Request approval", download CSV, etc. The vision is an **always-on concierge** that:

- Continuously monitors the seller's account and market conditions
- Proactively discovers new opportunities (not just when the user clicks Rescan)
- Takes actions on behalf of the seller (with approval gates for critical decisions)
- Communicates via a chat interface embedded in the web app (or voice)
- Maintains persistent context about the seller's situation, goals, and history

This is the **heartbeat pattern**: wake → observe → reason → act → sleep → repeat.

---

## Architecture: Heartbeat-Driven Concierge

```
┌─────────────────────────────────────────────────┐
│                  Chat Interface                   │
│           (Next.js embedded, or voice)            │
│                                                   │
│  User: "find me products under $50 I can sell"    │
│  Agent: "Found 3 new products. B0CX... has 35%    │
│          margin, 4 sellers. Want me to request     │
│          approval for the 2 that need ungating?"   │
│  User: "yes"                                      │
│  Agent: [submits approval applications]            │
└──────────────────────┬────────────────────────────┘
                       │
            ┌──────────▼──────────┐
            │   Agent Runtime     │
            │   (OpenFang +       │
            │    heartbeat loop)  │
            │                     │
            │  Persistent session │
            │  per tenant         │
            │  Memory enabled     │
            │  Tool access to     │
            │  all Go services    │
            └──────────┬──────────┘
                       │
         ┌─────────────┼─────────────┐
         │             │             │
    ┌────▼────┐  ┌─────▼─────┐  ┌───▼────┐
    │ SP-API  │  │ Assessment│  │Strategy│
    │ Tools   │  │ Service   │  │Service │
    │(search, │  │(scan,     │  │(goals, │
    │ eligib, │  │ enrich,   │  │ version│
    │ pricing)│  │ fingerpr) │  │ evolve)│
    └─────────┘  └───────────┘  └────────┘
```

### Heartbeat Cycle (Inngest cron, per tenant)

```
Every 6 hours (configurable):
  1. WAKE — Load tenant context (profile, strategy, last scan, credit balance)
  2. OBSERVE — Check for changes:
     - New products in monitored categories?
     - Price changes on tracked ASINs?
     - Eligibility changes (new brands ungated)?
     - Approval applications status updates?
  3. REASON — LLM evaluates:
     - Any new opportunities worth pursuing?
     - Should strategy be adjusted based on outcomes?
     - Any risks to flag (competitor entry, price drops)?
  4. ACT — Execute or propose:
     - Auto-actions: update product data, refresh eligibility cache
     - Proposed actions: "Found 5 new products, want me to add them?"
     - Notifications: push to chat, email, or Telegram
  5. SLEEP — Record what was done, schedule next heartbeat
```

### Chat Interface Architecture

```
Frontend (Next.js)
  └── ChatPanel component
        ├── SSE stream from /api/chat endpoint
        ├── User message input
        ├── Agent message rendering (markdown + action buttons)
        └── Inline approval buttons ("Approve" / "Dismiss")

Backend (Go API)
  └── POST /api/chat/message
        ├── Routes to OpenFang agent (persistent session, memory ON)
        ├── Agent has tool access to all Go services
        ├── Returns streaming response via SSE
        └── Actions requiring approval → create Suggestion with "pending" status

OpenFang Agent (per-tenant persistent session)
  └── System prompt: "You are an FBA concierge for {seller_name}..."
        ├── Tools: search_products, check_eligibility, get_pricing,
        │          create_suggestion, update_strategy, query_catalog
        ├── Memory: full conversation history retained
        ├── Context: seller profile, active strategy, recent discoveries
        └── Guardrails: never auto-execute purchases, listings, or pricing changes
```

---

## Principle 1: Runtime-Agnostic Agent Port

The agent runtime MUST be behind a port interface. We already have this:

```go
// internal/port/agent_runtime.go (exists today)
type AgentRuntime interface {
    RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error)
}
```

Current implementations: OpenFang adapter, Simulator (test mock). To switch runtimes, implement the interface — no pipeline code changes.

**Extended interface for concierge mode:**

```go
type AgentRuntime interface {
    RunAgent(ctx context.Context, task domain.AgentTask) (*domain.AgentOutput, error)
    
    // Concierge extensions
    StartSession(ctx context.Context, tenantID domain.TenantID, config SessionConfig) (SessionID, error)
    SendMessage(ctx context.Context, sessionID SessionID, message string) (*domain.AgentOutput, error)
    EndSession(ctx context.Context, sessionID SessionID) error
    
    // Heartbeat
    Heartbeat(ctx context.Context, sessionID SessionID, context HeartbeatContext) (*domain.AgentOutput, error)
}
```

This keeps the door open for OpenFang, OpenClaw, ZeroClaw, direct Claude API, or a custom runtime. The pipeline and chat interface never know which runtime is behind the port.

---

## Principle 2: Autoresearch — Karpathy Continuous Improvement Loop

The system doesn't just execute — it learns. Based on Karpathy's autoresearch pattern:

```
OBSERVE → HYPOTHESIZE → EXPERIMENT → EVALUATE → PROMOTE
   ↑                                                │
   └────────────────────────────────────────────────┘
```

### Applied to FBA Wholesale

| Phase | What Happens | Example |
|-------|-------------|---------|
| **Observe** | Track deal outcomes via PostHog | "Products in Tools category had 28% avg margin vs predicted 35%" |
| **Hypothesize** | Agent analyzes gap, proposes parameter change | "Margin estimate is too optimistic for Tools — adjust FBA fee multiplier from 1.0 to 1.15" |
| **Experiment** | A/B test the change (PostHog feature flag) | 50% of new Tools discoveries use old config, 50% use adjusted config |
| **Evaluate** | Compare realized margins after 2-week window | "Adjusted config: 29% realized vs 30% predicted. Old config: 24% realized vs 35% predicted." |
| **Promote** | If improvement > threshold, promote to default | New StrategyVersion created with updated scoring config, old version preserved for rollback |

### Implementation

- **1 experiment max per tenant** (cost predictability)
- **Inngest weekly cron** runs the autoresearch cycle
- **PostHog** for outcome tracking + A/B flag management
- **StrategyVersion** for config versioning (already built, supports `promoted_from_experiment_id`)
- **Human gate**: agent proposes experiment → user approves in chat → experiment runs

### What Gets Researched

- Scoring weights (margin vs BSR vs seller count importance)
- FBA fee estimation accuracy per category
- Eligibility prediction (which "restricted" products are actually ungatable)
- Category prioritization formula adjustments
- Optimal scan frequency per category (hot categories more often)

---

## Principle 3: Tenant-Isolated RAG Memory

Each tenant gets a persistent memory store. The concierge uses it to maintain context across sessions, learn from outcomes, and personalize recommendations.

### Architecture: pgvector in Supabase

```sql
-- Migration: 021_seller_memory.sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE seller_memory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    content TEXT NOT NULL,
    embedding vector(1536),  -- text-embedding-3-small
    memory_type TEXT NOT NULL,  -- conversation | outcome | learning | market | preference
    entity_type TEXT,  -- deal | brand | category | product | strategy
    entity_id TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ  -- optional TTL for ephemeral memories
);

-- HNSW index for fast similarity search
CREATE INDEX seller_memory_embedding_idx ON seller_memory
    USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

-- Tenant isolation via RLS
ALTER TABLE seller_memory ENABLE ROW LEVEL SECURITY;
ALTER TABLE seller_memory FORCE ROW LEVEL SECURITY;
CREATE POLICY seller_memory_isolation ON seller_memory
    USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
```

### What Gets Stored Per Tenant

| Memory Type | Content | TTL | Example |
|-------------|---------|-----|---------|
| **conversation** | Summarized chat exchanges (not raw turns) | 90 days | "User asked about pet supplies category, said they have supplier contacts there" |
| **outcome** | Deal results (predicted vs realized margin, sell-through) | Permanent | "B0CX23V5KK: predicted 32% margin, realized 28%, sold 45 units in 30 days" |
| **learning** | Autoresearch findings | Permanent | "Tools category FBA fees consistently 15% higher than calculator estimates" |
| **market** | Category/brand intelligence | 30 days | "Kitchen Storage BSR competition increased 20% in Q1, margins compressing" |
| **preference** | Seller-expressed preferences and constraints | Permanent | "User prefers products under $50, won't sell supplements, has $10K capital" |

### How the Concierge Uses RAG

Before every agent action (heartbeat or chat response):
1. Embed the current context/question
2. Retrieve top-K relevant memories for this tenant
3. Inject into system prompt as "seller context"
4. Agent responds with full awareness of history

### Isolation Guarantees

- **RLS enforced** — Postgres won't return rows from other tenants
- **Application-layer filter** — every query includes `WHERE tenant_id = $1`
- **Shared catalog data is separate** — product catalog is platform-wide (no tenant_id), seller memories are tenant-scoped
- **Embedding model shared** — one embedding API call, results stored per tenant

### Port Interface

```go
// internal/port/memory.go
type SellerMemoryRepo interface {
    Store(ctx context.Context, tenantID domain.TenantID, memory *domain.SellerMemory) error
    Search(ctx context.Context, tenantID domain.TenantID, query string, topK int) ([]domain.SellerMemory, error)
    ListByType(ctx context.Context, tenantID domain.TenantID, memoryType string, limit int) ([]domain.SellerMemory, error)
    Expire(ctx context.Context) error  // cleanup expired memories
}
```

---

## Why OpenFang First (Runtime-Agnostic)

### OpenFang Advantages for This Pattern

1. **Built-in heartbeat monitoring** — 30s interval, 180s timeout, auto-restart on unresponsive agents. Exactly the pattern we need for always-on concierge.
2. **Persistent sessions with memory** — Already implemented in our adapter (`agent_memory_enabled` tenant setting). Just needs to be turned on.
3. **Agent-as-a-service** — HTTP API means the Go backend and frontend can both talk to it without tight coupling.
4. **Rust runtime** — Low resource footprint for running N persistent agent sessions (one per active tenant).
5. **Already integrated** — We have the adapter, Docker config, and deployment pipeline.

### What Needs to Change in OpenFang Integration

| Current State | Target State |
|---------------|-------------|
| Stateless (session reset per call) | Persistent sessions with memory per tenant |
| Sequential pipeline (hardcoded 6 stages) | LLM-driven tool selection (agent decides what to do) |
| Dead config fields (ModelTier, Tools) | Active config: model tier, tool allowlist, timeout per agent |
| No chat interface | SSE-streaming chat panel in web app |
| User initiates all actions | Agent proposes actions, user approves critical ones |
| No heartbeat | Inngest cron triggers heartbeat cycle per tenant |

### What We Still Fix from the Patterns Doc

Even with OpenFang as runtime, we adopt the valuable patterns:

- **Pattern 1 (Typed outputs)**: Define Go structs for agent responses, validate after each call
- **Pattern 7 (Lifecycle hooks)**: Pre/post hooks for metrics, validation, domain events
- **Pattern 4 (Structured context)**: Replace `[]AgentContext` with typed `PipelineContext`
- **Pattern 2 (Task state machine)**: Track agent invocations in DB for observability

We skip Patterns 3, 5, 6 (parallelism, tool ACL, coordinator) — OpenFang handles these internally.

---

## Implementation Plan

### Phase 1: Chat Interface + Runtime Decoupling (2 weeks)
- Extend `AgentRuntime` port with `StartSession`, `SendMessage`, `Heartbeat` methods
- Implement concierge session in OpenFang adapter (persistent, memory ON)
- Build `/api/chat/message` endpoint with SSE streaming
- Build `ChatPanel` React component in the web app
- Concierge system prompt with tool definitions for all Go services
- Agent can answer questions about the seller's account, products, eligibility

### Phase 2: Proactive Heartbeat (1 week)
- Inngest cron job per tenant (every 6 hours, configurable)
- Heartbeat calls the concierge agent with "check for updates" prompt
- Agent uses tools to scan for changes, creates Suggestions for new opportunities
- Notifications pushed to chat panel / Telegram

### Phase 3: Tenant RAG Memory (2 weeks)
- Migration: `021_seller_memory.sql` (pgvector + HNSW index + RLS)
- `SellerMemoryRepo` port + Postgres adapter
- Embedding service (text-embedding-3-small via Anthropic or OpenAI)
- Store: conversation summaries, deal outcomes, preferences, market intel
- Inject top-K memories into concierge system prompt before every action
- Expiry cron for ephemeral memories (conversations: 90 days, market: 30 days)

### Phase 4: Action Execution + Approval Flow (1-2 weeks)
- Agent can propose actions (add product to list, request approval, adjust strategy)
- Approval flow: agent creates Suggestion → user approves in chat → agent executes
- Three tiers: Auto (cache refresh) / Suggest (new products) / Critical (listings, pricing)
- Non-critical actions auto-execute with audit log

### Phase 5: Autoresearch — Karpathy Loop (2-3 weeks)
- PostHog outcome tracking: predicted vs realized margins, sell-through rates
- Inngest weekly cron: autoresearch cycle per tenant
- Agent observes outcome data, hypothesizes parameter improvements
- Proposes experiment to user in chat → user approves → A/B test runs
- 1 experiment max per tenant (cost predictability)
- After evaluation window (2 weeks): promote winner as new StrategyVersion or revert
- Learnings stored in tenant RAG for compounding knowledge

### Phase 6: Multi-Tenant Cohort Learning (2 weeks)
- Anonymized aggregate insights from all tenants
- Cohort-based recommendations (new sellers learn from experienced sellers' patterns)
- Shared learnings: category-level FBA fee accuracy, seasonal trends, brand gating changes
- Stored in platform-wide (non-tenant-scoped) embeddings table
- Tenant concierge queries both personal RAG + cohort RAG

---

## Approval Gates

The concierge has three action tiers:

| Tier | Examples | Approval |
|------|----------|----------|
| **Auto** | Refresh eligibility cache, update product data, log insights | No approval needed |
| **Suggest** | New product opportunities, strategy adjustments, category recommendations | User approves in chat |
| **Critical** | Submit approval applications, create listings, set prices | User approves + confirms |

---

## Chat UX Vision

```
┌─────────────────────────────────────────┐
│ FBA Concierge                    [−][×] │
├─────────────────────────────────────────┤
│                                         │
│ 🤖 Good morning! I ran your overnight   │
│    scan and found 3 new opportunities:  │
│                                         │
│    1. B0CX23V5KK — Stanley Hammer       │
│       $24.99, 32% margin, 5 sellers     │
│       Status: ✅ Eligible                │
│                                         │
│    2. B0D2JGYX3F — Aoxun Gazebo         │
│       $499.98, 40% margin, 2 sellers    │
│       Status: 🟡 Needs approval         │
│       [Request Approval]                │
│                                         │
│    3. B0DR2C2FND — DeWalt Drill Kit     │
│       $239.00, 39% margin, 0 sellers    │
│       Status: 🟡 Needs approval         │
│       [Request Approval]                │
│                                         │
│    Want me to request approval for       │
│    #2 and #3?                           │
│                                         │
│ 👤 yes, go ahead                        │
│                                         │
│ 🤖 Done! I've submitted approval        │
│    requests for both. I'll notify you    │
│    when Amazon responds. Meanwhile,      │
│    product #1 is ready to list — want   │
│    me to add it to your inventory?      │
│                                         │
├─────────────────────────────────────────┤
│ Type a message...              [Send]   │
└─────────────────────────────────────────┘
```

---

## Decision

**Start with OpenFang behind a runtime-agnostic port**, transforming the platform into a continuous concierge:

1. **Runtime-agnostic** — `AgentRuntime` port interface, OpenFang as first implementation, swappable to OpenClaw/ZeroClaw/custom without pipeline changes
2. **From stateless tool → persistent concierge** (enable memory, persistent sessions)
3. **From user-initiated → agent-initiated** (heartbeat cron + proactive suggestions)
4. **From CLI/API only → chat interface** (embedded in web app, SSE streaming)
5. **From hardcoded pipeline → LLM-driven tool selection** (agent decides what to do)
6. **Tenant-isolated RAG** (pgvector + RLS, personal memory per seller)
7. **Autoresearch loop** (Karpathy method: observe → hypothesize → experiment → evaluate → promote)

Fix the type safety issues (Pattern 1, 7) regardless — those are bugs, not features.

## Consequences

- Agent runtime is a pluggable adapter — no vendor lock-in to OpenFang
- Every tenant gets a persistent agent session + isolated RAG memory
- The chat interface becomes the primary interaction surface
- Current button-clicking UI becomes visualization layer (graph/table for context, chat for action)
- The system improves autonomously via autoresearch — every deal outcome makes future recommendations better
- RAG memory compounds over time — longer-tenured sellers get increasingly personalized recommendations
- Need to define clear approval gates to prevent agent overreach
- pgvector adds embedding cost (~$0.001 per 1K tokens for text-embedding-3-small)

## References

- OpenFang heartbeat: 30s interval, 180s timeout, auto-restart ([openfang.sh](https://www.openfang.sh/))
- OpenClaw / ZeroClaw: alternative Rust runtimes for evaluation ([comparison](https://sparkco.ai/blog/openclaw-vs-zeroclaw-which-ai-agent-framework-should-you-choose-in-2026))
- Heartbeat pattern: wake → observe → reason → act ([MindStudio](https://www.mindstudio.ai/blog/agentic-os-heartbeat-pattern-proactive-ai-agent))
- Karpathy autoresearch: continuous hypothesis → experiment → evaluate loop
- `internal/port/agent_runtime.go` — existing runtime port (decoupling point)
- `internal/adapter/openfang/agent_runtime.go` — existing OpenFang adapter
- `internal/domain/tenant_settings.go` — `agent_memory_enabled` flag (already exists)
- `docs/superpowers/research/2026-04-08-continuous-learning-architecture.md` — RAG + autoresearch design
- `docs/superpowers/research/2026-04-08-ai-concierge-expert-brainstorm.md` — "Wealthfront moment" vision
