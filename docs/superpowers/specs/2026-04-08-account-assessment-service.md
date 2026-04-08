# Account Assessment Service — Design Spec

**Date:** 2026-04-08
**Status:** Draft — pending review
**Scope:** Assess each seller's unique situation at account connection, generate an eligibility fingerprint, classify seller archetype, and produce a personalized strategy brief that directs the discovery engine
**Research:** [AI Concierge Expert Brainstorm](../research/2026-04-08-ai-concierge-expert-brainstorm.md)

---

## 1. Goal

When a seller connects their Amazon account, the platform has a narrow window to prove value. Every other tool drops the user into a blank dashboard and says "search for something." We do the opposite: we immediately assess the seller's unique situation — what they can sell, what they cannot, what stage they are at, and what their best first moves are — then deliver a personalized Strategy Brief within 5 minutes.

This is the "Wealthfront moment" described in the expert research. The assessment turns a generic tool into a concierge that knows *this* seller.

The assessment accomplishes three things:

1. **Eligibility Fingerprint** — sample 300 ASINs across categories and brands to map what this seller can and cannot list, without exhaustively checking every product.
2. **Seller Archetype Classification** — determine whether the seller is a Greenhorn, RA-to-Wholesale transitioner, Expanding Pro, or Capital-Rich Beginner, so advice is calibrated to their stage.
3. **Strategy Brief** — synthesize the fingerprint and archetype into actionable output: top categories, quick-win brands, ungating roadmap, and capital allocation guidance.

The Strategy Brief then directs the existing discovery engine to scan only eligible categories and brands — no wasted API calls on restricted products the seller cannot list.

---

## 2. The Assessment Flow

### 2.1 Onboarding Sequence

```
Connect (30s)          Discover (3-5 min, async)       Reveal (60s)         Commit (30s)
─────────────────────  ──────────────────────────────  ──────────────────  ──────────────
User completes         Platform runs assessment in     Strategy Brief      User accepts
SP-API OAuth.          background. Progress shown in   appears with        or customizes
Credentials stored.    real-time: "Checking 30         top categories,     the plan. Discovery
                       categories... 47% complete"     quick-win brands,   engine reconfigured
                                                       ungating roadmap.   to match strategy.
```

### 2.2 What Happens During "Discover" (3-5 Minutes)

The assessment is a single Inngest workflow with parallelized steps:

```
1. Pull account metadata via SP-API           (~5s)
   ├── Seller account info (account age, marketplace)
   ├── Active listings count
   └── Account health metrics (if available)

2. Run eligibility sampling (parallel)        (~2-4 min, rate-limited)
   ├── 30 categories × 10 ASINs each = 300 checks
   ├── SP-API ListingRestrictions endpoint per ASIN
   ├── Results cached in brand_eligibility table
   └── Progress emitted every 30 ASINs via domain events

3. Classify seller archetype                  (~1s, deterministic)
   ├── Input: account age, listing count, eligibility results
   └── Output: archetype + confidence

4. Generate Strategy Brief                    (~10s, LLM)
   ├── Input: fingerprint + archetype + category economics
   ├── Output: top categories, quick-win brands, ungating roadmap
   └── Uses scoring formula from expert research

5. Configure discovery engine                 (~1s)
   ├── Update DiscoveryConfig with eligible categories
   ├── Set brand_eligibility cache for scanned brands
   └── Create initial scan rotation from strategy
```

### 2.3 SP-API Budget

The entire assessment consumes approximately:

| Endpoint | Calls | Rate Limit | Time |
|---|---|---|---|
| ListingRestrictions | 300 | 5/sec | ~60s |
| GetMyFeesEstimate | 0 | (deferred to discovery) | 0s |
| CatalogItems | ~30 | 5/sec | ~6s |
| CompetitivePricing | 0 | (deferred to discovery) | 0s |
| **Total** | **~330** | | **~90s sequential, ~60s with parallelism** |

The 300 restriction checks are the bottleneck. At 5 requests/second (SP-API documented rate for ListingRestrictions), the raw API time is 60 seconds. With overhead, error retries, and conservative rate limiting, expect 90-120 seconds. The rest of the assessment pipeline (archetype classification, strategy generation) runs in parallel or after the sampling completes.

---

## 3. Eligibility Fingerprint — The 300-ASIN Sampling Strategy

### 3.1 Why 300 ASINs

The expert research establishes that 300-500 SP-API calls yield ~90% accuracy on a seller's eligibility landscape. We target 300 as the minimum viable sample — 10 ASINs across 30 categories — because:

- It fits within a 2-minute SP-API budget at documented rates
- It covers all major wholesale-relevant categories
- It provides enough brand diversity to detect brand-level gating patterns
- Subsequent price list scans will fill gaps as specific brands are encountered

### 3.2 Category Selection (30 Categories)

The 30 categories are the Amazon top-level browse nodes most relevant to wholesale:

```
Tier 1 — High wholesale volume (10 categories, 4 samples each = 40 ASINs):
  Home & Kitchen, Kitchen & Dining, Sports & Outdoors, Tools & Home Improvement,
  Office Products, Patio Lawn & Garden, Automotive, Industrial & Scientific,
  Arts Crafts & Sewing, Musical Instruments

Tier 2 — Gated but high-margin (10 categories, 3 samples each = 30 ASINs):
  Grocery & Gourmet Food, Health & Household, Beauty & Personal Care,
  Toys & Games, Baby, Pet Supplies, Electronics, Clothing Shoes & Jewelry,
  Cell Phones & Accessories, Appliances

Tier 3 — Niche or low-volume (10 categories, 2 samples each = 20 ASINs):
  Books, Garden & Outdoor, Handmade Products, Collectibles & Fine Art,
  Software, Video Games, Movies & TV, CDs & Vinyl,
  Camera & Photo, Luggage

Calibration ASINs (10 additional):
  Known-open ASINs (should always pass) — 5 ASINs
  Known-restricted ASINs (should always fail) — 5 ASINs
  → Used to validate the assessment is working correctly
```

**Total: 100 ASINs (Tier 1 + calibration) + 90 ASINs (Tier 2) + 60 ASINs (Tier 3) + 50 ASINs (brand sampling below) = 300 ASINs**

