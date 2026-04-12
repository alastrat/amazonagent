# Risks & Improvement Opportunities: Estori Platform

*Last updated: 2026-04-11*
*Status: Draft -- pending engineering review*

---

## Critical Risks

### 1. Silent Agent Output Failures

**Severity:** Critical
**Affected code:** `internal/service/pipeline_orchestrator.go`

The entire 7-agent pipeline passes data via `map[string]any`. Every field extraction uses Go type assertions with silent zero-value fallbacks:

```go
passed, _ := gatingOut.Structured["passed"].(bool)  // "yes" -> false -> candidate killed
demand_score := getInt(demandOut.Structured, "demand_score")  // missing -> 0 -> bad score
```

`ValidateAgentOutput()` exists in `internal/domain/validation.go` but is never called in production paths. An LLM returning `"passed": "yes"` instead of `"passed": true` silently eliminates every candidate in a run with no error, no log, no alert.

**Impact:** Entire pipeline runs produce zero results with no indication of why. Users see "no deals found" and churn.

**Remediation:**
- Call `ValidateAgentOutput()` after every agent call in the pipeline
- Define typed structs for each agent's expected output (`GatingOutput`, `DemandOutput`, etc.)
- Emit structured warnings when type coercion fails
- Add pipeline health metric: if a run produces 0 survivors from N candidates, flag it
- Estimated effort: Medium

---

### 2. No RLS Policies -- Cross-Tenant Data Leak Vector

**Severity:** Critical
**Affected code:** Database migrations, `internal/adapter/supabase/auth_provider.go:129`

CLAUDE.md states "RLS as safety net" but no migration contains `ENABLE ROW LEVEL SECURITY` or `CREATE POLICY`. All tenant isolation relies on application-level `WHERE tenant_id = $2` clauses. Combined with the hardcoded fallback tenant ID (`"00000000-0000-0000-0000-000000000010"`) when JWT `app_metadata.tenant_id` is missing, a provisioning bug becomes a data leak.

**Impact:** New users without properly provisioned tenant IDs silently get access to another tenant's data. In a multi-tenant SaaS handling commercial data, this is a compliance and trust failure.

**Remediation:**
- Add RLS policies to every tenant-scoped table as a defense-in-depth layer
- Remove the hardcoded fallback tenant ID -- return an auth error instead
- Add middleware that rejects requests with missing tenant context
- Estimated effort: Low

---

### 3. Pipeline Code Duplication -- Guaranteed Behavioral Drift

**Severity:** Critical
**Affected code:** `internal/service/pipeline_orchestrator.go` lines 26-328 and 455-609

`RunPipeline()` and `EvaluateCandidate()` are near-identical 300-line implementations. They already diverge: `EvaluateCandidate` hardcodes `"US"` as marketplace instead of reading from config. The weighted scoring formula is duplicated verbatim across both methods.

**Impact:** Fixes applied to one code path don't reach the other. Scoring inconsistencies between batch discovery runs and single candidate evaluations. Bugs get fixed in one place and persist in the other.

**Remediation:**
- Extract a single `evaluateOne(ctx, candidate, config, criteria) (*CandidateResult, error)` method
- Both public methods become thin wrappers over the shared implementation
- Estimated effort: Low

---

## High-Priority Risks

### 4. Frontend Auth Token Leak in SSR

**Severity:** High
**Affected code:** `apps/web/src/lib/api-client.ts:290`

`apiClient` is a module-level singleton. The token is stored as `this.token`. In Next.js App Router with server components, module singletons persist across requests -- meaning one user's auth token can leak to another user's server-rendered request.

**Impact:** Cross-user session hijacking in production under concurrent load.

**Remediation:**
- Use request-scoped token passing (cookies or headers per-request) instead of a singleton token field
- Or ensure the API client is never used in server components
- Estimated effort: Low

---

### 5. Non-Atomic Batch Operations

**Severity:** High
**Affected code:** `internal/adapter/postgres/deal_repo.go` -- `CreateBatch`

`CreateBatch` iterates and calls `Create` one by one with no transaction wrapper. A failure at deal 7 of 10 leaves 6 orphaned deals in the database with no complete pipeline context.

**Impact:** Partial data corruption, phantom deals appearing in the UI with incomplete scoring data. No rollback mechanism.

**Remediation:**
- Wrap batch operations in a `pgx.Tx` transaction
- Either all deals persist or none do
- Estimated effort: Low

---

### 6. No Circuit Breaker on Agent Calls

**Severity:** High
**Affected code:** `internal/adapter/openfang/agent_runtime.go:34`

The OpenFang HTTP client has a 120s timeout but no retry logic, no circuit breaker, no backoff. During a large campaign run (100+ candidates), a slow LLM response blocks goroutines up to the full timeout duration.

