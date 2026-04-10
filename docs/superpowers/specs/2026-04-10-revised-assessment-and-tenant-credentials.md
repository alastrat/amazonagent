# Revised Account Assessment & Amazon Seller Account Integration — Design Spec

**Date:** 2026-04-10
**Status:** Draft — pending review
**Scope:** Replace the broken assessment system (hardcoded ASIN probes, global SP-API credentials, blank Reveal step) with per-tenant credential storage, a discovery-oriented assessment that finds products users CAN sell, and a frontend that delivers actionable next steps
**Supersedes:** [Account Assessment Service (2026-04-08)](2026-04-08-account-assessment-service.md)
**Research:** [AI Concierge Expert Brainstorm](../research/2026-04-08-ai-concierge-expert-brainstorm.md), [Continuous Learning Architecture](../research/2026-04-08-continuous-learning-architecture.md)

---

## 1. Problems With the Current System

The current assessment is broken in four concrete ways:

### 1.1 Global SP-API Credentials

The SP-API client (`internal/adapter/spapi/client.go`) reads credentials from environment variables at startup. There is one set of credentials for the entire system. This means:

- Every tenant's eligibility checks run against the *platform operator's* seller account, not the tenant's own account
- Eligibility results are meaningless — they reflect our restrictions, not theirs
- We cannot onboard real customers without giving them access to our credentials or vice versa

### 1.2 Hardcoded ASIN Probes Report "All Rejected"

The assessment probes 10 hardcoded ASINs (via `AssessmentProbe` structs). In practice, most or all come back restricted because:

- The probe set is tiny and skewed toward gated products
- The probes do not adapt to the seller's actual eligibility surface
- Result: the Reveal step shows "0 eligible products" — which tells the user nothing useful

### 1.3 Blank Categories and Meaningless Strategy

The Reveal step displays empty category tables and lets the user "approve" a strategy that has no substance. The strategy is generated from an empty fingerprint. There are no real product recommendations, no actual margins, no concrete actions.

### 1.4 No Practical Next Step

After completing onboarding, the user lands on a dashboard with nothing to do. No products to source. No categories to explore. No ungating roadmap. The "approved strategy" has no connection to the discovery engine.

---

## 2. What This Spec Changes

Three interlocking changes that fix the onboarding experience end-to-end:

| Part | What | Why |
|---|---|---|
| **Part 1** | Per-tenant SP-API credential storage | Eligibility checks must run against the seller's own account |
| **Part 2** | Discovery-oriented assessment loop | Find products the user CAN sell profitably, not just report restrictions |
| **Part 3** | Revised frontend with actionable outcomes | Deliver real product recommendations or a concrete ungating roadmap |

---

## 3. Part 1 — Amazon Seller Account Connection

### 3.1 New Table: `amazon_seller_accounts`

Each tenant connects one Amazon Seller Account. The table stores the SP-API credentials needed to make API calls on behalf of THAT specific seller.

```sql
CREATE TABLE amazon_seller_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL UNIQUE REFERENCES tenants(id) ON DELETE CASCADE,

    -- SP-API credentials (encrypted at rest)
    sp_api_client_id     TEXT NOT NULL,
    sp_api_client_secret TEXT NOT NULL,
    sp_api_refresh_token TEXT NOT NULL,
    seller_id            TEXT NOT NULL,
    marketplace_id       TEXT NOT NULL DEFAULT 'ATVPDKIKX0DER', -- US marketplace

    -- Credential health
    status          TEXT NOT NULL DEFAULT 'pending', -- pending | valid | invalid | expired
    last_verified   TIMESTAMPTZ,
    error_message   TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- RLS policy: tenants can only read their own credentials
ALTER TABLE amazon_seller_accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY amazon_seller_accounts_isolation ON amazon_seller_accounts
    USING (tenant_id = current_setting('app.tenant_id')::uuid);

-- Encrypt sensitive columns using Supabase Vault (pgsodium)
-- Alternatively: app-level encryption via Go's crypto/aes before storage
COMMENT ON COLUMN amazon_seller_accounts.sp_api_client_secret IS 'Encrypted at rest via app-level AES-256-GCM';
COMMENT ON COLUMN amazon_seller_accounts.sp_api_refresh_token IS 'Encrypted at rest via app-level AES-256-GCM';
```

### 3.2 Encryption Strategy

App-level encryption using AES-256-GCM, not Supabase column encryption. Reasons:

- **Portability** — encryption/decryption happens in Go, not tied to Postgres extensions
- **Key management** — encryption key stored as a single env var (`CREDENTIAL_ENCRYPTION_KEY`), rotatable
- **Defense in depth** — even a full DB dump does not expose credentials

The encryption flow:

```
Store: plaintext → AES-256-GCM encrypt(key, nonce) → base64 → write to DB
Load:  read from DB → base64 decode → AES-256-GCM decrypt(key, nonce) → plaintext
```

### 3.3 Domain Model

```go
// internal/domain/amazon_seller_accounts.go

type AmazonSellerAccount struct {
    ID               string    `json:"id"`
    TenantID         TenantID  `json:"tenant_id"`
    SPAPIClientID    string    `json:"sp_api_client_id"`
    SPAPIClientSecret string   `json:"-"` // never serialized
    SPAPIRefreshToken string   `json:"-"` // never serialized
    SellerID         string    `json:"seller_id"`
    MarketplaceID    string    `json:"marketplace_id"`
    Status           string    `json:"status"` // pending | valid | invalid | expired
    LastVerified     *time.Time `json:"last_verified,omitempty"`
    ErrorMessage     string    `json:"error_message,omitempty"`
    CreatedAt        time.Time `json:"created_at"`
    UpdatedAt        time.Time `json:"updated_at"`
}
```

