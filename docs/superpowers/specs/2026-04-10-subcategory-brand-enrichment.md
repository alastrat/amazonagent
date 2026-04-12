# Subcategory + Brand Enrichment — Design Spec

**Date:** 2026-04-10
**Status:** Draft
**Branch:** feat/seller-account-assessment-v2
**Problem:** SP-API `SearchProducts` returns products without brand names or subcategory detail. The graph shows "Other" for every brand and has no subcategory grouping.

---

## 1. Current Problem

`SearchProducts` returns: ASIN, title, BSR rank, and a broad category (e.g., "Home & Kitchen"). It does NOT reliably return:
- **Brand name** — empty for ~80% of results
- **Subcategory** — only the top-level classification
- **Seller count** — requires separate competitive pricing call

Result: the radial tree shows `Category → "Other" → "Other" → ...` — useless for decision-making.

---

## 2. Target Hierarchy

```
Amazon US (root)
  └── Home & Kitchen (category)
        ├── Kitchen Storage & Organization (subcategory)
        │     ├── Rubbermaid (brand) — 3 products, 2 eligible
        │     └── OXO (brand) — 1 product, 0 eligible
        ├── Kitchen Utensils & Gadgets (subcategory)
        │     └── KitchenAid (brand) — 2 products, 1 eligible
        └── Cookware (subcategory)
              └── Lodge (brand) — 1 product, 1 eligible
```

Click any brand → product table below shows ASIN, title, price, margin, sellers, eligibility.

---

## 3. Data Sources

### From `SearchProducts` (already called)
- ASIN, Title, AmazonPrice (sometimes)
- `salesRanks[].classificationRanks[].title` → this IS the subcategory name
- `salesRanks[].classificationRanks[].rank` → BSR in subcategory
- `summaries[].brandName` → sometimes empty

### From `GetProductDetails` (competitive pricing — new enrichment step)
- Buy Box price (real, not list price)
- Seller count (NumberOfOfferListings)
- Does NOT return brand name

### From `getCatalogItem` (single-item lookup — expensive but has brand)
- `summaries[].brandName` — reliable
- 2 req/sec rate limit
- Too slow for 400 products

### Best approach: extract from `SearchProducts` response more carefully

The `SearchProducts` endpoint with `includedData=summaries,salesRanks` already returns:
- `summaries[0].brandName` — it IS there, but we might not be parsing it correctly
- `salesRanks[0].classificationRanks[0].title` — the subcategory name

Let me check: the current `SearchProducts` in `spapi/client.go` already parses `brandName` from summaries. The issue might be that many products genuinely have no brand in Amazon's catalog, OR the brand field is in a different location for these results.

---

## 4. Implementation Plan

### Phase 1: Extract subcategory from existing search results

The SP-API `SearchProducts` response includes `salesRanks` which has `classificationRanks` with the subcategory:

```json
"salesRanks": [{
  "classificationRanks": [{
    "title": "Kitchen Storage & Organization",
    "rank": 1234
  }]
}]
```

**We're already parsing this as `BSRCategory` in `port.ProductSearchResult`.** The field exists but we're not using it in the assessment.

Changes:
- `AssessmentSearchResult` — add `Subcategory string` field
- Assessment scan — populate from `product.BSRCategory`
- `BrandProbeResult` — add `Subcategory string` field
- GetGraph — use subcategory as an intermediate tree level

### Phase 2: Batch enrich with competitive pricing for brands

After `SearchProducts` returns ASINs, call `GetProductDetails` (competitive pricing, batch 20) to get:
- Real Buy Box price
- Seller count
- (Brand is NOT in competitive pricing, but it confirms product exists)

This is already implemented in the SP-API client — `GetProductDetails` batches 20 ASINs. We just need to call it during the assessment for eligible products.

### Phase 3: Second-pass brand extraction

For products where brand is still empty after SearchProducts:
- Try extracting brand from the title (first 1-3 words before common patterns like "Set", "Pack", numbers)
- Or accept empty and group under subcategory directly (no brand level for unbranded products)

