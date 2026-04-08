# AI Concierge Platform — Expert Brainstorm

**Date:** 2026-04-08
**Status:** Research complete — informing product direction
**Experts:** FBA Business Strategist, AI Product Architect, Marketplace Economist, Competitive Analyst

---

## Executive Synthesis

### The Opportunity

Every Amazon wholesale tool today is a single-function blade — sellers duct-tape 4-6 tools at $200-400/mo and still make decisions in spreadsheets. Nobody connects a seller's specific situation (what they can sell, their capital, their stage) to an actionable strategy. The AI capabilities to do this only became viable in the last 18 months.

### The "Wealthfront Moment"

> Seller connects their Amazon account and uploads one supplier price list. In 60 seconds, the platform returns: "Based on your account, you have $14K in available capital, strong Buy Box performance in Health & Household. From this price list, I found 23 profitable ASINs. Here's your recommended first order — $3,200 across 8 SKUs — projected to return 31% ROI in 45 days. Want me to explain why I skipped the other 15?"

Every other tool says "here are 23 green checkmarks." The concierge says "here's what I'd do if I were you, and here's why."

---

## 1. The Ungating Journey (FBA Strategist)

### First 90 Days
- **Days 1-30:** Open categories only (Home & Kitchen, Office, Sports). Goal: velocity and account health.
- **Days 31-60:** First ungating wins (Grocery is easiest — invoices from known distributor). 25-35% ROI on consumables.
- **Days 61-90:** High-value unlocks (Toys, Health, specific brands).

### The Ungating Ladder (by ROI impact)
1. Grocery — easiest gate, highest replenishment, lowest competition
2. Health & Household — overlaps with Grocery distributors
3. Beauty — higher margins (30-40% ROI), brand-gating is the real barrier
4. Toys & Games — seasonal goldmine (Q4)
5. Major brands (Lego, Nike, Hasbro) — requires established metrics + real invoices

### Key Insight
The restriction isn't just category-level — it's brand-level. A seller can be approved for Grocery but blocked from selling Cheerios specifically. The pivot: stop searching all ASINs and start searching only within the seller's approved brand/category set, then expand that set strategically.

---

## 2. Account Assessment (Marketplace Economist)

### Eligibility Landscape
- New seller can access ~40-50% of Amazon's catalog by ASIN count
- But only ~20-30% of the *profitable wholesale* catalog

### Category Open Rates for New Accounts
| Category | Open Rate |
|---|---|
| Books | ~95% |
| Home & Kitchen | ~85% |
| Tools & Home Improvement | ~80% |
| Office Products | ~80% |
| Sports & Outdoors | ~75% |
| Toys & Games | ~60% |
| Grocery & Gourmet | ~30% |
| Beauty & Personal Care | ~25% |
| Health & Household | ~20% |

### Efficient Eligibility Sampling
- **300-500 SP-API calls** give ~90% accuracy on eligibility fingerprint
- Strategy: 8-12 representative ASINs × 30 categories = 240-360 calls
- Per category: 3 top-brand ASINs (likely gated) + 3 mid-tier + 3 generic + calibration ASINs
- Result: category-level eligibility score (0-100%) + brand-gate density estimate

### Category Prioritization Formula
```
Score = (E × 0.30) + (M × 0.25) + (1/C × 0.20) + (1/D × 0.15) + (1/K × 0.10)

E = eligible product count (normalized)
M = average net margin after FBA fees
C = competition density (avg offer count)
D = ungating difficulty (1=open, 2=invoice, 3=brand approval, 4=performance gate)
K = minimum capital required
```

### Ungating Economics
- Brand X with 50 ASINs, 25% avg margin → ~$2,800/mo, $33,600/yr gross profit
- Ungating cost: $300-$500 (invoices from authorized distributor)
- **ROI: 67-112x on ungating investment**

---

## 3. Seller Archetypes (FBA Strategist)