### 3.3 ASIN Selection Within Each Category

For each category, the sample ASINs are chosen to probe different gating levels:

```
Per-category sample (Tier 1 example: 4 ASINs):
  ASIN 1: Top brand in category (e.g., KitchenAid in Kitchen)     — likely brand-gated
  ASIN 2: Mid-tier brand (e.g., OXO in Kitchen)                   — sometimes gated
  ASIN 3: Generic/white-label product (e.g., Amazon Basics)       — usually open
  ASIN 4: High-BSR niche product (random selection)                — tests category gate
```

This stratified sampling reveals whether the restriction is at the *category* level (all 4 blocked = category-gated), the *brand* level (top brand blocked, generic open = brand-gated), or fully open (all pass).

### 3.4 Brand Sampling (50 Additional ASINs)

Beyond category coverage, 50 ASINs are dedicated to probing the top wholesale brands:

```
Top 25 wholesale brands × 2 ASINs each = 50 checks:
  Procter & Gamble, Unilever, Nestlé, Colgate-Palmolive, Church & Dwight,
  Henkel, SC Johnson, Reckitt, General Mills, Kellogg's,
  LEGO, Hasbro, Mattel, Nintendo, Samsung,
  3M, Energizer, Duracell, Rubbermaid, Clorox,
  Brita, Cuisinart, KitchenAid, Dyson, iRobot
```

These brands represent the highest-volume wholesale opportunities. Knowing which of these a seller can access immediately shapes the strategy.

### 3.5 The Fingerprint Output

The sampling produces an `EligibilityFingerprint` with:

```
Per category (30 entries):
  - category_name
  - category_browse_node_id
  - asins_checked: 2-4
  - asins_eligible: 0-4
  - eligibility_score: 0.0-1.0 (eligible / checked)
  - gate_type: open | category_gated | brand_gated | mixed
  - sample_brands_eligible: ["OXO", "AmazonBasics"]
  - sample_brands_restricted: ["KitchenAid"]

Per brand (25 entries):
  - brand_name
  - category
  - asins_checked: 2
  - eligible: true | false
  - gate_reason: "" | "Brand approval required" | "Invoice required"

Aggregate:
  - total_categories_open: 0-30
  - total_categories_partial: 0-30
  - total_categories_gated: 0-30
  - overall_openness_score: 0.0-1.0
  - estimated_accessible_catalog_pct: 20-80%
  - calibration_passed: true | false (sanity check)
```

### 3.6 ASIN Registry

The 300 sample ASINs are maintained as a curated registry in the database (`assessment_sample_asins` table), not hardcoded. This allows:

- Updating ASINs when products are discontinued
- A/B testing different sampling strategies
- Per-marketplace ASIN sets (US vs. EU vs. UK)
- Adding seasonal ASINs (Toys in Q4)

The registry is seeded during initial deployment and refreshed quarterly.

---

## 4. Seller Archetype Classification

### 4.1 Archetypes

| Archetype | Criteria | Strategy Implications |
|---|---|---|
| **Greenhorn** | Account age < 90 days AND active listings < 10 | Open categories only. Build account health. No ungating recommendations yet. Conservative capital allocation. |
| **RA-to-Wholesale** | Account age 6-12 months AND active listings 10-100 AND mostly individual (non-wholesale) listings | Has health metrics for ungating. Needs distributor education. Cash flow planning emphasis. Transition away from RA. |
| **Expanding Pro** | Account age > 12 months AND active listings > 100 AND good health metrics | More brands, better margins, automation. Power user features. Aggressive ungating. |
| **Capital-Rich Beginner** | Account age < 6 months AND (high credit limit OR stated capital > $30K) | Needs guardrails. Risk of over-investing before account health established. Measured scaling plan. |

### 4.2 Classification Logic

The classification is deterministic (no LLM) and runs as a decision tree:

```
Input signals:
  - account_age_days       (from SP-API seller account info)
  - active_listing_count   (from SP-API)
  - eligibility_openness   (from fingerprint — proxy for account maturity)
  - stated_capital         (from onboarding form, optional)
  - prior_experience       (from onboarding form, optional)

Decision tree:
  IF account_age_days < 90 AND active_listing_count < 10:
    IF stated_capital > 30000:
      → Capital-Rich Beginner (confidence: high if stated_capital provided, medium otherwise)
    ELSE:
      → Greenhorn (confidence: high)

  ELIF account_age_days BETWEEN 90 AND 365:
    IF active_listing_count > 100:
      → Expanding Pro (confidence: high)
    ELIF active_listing_count BETWEEN 10 AND 100:
      → RA-to-Wholesale (confidence: medium)
    ELSE:
      → Greenhorn (confidence: medium — old account but inactive)

  ELIF account_age_days > 365:
    IF active_listing_count > 100:
      → Expanding Pro (confidence: high)
    ELSE:
      → RA-to-Wholesale (confidence: medium)
```

The archetype can be overridden by the user ("I'm more experienced than this suggests") and re-evaluated as account data changes.

### 4.3 How Archetype Affects Strategy

| Dimension | Greenhorn | RA-to-Wholesale | Expanding Pro | Capital-Rich |
|---|---|---|---|---|
| Categories recommended | Open only (top 3) | Open + 1-2 easy ungates | All eligible + aggressive ungating | Open first, scale after health |
| Brands recommended | Generic/mid-tier | Mid-tier + first branded | Top brands | Mid-tier with guardrails |
| Capital allocation | 70% inventory, 14% reserve | 60% inventory, 20% ungating | 50% inventory, 30% expansion | 40% inventory, 30% reserve |
| Ungating roadmap | "Not yet — build health first" | Grocery first, then Health | 3-5 targets ranked by ROI | 1-2 safe targets max |
| Risk tolerance | Conservative | Moderate | Aggressive | Very conservative |
| Scan frequency | Weekly | Nightly | Twice daily | Weekly |

---

## 5. Strategy Brief Generation

### 5.1 What the Strategy Brief Contains

The Strategy Brief is the primary output of the assessment — the deliverable the user sees at the end of the "Discover" phase. It contains:

```
StrategyBrief {
  // Header
  generated_at, archetype, confidence_level

  // Section 1: Your Account Snapshot
  account_age_days, active_listings, overall_openness_score,
  estimated_accessible_catalog_pct

  // Section 2: Top Categories (ranked by CategoryScore)
  top_categories[] {
    category_name, eligibility_score, avg_margin, competition_level,
    ungating_difficulty, category_score (composite),
    estimated_monthly_revenue_potential,
    recommended_action: "Start here" | "Ungate next" | "Long-term target"
  }

  // Section 3: Quick-Win Brands (eligible, high-margin)
  quick_win_brands[] {
    brand_name, category, eligible: true,
    estimated_product_count, avg_margin,
    why: "High margin, low competition, you're already approved"
  }

  // Section 4: Ungating Roadmap (ordered by ROI / difficulty)
  ungating_targets[] {
    category_or_brand, current_status: "restricted",
    difficulty: 1-4 (invoice | brand_approval | performance_gate | impossible),
    estimated_cost, estimated_revenue_unlock,
    roi_on_ungating, recommended_month: 1-6,
    how: "Purchase $300 invoice from KeHE Distributors"
  }

  // Section 5: Capital Allocation (based on archetype)
  capital_plan {
    total_recommended_start,
    allocations[] { purpose, percentage, amount, rationale }
  }

  // Section 6: First Actions
  first_actions[] {
    priority: 1-5,
    action: "Upload your first price list from X category",
    why, estimated_time
  }
}
```

### 5.2 Category Scoring Formula

Categories are ranked using the formula from the expert research, adapted with eligibility data:

```
CategoryScore = (E x 0.30) + (M x 0.25) + (1/C x 0.20) + (1/D x 0.15) + (1/K x 0.10)

E = eligibility_score (from fingerprint, 0.0-1.0)
M = avg_net_margin (normalized 0-1, from category economics lookup table)
C = competition_density (normalized, avg offer count — lower is better)
D = ungating_difficulty (1=open, 2=invoice, 3=brand_approval, 4=performance_gate)
K = min_capital_required (normalized — lower is better)
```

For sellers with the **Greenhorn** archetype, `D` is weighted more heavily (0.25 instead of 0.15) because they cannot yet pursue ungating.

### 5.3 Strategy Generation (LLM)

The top categories, quick-win brands, and ungating roadmap are computed deterministically using the scoring formula and fingerprint data. The LLM is used only for:

1. **Narrative synthesis** — turning structured data into readable recommendations ("Based on your account, Home & Kitchen is your strongest category because...")
2. **Ungating guidance** — generating specific how-to instructions for each ungating target based on known distributor relationships
3. **Capital allocation rationale** — explaining why the allocation makes sense for this archetype

The LLM call uses a structured output schema to ensure the brief is complete and parseable.

---

## 6. Domain Models

### 6.1 New Types

