# Research Strategy: Current vs Proposed

## Current Flow (keyword-based)

```
User                    ToolResolver         SP-API            Exa              Sourcing Agent      Pipeline
 │                          │                  │                │                    │                │
 │  "stainless steel        │                  │                │                    │                │
 │   water bottle"          │                  │                │                    │                │
 ├─────────────────────────►│                  │                │                    │                │
 │                          │                  │                │                    │                │
 │                          │  keyword search  │                │                    │                │
 │                          │  "stainless      │                │                    │                │
 │                          │   steel water    │                │                    │                │
 │                          │   bottle"        │                │                    │                │
 │                          ├─────────────────►│                │                    │                │
 │                          │                  │                │                    │                │
 │                          │  20 random       │                │                    │                │
 │                          │  products        │                │                    │                │
 │                          │  (no BSR sort,   │                │                    │                │
 │                          │   no category    │                │                    │                │
 │                          │   analysis)      │                │                    │                │
 │                          │◄─────────────────┤                │                    │                │
 │                          │                  │                │                    │                │
 │                          │  generic search: │                │                    │                │
 │                          │  "...wholesale   │                │                    │                │
 │                          │   FBA"           │                │                    │                │
 │                          ├─────────────────────────────────►│                    │                │
 │                          │                  │                │                    │                │
 │                          │  5 generic       │                │                    │                │
 │                          │  web results     │                │                    │                │
 │                          │  (mostly noise)  │                │                    │                │
 │                          │◄─────────────────────────────────┤                    │                │
 │                          │                  │                │                    │                │
 │                          │  {20 products +  │                │                    │                │
 │                          │   5 web results} │                │                    │                │
 │                          ├──────────────────────────────────────────────────────►│                │
 │                          │                  │                │                    │                │
 │                          │                  │                │  "pick best 5-10"  │                │
 │                          │                  │                │  (no criteria for  │                │
 │                          │                  │                │   what "best" means│                │
 │                          │                  │                │   beyond the prompt│                │
 │                          │                  │                │                    │                │
 │                          │                  │                │    5 candidates    │                │
 │                          │◄──────────────────────────────────────────────────────┤                │
 │                          │                  │                │                    │                │
 │                          │                  │                │                    │  evaluate      │
 │                          ├────────────────────────────────────────────────────────────────────────►│
 │                          │                  │                │                    │                │
 │                          │                  │                │                    │ mostly private │
 │                          │                  │                │                    │ label or       │
 │                          │                  │                │                    │ restricted     │
 │                          │                  │                │                    │ brands         │
 │                          │                  │                │                    │                │
 │  0-2 deals               │                  │                │                    │  0-2 deals     │
 │  (often 0)               │                  │                │                    │                │
 │◄────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

**Problems:**
- Keyword search returns random mix of private label + wholesale products
- No BSR analysis (ceiling/floor) — doesn't find the profitable middle ground
- No category exploration — stuck in whatever keywords user types
- Exa results are generic, not actionable
- Sourcing agent has no strategy — just picks from what it's given
- Most results get eliminated at pre-gate (1-2 sellers = private label)

---

## Proposed Flow (category + BSR strategy)

```
User                    ToolResolver         SP-API            Exa              Sourcing Agent      Pipeline
 │                          │                  │                │                    │                │
 │  "kitchen"               │                  │                │                    │                │
 │  (or auto-discovery)     │                  │                │                    │                │
 ├─────────────────────────►│                  │                │                    │                │
 │                          │                  │                │                    │                │
 │                          │  ┌─────────────────────────────────────────────────┐   │                │
 │                          │  │ PHASE 1: Category Intelligence                 │   │                │
 │                          │  └─────────────────────────────────────────────────┘   │                │
 │                          │                  │                │                    │                │
 │                          │  browse category │                │                    │                │
 │                          │  "Kitchen &      │                │                    │                │
 │                          │   Dining"        │                │                    │                │
 │                          │  sorted by BSR   │                │                    │                │
 │                          ├─────────────────►│                │                    │                │
 │                          │                  │                │                    │                │
 │                          │  top 100 by BSR  │                │                    │                │
 │                          │  (with prices +  │                │                    │                │
 │                          │   seller counts) │                │                    │                │
 │                          │◄─────────────────┤                │                    │                │
 │                          │                  │                │                    │                │
 │                          │  ┌─────────────────────────────────────────────────┐   │                │
 │                          │  │ PHASE 2: Ceiling/Floor Analysis (deterministic) │   │                │
 │                          │  └─────────────────────────────────────────────────┘   │                │
 │                          │                  │                │                    │                │
 │                          │  Find ceiling:   │                │                    │                │
 │                          │    BSR #1 = 8000 units/mo         │                    │                │
 │                          │  Find floor:     │                │                    │                │
 │                          │    BSR drops below 500 units/mo   │                    │                │
 │                          │  Sweet spot:     │                │                    │                │
 │                          │    BSR #10-#50   │                │                    │                │
 │                          │    (1000-5000    │                │                    │                │
 │                          │     units/mo)    │                │                    │                │
 │                          │                  │                │                    │                │
 │                          │  ┌─────────────────────────────────────────────────┐   │                │
 │                          │  │ PHASE 3: Wholesale Filter (deterministic)       │   │                │
 │                          │  └─────────────────────────────────────────────────┘   │                │
 │                          │                  │                │                    │                │
 │                          │  competitive     │                │                    │                │
 │                          │  pricing for     │                │                    │                │
 │                          │  sweet spot      │                │                    │                │
 │                          │  products        │                │                    │                │
 │                          ├─────────────────►│                │                    │                │
 │                          │                  │                │                    │                │
 │                          │  prices +        │                │                    │                │
 │                          │  seller counts   │                │                    │                │
 │                          │◄─────────────────┤                │                    │                │
 │                          │                  │                │                    │                │
 │                          │  Filter:         │                │                    │                │
 │                          │  ✗ seller_count < 3 (private label)                   │                │
 │                          │  ✗ blocked brands                 │                    │                │
 │                          │  ✗ margin < threshold             │                    │                │
 │                          │  ✓ 15 products pass               │                    │                │
 │                          │                  │                │                    │                │
 │                          │  ┌─────────────────────────────────────────────────┐   │                │
 │                          │  │ PHASE 4: Market Intelligence                    │   │                │
 │                          │  └─────────────────────────────────────────────────┘   │                │
 │                          │                  │                │                    │                │
 │                          │  per product:    │                │                    │                │
 │                          │  "Lodge cast iron│                │                    │                │
 │                          │   skillet        │                │                    │                │
 │                          │   wholesale      │                │                    │                │
 │                          │   distributor"   │                │                    │                │
 │                          ├─────────────────────────────────►│                    │                │
 │                          │                  │                │                    │                │
 │                          │  targeted        │                │                    │                │
 │                          │  supplier info   │                │                    │                │
 │                          │  + trend data    │                │                    │                │
 │                          │◄─────────────────────────────────┤                    │                │
 │                          │                  │                │                    │                │
 │                          │  ┌─────────────────────────────────────────────────┐   │                │
 │                          │  │ PHASE 5: AI Analysis (LLM via OpenFang)         │   │                │
 │                          │  └─────────────────────────────────────────────────┘   │                │
 │                          │                  │                │                    │                │
 │                          │  15 pre-vetted   │                │                    │                │
 │                          │  candidates with │                │                    │                │
 │                          │  real data:      │                │                    │                │
 │                          │  - BSR + trend   │                │                    │                │
 │                          │  - price         │                │                    │                │
 │                          │  - seller count  │                │                    │                │
 │                          │  - FBA fees      │                │                    │                │
 │                          │  - margin calc   │                │                    │                │
 │                          │  - supplier leads │               │                    │                │
 │                          │  - brand info    │                │                    │                │
 │                          ├──────────────────────────────────────────────────────►│                │
 │                          │                  │                │                    │                │
 │                          │                  │                │  Agent evaluates   │                │
 │                          │                  │                │  QUALITY of the    │                │
 │                          │                  │                │  opportunity, not  │                │
 │                          │                  │                │  the data itself   │                │
 │                          │                  │                │                    │                │
 │                          │                  │                │  "BSR #23 with 8   │                │
 │                          │                  │                │   sellers, 28%     │                │
 │                          │                  │                │   margin, Lodge    │                │
 │                          │                  │                │   brand — strong   │                │
 │                          │                  │                │   wholesale        │                │
 │                          │                  │                │   opportunity"     │                │
 │                          │                  │                │                    │                │
 │                          │                  │                │   10 candidates    │                │
 │                          │◄──────────────────────────────────────────────────────┤                │
 │                          │                  │                │                    │                │
 │                          │                  │                │                    │  evaluate      │
 │                          │                  │                │                    │  (gate →       │
 │                          │                  │                │                    │   profit →     │
 │                          │                  │                │                    │   demand →     │
 │                          │                  │                │                    │   supplier →   │
 │                          │                  │                │                    │   review)      │
 │                          ├────────────────────────────────────────────────────────────────────────►│
 │                          │                  │                │                    │                │
 │  5-10 deals              │                  │                │                    │  higher pass   │
 │  (real wholesale         │                  │                │                    │  rate because  │
 │   opportunities)         │                  │                │                    │  pre-vetted    │
 │◄────────────────────────────────────────────────────────────────────────────────────────────────────┤
