# Agent Orchestration Patterns: What to Replicate from Claude Code

*Last updated: 2026-04-11*
*Status: Draft -- pending engineering review*
*Source analysis: Claude Code open-source project (Anthropic)*

---

## Executive Summary

This document identifies seven agent orchestration patterns from Claude Code's architecture that can be replicated in Estori's FBA agent orchestrator. The patterns are prioritized by value and effort, with a phased implementation roadmap. The goal is to evolve Estori from a hardcoded sequential pipeline into a flexible, observable, and resilient multi-agent system.

---

## Architecture Comparison

| Concept | Claude Code (TypeScript) | Estori Today (Go) |
|---------|------------------------|--------------------|
| Agent definition | Typed `AgentDefinition` with tools, model, permissions, hooks | `AgentConfig` with system prompt, tools list (unwired), model tier (unwired) |
| Spawning | `AgentTool.call()` -- sync or async, in-process or isolated | `AgentRuntime.RunAgent()` -- always sync, always HTTP to OpenFang |
| Parallelism | N background agents in one turn, concurrent tool batching | Sequential within pipeline; Inngest fan-out for candidates only |
| Inter-agent comms | `SendMessage` tool, file-based mailbox, notification queue | `[]AgentContext` accumulated facts passed forward linearly |
| Task lifecycle | `pending -> running -> completed/failed/killed` state machine | No task abstraction -- pipeline stages are implicit |
| Tool access | Per-agent allow/disallow lists, MCP servers per agent | `ToolResolver` pre-fetches data; agents never call tools directly |
| Context isolation | `createSubagentContext()` with cloned state per agent | None -- all agents share the same pipeline context |
| Error handling | AbortController chains, max turns, graceful degradation | 120s HTTP timeout, no circuit breaker, no retry |
| Coordinator mode | Manager LLM orchestrates workers via tool calls | Hardcoded pipeline sequence in Go code |

---

## Pattern 1: Typed Agent Definitions

### Value: High | Effort: Low | Priority: P0

### What Claude Code Does

`AgentDefinition` is a typed schema -- name, system prompt, model, tool access, permissions, max turns, hooks. Agents are registered in a discoverable registry. The TypeScript definition:

```typescript
type AgentDefinition = {
  agentType: string
  source: 'built-in' | 'user' | 'plugin'
  getSystemPrompt(): string
  model?: ModelAlias
  permissionMode?: PermissionMode
  effort?: EffortValue
  background?: boolean
  isolation?: 'worktree' | 'remote'
  maxTurns?: number
  omitClaudeMd?: boolean
  mcpServers?: MCP.Config[]
  skills?: string[]
  hooks?: FrontmatterHooks
}
```

### What Estori Has

`AgentConfig` with `SystemPrompt`, `Tools []string`, `ModelTier string` -- but `Tools` and `ModelTier` are never read by the OpenFang adapter. They are dead fields defined in `internal/domain/pipeline.go` but ignored in `internal/adapter/openfang/agent_runtime.go`.

### What to Build

```go
// internal/domain/agent_definition.go

type AgentDefinition struct {
    Name         string            `json:"name"`
    Description  string            `json:"description"`
    SystemPrompt string            `json:"system_prompt"`
    ModelTier    ModelTier         `json:"model_tier"`    // fast | standard | reasoning
    Tools        []string          `json:"tools"`         // tool names this agent can use
    OutputSchema OutputSchemaSpec  `json:"output_schema"` // typed expected output
    MaxRetries   int               `json:"max_retries"`
    TimeoutSec   int               `json:"timeout_sec"`
    Hooks        *AgentHooks       `json:"-"`             // pre/post execution hooks
}

type ModelTier string

const (
    ModelTierFast      ModelTier = "fast"      // gating, pre-filters
    ModelTierStandard  ModelTier = "standard"  // demand, supplier, profitability
    ModelTierReasoning ModelTier = "reasoning" // reviewer, coordinator
)

type OutputSchemaSpec struct {
    Fields   []SchemaField `json:"fields"`
    Required []string      `json:"required"`
}

type SchemaField struct {
    Name     string `json:"name"`
    Type     string `json:"type"`     // bool, int, float, string, []string
    Default  any    `json:"default"`  // fallback if missing
}
```

### Typed Output Structs Per Agent

Replace `map[string]any` with Go structs for each agent stage:

