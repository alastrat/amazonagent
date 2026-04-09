# Tech Debt Backlog

**Date:** 2026-04-09
**Status:** Tracked — not blocking features, address opportunistically
**Source:** Code review of `feat/shared-catalog-and-credits` PR #2

---

## TD-1: Unify Brand / SharedBrand types and tables

**Priority:** High — divergence risk grows with every feature
**Effort:** 1-2 days
**Files affected:** ~8

### Problem

Two overlapping brand concepts:
- `domain.Brand` + `brands` table (Phase 1 original) — used by `BrandRepo`, `BrandEligibilityService`
- `domain.SharedBrand` + `brand_catalog` table (Phase 0 new) — used by `BrandCatalogRepo`, `SharedCatalogService`

Both store name + normalized_name. `SharedBrand` adds typical_gating, categories[], product_count. Neither references the other. Over time one gets updated, the other stale.

### Fix

1. Add columns to `brands` table: `typical_gating TEXT DEFAULT 'unknown'`, `categories TEXT[] DEFAULT '{}'`, `product_count INT DEFAULT 0`
2. Migrate data from `brand_catalog` into `brands`
3. Drop `brand_catalog` table
4. Merge `SharedBrand` fields into `domain.Brand`
5. Merge `BrandCatalogRepo` methods into `BrandRepo`
6. Update `SharedCatalogService` to use unified `BrandRepo`
7. Update all tests

### Risk

Migration needs to handle name collisions between the two tables (same brand in both). Use `ON CONFLICT (normalized_name) DO UPDATE` during merge.

---

## TD-2: Batch INSERT for repo "batch" methods

**Priority:** High — 300 round-trips during assessment scan
**Effort:** 1 day
**Files affected:** 5 repos

### Problem

Five repos have "batch" methods that loop individual INSERTs:

| Repo | Method | Rows | Round-trips |
|------|--------|:----:|:-----------:|
| `eligibility_fingerprint_repo` | `SaveProbeResults` | 300 | 300 |
| `eligibility_fingerprint_repo` | `SaveCategoryEligibilities` | ~30 | 30 |
| `shared_catalog_repo` | `UpsertProductBatch` | 10-200 | N |
| `suggestion_repo` | `CreateBatch` | ≤20 | 20 |
| `tenant_eligibility_repo` | `SetBatch` | N | N |

### Fix

Replace loops with multi-row INSERT:

```sql
INSERT INTO assessment_probe_results (fingerprint_id, tenant_id, asin, brand, category, tier, eligible, reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8),
       ($1, $2, $9, $10, $11, $12, $13, $14),
       ...
```

For upsert cases (`UpsertProductBatch`, `SetBatch`), use multi-row INSERT with ON CONFLICT.

`pgx.CopyFrom` is faster but doesn't support ON CONFLICT — use it only for insert-only tables (probe_results, credit_transactions). Use multi-row INSERT for upsert tables.

Consider extracting a helper:
```go
func BatchInsert(ctx, pool, table, columns, rows) error
func BatchUpsert(ctx, pool, table, columns, conflictColumns, rows) error
```

### Impact

Assessment scan: 300 round-trips → 1 (or a few chunks of 50). ~10x faster.

---

## TD-3: Parallel SP-API calls in assessment scan

**Priority:** Medium — improves onboarding UX (85s → ~17s)
**Effort:** 0.5 days
**Files affected:** 1 (assessment_service.go)

### Problem

`RunEligibilityScan` checks 300 ASINs sequentially. At 3.5 req/sec sustained, that's ~85 seconds. The onboarding "Discover" step makes the user wait this entire time.

### Fix

Use `errgroup` with bounded concurrency:

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(5) // 5 concurrent, rate limiter handles per-second

for _, probe := range probes {
    probe := probe
    g.Go(func() error {
        // check eligibility, write to thread-safe results
        return nil
    })
}
g.Wait()
```

Requires:
- `sync.Mutex` on `categoryMap` and `brandResults` slice (or use channels)
- Rate limiter integration (already built in SP-API client)
- Testing with mock that simulates concurrent access

### Impact

Assessment wall-clock: ~85s → ~17s. Onboarding "Discover" step becomes near-instant.

---

## TD-4: QueryBuilder helper for dynamic WHERE clauses

**Priority:** Low — reduces boilerplate, prevents placeholder bugs
**Effort:** 0.5 days
**Files affected:** 5+ repos

### Problem

Dynamic WHERE clause building with manual placeholder numbering is repeated across repos:

```go
where := "WHERE tenant_id = $1"
args := []any{tenantID}
argN := 2
if filter.Category != nil {
    where += fmt.Sprintf(" AND category = $%d", argN)
    args = append(args, *filter.Category)
    argN++
}
```

This pattern caused a real bug in `tenant_eligibility_repo.go` where placeholder ordering was wrong (fixed in CodeRabbit review). The manual `argN` tracking is error-prone.

### Fix

Extract a `QueryBuilder` in the postgres package:

```go
type QueryBuilder struct {
    where  strings.Builder
    args   []any
    argN   int
}

