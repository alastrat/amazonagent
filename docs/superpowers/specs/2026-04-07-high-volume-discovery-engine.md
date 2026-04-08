# High-Volume Discovery Engine — Design Spec

**Date:** 2026-04-07
**Status:** Draft — pending review
**Scope:** Redesign the product discovery pipeline from keyword-search (20 products) to supply-driven high-volume scanning (100K+ products) with tiered elimination funnel
**Research:** [Expert Analysis](../research/2026-04-07-high-volume-discovery-expert-analysis.md)

---

## 1. Goal

Replace the keyword-based product discovery with a supply-driven, brand-intelligent discovery engine that processes 100K+ products per scan while spending LLM tokens only on the top ~150 pre-qualified candidates. The price list scanner becomes the primary entry point. Category scanning runs as background enrichment. Brand intelligence compounds across every scan.

---

## 2. What Changes

### Before (current)

```
User types keywords → SP-API search (20 results) → filter all → LLM on survivors → 0 deals
```

### After

```
Two primary entry points, both feeding the same tiered funnel:

Entry A: Distributor Price List Upload (primary)
  CSV with UPC/wholesale cost → UPC-to-ASIN match → real margin calc → funnel

Entry B: Category Background Scan (secondary)
  Browse node enumeration → catalog search → estimated margin → funnel

                          ┌─────────────────────────┐
                          │    TIERED FUNNEL          │
                          │                           │
                          │  T0: Dedup (cache hit)    │  100K → 40K
                          │  T1: Local math (margin)  │  40K → 10K
                          │  T2: Brand gate (cached)  │  10K → 6K
                          │  T3: Enrich (SP-API batch) │  6K → 2K
                          │  T4: LLM pipeline          │  2K → 150 → 50-100 deals
                          └─────────────────────────┘
```

### What stays the same

- LLM agent pipeline (gating, profitability, demand, supplier, reviewer) — unchanged
- Deal lifecycle, approvals, dashboard — unchanged
- Hexagonal architecture — new code follows existing patterns

---

## 3. Architecture

### 3.1 Persistent Product Catalog

The system maintains a growing catalog of discovered products across all scans. Subsequent scans are incremental — only new/stale products consume API calls.

```
discovered_products
  ├── asin, title, brand_id, category, browse_node_id
  ├── estimated_price, buy_box_price, bsr_rank, seller_count
  ├── estimated_margin_pct, real_margin_pct, eligibility_status
  ├── data_quality (bitmask: 1=price, 2=bsr, 4=fees, 8=eligibility)
  ├── refresh_priority (computed: high margin + stale + competitive = refresh first)
  ├── source (pricelist | search | bestseller)
  ├── first_seen_at, last_seen_at, price_updated_at
  └── UNIQUE(tenant_id, asin)

price_history (partitioned by month)
  ├── asin, tenant_id, recorded_at
  ├── amazon_price, bsr_rank, seller_count, buy_box_price
  └── Auto-drop partitions older than 90 days

brands
  ├── id, name, normalized_name
  └── UNIQUE(normalized_name)

brand_eligibility
  ├── tenant_id, brand_id, category
  ├── status (unknown | eligible | restricted)
  ├── reason, sample_asin, checked_at
  └── UNIQUE(tenant_id, brand_id, category)

brand_intelligence (materialized view, refreshed every 6 hours)
  ├── tenant_id, brand_id, category
  ├── product_count, high_margin_count, avg_margin
  ├── avg_sellers, avg_bsr
  └── UNIQUE INDEX(tenant_id, brand_id, category)
```

### 3.2 The Tiered Funnel

Each tier is cheaper than the next. Processing stops as soon as a product is eliminated.

#### T0: Dedup / Cache Hit

- Check `discovered_products` — if ASIN was seen recently and data is fresh, skip API calls
- For price list uploads: UPC→ASIN mapping cached for 90 days
- **Cost:** $0, pure database lookups
- **Expected elimination:** ~60% of input (on repeat scans)

#### T1: Local Math

- For price list uploads: use REAL wholesale cost from CSV
- For category scans: estimate wholesale at 40% of Amazon price
- Calculate margin using deterministic FBA fee calculator
- Kill products below margin threshold (configurable, default 20%)
- Kill products with Amazon price outside viable range ($10-$200 default)
- **Cost:** $0, pure computation
- **Expected elimination:** ~70% of remaining