### 3.4 Port Interface

```go
// internal/port/credential_repo.go

type TenantCredentialRepo interface {
    Create(ctx context.Context, creds *domain.AmazonSellerAccount) error
    Get(ctx context.Context, tenantID domain.TenantID) (*domain.AmazonSellerAccount, error)
    Update(ctx context.Context, creds *domain.AmazonSellerAccount) error
    Delete(ctx context.Context, tenantID domain.TenantID) error
    UpdateStatus(ctx context.Context, tenantID domain.TenantID, status string, errMsg string) error
}
```

### 3.5 Per-Tenant SP-API Client Construction

The current `spapi.NewClient(clientID, clientSecret, refreshToken, marketplace, sellerID)` constructor already accepts credentials as parameters — it does not read env vars internally. The change is in the *call site*: instead of passing env vars, we load credentials from `amazon_seller_accounts` and construct a client per tenant.

```go
// internal/service/credential_service.go

type CredentialService struct {
    repo      port.TenantCredentialRepo
    encryptor *crypto.AESEncryptor
}

// GetSPAPIClient builds an SP-API client for a specific tenant.
func (s *CredentialService) GetSPAPIClient(ctx context.Context, tenantID domain.TenantID) (*spapi.Client, error) {
    creds, err := s.repo.Get(ctx, tenantID)
    if err != nil {
        return nil, fmt.Errorf("load tenant credentials: %w", err)
    }

    // Decrypt secrets
    clientSecret, err := s.encryptor.Decrypt(creds.SPAPIClientSecret)
    if err != nil {
        return nil, fmt.Errorf("decrypt client secret: %w", err)
    }
    refreshToken, err := s.encryptor.Decrypt(creds.SPAPIRefreshToken)
    if err != nil {
        return nil, fmt.Errorf("decrypt refresh token: %w", err)
    }

    marketplace := marketplaceFromID(creds.MarketplaceID)
    return spapi.NewClient(creds.SPAPIClientID, clientSecret, refreshToken, marketplace, creds.SellerID), nil
}
```

### 3.6 Seed Migration

A migration seeds the existing env-var credentials as the test tenant's credentials:

```sql
-- Migration: seed_test_amazon_seller_accounts.sql
-- Only runs if the test tenant exists and has no credentials yet
INSERT INTO amazon_seller_accounts (tenant_id, sp_api_client_id, sp_api_client_secret, sp_api_refresh_token, seller_id, marketplace_id, status)
SELECT
    id,
    current_setting('app.sp_api_client_id', true),
    current_setting('app.sp_api_client_secret', true),
    current_setting('app.sp_api_refresh_token', true),
    current_setting('app.seller_id', true),
    'ATVPDKIKX0DER',
    'valid'
FROM tenants
WHERE id = current_setting('app.test_tenant_id', true)::uuid
ON CONFLICT (tenant_id) DO NOTHING;
```

In practice, this migration will be run manually for the test tenant with the encryption applied at the app layer. The SQL above illustrates the intent; the actual seeding will happen via a Go script that encrypts the values before insertion.

### 3.7 Future: OAuth Flow

When the Amazon app is published and approved, onboarding Step 1 becomes a "Connect with Amazon" button that initiates the OAuth redirect flow. The OAuth callback writes to `amazon_seller_accounts` the same way the manual form does. The rest of the system is unchanged — it reads from `amazon_seller_accounts` regardless of how the credentials got there.

This spec does NOT implement OAuth. It prepares the credential storage layer that OAuth will write to later.

---

## 4. Part 2 — Revised Assessment: Find What Users CAN Sell

### 4.1 Philosophy Change

The current assessment asks: "What percentage of products is this seller restricted from?"
The revised assessment asks: "What products can this seller sell profitably right now?"

This is a fundamental shift. The assessment is not a diagnostic — it is a discovery operation. It searches for opportunity, not restriction.

### 4.2 The Discovery Loop

The assessment runs in three phases:

```
Phase 1: Broad Category Search
  20 categories x 20 products each = 400 catalog searches
  Check eligibility on each product = up to 400 restriction checks
  Result: map of eligible ASINs, brands, and categories with open rates

Phase 2: Evaluate Eligible Products
  Take all eligible products from Phase 1
  Run through existing funnel T1-T3 (margin filter, competitive pricing)
  Filter: margin > 15%, sellers >= 2
  Result: qualified products the user can sell profitably

Phase 3: Build Strategy From Real Data
  IF qualified products found → sourcing strategy with real targets
  IF nothing found → ungating roadmap with specific actions
```

### 4.3 Phase 1 — Broad Category Search

#### Category List (20 High-Value Wholesale Categories)

These are the Amazon top-level browse nodes with the highest wholesale relevance, ordered by expected open rate for new accounts:

| # | Category | Browse Node ID | Expected Open Rate |
|---|---|---|---|
| 1 | Home & Kitchen | 1055398 | ~85% |
| 2 | Tools & Home Improvement | 228013 | ~80% |
| 3 | Office Products | 1064954 | ~80% |
| 4 | Sports & Outdoors | 3375251 | ~75% |
| 5 | Patio, Lawn & Garden | 2972638 | ~70% |
| 6 | Automotive | 15684181 | ~70% |
| 7 | Arts, Crafts & Sewing | 2617941011 | ~70% |
| 8 | Industrial & Scientific | 16310091 | ~65% |
| 9 | Pet Supplies | 2619533011 | ~65% |
| 10 | Musical Instruments | 11091801 | ~65% |
| 11 | Toys & Games | 165793011 | ~60% |
| 12 | Baby Products | 165796011 | ~55% |
| 13 | Kitchen & Dining | 284507 | ~55% |
| 14 | Clothing, Shoes & Jewelry | 7141123011 | ~50% |
| 15 | Electronics | 172282 | ~45% |
| 16 | Cell Phones & Accessories | 2335752011 | ~45% |
| 17 | Grocery & Gourmet Food | 16310101 | ~30% |
| 18 | Beauty & Personal Care | 3760911 | ~25% |
| 19 | Health & Household | 3760901 | ~20% |
| 20 | Video Games | 468642 | ~40% |

#### Search Strategy Per Category

For each category:

1. Call `SearchByBrowseNode(nodeID, marketplace, "")` to get 20 products
2. For each product returned, call `CheckListingEligibility(asin, marketplace)` using the tenant's own SP-API credentials
3. Record result: ASIN, brand, category, eligible (yes/no), reason if restricted
4. Track running totals: eligible count, open brands, open categories

#### What Gets Recorded

```go
// Per-ASIN result during assessment
type AssessmentSearchResult struct {
    ASIN           string
    Title          string
    Brand          string
    Category       string
    AmazonPrice    float64
    BSRRank        int
    SellerCount    int
    Eligible       bool
    RestrictionReason string
}
```

After Phase 1 completes, we have:

- A list of eligible ASINs (typically 100-250 out of 400 searched)
- A set of open brands (brands where at least 1 ASIN is eligible)
- Category-level open rates (what percentage of products in each category are eligible)
- A set of restriction patterns (which categories/brands are fully gated)

### 4.3.1 Circuit Breakers

The discovery loop MUST have circuit breakers to prevent runaway API consumption and infinite cycling. These are non-negotiable safety mechanisms:

#### Per-Category Circuit Breaker
```
For each category:
  IF first 5 products are ALL restricted → skip remaining 15 in this category
  Rationale: if 5 random products from catalog search are all gated,
  the category is likely fully restricted. Don't waste 15 more API calls.
```

#### Early Success Exit
```
IF we've found >= 50 eligible products across all categories → stop searching
Rationale: 50 eligible products is more than enough to build a strategy.
No need to scan all 20 categories.
```

#### Total API Call Budget
```
Hard cap: 600 SP-API calls per assessment (search + eligibility combined)
IF budget exhausted → stop, build strategy from whatever we've found
Log: "Assessment budget exhausted after X calls, Y eligible products found"
```

#### Time Budget
```
Hard cap: 5 minutes wall-clock time per assessment
IF exceeded → stop, build strategy from whatever we've found
Prevents Inngest function timeout and user waiting indefinitely
```

#### Repeated Failure Detection
```
IF 3 consecutive categories yield 0 eligible products → switch strategy:
  Skip remaining mid/low open-rate categories
  Jump to highest expected open-rate categories not yet scanned
Prevents wasting budget on a heavily restricted account
```

#### Zero Results Safety
```
IF zero eligible products found after all categories scanned:
  Do NOT loop again with different parameters
  Instead → immediately go to "ungating roadmap" outcome
  Log: "Assessment complete: 0 eligible products, account needs ungating"
```

All circuit breaker activations are logged and recorded in the assessment metadata so the platform can learn which accounts tend to be restricted and adjust scanning strategy over time.

### 4.4 Phase 2 — Evaluate Eligible Products

Take all eligible ASINs from Phase 1 and run them through the existing funnel tiers T1-T3:

**T1 — Local Math (Margin Filter):**
- Estimate wholesale cost at 40% of retail (no price list available yet during onboarding)
- Calculate FBA fees using `domain.CalculateFBAFees`
- Kill products with estimated net margin < 15%
- Kill products outside $10-$200 price range

**T2 — Brand Gate (Skip):**
- Already confirmed eligible in Phase 1 — no need to re-check

**T3 — Competitive Pricing Enrichment:**
- Batch `GetProductDetails` for real Buy Box price and seller count
- Kill products with seller count < 2 (no competitive baseline)
- Recalculate margin with real Buy Box price
- Kill products where real margin < 15%

The funnel service (`internal/service/funnel_service.go`) already implements this logic. The assessment wraps it:

```go
// Convert eligible AssessmentSearchResults to FunnelInputs
var funnelInputs []FunnelInput
for _, r := range eligibleProducts {
    funnelInputs = append(funnelInputs, FunnelInput{
        ASIN:           r.ASIN,
        Title:          r.Title,
        Brand:          r.Brand,
        Category:       r.Category,
        EstimatedPrice: r.AmazonPrice,
        WholesaleCost:  0, // estimate at 40% in funnel
        BSRRank:        r.BSRRank,
        SellerCount:    r.SellerCount,
        Source:         domain.ScanTypeAssessment,
    })
}

survivors, stats, err := funnelService.ProcessBatch(ctx, tenantID, funnelInputs, thresholds)
```

### 4.5 Phase 3 — Build Strategy From Real Data

Two possible outcomes:

#### Outcome A: Opportunities Found (survivors > 0)

The strategy is built from real data:

```go
type AssessmentOutcome struct {
    HasOpportunities     bool
    QualifiedProducts    []FunnelSurvivor  // products passing T1-T3
    EligibleCategories   []CategorySummary // categories with eligible products
    OpenBrands           []string          // brands with at least 1 eligible ASIN
    TopRecommendations   []ProductRecommendation // top 10 by estimated margin
    StrategyGoals        []StrategyGoal    // revenue targets based on opportunity size
    EstimatedMonthlyRev  float64           // based on qualified product count and avg margin
}

type CategorySummary struct {
    Category      string
    EligibleCount int
    QualifiedCount int   // passed funnel
    AvgMarginPct  float64
    OpenRate      float64
}

type ProductRecommendation struct {
    ASIN          string
    Title         string
    Brand         string
    Category      string
    BuyBoxPrice   float64
    EstMarginPct  float64
    SellerCount   int
    BSRRank       int
}
```

Strategy goals are derived from real opportunity:

```
Example:
  15 qualified products found across 4 categories
  Average estimated margin: 22%
  Best categories: Home & Kitchen (8 products), Office (4 products), Sports (3 products)

  Goal 1: "List first 5 products from Home & Kitchen by [+14 days]"
  Goal 2: "Reach $2,000/mo revenue by [+30 days]"
  Goal 3: "Expand to 15 listed products across 3 categories by [+60 days]"
```

The discovery engine is configured to scan these specific categories and brands daily.

#### Outcome B: Nothing Found (survivors == 0)

The strategy becomes an ungating roadmap:

```go
type UngatingRoadmap struct {
    RestrictedCategories []RestrictedCategory
    RecommendedPath      []UngatingStep
    EstimatedTimeline    string // e.g., "30-60 days to first eligible products"
}

type RestrictedCategory struct {
    Category    string
    OpenRate    float64 // how many ASINs were eligible (even if not profitable)
    Difficulty  string  // easy | medium | hard
}

type UngatingStep struct {
    Order       int
    Category    string
    Action      string  // e.g., "Apply for Grocery ungating via KeHE invoice"
    Difficulty  string
    EstDays     int     // estimated days to complete
    Impact      string  // e.g., "Unlocks ~47 profitable ASINs"
}
```

The ungating ladder (from expert research) prioritizes:

1. **Grocery** — easiest gate, highest replenishment, overlapping distributors
2. **Health & Household** — overlaps with Grocery distributors, 20-30% margins
3. **Beauty** — higher margins (30-40%), but brand-gating is the real barrier
4. **Toys & Games** — seasonal opportunity (Q4), moderate difficulty

For each recommended category, the system provides:

- Which distributors to contact (e.g., KeHE for Grocery, UNFI as backup)
- What documents are needed (business license, resale certificate, invoices)
- Estimated timeline (e.g., "KeHE account approval: 5-10 business days")
- Expected impact (e.g., "Grocery ungating typically unlocks 30-50 profitable ASINs")

Strategy goal for the restricted case:

```
Goal 1: "Get ungated in Grocery within 30 days"
Goal 2: "List first 5 Grocery products by [+45 days]"
Goal 3: "Reach $1,000/mo revenue by [+60 days]"
```

### 4.6 Inngest Workflow

The assessment runs as an Inngest workflow with discrete, retryable steps:

```go
inngest.CreateFunction(
    inngest.FunctionOpts{
        ID:   "assessment/run-discovery",
        Name: "Run Account Assessment",
    },
    inngest.EventTrigger("assessment/started"),
    func(ctx context.Context, input inngest.Input) (any, error) {
        tenantID := input.Event.Data["tenant_id"]

        // Step 1: Validate credentials
        _, err := step.Run(ctx, "validate-credentials", func(ctx context.Context) (any, error) {
            // Build SP-API client from tenant credentials
            // Call a lightweight endpoint to verify access works
            // Update credential status to "valid" or "invalid"
        })

        // Step 2: Search categories (20 categories x 20 products)
        searchResults, err := step.Run(ctx, "search-categories", func(ctx context.Context) (any, error) {
            // For each of 20 categories:
            //   SearchByBrowseNode → collect up to 20 products
            // Emit progress events every 5 categories
            // Return: all found products (up to 400)
        })

        // Step 3: Check eligibility on found products
        eligibilityResults, err := step.Run(ctx, "check-eligibility", func(ctx context.Context) (any, error) {
            // For each product from Step 2:
            //   CheckListingEligibility using tenant's seller_id
            // Rate limited at ~3.5/sec (conservative)
            // Emit progress events every 50 checks
            // Return: products annotated with eligible/restricted
        })

        // Step 4: Evaluate profitability (funnel T1-T3)
        funnelResults, err := step.Run(ctx, "evaluate-profitability", func(ctx context.Context) (any, error) {
            // Filter eligible products through funnel
            // T1: margin > 15%, price $10-$200
            // T3: real Buy Box price, seller count >= 2
            // Return: qualified survivors
        })

        // Step 5: Build eligibility fingerprint
        _, err = step.Run(ctx, "build-fingerprint", func(ctx context.Context) (any, error) {
            // Aggregate results into EligibilityFingerprint
            // Save to DB with category open rates, brand results
        })

        // Step 6: Generate strategy
        _, err = step.Run(ctx, "generate-strategy", func(ctx context.Context) (any, error) {
            // IF survivors > 0: build sourcing strategy with real products
            // IF survivors == 0: build ungating roadmap
            // Save strategy brief to DB
        })

        // Step 7: Mark complete
        _, err = step.Run(ctx, "complete-assessment", func(ctx context.Context) (any, error) {
            // Update seller_profile.assessment_status = "completed"
            // Emit "assessment/completed" event
        })

        return nil, nil
    },
)
```