func NewQueryBuilder(baseWhere string, args ...any) *QueryBuilder
func (q *QueryBuilder) And(clause string, arg any) *QueryBuilder
func (q *QueryBuilder) OrderBy(column, direction string) *QueryBuilder
func (q *QueryBuilder) Limit(n int) *QueryBuilder
func (q *QueryBuilder) Build() (string, []any)
```

Usage:
```go
qb := postgres.NewQuery("WHERE tenant_id = $?", tenantID)
if filter.Category != nil {
    qb.And("category = $?", *filter.Category)
}
query, args := qb.OrderBy("created_at", "DESC").Limit(50).Build()
```

### Impact

Prevents placeholder ordering bugs. Reduces ~5 lines of boilerplate per filter to 1 line.

---

## TD-5: Unify DiscoveredProduct / SharedProduct types

**Priority:** Low — same situation as TD-1 but for products
**Effort:** 2-3 days (larger scope)
**Files affected:** ~12

### Problem

Two product types with 8+ overlapping fields:
- `domain.DiscoveredProduct` — per-tenant, in `discovered_products` table
- `domain.SharedProduct` — platform-wide, in `product_catalog` table

Plus `DiscoverySuggestion` duplicates many of the same fields (ASIN, Title, Brand, Category, BuyBoxPrice, EstimatedMargin, BSRRank, SellerCount).

### Fix

Extract a `ProductCore` embedded struct:
```go
type ProductCore struct {
    ASIN        string  `json:"asin"`
    Title       string  `json:"title"`
    Brand       string  `json:"brand"`
    Category    string  `json:"category"`
    BuyBoxPrice float64 `json:"buy_box_price"`
    BSRRank     int     `json:"bsr_rank"`
    SellerCount int     `json:"seller_count"`
}

type SharedProduct struct {
    ProductCore
    EstimatedMargin float64    `json:"estimated_margin_pct"`
    // ... shared-only fields
}

type DiscoveredProduct struct {
    ProductCore
    TenantID TenantID `json:"tenant_id"`
    // ... tenant-only fields
}
```

### Deferred because

This touches the discovery engine (PR #1, already merged), the shared catalog (PR #2), and the suggestion system. Needs careful migration of both tables and all repos/services that reference these types. Better as a standalone refactor PR.

---

## TD-6: Monthly credit reset should be a scheduled job, not inline

**Priority:** Low — correctness issue under high concurrency
**Effort:** 0.5 days
**Files affected:** 2

### Problem

`CreditService.GetBalance` checks `time.Now().After(account.ResetAt)` on every read and triggers a write (reset + re-fetch) inline. This means:
- Every credit check during a request can trigger a DB write
- Under concurrency, multiple requests can race to reset the same account
- Read path has unexpected write latency

### Fix

1. Add an Inngest cron (1st of month, midnight UTC) that resets all accounts:
   ```sql
   UPDATE credit_accounts SET used_this_month = 0,
     reset_at = date_trunc('month', now()) + interval '1 month'
   WHERE reset_at <= now()
   ```
2. Remove the inline reset check from `GetBalance`
3. The `ResetMonthly` repo method already exists — just needs a cron caller

### Impact

Eliminates write-on-read contention. Credit reads become pure reads.

---

## Priority Summary

| ID | What | Priority | Effort | Risk if deferred |
|----|------|----------|--------|-----------------|
| TD-1 | Unify Brand/SharedBrand | High | 1-2 days | Data divergence between two brand tables |
| TD-2 | Batch INSERT | High | 1 day | 300 round-trips per assessment (slow onboarding) |
| TD-3 | Parallel SP-API | Medium | 0.5 days | 85s assessment wait (poor UX) |
| TD-4 | QueryBuilder | Low | 0.5 days | Placeholder bugs (already had one) |
| TD-5 | Unify product types | Low | 2-3 days | Field drift between 3 product-like types |
| TD-6 | Credit reset cron | Low | 0.5 days | Write-on-read under concurrency |

**Recommended order:** TD-2 → TD-1 → TD-3 → TD-6 → TD-4 → TD-5

TD-2 + TD-3 together cut assessment time from ~85s + 300 round-trips to ~17s + 1 round-trip — directly improves the onboarding "Wealthfront moment."
