# Phase 1: Product Sourcing + Deal Scoring — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the MVP backend and frontend for campaign-based product sourcing, 7-agent quality-gated research pipeline, deal scoring, and a dashboard to review results.

**Architecture:** Hexagonal Go backend behind REST API. Supabase for auth + Postgres. Inngest for durable workflows. OpenFang for agent orchestration. Next.js dashboard. All external systems behind interfaces — no direct SDK dependencies in domain/service code.

**Tech Stack:** Go 1.22+, Next.js 14 (App Router), TypeScript, Tailwind CSS, shadcn/ui, TanStack Query, Supabase (Auth + Postgres), Inngest, OpenFang, Amazon SP-API, Exa, Firecrawl, OpenAI

---

## File Structure

### Go Backend (`apps/api/`)

```
apps/api/
  main.go                          -- entrypoint, wires everything
  cmd/
    serve.go                       -- HTTP server boot
    worker.go                      -- Inngest worker boot

internal/
  domain/                          -- pure domain types, no dependencies
    campaign.go                    -- Campaign, CampaignType, CampaignStatus, Criteria
    deal.go                        -- Deal, DealStatus, DealScores, Evidence
    scoring.go                     -- ScoringConfig, ScoringWeights, Thresholds
    discovery.go                   -- DiscoveryConfig
    event.go                       -- DomainEvent
    approval.go                    -- Approval
    supplier.go                    -- Supplier, SupplierQuote (stubs for MVP)
    research.go                    -- ResearchResult, CandidateResult, AgentOutput
    tenant.go                      -- Tenant, Membership
    user.go                        -- User
    errors.go                      -- domain error types

  port/                            -- interfaces (ports in hexagonal arch)
    repository.go                  -- CampaignRepo, DealRepo, EventRepo, ScoringConfigRepo, DiscoveryConfigRepo
    agent_runtime.go               -- AgentRuntime interface
    durable_runtime.go             -- DurableRuntime interface
    auth_provider.go               -- AuthProvider interface
    analytics_provider.go          -- AnalyticsProvider interface
    clock.go                       -- Clock, IDGenerator interfaces

  service/                         -- business logic, depends only on domain + port
    campaign_service.go            -- create, get, list campaigns
    deal_service.go                -- create deals from research, approve/reject, list/filter
    scoring_service.go             -- get/update scoring config
    discovery_service.go           -- get/update discovery config
    event_service.go               -- emit + query domain events
    pipeline_service.go            -- orchestrate research pipeline trigger

  adapter/                         -- implementations of ports
    postgres/
      campaign_repo.go
      deal_repo.go
      event_repo.go
      scoring_config_repo.go
      discovery_config_repo.go
      db.go                        -- connection pool, migrations runner
      migrations/
        001_initial_schema.sql
    supabase/
      auth_provider.go             -- JWT validation, tenant extraction
    inngest/
      client.go                    -- Inngest client setup
      campaign_workflow.go         -- CampaignProcessingWorkflow
      discovery_workflow.go        -- DiscoverySchedulerWorkflow
    openfang/
      agent_runtime.go             -- OpenFang AgentRuntime adapter
      agents/
        sourcing.go                -- Sourcing agent definition
        demand.go                  -- Demand agent definition
        competition.go             -- Competition agent definition
        profitability.go           -- Profitability agent definition
        risk.go                    -- Risk agent definition
        supplier.go                -- Supplier agent definition
        reviewer.go                -- Reviewer agent definition
    posthog/
      analytics_provider.go        -- PostHog event capture + flags

  api/                             -- HTTP handlers
    router.go                      -- route registration
    middleware/
      auth.go                      -- JWT auth middleware
      tenant.go                    -- tenant context injection
      request_id.go                -- correlation ID
    handler/
      health.go                    -- health + readiness
      campaign_handler.go          -- POST/GET /campaigns
      deal_handler.go              -- GET /deals, GET /deals/:id, POST approve/reject
      scoring_handler.go           -- GET/PUT /config/scoring
      discovery_handler.go         -- GET/PUT /discovery
      event_handler.go             -- GET /events
      dashboard_handler.go         -- GET /dashboard/summary
    response/
      json.go                      -- standard JSON response helpers

  config/
    config.go                      -- env-based config struct
```

### Frontend (`apps/web/`)

```
apps/web/
  package.json
  next.config.ts
  tailwind.config.ts
  tsconfig.json

  src/
    app/
      layout.tsx                   -- root layout with providers
      page.tsx                     -- redirect to /dashboard
      (auth)/
        login/page.tsx
        signup/page.tsx
      (app)/
        layout.tsx                 -- app shell: sidebar, top nav
        dashboard/page.tsx         -- KPI cards, recent deals, recent campaigns
        campaigns/
          page.tsx                 -- campaign list
          new/page.tsx             -- create campaign form
          [id]/page.tsx            -- campaign detail + results
        deals/
          page.tsx                 -- deal explorer table
          [id]/page.tsx            -- deal detail: scores, evidence, approval
        discovery/page.tsx         -- continuous discovery config
        settings/page.tsx          -- placeholder

    lib/
      api-client.ts                -- typed fetch wrapper
      supabase.ts                  -- Supabase browser client
      query-keys.ts                -- TanStack Query key factory
      types.ts                     -- shared TypeScript types (mirrors Go domain)

    hooks/
      use-campaigns.ts             -- TanStack Query hooks for campaigns
      use-deals.ts                 -- TanStack Query hooks for deals
      use-scoring.ts               -- TanStack Query hooks for scoring config
      use-discovery.ts             -- TanStack Query hooks for discovery config

    components/
      ui/                          -- shadcn/ui primitives (installed via CLI)
      app-shell.tsx                -- sidebar + top nav
      score-badge.tsx              -- colored score badge component
      status-pill.tsx              -- deal/campaign status pill
      deal-table.tsx               -- deal explorer table with filters
      campaign-card.tsx            -- campaign summary card
      evidence-panel.tsx           -- expandable agent evidence panel
      metric-card.tsx              -- KPI metric card
      page-header.tsx              -- standard page header
      empty-state.tsx              -- empty state placeholder
```

### Infrastructure

```
docker-compose.yml                 -- Postgres, Inngest dev server
Makefile                           -- build, test, migrate, dev commands
.env.example                       -- all required env vars
go.work                            -- Go workspace
go.mod / go.sum
```

---

## Task 1: Repository Bootstrap + Go Module Setup

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.env.example`
- Create: `docker-compose.yml`
- Create: `.gitignore`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/pluriza/Documents/Work/Pluriza/Estori/fba_agent_orchestrator
go mod init github.com/pluriza/fba-agent-orchestrator
```

- [ ] **Step 2: Create `.gitignore`**

```gitignore
# Go
bin/
*.exe
*.test
*.out
vendor/

# Environment
.env
.env.local

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store

# Build
dist/
node_modules/

# Inngest
.inngest/
```

- [ ] **Step 3: Create `.env.example`**

```env
# Supabase
SUPABASE_URL=http://localhost:54321
SUPABASE_ANON_KEY=your-anon-key
SUPABASE_SERVICE_ROLE_KEY=your-service-role-key
SUPABASE_JWT_SECRET=your-jwt-secret

# Database
DATABASE_URL=postgres://postgres:postgres@localhost:54322/postgres?sslmode=disable

# Inngest
INNGEST_EVENT_KEY=test
INNGEST_SIGNING_KEY=test
INNGEST_DEV=true

# OpenFang
OPENFANG_API_URL=http://localhost:3100
OPENFANG_API_KEY=your-openfang-key

# PostHog
POSTHOG_API_KEY=your-posthog-key
POSTHOG_HOST=https://app.posthog.com

# Amazon SP-API
SP_API_CLIENT_ID=your-client-id
SP_API_CLIENT_SECRET=your-client-secret
SP_API_REFRESH_TOKEN=your-refresh-token
SP_API_MARKETPLACE_ID=ATVPDKIKX0DER

# Exa
EXA_API_KEY=your-exa-key

# Firecrawl
FIRECRAWL_API_KEY=your-firecrawl-key

# OpenAI
OPENAI_API_KEY=your-openai-key

# Server
PORT=8080
ENV=development
```

- [ ] **Step 4: Create `docker-compose.yml`**

```yaml
version: "3.9"

services:
  postgres:
    image: postgres:16-alpine
    ports:
      - "54322:5432"
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: fba_orchestrator
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

  inngest:
    image: inngest/inngest:latest
    ports:
      - "8288:8288"
    environment:
      INNGEST_DEV: "1"

volumes:
  pgdata:
```

- [ ] **Step 5: Create `Makefile`**

```makefile
.PHONY: dev test build migrate lint docker-up docker-down

# Start local infrastructure
docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Run Go API server
dev:
	go run ./apps/api/main.go

# Run all Go tests
test:
	go test ./... -v -count=1

# Run a single test by name
test-one:
	go test ./... -v -count=1 -run $(TEST)

# Build API binary
build:
	go build -o bin/api ./apps/api/main.go

# Run database migrations
migrate:
	go run ./apps/api/main.go migrate

# Lint
lint:
	golangci-lint run ./...

# Frontend dev
web-dev:
	cd apps/web && npm run dev

web-install:
	cd apps/web && npm install

web-build:
	cd apps/web && npm run build
```

- [ ] **Step 6: Commit**

```bash
git add go.mod Makefile .env.example docker-compose.yml .gitignore
git commit -m "chore: bootstrap Go module, Makefile, docker-compose, env config"
```

---

## Task 2: Domain Types

**Files:**
- Create: `internal/domain/tenant.go`
- Create: `internal/domain/user.go`
- Create: `internal/domain/campaign.go`
- Create: `internal/domain/deal.go`
- Create: `internal/domain/scoring.go`
- Create: `internal/domain/discovery.go`
- Create: `internal/domain/research.go`
- Create: `internal/domain/event.go`
- Create: `internal/domain/approval.go`
- Create: `internal/domain/supplier.go`
- Create: `internal/domain/errors.go`

- [ ] **Step 1: Create `internal/domain/errors.go`**

```go
package domain

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrValidation        = errors.New("validation error")
	ErrConflict          = errors.New("conflict")
)
```

- [ ] **Step 2: Create `internal/domain/tenant.go`**

```go
package domain

import "time"

type TenantID string
type UserID string

type Tenant struct {
	ID        TenantID  `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Membership struct {
	TenantID TenantID `json:"tenant_id"`
	UserID   UserID   `json:"user_id"`
	Role     Role     `json:"role"`
}

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)
```

- [ ] **Step 3: Create `internal/domain/user.go`**

```go
package domain

import "time"

type User struct {
	ID        UserID    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 4: Create `internal/domain/scoring.go`**

```go
package domain

import "time"

type ScoringConfigID string

type ScoringConfig struct {
	ID         ScoringConfigID `json:"id"`
	TenantID   TenantID        `json:"tenant_id"`
	Version    int             `json:"version"`
	Weights    ScoringWeights  `json:"weights"`
	Thresholds Thresholds      `json:"thresholds"`
	CreatedBy  string          `json:"created_by"` // "user" | "autoresearch"
	Active     bool            `json:"active"`
	CreatedAt  time.Time       `json:"created_at"`
}

type ScoringWeights struct {
	Demand      float64 `json:"demand"`
	Competition float64 `json:"competition"`
	Margin      float64 `json:"margin"`
	Risk        float64 `json:"risk"`
	Sourcing    float64 `json:"sourcing"`
}

type Thresholds struct {
	MinOverall      int `json:"min_overall"`
	MinPerDimension int `json:"min_per_dimension"`
}

// DefaultScoringWeights returns the initial scoring weights.
func DefaultScoringWeights() ScoringWeights {
	return ScoringWeights{
		Demand:      0.25,
		Competition: 0.20,
		Margin:      0.25,
		Risk:        0.15,
		Sourcing:    0.15,
	}
}

// DefaultThresholds returns the initial thresholds.
func DefaultThresholds() Thresholds {
	return Thresholds{
		MinOverall:      8,
		MinPerDimension: 6,
	}
}
```

- [ ] **Step 5: Create `internal/domain/campaign.go`**

```go
package domain

import (
	"fmt"
	"time"
)

type CampaignID string

type CampaignType string

const (
	CampaignTypeDiscoveryRun CampaignType = "discovery_run"
	CampaignTypeManual       CampaignType = "manual"
	CampaignTypeExperiment   CampaignType = "experiment"
)

type CampaignStatus string

const (
	CampaignStatusPending   CampaignStatus = "pending"
	CampaignStatusRunning   CampaignStatus = "running"
	CampaignStatusCompleted CampaignStatus = "completed"
	CampaignStatusFailed    CampaignStatus = "failed"
)

type Campaign struct {
	ID              CampaignID      `json:"id"`
	TenantID        TenantID        `json:"tenant_id"`
	Type            CampaignType    `json:"type"`
	Criteria        Criteria        `json:"criteria"`
	ScoringConfigID ScoringConfigID `json:"scoring_config_id"`
	ExperimentID    *string         `json:"experiment_id,omitempty"`
	SourceFile      *string         `json:"source_file,omitempty"`
	Status          CampaignStatus  `json:"status"`
	CreatedBy       string          `json:"created_by"` // "user" | "system" | "autoresearch"
	TriggerType     TriggerType     `json:"trigger_type"`
	CreatedAt       time.Time       `json:"created_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
}

type TriggerType string

const (
	TriggerChat        TriggerType = "chat"
	TriggerDashboard   TriggerType = "dashboard"
	TriggerScheduler   TriggerType = "scheduler"
	TriggerSpreadsheet TriggerType = "spreadsheet"
)

type Criteria struct {
	Keywords          []string `json:"keywords"`
	MinMonthlyRevenue *int     `json:"min_monthly_revenue,omitempty"`
	MinMarginPct      *float64 `json:"min_margin_pct,omitempty"`
	MaxWholesaleCost  *float64 `json:"max_wholesale_cost,omitempty"`
	MaxMOQ            *int     `json:"max_moq,omitempty"`
	PreferredBrands   []string `json:"preferred_brands,omitempty"`
	Marketplace       string   `json:"marketplace"` // "US", "EU", "UK"
}

// Transition advances campaign status. Returns error if transition is invalid.
func (c *Campaign) Transition(to CampaignStatus) error {
	valid := map[CampaignStatus][]CampaignStatus{
		CampaignStatusPending: {CampaignStatusRunning, CampaignStatusFailed},
		CampaignStatusRunning: {CampaignStatusCompleted, CampaignStatusFailed},
	}
	for _, allowed := range valid[c.Status] {
		if to == allowed {
			c.Status = to
			if to == CampaignStatusCompleted || to == CampaignStatusFailed {
				now := time.Now()
				c.CompletedAt = &now
			}
			return nil
		}
	}
	return fmt.Errorf("%w: cannot transition campaign from %s to %s", ErrInvalidTransition, c.Status, to)
}
```

- [ ] **Step 6: Create `internal/domain/deal.go`**

```go
package domain

import (
	"fmt"
	"time"
)

type DealID string

type DealStatus string

