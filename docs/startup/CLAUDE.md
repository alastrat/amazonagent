# CLAUDE.md

## Purpose
This repository implements a multi-tenant SaaS for Amazon FBA wholesale automation.

## Core Principles
- Business logic MUST be decoupled from orchestration runtimes
- All external systems must be behind interfaces
- Temporal is the primary durable execution engine
- Supabase is the source of truth for auth and database
- PostHog is the system for feature flags, experiments, and analytics
- Cloudflare Workers is EDGE ONLY (never core backend)

## Architecture Rules
- Use hexagonal architecture
- No service should directly depend on:
  - OpenFang
  - LangGraph
  - CrewAI
  - Temporal SDK
- Always go through internal interfaces

## Agent System Rules
- Agents are NOT the source of truth
- Agents produce suggestions, not final decisions
- All critical actions require validation or approval

## Experimentation Rules
- All changes must be measurable
- Prefer shadow mode before rollout
- Never auto-apply risky changes

## Coding Rules
- Idiomatic Go
- Strong typing
- Context propagation everywhere
- No global state
- No hidden side effects

## When adding features
- First extend domain models
- Then services
- Then API
- Then workflows
- Then frontend

## Forbidden
- Tight coupling to any orchestration framework
- Business logic inside adapters
- Feature flags only on frontend
- Direct DB access from frontend