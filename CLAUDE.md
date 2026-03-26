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

- Backend: Go 1.23+, chi router, pgx for Postgres
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