const (
	DealStatusDiscovered  DealStatus = "discovered"
	DealStatusAnalyzing   DealStatus = "analyzing"
	DealStatusNeedsReview DealStatus = "needs_review"
	DealStatusApproved    DealStatus = "approved"
	DealStatusRejected    DealStatus = "rejected"
	// Post-MVP statuses included for state machine completeness
	DealStatusSourcing   DealStatus = "sourcing"
	DealStatusProcuring  DealStatus = "procuring"
	DealStatusListing    DealStatus = "listing"
	DealStatusLive       DealStatus = "live"
	DealStatusMonitoring DealStatus = "monitoring"
	DealStatusReorder    DealStatus = "reorder"
	DealStatusArchived   DealStatus = "archived"
)

var validDealTransitions = map[DealStatus][]DealStatus{
	DealStatusDiscovered:  {DealStatusAnalyzing},
	DealStatusAnalyzing:   {DealStatusNeedsReview, DealStatusRejected},
	DealStatusNeedsReview: {DealStatusApproved, DealStatusRejected},
	DealStatusApproved:    {DealStatusSourcing, DealStatusArchived},
	DealStatusSourcing:    {DealStatusProcuring, DealStatusArchived},
	DealStatusProcuring:   {DealStatusListing, DealStatusArchived},
	DealStatusListing:     {DealStatusLive, DealStatusArchived},
	DealStatusLive:        {DealStatusMonitoring, DealStatusArchived},
	DealStatusMonitoring:  {DealStatusReorder, DealStatusArchived},
	DealStatusReorder:     {DealStatusProcuring, DealStatusArchived},
}

type Deal struct {
	ID              DealID     `json:"id"`
	TenantID        TenantID   `json:"tenant_id"`
	CampaignID      CampaignID `json:"campaign_id"`
	ASIN            string     `json:"asin"`
	Title           string     `json:"title"`
	Brand           string     `json:"brand"`
	Category        string     `json:"category"`
	Status          DealStatus `json:"status"`
	Scores          DealScores `json:"scores"`
	Evidence        Evidence   `json:"evidence"`
	ReviewerVerdict string     `json:"reviewer_verdict"`
	IterationCount  int        `json:"iteration_count"`
	SupplierID      *string    `json:"supplier_id,omitempty"`
	ListingID       *string    `json:"listing_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type DealScores struct {
	Demand              int     `json:"demand"`
	Competition         int     `json:"competition"`
	Margin              int     `json:"margin"`
	Risk                int     `json:"risk"`
	SourcingFeasibility int     `json:"sourcing_feasibility"`
	Overall             float64 `json:"overall"`
}

type Evidence struct {
	Demand      AgentEvidence `json:"demand"`
	Competition AgentEvidence `json:"competition"`
	Margin      AgentEvidence `json:"margin"`
	Risk        AgentEvidence `json:"risk"`
	Sourcing    AgentEvidence `json:"sourcing"`
}

type AgentEvidence struct {
	Reasoning string         `json:"reasoning"`
	Data      map[string]any `json:"data"`
}

// Transition advances deal status. Returns error if transition is invalid.
func (d *Deal) Transition(to DealStatus) error {
	for _, allowed := range validDealTransitions[d.Status] {
		if to == allowed {
			d.Status = to
			d.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("%w: cannot transition deal from %s to %s", ErrInvalidTransition, d.Status, to)
}
```

- [ ] **Step 7: Create `internal/domain/research.go`**

```go
package domain

// ResearchResult is the structured output from the research pipeline.
// Produced by the agent runtime, consumed by the Operations Core.
type ResearchResult struct {
	CampaignID    CampaignID        `json:"campaign_id"`
	Candidates    []CandidateResult `json:"candidates"`
	ResearchTrail []AgentTrailEntry `json:"research_trail"`
	Summary       string            `json:"summary"`
}

type CandidateResult struct {
	ASIN               string              `json:"asin"`
	Title              string              `json:"title"`
	Brand              string              `json:"brand"`
	Category           string              `json:"category"`
	Scores             DealScores          `json:"scores"`
	Evidence           Evidence            `json:"evidence"`
	SupplierCandidates []SupplierCandidate `json:"supplier_candidates"`
	OutreachDrafts     []string            `json:"outreach_drafts"`
	ReviewerVerdict    string              `json:"reviewer_verdict"`
	IterationCount     int                 `json:"iteration_count"`
}

type SupplierCandidate struct {
	Company       string  `json:"company"`
	Contact       string  `json:"contact"`
	UnitPrice     float64 `json:"unit_price"`
	MOQ           int     `json:"moq"`
	LeadTimeDays  int     `json:"lead_time_days"`
	ShippingTerms string  `json:"shipping_terms"`
	Authorized    bool    `json:"authorized"`
}

type AgentTrailEntry struct {
	AgentName  string `json:"agent_name"`
	ASIN       string `json:"asin"`
	Iteration  int    `json:"iteration"`
	Input      any    `json:"input"`
	Output     any    `json:"output"`
	DurationMs int64  `json:"duration_ms"`
}
```

- [ ] **Step 8: Create `internal/domain/discovery.go`**

```go
package domain

import "time"

type DiscoveryConfigID string

type DiscoveryConfig struct {
	ID              DiscoveryConfigID `json:"id"`
	TenantID        TenantID          `json:"tenant_id"`
	Categories      []string          `json:"categories"`
	BaselineCriteria Criteria          `json:"baseline_criteria"`
	ScoringConfigID ScoringConfigID   `json:"scoring_config_id"`
	Cadence         string            `json:"cadence"` // "nightly" | "twice_daily" | "weekly"
	Enabled         bool              `json:"enabled"`
	LastRunAt       *time.Time        `json:"last_run_at,omitempty"`
	NextRunAt       *time.Time        `json:"next_run_at,omitempty"`
}
```

- [ ] **Step 9: Create `internal/domain/event.go`**

```go
package domain

import "time"

type DomainEventID string

type DomainEvent struct {
	ID            DomainEventID  `json:"id"`
	TenantID      TenantID       `json:"tenant_id"`
	EventType     string         `json:"event_type"`
	EntityType    string         `json:"entity_type"`
	EntityID      string         `json:"entity_id"`
	Payload       map[string]any `json:"payload"`
	CorrelationID string         `json:"correlation_id"`
	ActorID       string         `json:"actor_id"`
	Timestamp     time.Time      `json:"timestamp"`
}
```

- [ ] **Step 10: Create `internal/domain/approval.go`**

```go
package domain

import "time"

type ApprovalID string

type ApprovalDecision string

const (
	ApprovalDecisionApproved ApprovalDecision = "approved"
	ApprovalDecisionRejected ApprovalDecision = "rejected"
)

type Approval struct {
	ID         ApprovalID       `json:"id"`
	TenantID   TenantID         `json:"tenant_id"`
	EntityType string           `json:"entity_type"`
	EntityID   string           `json:"entity_id"`
	RequestedBy string          `json:"requested_by"`
	DecidedBy  *string          `json:"decided_by,omitempty"`
	Decision   *ApprovalDecision `json:"decision,omitempty"`
	Reason     *string          `json:"reason,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
	DecidedAt  *time.Time       `json:"decided_at,omitempty"`
}
```

- [ ] **Step 11: Create `internal/domain/supplier.go`** (MVP stub — full implementation in Phase 2)

```go
package domain

import "time"

type SupplierID string

type Supplier struct {
	ID                  SupplierID `json:"id"`
	TenantID            TenantID   `json:"tenant_id"`
	Name                string     `json:"name"`
	Website             string     `json:"website"`
	AuthorizationStatus string     `json:"authorization_status"`
	ReliabilityScore    *float64   `json:"reliability_score,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
```

- [ ] **Step 12: Run `go build ./...` to verify all types compile**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 13: Commit**

```bash
git add internal/domain/
git commit -m "feat: add domain types — campaign, deal, scoring, research, events"
```

---

## Task 3: Port Interfaces

**Files:**
- Create: `internal/port/repository.go`
- Create: `internal/port/agent_runtime.go`
- Create: `internal/port/durable_runtime.go`
- Create: `internal/port/auth_provider.go`
- Create: `internal/port/analytics_provider.go`
- Create: `internal/port/clock.go`

- [ ] **Step 1: Create `internal/port/repository.go`**

```go
package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type CampaignRepo interface {
	Create(ctx context.Context, c *domain.Campaign) error
	GetByID(ctx context.Context, tenantID domain.TenantID, id domain.CampaignID) (*domain.Campaign, error)
	List(ctx context.Context, tenantID domain.TenantID, filter CampaignFilter) ([]domain.Campaign, error)
	Update(ctx context.Context, c *domain.Campaign) error
}

type CampaignFilter struct {
	Status *domain.CampaignStatus
	Type   *domain.CampaignType
	Limit  int
	Offset int
}

type DealRepo interface {
	Create(ctx context.Context, d *domain.Deal) error
	CreateBatch(ctx context.Context, deals []domain.Deal) error
	GetByID(ctx context.Context, tenantID domain.TenantID, id domain.DealID) (*domain.Deal, error)
	List(ctx context.Context, tenantID domain.TenantID, filter DealFilter) ([]domain.Deal, int, error) // returns deals, total count, error
	Update(ctx context.Context, d *domain.Deal) error
}

type DealFilter struct {
	CampaignID *domain.CampaignID
	Status     *domain.DealStatus
	MinScore   *float64
	Search     *string // search in title, brand, ASIN
	SortBy     string  // "overall_score", "created_at", "margin"
	SortDir    string  // "asc", "desc"
	Limit      int
	Offset     int
}

type EventRepo interface {
	Create(ctx context.Context, e *domain.DomainEvent) error
	List(ctx context.Context, tenantID domain.TenantID, filter EventFilter) ([]domain.DomainEvent, error)
}

type EventFilter struct {
	EntityType *string
	EntityID   *string
	EventType  *string
	Limit      int
	Offset     int
}

type ScoringConfigRepo interface {
	Create(ctx context.Context, sc *domain.ScoringConfig) error
	GetActive(ctx context.Context, tenantID domain.TenantID) (*domain.ScoringConfig, error)
	GetByID(ctx context.Context, tenantID domain.TenantID, id domain.ScoringConfigID) (*domain.ScoringConfig, error)
	SetActive(ctx context.Context, tenantID domain.TenantID, id domain.ScoringConfigID) error
}

type DiscoveryConfigRepo interface {
	Get(ctx context.Context, tenantID domain.TenantID) (*domain.DiscoveryConfig, error)
	Upsert(ctx context.Context, dc *domain.DiscoveryConfig) error
}
```

- [ ] **Step 2: Create `internal/port/agent_runtime.go`**

```go
package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// AgentRuntime abstracts the agent orchestration system (OpenFang, ZeroClaw, etc.).
// The Operations Core never calls OpenFang directly — only through this interface.
type AgentRuntime interface {
	// RunResearchPipeline executes the full 7-agent quality-gated pipeline.
	// Returns structured research results or an error.
	// This is a long-running operation — callers should use it within a durable workflow.
	RunResearchPipeline(ctx context.Context, input PipelineInput) (*domain.ResearchResult, error)
}

type PipelineInput struct {
	CampaignID    domain.CampaignID    `json:"campaign_id"`
	TenantID      domain.TenantID      `json:"tenant_id"`
	Criteria      domain.Criteria      `json:"criteria"`
	ScoringConfig domain.ScoringConfig `json:"scoring_config"`
	SourceASINs   []string             `json:"source_asins,omitempty"` // for spreadsheet uploads
}
```

- [ ] **Step 3: Create `internal/port/durable_runtime.go`**

```go
package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// DurableRuntime abstracts durable execution (Inngest, Temporal, etc.).
type DurableRuntime interface {
	// TriggerCampaignProcessing kicks off the campaign research workflow.
	TriggerCampaignProcessing(ctx context.Context, campaignID domain.CampaignID, tenantID domain.TenantID) error

	// TriggerDiscoveryRun kicks off a scheduled discovery campaign.
	TriggerDiscoveryRun(ctx context.Context, tenantID domain.TenantID) error
}
```

- [ ] **Step 4: Create `internal/port/auth_provider.go`**

```go
package port

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

// AuthContext is extracted from the request by auth middleware.
type AuthContext struct {
	UserID   domain.UserID
	TenantID domain.TenantID
	Role     domain.Role
}

// AuthProvider abstracts auth token validation (Supabase, etc.).
type AuthProvider interface {
	// ValidateToken validates a JWT and returns the auth context.
	ValidateToken(ctx context.Context, token string) (*AuthContext, error)
}
```

- [ ] **Step 5: Create `internal/port/analytics_provider.go`**

```go
package port

import "context"

// AnalyticsProvider abstracts analytics/event capture (PostHog, etc.).
type AnalyticsProvider interface {
	// CaptureEvent sends a domain event to the analytics system.
	CaptureEvent(ctx context.Context, distinctID string, eventName string, properties map[string]any) error

	// IsFeatureEnabled checks a feature flag.
	IsFeatureEnabled(ctx context.Context, flagKey string, distinctID string) (bool, error)
}
```

- [ ] **Step 6: Create `internal/port/clock.go`**

```go
package port

import "time"

// Clock abstracts time for testing.
type Clock interface {
	Now() time.Time
}

// RealClock returns actual system time.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

// IDGenerator abstracts ID generation for testing.
type IDGenerator interface {
	New() string
}
```

- [ ] **Step 7: Run `go build ./...` to verify compilation**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 8: Commit**

```bash
git add internal/port/
git commit -m "feat: add port interfaces — repos, agent runtime, durable runtime, auth, analytics"
```

---

## Task 4: Configuration + App Entrypoint

**Files:**
- Create: `internal/config/config.go`
- Create: `apps/api/main.go`

- [ ] **Step 1: Install dependencies**

```bash
go get github.com/joho/godotenv
go get github.com/go-chi/chi/v5
```

- [ ] **Step 2: Create `internal/config/config.go`**

```go
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        int
	Env         string
	DatabaseURL string

	SupabaseURL            string
	SupabaseAnonKey        string
	SupabaseServiceRoleKey string
	SupabaseJWTSecret      string

	InngestEventKey   string
	InngestSigningKey string
	InngestDev        bool

	OpenFangAPIURL string
	OpenFangAPIKey string

	PostHogAPIKey string
	PostHogHost   string

	SPAPIClientID     string
	SPAPIClientSecret string
	SPAPIRefreshToken string
	SPAPIMarketplace  string

	ExaAPIKey       string
	FirecrawlAPIKey string
	OpenAIAPIKey    string
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	inngestDev, _ := strconv.ParseBool(getEnv("INNGEST_DEV", "true"))

	cfg := &Config{
		Port:        port,
		Env:         getEnv("ENV", "development"),
		DatabaseURL: mustEnv("DATABASE_URL"),

		SupabaseURL:            getEnv("SUPABASE_URL", ""),
		SupabaseAnonKey:        getEnv("SUPABASE_ANON_KEY", ""),
		SupabaseServiceRoleKey: getEnv("SUPABASE_SERVICE_ROLE_KEY", ""),
		SupabaseJWTSecret:      getEnv("SUPABASE_JWT_SECRET", ""),

		InngestEventKey:   getEnv("INNGEST_EVENT_KEY", "test"),
		InngestSigningKey: getEnv("INNGEST_SIGNING_KEY", "test"),
		InngestDev:        inngestDev,

		OpenFangAPIURL: getEnv("OPENFANG_API_URL", ""),
		OpenFangAPIKey: getEnv("OPENFANG_API_KEY", ""),

		PostHogAPIKey: getEnv("POSTHOG_API_KEY", ""),
		PostHogHost:   getEnv("POSTHOG_HOST", "https://app.posthog.com"),

		SPAPIClientID:     getEnv("SP_API_CLIENT_ID", ""),
		SPAPIClientSecret: getEnv("SP_API_CLIENT_SECRET", ""),
		SPAPIRefreshToken: getEnv("SP_API_REFRESH_TOKEN", ""),
		SPAPIMarketplace:  getEnv("SP_API_MARKETPLACE_ID", "ATVPDKIKX0DER"),

		ExaAPIKey:       getEnv("EXA_API_KEY", ""),
		FirecrawlAPIKey: getEnv("FIRECRAWL_API_KEY", ""),
		OpenAIAPIKey:    getEnv("OPENAI_API_KEY", ""),
	}

	return cfg, nil
}

func (c *Config) IsDev() bool {
	return c.Env == "development"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required env var %s is not set", key))
	}
	return v
}
```

- [ ] **Step 3: Create `apps/api/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pluriza/fba-agent-orchestrator/internal/config"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// TODO: wire up database, repos, services, handlers in subsequent tasks
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("starting server", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}
```

- [ ] **Step 4: Verify it compiles and starts**

```bash
DATABASE_URL=postgres://localhost/test go build ./apps/api/...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/config/ apps/api/
git commit -m "feat: add config loader and API entrypoint with health endpoint"
```

---

## Task 5: Database Schema + Postgres Repos

**Files:**
- Create: `internal/adapter/postgres/db.go`
- Create: `internal/adapter/postgres/migrations/001_initial_schema.sql`
- Create: `internal/adapter/postgres/campaign_repo.go`
- Create: `internal/adapter/postgres/deal_repo.go`
- Create: `internal/adapter/postgres/event_repo.go`
- Create: `internal/adapter/postgres/scoring_config_repo.go`
- Create: `internal/adapter/postgres/discovery_config_repo.go`
- Test: `internal/adapter/postgres/campaign_repo_test.go`
- Test: `internal/adapter/postgres/deal_repo_test.go`

- [ ] **Step 1: Install postgres driver**

```bash
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/pgxpool
```

- [ ] **Step 2: Create migration `internal/adapter/postgres/migrations/001_initial_schema.sql`**

```sql
-- Scoring configs
CREATE TABLE scoring_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    version INT NOT NULL DEFAULT 1,
    weights JSONB NOT NULL,
    thresholds JSONB NOT NULL,
    created_by TEXT NOT NULL DEFAULT 'user',
    active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_scoring_configs_tenant_active ON scoring_configs (tenant_id, active) WHERE active = true;

-- Discovery configs
CREATE TABLE discovery_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL UNIQUE,
    categories JSONB NOT NULL DEFAULT '[]',
    baseline_criteria JSONB NOT NULL DEFAULT '{}',
    scoring_config_id UUID REFERENCES scoring_configs(id),
    cadence TEXT NOT NULL DEFAULT 'nightly',
    enabled BOOLEAN NOT NULL DEFAULT false,
    last_run_at TIMESTAMPTZ,
    next_run_at TIMESTAMPTZ
);

-- Campaigns
CREATE TABLE campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    type TEXT NOT NULL,
    criteria JSONB NOT NULL,
    scoring_config_id UUID REFERENCES scoring_configs(id),
    experiment_id UUID,
    source_file TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_by TEXT NOT NULL DEFAULT 'user',
    trigger_type TEXT NOT NULL DEFAULT 'dashboard',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_campaigns_tenant ON campaigns (tenant_id);
CREATE INDEX idx_campaigns_tenant_status ON campaigns (tenant_id, status);

-- Deals
CREATE TABLE deals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    campaign_id UUID NOT NULL REFERENCES campaigns(id),
    asin TEXT NOT NULL,
    title TEXT NOT NULL,
    brand TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'discovered',
    scores JSONB NOT NULL DEFAULT '{}',
    evidence JSONB NOT NULL DEFAULT '{}',
    reviewer_verdict TEXT NOT NULL DEFAULT '',
    iteration_count INT NOT NULL DEFAULT 0,
    supplier_id UUID,
    listing_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_deals_tenant ON deals (tenant_id);
CREATE INDEX idx_deals_tenant_status ON deals (tenant_id, status);
CREATE INDEX idx_deals_campaign ON deals (campaign_id);
CREATE INDEX idx_deals_asin ON deals (tenant_id, asin);

-- Domain events
CREATE TABLE domain_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    correlation_id TEXT NOT NULL DEFAULT '',
    actor_id TEXT NOT NULL DEFAULT '',
    timestamp TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_domain_events_tenant ON domain_events (tenant_id);
CREATE INDEX idx_domain_events_entity ON domain_events (tenant_id, entity_type, entity_id);
CREATE INDEX idx_domain_events_type ON domain_events (tenant_id, event_type);
```

- [ ] **Step 3: Create `internal/adapter/postgres/db.go`**

```go
package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, thisFile, _, _ := runtime.Caller(0)
	migrationsDir := filepath.Join(filepath.Dir(thisFile), "migrations")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Create migrations tracking table
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}

		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)", entry.Name()).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", entry.Name(), err)
		}
		if exists {
			continue
		}

		sql, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		_, err = pool.Exec(ctx, string(sql))
		if err != nil {
			return fmt.Errorf("run migration %s: %w", entry.Name(), err)
		}

		_, err = pool.Exec(ctx, "INSERT INTO schema_migrations (filename) VALUES ($1)", entry.Name())
		if err != nil {
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}
```

- [ ] **Step 4: Create `internal/adapter/postgres/campaign_repo.go`**

```go
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type CampaignRepo struct {
	pool *pgxpool.Pool
}

func NewCampaignRepo(pool *pgxpool.Pool) *CampaignRepo {
	return &CampaignRepo{pool: pool}
}

func (r *CampaignRepo) Create(ctx context.Context, c *domain.Campaign) error {
	criteria, err := json.Marshal(c.Criteria)
	if err != nil {
		return fmt.Errorf("marshal criteria: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO campaigns (id, tenant_id, type, criteria, scoring_config_id, experiment_id, source_file, status, created_by, trigger_type, created_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, c.ID, c.TenantID, c.Type, criteria, c.ScoringConfigID, c.ExperimentID, c.SourceFile, c.Status, c.CreatedBy, c.TriggerType, c.CreatedAt, c.CompletedAt)
	if err != nil {
		return fmt.Errorf("insert campaign: %w", err)
	}
	return nil
}

func (r *CampaignRepo) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.CampaignID) (*domain.Campaign, error) {
	var c domain.Campaign
	var criteriaJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, type, criteria, scoring_config_id, experiment_id, source_file, status, created_by, trigger_type, created_at, completed_at
		FROM campaigns
		WHERE id = $1 AND tenant_id = $2
	`, id, tenantID).Scan(
		&c.ID, &c.TenantID, &c.Type, &criteriaJSON, &c.ScoringConfigID, &c.ExperimentID, &c.SourceFile, &c.Status, &c.CreatedBy, &c.TriggerType, &c.CreatedAt, &c.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get campaign: %w", err)
	}
	if err := json.Unmarshal(criteriaJSON, &c.Criteria); err != nil {
		return nil, fmt.Errorf("unmarshal criteria: %w", err)
	}
	return &c, nil
}

