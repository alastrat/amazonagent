You are a principal staff engineer and elite systems architect. Build the foundation of a production-grade SaaS platform for Amazon FBA wholesale resellers.

IMPORTANT
This is PASS 1 of a 4-pass build.
In this pass, do NOT try to fully implement the entire product.
Focus on:

- monorepo scaffolding
- architecture
- core interfaces
- config
- infra skeleton
- app bootstrapping
- database schema foundations
- adapter boundaries
- documentation
- compile-safe stubs

GOAL
Create the initial monorepo and architectural foundation for a multi-tenant SaaS that helps Amazon FBA wholesale resellers source, analyze, rank, approve, monitor, and continuously improve wholesale opportunities.

MANDATORY STACK

- Backend: Go
- Frontend: Next.js + TypeScript + Tailwind + shadcn/ui
- Auth + DB: Supabase
- Analytics + feature flags + experiments: PostHog
- Durable execution: Temporal as primary
- Edge: Cloudflare Workers
- Swappable agent runtimes:
  - OpenFang
  - LangGraph
  - CrewAI

ARCHITECTURE REQUIREMENTS
The system must decouple:

1. business logic
2. agent orchestration runtime
3. durable workflow runtime

Create explicit interfaces for:

- AgentRuntime
- DurableRuntime
- EventBus
- AuditLogger
- ExperimentProvider
- FeatureFlagProvider
- AuthProvider

BUSINESS CONTEXT
This SaaS should eventually support:

- supplier catalog upload
- normalization
- ASIN matching
- deal scoring
- profitability calculation
- supplier CRM
- outreach workflows
- approval queues
- replenishment monitoring
- autoresearch-inspired continuous improvement
- A/B testing
- audit logs

MONOREPO STRUCTURE
Create:

/apps
  /api
  /worker
  /web
  /edge

/services
  /catalog
  /deals
  /suppliers
  /experiments
  /scoring
  /compliance
  /auth
  /audit
  /analytics

/internal/platform
  /config
  /database
  /events
  /observability
  /orchestration
  /durable
  /posthog
  /supabase
  /http
  /middleware

/adapters
  /orchestration/openfang
  /orchestration/langgraph
  /orchestration/crewai
  /durable/temporal
  /durable/inngest
  /durable/cloudflare
  /analytics/posthog
  /auth/supabase

/pkg
/deploy
/docs

WHAT TO IMPLEMENT IN THIS PASS

1. MONOREPO SETUP

- complete folder tree
- Go module/workspace setup
- package boundaries
- Makefile
- Docker Compose
- env example files
- task scripts
- README

1. API APP SCAFFOLD

- app entrypoint
- router
- health endpoint
- readiness endpoint
- middleware skeleton
- versioned API structure
- dependency wiring container

1. WORKER APP SCAFFOLD

- worker entrypoint
- temporal worker bootstrap
- registration placeholders for workflows/activities
- structured logging bootstrap

1. WEB APP SCAFFOLD

- Next.js app router structure
- base layout
- placeholder dashboard
- auth shell pages
- shared UI setup
- Tailwind config
- shadcn-friendly structure

1. EDGE APP SCAFFOLD

- Cloudflare Worker app
- webhook endpoint stub
- approval-link endpoint stub
- signature validation helper placeholder
- forwarding stub to core API

1. DOMAIN MODELS

Define initial models/types for:

- Tenant
- User
- Membership
- Catalog
- CatalogItem
- Product
- ASINMatch
- Deal
- Supplier
- SupplierContact
- OutreachSequence
- Approval
- InventorySnapshot
- Experiment
- ExperimentVariant
- ExperimentRun
- AuditLog
- DomainEvent

1. INTERFACES

Define clean interfaces for:

- AgentRuntime
- DurableRuntime
- CatalogRepository
- DealRepository
- SupplierRepository
- ExperimentRepository
- AuditRepository
- FeatureFlagProvider
- AnalyticsProvider
- AuthProvider
- Clock
- IDGenerator

1. DATABASE FOUNDATIONS

- SQL migrations for initial schema
- tenant-aware tables
- indexes
- constraints
- created_at/updated_at
- audit/event tables
- seed data skeleton

1. SUPABASE INTEGRATION FOUNDATION

- auth middleware contract
- JWT verification abstraction
- tenant membership resolution
- RLS strategy doc
- Supabase config package

1. POSTHOG FOUNDATION

- analytics provider abstraction
- feature flag abstraction
- experiment exposure abstraction
- no deep business implementation yet

1. TEMPORAL FOUNDATION

- DurableRuntime interface
- Temporal adapter scaffold
- workflow and activity registration points
- no deep workflow logic yet

1. ORCHESTRATION FOUNDATION

- AgentRuntime interface
- adapter scaffolds for OpenFang / LangGraph / CrewAI
- each adapter should compile with stub methods

1. DOCUMENTATION

Create:

- main README
- architecture overview
- ADRs:
  - decoupled orchestration
  - decoupled durable runtime
  - Temporal as primary engine
  - Cloudflare Workers as edge layer
  - PostHog for analytics/flags/experiments
  - Supabase for auth and DB

CODE QUALITY RULES

- Use idiomatic Go
- Use clean architecture / hexagonal principles
- Keep code compile-safe
- Prefer realistic interfaces over vague placeholders
- No pseudo-code unless absolutely necessary
- Stubs must be clearly isolated behind interfaces
- Keep imports coherent
- Use structured logging
- Use context.Context everywhere appropriate

OUTPUT FORMAT

1. Print the complete folder tree first.
2. Then generate files in logical order.
3. For every file, use:

FILE: 


END THIS PASS WITH

- what is implemented
- what remains for Pass 2
- exact commands to run locally

