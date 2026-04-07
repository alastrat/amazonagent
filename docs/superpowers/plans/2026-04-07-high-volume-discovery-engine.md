# High-Volume Discovery Engine — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace keyword-based discovery (20 products) with a supply-driven, tiered elimination funnel that processes 100K+ products per scan. Price list upload becomes the primary entry point. Brand intelligence compounds across scans.

**Spec:** [High-Volume Discovery Engine Spec](../specs/2026-04-07-high-volume-discovery-engine.md)
**Research:** [Expert Analysis](../research/2026-04-07-high-volume-discovery-expert-analysis.md)

**Architecture:** Tiered funnel (T0: dedup → T1: local math → T2: brand gate → T3: SP-API enrich → T4: LLM pipeline). Persistent product catalog in Postgres. Adaptive SP-API rate limiter. Enhanced price list scanner feeds funnel. Category scanning as background enrichment.

**Tech Stack:** Go 1.23+, Postgres (pgx), Inngest, SP-API, existing hexagonal architecture (domain → port → service → adapter)

---

## Dependency Graph

```
Phase A: Persistent Catalog (foundation — everything depends on this)
    │
    ├── Phase B: Tiered Funnel + Rate Limiter (depends on A)
    │       │
    │       ├── Phase C: Enhanced Price List Scanner (depends on B)
    │       │
    │       └── Phase D: Category Background Scan (depends on B)
    │
    └── Phase E: Catalog Refresh + Brand Intelligence (depends on A, B)
            │
            └── Phase F: Frontend (depends on C, D, E)
```

Phases C and D are independent and can be parallelized after B.

---

## File Structure

### New files

```
internal/domain/catalog.go                          -- DiscoveredProduct, PriceSnapshot, ScanJob, ScanType
internal/domain/browse_node.go                      -- BrowseNode domain type
internal/port/catalog.go                            -- DiscoveredProductRepo, PriceHistoryRepo, BrowseNodeRepo, ScanJobRepo
internal/port/rate_limiter.go                       -- RateLimiter interface
internal/adapter/postgres/discovered_product_repo.go
internal/adapter/postgres/price_history_repo.go
internal/adapter/postgres/browse_node_repo.go
internal/adapter/postgres/scan_job_repo.go
internal/adapter/postgres/migrations/005_discovered_products.sql
internal/adapter/postgres/migrations/006_price_history.sql
internal/adapter/postgres/migrations/007_browse_nodes.sql
internal/adapter/postgres/migrations/008_scan_jobs.sql
internal/adapter/postgres/migrations/009_brand_intelligence_view.sql
internal/adapter/spapi/rate_limiter.go              -- Adaptive token bucket rate limiter
internal/service/catalog_service.go                 -- Catalog upsert, refresh
internal/service/catalog_service_test.go
internal/service/funnel_service.go                  -- T0-T3 tiered elimination funnel
internal/service/funnel_service_test.go
internal/service/category_scan_service.go           -- Browse node rotation + nightly scan
internal/service/category_scan_service_test.go
internal/api/handler/catalog_handler.go             -- Catalog + brand intelligence API
internal/api/handler/scan_handler.go                -- Scan job API
apps/web/src/app/(app)/catalog/page.tsx             -- Catalog explorer
apps/web/src/app/(app)/brands/page.tsx              -- Brand intelligence
apps/web/src/app/(app)/brands/[id]/page.tsx         -- Brand detail
apps/web/src/hooks/use-catalog.ts
apps/web/src/hooks/use-brands.ts
apps/web/src/hooks/use-scans.ts
```

### Modified files

```
internal/domain/brand.go                            -- Add Category field to BrandEligibility
internal/domain/campaign.go                         -- Add ScanType field
internal/port/repository.go                         -- Add new repo interfaces (or in new file)
internal/adapter/postgres/brand_repo.go             -- Category-scoped eligibility queries
internal/adapter/spapi/client.go                    -- Integrate rate limiter
internal/service/pricelist_scanner.go               -- Rewrite to use funnel + catalog
internal/service/brand_eligibility_service.go       -- Category-scoped checks
internal/adapter/inngest/client.go                  -- New workflows (pricelist, category scan, refresh)
internal/api/router.go                              -- Mount new endpoints
internal/api/handler/pricelist_handler.go           -- Enhanced upload with progress
apps/api/main.go                                    -- Wire new services
apps/web/src/app/(app)/layout.tsx                   -- Add nav items
apps/web/src/lib/types.ts                           -- New frontend types
apps/web/src/lib/api-client.ts                      -- New API methods
```

---

## Phase A: Persistent Product Catalog

