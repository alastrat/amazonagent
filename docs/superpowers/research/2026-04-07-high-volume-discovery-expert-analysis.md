# High-Volume Discovery Engine — Expert Analysis

**Date:** 2026-04-07
**Status:** Research complete — informing spec
**Experts consulted:** FBA Wholesale Strategist, Systems Architect, Data Engineer, SP-API Specialist

---

## 1. Problem Statement

The current discovery pipeline processes ~20 products per campaign via keyword-based SP-API catalog search. Products are filtered by seller count (>=3), estimated margin (>=15%), and brand eligibility. In practice, all 20 products are eliminated by pre-qualification filters, resulting in zero deals.

The system needs to scale to 100K+ products while being strategic about API calls and LLM costs.

---

## 2. Core Finding: Wrong Entry Point

All four experts independently reached the same conclusion:

> **"The system is built around 'scan Amazon, find products' when the real workflow is 'I have distributor price lists, help me pick winners.'"**

Successful wholesale resellers don't search Amazon for products. They work backwards from supply:

1. Get authorized with distributors (Kehe, UNFI, McLane, etc.)
2. Receive price lists — 5K to 200K SKUs per distributor, updated weekly/monthly
3. Scan those lists against Amazon — match UPC to ASIN, calculate real margin
4. Filter survivors by brand quality — BSR, competition, gating, Buy Box stability
5. Deep-dive the top 1-2% — historical trends, sentiment, seasonal patterns

The keyword-search approach is an arbitrage workflow bolted onto a wholesale tool. The price list scanner (already built) should be the primary pipeline, not a side feature.

---

## 3. SP-API Rate Limits — Practical Reality

### Documented vs Actual Rates

| Endpoint | Documented | Practical Sustained | Unit |
|----------|:---:|:---:|------|
| Catalog Items (search) | 2 req/sec | 1.5-2 req/sec | Per selling partner |
| Catalog Items (getItem) | 2 req/sec | 1.5 req/sec | Per selling partner |
| Competitive Pricing | 10 req/sec | 6-8 req/sec | Per selling partner |
| Listing Restrictions | 5 req/sec | 3-5 req/sec | Per selling partner |
| Product Fees | 10 req/sec | 8 req/sec | Per selling partner |
| Browse Tree | 1 req/sec | 0.8 req/sec | Per selling partner |

### Critical Constraints

- **Rate limits are per-selling-partner credentials**, not per-application. Multi-tenant SaaS sharing one seller account shares one quota.
- **Catalog search is capped at 200 items per query** (10 pages × 20 results). No way to enumerate entire categories via search alone.
- **Undocumented daily quota:** ~50,000-100,000 catalog search calls/day depending on usage tier. Exceeding this produces 429s that don't clear with normal backoff — must wait until midnight UTC reset.
- **Throttling behavior:** HTTP 429 with `x-amzn-RateLimit-Limit` header. No Retry-After header. Requires exponential backoff with jitter.

### Throughput Calculations

| Operation | Calls Needed (100K products) | Rate | Wall Time |
|-----------|---------------------------:|-----:|---------:|
| Catalog enrichment (batch 20) | 5,000 | 2/sec | 42 min |
| Competitive pricing (batch 20) | 5,000 | 7/sec | 12 min |
| Listing restrictions (1 per ASIN) | 100,000 | 4/sec | 6.9 hours |
| With brand cache (80% hit) | 20,000 | 4/sec | 1.4 hours |

**Listing restrictions is the bottleneck without caching.** Brand-level caching reduces it from 6.9 hours to 1.4 hours.

### `includedData` Optimization

Using `includedData=summaries,salesRanks,dimensions,identifiers` in one catalog call provides:
- `summaries`: title, brand, category, image
- `salesRanks`: BSR rank + category
- `dimensions`: weight + dimensions (needed for FBA fee calc)
- `identifiers`: UPC/EAN for supplier matching

**What requires separate calls:**
- Actual seller price (Buy Box) — requires Competitive Pricing
- Seller count — requires Competitive Pricing
- Listing eligibility — requires Listing Restrictions (1 ASIN/call)
- FBA fees — deterministic calculator handles this, no API needed

