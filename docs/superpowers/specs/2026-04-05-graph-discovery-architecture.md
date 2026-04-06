# Product Discovery Architecture — Expert Review Synthesis

**Date:** 2026-04-05
**Status:** Approved — proceeding with implementation
**Reviewers:** Database Architect, Cost/Performance Analyst, FBA Operations Expert

---

## Decision: PostgreSQL, not Neo4j

Unanimous across all three experts. The data model is relational (brand → products, category → subcategory). Neo4j adds $40-1,800/month in cost, a second database to manage, data sync complexity, and JVM memory overhead — with no performance benefit for our access patterns.

PostgreSQL handles all 7 access patterns well:
- Product lookups by brand/category: indexed JOINs (<100ms)
- Category hierarchy: `ltree` extension
- Bulk writes (50K products/night): `COPY` command (3-10s)
- Brand eligibility sets: anti-joins on indexed columns
- Hot query (eligible products by margin): materialized view

---

## Revised Strategy (incorporating FBA expert feedback)

The FBA expert challenged the core approach: "The system is built around 'scan Amazon, find products' when the real workflow is 'I have distributor price lists, help me pick winners.'"

### Revised priority order:

| Phase | What | Why | Time |
|-------|------|-----|------|
| **0** | Brand eligibility caching | 93% fewer API calls, works within current architecture | 1 week |
| **1** | Distributor price list scanner | The killer feature resellers actually pay for | 2-3 weeks |
| **2** | Eligible product catalog (nightly scan) | Background discovery, builds persistent intelligence | 2 weeks |
| **3** | Distributor intelligence | Brand → distributor mapping, highest-value data | Ongoing |

---

## Phase 0: Brand Eligibility Caching (immediate)

### Problem
Currently we check listing eligibility per ASIN. Most brands have 10-100 products. If Brand X is restricted, checking 50 Brand X ASINs wastes 49 API calls.

### Solution
Add a `brands` table that caches eligibility status per brand per tenant.

```sql
CREATE TABLE brands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    normalized_name TEXT NOT NULL,  -- lowercase, trimmed
    UNIQUE(normalized_name)
);

CREATE TABLE brand_eligibility (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    brand_id UUID NOT NULL REFERENCES brands(id),
    status TEXT NOT NULL DEFAULT 'unknown',  -- unknown, eligible, restricted
    reason TEXT NOT NULL DEFAULT '',
    sample_asin TEXT NOT NULL DEFAULT '',    -- ASIN used to check
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, brand_id)
);

CREATE INDEX idx_brand_eligibility_tenant ON brand_eligibility(tenant_id);
CREATE INDEX idx_brand_eligibility_status ON brand_eligibility(tenant_id, status);
```

### How it works in the pipeline

```
Product found with brand="Scotch"
  → Look up brand_eligibility for this tenant
  → If status="restricted" and checked < 7 days ago → SKIP (no API call)
  → If status="eligible" and checked < 7 days ago → PASS (no API call)
  → If status="unknown" or stale → check ONE ASIN via SP-API restrictions
    → Cache result for entire brand
    → All future Scotch products use cached result
```

### Expected impact
- First scan of 50K products: ~3,300 brand checks instead of 50,000 ASIN checks (**93% reduction**)
- Subsequent scans: ~500 new brand checks per night (**99% reduction**)
- Stale data mitigation: re-check brands older than 7 days

---

## Phase 1: Distributor Price List Scanner (the real product)

### What resellers actually do
1. Get a price list from a distributor (CSV: UPC, wholesale cost, product name)
2. Match UPCs to Amazon ASINs
3. Calculate real margin (wholesale cost vs Amazon price minus FBA fees)
4. Check eligibility
5. Decide what to order

### What we build
```
Upload CSV → match UPC/EAN to ASIN (SP-API) → get Amazon price →
calculate REAL margin (actual wholesale cost, not estimated) →
check brand eligibility (cached) → ASIN-level verify (for top candidates) →
AI evaluation (only for pre-qualified, real-margin products)
```

This is the feature SellerAmp ($17/mo) and Tactical Arbitrage ($65/mo) provide — but without AI evaluation. Adding the AI layer on top of real data is the differentiator.