```go
// internal/domain/agent_outputs.go

type GatingOutput struct {
    Passed    bool     `json:"passed"`
    RiskScore float64  `json:"risk_score"`
    Flags     []string `json:"flags"`
    Reasoning string   `json:"reasoning"`
}

type ProfitabilityOutput struct {
    MarginPercent    float64 `json:"margin_percent"`
    FBAFees          float64 `json:"fba_fees"`
    NetProfit        float64 `json:"net_profit"`
    QualitativeScore int     `json:"qualitative_score"`
    Reasoning        string  `json:"reasoning"`
}

type DemandOutput struct {
    DemandScore      int    `json:"demand_score"`      // 1-10
    CompetitionScore int    `json:"competition_score"` // 1-10
    BuyBoxDynamics   string `json:"buy_box_dynamics"`
    Reasoning        string `json:"reasoning"`
}

type SupplierOutput struct {
    Suppliers     []SupplierInfo `json:"suppliers"`
    OutreachDraft string         `json:"outreach_draft"`
    Reasoning     string         `json:"reasoning"`
}

type SupplierInfo struct {
    Name     string  `json:"name"`
    Price    float64 `json:"price"`
    MOQ      int     `json:"moq"`
    LeadDays int     `json:"lead_days"`
    Source   string  `json:"source"`
}

type ReviewerOutput struct {
    OpportunityViability  int    `json:"opportunity_viability"`  // 1-10
    ExecutionConfidence   int    `json:"execution_confidence"`   // 1-10
    SourcingFeasibility   int    `json:"sourcing_feasibility"`   // 1-10
    CompositeScore        float64 `json:"composite_score"`
    Tier                  string `json:"tier"`                   // A, B, C, Cut
    Reasoning             string `json:"reasoning"`
}
```

### Agent Registry

```go
// internal/domain/agent_registry.go

var AgentRegistry = map[string]AgentDefinition{
    "gating": {
        Name:         "gating",
        Description:  "Evaluates IP risk, brand restrictions, category gating, hazmat flags",
        ModelTier:    ModelTierFast,
        Tools:        []string{"sp_api_restrictions", "brand_database"},
        MaxRetries:   1,
        TimeoutSec:   30,
    },
    "profitability": {
        Name:         "profitability",
        Description:  "Evaluates margin viability with qualitative LLM assessment",
        ModelTier:    ModelTierStandard,
        Tools:        []string{"sp_api_fees", "price_history"},
        MaxRetries:   1,
        TimeoutSec:   45,
    },
    "demand": {
        Name:         "demand",
        Description:  "Scores sales velocity, BSR trends, buy box dynamics",
        ModelTier:    ModelTierStandard,
        Tools:        []string{"keepa_data", "price_history"},
        MaxRetries:   1,
        TimeoutSec:   45,
    },
    "supplier": {
        Name:         "supplier",
        Description:  "Discovers wholesale suppliers, compares pricing and MOQ",
        ModelTier:    ModelTierStandard,
        Tools:        []string{"web_search", "web_scrape", "supplier_database"},
        MaxRetries:   2,
        TimeoutSec:   60,
    },
    "reviewer": {
        Name:         "reviewer",
        Description:  "Final scoring: opportunity viability, execution confidence, sourcing feasibility",
        ModelTier:    ModelTierReasoning,
        Tools:        []string{},
        MaxRetries:   1,
        TimeoutSec:   60,
    },
}
```

### Impact

- Eliminates the #1 critical risk (silent type assertion failures on `map[string]any`)
- Enables per-agent model selection (fast models for gating, reasoning models for review)
- Makes agent capabilities explicit, testable, and discoverable
- `Tools` field drives `ToolResolver` to only fetch what each agent needs

### Key Files to Modify

- `internal/domain/agent.go` -- add `AgentDefinition` and typed output structs
- `internal/domain/pipeline.go` -- replace `AgentConfig` usage with `AgentDefinition`
- `internal/adapter/openfang/agent_runtime.go` -- read `ModelTier` and `Tools` from definition
- `internal/service/pipeline_orchestrator.go` -- unmarshal agent outputs into typed structs
- `internal/service/tool_resolver.go` -- filter pre-fetch calls based on agent's tool list

---

## Pattern 2: Task State Machine

### Value: High | Effort: Medium | Priority: P0

### What Claude Code Does

Every spawned agent becomes a `Task` with states `pending -> running -> completed | failed | killed`. Tasks have IDs, progress tracking, output files, and duration metrics. The parent can query, message, or kill any task.

```typescript
// Claude Code task states
type TaskStatus = 'pending' | 'running' | 'completed' | 'failed' | 'killed'
type TaskType = 'local_agent' | 'local_bash' | 'remote_agent' | 'in_process_teammate'
```

### What Estori Has

No task abstraction. Pipeline stages are implicit function calls. Inngest steps have their own opaque state, but there's no unified task model visible to the application layer. When a pipeline fails at stage 4 of 6, there's no record of what happened at stages 1-3.

### What to Build