func (r *CampaignRepo) List(ctx context.Context, tenantID domain.TenantID, filter port.CampaignFilter) ([]domain.Campaign, error) {
	query := `SELECT id, tenant_id, type, criteria, scoring_config_id, experiment_id, source_file, status, created_by, trigger_type, created_at, completed_at
		FROM campaigns WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2

	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Type != nil {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, *filter.Type)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []domain.Campaign
	for rows.Next() {
		var c domain.Campaign
		var criteriaJSON []byte
		if err := rows.Scan(&c.ID, &c.TenantID, &c.Type, &criteriaJSON, &c.ScoringConfigID, &c.ExperimentID, &c.SourceFile, &c.Status, &c.CreatedBy, &c.TriggerType, &c.CreatedAt, &c.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan campaign: %w", err)
		}
		if err := json.Unmarshal(criteriaJSON, &c.Criteria); err != nil {
			return nil, fmt.Errorf("unmarshal criteria: %w", err)
		}
		campaigns = append(campaigns, c)
	}
	return campaigns, nil
}

func (r *CampaignRepo) Update(ctx context.Context, c *domain.Campaign) error {
	criteria, err := json.Marshal(c.Criteria)
	if err != nil {
		return fmt.Errorf("marshal criteria: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE campaigns SET status = $1, completed_at = $2, criteria = $3 WHERE id = $4 AND tenant_id = $5
	`, c.Status, c.CompletedAt, criteria, c.ID, c.TenantID)
	if err != nil {
		return fmt.Errorf("update campaign: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Create `internal/adapter/postgres/deal_repo.go`**

```go
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type DealRepo struct {
	pool *pgxpool.Pool
}

func NewDealRepo(pool *pgxpool.Pool) *DealRepo {
	return &DealRepo{pool: pool}
}

func (r *DealRepo) Create(ctx context.Context, d *domain.Deal) error {
	scores, _ := json.Marshal(d.Scores)
	evidence, _ := json.Marshal(d.Evidence)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO deals (id, tenant_id, campaign_id, asin, title, brand, category, status, scores, evidence, reviewer_verdict, iteration_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, d.ID, d.TenantID, d.CampaignID, d.ASIN, d.Title, d.Brand, d.Category, d.Status, scores, evidence, d.ReviewerVerdict, d.IterationCount, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert deal: %w", err)
	}
	return nil
}

func (r *DealRepo) CreateBatch(ctx context.Context, deals []domain.Deal) error {
	for _, d := range deals {
		if err := r.Create(ctx, &d); err != nil {
			return err
		}
	}
	return nil
}

func (r *DealRepo) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.DealID) (*domain.Deal, error) {
	var d domain.Deal
	var scoresJSON, evidenceJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, campaign_id, asin, title, brand, category, status, scores, evidence, reviewer_verdict, iteration_count, supplier_id, listing_id, created_at, updated_at
		FROM deals WHERE id = $1 AND tenant_id = $2
	`, id, tenantID).Scan(
		&d.ID, &d.TenantID, &d.CampaignID, &d.ASIN, &d.Title, &d.Brand, &d.Category, &d.Status,
		&scoresJSON, &evidenceJSON, &d.ReviewerVerdict, &d.IterationCount,
		&d.SupplierID, &d.ListingID, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get deal: %w", err)
	}
	json.Unmarshal(scoresJSON, &d.Scores)
	json.Unmarshal(evidenceJSON, &d.Evidence)
	return &d, nil
}