#### T2: Brand Gate

- Look up brand in `brand_eligibility` cache
- If cached and fresh (< 14 days): use cached result
- If uncached: queue for batch eligibility check (one sample ASIN per brand)
- Kill products from restricted brands
- Kill products from blocklisted brands (existing `brand_blocklist` table)
- **Cost:** ~500 SP-API calls per scan for newly discovered brands only
- **Expected elimination:** ~30-40% of remaining

#### T3: Competitive Pricing Enrichment

- Batch SP-API `getCompetitivePricing` (20 ASINs/call)
- Get real Buy Box price, seller count
- **Store both prices:** `estimated_price` (from catalog/CSV) is preserved for BI analysis; `buy_box_price` (from SP-API) is added alongside it
- Calculate `real_margin_pct` from competitive price; keep `estimated_margin_pct` from T1 for comparison
- Filtering decisions use `real_margin_pct` (competitive price), but both values persist in `discovered_products` for offline analysis (estimate accuracy, price drift, margin prediction quality)
- Kill: seller count < 3 (configurable), real margin below threshold
- **Cost:** ~200-400 SP-API calls per scan
- **Expected elimination:** ~50% of remaining

#### T4: LLM Pipeline

- Existing 5-agent pipeline: gating → profitability → demand+competition → supplier → reviewer
- Only the top ~150-200 candidates reach this tier
- Results become deals in the dashboard
- **Cost:** $0.05-0.25 per product, ~$7-37 per scan

### 3.3 Rate Limiter

New adapter behind a `port.RateLimiter` interface. Manages SP-API rate limits per endpoint.

```go
type RateLimiter interface {
    Wait(ctx context.Context, endpoint string) error
    ReportThrottle(endpoint string)
}
```

Implementation: adaptive token bucket using `golang.org/x/time/rate`. One limiter per SP-API endpoint. Starts at 80% of documented rate. On 429 responses, halves fill rate and recovers linearly. Tracks rolling 1-minute success rate.

### 3.4 Scan Types

#### Price List Scan (primary)

```
User uploads CSV → parse → match UPC/EAN to ASIN (batch 20) →
  for each matched product:
    T1 (real margin from CSV wholesale cost) →
    T2 (brand gate) →
    T3 (competitive pricing for real Amazon price) →
    T4 (LLM on survivors)
→ results as deals in campaign
```

Advantages over keyword search:
- Real wholesale cost (not estimated)
- 5K-200K products per upload
- Direct path from "distributor sent me a list" to "here are the winners"

#### Category Background Scan (secondary)

```
Nightly Inngest cron:
  Pick next N browse nodes from rotation queue →
  searchCatalogItems per node (up to 200 results each) →
  Insert/update discovered_products →
  Run T1-T3 on new/stale products →
  T4 candidates flagged for daily LLM batch
```

Not exhaustive — rotates through categories over a week. Builds the persistent catalog that makes subsequent price list scans cheaper (cached ASINs, cached brands).

#### On-Demand Campaign (kept, deprioritized)

Keyword search still works for ad-hoc exploration. Limited to 20 results by SP-API. Useful for quick checks, not for bulk discovery. Filters are relaxed (lower seller count threshold) to avoid the "0 results" problem.

---

## 4. Domain Model Changes

### New Types

```go
// ScanType distinguishes how products enter the pipeline
type ScanType string
const (
    ScanTypePriceList ScanType = "pricelist"
    ScanTypeCategory  ScanType = "category"
    ScanTypeKeyword   ScanType = "keyword"  // legacy
)

// DiscoveredProduct is a persistent catalog entry
type DiscoveredProduct struct {
    ID                 string
    TenantID           TenantID
    ASIN               string
    Title              string
    BrandID            string
    Category           string
    BrowseNodeID       string
    EstimatedPrice     float64 // from catalog search or CSV MSRP
    BuyBoxPrice   float64 // from SP-API competitive pricing (real Buy Box price)
    BSRRank            int
    SellerCount        int
    EstimatedMarginPct float64 // margin based on estimated price (T1)
    RealMarginPct      float64 // margin based on competitive price (T3), 0 if not yet enriched
    EligibilityStatus  string  // unknown, eligible, restricted
    DataQuality        int     // bitmask
    RefreshPriority    float64 // computed
    Source             ScanType
    FirstSeenAt        time.Time
    LastSeenAt         time.Time
    PriceUpdatedAt     *time.Time
}

// PriceSnapshot tracks price/rank changes over time
type PriceSnapshot struct {
    ASIN        string
    TenantID    TenantID
    RecordedAt  time.Time
    AmazonPrice float64
    BSRRank     int
    SellerCount int
}

// BrandIntelligence is the pre-computed brand-level view
type BrandIntelligence struct {
    TenantID        TenantID
    BrandID         string
    BrandName       string
    Category        string
    ProductCount    int
    HighMarginCount int
    AvgMargin       float64
    AvgSellers      float64
    AvgBSR          float64
    EligibleStatus  string
}

// ScanJob tracks a background scan's progress
type ScanJob struct {
    ID          string
    TenantID    TenantID
    Type        ScanType
    Status      string // pending, running, completed, failed
    TotalItems  int
    Processed   int
    Qualified   int
    Eliminated  int
    StartedAt   time.Time
    CompletedAt *time.Time
}
```