| Archetype | Profile | Strategy |
|---|---|---|
| **The Greenhorn** | New account, <90 days | Open categories only, build health metrics, no ungating yet |
| **RA-to-Wholesale** | 6-12 mo RA experience | Has health metrics, needs distributor education, cash flow planning |
| **Expanding Pro** | 1yr+, $10-50K/mo | More brands, better margins, automation — our power user |
| **Capital-Rich Beginner** | New, $50K+ to deploy | Needs guardrails against over-investing before health established |

---

## 4. Growth Curves (Marketplace Economist)

### Monthly Gross Revenue
| Month | Bottom 50% | Median | Top 10% |
|---|---|---|---|
| 1 | $500 | $2,000 | $5,000 |
| 3 | $1,500 | $6,000 | $25,000 |
| 6 | $3,000 | $15,000 | $60,000 |
| 12 | $5,000 | $30,000 | $150,000+ |

### What Separates Top 10%
1. 20+ supplier accounts within 90 days
2. Aggressive ungating in first 60 days
3. Systematic repricing (not manual)
4. Reinvest 80%+ of profits months 1-6
5. Kill slow movers fast (<30 day tolerance)

### Unit Economics by Category
| Category | Avg Net Margin | Sell-Through (30d) | Annualized ROI | Capital Turns/Yr |
|---|---|---|---|---|
| Home & Kitchen | 18-22% | 60-70% | 120-180% | 6-8x |
| Office | 12-18% | 70-80% | 110-160% | 8-10x |
| Grocery (ungated) | 20-30% | 65-80% | 200-300% | 8-12x |
| Beauty (ungated) | 25-35% | 55-70% | 180-280% | 7-9x |

**Key metric: Annualized ROI = margin × turns/year.** A 15% margin product turning 10x/year (150% ROI) beats 30% margin turning 3x (90% ROI).

---

## 5. Capital Allocation — $5K Start (Economist)

| Allocation | Amount | Purpose |
|---|---|---|
| Initial inventory | $3,500 (70%) | 7-10 ASINs, $350-$500 each |
| Ungating invoices | $500 (10%) | 1-2 category/brand unlocks |
| Tools & subscriptions | $300 (6%) | Repricer, sourcing tool |
| Reserve/buffer | $700 (14%) | Restock fast sellers |

---

## 6. Product Architecture (AI Product Architect)

### Onboarding Flow (< 10 minutes)
1. **Connect** (30s) — OAuth to SP-API
2. **Discover** (3-5 min, async) — eligibility scan, account health, brand mapping
3. **Reveal** (60s) — "Strategy Brief": top 3 categories, 5 quick-win brands, 2 high-value ungating targets
4. **Commit** (30s) — "Start this plan" generates Growth Plan

### Growth Plan Model — "Quest Log"
Not kanban, not timeline. Tree of milestones containing tasks:
```
GrowthPlan (1:1 with tenant)
  └── Milestone[] (ordered, unlockable)
       └── Task[] (concrete actions, some auto-completable)
            └── TaskEvent[] (audit trail)
```
Show "Plan Progress: 34%" prominently. Weekly deltas. Streak counters.

### AI Advisor Pattern — Not a Chatbot
- **Dashboard nudges** (primary): "3 new products match your strategy"
- **Weekly digest** (email + in-app): metrics, new opportunities, completed tasks
- **Reactive chat** (secondary): "Why did you recommend this brand?"
- Cadence: daily scan (auto), weekly digest (pushed), monthly strategy review (prompted)

### Automation Spectrum
| Tier | Actions | Approval |
|---|---|---|
| Auto | Scan, check eligibility, refresh prices | None |
| Suggest | New products, brands to ungate, outreach drafts | One-click approve/dismiss |
| Manual | Place POs, send outreach, adjust strategy | User initiates |