**Checkpoint:** After Phase A, `discovered_products`, `price_history`, `browse_nodes`, and `scan_jobs` tables exist with repos. CatalogService can upsert and query products. All tests pass.

### Task A1: Domain Types

**Files:**
- Create: `internal/domain/catalog.go`
- Create: `internal/domain/browse_node.go`

- [ ] **Step 1: Create `internal/domain/catalog.go`**

```go
package domain

import "time"

type ScanType string

const (
    ScanTypePriceList ScanType = "pricelist"
    ScanTypeCategory  ScanType = "category"
    ScanTypeKeyword   ScanType = "keyword"
)

// DataQuality bitmask flags
const (
    DataQualityPrice       = 1  // has estimated_price
    DataQualityBSR         = 2  // has bsr_rank
    DataQualityFees        = 4  // has FBA fee calculation
    DataQualityEligibility = 8  // has eligibility check
    DataQualityBuyBox      = 16 // has buy_box_price from competitive pricing
)

type DiscoveredProduct struct {
    ID                 string    `json:"id"`
    TenantID           TenantID  `json:"tenant_id"`
    ASIN               string    `json:"asin"`
    Title              string    `json:"title"`
    BrandID            string    `json:"brand_id,omitempty"`
    Category           string    `json:"category"`
    BrowseNodeID       string    `json:"browse_node_id,omitempty"`
    EstimatedPrice     float64   `json:"estimated_price"`
    BuyBoxPrice        float64   `json:"buy_box_price"`
    BSRRank            int       `json:"bsr_rank"`
    SellerCount        int       `json:"seller_count"`
    EstimatedMarginPct float64   `json:"estimated_margin_pct"`
    RealMarginPct      float64   `json:"real_margin_pct"`
    EligibilityStatus  string    `json:"eligibility_status"`
    DataQuality        int       `json:"data_quality"`
    RefreshPriority    float64   `json:"refresh_priority"`
    Source             ScanType  `json:"source"`
    FirstSeenAt        time.Time `json:"first_seen_at"`
    LastSeenAt         time.Time `json:"last_seen_at"`
    PriceUpdatedAt     *time.Time `json:"price_updated_at,omitempty"`
}

type PriceSnapshot struct {
    ASIN        string    `json:"asin"`
    TenantID    TenantID  `json:"tenant_id"`
    RecordedAt  time.Time `json:"recorded_at"`
    AmazonPrice float64   `json:"amazon_price"`
    BSRRank     int       `json:"bsr_rank"`
    SellerCount int       `json:"seller_count"`
}

type ScanJobID string

type ScanJob struct {
    ID          ScanJobID `json:"id"`
    TenantID    TenantID  `json:"tenant_id"`
    Type        ScanType  `json:"type"`
    Status      string    `json:"status"`
    TotalItems  int       `json:"total_items"`
    Processed   int       `json:"processed"`
    Qualified   int       `json:"qualified"`
    Eliminated  int       `json:"eliminated"`
    StartedAt   time.Time `json:"started_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
    Metadata    map[string]any `json:"metadata,omitempty"`
}

type BrandIntelligence struct {
    TenantID        TenantID `json:"tenant_id"`
    BrandID         string   `json:"brand_id"`
    BrandName       string   `json:"brand_name"`
    Category        string   `json:"category"`
    ProductCount    int      `json:"product_count"`
    HighMarginCount int      `json:"high_margin_count"`
    AvgMargin       float64  `json:"avg_margin"`
    AvgSellers      float64  `json:"avg_sellers"`
    AvgBSR          float64  `json:"avg_bsr"`
}
```

- [ ] **Step 2: Create `internal/domain/browse_node.go`**

```go
package domain

import "time"

type BrowseNode struct {
    ID            string     `json:"id"`
    AmazonNodeID  string     `json:"amazon_node_id"`
    Name          string     `json:"name"`
    ParentNodeID  string     `json:"parent_node_id,omitempty"`
    Depth         int        `json:"depth"`
    IsLeaf        bool       `json:"is_leaf"`
    LastScannedAt *time.Time `json:"last_scanned_at,omitempty"`
    ProductsFound int        `json:"products_found"`
    ScanPriority  float64    `json:"scan_priority"`
}
```

- [ ] **Step 3: Add ScanType to Campaign**

In `internal/domain/campaign.go`, add `ScanType ScanType` field to the `Campaign` struct.

- [ ] **Step 4: Verify build**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

---

### Task A2: Database Migrations

**Files:**
- Create: `internal/adapter/postgres/migrations/005_discovered_products.sql`
- Create: `internal/adapter/postgres/migrations/006_price_history.sql`
- Create: `internal/adapter/postgres/migrations/007_browse_nodes.sql`
- Create: `internal/adapter/postgres/migrations/008_scan_jobs.sql`
- Create: `internal/adapter/postgres/migrations/009_brand_intelligence_view.sql`

- [ ] **Step 1: Create migration 005 — discovered_products**

```sql
CREATE TABLE IF NOT EXISTS discovered_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    asin TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    brand_id UUID REFERENCES brands(id),
    category TEXT NOT NULL DEFAULT '',
    browse_node_id TEXT,
    estimated_price NUMERIC(10,2),
    buy_box_price NUMERIC(10,2),
    bsr_rank INT,
    seller_count INT,
    estimated_margin_pct NUMERIC(5,2),
    real_margin_pct NUMERIC(5,2),
    eligibility_status TEXT NOT NULL DEFAULT 'unknown',
    data_quality SMALLINT NOT NULL DEFAULT 0,
    refresh_priority REAL NOT NULL DEFAULT 0.0,
    source TEXT NOT NULL DEFAULT 'search',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    price_updated_at TIMESTAMPTZ,
    UNIQUE(tenant_id, asin)
);