### Modified Types

```go
// Campaign — add ScanType field
type Campaign struct {
    // ... existing fields ...
    ScanType ScanType `json:"scan_type"`
}

// BrandEligibility — add category scope
type BrandEligibility struct {
    // ... existing fields ...
    Category string `json:"category"` // scope eligibility per category
}
```

---

## 5. New Interfaces (Ports)

```go
// DiscoveredProductRepo manages the persistent product catalog
type DiscoveredProductRepo interface {
    Upsert(ctx context.Context, product *DiscoveredProduct) error
    UpsertBatch(ctx context.Context, products []DiscoveredProduct) error
    GetByASIN(ctx context.Context, tenantID TenantID, asin string) (*DiscoveredProduct, error)
    GetByASINs(ctx context.Context, tenantID TenantID, asins []string) ([]DiscoveredProduct, error)
    ListStale(ctx context.Context, tenantID TenantID, olderThan time.Time, limit int) ([]DiscoveredProduct, error)
    ListByRefreshPriority(ctx context.Context, tenantID TenantID, limit int) ([]DiscoveredProduct, error)
    UpdatePricing(ctx context.Context, tenantID TenantID, asin string, price float64, sellers int, bsr int) error
}

// PriceHistoryRepo tracks price changes over time
type PriceHistoryRepo interface {
    Record(ctx context.Context, snapshot PriceSnapshot) error
    RecordBatch(ctx context.Context, snapshots []PriceSnapshot) error
    GetHistory(ctx context.Context, tenantID TenantID, asin string, since time.Time) ([]PriceSnapshot, error)
}

// BrowseNodeRepo manages category tree for scanning
type BrowseNodeRepo interface {
    Upsert(ctx context.Context, node BrowseNode) error
    GetNextForScan(ctx context.Context, limit int) ([]BrowseNode, error)
    MarkScanned(ctx context.Context, nodeID string, productsFound int) error
}

// RateLimiter manages SP-API rate limits
type RateLimiter interface {
    Wait(ctx context.Context, endpoint string) error
    ReportThrottle(endpoint string)
}
```

---

## 6. New Services

### CatalogService

Manages the persistent product catalog. Central point for all product data writes.

```go
type CatalogService struct {
    products  DiscoveredProductRepo
    prices    PriceHistoryRepo
    brands    BrandRepo
    idGen     IDGenerator
}

// UpsertProducts inserts or updates products from any source
func (s *CatalogService) UpsertProducts(ctx, tenantID, products []DiscoveredProduct) error

// RefreshPricing fetches current pricing for stale products
func (s *CatalogService) RefreshPricing(ctx, tenantID, asins []string) error
```

### FunnelService

Runs the tiered elimination funnel on a batch of products.

```go
type FunnelService struct {
    catalog          *CatalogService
    brandEligibility *BrandEligibilityService
    brandBlocklist   *BrandBlocklistService
    spapi            ProductSearcher
    rateLimiter      RateLimiter
}

// ProcessBatch runs T0-T3 on a batch of products, returns survivors for T4
func (s *FunnelService) ProcessBatch(ctx, tenantID, products []FunnelInput, thresholds PipelineThresholds) ([]FunnelSurvivor, FunnelStats, error)
```

### CategoryScanService

Orchestrates nightly category scanning.

