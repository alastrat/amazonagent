# Assessment UI Fixes — Plan

**Date:** 2026-04-10
**Status:** Pending — 6 known issues from E2E testing
**Branch:** feat/seller-account-assessment-v2

---

## Known Issues (from testing session)

### Issue 1: Inngest local dev "Unable to reach SDK URL"

**Problem:** Inngest Docker container can't callback to the host API reliably. The assessment runs but Inngest marks it as "Failed" even though the work completed.

**Root cause:** `host.docker.internal` resolution is flaky on macOS Docker Desktop. The Inngest SDK tries to register/sync with the dev server but the signing key and URL resolution cause intermittent failures.

**Fix:**
- Add `INNGEST_SERVE_ORIGIN=http://host.docker.internal:8081` to the API env vars in start.sh
- This tells the SDK to advertise a reachable URL to the dev server
- Alternative: run Inngest natively (not in Docker) to avoid host networking issues

**Effort:** 30 min

---

### Issue 2: Graph shows 160+ flat ASIN nodes — needs hierarchical drill-down

**Problem:** Every individual ASIN is rendered as a separate node in the force-directed graph. With 160+ nodes they overlap and become unreadable. No way to drill into categories.

**What we need:** A hierarchical visualization where:
- Level 0: Amazon Marketplace (root)
- Level 1: Categories (20 nodes) — click to expand
- Level 2: Brands within a category — click to expand
- Level 3: Products within a brand (only shown when drilled in)

**Library evaluation:** (pending research agent results)

**Fix:**
- Replace react-force-graph-2d with a hierarchical library (d3-hierarchy collapsible tree or @visx/hierarchy)
- Refactor `/assessment/graph` endpoint to return tree structure instead of flat nodes
- Add click-to-expand interaction
- Only show Level 1 (categories) by default
- Each level shows aggregate stats (product count, eligible count, open rate)

**Effort:** 1-2 days

---

### Issue 3: Category names blank in eligibility table

**Problem:** The Category Eligibility table shows rows but the "Category" column is empty.

**Root cause:** The fingerprint's `categories[]` stores category data from the scan, but the `category` field in the `assessment_probe_results` table may be empty because the SP-API `SearchProducts` response uses a different category field than what we're storing.

**Fix:**
- Check what category name the search results return — likely `Category` field from `port.ProductSearchResult`
- Ensure `AssessmentSearchResult.Category` is populated from the search response
- In the graph endpoint, use the category name from `DiscoveryCategories` (which we have) instead of relying on the fingerprint

**Effort:** 1 hour

---

### Issue 4: "80/20 categories scanned" — counting bug

**Problem:** Stats show 80/20 or 40/20 categories scanned, which is impossible.

**Root cause:** The `categories_scanned` in the graph endpoint counts `len(fingerprint.Categories)` but the fingerprint may have duplicate category entries (one per scan attempt, not deduplicated).

**Fix:**
- Deduplicate categories in the fingerprint before counting
- Or use a map to count unique categories in the graph endpoint
- Also fix the graph endpoint to return accurate `categories_total` from `len(DiscoveryCategories)`

**Effort:** 30 min

---

### Issue 5: No product table on Reveal step

**Problem:** The Reveal step shows categories and a graph but doesn't list the actual eligible products the user can sell. Users need to see specific ASINs, titles, prices, and margins.

**Fix:**
- Add a "Top Opportunities" table to the Reveal step
- Pull data from the assessment outcome (which includes `OpportunityResult.Products`)
- Table columns: ASIN, Title, Brand, Category, Price, Estimated Margin, Seller Count
- Sort by margin descending
- Cap at 20 products

**Effort:** 1-2 hours

---

### Issue 6: Assessment runs but data only visible after completion

**Problem:** During the Discover step, the graph only shows the root node because the fingerprint isn't saved until the scan completes. Users see a mostly empty graph during the 3-minute scan.

**Fix:**
- Option A: Save partial fingerprint data incrementally (after each category) so the graph updates in real-time
- Option B: Return scan progress data from a separate in-memory cache that the Inngest workflow updates per-step
- Option C: Accept the current behavior and improve the progress bar/stats to give better feedback during scanning

**Recommended:** Option C for now (simplest), Option A for next iteration.

**Effort:** Option C: 30 min, Option A: 1 day

---

## Fix Priority

| # | Issue | Priority | Effort |
|---|-------|----------|--------|
| 3 | Category names blank | High — quick win | 1 hour |
| 4 | Counting bug (80/20) | High — quick win | 30 min |
| 5 | No product table | High — core value | 1-2 hours |
| 2 | Graph drill-down | Medium — UX quality | 1-2 days |
| 1 | Inngest local dev | Medium — dev experience | 30 min |
| 6 | Real-time graph updates | Low — polish | 30 min - 1 day |

**Recommended order:** 3 → 4 → 5 → 1 → 2 → 6

Quick wins first (category names, counting bug, product table), then the graph library swap.