```

**Key differences:**

| Aspect | Current | Proposed |
|--------|---------|----------|
| Input | User types keywords | Category or keyword → expanded |
| Discovery method | Keyword search (random) | BSR-sorted category browse |
| Candidate selection | LLM picks from 20 random | Deterministic ceiling/floor analysis |
| Pre-filtering | After LLM sourcing agent | Before any LLM call |
| Seller count check | After sourcing (wastes LLM) | During category scan (free) |
| Brand filtering | After sourcing (wastes LLM) | During category scan (free) |
| Margin filtering | After sourcing (wastes LLM) | During category scan (free) |
| What LLM does | Picks products + calculates | Assesses pre-vetted opportunities |
| Exa usage | Generic "wholesale FBA" search | Targeted per-brand supplier search |
| Expected pass rate | ~0-10% (mostly private label) | ~30-60% (pre-vetted for wholesale) |
| LLM cost per campaign | ~$1.20 (wastes on bad candidates) | ~$0.60 (only evaluates good ones) |
| Candidates reaching LLM | 5-10 (unfiltered) | 10-15 (all pre-qualified) |
| Deals produced | 0-2 | 5-10 |

---

## SP-API Endpoints Needed

| Phase | Endpoint | Purpose | Current |
|-------|----------|---------|---------|
| Category browse | `/catalog/2022-04-01/items?classificationIds=...&pageSize=20` | Browse by category | Not used |
| BSR data | Already in catalog response `salesRanks` | Ceiling/floor analysis | Partially used |
| Competitive pricing | `/products/pricing/v0/competitivePrice` | Price + seller count | Used |
| Fee estimate | `/products/fees/v0/items/{Asin}/feesEstimate` | Real FBA fees | Using deterministic calc |
| Listing restrictions | `/listings/2021-08-01/restrictions` | Can we sell this? | Used (unreliable) |