CREATE INDEX IF NOT EXISTS idx_dp_tenant_brand ON discovered_products(tenant_id, brand_id);
CREATE INDEX IF NOT EXISTS idx_dp_tenant_category ON discovered_products(tenant_id, category);
CREATE INDEX IF NOT EXISTS idx_dp_tenant_refresh ON discovered_products(tenant_id, refresh_priority DESC)
    WHERE data_quality < 31;
CREATE INDEX IF NOT EXISTS idx_dp_tenant_margin ON discovered_products(tenant_id, estimated_margin_pct DESC);
CREATE INDEX IF NOT EXISTS idx_dp_browse_node ON discovered_products(browse_node_id);
```

- [ ] **Step 2: Create migration 006 — price_history (partitioned)**

```sql
CREATE TABLE IF NOT EXISTS price_history (
    id UUID DEFAULT gen_random_uuid(),
    asin TEXT NOT NULL,
    tenant_id UUID NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    amazon_price NUMERIC(10,2),
    bsr_rank INT,
    seller_count INT
) PARTITION BY RANGE (recorded_at);

CREATE TABLE IF NOT EXISTS price_history_2026_04 PARTITION OF price_history
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE IF NOT EXISTS price_history_2026_05 PARTITION OF price_history
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS price_history_2026_06 PARTITION OF price_history
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE INDEX IF NOT EXISTS idx_ph_asin_time ON price_history(tenant_id, asin, recorded_at DESC);
```

- [ ] **Step 3: Create migration 007 — browse_nodes**

```sql
CREATE TABLE IF NOT EXISTS browse_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    amazon_node_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    parent_node_id TEXT,
    depth INT NOT NULL DEFAULT 0,
    is_leaf BOOLEAN NOT NULL DEFAULT false,
    last_scanned_at TIMESTAMPTZ,
    products_found INT DEFAULT 0,
    scan_priority REAL NOT NULL DEFAULT 0.0
);

CREATE INDEX IF NOT EXISTS idx_bn_scan ON browse_nodes(last_scanned_at NULLS FIRST)
    WHERE is_leaf = true;
```

- [ ] **Step 4: Create migration 008 — scan_jobs**

```sql
CREATE TABLE IF NOT EXISTS scan_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    total_items INT NOT NULL DEFAULT 0,
    processed INT NOT NULL DEFAULT 0,
    qualified INT NOT NULL DEFAULT 0,
    eliminated INT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_sj_tenant_status ON scan_jobs(tenant_id, status);
```

- [ ] **Step 5: Create migration 009 — brand_intelligence materialized view**

```sql
CREATE MATERIALIZED VIEW IF NOT EXISTS brand_intelligence AS
SELECT
    dp.tenant_id,
    dp.brand_id,
    b.name AS brand_name,
    dp.category,
    COUNT(*) AS product_count,
    COUNT(*) FILTER (WHERE dp.estimated_margin_pct >= 20) AS high_margin_count,
    AVG(dp.estimated_margin_pct) AS avg_margin,
    AVG(dp.seller_count) AS avg_sellers,
    AVG(dp.bsr_rank) FILTER (WHERE dp.bsr_rank > 0) AS avg_bsr
FROM discovered_products dp
JOIN brands b ON dp.brand_id = b.id
GROUP BY dp.tenant_id, dp.brand_id, b.name, dp.category;

CREATE UNIQUE INDEX IF NOT EXISTS idx_bi_tenant_brand_cat
    ON brand_intelligence(tenant_id, brand_id, category);