**Decision: don't extract from title — too fragile. Group brandless products under a "Generic" node within the subcategory.**

---

## 5. Tree Structure Changes

### Backend GetGraph response

```json
{
  "tree": {
    "id": "root",
    "name": "Amazon US",
    "children": [
      {
        "id": "cat-home-kitchen",
        "name": "Home & Kitchen",
        "type": "category",
        "open_rate": 25.0,
        "eligible_count": 5,
        "total_count": 20,
        "children": [
          {
            "id": "subcat-kitchen-storage",
            "name": "Kitchen Storage & Organization",
            "type": "subcategory",
            "eligible_count": 3,
            "total_count": 8,
            "children": [
              {
                "id": "brand-rubbermaid",
                "name": "Rubbermaid",
                "type": "brand",
                "eligible": true,
                "product_count": 3
              },
              {
                "id": "brand-generic-kitchen-storage",
                "name": "Generic",
                "type": "brand",
                "eligible": false,
                "product_count": 2
              }
            ]
          }
        ]
      }
    ]
  },
  "products": [...]
}
```

### Frontend ECharts tree

- `initialTreeDepth: 2` (show root + categories + subcategories expanded)
- Subcategory nodes: smaller than category, color-coded by eligible ratio
- Brand nodes: leaf level, green/red by eligibility
- Click brand → product table
- Click subcategory → product table filtered by subcategory

---

## 6. Assessment Scan Changes

### Current flow
```
For each category:
  SearchProducts(category_name) → 20 products
  For each product: CheckListingEligibility → eligible/restricted
```

### New flow
```
For each category:
  SearchProducts(category_name) → 20 products
    Extract: ASIN, title, brand (from summaries), subcategory (from salesRanks)
  
  Batch GetProductDetails(eligible_ASINs, 20 per batch) → real price, seller count
    Merge: buy_box_price, seller_count into product data
  
  For each product: CheckListingEligibility → eligible/restricted
  
  Store: subcategory + brand with each product result
```

API call budget change:
- Current: 20 categories × (1 search + 20 eligibility) = ~420 calls
- New: 20 categories × (1 search + 1 batch pricing + 20 eligibility) = ~440 calls
- Within 600 call budget, minimal increase

---

## 7. Domain Model Changes

### AssessmentSearchResult
Add:
```go
Subcategory string `json:"subcategory"`
```

### BrandProbeResult
Add:
```go
Subcategory string `json:"subcategory"`
```

### Migration
Add `subcategory` column to `assessment_probe_results` table.

---

## 8. Frontend Changes

### TreeNode type
Add `"subcategory"` to the type union.

### ECharts component
- Add subcategory node styling (medium size between category and brand)
- Subcategory color: proportional to eligible/total ratio
- `initialTreeDepth: 2` to show subcategories expanded by default

### Product table
- Add subcategory column
- Filter by subcategory when subcategory node is clicked

---

## 9. Files to Modify

| File | Change |
|------|--------|
| `internal/domain/seller_profile.go` | Add Subcategory to AssessmentSearchResult + BrandProbeResult |
| `internal/service/assessment_service.go` | Populate subcategory from BSRCategory, add batch GetProductDetails call |
| `internal/api/handler/assessment_handler.go` | Add subcategory level to tree, group brands under subcategories |
| `internal/adapter/postgres/eligibility_fingerprint_repo.go` | Persist/read subcategory field |
| `internal/adapter/postgres/migrations/018_subcategory.sql` | Add subcategory column |
| `apps/web/src/lib/types.ts` | Add subcategory to TreeNode type union + ProductDetail |
| `apps/web/src/components/discovery-graph.tsx` | Add subcategory node styling |
| `apps/web/src/components/discovery-product-table.tsx` | Add subcategory filter + column |

---

## 10. Estimated Effort

- Backend: 2-3 hours (subcategory extraction + enrichment + tree restructure)
- Frontend: 1-2 hours (tree styling + table column)
- Testing: 1 hour

Total: ~half day