func (r *DealRepo) List(ctx context.Context, tenantID domain.TenantID, filter port.DealFilter) ([]domain.Deal, int, error) {
	// Count query
	countQuery := "SELECT COUNT(*) FROM deals WHERE tenant_id = $1"
	countArgs := []any{tenantID}
	argIdx := 2

	if filter.CampaignID != nil {
		countQuery += fmt.Sprintf(" AND campaign_id = $%d", argIdx)
		countArgs = append(countArgs, *filter.CampaignID)
		argIdx++
	}
	if filter.Status != nil {
		countQuery += fmt.Sprintf(" AND status = $%d", argIdx)
		countArgs = append(countArgs, *filter.Status)
		argIdx++
	}
	if filter.Search != nil {
		countQuery += fmt.Sprintf(" AND (title ILIKE $%d OR brand ILIKE $%d OR asin ILIKE $%d)", argIdx, argIdx, argIdx)
		countArgs = append(countArgs, "%"+*filter.Search+"%")
		argIdx++
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count deals: %w", err)
	}

	// Data query
	query := `SELECT id, tenant_id, campaign_id, asin, title, brand, category, status, scores, evidence, reviewer_verdict, iteration_count, supplier_id, listing_id, created_at, updated_at
		FROM deals WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx = 2

	if filter.CampaignID != nil {
		query += fmt.Sprintf(" AND campaign_id = $%d", argIdx)
		args = append(args, *filter.CampaignID)
		argIdx++
	}
	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.Search != nil {
		query += fmt.Sprintf(" AND (title ILIKE $%d OR brand ILIKE $%d OR asin ILIKE $%d)", argIdx, argIdx, argIdx)
		args = append(args, "%"+*filter.Search+"%")
		argIdx++
	}

	sortCol := "created_at"
	switch filter.SortBy {
	case "overall_score":
		sortCol = "(scores->>'overall')::float"
	case "margin":
		sortCol = "(scores->>'margin')::int"
	case "created_at":
		sortCol = "created_at"
	}
	sortDir := "DESC"
	if filter.SortDir == "asc" {
		sortDir = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortCol, sortDir)

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list deals: %w", err)
	}
	defer rows.Close()

	var deals []domain.Deal
	for rows.Next() {
		var d domain.Deal
		var scoresJSON, evidenceJSON []byte
		if err := rows.Scan(&d.ID, &d.TenantID, &d.CampaignID, &d.ASIN, &d.Title, &d.Brand, &d.Category, &d.Status, &scoresJSON, &evidenceJSON, &d.ReviewerVerdict, &d.IterationCount, &d.SupplierID, &d.ListingID, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan deal: %w", err)
		}
		json.Unmarshal(scoresJSON, &d.Scores)
		json.Unmarshal(evidenceJSON, &d.Evidence)
		deals = append(deals, d)
	}
	return deals, total, nil
}

func (r *DealRepo) Update(ctx context.Context, d *domain.Deal) error {
	scores, _ := json.Marshal(d.Scores)
	evidence, _ := json.Marshal(d.Evidence)

	_, err := r.pool.Exec(ctx, `
		UPDATE deals SET status = $1, scores = $2, evidence = $3, reviewer_verdict = $4, iteration_count = $5, supplier_id = $6, listing_id = $7, updated_at = $8
		WHERE id = $9 AND tenant_id = $10
	`, d.Status, scores, evidence, d.ReviewerVerdict, d.IterationCount, d.SupplierID, d.ListingID, d.UpdatedAt, d.ID, d.TenantID)
	if err != nil {
		return fmt.Errorf("update deal: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: Create `internal/adapter/postgres/event_repo.go`**

```go
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type EventRepo struct {
	pool *pgxpool.Pool
}

func NewEventRepo(pool *pgxpool.Pool) *EventRepo {
	return &EventRepo{pool: pool}
}

func (r *EventRepo) Create(ctx context.Context, e *domain.DomainEvent) error {
	payload, _ := json.Marshal(e.Payload)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO domain_events (id, tenant_id, event_type, entity_type, entity_id, payload, correlation_id, actor_id, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, e.ID, e.TenantID, e.EventType, e.EntityType, e.EntityID, payload, e.CorrelationID, e.ActorID, e.Timestamp)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func (r *EventRepo) List(ctx context.Context, tenantID domain.TenantID, filter port.EventFilter) ([]domain.DomainEvent, error) {
	query := `SELECT id, tenant_id, event_type, entity_type, entity_id, payload, correlation_id, actor_id, timestamp
		FROM domain_events WHERE tenant_id = $1`
	args := []any{tenantID}
	argIdx := 2

	if filter.EntityType != nil {
		query += fmt.Sprintf(" AND entity_type = $%d", argIdx)
		args = append(args, *filter.EntityType)
		argIdx++
	}
	if filter.EntityID != nil {
		query += fmt.Sprintf(" AND entity_id = $%d", argIdx)
		args = append(args, *filter.EntityID)
		argIdx++
	}
	if filter.EventType != nil {
		query += fmt.Sprintf(" AND event_type = $%d", argIdx)
		args = append(args, *filter.EventType)
		argIdx++
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []domain.DomainEvent
	for rows.Next() {
		var e domain.DomainEvent
		var payloadJSON []byte
		if err := rows.Scan(&e.ID, &e.TenantID, &e.EventType, &e.EntityType, &e.EntityID, &payloadJSON, &e.CorrelationID, &e.ActorID, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		json.Unmarshal(payloadJSON, &e.Payload)
		events = append(events, e)
	}
	return events, nil
}
```

- [ ] **Step 7: Create `internal/adapter/postgres/scoring_config_repo.go`**

```go
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type ScoringConfigRepo struct {
	pool *pgxpool.Pool
}

func NewScoringConfigRepo(pool *pgxpool.Pool) *ScoringConfigRepo {
	return &ScoringConfigRepo{pool: pool}
}

func (r *ScoringConfigRepo) Create(ctx context.Context, sc *domain.ScoringConfig) error {
	weights, _ := json.Marshal(sc.Weights)
	thresholds, _ := json.Marshal(sc.Thresholds)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO scoring_configs (id, tenant_id, version, weights, thresholds, created_by, active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, sc.ID, sc.TenantID, sc.Version, weights, thresholds, sc.CreatedBy, sc.Active, sc.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert scoring config: %w", err)
	}
	return nil
}

func (r *ScoringConfigRepo) GetActive(ctx context.Context, tenantID domain.TenantID) (*domain.ScoringConfig, error) {
	var sc domain.ScoringConfig
	var weightsJSON, thresholdsJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, version, weights, thresholds, created_by, active, created_at
		FROM scoring_configs WHERE tenant_id = $1 AND active = true
	`, tenantID).Scan(&sc.ID, &sc.TenantID, &sc.Version, &weightsJSON, &thresholdsJSON, &sc.CreatedBy, &sc.Active, &sc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get active scoring config: %w", err)
	}
	json.Unmarshal(weightsJSON, &sc.Weights)
	json.Unmarshal(thresholdsJSON, &sc.Thresholds)
	return &sc, nil
}

func (r *ScoringConfigRepo) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.ScoringConfigID) (*domain.ScoringConfig, error) {
	var sc domain.ScoringConfig
	var weightsJSON, thresholdsJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, version, weights, thresholds, created_by, active, created_at
		FROM scoring_configs WHERE id = $1 AND tenant_id = $2
	`, id, tenantID).Scan(&sc.ID, &sc.TenantID, &sc.Version, &weightsJSON, &thresholdsJSON, &sc.CreatedBy, &sc.Active, &sc.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get scoring config: %w", err)
	}
	json.Unmarshal(weightsJSON, &sc.Weights)
	json.Unmarshal(thresholdsJSON, &sc.Thresholds)
	return &sc, nil
}

func (r *ScoringConfigRepo) SetActive(ctx context.Context, tenantID domain.TenantID, id domain.ScoringConfigID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "UPDATE scoring_configs SET active = false WHERE tenant_id = $1 AND active = true", tenantID)
	if err != nil {
		return fmt.Errorf("deactivate old: %w", err)
	}

	_, err = tx.Exec(ctx, "UPDATE scoring_configs SET active = true WHERE id = $1 AND tenant_id = $2", id, tenantID)
	if err != nil {
		return fmt.Errorf("activate new: %w", err)
	}

	return tx.Commit(ctx)
}
```

- [ ] **Step 8: Create `internal/adapter/postgres/discovery_config_repo.go`**

```go
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type DiscoveryConfigRepo struct {
	pool *pgxpool.Pool
}

func NewDiscoveryConfigRepo(pool *pgxpool.Pool) *DiscoveryConfigRepo {
	return &DiscoveryConfigRepo{pool: pool}
}

func (r *DiscoveryConfigRepo) Get(ctx context.Context, tenantID domain.TenantID) (*domain.DiscoveryConfig, error) {
	var dc domain.DiscoveryConfig
	var categoriesJSON, criteriaJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, categories, baseline_criteria, scoring_config_id, cadence, enabled, last_run_at, next_run_at
		FROM discovery_configs WHERE tenant_id = $1
	`, tenantID).Scan(&dc.ID, &dc.TenantID, &categoriesJSON, &criteriaJSON, &dc.ScoringConfigID, &dc.Cadence, &dc.Enabled, &dc.LastRunAt, &dc.NextRunAt)
	if err != nil {
		return nil, fmt.Errorf("get discovery config: %w", err)
	}
	json.Unmarshal(categoriesJSON, &dc.Categories)
	json.Unmarshal(criteriaJSON, &dc.BaselineCriteria)
	return &dc, nil
}

func (r *DiscoveryConfigRepo) Upsert(ctx context.Context, dc *domain.DiscoveryConfig) error {
	categories, _ := json.Marshal(dc.Categories)
	criteria, _ := json.Marshal(dc.BaselineCriteria)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO discovery_configs (id, tenant_id, categories, baseline_criteria, scoring_config_id, cadence, enabled, last_run_at, next_run_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id)
		DO UPDATE SET categories = $3, baseline_criteria = $4, scoring_config_id = $5, cadence = $6, enabled = $7, last_run_at = $8, next_run_at = $9
	`, dc.ID, dc.TenantID, categories, criteria, dc.ScoringConfigID, dc.Cadence, dc.Enabled, dc.LastRunAt, dc.NextRunAt)
	if err != nil {
		return fmt.Errorf("upsert discovery config: %w", err)
	}
	return nil
}
```

- [ ] **Step 9: Run `go build ./...` to verify**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 10: Commit**

```bash
git add internal/adapter/postgres/
git commit -m "feat: add Postgres repos, migrations, and connection pool"
```

---

## Task 6: Domain Services

**Files:**
- Create: `internal/service/campaign_service.go`
- Create: `internal/service/deal_service.go`
- Create: `internal/service/scoring_service.go`
- Create: `internal/service/discovery_service.go`
- Create: `internal/service/event_service.go`
- Create: `internal/service/pipeline_service.go`
- Test: `internal/service/deal_service_test.go`

- [ ] **Step 1: Create `internal/service/event_service.go`**

```go
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type EventService struct {
	repo      port.EventRepo
	analytics port.AnalyticsProvider
	idGen     port.IDGenerator
}

func NewEventService(repo port.EventRepo, analytics port.AnalyticsProvider, idGen port.IDGenerator) *EventService {
	return &EventService{repo: repo, analytics: analytics, idGen: idGen}
}

func (s *EventService) Emit(ctx context.Context, tenantID domain.TenantID, eventType, entityType, entityID, actorID string, payload map[string]any) error {
	event := &domain.DomainEvent{
		ID:            domain.DomainEventID(s.idGen.New()),
		TenantID:      tenantID,
		EventType:     eventType,
		EntityType:    entityType,
		EntityID:      entityID,
		Payload:       payload,
		CorrelationID: s.idGen.New(),
		ActorID:       actorID,
		Timestamp:     time.Now(),
	}

	if err := s.repo.Create(ctx, event); err != nil {
		return err
	}

	// Fire-and-forget to analytics — don't fail the operation if PostHog is down
	if s.analytics != nil {
		if err := s.analytics.CaptureEvent(ctx, string(tenantID), eventType, payload); err != nil {
			slog.Warn("failed to capture analytics event", "event_type", eventType, "error", err)
		}
	}

	return nil
}

func (s *EventService) List(ctx context.Context, tenantID domain.TenantID, filter port.EventFilter) ([]domain.DomainEvent, error) {
	return s.repo.List(ctx, tenantID, filter)
}
```

- [ ] **Step 2: Create `internal/service/campaign_service.go`**

```go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type CampaignService struct {
	repo     port.CampaignRepo
	scoring  port.ScoringConfigRepo
	events   *EventService
	durable  port.DurableRuntime
	idGen    port.IDGenerator
}

func NewCampaignService(repo port.CampaignRepo, scoring port.ScoringConfigRepo, events *EventService, durable port.DurableRuntime, idGen port.IDGenerator) *CampaignService {
	return &CampaignService{repo: repo, scoring: scoring, events: events, durable: durable, idGen: idGen}
}

type CreateCampaignInput struct {
	TenantID    domain.TenantID
	Type        domain.CampaignType
	TriggerType domain.TriggerType
	Criteria    domain.Criteria
	SourceFile  *string
	CreatedBy   string
}

func (s *CampaignService) Create(ctx context.Context, input CreateCampaignInput) (*domain.Campaign, error) {
	// Get active scoring config for tenant
	sc, err := s.scoring.GetActive(ctx, input.TenantID)
	if err != nil {
		return nil, fmt.Errorf("get active scoring config: %w", err)
	}

	campaign := &domain.Campaign{
		ID:              domain.CampaignID(s.idGen.New()),
		TenantID:        input.TenantID,
		Type:            input.Type,
		Criteria:        input.Criteria,
		ScoringConfigID: sc.ID,
		SourceFile:      input.SourceFile,
		Status:          domain.CampaignStatusPending,
		CreatedBy:       input.CreatedBy,
		TriggerType:     input.TriggerType,
		CreatedAt:       time.Now(),
	}

	if err := s.repo.Create(ctx, campaign); err != nil {
		return nil, fmt.Errorf("create campaign: %w", err)
	}

	_ = s.events.Emit(ctx, input.TenantID, "campaign_created", "campaign", string(campaign.ID), input.CreatedBy, map[string]any{
		"type":         input.Type,
		"trigger_type": input.TriggerType,
	})

	// Trigger the durable workflow
	if s.durable != nil {
		if err := s.durable.TriggerCampaignProcessing(ctx, campaign.ID, input.TenantID); err != nil {
			return nil, fmt.Errorf("trigger campaign processing: %w", err)
		}
	}

	return campaign, nil
}

func (s *CampaignService) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.CampaignID) (*domain.Campaign, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

func (s *CampaignService) List(ctx context.Context, tenantID domain.TenantID, filter port.CampaignFilter) ([]domain.Campaign, error) {
	return s.repo.List(ctx, tenantID, filter)
}
```

- [ ] **Step 3: Create `internal/service/deal_service.go`**

```go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type DealService struct {
	repo   port.DealRepo
	events *EventService
	idGen  port.IDGenerator
}

func NewDealService(repo port.DealRepo, events *EventService, idGen port.IDGenerator) *DealService {
	return &DealService{repo: repo, events: events, idGen: idGen}
}

// CreateFromResearch converts research pipeline results into deals.
func (s *DealService) CreateFromResearch(ctx context.Context, tenantID domain.TenantID, result *domain.ResearchResult) ([]domain.Deal, error) {
	var deals []domain.Deal
	now := time.Now()

	for _, c := range result.Candidates {
		deal := domain.Deal{
			ID:              domain.DealID(s.idGen.New()),
			TenantID:        tenantID,
			CampaignID:      result.CampaignID,
			ASIN:            c.ASIN,
			Title:           c.Title,
			Brand:           c.Brand,
			Category:        c.Category,
			Status:          domain.DealStatusNeedsReview,
			Scores:          c.Scores,
			Evidence:        c.Evidence,
			ReviewerVerdict: c.ReviewerVerdict,
			IterationCount:  c.IterationCount,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		deals = append(deals, deal)
	}

	if err := s.repo.CreateBatch(ctx, deals); err != nil {
		return nil, fmt.Errorf("create deals batch: %w", err)
	}

	for _, d := range deals {
		_ = s.events.Emit(ctx, tenantID, "deal_created", "deal", string(d.ID), "system", map[string]any{
			"campaign_id": result.CampaignID,
			"asin":        d.ASIN,
			"overall":     d.Scores.Overall,
		})
	}

	return deals, nil
}

func (s *DealService) GetByID(ctx context.Context, tenantID domain.TenantID, id domain.DealID) (*domain.Deal, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

func (s *DealService) List(ctx context.Context, tenantID domain.TenantID, filter port.DealFilter) ([]domain.Deal, int, error) {
	return s.repo.List(ctx, tenantID, filter)
}

func (s *DealService) Approve(ctx context.Context, tenantID domain.TenantID, id domain.DealID, userID string) (*domain.Deal, error) {
	deal, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	if err := deal.Transition(domain.DealStatusApproved); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, deal); err != nil {
		return nil, fmt.Errorf("update deal: %w", err)
	}

	_ = s.events.Emit(ctx, tenantID, "deal_approved", "deal", string(id), userID, map[string]any{
		"asin":    deal.ASIN,
		"overall": deal.Scores.Overall,
	})

	return deal, nil
}

func (s *DealService) Reject(ctx context.Context, tenantID domain.TenantID, id domain.DealID, userID string, reason string) (*domain.Deal, error) {
	deal, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}

	if err := deal.Transition(domain.DealStatusRejected); err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, deal); err != nil {
		return nil, fmt.Errorf("update deal: %w", err)
	}

	_ = s.events.Emit(ctx, tenantID, "deal_rejected", "deal", string(id), userID, map[string]any{
		"asin":   deal.ASIN,
		"reason": reason,
	})

	return deal, nil
}
```

- [ ] **Step 4: Write test `internal/service/deal_service_test.go`**

```go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

// --- Test doubles ---

type memDealRepo struct {
	deals map[domain.DealID]*domain.Deal
}

func newMemDealRepo() *memDealRepo {
	return &memDealRepo{deals: make(map[domain.DealID]*domain.Deal)}
}

func (r *memDealRepo) Create(_ context.Context, d *domain.Deal) error {
	r.deals[d.ID] = d
	return nil
}

func (r *memDealRepo) CreateBatch(_ context.Context, deals []domain.Deal) error {
	for i := range deals {
		r.deals[deals[i].ID] = &deals[i]
	}
	return nil
}