```go
// internal/domain/agent_task.go

type TaskStatus string

const (
    TaskStatusPending   TaskStatus = "pending"
    TaskStatusRunning   TaskStatus = "running"
    TaskStatusCompleted TaskStatus = "completed"
    TaskStatusFailed    TaskStatus = "failed"
    TaskStatusKilled    TaskStatus = "killed"
    TaskStatusRetrying  TaskStatus = "retrying"
)

type AgentTaskRecord struct {
    ID            string      `json:"id"`
    PipelineRunID string      `json:"pipeline_run_id"`
    ParentTaskID  *string     `json:"parent_task_id"`  // enables hierarchy
    AgentName     string      `json:"agent_name"`
    CandidateASIN *string     `json:"candidate_asin"`
    Status        TaskStatus  `json:"status"`
    Input         any         `json:"input"`            // typed per agent
    Output        any         `json:"output"`           // typed per agent
    Error         *string     `json:"error"`
    TokensUsed    int         `json:"tokens_used"`
    DurationMs    int64       `json:"duration_ms"`
    RetryCount    int         `json:"retry_count"`
    StartedAt     time.Time   `json:"started_at"`
    CompletedAt   *time.Time  `json:"completed_at"`
    TenantID      string      `json:"tenant_id"`
}

// State transitions
var validTaskTransitions = map[TaskStatus][]TaskStatus{
    TaskStatusPending:   {TaskStatusRunning, TaskStatusKilled},
    TaskStatusRunning:   {TaskStatusCompleted, TaskStatusFailed, TaskStatusKilled},
    TaskStatusFailed:    {TaskStatusRetrying, TaskStatusKilled},
    TaskStatusRetrying:  {TaskStatusRunning},
}

func (t *AgentTaskRecord) Transition(to TaskStatus) error {
    valid := validTaskTransitions[t.Status]
    for _, s := range valid {
        if s == to {
            t.Status = to
            if to == TaskStatusCompleted || to == TaskStatusFailed || to == TaskStatusKilled {
                now := time.Now()
                t.CompletedAt = &now
                t.DurationMs = now.Sub(t.StartedAt).Milliseconds()
            }
            return nil
        }
    }
    return fmt.Errorf("invalid task transition: %s -> %s", t.Status, to)
}
```

### Pipeline Run Record

```go
type PipelineRun struct {
    ID              string        `json:"id"`
    CampaignID      string        `json:"campaign_id"`
    TenantID        string        `json:"tenant_id"`
    Status          TaskStatus    `json:"status"`
    TotalCandidates int           `json:"total_candidates"`
    Survived        int           `json:"survived"`
    Tasks           []AgentTaskRecord `json:"tasks"`
    StartedAt       time.Time     `json:"started_at"`
    CompletedAt     *time.Time    `json:"completed_at"`
    TotalTokensUsed int           `json:"total_tokens_used"`
    TotalDurationMs int64         `json:"total_duration_ms"`
}
```

### Integration with Pipeline Orchestrator

```go
// Wrapping every RunAgent call
func (o *PipelineOrchestrator) runAgentWithTracking(
    ctx context.Context,
    run *PipelineRun,
    def AgentDefinition,
    task domain.AgentTask,
    parentTaskID *string,
) (*domain.AgentOutput, *AgentTaskRecord, error) {
    record := &AgentTaskRecord{
        ID:            generateTaskID(),
        PipelineRunID: run.ID,
        ParentTaskID:  parentTaskID,
        AgentName:     def.Name,
        Status:        TaskStatusPending,
        StartedAt:     time.Now(),
    }

    // Execute hooks
    if def.Hooks != nil && def.Hooks.PreRun != nil {
        if err := def.Hooks.PreRun(ctx, record); err != nil {
            return nil, record, err
        }
    }

    record.Transition(TaskStatusRunning)

    output, err := o.agentRuntime.RunAgent(ctx, task)

    if err != nil {
        record.Transition(TaskStatusFailed)
        errStr := err.Error()
        record.Error = &errStr
    } else {
        record.Status = TaskStatusCompleted
        record.Output = output.Structured
        record.TokensUsed = output.TokensUsed
    }

    now := time.Now()
    record.CompletedAt = &now
    record.DurationMs = now.Sub(record.StartedAt).Milliseconds()

    // Execute hooks
    if def.Hooks != nil && def.Hooks.PostRun != nil {
        def.Hooks.PostRun(ctx, record, output)
    }

    run.Tasks = append(run.Tasks, *record)
    return output, record, err
}
```

### Impact

- Full observability into every agent call (currently a black box)
- Ability to retry individual failed stages instead of re-running entire pipelines
- Parent-child relationship tree for hierarchical orchestration
- Pipeline health metrics: tokens used, duration, pass rates per stage
- Foundation for the coordinator pattern

### Database Migration

```sql
CREATE TABLE agent_task_records (
    id              TEXT PRIMARY KEY,
    pipeline_run_id TEXT NOT NULL,
    parent_task_id  TEXT REFERENCES agent_task_records(id),
    agent_name      TEXT NOT NULL,
    candidate_asin  TEXT,
    status          TEXT NOT NULL DEFAULT 'pending',
    input           JSONB,
    output          JSONB,
    error           TEXT,
    tokens_used     INTEGER DEFAULT 0,
    duration_ms     BIGINT DEFAULT 0,
    retry_count     INTEGER DEFAULT 0,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    tenant_id       TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_agent_tasks_pipeline ON agent_task_records(pipeline_run_id);
CREATE INDEX idx_agent_tasks_tenant ON agent_task_records(tenant_id);
CREATE INDEX idx_agent_tasks_status ON agent_task_records(status);
```