### 4.7 SP-API Budget and Timing

| Step | Endpoint | Max Calls | Rate Limit | Est. Time |
|---|---|---|---|---|
| validate-credentials | Auth token request | 1 | N/A | ~1s |
| search-categories | CatalogItems (SearchByBrowseNode) | 20 | 2/sec | ~10s |
| check-eligibility | ListingRestrictions | ~400 | 3.5/sec | ~115s |
| evaluate-profitability | CompetitivePricing (GetProductDetails) | ~20 batches of 20 | 10/sec | ~10s |
| **Total** | | **~440-460** | | **~136s (~2.5 min)** |

The eligibility checks are the bottleneck. At 3.5 requests/second (conservative — documented limit is 5/sec but we budget for retries), 400 checks take ~115 seconds. Everything else is fast.

**Target: < 3 minutes total.**

### 4.8 Credit Cost

The assessment is **FREE for all new users**. It is an onboarding gift, not charged from the credit balance. Rationale:

- The assessment is the first-value moment — charging for it creates friction before the user has seen any value
- Estimated cost per assessment: ~400-600 SP-API calls, minimal compute
- The assessment directly drives conversion: a user who sees real product recommendations is far more likely to subscribe

### 4.9 Assessment Service Changes

The `AssessmentService` in `internal/service/assessment_service.go` changes significantly:

**Current signature:**
```go
func (s *AssessmentService) StartAssessment(ctx, tenantID, accountAgeDays, activeListings, statedCapital)
```

**New signature:**
```go
func (s *AssessmentService) StartAssessment(ctx, tenantID)
```

Account age, listings, and capital are no longer collected as a form. The assessment infers the seller's situation from the SP-API data and eligibility results. Archetype classification moves to post-assessment (after we have real data) rather than pre-assessment (from user-reported numbers).

**Removed:**
- `AssessmentProbe` struct and hardcoded probe dataset — replaced by dynamic category search
- Manual account_age / listings / capital input — inferred from SP-API or deferred

**Added:**
- `CredentialService` dependency — to build per-tenant SP-API client
- `FunnelService` dependency — to run T1-T3 on eligible products
- `AssessmentOutcome` / `UngatingRoadmap` result types
- Progress event emission for real-time frontend updates

---

## 5. Part 3 — Revised Frontend

### 5.1 Onboarding Step 1: Connect

**Current:** Form with account_age, active_listings, stated_capital fields.

**Revised:** SP-API credential input form.

```
┌─────────────────────────────────────────────────┐
│  Connect Your Amazon Seller Account              │
│                                                   │
│  To analyze your account, we need your SP-API     │
│  credentials. You can find these in your Amazon   │
│  Seller Central developer settings.               │
│                                                   │
│  SP-API Client ID       [________________________]│
│  SP-API Client Secret   [________________________]│
│  Refresh Token          [________________________]│
│  Seller ID              [________________________]│
│                                                   │
│  [?] How do I find these credentials?             │
│                                                   │
│  [ Connect Account ]                              │
│                                                   │
│  ────────────────────────────────────────────     │
│  Coming soon: Connect with Amazon (one-click)     │
└─────────────────────────────────────────────────┘
```

On submit:

1. Encrypt secrets and store in `amazon_seller_accounts`
2. Call SP-API token endpoint to validate credentials
3. If valid: set status = "valid", proceed to Step 2
4. If invalid: show error, let user re-enter

### 5.2 Onboarding Step 2: Discover

**Current:** Spinner with "Running assessment..." and no detail.

**Revised:** Live progress showing category-by-category search with running totals.

```
┌─────────────────────────────────────────────────┐
│  Discovering Your Opportunities                   │
│                                                   │
│  ████████████████████░░░░░░░░░░  65%              │
│                                                   │
│  Searching Office Products...                     │
│                                                   │
│  ✓ Home & Kitchen          12/20 eligible         │
│  ✓ Tools & Home Improvement 15/20 eligible        │
│  ✓ Office Products          8/20 eligible         │
│  ◌ Sports & Outdoors        searching...          │
│  ○ Patio, Lawn & Garden     queued                │
│  ...                                              │
│                                                   │
│  ──────────────────────────────────────────       │
│  Running totals:                                  │
│  📦 Products found: 260                           │
│  ✓  Eligible: 142                                 │
│  💰 Profitable: 23 (checking...)                  │
│                                                   │
│  Estimated time remaining: ~1 minute              │
└─────────────────────────────────────────────────┘
```

Progress updates arrive via server-sent events or polling the assessment status endpoint. The Inngest workflow emits progress events at each category completion.

### 5.2.1 Discovery Graph Visualization

The assessment is fundamentally a **graph exploration** — traversing Amazon's category → brand → product hierarchy and probing eligibility at each node. The frontend should visualize this as an interactive graph that builds in real-time as the scan progresses.

#### Data Structure (Graph Nodes and Edges)