```go
// SellerProfile captures the assessed state of a seller's Amazon account.
// Created during onboarding assessment, updated periodically.
type SellerProfile struct {
    ID                  string            `json:"id"`
    TenantID            TenantID          `json:"tenant_id"`
    SellerID            string            `json:"seller_id"`            // Amazon Seller ID
    Marketplace         string            `json:"marketplace"`          // US, UK, EU
    AccountAgeDays      int               `json:"account_age_days"`
    ActiveListingCount  int               `json:"active_listing_count"`
    Archetype           SellerArchetype   `json:"archetype"`
    ArchetypeConfidence string            `json:"archetype_confidence"` // high, medium, low
    ArchetypeOverride   *SellerArchetype  `json:"archetype_override,omitempty"` // user override
    StatedCapital       *float64          `json:"stated_capital,omitempty"`
    PriorExperience     *string           `json:"prior_experience,omitempty"`
    AssessmentStatus    AssessmentStatus  `json:"assessment_status"`
    AssessedAt          *time.Time        `json:"assessed_at,omitempty"`
    NextReassessAt      *time.Time        `json:"next_reassess_at,omitempty"`
    CreatedAt           time.Time         `json:"created_at"`
    UpdatedAt           time.Time         `json:"updated_at"`
}

type SellerArchetype string

const (
    ArchetypeGreenhorn       SellerArchetype = "greenhorn"
    ArchetypeRAToWholesale   SellerArchetype = "ra_to_wholesale"
    ArchetypeExpandingPro    SellerArchetype = "expanding_pro"
    ArchetypeCapitalRich     SellerArchetype = "capital_rich_beginner"
)

type AssessmentStatus string

const (
    AssessmentPending    AssessmentStatus = "pending"
    AssessmentRunning    AssessmentStatus = "running"
    AssessmentCompleted  AssessmentStatus = "completed"
    AssessmentFailed     AssessmentStatus = "failed"
)

// EligibilityFingerprint is the result of sampling 300 ASINs across categories/brands.
type EligibilityFingerprint struct {
    ID                         string                   `json:"id"`
    TenantID                   TenantID                 `json:"tenant_id"`
    SellerProfileID            string                   `json:"seller_profile_id"`
    TotalASINsChecked          int                      `json:"total_asins_checked"`
    TotalASINsEligible         int                      `json:"total_asins_eligible"`
    OverallOpennessScore       float64                  `json:"overall_openness_score"`   // 0.0-1.0
    EstimatedAccessiblePct     float64                  `json:"estimated_accessible_pct"` // 20-80%
    CalibrationPassed          bool                     `json:"calibration_passed"`
    CategoryResults            []CategoryEligibility    `json:"category_results"`
    BrandResults               []BrandEligibilityResult `json:"brand_results"`
    SamplingDurationMs         int64                    `json:"sampling_duration_ms"`
    CreatedAt                  time.Time                `json:"created_at"`
}

// CategoryEligibility is the per-category result of eligibility sampling.
type CategoryEligibility struct {
    CategoryName       string   `json:"category_name"`
    BrowseNodeID       string   `json:"browse_node_id"`
    ASINsChecked       int      `json:"asins_checked"`
    ASINsEligible      int      `json:"asins_eligible"`
    EligibilityScore   float64  `json:"eligibility_score"`    // 0.0-1.0
    GateType           GateType `json:"gate_type"`
    BrandsEligible     []string `json:"brands_eligible"`
    BrandsRestricted   []string `json:"brands_restricted"`
}

type GateType string

const (
    GateTypeOpen          GateType = "open"            // all samples pass
    GateTypeCategoryGated GateType = "category_gated"  // all samples fail
    GateTypeBrandGated    GateType = "brand_gated"     // generic pass, branded fail
    GateTypeMixed         GateType = "mixed"           // some pass, some fail
)

// BrandEligibilityResult is the per-brand result from the 50 brand-specific checks.
type BrandEligibilityResult struct {
    BrandName     string `json:"brand_name"`
    Category      string `json:"category"`
    ASINsChecked  int    `json:"asins_checked"`
    Eligible      bool   `json:"eligible"`
    GateReason    string `json:"gate_reason,omitempty"`
}

// StrategyBrief is the actionable output presented to the seller after assessment.
type StrategyBrief struct {
    ID                   string              `json:"id"`
    TenantID             TenantID            `json:"tenant_id"`
    SellerProfileID      string              `json:"seller_profile_id"`
    FingerprintID        string              `json:"fingerprint_id"`
    Archetype            SellerArchetype     `json:"archetype"`
    GeneratedAt          time.Time           `json:"generated_at"`
    TopCategories        []CategoryScore     `json:"top_categories"`
    QuickWinBrands       []BrandOpportunity  `json:"quick_win_brands"`
    UngatingTargets      []UngatingTarget    `json:"ungating_targets"`
    CapitalPlan          CapitalPlan         `json:"capital_plan"`
    FirstActions         []ActionItem        `json:"first_actions"`
    NarrativeSummary     string              `json:"narrative_summary"` // LLM-generated prose
    Version              int                 `json:"version"`           // increments on reassessment
    CreatedAt            time.Time           `json:"created_at"`
}

// CategoryScore is a ranked category recommendation in the Strategy Brief.
type CategoryScore struct {
    CategoryName              string  `json:"category_name"`
    BrowseNodeID              string  `json:"browse_node_id"`
    EligibilityScore          float64 `json:"eligibility_score"`
    AvgMargin                 float64 `json:"avg_margin"`
    CompetitionDensity        float64 `json:"competition_density"`
    UngatingDifficulty        int     `json:"ungating_difficulty"`       // 1-4
    MinCapitalRequired        float64 `json:"min_capital_required"`
    CompositeScore            float64 `json:"composite_score"`
    EstMonthlyRevenuePotential float64 `json:"est_monthly_revenue_potential"`
    RecommendedAction         string  `json:"recommended_action"`        // start_here, ungate_next, long_term
}

// BrandOpportunity is a quick-win brand recommendation.
type BrandOpportunity struct {
    BrandName             string  `json:"brand_name"`
    Category              string  `json:"category"`
    Eligible              bool    `json:"eligible"`
    EstimatedProductCount int     `json:"estimated_product_count"`
    AvgMargin             float64 `json:"avg_margin"`
    Rationale             string  `json:"rationale"`
}

// UngatingTarget is a category or brand the seller should work toward accessing.
type UngatingTarget struct {
    Name                string  `json:"name"`             // category or brand name
    TargetType          string  `json:"target_type"`      // category | brand
    CurrentStatus       string  `json:"current_status"`   // restricted
    Difficulty          int     `json:"difficulty"`        // 1=invoice, 2=brand_approval, 3=performance, 4=impossible
    EstimatedCost       float64 `json:"estimated_cost"`
    EstRevenueUnlock    float64 `json:"est_revenue_unlock"`
    ROIOnUngating       float64 `json:"roi_on_ungating"`
    RecommendedMonth    int     `json:"recommended_month"` // 1-6
    HowToUngate         string  `json:"how_to_ungate"`     // LLM-generated instructions
}

// CapitalPlan is the recommended capital allocation.
type CapitalPlan struct {
    TotalRecommendedStart float64             `json:"total_recommended_start"`
    Allocations           []CapitalAllocation `json:"allocations"`
}

type CapitalAllocation struct {
    Purpose    string  `json:"purpose"`     // inventory, ungating, tools, reserve
    Percentage float64 `json:"percentage"`
    Amount     float64 `json:"amount"`
    Rationale  string  `json:"rationale"`
}

// ActionItem is a concrete first step for the seller.
type ActionItem struct {
    Priority      int    `json:"priority"`
    Action        string `json:"action"`
    Why           string `json:"why"`
    EstimatedTime string `json:"estimated_time"`
}

// AssessmentSampleASIN is a curated ASIN used in eligibility sampling.
type AssessmentSampleASIN struct {
    ID            string `json:"id"`
    ASIN          string `json:"asin"`
    Category      string `json:"category"`
    BrowseNodeID  string `json:"browse_node_id"`
    BrandName     string `json:"brand_name"`
    SampleTier    string `json:"sample_tier"`    // tier1, tier2, tier3, calibration, brand
    SampleRole    string `json:"sample_role"`    // top_brand, mid_brand, generic, niche, known_open, known_restricted
    Marketplace   string `json:"marketplace"`
    Active        bool   `json:"active"`
    LastVerified  time.Time `json:"last_verified"`
}
```

### 6.2 Modified Types

```go
// DiscoveryConfig — add strategy brief reference
type DiscoveryConfig struct {
    // ... existing fields ...
    StrategyBriefID  *string `json:"strategy_brief_id,omitempty"` // links to active strategy
}

// TenantSettings — add assessment preferences
type TenantSettings struct {
    // ... existing fields ...
    AssessmentAutoReassess bool   `json:"assessment_auto_reassess"` // re-run monthly
    AssessmentReassessDays int    `json:"assessment_reassess_days"` // default: 30
}
```

---

## 7. New Interfaces (Ports)

### 7.1 AssessmentService Port