```

- [ ] **Step 6: Run migrations locally**

```bash
make docker-up && make migrate
```

- [ ] **Step 7: Commit**

---

### Task A3: Port Interfaces

**Files:**
- Create: `internal/port/catalog.go`

- [ ] **Step 1: Create `internal/port/catalog.go`** with interfaces: `DiscoveredProductRepo`, `PriceHistoryRepo`, `BrowseNodeRepo`, `ScanJobRepo`

Follow the patterns in `internal/port/repository.go`. All methods take `context.Context` and `domain.TenantID` where appropriate.

```go
package port

import (
    "context"
    "time"
    "github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type DiscoveredProductRepo interface {
    Upsert(ctx context.Context, product *domain.DiscoveredProduct) error
    UpsertBatch(ctx context.Context, products []domain.DiscoveredProduct) error
    GetByASIN(ctx context.Context, tenantID domain.TenantID, asin string) (*domain.DiscoveredProduct, error)
    GetByASINs(ctx context.Context, tenantID domain.TenantID, asins []string) ([]domain.DiscoveredProduct, error)
    List(ctx context.Context, tenantID domain.TenantID, filter DiscoveredProductFilter) ([]domain.DiscoveredProduct, int, error)
    ListStale(ctx context.Context, tenantID domain.TenantID, olderThan time.Time, limit int) ([]domain.DiscoveredProduct, error)
    ListByRefreshPriority(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.DiscoveredProduct, error)
    UpdatePricing(ctx context.Context, tenantID domain.TenantID, asin string, buyBoxPrice float64, sellers int, bsr int, realMarginPct float64) error
}

type DiscoveredProductFilter struct {
    Category          *string
    BrandID           *string
    MinMargin         *float64
    MinSellers        *int
    EligibilityStatus *string
    Source            *domain.ScanType
    Search            *string
    SortBy            string
    SortDir           string
    Limit             int
    Offset            int
}

type PriceHistoryRepo interface {
    Record(ctx context.Context, snapshot domain.PriceSnapshot) error
    RecordBatch(ctx context.Context, snapshots []domain.PriceSnapshot) error
    GetHistory(ctx context.Context, tenantID domain.TenantID, asin string, since time.Time) ([]domain.PriceSnapshot, error)
}

type BrowseNodeRepo interface {
    Upsert(ctx context.Context, node *domain.BrowseNode) error
    UpsertBatch(ctx context.Context, nodes []domain.BrowseNode) error
    GetNextForScan(ctx context.Context, limit int) ([]domain.BrowseNode, error)
    MarkScanned(ctx context.Context, amazonNodeID string, productsFound int) error
}

type ScanJobRepo interface {
    Create(ctx context.Context, job *domain.ScanJob) error
    GetByID(ctx context.Context, tenantID domain.TenantID, id domain.ScanJobID) (*domain.ScanJob, error)
    List(ctx context.Context, tenantID domain.TenantID, limit int) ([]domain.ScanJob, error)
    UpdateProgress(ctx context.Context, id domain.ScanJobID, processed, qualified, eliminated int) error
    Complete(ctx context.Context, id domain.ScanJobID) error
    Fail(ctx context.Context, id domain.ScanJobID) error
}

type BrandIntelligenceRepo interface {
    List(ctx context.Context, tenantID domain.TenantID, filter BrandIntelligenceFilter) ([]domain.BrandIntelligence, error)
    Refresh(ctx context.Context) error
}