---

## 4. The Elimination Funnel — Cost Model

### Five-Tier Processing

| Tier | Action | Cost/Product | Survivors | Cumulative API Calls (100K input) |
|:----:|--------|:-----------:|:---------:|----------------------------------:|
| T0 | Cache hit — skip ASINs seen recently | $0 | ~40K new | 0 |
| T1 | Local math — cached price × estimated margin, kill < 20% | $0 | ~8-12K | 0 |
| T2 | Brand gate — cached brand eligibility (7-14 day TTL) | ~$0 (amortized) | ~5-8K | ~500 (new brands only) |
| T3 | Competitive pricing — batch 20 ASINs/call, real price + seller count | ~$0.0001 | ~1.5-2K | ~400 batch calls |
| T4 | LLM pipeline — 5 agent calls per survivor | $0.05-0.25 | 50-100 deals | 5 LLM calls each |

### Monthly Cost Projection (100K products/night, 30 days)

| Item | Monthly Cost |
|------|:-----------:|
| SP-API calls (free tier) | $0 |
| LLM agents (~150/night × $0.15 avg × 30) | ~$675 |
| Exa/Firecrawl (LLM tier only) | ~$200-400 |
| Supabase Postgres (Pro) | ~$75 |
| Inngest step executions | ~$50 |
| **Total** | **~$1,000-1,200/mo** |

At 500K/night, LLM cost stays flat (same top-200 output). API volume scales but SP-API is free. Main increase is compute/Postgres IOPS: ~$1,500-1,800/mo.

---

## 5. Brand-Level Intelligence

### Why Brands, Not Products

- **Brand-to-product ratio:** 1:15-1:50 for wholesale catalogs
- **Brand eligibility is mostly binary:** ~85-90% of brands are either fully gated or fully open
- **Exceptions (~5-10%):** Category gating (brand open in Beauty but restricted in Grocery), ASIN-level blocks (IP complaints on specific ASINs)
- **Recommendation:** Cache at `(brand, category)` pair, not just brand. Re-check on 14-day cadence.

### Brand Score (composite intelligence)

Over time, accumulate per-brand:
- Average margin across ASINs
- Gating status per category
- Average BSR and competition density
- MAP enforcement reputation
- Distributor availability (which distributors carry this brand)
- Last scanned date and ASIN count on Amazon

When a new price list arrives, instantly skip every brand flagged as gated, low-margin, or high-competition. Second scan of the same distributor drops from 100K lookups to ~5K (new SKUs + stale brand re-checks).

---

## 6. Data Acquisition Strategy

### Primary: Distributor Price Lists (current priority)

Best product-per-cost ratio. Real wholesale costs. Direct path to deals.

Flow: Upload CSV → match UPC/EAN to ASIN (batch 20/call) → get competitive pricing (batch 20/call) → deterministic FBA fee calc → brand eligibility check (cached) → LLM evaluation on survivors

### Secondary: Category Scanning (background enrichment)

SP-API catalog search by browse node: ~20 items/call, paginate up to 200 items per node. Kitchen & Dining has ~2,000-3,000 leaf browse nodes.

**Cold start estimate for one major category (~500K ASINs):**
- Browse tree crawl: ~2,500 leaf nodes, ~2,500 API calls, ~1 hour
- Catalog search per node: ~125K calls over multiple nights
- Pricing enrichment: 25K batch calls, ~1 hour
- Brand eligibility: ~5K brands, ~5K calls, ~25 min
- **Total: 4-6 nights of incremental scanning**

**Incremental strategy:** Don't re-scan all nodes nightly. Rotate through categories weekly (scan ~700 nodes/night). Refresh pricing for high-priority products daily, rest weekly.

### Tertiary: Reverse Sourcing

"Show me which distributors carry brands I already know are profitable." Requires mapping brand → distributor relationships. Extremely high-value for retention but depends on having brand intelligence and distributor catalog data.

---

## 7. Caching Strategy