### Key Files to Modify

- `internal/domain/` -- new `agent_task.go` file
- `internal/port/` -- new `AgentTaskRepository` interface
- `internal/adapter/postgres/` -- new `agent_task_repo.go`
- `internal/service/pipeline_orchestrator.go` -- wrap all `RunAgent` calls with tracking
- Database migration -- new table

---

## Pattern 3: Parallel Agent Execution Within Pipelines

### Value: High | Effort: Medium | Priority: P1

### What Claude Code Does

The parent emits N `Agent` tool-use blocks in one turn. The tool orchestrator partitions them: read-only agents run concurrently (up to `MAX_CONCURRENCY=10`), write agents run serially. Results arrive as `<task-notification>` messages.

```typescript
// Claude Code tool orchestration
async function* runTools(toolUseMessages, context) {
    const { concurrent, serial } = partitionToolCalls(toolUseMessages)
    // concurrent tools run with all() combinator
    yield* all(concurrent.map(t => runTool(t)), MAX_CONCURRENCY)
    // serial tools run one at a time
    for (const tool of serial) {
        yield* runTool(tool)
    }
}
```

### What Estori Has

All 5 LLM stages per candidate run sequentially in `pipeline_orchestrator.go`. But demand and supplier agents have no data dependency on each other -- they both depend only on gating and profitability outputs, not on each other.

### What to Build

```go
// internal/service/parallel_runner.go

type ParallelAgent struct {
    Definition AgentDefinition
    Task       domain.AgentTask
}

type AgentResult struct {
    AgentName string
    Output    *domain.AgentOutput
    Task      *AgentTaskRecord
    Err       error
}

func (o *PipelineOrchestrator) RunParallel(
    ctx context.Context,
    run *PipelineRun,
    agents []ParallelAgent,
    parentTaskID *string,
) []AgentResult {
    results := make(chan AgentResult, len(agents))
    ctx, cancel := context.WithTimeout(ctx, maxParallelTimeout(agents))
    defer cancel()

    for _, a := range agents {
        go func(agent ParallelAgent) {
            output, record, err := o.runAgentWithTracking(ctx, run, agent.Definition, agent.Task, parentTaskID)
            results <- AgentResult{
                AgentName: agent.Definition.Name,
                Output:    output,
                Task:      record,
                Err:       err,
            }
        }(a)
    }

    collected := make([]AgentResult, 0, len(agents))
    for i := 0; i < len(agents); i++ {
        collected = append(collected, <-results)
    }
    return collected
}

func maxParallelTimeout(agents []ParallelAgent) time.Duration {
    max := 0
    for _, a := range agents {
        if a.Definition.TimeoutSec > max {
            max = a.Definition.TimeoutSec
        }
    }
    return time.Duration(max) * time.Second
}
```

### Pipeline Integration

```go
// In evaluateOne() -- the deduplicated pipeline method

// Sequential: sourcing -> gating -> profitability (each depends on prior)
// ...

// Parallel: demand + supplier (no mutual dependency)
parallelResults := o.RunParallel(ctx, run, []ParallelAgent{
    {Definition: AgentRegistry["demand"], Task: demandTask},
    {Definition: AgentRegistry["supplier"], Task: supplierTask},
}, profitabilityTaskID)

var demandOut, supplierOut *domain.AgentOutput
for _, r := range parallelResults {
    if r.Err != nil {
        // handle error per agent
        continue
    }
    switch r.AgentName {
    case "demand":
        demandOut = r.Output
    case "supplier":
        supplierOut = r.Output
    }
}

// Sequential: reviewer (depends on all above)
// ...
```

### Inngest Integration

For Inngest-based execution, use `step.Group()` for parallel steps:

```go
// In evaluate-candidate Inngest function
demandResult, _ := step.Run(ctx, "agent-demand", func(ctx context.Context) (string, error) { ... })
supplierResult, _ := step.Run(ctx, "agent-supplier", func(ctx context.Context) (string, error) { ... })
// Inngest runs these concurrently when they have no step dependencies
```

### Dependency Graph

```
sourcing
    |
    v
gating
    |
    v
profitability
    |
    +-------+-------+
    |               |
    v               v
  demand         supplier     <-- PARALLEL
    |               |
    +-------+-------+
            |
            v
         reviewer
```

### Impact

- ~40% reduction in per-candidate evaluation time
- Demand and supplier agents run simultaneously (typically 30-60s each)
- Inngest concurrency limit (currently 3) applies per function, not per step
- Foundation for more complex parallel topologies later

### Key Files to Modify