type BrandIntelligenceFilter struct {
    Category     *string
    MinMargin    *float64
    MinProducts  *int
    Eligible     *bool
    Search       *string
    SortBy       string
    SortDir      string
    Limit        int
    Offset       int
}
```

- [ ] **Step 2: Verify build**
- [ ] **Step 3: Commit**

---

### Task A4: Postgres Repos

**Files:**
- Create: `internal/adapter/postgres/discovered_product_repo.go`
- Create: `internal/adapter/postgres/price_history_repo.go`
- Create: `internal/adapter/postgres/browse_node_repo.go`
- Create: `internal/adapter/postgres/scan_job_repo.go`

- [ ] **Step 1: Implement `DiscoveredProductRepo`** — follow patterns in `deal_repo.go`. Key method: `UpsertBatch` uses `ON CONFLICT (tenant_id, asin) DO UPDATE` to merge new data without losing existing fields. Only update `last_seen_at` on re-discovery.

- [ ] **Step 2: Implement `PriceHistoryRepo`** — `RecordBatch` uses `COPY` or multi-value INSERT for efficiency. Partitioned table — ensure inserts target the correct partition.

- [ ] **Step 3: Implement `BrowseNodeRepo`** — `GetNextForScan` orders by `last_scanned_at NULLS FIRST` (unscanned nodes first), then by `scan_priority DESC`.

- [ ] **Step 4: Implement `ScanJobRepo`** — `UpdateProgress` is called frequently during scans; use a single UPDATE statement.

- [ ] **Step 5: Verify build + test with `make test`**
- [ ] **Step 6: Commit**

---

### Task A5: CatalogService

**Files:**
- Create: `internal/service/catalog_service.go`
- Create: `internal/service/catalog_service_test.go`

- [ ] **Step 1: Implement `CatalogService`**

```go
type CatalogService struct {
    products  port.DiscoveredProductRepo
    prices    port.PriceHistoryRepo
    brands    BrandRepo  // existing interface in this package
    idGen     port.IDGenerator
}
```

Methods:
- `UpsertProducts(ctx, tenantID, products)` — upserts to discovered_products, resolves brand_id via brands table
- `RecordPriceSnapshot(ctx, tenantID, snapshots)` — writes to price_history
- `GetByASINs(ctx, tenantID, asins)` — cache-check for T0 dedup
- `UpdateRefreshPriority(ctx, tenantID)` — recomputes priority: `(margin/100)*0.4 + (staleness_days)*0.3 + (seller_score)*0.3`

- [ ] **Step 2: Write tests** — test upsert idempotency (same ASIN twice should update not duplicate), test refresh priority computation
- [ ] **Step 3: Verify `make test`**
- [ ] **Step 4: Commit**

---

### Task A6: Wire Phase A into main.go

**Files:**
- Modify: `apps/api/main.go`

- [ ] **Step 1: Instantiate new repos** (discovered product, price history, browse node, scan job)
- [ ] **Step 2: Instantiate CatalogService**
- [ ] **Step 3: Verify build + `make test`**
- [ ] **Step 4: Commit**

---

**CHECKPOINT A:** Deploy and verify. Run `make migrate` against Supabase to create tables. Verify tables exist. CatalogService can upsert and query products. All existing tests still pass.

---

## Phase B: Tiered Funnel + Rate Limiter

**Checkpoint:** After Phase B, the FunnelService can process a batch of products through T0-T3 with adaptive rate limiting. Unit tests prove each tier eliminates correctly.

### Task B1: Rate Limiter

**Files:**
- Create: `internal/port/rate_limiter.go`
- Create: `internal/adapter/spapi/rate_limiter.go`

- [ ] **Step 1: Create `internal/port/rate_limiter.go`**

```go
package port

import "context"

type RateLimiter interface {
    Wait(ctx context.Context, endpoint string) error
    ReportThrottle(endpoint string)
}
```

- [ ] **Step 2: Implement adaptive rate limiter** in `internal/adapter/spapi/rate_limiter.go`

Uses `golang.org/x/time/rate`. One `rate.Limiter` per SP-API endpoint. Config:

| Endpoint | Initial Rate |
|----------|:---:|
| catalog_search | 1.5/sec |
| competitive_pricing | 7/sec |
| listing_restrictions | 3.5/sec |
| catalog_items | 1.5/sec |

On `ReportThrottle`: halve the current rate, floor at 0.5/sec. Recovery: increase by 10% every 30 seconds until back to initial rate.

- [ ] **Step 3: Integrate rate limiter into SP-API client** — add `rateLimiter.Wait(ctx, endpoint)` before each API call in `internal/adapter/spapi/client.go`. Call `ReportThrottle` on HTTP 429.

- [ ] **Step 4: Verify build**
- [ ] **Step 5: Commit**

---

### Task B2: Funnel Service

**Files:**
- Create: `internal/service/funnel_service.go`
- Create: `internal/service/funnel_service_test.go`

- [ ] **Step 1: Define funnel input/output types**

```go
// FunnelInput is a product entering the funnel from any source
type FunnelInput struct {
    ASIN           string
    Title          string
    Brand          string
    Category       string
    EstimatedPrice float64  // from catalog or CSV MSRP
    WholesaleCost  float64  // from price list (0 if unknown)
    BSRRank        int
    SellerCount    int      // may be 0 if not yet enriched
    Source         domain.ScanType
}

// FunnelSurvivor is a product that passed T0-T3
type FunnelSurvivor struct {
    domain.DiscoveredProduct
    WholesaleCost float64 // carried from input (price list only)
}

// FunnelStats tracks elimination at each tier
type FunnelStats struct {
    InputCount      int `json:"input_count"`
    T0Deduped       int `json:"t0_deduped"`
    T1MarginKilled  int `json:"t1_margin_killed"`
    T2BrandKilled   int `json:"t2_brand_killed"`
    T3EnrichKilled  int `json:"t3_enrich_killed"`
    SurvivorCount   int `json:"survivor_count"`
}
```

- [ ] **Step 2: Implement `FunnelService.ProcessBatch`**

```go
type FunnelService struct {
    catalog          *CatalogService
    brandEligibility *BrandEligibilityService
    brandBlocklist   *BrandBlocklistService
    spapi            port.ProductSearcher
    rateLimiter      port.RateLimiter
}