```go
// AssessmentService orchestrates the full account assessment flow.
// This is the primary port — the Inngest workflow calls this.
type AssessmentService interface {
    // StartAssessment triggers the full assessment pipeline for a tenant.
    // Returns the created SellerProfile (in pending status).
    StartAssessment(ctx context.Context, tenantID TenantID, input AssessmentInput) (*SellerProfile, error)

    // GetProfile returns the current seller profile for a tenant.
    GetProfile(ctx context.Context, tenantID TenantID) (*SellerProfile, error)

    // GetFingerprint returns the eligibility fingerprint for a tenant.
    GetFingerprint(ctx context.Context, tenantID TenantID) (*EligibilityFingerprint, error)

    // GetStrategyBrief returns the current strategy brief for a tenant.
    GetStrategyBrief(ctx context.Context, tenantID TenantID) (*StrategyBrief, error)

    // ReassessAccount triggers a re-assessment (updated fingerprint + refreshed strategy).
    ReassessAccount(ctx context.Context, tenantID TenantID) (*SellerProfile, error)

    // OverrideArchetype lets the user correct their archetype classification.
    OverrideArchetype(ctx context.Context, tenantID TenantID, archetype SellerArchetype) error
}

// AssessmentInput is the data collected from the onboarding form + SP-API.
type AssessmentInput struct {
    SellerID        string   // from SP-API OAuth
    Marketplace     string   // US, UK, EU
    StatedCapital   *float64 // optional, from onboarding form
    PriorExperience *string  // optional, from onboarding form
}
```

### 7.2 StrategyEngine Port

```go
// StrategyEngine generates strategy briefs from assessment data.
// Implementation uses deterministic scoring + LLM for narrative.
type StrategyEngine interface {
    // GenerateBrief produces a StrategyBrief from fingerprint + profile.
    GenerateBrief(ctx context.Context, profile *SellerProfile, fingerprint *EligibilityFingerprint) (*StrategyBrief, error)

    // ScoreCategories ranks categories using the composite scoring formula.
    ScoreCategories(ctx context.Context, fingerprint *EligibilityFingerprint, archetype SellerArchetype) ([]CategoryScore, error)

    // IdentifyQuickWins finds eligible brands with high margin potential.
    IdentifyQuickWins(ctx context.Context, fingerprint *EligibilityFingerprint) ([]BrandOpportunity, error)

    // BuildUngatingRoadmap orders ungating targets by ROI / difficulty for the archetype.
    BuildUngatingRoadmap(ctx context.Context, fingerprint *EligibilityFingerprint, archetype SellerArchetype) ([]UngatingTarget, error)
}
```

### 7.3 EligibilitySampler Port

```go
// EligibilitySampler runs the 300-ASIN sampling against SP-API.
// Separated from AssessmentService so it can be tested independently
// and potentially swapped for different sampling strategies.
type EligibilitySampler interface {
    // RunSampling executes the full 300-ASIN eligibility scan.
    // Emits progress events via the provided callback.
    RunSampling(ctx context.Context, tenantID TenantID, marketplace string, onProgress func(checked, total int)) (*EligibilityFingerprint, error)

    // GetSampleASINs returns the curated ASIN set for a marketplace.
    GetSampleASINs(ctx context.Context, marketplace string) ([]AssessmentSampleASIN, error)
}
```

### 7.4 Repository Ports

```go
// SellerProfileRepo persists seller profiles.
type SellerProfileRepo interface {
    Create(ctx context.Context, profile *SellerProfile) error
    GetByTenantID(ctx context.Context, tenantID TenantID) (*SellerProfile, error)
    Update(ctx context.Context, profile *SellerProfile) error
}

// EligibilityFingerprintRepo persists fingerprint results.
type EligibilityFingerprintRepo interface {
    Create(ctx context.Context, fp *EligibilityFingerprint) error
    GetByTenantID(ctx context.Context, tenantID TenantID) (*EligibilityFingerprint, error)
    GetByID(ctx context.Context, id string) (*EligibilityFingerprint, error)
}

// StrategyBriefRepo persists strategy briefs.
type StrategyBriefRepo interface {
    Create(ctx context.Context, brief *StrategyBrief) error
    GetByTenantID(ctx context.Context, tenantID TenantID) (*StrategyBrief, error)
    GetByID(ctx context.Context, id string) (*StrategyBrief, error)
    ListByTenantID(ctx context.Context, tenantID TenantID) ([]StrategyBrief, error) // history
}

// AssessmentSampleRepo manages the curated ASIN registry.
type AssessmentSampleRepo interface {
    ListByMarketplace(ctx context.Context, marketplace string) ([]AssessmentSampleASIN, error)
    Upsert(ctx context.Context, sample *AssessmentSampleASIN) error
    UpsertBatch(ctx context.Context, samples []AssessmentSampleASIN) error
    Deactivate(ctx context.Context, asin string, marketplace string) error
}
```

---

## 8. Database Schema

### 8.1 New Tables