- `internal/service/` -- new `parallel_runner.go`
- `internal/service/pipeline_orchestrator.go` -- replace sequential demand/supplier with `RunParallel`
- `internal/adapter/inngest/client.go` -- restructure steps for parallel execution

---

## Pattern 4: Structured Inter-Agent Context

### Value: Medium | Effort: Low | Priority: P1

### What Claude Code Does

`SendMessage` tool lets the coordinator send follow-up prompts to running agents. Agents communicate via file-based mailboxes. Structured message types exist (`shutdown_request`, `plan_approval_response`).

### What Estori Has

`[]AgentContext` -- a linear accumulation of `{AgentName, Facts map[string]any, Flags []string}` passed forward. No backward communication, no branching. Defined in `internal/domain/agent.go`.

### What to Build

#### Typed Context Per Stage

```go
// internal/domain/agent_context.go

type PipelineContext struct {
    Candidate    CandidateInfo          `json:"candidate"`
    Gating       *GatingContext         `json:"gating,omitempty"`
    Profitability *ProfitabilityContext  `json:"profitability,omitempty"`
    Demand       *DemandContext         `json:"demand,omitempty"`
    Supplier     *SupplierContext       `json:"supplier,omitempty"`
}

type CandidateInfo struct {
    ASIN          string  `json:"asin"`
    Title         string  `json:"title"`
    Brand         string  `json:"brand"`
    Category      string  `json:"category"`
    BuyBoxPrice   float64 `json:"buy_box_price"`
    WholesaleCost float64 `json:"wholesale_cost"`
    SellerCount   int     `json:"seller_count"`
}

type GatingContext struct {
    Passed    bool     `json:"passed"`
    RiskScore float64  `json:"risk_score"`
    Flags     []string `json:"flags"`
    Reasoning string   `json:"reasoning"`
}

type ProfitabilityContext struct {
    MarginPercent    float64 `json:"margin_percent"`
    FBAFees          float64 `json:"fba_fees"`
    NetProfit        float64 `json:"net_profit"`
    ROI              float64 `json:"roi"`
    QualitativeScore int     `json:"qualitative_score"`
    Reasoning        string  `json:"reasoning"`
}

type DemandContext struct {
    DemandScore      int    `json:"demand_score"`
    CompetitionScore int    `json:"competition_score"`
    BSRTrend         string `json:"bsr_trend"`
    BuyBoxStability  string `json:"buy_box_stability"`
    Reasoning        string `json:"reasoning"`
}

type SupplierContext struct {
    TopSupplier   string  `json:"top_supplier"`
    BestPrice     float64 `json:"best_price"`
    MinMOQ        int     `json:"min_moq"`
    LeadDays      int     `json:"lead_days"`
    SupplierCount int     `json:"supplier_count"`
    Reasoning     string  `json:"reasoning"`
}
```

#### Reviewer Feedback Loop

```go
// When reviewer scores a candidate as borderline (tier B/C boundary),
// it can request re-evaluation from a specific upstream agent.

type ReevaluationRequest struct {
    TargetAgent string `json:"target_agent"` // "demand" or "supplier"
    Question    string `json:"question"`     // specific question to answer
    Context     string `json:"context"`      // why re-evaluation is needed
}

type ReviewerOutput struct {
    OpportunityViability int                   `json:"opportunity_viability"`
    ExecutionConfidence  int                   `json:"execution_confidence"`
    SourcingFeasibility  int                   `json:"sourcing_feasibility"`
    CompositeScore       float64               `json:"composite_score"`
    Tier                 string                `json:"tier"`
    Reasoning            string                `json:"reasoning"`
    ReevaluateRequests   []ReevaluationRequest `json:"reevaluate_requests,omitempty"`
}
```

### Impact

- Type safety throughout the pipeline -- compile-time guarantees instead of runtime assertions
- Downstream agents receive only relevant, structured context
- Reviewer can trigger targeted re-evaluation without full pipeline re-run
- Foundation for the coordinator pattern

### Key Files to Modify

- `internal/domain/agent.go` -- replace `AgentContext.Facts map[string]any` with typed structs
- `internal/service/pipeline_orchestrator.go` -- unmarshal into typed context at each stage
- `internal/service/reviewer.go` -- add re-evaluation request handling

---

## Pattern 5: Per-Agent Tool Access Control

### Value: Medium | Effort: Low | Priority: P1

### What Claude Code Does

Each `AgentDefinition` declares `tools` (allowlist) or `disallowedTools` (blocklist). `resolveAgentTools()` filters the available tool pool per agent. Sub-agents can have MCP servers unavailable to the parent.

```typescript
// Claude Code tool resolution
function resolveAgentTools(agentDef, parentTools) {
    if (agentDef.tools) return filterByAllowlist(parentTools, agentDef.tools)
    if (agentDef.disallowedTools) return filterByBlocklist(parentTools, agentDef.disallowedTools)
    return parentTools
}
```

### What Estori Has