func (s *FunnelService) ProcessBatch(
    ctx context.Context,
    tenantID domain.TenantID,
    products []FunnelInput,
    thresholds domain.PipelineThresholds,
) ([]FunnelSurvivor, FunnelStats, error)
```

Implementation:
- **T0:** For each product, check `catalog.GetByASINs` — if ASIN exists and `price_updated_at` < 24h ago, skip API enrichment (use cached data). Still apply T1/T2 filters on cached data.
- **T1:** Calculate margin. If `WholesaleCost > 0` (price list), use real cost. Otherwise, estimate at 40% of `EstimatedPrice`. Apply FBA fee calculator. Kill if margin < threshold or price outside $10-$200 range. Log each elimination with reason.
- **T2:** Group by brand. Use `BrandEligibilityService.BatchCheckBrands`. Kill restricted + blocklisted. Log each.
- **T3:** Batch remaining ASINs in groups of 20 → SP-API `getCompetitivePricing` (respecting rate limiter). Update `buy_box_price`, `seller_count`, `real_margin_pct` on `DiscoveredProduct`. Kill if seller count < threshold or real margin < threshold. Record price snapshot. Log each.
- Write all surviving products to catalog via `CatalogService.UpsertProducts`.

- [ ] **Step 3: Write tests** — test each tier independently. Mock SP-API and brand eligibility. Verify FunnelStats counts are correct. Verify both prices are preserved (estimated + buy box).

- [ ] **Step 4: Verify `make test`**
- [ ] **Step 5: Commit**

---

**CHECKPOINT B:** FunnelService processes batches end-to-end with proper elimination stats. Rate limiter adapts to throttling. All tests pass. No Inngest integration yet — funnel is a pure service.

---

## Phase C: Enhanced Price List Scanner

**Checkpoint:** After Phase C, uploading a distributor CSV runs through the tiered funnel, writes to the persistent catalog, and triggers LLM evaluation on survivors via Inngest.

### Task C1: Rewrite PriceListScanner to Use Funnel

**Files:**
- Modify: `internal/service/pricelist_scanner.go`
- Modify: `internal/service/pricelist_scanner_test.go`

- [ ] **Step 1: Refactor `PriceListScanner`** — add `FunnelService` and `CatalogService` as dependencies. Keep existing CSV parsing. Replace direct SP-API calls with funnel processing.

New flow:
1. Parse CSV → `[]PriceListItem` (existing)
2. Batch UPC→ASIN via `spapi.LookupByIdentifier` (existing, chunk into 20s)
3. Convert matched items to `[]FunnelInput` with `WholesaleCost` from CSV
4. Call `FunnelService.ProcessBatch` → survivors + stats
5. Create a `ScanJob` to track progress
6. Return survivors as `[]PriceListMatch` (existing type, enriched with funnel data)

- [ ] **Step 2: Update tests**
- [ ] **Step 3: Verify `make test`**
- [ ] **Step 4: Commit**

---

### Task C2: Inngest Price List Workflow

**Files:**
- Modify: `internal/adapter/inngest/client.go`

- [ ] **Step 1: Add `pricelist/uploaded` Inngest function**

Steps:
1. `parse-csv` — parse file, extract items
2. `match-identifiers` — batch UPC→ASIN lookup (chunk 500 at a time)
3. `run-funnel` — call FunnelService.ProcessBatch
4. `create-scan-job` — persist scan job with stats
5. `dispatch-llm` — for each survivor, emit `candidate/evaluate` event (reuse existing evaluate-candidate function)
6. `wait-for-evaluations` — sleep proportional to survivor count
7. `complete-scan` — mark scan job complete, update campaign status

- [ ] **Step 2: Update `PriceListHandler` to trigger Inngest workflow** instead of synchronous processing

- [ ] **Step 3: Test end-to-end locally** with `make docker-up && make dev`, upload a small CSV

- [ ] **Step 4: Commit**

---

### Task C3: Price List API Enhancement

**Files:**
- Modify: `internal/api/handler/pricelist_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Add endpoints:**
- `GET /pricelists/:id/status` — returns scan job progress
- `GET /pricelists/:id/results` — returns matched + qualified results

- [ ] **Step 2: Mount routes**
- [ ] **Step 3: Verify build**
- [ ] **Step 4: Commit**

---

**CHECKPOINT C:** Upload a distributor CSV → products flow through T0-T3 → survivors get LLM-evaluated → deals appear in dashboard. Scan job tracks progress. Both estimated and buy_box prices are stored.

