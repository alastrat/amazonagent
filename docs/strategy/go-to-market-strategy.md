# Go-to-Market Strategy: Estori -- Agentic Wholesale Automation

*Last updated: 2026-04-11*
*Status: Draft -- pending founder review*

---

## Executive Summary

Estori is an agentic wholesale sourcing platform for Amazon FBA sellers. Unlike existing tools (Jungle Scout, Helium 10, SellerAmp) that provide data for human interpretation, Estori runs a 7-agent AI pipeline that autonomously discovers, evaluates, and ranks wholesale deals -- then pauses for human approval before any irreversible action. The platform's unique advantages are account-specific eligibility fingerprinting via SP-API, a shared catalog network effect where every scan enriches platform-wide data, and continuous autonomous discovery with human approval gates.

---

## 1. Market Positioning

### Target Customer

Mid-stage Amazon FBA wholesale sellers doing $10K-$100K/month who currently cobble together 4-6 tools (Keepa, SellerAmp, manual spreadsheet scanning, VA labor) to source products. The primary beachhead segment is the "RA-to-wholesale" seller transitioning from retail arbitrage who needs scalable sourcing but cannot afford a full-time analyst.

### Seller Archetypes

| Archetype | Description | Needs | Tier Target |
|-----------|-------------|-------|-------------|
| ra_to_wholesale | Transitioning from retail arbitrage | Guidance, scalable sourcing | Starter / Growth |
| expanding_pro | Established wholesale seller scaling | Efficiency, deal volume | Growth |
| capital_rich | High capital, limited time | Turnkey deal flow | Scale / Enterprise |

### Market Size

- ~300K active US FBA sellers
- ~40K do wholesale at meaningful volume
- At $149/mo average: **$71M/year TAM** for wholesale-specific tooling
- Adjacent market (all FBA sellers who could adopt wholesale): 3-5x larger

### Competitive Landscape

| Competitor | Category | What They Do | What They Don't Do |
|-----------|----------|-------------|-------------------|
| Jungle Scout | Research tool | Keyword research, product database, supplier database | No automated evaluation, no eligibility checking, no deal pipeline |
| Helium 10 | Research tool | Keyword research, listing optimization, review analytics | No batch price list processing, no agent-based scoring |
| SmartScout | Research tool | Brand/category analysis, subcategory explorer | No account-specific data, no autonomous sourcing |
| SellerAmp / BuyBotPro | Calculator | Single-ASIN profitability scoring via chrome extension | No batch processing, no pipeline, no eligibility |
| Keepa | Data provider | Price history, BSR tracking, deal alerts | Raw data only, no interpretation or scoring |
| Tactical Arbitrage | Scanner | Source-list scanning, price comparison | No AI evaluation, no eligibility, no deal lifecycle |

### Unique Value Proposition

**"Upload a price list, get back profitable deals you can actually sell."**

Three differentiators:

1. **Account-specific eligibility fingerprinting** -- Not generic gating data, but what YOUR account can sell, verified against SP-API in real time.
2. **Shared catalog network effect** -- Every scan enriches a platform-wide product database. The more sellers use Estori, the faster and cheaper scans become for everyone.
3. **Continuous autonomous discovery** -- Not a one-time scan but an ongoing agent pipeline with strategy versioning, nightly discovery runs, and human approval gates.

### Category Creation

Position as **"agentic sourcing"** -- not "product research." The platform replaces the VA + spreadsheet + 5-tool stack, not adds another tab to the browser.

Messaging framework: *"Your wholesale sourcing team, automated."*

---

## 2. Pricing Strategy

### Credit-Based Model

| Tier | Monthly Price | Credits/Month | Target Segment | Key Features |
|------|-------------|---------------|----------------|-------------|
| Free | $0 | 500 | Trial / Assessment | Account assessment, eligibility map, limited catalog access |
| Starter | $79/mo | 5,000 | Small-scale sellers | 1-2 price list scans/month, manual campaigns |
| Growth | $199/mo | 25,000 | Scaling sellers | Continuous nightly discovery, multiple suppliers, priority enrichment |
| Scale | $499/mo | 100,000 | High-volume operations | Multi-marketplace, advanced strategy versioning, API access |

### Credit Consumption

| Operation | Credits |
|-----------|---------|
| Eligibility check | 1 per ASIN |
| Product enrichment | 2 per ASIN |
| Assessment scan | 5 per ASIN |
| Discovery run | 10 per candidate evaluated |

### Free Tier Strategy

The onboarding flow (Connect -> Discover -> Reveal -> Commit) is the conversion funnel:

1. **Connect** -- Seller links their SP-API credentials (free, no credit card)
2. **Discover** -- Platform runs account assessment, maps eligibility across categories
3. **Reveal** -- Seller sees their eligibility fingerprint, opportunity map, and ungating roadmap
4. **Commit** -- Seller converts to paid to act on the opportunities

This is the "aha moment." No existing tool gives sellers a comprehensive eligibility map tied to their specific account.

### Enterprise Consideration

For prep centers and aggregators running 10+ seller accounts: custom pricing with white-label or multi-tenant plan.

---

## 3. Distribution Channels

### Primary: Free Account Assessment as Lead Magnet

FBA sellers are obsessed with knowing what categories they are ungated in. No existing tool gives them a comprehensive eligibility map tied to their specific account.

**Core CTA:** *"See what you can sell on Amazon in 60 seconds."*