`ToolResolver` pre-fetches all data for all agents. Every agent gets the same data injected into its input map, regardless of what it needs. Defined in `internal/service/tool_resolver.go`.

### What to Build

```go
// internal/service/tool_resolver.go -- enhanced

type ToolDefinition struct {
    Name        string
    Description string
    Fetch       func(ctx context.Context, candidate map[string]any) (map[string]any, error)
    CostCredits int  // credit cost per invocation
}

var ToolRegistry = map[string]ToolDefinition{
    "sp_api_restrictions": {
        Name:        "sp_api_restrictions",
        Description: "Check selling eligibility via SP-API",
        CostCredits: 1,
    },
    "sp_api_fees": {
        Name:        "sp_api_fees",
        Description: "Get FBA fee estimates via SP-API",
        CostCredits: 1,
    },
    "brand_database": {
        Name:        "brand_database",
        Description: "Check brand blocklist and restriction database",
        CostCredits: 0,
    },
    "keepa_data": {
        Name:        "keepa_data",
        Description: "Historical pricing and BSR data from Keepa",
        CostCredits: 2,
    },
    "price_history": {
        Name:        "price_history",
        Description: "Price history from shared catalog",
        CostCredits: 0,
    },
    "web_search": {
        Name:        "web_search",
        Description: "Web search via Exa for supplier discovery",
        CostCredits: 2,
    },
    "web_scrape": {
        Name:        "web_scrape",
        Description: "Web page scraping via Firecrawl",
        CostCredits: 3,
    },
    "supplier_database": {
        Name:        "supplier_database",
        Description: "Internal supplier database lookup",
        CostCredits: 0,
    },
}

// ResolveFor fetches only the data this agent needs
func (r *ToolResolver) ResolveFor(
    ctx context.Context,
    agentDef AgentDefinition,
    candidate map[string]any,
) (map[string]any, error) {
    enriched := make(map[string]any)
    for _, toolName := range agentDef.Tools {
        tool, ok := ToolRegistry[toolName]
        if !ok {
            continue
        }
        data, err := tool.Fetch(ctx, candidate)
        if err != nil {
            // log warning, continue with partial data
            continue
        }
        for k, v := range data {
            enriched[k] = v
        }
    }
    return enriched, nil
}
```

### Impact

- Reduces token cost: agents don't receive irrelevant data in their context
- Reduces latency: fewer pre-fetch API calls per stage
- Makes data dependencies explicit and testable
- Credit cost tracking per tool invocation

### Key Files to Modify

- `internal/service/tool_resolver.go` -- add `ToolDefinition` registry, `ResolveFor` method
- `internal/service/pipeline_orchestrator.go` -- call `ResolveFor(agentDef, ...)` instead of global resolve

---

## Pattern 6: Coordinator / Sub-Agent Hierarchy

### Value: High | Effort: High | Priority: P2 (Future)

### What Claude Code Does

`CLAUDE_CODE_COORDINATOR_MODE=1` transforms the system into a manager + workers architecture. The coordinator has a dedicated system prompt focused on Research -> Synthesis -> Implementation -> Verification phases. Workers report back via task notifications; the coordinator decides what to do next.

Key properties:
- Coordinator has a workflow-oriented system prompt, not a task-specific one
- Workers are background agents with restricted tool sets
- Communication is async: workers complete independently, coordinator synthesizes
- The coordinator LLM decides sequencing, not hardcoded pipeline logic

### What Estori Has

A hardcoded pipeline sequence in Go code. The "reviewer" agent is the closest to a coordinator -- it sees all upstream context and makes the final call. But it has no ability to request more information, re-route candidates, or adapt the pipeline.

### What to Build (Phased)

#### Phase A: Meta-Reviewer with Re-evaluation (Medium Effort)

Keep the current pipeline but enhance the reviewer to trigger re-evaluation loops:

```go
type CoordinatorDecision struct {
    Action       string                `json:"action"`       // "approve", "reject", "reevaluate"
    Tier         string                `json:"tier"`
    Reevaluate   []ReevaluationRequest `json:"reevaluate,omitempty"`
    MaxLoops     int                   `json:"max_loops"`    // prevent infinite loops
}

func (o *PipelineOrchestrator) evaluateWithCoordination(
    ctx context.Context,
    candidate map[string]any,
    pipelineCtx *PipelineContext,
) (*ReviewerOutput, error) {
    for loop := 0; loop < maxReevalLoops; loop++ {
        reviewOut, err := o.runReviewer(ctx, pipelineCtx)
        if err != nil {
            return nil, err
        }
        if len(reviewOut.ReevaluateRequests) == 0 {
            return reviewOut, nil // final decision
        }
        // Re-run requested agents with refined questions
        for _, req := range reviewOut.ReevaluateRequests {
            refined, err := o.runAgentWithQuestion(ctx, req)
            if err != nil {
                continue
            }
            pipelineCtx.UpdateContext(req.TargetAgent, refined)
        }
    }
    return reviewOut, nil
}
```

