You are continuing PASS 3 of a 4-pass build for a production-grade SaaS platform for Amazon FBA wholesale resellers.

IMPORTANT
Assume PASS 1 and PASS 2 already created:
- monorepo scaffold
- backend services
- API endpoints
- Temporal workflows
- Supabase auth integration
- PostHog backend integration

In this pass, focus on the frontend only.
Do NOT redesign the backend.
Consume the existing backend APIs.

FRONTEND STACK
- Next.js App Router
- TypeScript
- Tailwind CSS
- shadcn/ui
- TanStack Query
- minimal client state
- Supabase auth
- PostHog client integration

GOAL
Build a polished, credible B2B SaaS frontend for operators making real purchasing and workflow decisions.

UX DIRECTION
The UI should feel:
- premium
- operational
- fast
- uncluttered
- trustworthy
- not like a generic CRUD admin panel

Avoid:
- template-looking dashboards
- excessive gradients
- gimmicky animations
- dense unreadable tables

Prefer:
- clean panels
- sticky filters
- rich data tables
- split views
- detailed drilldowns
- strong empty/loading/error states
- practical operator workflows

PAGES TO IMPLEMENT

1. AUTH
- sign in
- sign up
- invite acceptance placeholder
- protected routes
- session bootstrap

2. APP SHELL
- tenant-aware layout
- sidebar
- top navigation
- breadcrumbs
- user menu
- quick actions

3. DASHBOARD
- KPI cards
- recent deals
- pending approvals
- active experiments
- supplier pipeline snapshot
- replenishment alerts

4. CATALOGS
- upload page with drag-and-drop
- upload validation state
- upload progress placeholder tied to API state
- catalog history list
- catalog detail page with items table

5. DEAL EXPLORER
- advanced table
- filters
- search
- sort
- score badges
- status pills
- profitability summary
- risk indicators

6. DEAL DETAIL
- score breakdown
- profitability panel
- ASIN match evidence
- supplier candidates
- activity timeline
- approval actions

7. SUPPLIER CRM
- suppliers list
- supplier detail page
- contacts
- notes/timeline
- related deals
- outreach state

8. APPROVALS INBOX
- human review queue
- review drawer/panel
- approve / reject / request changes actions
- comments

9. REPLENISHMENT
- stock risk dashboard
- reorder suggestions
- monitor status

10. EXPERIMENTS
- experiments list
- detail page
- variants
- metrics
- status
- keep / revert controls
- shadow mode indicators

11. AUDIT
- timeline page
- filters by actor/entity/action
- correlation IDs
- event details

12. SETTINGS
- tenant settings
- members/roles
- integrations overview
- PostHog diagnostics
- feature flag diagnostics

FRONTEND IMPLEMENTATION REQUIREMENTS

DATA LAYER
- use TanStack Query
- typed API client
- query keys
- loading/error states
- optimistic updates only when safe

DESIGN SYSTEM
- reusable layout primitives
- table components
- filter bar
- metric card
- detail panel
- timeline component
- status badge
- score badge
- risk badge
- empty state
- page header
- command palette optional

AUTH
- integrate Supabase auth on frontend
- route guards
- session provider
- login/logout flows
- API token forwarding strategy

POSTHOG
- add PostHog client integration
- page view capture
- important UX events
- feature flag bootstrap hooks
- experiment-aware UI toggles only for non-critical client features

QUALITY
- good component decomposition
- accessible UI
- coherent visual system
- no dead demo text
- realistic labels and flows
- strong types

OUTPUT FORMAT
1. List all modified/new frontend files first.
2. Then print each file:
FILE: <path>
<full contents>

END THIS PASS WITH
- implemented screens/components
- any backend assumptions made
- what Pass 4 should implement