**Impact:** A single slow agent call during a batch run can cascade into resource exhaustion. No graceful degradation. Users experience hung pipelines with no feedback.

**Remediation:**
- Implement a circuit breaker pattern (e.g., `sony/gobreaker`)
- Add exponential backoff with jitter for retries
- Set per-agent call budgets (max concurrent calls, max total duration)
- Estimated effort: Low

---

## Moderate Risks

### 7. Price List Item-to-ASIN Mapping Bug

**Severity:** Moderate
**Affected code:** `internal/service/pricelist_scanner.go` lines 401-418, line 225

In `ScanWithFunnel`, the code iterates all items looking for `WholesaleCost > 0` and breaks on the first match -- meaning every ASIN in a multi-item scan gets the wholesale cost of the first profitable item. `ScanPriceList` maps results by position index, not by identifier, making UPC-to-ASIN mapping correct only if the API returns results in identical order.

**Impact:** Margin calculations are wrong for multi-item price lists. Users approve deals based on incorrect profitability data. This directly undermines the platform's core value proposition.

**Remediation:**
- Map results by UPC/ASIN identifier, not by array position
- Assign wholesale cost per-item, not from first match
- Estimated effort: Low

---

### 8. Unstructured Domain Events

**Severity:** Moderate
**Affected code:** Domain event infrastructure

`DomainEvent` payload is `map[string]any` with no schema, no event type registry, and no versioning field. Consuming events requires brittle string matching on `event_type` with no compile-time safety.

**Impact:** As the system scales, event consumers break silently when payload shapes change. Adding new event consumers is error-prone. No ability to replay or validate historical events.

**Remediation:**
- Define typed event payloads per event type
- Add an event version field for forward compatibility
- Create an event type registry with consumer contracts
- Estimated effort: Medium

---

### 9. Error Type Conflation at API Layer

**Severity:** Moderate
**Affected code:** `internal/api/handler/deal_handler.go:77`

Returns HTTP 404 for any `GetByID` error, including database connectivity failures. The domain `ErrNotFound` sentinel exists but isn't checked with `errors.Is`.

**Impact:** Database outages surface as "not found" to the user, masking infrastructure issues from monitoring and alerting. Operators cannot distinguish "entity missing" from "system broken."

**Remediation:**
- Use `errors.Is(err, ErrNotFound)` to discriminate error types
- Return 500 for infrastructure errors, 404 only for genuine not-found
- Estimated effort: Low

---

## Improvement Opportunities

### Summary Table

| Area | Opportunity | Effort | Impact | Priority |
|------|-----------|--------|--------|----------|
| Typed agent outputs | Replace `map[string]any` with Go structs per agent stage | Medium | Eliminates #1 silent failure mode | P0 |
| RLS policies | Add Postgres RLS to all tenant-scoped tables | Low | Defense-in-depth for multi-tenancy | P0 |
| Pipeline dedup | Extract shared `evaluateOne()` method | Low | Prevents scoring drift between code paths | P0 |
| Request-scoped auth | Fix singleton API client token handling | Low | Prevents cross-user token leak in SSR | P1 |
| Transaction batching | Wrap `CreateBatch` in `pgx.Tx` | Low | Prevents partial data corruption | P1 |
| Circuit breaker | Add `gobreaker` to OpenFang adapter | Low | Graceful degradation under LLM latency | P1 |
| Observability | Add structured logging + pipeline health metrics | Medium | Makes silent failures visible; enables alerting | P1 |
| Price list mapping | Fix UPC-to-ASIN mapping to use identifiers, not position | Low | Correct margin calculations | P1 |
| Event schema registry | Typed event payloads with versioning | Medium | Enables reliable event consumption at scale | P2 |
| Error discrimination | Use `errors.Is(err, ErrNotFound)` at handler layer | Low | Correct HTTP status codes, better monitoring | P2 |

### Recommended Execution Order

**Phase 1 -- Before any customer-facing launch (P0):**
1. Remove hardcoded fallback tenant ID and add RLS policies
2. Call `ValidateAgentOutput()` in the pipeline; add pipeline health alerting
3. Deduplicate `RunPipeline` / `EvaluateCandidate`

**Phase 2 -- Before scaling beyond design partners (P1):**
4. Fix API client singleton token leak
5. Add transaction wrapping to batch operations
6. Implement circuit breaker on OpenFang calls
7. Fix price list item-to-ASIN mapping
8. Add structured observability (pipeline run metrics, agent call latency, error rates)

**Phase 3 -- Before enterprise tier (P2):**
9. Typed domain event payloads with versioning
10. Error type discrimination at handler layer

---

## Go-to-Market Risks

For market, competitive, and platform dependency risks, see [Go-to-Market Strategy](go-to-market-strategy.md#5-risk-analysis).