---

## Phase D: Category Background Scan

**Checkpoint:** After Phase D, a nightly Inngest cron scans browse nodes, discovers products, runs them through the funnel, and builds the persistent catalog incrementally.

### Task D1: CategoryScanService

**Files:**
- Create: `internal/service/category_scan_service.go`
- Create: `internal/service/category_scan_service_test.go`

- [ ] **Step 1: Implement `CategoryScanService`**

```go
type CategoryScanService struct {
    nodes       port.BrowseNodeRepo
    catalog     *CatalogService
    funnel      *FunnelService
    spapi       port.ProductSearcher
    rateLimiter port.RateLimiter
}
```

Methods:
- `ScanNodes(ctx, tenantID, nodes []BrowseNode, thresholds)` — for each node, call SP-API `searchCatalogItems` with browse node filter, paginate up to 200 results. Convert to `FunnelInput` (with `EstimatedPrice` from catalog data, `WholesaleCost = 0`). Run through funnel.
- `PickNextNodes(ctx, limit)` — delegates to `BrowseNodeRepo.GetNextForScan`

- [ ] **Step 2: Write tests** — mock SP-API to return known products, verify they flow through funnel and land in catalog
- [ ] **Step 3: Commit**

---

### Task D2: SP-API Browse Node Search

**Files:**
- Modify: `internal/adapter/spapi/client.go`
- Modify: `internal/port/tools.go`

- [ ] **Step 1: Add `SearchByBrowseNode(ctx, nodeID, marketplace, pageToken) ([]ProductSearchResult, nextPageToken, error)` to `ProductSearcher` interface**

- [ ] **Step 2: Implement in SP-API client** — uses `classificationIds` parameter on catalog items search. Paginates via `nextToken`. Respects rate limiter.

- [ ] **Step 3: Verify build**
- [ ] **Step 4: Commit**

---

### Task D3: Inngest Nightly Scan Workflow

**Files:**
- Modify: `internal/adapter/inngest/client.go`

- [ ] **Step 1: Add `scan/nightly` Inngest cron function** (e.g., `0 2 * * *` — 2 AM UTC)

Steps:
1. `pick-nodes` — get next N nodes from rotation (default: 100)
2. `search-nodes` — fan-out per node, search + paginate
3. `run-funnel` — process all discovered products through T0-T3
4. `dispatch-llm-batch` — top survivors by margin get `candidate/evaluate` events
5. `complete-scan` — create scan job record, log stats

- [ ] **Step 2: Wire into main.go**
- [ ] **Step 3: Commit**

---

### Task D4: Category Scan API

**Files:**
- Create: `internal/api/handler/scan_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Add endpoints:**
- `GET /scans` — list scan jobs
- `GET /scans/:id` — scan detail with funnel stats
- `POST /scans/category` — trigger manual category scan

- [ ] **Step 2: Mount routes**
- [ ] **Step 3: Commit**

---

**CHECKPOINT D:** Nightly cron discovers products from browse nodes, runs them through the funnel, and populates the catalog. Manual trigger available via API. Scan history queryable.

---

## Phase E: Catalog Refresh + Brand Intelligence

**Checkpoint:** After Phase E, stale products are refreshed based on priority, brand intelligence is queryable, and materialized view refreshes on schedule.

### Task E1: Catalog Refresh Workflow

**Files:**
- Modify: `internal/adapter/inngest/client.go`
- Modify: `internal/service/catalog_service.go`

- [ ] **Step 1: Add `UpdateRefreshPriority` method to CatalogService** — runs SQL:

```sql
UPDATE discovered_products SET refresh_priority =
    (COALESCE(estimated_margin_pct, 0) / 100.0) * 0.4
    + (EXTRACT(EPOCH FROM now() - COALESCE(price_updated_at, first_seen_at)) / 86400.0) * 0.3
    + (CASE WHEN seller_count BETWEEN 3 AND 10 THEN 0.3 ELSE 0.1 END)
WHERE tenant_id = $1;
```

- [ ] **Step 2: Add `catalog/refresh` Inngest cron function** (e.g., `0 6 * * *` — 6 AM UTC)

Steps:
1. `select-stale` — top N products by refresh_priority
2. `batch-pricing` — competitive pricing in 20-ASIN batches via funnel T3 logic
3. `update-catalog` — write new prices, record price_history
4. `recompute-priority` — call UpdateRefreshPriority
5. `refresh-views` — `REFRESH MATERIALIZED VIEW CONCURRENTLY brand_intelligence`

- [ ] **Step 3: Commit**

---

### Task E2: Brand Intelligence API

**Files:**
- Create: `internal/api/handler/catalog_handler.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Add endpoints:**
- `GET /catalog/products` — list discovered products with filters (category, margin, sellers, eligibility, search)
- `GET /catalog/brands` — list brand intelligence (from materialized view)
- `GET /catalog/brands/:id/products` — products for a specific brand
- `GET /catalog/stats` — catalog size, freshness stats, scan counts