```go
type CategoryScanService struct {
    nodes    BrowseNodeRepo
    catalog  *CatalogService
    funnel   *FunnelService
    spapi    ProductSearcher
}

// ScanNextBatch picks browse nodes from the rotation queue and scans them
func (s *CategoryScanService) ScanNextBatch(ctx, tenantID, maxNodes int) (*ScanJob, error)
```

### Enhanced PriceListScanner

Extend the existing scanner to use the tiered funnel and persistent catalog.

```go
// ProcessPriceList now uses the funnel and updates the persistent catalog
func (s *PriceListScanner) ProcessPriceList(ctx, tenantID, items []PriceListItem, thresholds PipelineThresholds) ([]PriceListMatch, error)
```

---

## 7. Database Migration

```sql
-- Discovered products catalog
CREATE TABLE discovered_products (
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

CREATE INDEX idx_dp_tenant_brand ON discovered_products(tenant_id, brand_id);
CREATE INDEX idx_dp_tenant_category ON discovered_products(tenant_id, category);
CREATE INDEX idx_dp_tenant_refresh ON discovered_products(tenant_id, refresh_priority DESC)
    WHERE data_quality < 15;
CREATE INDEX idx_dp_tenant_margin ON discovered_products(tenant_id, estimated_margin_pct DESC);
CREATE INDEX idx_dp_browse_node ON discovered_products(browse_node_id);

-- Price history (partitioned by month)
CREATE TABLE price_history (
    id UUID DEFAULT gen_random_uuid(),
    asin TEXT NOT NULL,
    tenant_id UUID NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    amazon_price NUMERIC(10,2),
    bsr_rank INT,
    seller_count INT
) PARTITION BY RANGE (recorded_at);

CREATE TABLE price_history_2026_04 PARTITION OF price_history
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE price_history_2026_05 PARTITION OF price_history
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE INDEX idx_ph_asin_time ON price_history(tenant_id, asin, recorded_at DESC);

-- Browse nodes for category scanning
CREATE TABLE browse_nodes (
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

CREATE INDEX idx_bn_scan ON browse_nodes(last_scanned_at NULLS FIRST)
    WHERE is_leaf = true;

-- Extend brand_eligibility to scope by category
ALTER TABLE brand_eligibility ADD COLUMN IF NOT EXISTS
    category TEXT NOT NULL DEFAULT '';
-- Drop old unique constraint and create new one
-- (migration handles this carefully)

-- Materialized view for brand intelligence
CREATE MATERIALIZED VIEW brand_intelligence AS
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

CREATE UNIQUE INDEX ON brand_intelligence(tenant_id, brand_id, category);

-- Scan job tracking
CREATE TABLE scan_jobs (
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

CREATE INDEX idx_sj_tenant_status ON scan_jobs(tenant_id, status);
```

---

## 8. Inngest Workflows

### Price List Processing (enhanced)

```
pricelist/uploaded
  → step: parse-csv (extract items)
  → step: match-identifiers (batch UPC→ASIN, 500-ASIN chunks)
  → step: run-funnel (T0-T3 per batch, fan-out)
  → step: collect-survivors (aggregate qualified products)
  → step: llm-evaluate (fan-out per candidate, existing pipeline)
  → step: complete-scan (update campaign, notify)
```

### Category Background Scan (new)

```
scan/nightly (Inngest cron, e.g. 2:00 AM UTC)
  → step: pick-nodes (next N browse nodes from rotation)
  → step: search-nodes (fan-out per node, 200 products each)
  → step: run-funnel (T0-T3 on new/stale products)
  → step: llm-batch (top candidates get LLM evaluation)
  → step: complete-scan (update stats, refresh materialized view)
```

### Catalog Refresh (new)

```
catalog/refresh (Inngest cron, e.g. 6:00 AM UTC)
  → step: select-stale (top N products by refresh_priority)
  → step: batch-pricing (competitive pricing in 20-ASIN batches)
  → step: update-catalog (write new prices, record price_history)
  → step: recompute-priority (update refresh_priority scores)
  → step: refresh-views (REFRESH MATERIALIZED VIEW CONCURRENTLY)
```

---

## 9. API Changes