```sql
-- Seller profiles (one per tenant)
CREATE TABLE seller_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL UNIQUE REFERENCES tenants(id),
    seller_id TEXT NOT NULL DEFAULT '',
    marketplace TEXT NOT NULL DEFAULT 'US',
    account_age_days INT NOT NULL DEFAULT 0,
    active_listing_count INT NOT NULL DEFAULT 0,
    archetype TEXT NOT NULL DEFAULT 'greenhorn',
    archetype_confidence TEXT NOT NULL DEFAULT 'low',
    archetype_override TEXT,
    stated_capital NUMERIC(12,2),
    prior_experience TEXT,
    assessment_status TEXT NOT NULL DEFAULT 'pending',
    assessed_at TIMESTAMPTZ,
    next_reassess_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sp_status ON seller_profiles(assessment_status);

-- Eligibility fingerprints (one active per tenant, history retained)
CREATE TABLE eligibility_fingerprints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    seller_profile_id UUID NOT NULL REFERENCES seller_profiles(id),
    total_asins_checked INT NOT NULL DEFAULT 0,
    total_asins_eligible INT NOT NULL DEFAULT 0,
    overall_openness_score NUMERIC(4,3) NOT NULL DEFAULT 0.0,
    estimated_accessible_pct NUMERIC(5,2) NOT NULL DEFAULT 0.0,
    calibration_passed BOOLEAN NOT NULL DEFAULT false,
    category_results JSONB NOT NULL DEFAULT '[]',
    brand_results JSONB NOT NULL DEFAULT '[]',
    sampling_duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ef_tenant ON eligibility_fingerprints(tenant_id, created_at DESC);

-- Strategy briefs (one active per tenant, history retained)
CREATE TABLE strategy_briefs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    seller_profile_id UUID NOT NULL REFERENCES seller_profiles(id),
    fingerprint_id UUID NOT NULL REFERENCES eligibility_fingerprints(id),
    archetype TEXT NOT NULL,
    top_categories JSONB NOT NULL DEFAULT '[]',
    quick_win_brands JSONB NOT NULL DEFAULT '[]',
    ungating_targets JSONB NOT NULL DEFAULT '[]',
    capital_plan JSONB NOT NULL DEFAULT '{}',
    first_actions JSONB NOT NULL DEFAULT '[]',
    narrative_summary TEXT NOT NULL DEFAULT '',
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sb_tenant ON strategy_briefs(tenant_id, created_at DESC);

-- Curated ASIN registry for eligibility sampling
CREATE TABLE assessment_sample_asins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    asin TEXT NOT NULL,
    category TEXT NOT NULL,
    browse_node_id TEXT NOT NULL DEFAULT '',
    brand_name TEXT NOT NULL DEFAULT '',
    sample_tier TEXT NOT NULL,         -- tier1, tier2, tier3, calibration, brand
    sample_role TEXT NOT NULL,         -- top_brand, mid_brand, generic, niche, known_open, known_restricted
    marketplace TEXT NOT NULL DEFAULT 'US',
    active BOOLEAN NOT NULL DEFAULT true,
    last_verified TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(asin, marketplace)
);

CREATE INDEX idx_asa_marketplace ON assessment_sample_asins(marketplace, active)
    WHERE active = true;

-- RLS policies (standard pattern)
ALTER TABLE seller_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE eligibility_fingerprints ENABLE ROW LEVEL SECURITY;
ALTER TABLE strategy_briefs ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_sp ON seller_profiles
    USING (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE POLICY tenant_isolation_ef ON eligibility_fingerprints
    USING (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE POLICY tenant_isolation_sb ON strategy_briefs
    USING (tenant_id = current_setting('app.current_tenant')::uuid);
```

### 8.2 Migration

Single migration file: `migrations/NNNN_create_assessment_tables.up.sql`

The migration also seeds the `assessment_sample_asins` table with the initial 300-ASIN set for the US marketplace. The seed data is maintained in a separate SQL file (`migrations/seed_assessment_asins_us.sql`) loaded by the migration.

### 8.3 Schema Changes to Existing Tables

```sql
-- Add strategy_brief_id to discovery_configs
ALTER TABLE discovery_configs ADD COLUMN IF NOT EXISTS
    strategy_brief_id UUID REFERENCES strategy_briefs(id);

-- Add assessment preferences to tenant_settings
ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS
    assessment_auto_reassess BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE tenant_settings ADD COLUMN IF NOT EXISTS
    assessment_reassess_days INT NOT NULL DEFAULT 30;
```

---

## 9. Inngest Workflows

### 9.1 Account Assessment Workflow

```
Event: assessment/started
Trigger: User completes SP-API OAuth OR user requests reassessment

→ step: create-profile
    Create or update SellerProfile with status=running
    Pull account metadata via SP-API (account age, listing count)

→ step: run-eligibility-sampling (the long step, ~90-120s)
    Load 300 sample ASINs from registry
    Check ListingRestrictions in batches of 10 (rate-limited to 5/sec)
    Emit progress events every 30 ASINs: "assessment/progress" { checked, total }
    Write results to brand_eligibility cache (reuses existing table)
    Produce EligibilityFingerprint

→ step: classify-archetype
    Run decision tree on profile + fingerprint
    Update SellerProfile with archetype

→ step: generate-strategy
    Call StrategyEngine.GenerateBrief()
    Deterministic scoring for categories, brands, ungating targets
    LLM call for narrative synthesis and ungating instructions
    Persist StrategyBrief

→ step: configure-discovery
    Update DiscoveryConfig with eligible categories from strategy
    Set brand_eligibility entries for all sampled brands
    Create initial browse node scan rotation matching top categories

→ step: complete-assessment
    Update SellerProfile status=completed, assessed_at=now()
    Set next_reassess_at = now + reassess_days
    Emit domain event: "assessment/completed"
    Trigger notification to user: "Your strategy is ready"
```

### 9.2 Monthly Reassessment Workflow

```
Event: Inngest cron (daily at 06:00 UTC)
Logic: Query seller_profiles WHERE next_reassess_at <= now() AND assessment_auto_reassess = true

→ For each tenant due for reassessment:
    → Send "assessment/started" event (reuses the main workflow)
    → New fingerprint is compared with previous
    → Strategy Brief version increments
    → Differences highlighted: "You gained access to 3 new brands since last month"
```

### 9.3 Progress Events

During the sampling step, the workflow emits progress events that the frontend consumes via polling or server-sent events:

```
assessment/progress {
  tenant_id,
  phase: "sampling" | "classifying" | "generating_strategy" | "configuring",
  checked: 120,
  total: 300,
  current_category: "Health & Household",
  pct_complete: 40
}
```

---

## 10. API Endpoints

### 10.1 Assessment Endpoints

```
# Trigger assessment (called after SP-API OAuth completes)
POST   /assessment/start
  Body: { marketplace: "US", stated_capital?: 5000, prior_experience?: "6 months RA" }
  Response: 202 { seller_profile_id, status: "running" }

# Get assessment progress (polled by frontend during onboarding)
GET    /assessment/progress
  Response: 200 {
    status: "running",
    phase: "sampling",
    pct_complete: 47,
    current_category: "Health & Household"
  }

# Get seller profile
GET    /assessment/profile
  Response: 200 { SellerProfile }

# Get eligibility fingerprint
GET    /assessment/fingerprint
  Response: 200 { EligibilityFingerprint }

# Get strategy brief
GET    /assessment/strategy
  Response: 200 { StrategyBrief }

# Override archetype
PUT    /assessment/archetype
  Body: { archetype: "expanding_pro" }
  Response: 200 { SellerProfile (updated) }

# Trigger reassessment
POST   /assessment/reassess
  Response: 202 { seller_profile_id, status: "running" }

# Get strategy brief history
GET    /assessment/strategy/history
  Response: 200 { briefs: [StrategyBrief, ...] }
```

