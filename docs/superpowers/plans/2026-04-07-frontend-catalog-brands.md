# Frontend: Catalog Explorer + Brand Intelligence — Implementation Plan

**Date:** 2026-04-07
**Status:** Ready for implementation
**Depends on:** Phases A-E backend complete

---

## Build Sequence

### Phase 1 — Types and API layer

- [ ] Add `DiscoveredProduct`, `BrandIntelligence`, `CatalogStats`, `ScanJob`, `FunnelStats`, `UploadFunnelResponse` types to `types.ts`
- [ ] Add `getCatalogProducts`, `getCatalogStats`, `getBrands`, `getBrandProducts`, `evaluateProducts`, `uploadPriceList`, `getScans`, `getScan` methods to `api-client.ts`
- [ ] Add `catalog`, `brands`, `scans` query key entries to `query-keys.ts`

### Phase 2 — Hooks

- [ ] Create `use-catalog.ts` — `useCatalogProducts(params)`, `useCatalogStats()`, `useEvaluateProducts()`
- [ ] Create `use-brands.ts` — `useBrands(params)`, `useBrandProducts(brandId, params)`
- [ ] Create `use-scans.ts` — `useScans()`, `useScan(id, isActive)`, `useUploadPriceList()`, `usePollScanJob(id)`

### Phase 3 — Shared components

- [ ] Create `eligibility-badge.tsx` — Badge with color per eligibility status
- [ ] Create `funnel-pipeline.tsx` — horizontal pipeline with proportional fill bars per tier
- [ ] Create `scan-progress-card.tsx` — self-polling card with progress bar + funnel
- [ ] Create `price-list-upload-dialog.tsx` — drag-and-drop CSV upload dialog

### Phase 4 — Pages

- [ ] Create `/catalog` page — product table with search, sort, filter, bulk "Evaluate Selected"
- [ ] Create `/brands` page — brand intelligence table with filters
- [ ] Create `/brands/[id]` page — brand detail + products table
- [ ] Create `/scans` page — scan history with upload button, active progress cards
- [ ] Create `/scans/[id]` page — scan results with funnel visualization + survivors table

### Phase 5 — Modifications

- [ ] Add Catalog, Brands, Scans nav items to `app-shell.tsx`
- [ ] Add "Upload Price List" button + dialog to dashboard
- [ ] Add scan history section to discovery page

---

## Key Patterns (from codebase analysis)

- Pages: `"use client"`, `useState` for filters, params passed to hooks
- Hooks: `useQuery`/`useMutation` wrapping `apiClient`, invalidate on success
- Tables: inline `<table>` with `rounded-lg border`, `bg-muted/50` header, `hover:bg-muted/30` rows
- Filters: raw `<select>` with Tailwind classes (not shadcn Select)
- Nav: hardcoded `navItems` in `app-shell.tsx`

## Types

```ts
export interface DiscoveredProduct {
  id: string; tenant_id: string; asin: string; title: string;
  brand_id: string; category: string; browse_node_id?: string;
  estimated_price?: number; buy_box_price?: number;
  bsr_rank?: number; seller_count?: number;
  estimated_margin_pct?: number; real_margin_pct?: number;
  eligibility_status: string; data_quality: number;
  refresh_priority: number; source: string;
  first_seen_at: string; last_seen_at: string; price_updated_at?: string;
}

export interface BrandIntelligence {
  tenant_id: string; brand_id: string; brand_name: string;
  category: string; product_count: number; high_margin_count: number;
  avg_margin: number; avg_sellers: number; avg_bsr: number;
}

export interface FunnelStats {
  input_count: number; t0_deduped: number;
  t1_margin_killed: number; t2_brand_killed: number;
  t3_enrich_killed: number; survivor_count: number;
}

export interface ScanJob {
  id: string; type: string; status: string;
  total_items: number; processed: number;
  qualified: number; eliminated: number;
  started_at: string; completed_at?: string;
  metadata?: Record<string, any>;
}
```

## Funnel Visualization

Horizontal pipeline: five stage blocks connected by chevrons. Each block:
- Stage label (top, muted)
- Count in bold
- Fill bar proportional to `count / input_count`
- Drop label: "- N eliminated" in red

Stages: Input → Deduped → Margin Pass → Brand Pass → Survivors (emerald highlight)

## Upload Dialog

- Drag-and-drop zone with dashed border
- File picker (`.csv` only)
- Distributor name input
- On submit → `POST /pricelist/upload-funnel` → poll `GET /scans/:id` every 2s
- Live funnel pipeline during processing
- "View Survivors" button on completion

## Polling

`useScan(id, isActive)` with `refetchInterval: (query) => terminal ? false : 2000`. Stop on `completed`/`failed`.

## Upload multipart

Skip default `Content-Type: application/json` — let browser set `multipart/form-data` boundary automatically.