#### Phase B: Full Coordinator Mode (High Effort, Future)

Replace the hardcoded pipeline with an LLM-driven orchestrator:

```go
type CampaignCoordinator struct {
    agentRuntime port.AgentRuntime
    registry     map[string]AgentDefinition
    taskStore    port.AgentTaskRepository
}

// The coordinator is itself an agent that decides what to do
func (c *CampaignCoordinator) Orchestrate(ctx context.Context, campaign Campaign) error {
    coordinatorDef := AgentDefinition{
        Name:      "coordinator",
        ModelTier: ModelTierReasoning,
        SystemPrompt: `You are the campaign coordinator. You have these worker agents available:
            - gating: evaluates IP risk and restrictions
            - profitability: evaluates margin viability
            - demand: scores sales velocity and competition
            - supplier: discovers wholesale suppliers
            - reviewer: final scoring and tier assignment

            For each candidate, decide:
            1. Which agents to run and in what order
            2. Whether to run agents in parallel when possible
            3. Whether to request re-evaluation based on results
            4. When to make a final decision

            Report your decisions as structured JSON.`,
    }
    // ... coordinator loop that interprets LLM output as orchestration commands
}
```

### Impact

- Pipeline adapts to candidate characteristics without code changes
- Coordinator can skip unnecessary stages (e.g., skip supplier search for clearly unprofitable candidates)
- Re-evaluation loops improve deal quality for borderline candidates
- Foundation for adding new agent types without pipeline code changes

### Key Files to Modify

- `internal/service/reviewer.go` -- add re-evaluation loop (Phase A)
- `internal/service/` -- new `coordinator.go` (Phase B)
- `internal/domain/` -- coordinator decision types

---

## Pattern 7: Lifecycle Hooks

### Value: Medium | Effort: Low | Priority: P0

### What Claude Code Does

`AgentDefinition` includes `hooks: FrontmatterHooks`. `SubagentStart` hooks run before agent execution. The execution loop calls hooks at defined lifecycle points: pre-run, post-run, on-error.

### What Estori Has

No hook system. Agent execution is a straight `RunAgent()` call. `ValidateAgentOutput()` exists in `domain/validation.go` but is never called because there's no post-run hook to trigger it.

### What to Build

```go
// internal/domain/agent_hooks.go

type AgentHooks struct {
    PreRun  []HookFunc
    PostRun []PostRunHookFunc
    OnError []ErrorHookFunc
}

type HookFunc func(ctx context.Context, task *AgentTaskRecord) error
type PostRunHookFunc func(ctx context.Context, task *AgentTaskRecord, output *AgentOutput) error
type ErrorHookFunc func(ctx context.Context, task *AgentTaskRecord, err error) error
```

### Default Hooks

```go
// internal/service/default_hooks.go

// ValidateOutputHook calls ValidateAgentOutput after every agent run
func ValidateOutputHook(ctx context.Context, task *AgentTaskRecord, output *AgentOutput) error {
    if output == nil {
        return fmt.Errorf("agent %s returned nil output", task.AgentName)
    }
    warnings := domain.ValidateAgentOutput(task.AgentName, output)
    if len(warnings) > 0 {
        log.Warn().
            Str("agent", task.AgentName).
            Str("task_id", task.ID).
            Strs("warnings", warnings).
            Msg("agent output validation warnings")
    }
    return nil
}

// MetricsHook emits agent execution metrics
func MetricsHook(ctx context.Context, task *AgentTaskRecord, output *AgentOutput) error {
    metrics.AgentDuration.WithLabelValues(task.AgentName).Observe(float64(task.DurationMs))
    metrics.AgentTokens.WithLabelValues(task.AgentName).Add(float64(task.TokensUsed))
    if task.Status == TaskStatusFailed {
        metrics.AgentFailures.WithLabelValues(task.AgentName).Inc()
    }
    return nil
}

// DomainEventHook emits domain events for agent lifecycle
func DomainEventHook(ctx context.Context, task *AgentTaskRecord, output *AgentOutput) error {
    event := domain.DomainEvent{
        Type:      "agent.completed",
        TenantID:  task.TenantID,
        Payload: map[string]any{
            "task_id":     task.ID,
            "agent_name":  task.AgentName,
            "status":      task.Status,
            "duration_ms": task.DurationMs,
            "tokens_used": task.TokensUsed,
        },
    }
    return eventStore.Publish(ctx, event)
}

// RateLimitHook checks rate limits before agent execution
func RateLimitHook(ctx context.Context, task *AgentTaskRecord) error {
    allowed, err := rateLimiter.Allow(ctx, task.TenantID, task.AgentName)
    if err != nil {
        return err
    }
    if !allowed {
        return fmt.Errorf("rate limit exceeded for tenant %s, agent %s", task.TenantID, task.AgentName)
    }
    return nil
}
```

### Wiring Hooks to Agent Definitions