```
Graph structure being explored:

  Amazon Marketplace (root)
    ├── Category: Home & Kitchen
    │     ├── Brand: Rubbermaid → [eligible]
    │     │     ├── ASIN: B002YK46UQ → eligible, $24.99, 22% margin
    │     │     └── ASIN: B00004OCKR → eligible, $12.99, 18% margin
    │     ├── Brand: KitchenAid → [restricted]
    │     │     └── ASIN: B0XX... → restricted: "brand approval required"
    │     └── Brand: OXO → [eligible]
    │           └── ASIN: B0YY... → eligible, $19.99, 25% margin
    ├── Category: Office Products
    │     ├── Brand: 3M → [eligible]
    │     └── Brand: Post-it → [restricted]
    ├── Category: Beauty → [fully restricted]
    │     └── (all products gated)
    ...
```

#### Visualization Component: `<DiscoveryGraph />`

A real-time animated graph that renders during the Discover step and persists in the Reveal step.

**Layout:** Force-directed or radial tree layout with three levels:
- **Level 0 (center):** Marketplace node
- **Level 1 (ring 1):** Category nodes — color-coded by open rate (green = mostly open, red = mostly restricted, gray = not yet scanned)
- **Level 2 (ring 2):** Brand nodes — only shown for categories that have been scanned. Green = eligible, Red = restricted
- **Level 3 (outer):** Product nodes — only shown on hover/click of a brand. Shows ASIN, price, margin

**Real-time animation during Discover step:**
1. Categories start as gray nodes in a ring around the center
2. As each category is scanned, it animates: gray → scanning (pulsing blue) → result (green/yellow/red based on open rate)
3. Brand nodes appear as children of the category being scanned
4. Eligible brands pulse green briefly, restricted brands appear red
5. Running counters update: "12 categories scanned, 47 eligible products, 8 open brands"

**Interaction in Reveal step (static, explorable):**
- Click a category → expands to show brands
- Click a brand → expands to show products with price/margin
- Hover any node → tooltip with details
- Color legend: green = eligible, red = restricted, gray = not scanned, yellow = partially open

**Technical implementation:**
- Use a lightweight graph library: `react-force-graph-2d` (48KB, canvas-based, handles 500+ nodes) or `d3-force` directly
- Data fed from the assessment status endpoint which returns the graph structure progressively
- The `EligibilityFingerprint` type already has `categories[]` and `brand_results[]` — just needs to be structured as nodes/edges for the graph

**API response structure for graph:**
```json
{
  "graph": {
    "nodes": [
      {"id": "marketplace", "type": "root", "label": "Amazon US"},
      {"id": "cat-home", "type": "category", "label": "Home & Kitchen", "open_rate": 75, "status": "scanned"},
      {"id": "brand-rubbermaid", "type": "brand", "label": "Rubbermaid", "eligible": true, "category": "cat-home"},
      {"id": "B002YK46UQ", "type": "product", "label": "Storage Container", "eligible": true, "price": 24.99, "margin": 22.1, "brand": "brand-rubbermaid"}
    ],
    "edges": [
      {"source": "marketplace", "target": "cat-home"},
      {"source": "cat-home", "target": "brand-rubbermaid"},
      {"source": "brand-rubbermaid", "target": "B002YK46UQ"}
    ]
  },
  "stats": {
    "categories_scanned": 12,
    "categories_total": 20,
    "eligible_products": 47,
    "restricted_products": 153,
    "open_brands": 8,
    "restricted_brands": 34
  }
}
```

**Fallback for simple rendering:**
If the graph library adds too much bundle size or complexity, a simpler treemap or sunburst chart can represent the same data:
- Outer ring: categories (area proportional to product count)
- Inner ring: brands within each category
- Color: green/red/gray for eligibility

The key principle: **the user should SEE the exploration happening**, understand the shape of their eligibility landscape, and feel confident the system is doing thorough work on their behalf.

### 5.3 Onboarding Step 3: Reveal

Two distinct views depending on outcome:

#### Outcome A: Opportunities Found

```
┌─────────────────────────────────────────────────┐
│  Great News! You Can Sell in 8 Categories         │
│                                                   │
│  We found 23 profitable products across your      │
│  eligible categories. Here's your opportunity map. │
│                                                   │
│  ┌─────────────────────────────────────────────┐  │
│  │ Category             Products  Avg Margin    │  │
│  │ Home & Kitchen            8      24.2%       │  │
│  │ Office Products           5      19.8%       │  │
│  │ Tools & Home Imp.         4      21.1%       │  │
│  │ Sports & Outdoors         3      18.5%       │  │
│  │ Pet Supplies              2      22.7%       │  │
│  │ Automotive                1      25.3%       │  │
│  └─────────────────────────────────────────────┘  │
│                                                   │
│  Top Recommendations                              │
│  ┌─────────────────────────────────────────────┐  │
│  │ ASIN       Title              Price  Margin  │  │
│  │ B0CX23V5KK Kitchen Utensil.. $29.99  24.1%  │  │
│  │ B0D1FG89NM Silicone Baking.. $14.99  21.3%  │  │
│  │ B0BY7K3PQR Bamboo Cutting..  $24.99  19.8%  │  │
│  │ ... (7 more)                                 │  │
│  └─────────────────────────────────────────────┘  │
│                                                   │
│  Your Strategy                                    │
│  Goal 1: List 5 products in Home & Kitchen        │
│          by April 24                              │
│  Goal 2: Reach $2,000/mo revenue by May 10        │
│  Goal 3: Expand to 15 products by June 10         │
│                                                   │
│  [ Approve Strategy ]                             │
└─────────────────────────────────────────────────┘
```

#### Outcome B: Nothing Found (Restricted Account)