| Data | Cache Duration | Reason |
|------|:-----------:|--------|
| UPC → ASIN mapping | 90 days | Rarely changes |
| FBA fee calculation | 30 days | Fee schedules change ~2x/year |
| Brand eligibility | 7-14 days | Changes rarely, expensive to check |
| BSR / seller count | 24-48 hours | Changes frequently, but tier (top 1% vs bottom 50%) is stable |
| Buy Box price | 6-12 hours | Volatile, but precision only needed for final candidates |
| Product title/brand/category | 30 days | Very stable |

---

## 8. Future Enhancement: Keepa Integration

**NOT in current scope — documented for future planning.**

### What Keepa Provides

Keepa API returns historical price, BSR, seller count, and Buy Box data at 100 ASINs per API token (~$0.01-0.04 per token).

### How It Would Improve the Pipeline

| Current (SP-API only) | With Keepa |
|----------------------|-----------|
| BSR is a point-in-time snapshot | 90-day BSR trend → filter out seasonal spikes |
| Price is current only | 30-day price range → filter out race-to-bottom products |
| No out-of-stock history | OOS rate → filter unreliable supply |
| Must call SP-API for every refresh | Keepa historical data reduces SP-API refresh calls by 60-70% |

### Cost Impact

| Metric | Current | With Keepa |
|--------|:------:|:--------:|
| SP-API calls for BSR/price refresh | ~5K/night | ~1.5K/night |
| Keepa cost | $0 | $50-150/month |
| Data quality for filtering | Point-in-time | 90-day historical |
| False positive rate (products that look good but aren't) | ~30-40% | ~10-15% |

### Integration Point

Keepa would slot in between T3 (competitive pricing) and T5 (LLM pipeline) as a T4 historical analysis filter. The ~2K survivors from T3 would be batch-checked via Keepa (20 Keepa tokens for 2K ASINs = ~$0.40-0.80/night) to eliminate volatile/seasonal/unreliable products before expensive LLM evaluation.

### Estimated Monthly Savings

LLM tier sees 300-500 products instead of 1,500-2,000. At $0.15/product average: saves ~$150-225/month in LLM costs, partially offsetting the $50-150/month Keepa cost. Net effect: better deal quality at similar cost.

---

## 9. Infrastructure Decisions

| Concern | Recommendation | Rationale |
|---------|---------------|-----------|
| Rate limiting | Adaptive token bucket per SP-API endpoint | `golang.org/x/time/rate`, start at 80% of documented rate, back off on 429s |
| Batch processing | Inngest fan-out, 500-ASIN chunks, checkpoint in Postgres | Survives restarts, idempotent retries |
| Catalog storage | Postgres with `discovered_products` + `price_history` (partitioned monthly) | Good enough to 1M+ rows, familiar stack |
| Brand intelligence | Materialized view refreshed every 6 hours | Pre-computed aggregates for dashboard queries |
| Refresh scheduling | Priority-based (high margin + stale + competitive = refresh first) | Maximizes value per API call |

---

## 10. Expert Disagreements

### Catalog search daily quota
- Systems Architect estimated 50K-100K calls/day undocumented quota
- SP-API Specialist confirmed the cap but noted it varies by usage tier
- **Resolution:** Budget 40K search calls/day for scanning, reserve 10K for on-demand user queries

### Listing restrictions throughput
- Systems Architect calculated 18K ASINs/hour (5 req/sec)
- SP-API Specialist estimated 12.6K-18K/hour (3.5-5 req/sec)
- **Resolution:** Plan for 3.5 req/sec sustained as conservative baseline

### Browse node scanning depth
- Data Engineer estimated 1,000 results per node via pagination
- SP-API Specialist corrected to 200 items max (10 pages)
- **Resolution:** Use the 200-item cap; deeper coverage requires drilling into child nodes

### Brand eligibility caching scope
- FBA Strategist recommended brand-level caching
- SP-API Specialist noted 5-10% of brands have mixed eligibility per category
- **Resolution:** Cache at (brand, category) pair, do per-ASIN recheck on final candidates only