```go
// In agent registry setup

var DefaultHooks = AgentHooks{
    PreRun:  []HookFunc{RateLimitHook},
    PostRun: []PostRunHookFunc{ValidateOutputHook, MetricsHook, DomainEventHook},
    OnError: []ErrorHookFunc{ErrorMetricsHook},
}

// Applied to every agent definition at initialization
func init() {
    for name, def := range AgentRegistry {
        def.Hooks = &DefaultHooks
        AgentRegistry[name] = def
    }
}
```

### Impact

- `ValidateAgentOutput()` finally gets called -- as a `PostRun` hook, not forgotten dead code
- Per-agent metrics (duration, tokens, failure rate) available for dashboards
- Rate limiting per tenant per agent prevents runaway costs
- Domain events emitted for every agent lifecycle transition
- Easy to add new cross-cutting concerns without modifying pipeline code

### Key Files to Modify

- `internal/domain/` -- new `agent_hooks.go`
- `internal/service/` -- new `default_hooks.go`
- `internal/service/pipeline_orchestrator.go` -- execute hooks around `RunAgent` calls

---

## Implementation Roadmap

### Phase 1: Foundation (Weeks 1-2)

**Goal:** Fix critical risks, establish typed contracts.

| Task | Pattern | Effort | Blocks |
|------|---------|--------|--------|
| Define `AgentDefinition` struct and registry | Pattern 1 | 2 days | Everything |
| Define typed output structs per agent | Pattern 1 | 1 day | Pattern 4 |
| Implement `AgentHooks` and default hooks | Pattern 7 | 2 days | Pattern 2 |
| Wire `ModelTier` through OpenFang adapter | Pattern 1 | 1 day | None |
| Wire `Tools` through `ToolResolver` | Pattern 5 | 2 days | None |

**Verification:** All existing pipeline tests pass with typed outputs. `ValidateAgentOutput` is called after every agent run. Per-agent model selection works.

### Phase 2: Observability (Weeks 3-4)

**Goal:** Full visibility into pipeline execution.

| Task | Pattern | Effort | Blocks |
|------|---------|--------|--------|
| Implement `AgentTaskRecord` and state machine | Pattern 2 | 3 days | Pattern 3 |
| Create `agent_task_records` migration | Pattern 2 | 1 day | None |
| Implement `AgentTaskRepository` (port + adapter) | Pattern 2 | 2 days | None |
| Wrap all `RunAgent` calls with `runAgentWithTracking` | Pattern 2 | 2 days | Pattern 7 |
| Add pipeline run metrics and dashboard | Pattern 2 | 2 days | None |

**Verification:** Every pipeline run produces a complete task tree in the database. Dashboard shows per-agent duration, token usage, and failure rates.

### Phase 3: Parallelism and Context (Weeks 5-6)

**Goal:** Faster pipelines, better data flow.

| Task | Pattern | Effort | Blocks |
|------|---------|--------|--------|
| Implement `RunParallel` for demand + supplier | Pattern 3 | 3 days | Pattern 2 |
| Restructure Inngest steps for parallel execution | Pattern 3 | 2 days | None |
| Replace `[]AgentContext` with typed `PipelineContext` | Pattern 4 | 3 days | Pattern 1 |
| Deduplicate `RunPipeline` / `EvaluateCandidate` | -- | 1 day | Pattern 4 |

**Verification:** Demand + supplier run in parallel. Per-candidate evaluation time reduced ~40%. Pipeline context is fully typed.

### Phase 4: Coordination (Weeks 7-10)

**Goal:** Adaptive pipeline behavior.

| Task | Pattern | Effort | Blocks |
|------|---------|--------|--------|
| Implement reviewer re-evaluation loop | Pattern 6 Phase A | 3 days | Pattern 4 |
| Add `ReevaluationRequest` handling | Pattern 6 Phase A | 2 days | None |
| Design coordinator system prompt | Pattern 6 Phase B | 3 days | All above |
| Implement coordinator orchestration loop | Pattern 6 Phase B | 5 days | All above |

**Verification:** Reviewer can trigger re-evaluation for borderline candidates. Coordinator mode can run a full campaign with dynamic agent sequencing.

---

## Summary

| Pattern | Value | Effort | Phase |
|---------|-------|--------|-------|
| 1. Typed Agent Definitions | High | Low | 1 |
| 2. Task State Machine | High | Medium | 2 |
| 3. Parallel Execution | High | Medium | 3 |
| 4. Structured Context | Medium | Low | 3 |
| 5. Per-Agent Tool Access | Medium | Low | 1 |
| 6. Coordinator Hierarchy | High | High | 4 |
| 7. Lifecycle Hooks | Medium | Low | 1 |

Phase 1 is the highest-leverage work: it fixes the critical silent-failure risk, enables per-agent model selection, creates the hook system that makes `ValidateAgentOutput` operational, and establishes the foundation for every subsequent phase.