func (r *memDealRepo) GetByID(_ context.Context, _ domain.TenantID, id domain.DealID) (*domain.Deal, error) {
	d, ok := r.deals[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return d, nil
}

func (r *memDealRepo) List(_ context.Context, _ domain.TenantID, _ interface{ Limit int }) ([]domain.Deal, int, error) {
	return nil, 0, nil
}

func (r *memDealRepo) Update(_ context.Context, d *domain.Deal) error {
	r.deals[d.ID] = d
	return nil
}

type memEventRepo struct{}

func (r *memEventRepo) Create(_ context.Context, _ *domain.DomainEvent) error { return nil }
func (r *memEventRepo) List(_ context.Context, _ domain.TenantID, _ interface{}) ([]domain.DomainEvent, error) {
	return nil, nil
}

type seqIDGen struct{ counter int }

func (g *seqIDGen) New() string {
	g.counter++
	return fmt.Sprintf("id-%d", g.counter)
}

// --- Tests ---

func TestDealService_CreateFromResearch(t *testing.T) {
	repo := newMemDealRepo()
	idGen := &seqIDGen{}
	eventSvc := service.NewEventService(&memEventRepo{}, nil, idGen)
	svc := service.NewDealService(repo, eventSvc, idGen)

	result := &domain.ResearchResult{
		CampaignID: "camp-1",
		Candidates: []domain.CandidateResult{
			{
				ASIN:            "B0TEST001",
				Title:           "Test Product",
				Brand:           "TestBrand",
				Category:        "Kitchen",
				Scores:          domain.DealScores{Demand: 9, Competition: 8, Margin: 9, Risk: 8, SourcingFeasibility: 8, Overall: 8.5},
				ReviewerVerdict: "PASS",
				IterationCount:  1,
			},
		},
	}

	deals, err := svc.CreateFromResearch(context.Background(), "tenant-1", result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deals) != 1 {
		t.Fatalf("expected 1 deal, got %d", len(deals))
	}
	if deals[0].ASIN != "B0TEST001" {
		t.Errorf("expected ASIN B0TEST001, got %s", deals[0].ASIN)
	}
	if deals[0].Status != domain.DealStatusNeedsReview {
		t.Errorf("expected status needs_review, got %s", deals[0].Status)
	}
}

func TestDealService_Approve(t *testing.T) {
	repo := newMemDealRepo()
	idGen := &seqIDGen{}
	eventSvc := service.NewEventService(&memEventRepo{}, nil, idGen)
	svc := service.NewDealService(repo, eventSvc, idGen)

	deal := &domain.Deal{
		ID:       "deal-1",
		TenantID: "tenant-1",
		ASIN:     "B0TEST001",
		Status:   domain.DealStatusNeedsReview,
		UpdatedAt: time.Now(),
	}
	repo.Create(context.Background(), deal)

	approved, err := svc.Approve(context.Background(), "tenant-1", "deal-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved.Status != domain.DealStatusApproved {
		t.Errorf("expected status approved, got %s", approved.Status)
	}
}

func TestDealService_Approve_InvalidTransition(t *testing.T) {
	repo := newMemDealRepo()
	idGen := &seqIDGen{}
	eventSvc := service.NewEventService(&memEventRepo{}, nil, idGen)
	svc := service.NewDealService(repo, eventSvc, idGen)

	deal := &domain.Deal{
		ID:       "deal-1",
		TenantID: "tenant-1",
		Status:   domain.DealStatusDiscovered, // can't approve from discovered
		UpdatedAt: time.Now(),
	}
	repo.Create(context.Background(), deal)

	_, err := svc.Approve(context.Background(), "tenant-1", "deal-1", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
}
```

- [ ] **Step 5: Fix compilation — the test doubles need correct interface signatures**

The test file above uses simplified interfaces. We need to make the test doubles match the actual `port` interfaces. Replace the `List` methods on `memDealRepo` and `memEventRepo` with correct signatures:

```go
package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

// --- Test doubles ---

type memDealRepo struct {
	deals map[domain.DealID]*domain.Deal
}

func newMemDealRepo() *memDealRepo {
	return &memDealRepo{deals: make(map[domain.DealID]*domain.Deal)}
}

func (r *memDealRepo) Create(_ context.Context, d *domain.Deal) error {
	r.deals[d.ID] = d
	return nil
}

func (r *memDealRepo) CreateBatch(_ context.Context, deals []domain.Deal) error {
	for i := range deals {
		r.deals[deals[i].ID] = &deals[i]
	}
	return nil
}

func (r *memDealRepo) GetByID(_ context.Context, _ domain.TenantID, id domain.DealID) (*domain.Deal, error) {
	d, ok := r.deals[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return d, nil
}

func (r *memDealRepo) List(_ context.Context, _ domain.TenantID, _ port.DealFilter) ([]domain.Deal, int, error) {
	var all []domain.Deal
	for _, d := range r.deals {
		all = append(all, *d)
	}
	return all, len(all), nil
}

func (r *memDealRepo) Update(_ context.Context, d *domain.Deal) error {
	r.deals[d.ID] = d
	return nil
}

type memEventRepo struct{}

func (r *memEventRepo) Create(_ context.Context, _ *domain.DomainEvent) error { return nil }
func (r *memEventRepo) List(_ context.Context, _ domain.TenantID, _ port.EventFilter) ([]domain.DomainEvent, error) {
	return nil, nil
}

type seqIDGen struct{ counter int }

func (g *seqIDGen) New() string {
	g.counter++
	return fmt.Sprintf("id-%d", g.counter)
}

// --- Tests ---

func TestDealService_CreateFromResearch(t *testing.T) {
	repo := newMemDealRepo()
	idGen := &seqIDGen{}
	eventSvc := service.NewEventService(&memEventRepo{}, nil, idGen)
	svc := service.NewDealService(repo, eventSvc, idGen)

	result := &domain.ResearchResult{
		CampaignID: "camp-1",
		Candidates: []domain.CandidateResult{
			{
				ASIN:            "B0TEST001",
				Title:           "Test Product",
				Brand:           "TestBrand",
				Category:        "Kitchen",
				Scores:          domain.DealScores{Demand: 9, Competition: 8, Margin: 9, Risk: 8, SourcingFeasibility: 8, Overall: 8.5},
				ReviewerVerdict: "PASS",
				IterationCount:  1,
			},
		},
	}

	deals, err := svc.CreateFromResearch(context.Background(), "tenant-1", result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deals) != 1 {
		t.Fatalf("expected 1 deal, got %d", len(deals))
	}
	if deals[0].ASIN != "B0TEST001" {
		t.Errorf("expected ASIN B0TEST001, got %s", deals[0].ASIN)
	}
	if deals[0].Status != domain.DealStatusNeedsReview {
		t.Errorf("expected status needs_review, got %s", deals[0].Status)
	}
}

func TestDealService_Approve(t *testing.T) {
	repo := newMemDealRepo()
	idGen := &seqIDGen{}
	eventSvc := service.NewEventService(&memEventRepo{}, nil, idGen)
	svc := service.NewDealService(repo, eventSvc, idGen)

	deal := &domain.Deal{
		ID:        "deal-1",
		TenantID:  "tenant-1",
		ASIN:      "B0TEST001",
		Status:    domain.DealStatusNeedsReview,
		UpdatedAt: time.Now(),
	}
	repo.Create(context.Background(), deal)

	approved, err := svc.Approve(context.Background(), "tenant-1", "deal-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved.Status != domain.DealStatusApproved {
		t.Errorf("expected status approved, got %s", approved.Status)
	}
}

func TestDealService_Approve_InvalidTransition(t *testing.T) {
	repo := newMemDealRepo()
	idGen := &seqIDGen{}
	eventSvc := service.NewEventService(&memEventRepo{}, nil, idGen)
	svc := service.NewDealService(repo, eventSvc, idGen)

	deal := &domain.Deal{
		ID:        "deal-1",
		TenantID:  "tenant-1",
		Status:    domain.DealStatusDiscovered,
		UpdatedAt: time.Now(),
	}
	repo.Create(context.Background(), deal)

	_, err := svc.Approve(context.Background(), "tenant-1", "deal-1", "user-1")
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
}
```

- [ ] **Step 6: Create remaining services**

Create `internal/service/scoring_service.go`:

```go
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type ScoringService struct {
	repo  port.ScoringConfigRepo
	idGen port.IDGenerator
}

func NewScoringService(repo port.ScoringConfigRepo, idGen port.IDGenerator) *ScoringService {
	return &ScoringService{repo: repo, idGen: idGen}
}

func (s *ScoringService) GetActive(ctx context.Context, tenantID domain.TenantID) (*domain.ScoringConfig, error) {
	return s.repo.GetActive(ctx, tenantID)
}

func (s *ScoringService) Update(ctx context.Context, tenantID domain.TenantID, weights domain.ScoringWeights, thresholds domain.Thresholds) (*domain.ScoringConfig, error) {
	current, err := s.repo.GetActive(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get current config: %w", err)
	}

	newConfig := &domain.ScoringConfig{
		ID:         domain.ScoringConfigID(s.idGen.New()),
		TenantID:   tenantID,
		Version:    current.Version + 1,
		Weights:    weights,
		Thresholds: thresholds,
		CreatedBy:  "user",
		Active:     true,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, newConfig); err != nil {
		return nil, fmt.Errorf("create config: %w", err)
	}

	if err := s.repo.SetActive(ctx, tenantID, newConfig.ID); err != nil {
		return nil, fmt.Errorf("set active: %w", err)
	}

	return newConfig, nil
}

// EnsureDefault creates a default scoring config if none exists for the tenant.
func (s *ScoringService) EnsureDefault(ctx context.Context, tenantID domain.TenantID) error {
	_, err := s.repo.GetActive(ctx, tenantID)
	if err == nil {
		return nil // already exists
	}

	sc := &domain.ScoringConfig{
		ID:         domain.ScoringConfigID(s.idGen.New()),
		TenantID:   tenantID,
		Version:    1,
		Weights:    domain.DefaultScoringWeights(),
		Thresholds: domain.DefaultThresholds(),
		CreatedBy:  "system",
		Active:     true,
		CreatedAt:  time.Now(),
	}
	return s.repo.Create(ctx, sc)
}
```

Create `internal/service/discovery_service.go`:

```go
package service

import (
	"context"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type DiscoveryService struct {
	repo port.DiscoveryConfigRepo
}

func NewDiscoveryService(repo port.DiscoveryConfigRepo) *DiscoveryService {
	return &DiscoveryService{repo: repo}
}

func (s *DiscoveryService) Get(ctx context.Context, tenantID domain.TenantID) (*domain.DiscoveryConfig, error) {
	return s.repo.Get(ctx, tenantID)
}

func (s *DiscoveryService) Update(ctx context.Context, dc *domain.DiscoveryConfig) error {
	return s.repo.Upsert(ctx, dc)
}
```

Create `internal/service/pipeline_service.go`:

```go
package service

import (
	"context"
	"fmt"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// PipelineService orchestrates the research pipeline execution.
// It bridges the Operations Core and the Agent Runtime.
type PipelineService struct {
	agentRuntime port.AgentRuntime
	campaigns    port.CampaignRepo
	scoring      port.ScoringConfigRepo
	deals        *DealService
}

func NewPipelineService(agentRuntime port.AgentRuntime, campaigns port.CampaignRepo, scoring port.ScoringConfigRepo, deals *DealService) *PipelineService {
	return &PipelineService{agentRuntime: agentRuntime, campaigns: campaigns, scoring: scoring, deals: deals}
}

// RunCampaign executes the research pipeline for a campaign and stores results as deals.
// Called by the durable workflow (Inngest).
func (s *PipelineService) RunCampaign(ctx context.Context, campaignID domain.CampaignID, tenantID domain.TenantID) error {
	campaign, err := s.campaigns.GetByID(ctx, tenantID, campaignID)
	if err != nil {
		return fmt.Errorf("get campaign: %w", err)
	}

	if err := campaign.Transition(domain.CampaignStatusRunning); err != nil {
		return err
	}
	if err := s.campaigns.Update(ctx, campaign); err != nil {
		return fmt.Errorf("update campaign to running: %w", err)
	}

	sc, err := s.scoring.GetByID(ctx, tenantID, campaign.ScoringConfigID)
	if err != nil {
		return fmt.Errorf("get scoring config: %w", err)
	}

	input := port.PipelineInput{
		CampaignID:    campaignID,
		TenantID:      tenantID,
		Criteria:      campaign.Criteria,
		ScoringConfig: *sc,
	}

	result, err := s.agentRuntime.RunResearchPipeline(ctx, input)
	if err != nil {
		_ = campaign.Transition(domain.CampaignStatusFailed)
		_ = s.campaigns.Update(ctx, campaign)
		return fmt.Errorf("run research pipeline: %w", err)
	}

	_, err = s.deals.CreateFromResearch(ctx, tenantID, result)
	if err != nil {
		_ = campaign.Transition(domain.CampaignStatusFailed)
		_ = s.campaigns.Update(ctx, campaign)
		return fmt.Errorf("create deals from research: %w", err)
	}

	if err := campaign.Transition(domain.CampaignStatusCompleted); err != nil {
		return err
	}
	return s.campaigns.Update(ctx, campaign)
}
```

- [ ] **Step 7: Run tests**

```bash
go test ./internal/service/... -v -count=1
```

Expected: 3 tests pass

- [ ] **Step 8: Commit**

```bash
git add internal/service/
git commit -m "feat: add domain services — campaign, deal, scoring, discovery, events, pipeline"
```

---

## Task 7: HTTP Handlers + Router

**Files:**
- Create: `internal/api/response/json.go`
- Create: `internal/api/middleware/auth.go`
- Create: `internal/api/middleware/tenant.go`
- Create: `internal/api/middleware/request_id.go`
- Create: `internal/api/handler/health.go`
- Create: `internal/api/handler/campaign_handler.go`
- Create: `internal/api/handler/deal_handler.go`
- Create: `internal/api/handler/scoring_handler.go`
- Create: `internal/api/handler/discovery_handler.go`
- Create: `internal/api/handler/event_handler.go`
- Create: `internal/api/handler/dashboard_handler.go`
- Create: `internal/api/router.go`

- [ ] **Step 1: Create `internal/api/response/json.go`**

```go
package response

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}
```

- [ ] **Step 2: Create `internal/api/middleware/request_id.go`**

```go
package middleware

import (
	"context"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type ctxKeyRequestID struct{}

func RequestID(idGen port.IDGenerator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = idGen.New()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID{}).(string)
	return id
}
```

- [ ] **Step 3: Create `internal/api/middleware/auth.go`**

```go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type ctxKeyAuth struct{}

func Auth(provider port.AuthProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				response.Error(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			if token == header {
				response.Error(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}

			authCtx, err := provider.ValidateToken(r.Context(), token)
			if err != nil {
				response.Error(w, http.StatusUnauthorized, "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyAuth{}, authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetAuthContext(ctx context.Context) *port.AuthContext {
	ac, _ := ctx.Value(ctxKeyAuth{}).(*port.AuthContext)
	return ac
}
```

- [ ] **Step 4: Create `internal/api/middleware/tenant.go`**

```go
package middleware

import (
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
)

// RequireTenant ensures the request has a valid tenant context from auth.
func RequireTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ac := GetAuthContext(r.Context())
		if ac == nil || ac.TenantID == "" {
			response.Error(w, http.StatusForbidden, "no tenant context")
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 5: Create handlers**

Create `internal/api/handler/health.go`:

```go
package handler

import (
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler { return &HealthHandler{} }

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
}
```

Create `internal/api/handler/campaign_handler.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type CampaignHandler struct {
	svc *service.CampaignService
}

func NewCampaignHandler(svc *service.CampaignService) *CampaignHandler {
	return &CampaignHandler{svc: svc}
}

type createCampaignRequest struct {
	Type        domain.CampaignType `json:"type"`
	TriggerType domain.TriggerType  `json:"trigger_type"`
	Criteria    domain.Criteria     `json:"criteria"`
	SourceFile  *string             `json:"source_file,omitempty"`
}

func (h *CampaignHandler) Create(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	var req createCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	campaign, err := h.svc.Create(r.Context(), service.CreateCampaignInput{
		TenantID:    ac.TenantID,
		Type:        req.Type,
		TriggerType: req.TriggerType,
		Criteria:    req.Criteria,
		SourceFile:  req.SourceFile,
		CreatedBy:   string(ac.UserID),
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusCreated, campaign)
}

func (h *CampaignHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	id := chi.URLParam(r, "id")

	campaign, err := h.svc.GetByID(r.Context(), ac.TenantID, domain.CampaignID(id))
	if err != nil {
		response.Error(w, http.StatusNotFound, "campaign not found")
		return
	}

	response.JSON(w, http.StatusOK, campaign)
}

func (h *CampaignHandler) List(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	campaigns, err := h.svc.List(r.Context(), ac.TenantID, port.CampaignFilter{
		Limit: 50,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, campaigns)
}
```

Create `internal/api/handler/deal_handler.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type DealHandler struct {
	svc *service.DealService
}

func NewDealHandler(svc *service.DealService) *DealHandler {
	return &DealHandler{svc: svc}
}

func (h *DealHandler) List(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	q := r.URL.Query()

	filter := port.DealFilter{
		SortBy:  q.Get("sort_by"),
		SortDir: q.Get("sort_dir"),
		Limit:   50,
	}

	if v := q.Get("campaign_id"); v != "" {
		cid := domain.CampaignID(v)
		filter.CampaignID = &cid
	}
	if v := q.Get("status"); v != "" {
		s := domain.DealStatus(v)
		filter.Status = &s
	}
	if v := q.Get("search"); v != "" {
		filter.Search = &v
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			filter.Offset = n
		}
	}

	deals, total, err := h.svc.List(r.Context(), ac.TenantID, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"deals": deals,
		"total": total,
	})
}

func (h *DealHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	id := chi.URLParam(r, "id")

	deal, err := h.svc.GetByID(r.Context(), ac.TenantID, domain.DealID(id))
	if err != nil {
		response.Error(w, http.StatusNotFound, "deal not found")
		return
	}

	response.JSON(w, http.StatusOK, deal)
}

type dealDecisionRequest struct {
	Reason string `json:"reason,omitempty"`
}

func (h *DealHandler) Approve(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	id := chi.URLParam(r, "id")

	deal, err := h.svc.Approve(r.Context(), ac.TenantID, domain.DealID(id), string(ac.UserID))
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, deal)
}

func (h *DealHandler) Reject(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	id := chi.URLParam(r, "id")

	var req dealDecisionRequest
	json.NewDecoder(r.Body).Decode(&req)

	deal, err := h.svc.Reject(r.Context(), ac.TenantID, domain.DealID(id), string(ac.UserID), req.Reason)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, deal)
}
```

Create `internal/api/handler/scoring_handler.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type ScoringHandler struct {
	svc *service.ScoringService
}

func NewScoringHandler(svc *service.ScoringService) *ScoringHandler {
	return &ScoringHandler{svc: svc}
}

func (h *ScoringHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	sc, err := h.svc.GetActive(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "no scoring config found")
		return
	}

	response.JSON(w, http.StatusOK, sc)
}

type updateScoringRequest struct {
	Weights    domain.ScoringWeights `json:"weights"`
	Thresholds domain.Thresholds     `json:"thresholds"`
}

func (h *ScoringHandler) Update(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	var req updateScoringRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sc, err := h.svc.Update(r.Context(), ac.TenantID, req.Weights, req.Thresholds)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, sc)
}
```

Create `internal/api/handler/discovery_handler.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type DiscoveryHandler struct {
	svc *service.DiscoveryService
}

func NewDiscoveryHandler(svc *service.DiscoveryService) *DiscoveryHandler {
	return &DiscoveryHandler{svc: svc}
}

func (h *DiscoveryHandler) Get(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	dc, err := h.svc.Get(r.Context(), ac.TenantID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "no discovery config found")
		return
	}

	response.JSON(w, http.StatusOK, dc)
}

func (h *DiscoveryHandler) Update(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	var dc domain.DiscoveryConfig
	if err := json.NewDecoder(r.Body).Decode(&dc); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	dc.TenantID = ac.TenantID

	if err := h.svc.Update(r.Context(), &dc); err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, dc)
}
```

Create `internal/api/handler/event_handler.go`:

```go
package handler

import (
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type EventHandler struct {
	svc *service.EventService
}

func NewEventHandler(svc *service.EventService) *EventHandler {
	return &EventHandler{svc: svc}
}

func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	q := r.URL.Query()

	filter := port.EventFilter{Limit: 100}
	if v := q.Get("entity_type"); v != "" {
		filter.EntityType = &v
	}
	if v := q.Get("entity_id"); v != "" {
		filter.EntityID = &v
	}

	events, err := h.svc.List(r.Context(), ac.TenantID, filter)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.JSON(w, http.StatusOK, events)
}
```

Create `internal/api/handler/dashboard_handler.go`:

```go
package handler

import (
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type DashboardHandler struct {
	campaigns *service.CampaignService
	deals     *service.DealService
}

func NewDashboardHandler(campaigns *service.CampaignService, deals *service.DealService) *DashboardHandler {
	return &DashboardHandler{campaigns: campaigns, deals: deals}
}

func (h *DashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	needsReview := domain.DealStatusNeedsReview
	_, reviewCount, _ := h.deals.List(r.Context(), ac.TenantID, port.DealFilter{Status: &needsReview, Limit: 0})

	approved := domain.DealStatusApproved
	_, approvedCount, _ := h.deals.List(r.Context(), ac.TenantID, port.DealFilter{Status: &approved, Limit: 0})

	running := domain.CampaignStatusRunning
	activeCampaigns, _ := h.campaigns.List(r.Context(), ac.TenantID, port.CampaignFilter{Status: &running})

	recentDeals, _, _ := h.deals.List(r.Context(), ac.TenantID, port.DealFilter{Limit: 5, SortBy: "created_at", SortDir: "desc"})

	response.JSON(w, http.StatusOK, map[string]any{
		"deals_pending_review": reviewCount,
		"deals_approved":       approvedCount,
		"active_campaigns":     len(activeCampaigns),
		"recent_deals":         recentDeals,
	})
}
```

- [ ] **Step 6: Create `internal/api/router.go`**

```go
package api

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/handler"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type Handlers struct {
	Health    *handler.HealthHandler
	Campaign  *handler.CampaignHandler
	Deal      *handler.DealHandler
	Scoring   *handler.ScoringHandler
	Discovery *handler.DiscoveryHandler
	Event     *handler.EventHandler
	Dashboard *handler.DashboardHandler
}

func NewRouter(h Handlers, auth port.AuthProvider, idGen port.IDGenerator) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestID(idGen))

	// Public routes
	r.Get("/health", h.Health.Health)
	r.Get("/ready", h.Health.Ready)

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(auth))
		r.Use(middleware.RequireTenant)

		r.Post("/campaigns", h.Campaign.Create)
		r.Get("/campaigns", h.Campaign.List)
		r.Get("/campaigns/{id}", h.Campaign.GetByID)

		r.Get("/deals", h.Deal.List)
		r.Get("/deals/{id}", h.Deal.GetByID)
		r.Post("/deals/{id}/approve", h.Deal.Approve)
		r.Post("/deals/{id}/reject", h.Deal.Reject)

		r.Get("/config/scoring", h.Scoring.Get)
		r.Put("/config/scoring", h.Scoring.Update)

		r.Get("/discovery", h.Discovery.Get)
		r.Put("/discovery", h.Discovery.Update)

		r.Get("/events", h.Event.List)

		r.Get("/dashboard/summary", h.Dashboard.Summary)
	})

	return r
}
```

- [ ] **Step 7: Run `go build ./...` to verify**

```bash
go get github.com/go-chi/chi/v5
go build ./...
```

Expected: no errors

- [ ] **Step 8: Commit**

```bash
git add internal/api/
git commit -m "feat: add HTTP handlers, middleware, and router"
```

---

## Task 8: Adapter Stubs (OpenFang, Inngest, Supabase Auth, PostHog)

**Files:**
- Create: `internal/adapter/openfang/agent_runtime.go`
- Create: `internal/adapter/inngest/client.go`
- Create: `internal/adapter/inngest/campaign_workflow.go`
- Create: `internal/adapter/supabase/auth_provider.go`
- Create: `internal/adapter/posthog/analytics_provider.go`

- [ ] **Step 1: Create `internal/adapter/openfang/agent_runtime.go`**

```go
package openfang

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type AgentRuntime struct {
	apiURL string
	apiKey string
}

func NewAgentRuntime(apiURL, apiKey string) *AgentRuntime {
	return &AgentRuntime{apiURL: apiURL, apiKey: apiKey}
}

func (r *AgentRuntime) RunResearchPipeline(ctx context.Context, input port.PipelineInput) (*domain.ResearchResult, error) {
	slog.Info("running research pipeline via OpenFang",
		"campaign_id", input.CampaignID,
		"tenant_id", input.TenantID,
		"keywords", input.Criteria.Keywords,
	)

	// TODO: implement actual OpenFang API calls
	// For now, return a stub result so the full flow can be tested end-to-end
	return nil, fmt.Errorf("OpenFang research pipeline not yet implemented — configure OPENFANG_API_URL and OPENFANG_API_KEY")
}
```

- [ ] **Step 2: Create `internal/adapter/inngest/client.go`**

```go
package inngest

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
)

type DurableRuntime struct {
	eventKey string
	dev      bool
}

func NewDurableRuntime(eventKey string, dev bool) *DurableRuntime {
	return &DurableRuntime{eventKey: eventKey, dev: dev}
}

func (r *DurableRuntime) TriggerCampaignProcessing(ctx context.Context, campaignID domain.CampaignID, tenantID domain.TenantID) error {
	slog.Info("triggering campaign processing workflow",
		"campaign_id", campaignID,
		"tenant_id", tenantID,
	)

	// TODO: implement Inngest event send
	// For now log and return nil so campaign creation works end-to-end
	if r.dev {
		slog.Warn("Inngest dev mode — workflow trigger is a no-op")
		return nil
	}

	return fmt.Errorf("Inngest campaign processing not yet implemented")
}

func (r *DurableRuntime) TriggerDiscoveryRun(ctx context.Context, tenantID domain.TenantID) error {
	slog.Info("triggering discovery run", "tenant_id", tenantID)

	if r.dev {
		slog.Warn("Inngest dev mode — discovery trigger is a no-op")
		return nil
	}

	return fmt.Errorf("Inngest discovery run not yet implemented")
}
```

- [ ] **Step 3: Create `internal/adapter/supabase/auth_provider.go`**

```go
package supabase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

type AuthProvider struct {
	jwtSecret string
	isDev     bool
}

func NewAuthProvider(jwtSecret string, isDev bool) *AuthProvider {
	return &AuthProvider{jwtSecret: jwtSecret, isDev: isDev}
}

func (p *AuthProvider) ValidateToken(ctx context.Context, token string) (*port.AuthContext, error) {
	// In dev mode, accept a simple format: "dev-<user_id>-<tenant_id>"
	if p.isDev && len(token) > 4 && token[:4] == "dev-" {
		slog.Warn("using dev auth mode — do not use in production")
		// Parse: dev-user123-tenant456
		// Simple split for dev convenience
		return &port.AuthContext{
			UserID:   domain.UserID("dev-user"),
			TenantID: domain.TenantID("dev-tenant"),
			Role:     domain.RoleOwner,
		}, nil
	}

	// TODO: implement real Supabase JWT validation
	// 1. Decode JWT with jwtSecret
	// 2. Extract user_id from sub claim
	// 3. Look up tenant membership
	// 4. Return AuthContext

	return nil, fmt.Errorf("Supabase JWT validation not yet implemented — use dev token format 'dev-<user>-<tenant>'")
}
```

- [ ] **Step 4: Create `internal/adapter/posthog/analytics_provider.go`**

```go
package posthog

import (
	"context"
	"log/slog"
)

type AnalyticsProvider struct {
	apiKey string
	host   string
	isDev  bool
}

func NewAnalyticsProvider(apiKey, host string, isDev bool) *AnalyticsProvider {
	return &AnalyticsProvider{apiKey: apiKey, host: host, isDev: isDev}
}

func (p *AnalyticsProvider) CaptureEvent(ctx context.Context, distinctID string, eventName string, properties map[string]any) error {
	if p.isDev || p.apiKey == "" {
		slog.Debug("posthog capture (no-op)", "event", eventName, "distinct_id", distinctID)
		return nil
	}

	// TODO: implement PostHog HTTP API call
	slog.Info("posthog capture", "event", eventName, "distinct_id", distinctID)
	return nil
}

func (p *AnalyticsProvider) IsFeatureEnabled(ctx context.Context, flagKey string, distinctID string) (bool, error) {
	if p.isDev || p.apiKey == "" {
		return false, nil
	}

	// TODO: implement PostHog feature flag check
	return false, nil
}
```

- [ ] **Step 5: Run `go build ./...`**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/adapter/openfang/ internal/adapter/inngest/ internal/adapter/supabase/ internal/adapter/posthog/
git commit -m "feat: add adapter stubs — OpenFang, Inngest, Supabase auth, PostHog"
```

---

## Task 9: Wire Everything in main.go

**Files:**
- Modify: `apps/api/main.go`
- Create: `internal/port/idgen.go`

- [ ] **Step 1: Create `internal/port/idgen.go`**

```go
package port

import "github.com/google/uuid"

type UUIDGenerator struct{}

func (UUIDGenerator) New() string { return uuid.New().String() }
```

Install uuid:

```bash
go get github.com/google/uuid
```

- [ ] **Step 2: Rewrite `apps/api/main.go` with full wiring**

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/inngest"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/openfang"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/posthog"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/postgres"
	"github.com/pluriza/fba-agent-orchestrator/internal/adapter/supabase"
	"github.com/pluriza/fba-agent-orchestrator/internal/api"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/handler"
	"github.com/pluriza/fba-agent-orchestrator/internal/config"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

func main() {
	_ = godotenv.Load()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Database
	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := postgres.RunMigrations(ctx, pool); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Repos
	campaignRepo := postgres.NewCampaignRepo(pool)
	dealRepo := postgres.NewDealRepo(pool)
	eventRepo := postgres.NewEventRepo(pool)
	scoringRepo := postgres.NewScoringConfigRepo(pool)
	discoveryRepo := postgres.NewDiscoveryConfigRepo(pool)

	// Adapters
	authProvider := supabase.NewAuthProvider(cfg.SupabaseJWTSecret, cfg.IsDev())
	analyticsProvider := posthog.NewAnalyticsProvider(cfg.PostHogAPIKey, cfg.PostHogHost, cfg.IsDev())
	agentRuntime := openfang.NewAgentRuntime(cfg.OpenFangAPIURL, cfg.OpenFangAPIKey)
	durableRuntime := inngest.NewDurableRuntime(cfg.InngestEventKey, cfg.InngestDev)

	// ID generator
	idGen := port.UUIDGenerator{}

	// Services
	eventSvc := service.NewEventService(eventRepo, analyticsProvider, idGen)
	scoringSvc := service.NewScoringService(scoringRepo, idGen)
	dealSvc := service.NewDealService(dealRepo, eventSvc, idGen)
	campaignSvc := service.NewCampaignService(campaignRepo, scoringRepo, eventSvc, durableRuntime, idGen)
	discoverySvc := service.NewDiscoveryService(discoveryRepo)
	_ = service.NewPipelineService(agentRuntime, campaignRepo, scoringRepo, dealSvc)

	// Handlers
	handlers := api.Handlers{
		Health:    handler.NewHealthHandler(),
		Campaign:  handler.NewCampaignHandler(campaignSvc),
		Deal:      handler.NewDealHandler(dealSvc),
		Scoring:   handler.NewScoringHandler(scoringSvc),
		Discovery: handler.NewDiscoveryHandler(discoverySvc),
		Event:     handler.NewEventHandler(eventSvc),
		Dashboard: handler.NewDashboardHandler(campaignSvc, dealSvc),
	}

	router := api.NewRouter(handlers, authProvider, idGen)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("starting server", "port", cfg.Port, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}
```

- [ ] **Step 3: Run `go build ./...`**

```bash
go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add apps/api/main.go internal/port/idgen.go
git commit -m "feat: wire all dependencies in main.go — full backend assembled"
```

---

## Task 10: Frontend Bootstrap

**Files:**
- Create: `apps/web/package.json`
- Create: `apps/web/next.config.ts`
- Create: `apps/web/tailwind.config.ts`
- Create: `apps/web/tsconfig.json`
- Create: `apps/web/src/app/layout.tsx`
- Create: `apps/web/src/app/page.tsx`
- Create: `apps/web/src/lib/types.ts`
- Create: `apps/web/src/lib/api-client.ts`

- [ ] **Step 1: Create Next.js app**

```bash
cd /Users/pluriza/Documents/Work/Pluriza/Estori/fba_agent_orchestrator
mkdir -p apps/web
cd apps/web
npx create-next-app@latest . --typescript --tailwind --eslint --app --src-dir --no-import-alias --use-npm
```

- [ ] **Step 2: Install dependencies**

```bash
cd /Users/pluriza/Documents/Work/Pluriza/Estori/fba_agent_orchestrator/apps/web
npm install @tanstack/react-query @supabase/supabase-js
npx shadcn@latest init -d
npx shadcn@latest add button card table badge input select tabs separator dropdown-menu sheet dialog
```

- [ ] **Step 3: Create `apps/web/src/lib/types.ts`**

```typescript
// Domain types mirroring Go backend

export type CampaignType = "discovery_run" | "manual" | "experiment";
export type CampaignStatus = "pending" | "running" | "completed" | "failed";
export type TriggerType = "chat" | "dashboard" | "scheduler" | "spreadsheet";

export type DealStatus =
  | "discovered"
  | "analyzing"
  | "needs_review"
  | "approved"
  | "rejected"
  | "sourcing"
  | "procuring"
  | "listing"
  | "live"
  | "monitoring"
  | "reorder"
  | "archived";

export interface Criteria {
  keywords: string[];
  min_monthly_revenue?: number;
  min_margin_pct?: number;
  max_wholesale_cost?: number;
  max_moq?: number;
  preferred_brands?: string[];
  marketplace: string;
}

export interface Campaign {
  id: string;
  tenant_id: string;
  type: CampaignType;
  criteria: Criteria;
  scoring_config_id: string;
  experiment_id?: string;
  source_file?: string;
  status: CampaignStatus;
  created_by: string;
  trigger_type: TriggerType;
  created_at: string;
  completed_at?: string;
}

export interface DealScores {
  demand: number;
  competition: number;
  margin: number;
  risk: number;
  sourcing_feasibility: number;
  overall: number;
}

export interface AgentEvidence {
  reasoning: string;
  data: Record<string, unknown>;
}

export interface Evidence {
  demand: AgentEvidence;
  competition: AgentEvidence;
  margin: AgentEvidence;
  risk: AgentEvidence;
  sourcing: AgentEvidence;
}

export interface Deal {
  id: string;
  tenant_id: string;
  campaign_id: string;
  asin: string;
  title: string;
  brand: string;
  category: string;
  status: DealStatus;
  scores: DealScores;
  evidence: Evidence;
  reviewer_verdict: string;
  iteration_count: number;
  created_at: string;
  updated_at: string;
}

export interface ScoringWeights {
  demand: number;
  competition: number;
  margin: number;
  risk: number;
  sourcing: number;
}

export interface Thresholds {
  min_overall: number;
  min_per_dimension: number;
}

export interface ScoringConfig {
  id: string;
  tenant_id: string;
  version: number;
  weights: ScoringWeights;
  thresholds: Thresholds;
  created_by: string;
  active: boolean;
  created_at: string;
}

export interface DashboardSummary {
  deals_pending_review: number;
  deals_approved: number;
  active_campaigns: number;
  recent_deals: Deal[];
}

export interface DomainEvent {
  id: string;
  tenant_id: string;
  event_type: string;
  entity_type: string;
  entity_id: string;
  payload: Record<string, unknown>;
  correlation_id: string;
  actor_id: string;
  timestamp: string;
}
```

- [ ] **Step 4: Create `apps/web/src/lib/api-client.ts`**

```typescript
const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

class ApiClient {
  private token: string | null = null;

  setToken(token: string) {
    this.token = token;
  }

  private async fetch<T>(path: string, options?: RequestInit): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (this.token) {
      headers["Authorization"] = `Bearer ${this.token}`;
    }

    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: { ...headers, ...options?.headers },
    });

    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `API error: ${res.status}`);
    }

    return res.json();
  }

  // Campaigns
  getCampaigns() {
    return this.fetch<Campaign[]>("/campaigns");
  }

  getCampaign(id: string) {
    return this.fetch<Campaign>(`/campaigns/${id}`);
  }

  createCampaign(data: {
    type: string;
    trigger_type: string;
    criteria: Criteria;
  }) {
    return this.fetch<Campaign>("/campaigns", {
      method: "POST",
      body: JSON.stringify(data),
    });
  }

  // Deals
  getDeals(params?: Record<string, string>) {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return this.fetch<{ deals: Deal[]; total: number }>(`/deals${qs}`);
  }

  getDeal(id: string) {
    return this.fetch<Deal>(`/deals/${id}`);
  }

  approveDeal(id: string) {
    return this.fetch<Deal>(`/deals/${id}/approve`, { method: "POST" });
  }

  rejectDeal(id: string, reason?: string) {
    return this.fetch<Deal>(`/deals/${id}/reject`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    });
  }

  // Scoring
  getScoringConfig() {
    return this.fetch<ScoringConfig>("/config/scoring");
  }

  updateScoringConfig(data: {
    weights: ScoringWeights;
    thresholds: Thresholds;
  }) {
    return this.fetch<ScoringConfig>("/config/scoring", {
      method: "PUT",
      body: JSON.stringify(data),
    });
  }

  // Dashboard
  getDashboardSummary() {
    return this.fetch<DashboardSummary>("/dashboard/summary");
  }

  // Events
  getEvents(params?: Record<string, string>) {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return this.fetch<DomainEvent[]>(`/events${qs}`);
  }
}

import type {
  Campaign,
  Criteria,
  Deal,
  ScoringConfig,
  ScoringWeights,
  Thresholds,
  DashboardSummary,
  DomainEvent,
} from "./types";

export const apiClient = new ApiClient();
```

- [ ] **Step 5: Commit**

```bash
cd /Users/pluriza/Documents/Work/Pluriza/Estori/fba_agent_orchestrator
git add apps/web/
git commit -m "feat: bootstrap Next.js frontend with types and API client"
```

---

## Task 11: Frontend Pages — Dashboard, Campaigns, Deals

**Files:**
- Create: `apps/web/src/app/(app)/layout.tsx`
- Create: `apps/web/src/app/(app)/dashboard/page.tsx`
- Create: `apps/web/src/app/(app)/campaigns/page.tsx`
- Create: `apps/web/src/app/(app)/campaigns/new/page.tsx`
- Create: `apps/web/src/app/(app)/deals/page.tsx`
- Create: `apps/web/src/app/(app)/deals/[id]/page.tsx`
- Create: `apps/web/src/components/app-shell.tsx`
- Create: `apps/web/src/components/score-badge.tsx`
- Create: `apps/web/src/components/status-pill.tsx`
- Create: `apps/web/src/components/metric-card.tsx`
- Create: `apps/web/src/components/page-header.tsx`
- Create: `apps/web/src/components/empty-state.tsx`
- Create: `apps/web/src/components/evidence-panel.tsx`
- Create: `apps/web/src/hooks/use-campaigns.ts`
- Create: `apps/web/src/hooks/use-deals.ts`
- Create: `apps/web/src/lib/query-keys.ts`

This is the largest task. Due to the volume of frontend files, each step creates 2-3 related files.

- [ ] **Step 1: Create shared components**

Create `apps/web/src/lib/query-keys.ts`:
```typescript
export const queryKeys = {
  dashboard: ["dashboard"] as const,
  campaigns: {
    all: ["campaigns"] as const,
    detail: (id: string) => ["campaigns", id] as const,
  },
  deals: {
    all: ["deals"] as const,
    list: (params: Record<string, string>) => ["deals", params] as const,
    detail: (id: string) => ["deals", id] as const,
  },
  scoring: ["scoring"] as const,
  discovery: ["discovery"] as const,
  events: (params: Record<string, string>) => ["events", params] as const,
};
```

Create `apps/web/src/components/score-badge.tsx`:
```tsx
interface ScoreBadgeProps {
  score: number;
  label?: string;
}

export function ScoreBadge({ score, label }: ScoreBadgeProps) {
  const color =
    score >= 8
      ? "bg-green-100 text-green-800"
      : score >= 6
        ? "bg-yellow-100 text-yellow-800"
        : "bg-red-100 text-red-800";

  return (
    <span className={`inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium ${color}`}>
      {label && <span className="text-muted-foreground">{label}</span>}
      {score}/10
    </span>
  );
}
```

Create `apps/web/src/components/status-pill.tsx`:
```tsx
const statusColors: Record<string, string> = {
  pending: "bg-gray-100 text-gray-700",
  running: "bg-blue-100 text-blue-700",
  completed: "bg-green-100 text-green-700",
  failed: "bg-red-100 text-red-700",
  discovered: "bg-gray-100 text-gray-700",
  analyzing: "bg-blue-100 text-blue-700",
  needs_review: "bg-amber-100 text-amber-700",
  approved: "bg-green-100 text-green-700",
  rejected: "bg-red-100 text-red-700",
  sourcing: "bg-purple-100 text-purple-700",
  live: "bg-emerald-100 text-emerald-700",
  archived: "bg-gray-100 text-gray-500",
};

export function StatusPill({ status }: { status: string }) {
  const color = statusColors[status] || "bg-gray-100 text-gray-700";
  return (
    <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${color}`}>
      {status.replace(/_/g, " ")}
    </span>
  );
}
```

Create `apps/web/src/components/metric-card.tsx`:
```tsx
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

interface MetricCardProps {
  title: string;
  value: string | number;
  description?: string;
}

export function MetricCard({ title, value, description }: MetricCardProps) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{value}</div>
        {description && (
          <p className="text-xs text-muted-foreground">{description}</p>
        )}
      </CardContent>
    </Card>
  );
}
```

Create `apps/web/src/components/page-header.tsx`:
```tsx
interface PageHeaderProps {
  title: string;
  description?: string;
  action?: React.ReactNode;
}

export function PageHeader({ title, description, action }: PageHeaderProps) {
  return (
    <div className="flex items-center justify-between">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {description && (
          <p className="text-sm text-muted-foreground">{description}</p>
        )}
      </div>
      {action}
    </div>
  );
}
```

Create `apps/web/src/components/empty-state.tsx`:
```tsx
interface EmptyStateProps {
  title: string;
  description: string;
  action?: React.ReactNode;
}

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed p-12 text-center">
      <h3 className="text-lg font-medium">{title}</h3>
      <p className="mt-1 text-sm text-muted-foreground">{description}</p>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}
```

Create `apps/web/src/components/evidence-panel.tsx`:
```tsx
"use client";

import { useState } from "react";
import type { Evidence } from "@/lib/types";

export function EvidencePanel({ evidence }: { evidence: Evidence }) {
  const [expanded, setExpanded] = useState<string | null>(null);

  const sections = [
    { key: "demand", label: "Demand Analysis", data: evidence.demand },
    { key: "competition", label: "Competition Analysis", data: evidence.competition },
    { key: "margin", label: "Profitability Analysis", data: evidence.margin },
    { key: "risk", label: "Risk Assessment", data: evidence.risk },
    { key: "sourcing", label: "Sourcing Feasibility", data: evidence.sourcing },
  ];

  return (
    <div className="space-y-2">
      {sections.map((section) => (
        <div key={section.key} className="rounded-lg border">
          <button
            className="flex w-full items-center justify-between p-3 text-left text-sm font-medium"
            onClick={() => setExpanded(expanded === section.key ? null : section.key)}
          >
            {section.label}
            <span className="text-muted-foreground">{expanded === section.key ? "−" : "+"}</span>
          </button>
          {expanded === section.key && section.data && (
            <div className="border-t px-3 pb-3 pt-2 text-sm">
              <p className="whitespace-pre-wrap">{section.data.reasoning}</p>
              {section.data.data && Object.keys(section.data.data).length > 0 && (
                <pre className="mt-2 overflow-auto rounded bg-muted p-2 text-xs">
                  {JSON.stringify(section.data.data, null, 2)}
                </pre>
              )}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 2: Create app shell and hooks**

Create `apps/web/src/components/app-shell.tsx`:
```tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const navItems = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/campaigns", label: "Campaigns" },
  { href: "/deals", label: "Deals" },
  { href: "/discovery", label: "Discovery" },
  { href: "/settings", label: "Settings" },
];

export function AppShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <div className="flex h-screen">
      <aside className="flex w-56 flex-col border-r bg-muted/30">
        <div className="p-4">
          <h2 className="text-lg font-semibold">FBA Orchestrator</h2>
        </div>
        <nav className="flex-1 space-y-1 px-2">
          {navItems.map((item) => {
            const active = pathname.startsWith(item.href);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`block rounded-md px-3 py-2 text-sm font-medium ${
                  active
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-muted"
                }`}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>
      </aside>
      <main className="flex-1 overflow-auto p-6">{children}</main>
    </div>
  );
}
```

Create `apps/web/src/hooks/use-campaigns.ts`:
```typescript
"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import type { Criteria } from "@/lib/types";

export function useCampaigns() {
  return useQuery({
    queryKey: queryKeys.campaigns.all,
    queryFn: () => apiClient.getCampaigns(),
  });
}

export function useCampaign(id: string) {
  return useQuery({
    queryKey: queryKeys.campaigns.detail(id),
    queryFn: () => apiClient.getCampaign(id),
  });
}

export function useCreateCampaign() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { type: string; trigger_type: string; criteria: Criteria }) =>
      apiClient.createCampaign(data),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.campaigns.all }),
  });
}
```

Create `apps/web/src/hooks/use-deals.ts`:
```typescript
"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";

export function useDeals(params?: Record<string, string>) {
  return useQuery({
    queryKey: queryKeys.deals.list(params || {}),
    queryFn: () => apiClient.getDeals(params),
  });
}

export function useDeal(id: string) {
  return useQuery({
    queryKey: queryKeys.deals.detail(id),
    queryFn: () => apiClient.getDeal(id),
  });
}

export function useApproveDeal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiClient.approveDeal(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.deals.all }),
  });
}

export function useRejectDeal() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, reason }: { id: string; reason?: string }) =>
      apiClient.rejectDeal(id, reason),
    onSuccess: () => qc.invalidateQueries({ queryKey: queryKeys.deals.all }),
  });
}
```

- [ ] **Step 3: Create pages**

Create `apps/web/src/app/(app)/layout.tsx`:
```tsx
"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { AppShell } from "@/components/app-shell";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => new QueryClient());

  return (
    <QueryClientProvider client={queryClient}>
      <AppShell>{children}</AppShell>
    </QueryClientProvider>
  );
}
```

Create `apps/web/src/app/(app)/dashboard/page.tsx`:
```tsx
"use client";

import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api-client";
import { queryKeys } from "@/lib/query-keys";
import { MetricCard } from "@/components/metric-card";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { ScoreBadge } from "@/components/score-badge";

export default function DashboardPage() {
  const { data, isLoading } = useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: () => apiClient.getDashboardSummary(),
  });

  if (isLoading) return <div className="p-4">Loading...</div>;

  return (
    <div className="space-y-6">
      <PageHeader title="Dashboard" description="Overview of your sourcing pipeline" />

      <div className="grid gap-4 md:grid-cols-3">
        <MetricCard title="Pending Review" value={data?.deals_pending_review ?? 0} description="Deals awaiting your decision" />
        <MetricCard title="Approved Deals" value={data?.deals_approved ?? 0} description="Ready for sourcing" />
        <MetricCard title="Active Campaigns" value={data?.active_campaigns ?? 0} description="Currently running" />
      </div>

      <div>
        <h2 className="mb-3 text-lg font-medium">Recent Deals</h2>
        {data?.recent_deals && data.recent_deals.length > 0 ? (
          <div className="rounded-lg border">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-2 text-left font-medium">ASIN</th>
                  <th className="px-4 py-2 text-left font-medium">Title</th>
                  <th className="px-4 py-2 text-left font-medium">Score</th>
                  <th className="px-4 py-2 text-left font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {data.recent_deals.map((deal) => (
                  <tr key={deal.id} className="border-b last:border-0">
                    <td className="px-4 py-2 font-mono text-xs">{deal.asin}</td>
                    <td className="px-4 py-2">{deal.title}</td>
                    <td className="px-4 py-2"><ScoreBadge score={Math.round(deal.scores.overall)} /></td>
                    <td className="px-4 py-2"><StatusPill status={deal.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">No deals yet. Create a campaign to get started.</p>
        )}
      </div>
    </div>
  );
}
```

Create `apps/web/src/app/(app)/campaigns/page.tsx`:
```tsx
"use client";

import Link from "next/link";
import { useCampaigns } from "@/hooks/use-campaigns";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";

export default function CampaignsPage() {
  const { data: campaigns, isLoading } = useCampaigns();

  if (isLoading) return <div className="p-4">Loading...</div>;

  return (
    <div className="space-y-6">
      <PageHeader
        title="Campaigns"
        description="Research campaigns and discovery runs"
        action={
          <Link href="/campaigns/new">
            <Button>New Campaign</Button>
          </Link>
        }
      />

      {!campaigns || campaigns.length === 0 ? (
        <EmptyState
          title="No campaigns yet"
          description="Create your first campaign to start discovering profitable products."
          action={
            <Link href="/campaigns/new">
              <Button>Create Campaign</Button>
            </Link>
          }
        />
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-2 text-left font-medium">Type</th>
                <th className="px-4 py-2 text-left font-medium">Keywords</th>
                <th className="px-4 py-2 text-left font-medium">Status</th>
                <th className="px-4 py-2 text-left font-medium">Created</th>
              </tr>
            </thead>
            <tbody>
              {campaigns.map((c) => (
                <tr key={c.id} className="border-b last:border-0">
                  <td className="px-4 py-2">
                    <Link href={`/campaigns/${c.id}`} className="text-primary hover:underline">
                      {c.type}
                    </Link>
                  </td>
                  <td className="px-4 py-2">{c.criteria.keywords?.join(", ") || "—"}</td>
                  <td className="px-4 py-2"><StatusPill status={c.status} /></td>
                  <td className="px-4 py-2 text-muted-foreground">{new Date(c.created_at).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
```

Create `apps/web/src/app/(app)/campaigns/new/page.tsx`:
```tsx
"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useCreateCampaign } from "@/hooks/use-campaigns";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function NewCampaignPage() {
  const router = useRouter();
  const createCampaign = useCreateCampaign();
  const [keywords, setKeywords] = useState("");
  const [marketplace, setMarketplace] = useState("US");
  const [minMargin, setMinMargin] = useState("30");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await createCampaign.mutateAsync({
      type: "manual",
      trigger_type: "dashboard",
      criteria: {
        keywords: keywords.split(",").map((k) => k.trim()).filter(Boolean),
        min_margin_pct: parseFloat(minMargin) || undefined,
        marketplace,
      },
    });
    router.push("/campaigns");
  };

  return (
    <div className="space-y-6">
      <PageHeader title="New Campaign" description="Start a new product research campaign" />

      <Card className="max-w-lg">
        <CardHeader>
          <CardTitle>Campaign Criteria</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="text-sm font-medium">Keywords (comma-separated)</label>
              <Input
                value={keywords}
                onChange={(e) => setKeywords(e.target.value)}
                placeholder="kitchen gadgets, home fitness"
                required
              />
            </div>
            <div>
              <label className="text-sm font-medium">Marketplace</label>
              <select
                value={marketplace}
                onChange={(e) => setMarketplace(e.target.value)}
                className="flex h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm"
              >
                <option value="US">US</option>
                <option value="UK">UK</option>
                <option value="EU">EU</option>
              </select>
            </div>
            <div>
              <label className="text-sm font-medium">Minimum Margin %</label>
              <Input
                type="number"
                value={minMargin}
                onChange={(e) => setMinMargin(e.target.value)}
                placeholder="30"
              />
            </div>
            <Button type="submit" disabled={createCampaign.isPending}>
              {createCampaign.isPending ? "Creating..." : "Create Campaign"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
```

Create `apps/web/src/app/(app)/deals/page.tsx`:
```tsx
"use client";

import Link from "next/link";
import { useState } from "react";
import { useDeals } from "@/hooks/use-deals";
import { PageHeader } from "@/components/page-header";
import { StatusPill } from "@/components/status-pill";
import { ScoreBadge } from "@/components/score-badge";
import { EmptyState } from "@/components/empty-state";
import { Input } from "@/components/ui/input";

export default function DealsPage() {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const params: Record<string, string> = {};
  if (search) params.search = search;
  if (status) params.status = status;

  const { data, isLoading } = useDeals(params);

  return (
    <div className="space-y-6">
      <PageHeader title="Deal Explorer" description={`${data?.total ?? 0} deals found`} />

      <div className="flex gap-3">
        <Input
          placeholder="Search by title, brand, or ASIN..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
        <select
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          className="flex h-9 rounded-md border bg-transparent px-3 py-1 text-sm"
        >
          <option value="">All statuses</option>
          <option value="needs_review">Needs Review</option>
          <option value="approved">Approved</option>
          <option value="rejected">Rejected</option>
        </select>
      </div>

      {isLoading ? (
        <div>Loading...</div>
      ) : !data?.deals || data.deals.length === 0 ? (
        <EmptyState title="No deals found" description="Adjust your filters or run a campaign to generate deals." />
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="px-4 py-2 text-left font-medium">ASIN</th>
                <th className="px-4 py-2 text-left font-medium">Title</th>
                <th className="px-4 py-2 text-left font-medium">Brand</th>
                <th className="px-4 py-2 text-left font-medium">Score</th>
                <th className="px-4 py-2 text-left font-medium">Margin</th>
                <th className="px-4 py-2 text-left font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {data.deals.map((deal) => (
                <tr key={deal.id} className="border-b last:border-0 hover:bg-muted/30">
                  <td className="px-4 py-2">
                    <Link href={`/deals/${deal.id}`} className="font-mono text-xs text-primary hover:underline">
                      {deal.asin}
                    </Link>
                  </td>
                  <td className="px-4 py-2">{deal.title}</td>
                  <td className="px-4 py-2 text-muted-foreground">{deal.brand}</td>
                  <td className="px-4 py-2"><ScoreBadge score={Math.round(deal.scores.overall)} /></td>
                  <td className="px-4 py-2"><ScoreBadge score={deal.scores.margin} label="Margin" /></td>
                  <td className="px-4 py-2"><StatusPill status={deal.status} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
```

Create `apps/web/src/app/(app)/deals/[id]/page.tsx`:
```tsx
"use client";

import { use } from "react";
import { useDeal, useApproveDeal, useRejectDeal } from "@/hooks/use-deals";
import { PageHeader } from "@/components/page-header";
import { ScoreBadge } from "@/components/score-badge";
import { StatusPill } from "@/components/status-pill";
import { EvidencePanel } from "@/components/evidence-panel";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export default function DealDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { data: deal, isLoading } = useDeal(id);
  const approve = useApproveDeal();
  const reject = useRejectDeal();

  if (isLoading) return <div className="p-4">Loading...</div>;
  if (!deal) return <div className="p-4">Deal not found</div>;

  return (
    <div className="space-y-6">
      <PageHeader
        title={deal.title}
        description={`${deal.asin} · ${deal.brand} · ${deal.category}`}
        action={
          deal.status === "needs_review" ? (
            <div className="flex gap-2">
              <Button
                variant="outline"
                onClick={() => reject.mutate({ id: deal.id, reason: "Not a good fit" })}
                disabled={reject.isPending}
              >
                Reject
              </Button>
              <Button
                onClick={() => approve.mutate(deal.id)}
                disabled={approve.isPending}
              >
                Approve
              </Button>
            </div>
          ) : (
            <StatusPill status={deal.status} />
          )
        }
      />

      <div className="grid gap-4 md:grid-cols-5">
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Demand</CardTitle></CardHeader>
          <CardContent><ScoreBadge score={deal.scores.demand} /></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Competition</CardTitle></CardHeader>
          <CardContent><ScoreBadge score={deal.scores.competition} /></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Margin</CardTitle></CardHeader>
          <CardContent><ScoreBadge score={deal.scores.margin} /></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Risk</CardTitle></CardHeader>
          <CardContent><ScoreBadge score={deal.scores.risk} /></CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2"><CardTitle className="text-sm">Sourcing</CardTitle></CardHeader>
          <CardContent><ScoreBadge score={deal.scores.sourcing_feasibility} /></CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader><CardTitle>Overall Score</CardTitle></CardHeader>
        <CardContent>
          <div className="text-3xl font-bold">{deal.scores.overall.toFixed(1)}/10</div>
          <p className="mt-1 text-sm text-muted-foreground">
            Reviewer verdict: {deal.reviewer_verdict} (iteration {deal.iteration_count})
          </p>
        </CardContent>
      </Card>

      <div>
        <h2 className="mb-3 text-lg font-medium">Agent Evidence</h2>
        <EvidencePanel evidence={deal.evidence} />
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Update root page to redirect**

Create `apps/web/src/app/page.tsx`:
```tsx
import { redirect } from "next/navigation";

export default function Home() {
  redirect("/dashboard");
}
```

- [ ] **Step 5: Verify frontend builds**

```bash
cd /Users/pluriza/Documents/Work/Pluriza/Estori/fba_agent_orchestrator/apps/web
npm run build
```

Expected: build succeeds (there may be warnings about missing API, which is expected)

- [ ] **Step 6: Commit**

```bash
cd /Users/pluriza/Documents/Work/Pluriza/Estori/fba_agent_orchestrator
git add apps/web/
git commit -m "feat: add dashboard, campaigns, and deals pages with shared components"
```

---

## Task 12: CLAUDE.md + Final Documentation

**Files:**
- Create: `CLAUDE.md`
- Update: `Makefile` (if needed)

- [ ] **Step 1: Create `CLAUDE.md`**

```markdown
# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Purpose

Multi-tenant SaaS for Amazon FBA wholesale automation. Agents source, score, and list products — humans approve every critical decision.

## Architecture

Three-layer system:
- **Research Pipeline** (OpenFang/ZeroClaw) — 7-agent quality-gated product research
- **Operations Core** (Go) — deal lifecycle, tenancy, approvals, domain events
- **Continuous Improvement** (Autoresearch + PostHog) — A/B experiments on scoring config

Hexagonal architecture: domain types in `internal/domain/`, interfaces in `internal/port/`, implementations in `internal/adapter/`, business logic in `internal/service/`.

## Stack

- Backend: Go 1.22+, chi router, pgx for Postgres
- Frontend: Next.js 14 (App Router), TypeScript, Tailwind, shadcn/ui, TanStack Query
- Auth + DB: Supabase (Postgres + Auth + RLS)
- Durable execution: Inngest
- Agent orchestration: OpenFang (primary), ZeroClaw (evaluation planned)
- Analytics + flags: PostHog

## Commands

```bash
make docker-up          # start Postgres + Inngest dev server
make dev                # run Go API server
make test               # run all Go tests
make test-one TEST=Name # run a single test by name
make migrate            # run database migrations
make web-install        # install frontend deps
make web-dev            # run Next.js dev server
make build              # build Go binary
```

## Core Principles

- Business logic in `internal/service/`, NEVER in adapters or handlers
- All external systems behind interfaces in `internal/port/`
- No service directly imports OpenFang, Inngest, or Supabase SDKs
- Agents produce suggestions, humans approve — no auto-execution of critical actions
- Every state transition emits a domain event
- Context propagation everywhere (`context.Context`)
- Tenant ID required in every query — RLS as safety net, not primary mechanism

## When Adding Features

1. Domain types in `internal/domain/`
2. Port interfaces in `internal/port/`
3. Service logic in `internal/service/`
4. Adapter implementation in `internal/adapter/`
5. HTTP handler in `internal/api/handler/`
6. Route in `internal/api/router.go`
7. Frontend types in `apps/web/src/lib/types.ts`
8. Frontend hook + page

## Forbidden

- Business logic inside adapters
- Direct DB access from handlers (always go through service -> repo)
- Tight coupling to any orchestration framework
- Auto-applying risky changes without human approval
- Global mutable state
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add CLAUDE.md with architecture guide and dev commands"
```

---

## Self-Review Results

**Spec coverage:**
- Campaign model + CRUD: Task 2, 5, 6, 7 ✓
- Deal model + lifecycle + approve/reject: Task 2, 5, 6, 7 ✓
- ScoringConfig + discovery config: Task 2, 5, 6, 7 ✓
- Research pipeline interface: Task 3 ✓
- Agent pipeline structure: Task 3 (interface), Task 8 (OpenFang stub) ✓
- Domain events + audit trail: Task 2, 5, 6, 7 ✓
- Inngest durable workflows: Task 8 (stub) ✓
- Supabase auth: Task 8 (stub with dev mode) ✓
- PostHog analytics: Task 8 (stub with no-op fallback) ✓
- Dashboard page: Task 11 ✓
- Campaign list + create: Task 11 ✓
- Deal explorer + detail + approve/reject: Task 11 ✓
- Spreadsheet upload: Covered by campaign model (trigger_type=spreadsheet) — actual upload UI deferred to a follow-up task
- Chat interface (OpenFang channels): Covered by AgentRuntime interface — actual channel config is OpenFang-side, not in our codebase
- Continuous discovery scheduler: Covered by DiscoveryConfig model + Inngest stub — actual scheduling logic deferred to Inngest implementation

**Placeholder scan:** No TBDs or vague steps. All TODOs are in adapter stubs and are clearly marked as future implementation (OpenFang API calls, Inngest event sends, Supabase JWT parsing, PostHog HTTP calls).

**Type consistency:** Verified Campaign, Deal, ScoringConfig, ResearchResult types are used consistently across domain, service, handler, and frontend types. `DealScores` field names match between Go and TypeScript.

**Gap noted:** The test in Task 6 is a unit test with in-memory doubles. Integration tests against Postgres are not included in this plan — they require the docker-compose stack and are better added once the basic flow works end-to-end.