```
┌─────────────────────────────────────────────────┐
│  Your Account Needs Ungating                      │
│                                                   │
│  We searched 400 products across 20 categories.   │
│  Your account is currently restricted from selling │
│  profitably in those categories. This is normal    │
│  for new accounts — here's your path forward.      │
│                                                   │
│  Ungating Roadmap                                 │
│  ┌─────────────────────────────────────────────┐  │
│  │ Step 1: Get Ungated in Grocery (Easiest)     │  │
│  │   Action: Apply via KeHE distributor invoice  │  │
│  │   Timeline: 2-3 weeks                         │  │
│  │   Impact: Unlocks ~47 profitable products     │  │
│  │                                               │  │
│  │ Step 2: Get Ungated in Health & Household     │  │
│  │   Action: Same KeHE invoice often works       │  │
│  │   Timeline: 1-2 weeks after Grocery           │  │
│  │   Impact: Unlocks ~30 more products           │  │
│  │                                               │  │
│  │ Step 3: Get Ungated in Beauty                 │  │
│  │   Action: Invoice from authorized distributor │  │
│  │   Timeline: 2-4 weeks                         │  │
│  │   Impact: 30-40% margins once unlocked        │  │
│  └─────────────────────────────────────────────┘  │
│                                                   │
│  Your Strategy                                    │
│  Goal 1: Get ungated in Grocery within 30 days    │
│  Goal 2: List first 5 Grocery products by day 45  │
│  Goal 3: Reach $1,000/mo revenue by day 60        │
│                                                   │
│  [ Start Ungating Plan ]                          │
└─────────────────────────────────────────────────┘
```

### 5.4 Onboarding Step 4: Commit

The user approves the strategy (whether sourcing or ungating). On approval:

- The strategy is saved as `strategy_versions` v1 (per continuous learning architecture)
- The discovery engine is configured to scan the relevant categories
- The user lands on the dashboard with:
  - **If sourcing strategy:** product recommendations ready to act on, "Find a supplier for these products" as first action
  - **If ungating roadmap:** first ungating task prominently displayed, progress tracker visible

---

## 6. Domain Model Changes

### 6.1 New Types

| Type | File | Purpose |
|---|---|---|
| `AmazonSellerAccount` | `internal/domain/amazon_seller_accounts.go` | Encrypted SP-API creds per tenant |
| `AssessmentOutcome` | `internal/domain/assessment_outcome.go` | Result of the discovery assessment |
| `UngatingRoadmap` | `internal/domain/assessment_outcome.go` | Path for restricted accounts |
| `ProductRecommendation` | `internal/domain/assessment_outcome.go` | Top product picks from assessment |
| `UngatingStep` | `internal/domain/assessment_outcome.go` | Single step in ungating roadmap |

### 6.2 Modified Types

| Type | Change |
|---|---|
| `SellerProfile` | Remove `AccountAgeDays`, `ActiveListings`, `StatedCapital` (no longer user-reported). Add `HasOpportunities bool`, `QualifiedProductCount int`, `EligibleCategoryCount int`. |
| `AssessmentProbe` | **Deleted** — no longer needed. Dynamic search replaces static probes. |
| `EligibilityFingerprint` | Unchanged — still stores category open rates and brand results, but now populated from real search results instead of hardcoded probes. |

### 6.3 New Interfaces

| Interface | File | Purpose |
|---|---|---|
| `TenantCredentialRepo` | `internal/port/credential_repo.go` | CRUD for encrypted credentials |
| `AssessmentOutcomeRepo` | `internal/port/assessment_repo.go` | Store assessment results and recommendations |

### 6.4 Modified Services

| Service | Change |
|---|---|
| `AssessmentService` | Rewritten: accepts `CredentialService` + `FunnelService`, runs discovery loop instead of probe scan |
| `CredentialService` (new) | Manages tenant credentials, builds per-tenant SP-API clients |

---

## 7. Database Changes

### 7.1 New Tables

**`amazon_seller_accounts`** (see Section 3.1 for full schema)

**`assessment_outcomes`** — stores the assessment result:

```sql
CREATE TABLE assessment_outcomes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    fingerprint_id UUID REFERENCES eligibility_fingerprints(id),

    has_opportunities   BOOLEAN NOT NULL DEFAULT false,
    qualified_count     INT NOT NULL DEFAULT 0,
    eligible_categories INT NOT NULL DEFAULT 0,
    open_brands         INT NOT NULL DEFAULT 0,

    -- JSON blobs for flexible storage
    top_recommendations JSONB NOT NULL DEFAULT '[]',  -- ProductRecommendation[]
    category_summaries  JSONB NOT NULL DEFAULT '[]',  -- CategorySummary[]
    ungating_roadmap    JSONB,                         -- UngatingRoadmap (null if opportunities found)
    strategy_goals      JSONB NOT NULL DEFAULT '[]',  -- StrategyGoal[]

    -- Assessment metadata
    products_searched   INT NOT NULL DEFAULT 0,
    products_eligible   INT NOT NULL DEFAULT 0,
    products_qualified  INT NOT NULL DEFAULT 0,
    api_calls_used      INT NOT NULL DEFAULT 0,
    duration_seconds    INT NOT NULL DEFAULT 0,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ao_tenant ON assessment_outcomes(tenant_id);
```

### 7.2 Modified Tables

No existing tables are modified. The existing `eligibility_fingerprints`, `brand_probe_results`, and `category_eligibilities` tables continue to work — they just get populated differently (from search results instead of hardcoded probes).