### Community Channels (in order of density)

1. **Wholesale Facebook groups** -- The Wholesale Formula alumni, Amazon Wholesale Sourcing
2. **Reddit** -- r/FulfillmentByAmazon
3. **YouTube wholesale educators** -- Todd Welch / Amazon Seller School, Larry Lubarsky, Dan Vas
4. **Amazon seller Discord communities**

### Content Strategy

Produce content around outcomes of the assessment using proprietary data from the shared catalog:

- "We analyzed 10,000 seller accounts -- here is what categories are actually open in 2026"
- Category-level open rate benchmarks
- Wholesale margin analysis by category
- Seasonal opportunity reports

This is defensible content -- no competitor can replicate it without the shared catalog data.

### Partnership Strategy

| Partner Type | Examples | Integration Model |
|-------------|---------|------------------|
| Wholesale distributors | Dollar Days, Kole Imports, S&P Wholesale | Embed as "check if this product works for you" alongside catalogs |
| Prep centers | MyFBAPrep, ShipBob FBA Prep | Referral partnership -- their customers are our target |
| Accounting tools | Sellerboard, InventoryLab | Data integration for margin tracking |
| Course creators | The Wholesale Formula, Wholesale Suite | Affiliate credits (not cash) -- aligns with credit model |

### Affiliate Program

Offer referral credits instead of cash payouts. Aligns incentives with the credit model and reduces CAC.

---

## 4. Launch Strategy

### Phase 1: Design Partners (Months 1-3)

**Goal:** Validate product-market fit with real wholesale sellers.

**Actions:**
- Recruit 20-30 wholesale sellers from Facebook groups and The Wholesale Formula alumni
- Provide Growth tier free for the duration
- Weekly feedback sessions, direct Slack channel
- Ship account assessment as public free tool to start building the shared catalog

**Targets:**

| Metric | Target |
|--------|--------|
| Weekly active usage | 80% |
| Price list uploads per user | 5+ |
| NPS | > 40 |

**Key Metric:** *Deals approved / deals suggested ratio* -- measures whether the agent pipeline produces quality output.

### Phase 2: Public Launch (Months 4-6)

**Goal:** Establish market presence and validate conversion funnel.

**Actions:**
- Open free tier with account assessment
- Launch content campaign around eligibility data
- Announce via 3-4 YouTube wholesale educator partnerships
- Begin community engagement in Facebook groups and Reddit

**Targets:**

| Metric | Target |
|--------|--------|
| Free assessments (month 1) | 1,000 |
| Free-to-paid conversion (30 day) | 5% |
| Paying customers (end of phase) | 100 |

**Key Metric:** *Time to first approved deal* < 24 hours from signup.

### Phase 3: Scale (Months 7-12)

**Goal:** Build defensible moat through network effects and feature depth.

**Actions:**
- Enable nightly discovery for Growth+ tiers
- Launch shared catalog network effect messaging ("50,000 products enriched -- your scans are 10x faster")
- Add marketplace expansion (UK, EU)
- Launch enterprise tier for aggregators and prep centers

**Targets:**

| Metric | Target |
|--------|--------|
| Paying accounts | 500 by month 12 |
| ARR | $100K by month 9 |
| Credit utilization rate | > 60% monthly |

**Key Metric:** *Monthly credit utilization rate* -- indicates stickiness and value delivery.

---

## 5. Risk Analysis

| Risk | Severity | Likelihood | Mitigation |
|------|----------|-----------|-----------|
| Amazon SP-API changes or access revocation | High | Medium | Shared catalog reduces API dependency; diversify to Walmart + TikTok Shop |
| Amazon TOS compliance issues | High | Low | SP-API eligibility checks are standard use case; rate limiting via shared catalog |
| Competitive response (Helium 10 builds wholesale mode) | Medium | Medium | Move fast on network effect; 10K sellers contributing = defensible moat |
| Slow adoption / long sales cycle | Medium | Medium | Free assessment reduces friction; no credit card for aha moment |
| LLM cost scaling with user growth | Medium | High | Deterministic pre-filters before LLM calls; shared catalog prevents redundant enrichment |
| Single platform dependency (Amazon only) | High | High | Multi-marketplace expansion roadmap; shared catalog as standalone asset |

---

## 6. Success Metrics by Phase

| Metric | Phase 1 | Phase 2 | Phase 3 |
|--------|---------|---------|---------|
| Free assessments | 50 | 1,000 | 5,000 |
| Paying customers | 0 (design partners) | 100 | 500 |
| ARR | $0 | $30K | $100K |
| Deal approval rate | Baseline | > 30% | > 40% |
| Credit utilization | Baseline | > 40% | > 60% |
| NPS | > 40 | > 45 | > 50 |
| Shared catalog ASINs | 10K | 100K | 500K |

---

## 7. Key Assumptions

1. FBA wholesale sellers will connect their SP-API credentials to a third-party platform (precedent: Jungle Scout, Helium 10, InventoryLab all require this)
2. The free account assessment provides enough value to drive word-of-mouth in seller communities
3. The shared catalog network effect creates meaningful speed/cost advantages within 6 months
4. 7-agent pipeline deal quality is high enough that sellers approve > 30% of suggestions
5. Credit-based pricing aligns with usage patterns (sellers who get value use more credits)

These assumptions should be validated during Phase 1 with design partners before committing to Phase 2 spend.