### 10.2 Authentication

All endpoints require the standard tenant authentication (JWT with tenant_id). The assessment is scoped to the authenticated tenant. No cross-tenant access.

---

## 11. Frontend — Onboarding Flow UX

### 11.1 The Four Screens

**Screen 1: Connect (route: `/onboarding/connect`)**

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  ┌────────────────────────────────────────┐          │
│  │        Connect Your Amazon Account     │          │
│  │                                        │          │
│  │  We'll analyze your selling privileges │          │
│  │  and build a strategy tailored to you. │          │
│  │                                        │          │
│  │  What we'll check:                     │          │
│  │  ✓ Which categories you can sell in    │          │
│  │  ✓ Which brands you're approved for    │          │
│  │  ✓ Your account maturity level         │          │
│  │                                        │          │
│  │  [Connect with Amazon]  ← SP-API OAuth │          │
│  └────────────────────────────────────────┘          │
│                                                      │
│  Optional: How much capital are you starting with?   │
│  [ $_______ ]                                        │
│                                                      │
│  How would you describe your experience?             │
│  ( ) Brand new to Amazon                             │
│  ( ) Been doing retail/online arbitrage              │
│  ( ) Experienced wholesale seller                    │
│  ( ) Have capital, new to wholesale                  │
│                                                      │
└──────────────────────────────────────────────────────┘
```

**Screen 2: Discover (route: `/onboarding/discover`)**

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Analyzing your account...                           │
│                                                      │
│  ████████████░░░░░░░░░  47%                          │
│                                                      │
│  Checking Health & Household...                      │
│                                                      │
│  ┌─────────────────────┬──────────────┐              │
│  │ Categories checked  │ 14 of 30     │              │
│  │ Brands sampled      │ 12 of 25     │              │
│  │ Open categories     │ 8 so far     │              │
│  │ Eligible brands     │ 6 so far     │              │
│  └─────────────────────┴──────────────┘              │
│                                                      │
│  As results come in, live-update the category        │
│  grid below (green = open, red = gated, gray = TBD) │
│                                                      │
│  [Home ✓] [Kitchen ✓] [Sports ✓] [Tools ✓]          │
│  [Grocery ✗] [Health ░] [Beauty ░] [Toys ░]          │
│  [Office ✓] [Patio ✓] [Auto ░] [Industrial ░]       │
│                                                      │
└──────────────────────────────────────────────────────┘
```

Polls `GET /assessment/progress` every 3 seconds. Category grid updates in real-time as results arrive. Estimated time remaining shown.

**Screen 3: Reveal (route: `/onboarding/strategy`)**

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Your Strategy Brief                                 │
│  ─────────────────                                   │
│                                                      │
│  You're an "RA-to-Wholesale" seller with access      │
│  to 62% of the wholesale catalog. Here's your plan.  │
│                                                      │
│  ┌─ TOP CATEGORIES ─────────────────────────────┐    │
│  │ 1. Home & Kitchen    Score: 87  [Start here] │    │
│  │ 2. Office Products   Score: 82  [Start here] │    │
│  │ 3. Sports & Outdoors Score: 78  [Start here] │    │
│  │ 4. Grocery           Score: 71  [Ungate next]│    │
│  │ 5. Health & Household Score: 68 [Long-term]  │    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│  ┌─ QUICK WINS (brands you can sell now) ───────┐    │
│  │ OXO (Kitchen) — ~40 products, ~24% margin    │    │
│  │ Rubbermaid (Home) — ~65 products, ~22% margin│    │
│  │ 3M (Office) — ~30 products, ~19% margin      │    │
│  │ + 2 more                                      │    │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│  ┌─ UNGATING ROADMAP ──────────────────────────┐     │
│  │ Month 1: Grocery (cost: ~$400, ROI: 84x)    │     │
│  │ Month 2: Health & Household (cost: ~$500)    │     │
│  │ Month 3: Toys & Games (for Q4 prep)          │     │
│  └──────────────────────────────────────────────┘    │
│                                                      │
│  ┌─ CAPITAL PLAN ($5,000) ─────────────────────┐     │
│  │ ████████████████░░░░░ Inventory: $3,000 (60%)│     │
│  │ ████░░░░░░░░░░░░░░░░ Ungating: $1,000 (20%) │     │
│  │ ██░░░░░░░░░░░░░░░░░░ Tools: $300 (6%)       │     │
│  │ ███░░░░░░░░░░░░░░░░░ Reserve: $700 (14%)    │     │
│  └──────────────────────────────────────────────┘    │
│                                                      │
└──────────────────────────────────────────────────────┘
```

**Screen 4: Commit (route: `/onboarding/commit`)**

```
┌──────────────────────────────────────────────────────┐
│                                                      │
│  Ready to start?                                     │
│                                                      │
│  Your discovery engine will scan:                    │
│  • 3 categories: Home & Kitchen, Office, Sports      │
│  • 5 priority brands: OXO, Rubbermaid, 3M, ...      │
│  • Nightly automatic scans                           │
│                                                      │
│  "Not quite right?"                                  │
│  [Edit categories]  [Change archetype]               │
│                                                      │
│  [ Start My Plan ]                                   │
│                                                      │
│  Or upload a price list to get immediate results:    │
│  [ Upload Price List ]                               │
│                                                      │
└──────────────────────────────────────────────────────┘
```

### 11.2 Post-Onboarding: Strategy Tab in Dashboard

After onboarding, the Strategy Brief is accessible from the main dashboard as a persistent "Strategy" tab:

```
/dashboard/strategy
  Strategy Brief card (current version)
  Archetype badge with "Change" option
  Category score table (sortable)
  Quick-win brands list
  Ungating roadmap timeline
  Capital allocation chart
  "Reassess My Account" button
  History: previous strategy versions (diffable)