```
# Price list (enhanced — now the primary flow)
POST   /pricelists/upload            Upload distributor CSV
GET    /pricelists/:id/status        Processing progress
GET    /pricelists/:id/results       Matched + qualified results

# Catalog (new)
GET    /catalog/products             Browse persistent catalog with filters
GET    /catalog/brands               Brand intelligence view
GET    /catalog/brands/:id/products  Products for a specific brand
GET    /catalog/stats                Catalog size, freshness, scan history

# Scans (new)
GET    /scans                        List scan jobs (pricelist, category, refresh)
GET    /scans/:id                    Scan detail with funnel stats
POST   /scans/category               Trigger manual category scan

# Existing campaign endpoint — add scan_type
POST   /campaigns                    Create campaign (now supports scan_type field)
```

---

## 10. Frontend Changes

### Price List Upload (promoted to primary)

Currently a side feature. Becomes the main action on the dashboard:
- Prominent "Upload Price List" button
- Drag-and-drop CSV upload
- Progress bar with funnel stats (matched → margin filter → brand gate → enriched → LLM → deals)
- Results table with real margins and qualification status

### Brand Intelligence Page (new)

```
/brands
  Table: brand name, category, product count, avg margin, eligible status, last scanned
  Filters: category, eligibility, min margin, min products
  Click → brand detail with product list

/brands/:id
  Brand summary card
  Products table with margins, BSR, seller count
  Eligibility status per category
  Related distributors (future)
```

### Catalog Explorer Page (new)

```
/catalog
  Product table with search, sort, filter
  Columns: ASIN, title, brand, category, price, margin, BSR, sellers, eligibility, last updated
  Filters: category, margin range, seller count, eligibility, data freshness
  Bulk actions: "Evaluate selected" (send to LLM pipeline)
```

### Scan History (new section in Discovery page)

- List of scan jobs with type, status, items processed, qualified count
- Funnel visualization: how many products eliminated at each tier
- Scheduled scans configuration (category rotation, refresh cadence)

---

## 11. Implementation Phases

| Phase | What | Delivers |
|-------|------|----------|
| **A** | Persistent catalog + migrations + repos | `discovered_products`, `price_history`, `browse_nodes` tables, repos, catalog service |
| **B** | Tiered funnel service + rate limiter | T0-T3 processing, adaptive SP-API rate limiting |
| **C** | Enhanced price list scanner | Price list upload feeds funnel, writes to persistent catalog, triggers LLM on survivors |
| **D** | Category background scan | Browse node rotation, Inngest nightly cron, incremental catalog building |
| **E** | Catalog refresh + brand intelligence | Priority-based refresh, materialized views, brand intelligence queries |
| **F** | Frontend — price list promotion + catalog + brands | Upload UX, catalog explorer, brand intelligence page, scan history |

---

## 12. What's NOT in This Spec

- Keepa integration (documented as future enhancement in expert analysis)
- Multi-seller-account rate limit pooling
- Reverse sourcing from distributor catalogs (depends on brand intelligence being built first)
- Real-time price monitoring / alerts (Phase 5 of the main roadmap)
- Autoresearch experiments on funnel thresholds (Phase 6)
- Multi-marketplace support (EU, UK) — same architecture, different marketplace IDs

---

## 13. Success Metrics

| Metric | Current | Target |
|--------|:------:|:-----:|
| Products scanned per campaign | 20 | 10K-200K (price list dependent) |
| Qualified candidates reaching LLM | 0 | 50-200 per scan |
| Deals generated per scan | 0 | 10-50 |
| Cost per scan (API + LLM) | ~$0 (nothing works) | $7-40 |
| Time to first deal (price list upload) | N/A | < 30 minutes |
| Brand cache hit rate (after warm-up) | 0% | > 80% |
| Repeat scan efficiency (same distributor) | N/A | 90% fewer API calls |

---

## 14. Open Questions

1. **Default margin threshold for price lists vs category scans** — price lists have real wholesale cost (higher confidence), category scans use estimated 40% wholesale (lower confidence). Should thresholds differ?
2. **Multi-tenant catalog sharing** — brand eligibility and BSR data are tenant-agnostic. Should we share a global catalog and scope only margin/eligibility per tenant?
3. **Price list format standardization** — distributors use wildly different CSV formats. How much flexibility does the parser need? Column auto-detection?
4. **Scan budget limits** — should there be a per-tenant daily cap on API calls to prevent one heavy user from exhausting shared rate limits?
5. **LLM candidate selection** — when the funnel produces 2K survivors but we only want to LLM-evaluate 150, how do we pick the best 150? Sort by margin? BSR? Composite score?