- [ ] **Step 2: Mount routes**
- [ ] **Step 3: Wire into main.go**
- [ ] **Step 4: Verify build**
- [ ] **Step 5: Commit**

---

**CHECKPOINT E:** Catalog self-refreshes on priority. Brand intelligence queryable via API. Materialized view stays fresh. Full backend complete.

---

## Phase F: Frontend

**Checkpoint:** After Phase F, dashboard promotes price list upload, catalog explorer and brand intelligence pages exist, scan history shows funnel stats.

### Task F1: Frontend Types + API Client

**Files:**
- Modify: `apps/web/src/lib/types.ts`
- Modify: `apps/web/src/lib/api-client.ts`

- [ ] **Step 1: Add TypeScript types** for DiscoveredProduct, BrandIntelligence, ScanJob, FunnelStats, PriceListUploadResult
- [ ] **Step 2: Add API client methods** for catalog, brands, scans endpoints
- [ ] **Step 3: Commit**

---

### Task F2: Catalog Explorer Page

**Files:**
- Create: `apps/web/src/app/(app)/catalog/page.tsx`
- Create: `apps/web/src/hooks/use-catalog.ts`

- [ ] **Step 1: Build `/catalog` page** — product table with search, sort, filter. Columns: ASIN, title, brand, category, estimated price, buy box price, estimated margin, real margin, BSR, sellers, eligibility, last updated. Filters: category, margin range, seller count, eligibility, source.

- [ ] **Step 2: Add "Evaluate Selected" bulk action** — sends selected ASINs to the LLM pipeline via campaign creation

- [ ] **Step 3: Commit**

---

### Task F3: Brand Intelligence Page

**Files:**
- Create: `apps/web/src/app/(app)/brands/page.tsx`
- Create: `apps/web/src/app/(app)/brands/[id]/page.tsx`
- Create: `apps/web/src/hooks/use-brands.ts`

- [ ] **Step 1: Build `/brands` page** — brand table with: name, category, product count, avg margin, eligible status. Filters: category, eligibility, min margin, min products.

- [ ] **Step 2: Build `/brands/:id` page** — brand summary card + products table + eligibility status per category.

- [ ] **Step 3: Commit**

---

### Task F4: Enhanced Price List Upload UX

**Files:**
- Modify: `apps/web/src/app/(app)/dashboard/page.tsx` or create component

- [ ] **Step 1: Add prominent "Upload Price List" button** to dashboard — above the campaign list
- [ ] **Step 2: Upload dialog with drag-and-drop CSV** support
- [ ] **Step 3: Progress display** — funnel stats (matched → margin filter → brand gate → enriched → LLM → deals) updating via polling
- [ ] **Step 4: Commit**

---

### Task F5: Scan History + Navigation

**Files:**
- Modify: `apps/web/src/app/(app)/discovery/page.tsx`
- Modify: `apps/web/src/app/(app)/layout.tsx`
- Create: `apps/web/src/hooks/use-scans.ts`

- [ ] **Step 1: Add scan history section** to Discovery page — list of scan jobs with type, status, items processed, qualified, funnel stats
- [ ] **Step 2: Add nav items** — Catalog, Brands in sidebar navigation
- [ ] **Step 3: Commit**

---

**CHECKPOINT F:** Full UI complete. Price list upload is the primary action. Catalog explorer and brand intelligence are browsable. Scan history shows funnel performance. Deploy to production.

---

## Relax Keyword Campaign Filters (Quick Fix)

This is independent of the main phases and can be done immediately to fix the "0 results" problem on existing keyword campaigns.

### Task Q1: Lower Default Thresholds for Keyword Campaigns

**Files:**
- Modify: `internal/domain/pipeline.go`
- Modify: `internal/adapter/inngest/client.go`

- [ ] **Step 1: In `DefaultPipelineThresholds()`**, lower `MinSellerCount` from 3 to 1 and `MinMarginPct` from 15.0 to 10.0
- [ ] **Step 2: Add per-elimination logging** in the `discover-products` Inngest step — log which filter killed each product and why
- [ ] **Step 3: Verify with `make test`**
- [ ] **Step 4: Deploy to Railway** — `railway up`
- [ ] **Step 5: Test: create a campaign with "stainless steel water bottle" — should produce results now**
- [ ] **Step 6: Commit**