```

---

## 12. Integration with Existing Discovery Engine

### 12.1 How Strategy Directs Discovery

The Strategy Brief directly configures the discovery engine to avoid wasting resources on ineligible products:

```
Assessment completes
  │
  ├── DiscoveryConfig.categories ← top_categories[].category_name
  │     (only eligible categories are scanned)
  │
  ├── DiscoveryConfig.strategy_brief_id ← brief.id
  │     (links discovery to current strategy)
  │
  ├── brand_eligibility table ← fingerprint brand results
  │     (T2 in the funnel uses these cached results — 0 extra API calls)
  │
  └── browse_nodes scan queue ← prioritized by category_score
        (nightly scan hits top-scoring nodes first)
```

### 12.2 Discovery Funnel Optimizations

The assessment pre-populates data that the discovery funnel (spec: 2026-04-07) uses at T2 (Brand Gate):

- **Before assessment:** T2 encounters unknown brands, queues SP-API calls, ~500 calls per scan
- **After assessment:** T2 finds cached eligibility for the 25 top brands + all brands sampled per category. Cache hit rate jumps from 0% to ~60-70% for the first scan. Subsequent scans compound further.

The funnel also benefits from strategy-directed scanning:

- **Before assessment:** Category scan rotates through all browse nodes indiscriminately
- **After assessment:** Category scan prioritizes nodes from `top_categories` in the Strategy Brief. A Greenhorn seller's nightly scan covers 3 open categories deeply rather than 30 categories superficially.

### 12.3 Price List Scans

When a user uploads a price list, the funnel can reference the fingerprint to immediately filter:

```
Price list uploaded
  → T1 (margin math — unchanged)
  → T2 (brand gate):
      For each product's brand:
        1. Check brand_eligibility cache (populated by assessment)
        2. If hit → use cached result (no API call)
        3. If miss → check SP-API, cache result (as before)
      Assessment reduces T2 API calls by ~60-70%
```

### 12.4 Reassessment Loop

As the seller uses the platform, new brand eligibility data accumulates from price list scans and category scans. Monthly reassessment incorporates this data:

```
Month 1: Assessment samples 300 ASINs → fingerprint covers 30 categories, 25 brands
Month 1-2: Price list scans check 200 additional brands → brand_eligibility grows
Month 2: Reassessment runs → fingerprint now informed by 500+ data points
          Strategy Brief v2 may recommend new categories that became accessible
          Discovery engine reconfigured automatically
```

---

## 13. Implementation Phases

| Phase | What | Delivers | Depends On |
|---|---|---|---|
| **1** | Domain models + migrations + repos | `seller_profiles`, `eligibility_fingerprints`, `strategy_briefs`, `assessment_sample_asins` tables. Go domain types. Repository implementations. | None |
| **2** | Eligibility sampler service | `EligibilitySampler` implementation using existing `BrandEligibilityService` and `ProductSearcher`. ASIN registry seeded. 300-ASIN scan working end-to-end. | Phase 1 |
| **3** | Archetype classifier + strategy engine | Deterministic archetype classification. Category scoring formula. Quick-win brand identification. Ungating roadmap builder. LLM narrative generation (calls existing agent pattern). | Phase 2 |
| **4** | Assessment service + Inngest workflow | `AssessmentService` implementation. Inngest workflow wiring (start, progress, complete). Progress event emission. | Phase 3 |
| **5** | API endpoints + discovery integration | REST endpoints for assessment lifecycle. `DiscoveryConfig` updated from strategy. Brand eligibility cache populated. Browse node queue prioritized. | Phase 4 |
| **6** | Frontend — onboarding flow | Connect, Discover, Reveal, Commit screens. Progress polling. Strategy Brief display. Strategy tab in dashboard. | Phase 5 |

Estimated total effort: 3-4 weeks (1 developer).

Phase 1-2 can be developed in parallel with other discovery engine work. Phase 5 is the critical integration point where assessment output flows into the existing funnel.

---

## 14. What's NOT in This Spec

- **Account health metrics** — SP-API does not expose account health (ODR, late shipment rate) directly. The archetype classifier uses listing count as a proxy. If SP-API adds health endpoints, reassessment can incorporate them.
- **Growth Plan / Quest Log** — the expert research describes a GrowthPlan with milestones and tasks. That is a separate feature built on top of the Strategy Brief. This spec covers the assessment and brief only.
- **Distributor matching** — the ungating roadmap recommends *which* categories/brands to ungate and estimates cost, but does not connect sellers to specific distributors. Distributor CRM is Phase 2 of the main roadmap.
- **Multi-marketplace assessment** — the spec describes a single-marketplace assessment (US). EU/UK use the same architecture with different ASIN registries. Multi-marketplace support is deferred.
- **Automated ungating** — the platform recommends ungating targets but does not file applications or submit invoices. All ungating is manual with platform guidance.
- **AI chat advisor** — the expert research describes an AI advisor that answers "why did you recommend this brand?" That reactive chat interface is a separate feature.
- **Competitive intelligence** — the fingerprint shows what *this* seller can access but does not compare with other sellers' eligibility profiles.
- **Pricing tier gating** — the assessment runs for all tiers. Gating features behind pricing tiers (e.g., ungating roadmap only for Concierge tier) is a product decision, not an architecture decision.
- **Historical strategy diff UI** — strategy brief versioning is in the schema but the diff visualization ("you gained 3 new brands since last month") is deferred to a future frontend iteration.

---

## 15. Open Questions

1. **Calibration failure handling** — if the 5 known-open calibration ASINs come back as restricted, the SP-API credentials may be invalid or the account is suspended. Should the assessment abort, or continue with a warning?
2. **ASIN registry maintenance** — who curates the 300 sample ASINs? Manual curation by the team, or automated selection from the discovered_products catalog based on brand/category coverage?
3. **Reassessment delta** — when reassessing, should we re-check all 300 ASINs or only the ones that were previously restricted (to detect newly gained access)?
4. **Strategy Brief LLM model** — the narrative synthesis and ungating instructions are LLM-generated. Which model (GPT-4o, Claude, etc.) and what is the acceptable cost per brief generation?
5. **Capital input** — stated_capital is optional. If not provided, should the strategy omit the capital plan entirely, or use a default assumption ($5K)?
6. **Brand sampling evolution** — the initial 25 brands are hardcoded in the registry. Should the system learn which brands to sample based on which distributors the seller works with?