### 7.3 Migrations

1. `create_amazon_seller_accounts.sql` — new table
2. `create_assessment_outcomes.sql` — new table
3. `seed_test_amazon_seller_accounts.go` — Go script to encrypt and insert test credentials

---

## 8. Implementation Phases

### Phase A: Per-Tenant Credentials (1-2 days)

**Scope:**
- `amazon_seller_accounts` table and migration
- `AmazonSellerAccount` domain type
- `TenantCredentialRepo` port and Postgres adapter
- `CredentialService` with AES-256-GCM encryption
- `GetSPAPIClient(tenantID)` method
- Seed test tenant credentials
- Frontend: credential input form (onboarding Step 1)
- API endpoint: `POST /api/v1/credentials` and `GET /api/v1/credentials/status`

**Verification:**
- Can store and retrieve encrypted credentials
- Can build SP-API client from stored credentials
- Can validate credentials by calling SP-API token endpoint
- Frontend shows credential form and handles validation

### Phase B: Revised Assessment Loop (2-3 days)

**Scope:**
- Rewrite `AssessmentService.StartAssessment` and `RunEligibilityScan`
- Implement three-phase discovery loop (search, eligibility, profitability)
- Integrate with `FunnelService` for T1-T3 evaluation
- Build `AssessmentOutcome` with real product recommendations
- Inngest workflow with progress events
- `assessment_outcomes` table and repo
- API endpoint: `GET /api/v1/assessment/progress` (SSE or polling)

**Verification:**
- Assessment finds real eligible products using test tenant credentials
- Funnel correctly filters to profitable products
- Assessment completes in < 3 minutes
- Progress events emitted at each category

### Phase C: Revised Frontend (1-2 days)

**Scope:**
- Onboarding Step 2: live progress display with category-by-category status
- Onboarding Step 3 (Outcome A): category table, product recommendations, strategy goals
- Onboarding Step 3 (Outcome B): ungating roadmap display
- Onboarding Step 4: strategy approval flow
- Dashboard post-onboarding: product recommendations or ungating tasks visible

**Verification:**
- Full onboarding flow from credential input to strategy approval
- Correct display for both "opportunities found" and "nothing found" outcomes
- Strategy approval creates `strategy_versions` v1

### Phase D: Ungating Roadmap Content (1 day)

**Scope:**
- Curate ungating step content for Grocery, Health, Beauty, Toys
- Include distributor names, document requirements, estimated timelines
- Map restriction reasons to ungating actions
- Build static dataset (not LLM-generated) for reliability

**Verification:**
- Restricted account sees concrete, actionable ungating steps
- Each step has a specific action, timeline, and expected impact

---

## 9. What's NOT in Scope

| Item | Reason | When |
|---|---|---|
| OAuth flow | Requires Amazon app publication and approval | After app is published |
| Actual ungating application automation | Complex, varies by category, high risk | Future — human-guided for now |
| Multi-marketplace support | Start with US only | After US is proven |
| LLM-generated strategy (T4) | Assessment strategy is template-based from real data, not LLM prose | Future — LLM enhancement later |
| RAG / pgvector integration | Continuous learning is a separate effort | Per continuous learning architecture spec |
| Autoresearch / A/B experiments | Requires outcome data that does not exist yet | After sellers have been using the platform |

---

## 10. Risk Assessment

| Risk | Severity | Mitigation |
|---|---|---|
| SP-API rate limits hit during assessment | Medium | Conservative 3.5/sec rate, adaptive rate limiter already exists in `spapi.Client` |
| All 400 products come back restricted | Low-Medium | This is the "Outcome B" path — handled with ungating roadmap. Also: 20 categories with high expected open rates makes total restriction unlikely for most accounts |
| Credential encryption key compromise | High | Key stored in env var (not DB), rotatable, credentials are useless without the decryption key. Future: use AWS KMS or similar |
| Assessment takes > 3 minutes | Low | Budget is ~2.5 minutes. Buffer exists. Can reduce category count from 20 to 15 if needed |
| User provides wrong credentials | Low | Validate-credentials step catches this before any scanning begins. Clear error message guides re-entry |

---

## 11. Success Criteria

The revised assessment is successful when:

1. **Credential storage works** — a new tenant can enter SP-API credentials, have them validated, and use them for all subsequent API calls
2. **Assessment finds real products** — for a test account with standard permissions, the assessment discovers 10+ profitable products
3. **Restricted accounts get a roadmap** — for a heavily restricted account, the assessment produces a concrete ungating plan with specific actions
4. **No more "all rejected"** — the assessment never shows a blank table of results
5. **User has a next step** — after completing onboarding, every user has either products to source or an ungating task to complete
6. **< 3 minutes** — the full assessment completes within the time budget

---

## 12. Relationship to Other Specs

- **High-Volume Discovery Engine (2026-04-07):** The assessment uses the same `FunnelService` (T1-T3) built for the discovery engine. The assessment is effectively the bootstrap scan that initializes the discovery engine's target categories.

- **Continuous Learning Architecture (2026-04-08):** The strategy generated by the assessment becomes `strategy_versions` v1. The continuous learning loop (daily discovery, autoresearch, strategy evolution) operates on top of this initial strategy. The assessment is the seed; continuous learning is the growth.

- **Account Assessment Service (2026-04-08, superseded):** This spec replaces the previous assessment spec entirely. The 300-ASIN static probe approach is replaced by the 400-product dynamic search approach. The fingerprint data model is retained but populated differently.