### Key insight from FBA expert
> "A tool that can process a 12,000-SKU price list in minutes with AI-powered evaluation — not just margin calc but competitive assessment, IP risk, and demand analysis — that's worth $200/month."

---

## Phase 2: Nightly Category Scanning

Secondary to price list scanning. Runs in background, builds persistent catalog.

```
Nightly Inngest cron:
  → Scan N categories (round-robin through Amazon browse tree)
  → Extract products with 3+ sellers
  → Check brand eligibility (cached — most brands already known)
  → Calculate estimated margins
  → Store in eligible_products table
  → Dashboard shows "N new opportunities discovered"
```

### Tables (Postgres)

```sql
-- Products catalog (persistent, grows over time)
CREATE TABLE discovered_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    asin TEXT NOT NULL,
    title TEXT NOT NULL,
    brand_id UUID REFERENCES brands(id),
    category TEXT NOT NULL DEFAULT '',
    amazon_price NUMERIC(10,2),
    bsr_rank INT,
    seller_count INT,
    estimated_margin_pct NUMERIC(5,2),
    eligibility_status TEXT NOT NULL DEFAULT 'unknown',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    price_updated_at TIMESTAMPTZ,
    UNIQUE(tenant_id, asin)
);

CREATE INDEX idx_discovered_tenant_eligible ON discovered_products(tenant_id, eligibility_status);
CREATE INDEX idx_discovered_margin ON discovered_products(tenant_id, estimated_margin_pct DESC);
CREATE INDEX idx_discovered_bsr ON discovered_products(tenant_id, bsr_rank);

-- Category scan tracking
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    amazon_browse_node_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    path ltree,
    last_scanned_at TIMESTAMPTZ,
    products_found INT DEFAULT 0,
    products_eligible INT DEFAULT 0
);

CREATE INDEX idx_categories_path ON categories USING GIST(path);

-- Materialized view for the dashboard "eligible products" query
CREATE MATERIALIZED VIEW eligible_product_summary AS
SELECT dp.*, b.name as brand_name, be.status as brand_status
FROM discovered_products dp
JOIN brands b ON dp.brand_id = b.id
LEFT JOIN brand_eligibility be ON b.id = be.brand_id AND be.tenant_id = dp.tenant_id
WHERE dp.eligibility_status = 'eligible'
  AND dp.seller_count >= 3
  AND dp.estimated_margin_pct >= 10
ORDER BY dp.estimated_margin_pct DESC;
```

---

## Performance & Cost Summary

| Metric | Current | Phase 0 (brand cache) | Phase 1+2 (full) |
|--------|---------|----------------------|-------------------|
| Products discovered/day | 200 | 200 (same flow) | 50,000 |
| API calls/campaign | ~22 | ~8 (brand-cached) | N/A (nightly scan) |
| LLM cost/qualified deal | $1.20 | $1.20 (same) | $0.20 |
| LLM waste rate | 40-60% | 20-30% | ~0% |
| Time to first deal | 10-30 min | 5-15 min | 1-4 min |
| Monthly infra | $0-25 | $0-25 (same DB) | $35-55 |
| Break-even (single) | — | Immediate (no cost) | ~28 months |
| Break-even (10 tenants) | — | Immediate | **5.7 months** |

---

## Key Risks & Mitigations

| Risk | Source | Mitigation |
|------|--------|------------|
| Brand eligibility has exceptions (category gating, ASIN-level blocks) | FBA expert | ASIN-level recheck at point of decision; brand-level only for pre-filtering |
| Estimated margins unreliable without real wholesale costs | FBA expert | Phase 1 (price list scanner) provides real costs; label estimates clearly |
| 50K products creates decision paralysis | FBA expert | Smart ranking + cap at top 50; AI evaluation on demand |
| Brand eligibility becomes stale | FBA expert | 7-day re-check cycle; flag stale data in UI |
| Competes with SellerAmp ($17) on price list scanning | FBA expert | Differentiate with AI evaluation layer, not just margin calc |

---

## What NOT to build

- Neo4j or any separate graph database
- "Competes with" relationship tracking (moderate value, high complexity)
- Category browsing as primary UI (wrong mental model for wholesale)
- Real-time eligibility for all 50K products (batch is fine, real-time at point of decision)