### Technical Architecture (fits existing hexagonal pattern)
```
internal/domain/
  growth_plan.go      // GrowthPlan, Milestone, Task
  seller_profile.go   // SellerProfile (assessment results, stage)
  suggestion.go       // Suggestion, ApprovalStatus

internal/port/
  strategy_engine.go  // interface for AI strategy generation

internal/service/
  onboarding_svc.go   // orchestrates assessment
  strategy_svc.go     // generates and evolves plans
  nudge_svc.go        // suggestions and digests
```

---

## 7. Competitive Position (Competitive Analyst)

### Current Market Gap
Every tool is single-function. Sellers spend $200-400/mo duct-taping tools. Nobody provides personalized strategy based on account-specific data.

### Why It Hasn't Been Built
- Data integration across SP-API + suppliers + accounting is hard
- Wholesale is smaller TAM than private label (where VC money went)
- Personalization requires seller-specific variables no tool collects
- AI capabilities only viable since ~2025

### Pricing Sweet Spot
| Tier | Price | What |
|---|---|---|
| Scanner | $79/mo | Product discovery, eligibility checking |
| Concierge | $149-199/mo | Full strategy, ungating roadmap, distributor matching |
| Enterprise | $299-499/mo | Multi-user, managed growth, priority scanning |

Replaces $250/mo tool stack at $149-199/mo — net savings for seller, premium for us.

### Go-to-Market
- YouTube (wholesale education creators)
- Facebook Groups (wholesale-specific, 10-50K members)
- Wholesale conferences
- Content-led funnels (webinars showing personalized analysis)
- CAC: $150-250, LTV: ~$2,450 (14-month median retention at $175/mo avg), LTV:CAC: 10-16x

### Moat (compounds over time)
1. **Seller decision history** — accept/reject/outcome pairs that train strategy models
2. **Supplier catalog intelligence** — aggregated across tenants, proprietary pricing data
3. **Portfolio optimization models** — capital allocation, diversification, seasonal timing
4. Incumbents adding "AI" will bolt ChatGPT onto existing UIs. That's autocomplete, not concierge.

### Risks
| Risk | Severity | Hedge |
|---|---|---|
| SP-API restrictions | High | Build value on supplier-side intelligence |
| Jungle Scout adds AI concierge | Medium | They're optimized for private label |
| AI commoditization | Medium | Model is commodity; data flywheel is not |
| Fee increases squeeze margins | Low-Med | Concierge that adapts is MORE valuable |

---

## 8. Failure Modes to Prevent (FBA Strategist)

1. **Cash flow death** — buying inventory that sells in 180 days not 30
2. **Race to bottom** — too many sellers on a listing, margin compresses to zero
3. **Account health neglect** — one bad metric tanks ungating applications
4. **Diversification failure** — 80% of revenue from 2 ASINs
5. **MAP violation blindness** — selling below minimum advertised price, brand-banned
6. **Overstock on seasonal** — buying Toys in January at Q4 prices
7. **Biggest killer: selling restricted products** → IP complaints → account suspension

---

## 9. Retention Mechanics by Month (Product Architect)

- **Month 1:** Onboarding magic. First products listed via platform guidance.
- **Month 2:** Weekly digests showing ROI. "Platform found you $X in opportunities."
- **Month 6:** Strategy evolution. New milestones unlock as seller graduates.
- **Month 12:** Platform is the operating system. Historical data, seasonal prep, switching cost enormous.

**Sticky mechanic:** Accumulated intelligence. "Your AI has analyzed 14,000 ASINs for you."

---

## 10. Distributor Relationship Support (FBA Strategist)

### The Process
1. Find distributors (manufacturer websites, trade shows, Wholesale Central, ThomasNet)
2. Apply — requires: business license, resale certificate, EIN, website, sometimes references
3. Get rejected initially — 60%+ rejection rate at tier-1
4. Start with tier-2/3 (KeHE, UNFI for grocery; S&S Worldwide for toys)

### How Platform Helps
- Maintain distributor-to-brand mapping database
- Track which distributors are new-seller-friendly
- Auto-generate application materials
- Tell sellers: "You need an invoice from X distributor to ungate in Y brand — here's how to get that account"
