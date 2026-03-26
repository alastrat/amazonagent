You are continuing PASS 2 of a 4-pass build for a production-grade SaaS platform for Amazon FBA wholesale resellers.

IMPORTANT
Assume PASS 1 already created the monorepo, scaffolding, interfaces, docs, and initial migrations.
In this pass, implement the backend core.
Do NOT rebuild the repo from scratch.
Extend the existing architecture cleanly.

FOCUS OF THIS PASS
- core Go domain services
- repositories
- API endpoints
- Temporal workflows/activities
- Supabase auth integration
- PostHog backend integration
- domain event logging
- approval flows
- compile-ready backend

BUSINESS CAPABILITIES TO IMPLEMENT

1. CATALOG INGESTION
- upload metadata model
- catalog creation flow
- catalog item normalization pipeline
- UPC/EAN/brand/title normalization utilities
- status transitions
- event emission

2. ASIN MATCH PIPELINE
- interfaces for ASIN matching provider
- stubbed but realistic matching service
- match evidence model
- confidence scoring model
- storage of candidate matches

3. DEAL SCORING
- profitability calculator
- margin model
- ROI model
- score breakdown structure:
  - demand
  - competition
  - margin
  - risk
  - buy-box potential
- weighted score calculator
- explanation fields for UI

4. DEAL LIFECYCLE
- draft
- analyzed
- needs_review
- approved
- rejected
- monitoring
- archived

5. SUPPLIER CRM BACKEND
- supplier CRUD
- supplier contacts
- supplier notes/timeline events
- relationship to deals
- outreach sequence model

6. APPROVAL QUEUE
- create approval requests
- approve / reject / request changes
- audit trail
- role-aware permissions

7. REPLENISHMENT MONITOR FOUNDATION
- inventory snapshot storage
- reorder recommendation structure
- low-stock threshold logic
- event emission

8. EXPERIMENT ENGINE FOUNDATION
- experiment definitions
- variants
- experiment runs
- shadow mode support
- assignment record storage
- keep/revert model
- link to PostHog identifiers

9. DOMAIN EVENTS + AUDIT
- emit domain events on all important transitions
- persist domain events
- persist audit logs
- correlation IDs

API ENDPOINTS TO IMPLEMENT
Add real REST endpoints for:
- auth/session bootstrap
- dashboard summary
- catalogs
- catalog items
- deals
- deal details
- suppliers
- supplier contacts
- approvals
- experiments
- audit timeline
- replenishment summary

Include:
- pagination
- filtering
- sorting
- idempotency middleware where relevant
- tenant scoping
- RBAC middleware hooks

TEMPORAL IMPLEMENTATION
Implement real Temporal support for:
1. CatalogProcessingWorkflow
   - normalize catalog
   - generate candidate ASIN matches
   - score candidate deals
   - mark items for review
2. SupplierOutreachWorkflow
   - draft outreach
   - wait for approval if required
   - send step placeholder
   - schedule follow-up
3. ReplenishmentMonitorWorkflow
   - periodic scan
   - create reorder suggestions
4. ExperimentEvaluationWorkflow
   - collect candidate metrics
   - compare variants
   - produce recommendation
   - require approval for risky changes

Use:
- activities
- retry policies
- signals
- long waits
- approval resume points
- structured workflow inputs/outputs

SUPABASE BACKEND INTEGRATION
Implement:
- JWT validation abstraction
- request auth middleware
- tenant membership lookup
- user context injection
- role resolution
- local dev config support

POSTHOG BACKEND INTEGRATION
Implement:
- analytics provider
- event capture methods
- feature flag evaluation abstraction
- experiment exposure/event helpers
- backend-side usage only where appropriate
- safe no-op fallback for local development

DATA ACCESS
Implement repositories for Postgres/Supabase-backed storage:
- catalogs
- deals
- suppliers
- approvals
- experiments
- audit logs
- domain events

CODE QUALITY RULES
- no fake architecture
- realistic service methods
- working handlers
- compile-safe code
- coherent package boundaries
- keep external provider logic behind adapters
- use interfaces from PASS 1 rather than bypassing them

OUTPUT FORMAT
1. List modified/new files first.
2. Then print each file:
FILE: <path>
<full contents>

END THIS PASS WITH
- implemented backend capabilities
- remaining backend gaps
- what Pass 3 should implement on the frontend