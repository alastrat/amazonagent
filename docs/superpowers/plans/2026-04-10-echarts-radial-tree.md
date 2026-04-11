# ECharts Radial Tree Discovery Graph — Implementation Plan

**Date:** 2026-04-10
**Status:** Ready for implementation
**Branch:** feat/seller-account-assessment-v2
**Library:** Apache ECharts via echarts-for-react

---

## Overview

Replace the current react-d3-tree/react-force-graph with an ECharts radial tree visualization matching the Apache ECharts tree-radial example. Categories and brands expand radially from a center root node. Clicking a brand shows its products in a table below.

---

## Backend Changes

### Enrich BrandProbeResult (`internal/domain/seller_profile.go`)

Add 4 fields to BrandProbeResult:
```go
Title        string  `json:"title"`
Price        float64 `json:"price"`
EstMarginPct float64 `json:"est_margin_pct"`
SellerCount  int     `json:"seller_count"`
```

### Populate in assessment (`internal/service/assessment_service.go`)

In `persistFingerprint`, map `AssessmentSearchResult` fields to `BrandProbeResult`:
- r.Title → Title
- r.AmazonPrice → Price
- r.SellerCount → SellerCount

### Restructure GetGraph endpoint (`internal/api/handler/assessment_handler.go`)

Include products as children of brands in the tree:
```
Root → Categories → Brands → Products (as tree leaf nodes)
```

Each product node: `{ id, name (title), type: "product", asin, price, est_margin_pct, seller_count, eligible, value: 1 }`

Add `value` field to all nodes for ECharts radial sizing:
- Root: category count
- Category: eligible_count
- Brand: eligible product count (min 1)
- Product: 1

Include all products inline (max ~400, under 200KB JSON — no lazy loading needed).

---

## Frontend Changes

### Install packages (`apps/web/`)

```bash
npm install echarts echarts-for-react
npm uninstall react-d3-tree react-force-graph-2d
```

### Rewrite DiscoveryGraph component (`apps/web/src/components/discovery-graph.tsx`)

Props:
```ts
interface DiscoveryGraphProps {
  tree: TreeNode;
  products?: ProductRecommendation[];
  width?: number;
  height?: number;
}
```

ECharts option:
```ts
{
  tooltip: { trigger: "item" },
  series: [{
    type: "tree",
    layout: "radial",
    data: [toEChartsTree(tree)],
    initialTreeDepth: 3,
    symbol: "circle",
    symbolSize: nodeType => root: 20, category: 14, brand: 9,
    emphasis: { focus: "descendant" },
    animationDurationUpdate: 750,
    roam: true,
    label: { show: true, fontSize: 10 }
  }]
}
```

Color coding (client-side):
- Category: green if open_rate > 50%, red if < 20%, yellow otherwise
- Brand: green (#22c55e) if eligible, red (#ef4444) if restricted
- Product: light green/light red based on eligible

### Click-to-Table Interaction

State: `useState<string | null>(null)` for `selectedNodeId`

ECharts click handler:
```ts
function handleNodeClick(params) {
  if (params.data.nodeType === "brand" || params.data.nodeType === "category") {
    setSelectedNodeId(prev => prev === params.data.id ? null : params.data.id);
  }
}
```

BrandProductTable (inline sub-component):
- Filters products by brand name (brand click) or category (category click)
- Columns: ASIN, Title, Brand, Category, Price, Est. Margin %, Sellers, Eligibility badge
- Sorted by margin descending
- Empty state: "Click a category or brand in the tree to see products"
- Appears below the chart with smooth transition

### Update onboarding page (`apps/web/src/app/(app)/onboarding/page.tsx`)

Discover step: `<DiscoveryGraph tree={tree} />` (no products during scan)
Reveal step: `<DiscoveryGraph tree={tree} products={outcome?.opportunity?.products} height={500} />`

---

## Files to Modify

| File | Change |
|------|--------|
| `internal/domain/seller_profile.go` | Add 4 fields to BrandProbeResult |
| `internal/service/assessment_service.go` | Populate new fields in persistFingerprint |
| `internal/api/handler/assessment_handler.go` | Include products in tree, add value fields |
| `apps/web/package.json` | Install echarts, remove react-d3-tree |
| `apps/web/src/components/discovery-graph.tsx` | Full rewrite with ECharts radial tree |
| `apps/web/src/app/(app)/onboarding/page.tsx` | Pass products prop, update height |
| `apps/web/src/lib/types.ts` | Add value field to TreeNode |

---

## Build Sequence

1. Backend: enrich BrandProbeResult + populate in assessment + restructure GetGraph
2. Frontend: install echarts + rewrite discovery-graph + update onboarding page
3. Test: `make stop && make start && make assess` → verify radial tree + click-to-table
