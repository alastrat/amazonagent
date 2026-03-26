You are continuing PASS 4 of a 4-pass build for a production-grade SaaS platform for Amazon FBA wholesale resellers.

IMPORTANT
Assume PASS 1-3 are already complete.
This final pass should add:
- orchestration adapters
- autoresearch-inspired continuous improvement
- experiment automation glue
- Cloudflare edge integration
- final hardening
- tests
- docs polish

Do NOT rewrite the project.
Add the missing advanced capabilities cleanly.

FOCUS OF THIS PASS

1. AGENT RUNTIME ADAPTERS
Complete adapter implementations/scaffolds for:
- OpenFang
- LangGraph
- CrewAI

Requirements:
- preserve the AgentRuntime interface
- keep business services decoupled
- provide realistic request/response models
- support:
  - task execution
  - workflow start
  - status polling
  - structured outputs
  - pause/resume/cancel hooks
  - memory abstraction
  - audit logging hooks

Adapters may be partial if external systems are unavailable, but must be coherent and usable.

2. AUTORESEARCH-INSPIRED ENGINE
Implement a continuous improvement subsystem that can:
- propose candidate changes
- register hypotheses
- create experiment variants
- assign exposure modes
- run shadow evaluations
- compare outcomes
- recommend keep/revert
- require approval for risky changes
- store experiment memory over time

Use cases:
- outreach template variants
- scoring-weight variants
- supplier ranking variants
- risk-threshold variants
- listing enrichment prompt variants
- workflow policy tuning

3. POSTHOG EXPERIMENT GLUE
Integrate the experiment engine with PostHog:
- flag/variant lookup abstraction
- exposure event capture
- conversion/outcome event helpers
- result summaries
- admin diagnostics
- experiment attribution helpers

Do NOT make critical business logic depend only on client-side PostHog.

4. CLOUDFLARE EDGE APP
Expand the edge app to support:
- webhook ingress
- signature validation
- approval link resolution
- edge-safe forwarding to core API
- optional lightweight workflow kickoff
- durable-safe request correlation
- replay protection where appropriate

5. TESTING / HARDENING
Add:
- adapter contract tests
- workflow tests
- experiment simulation tests
- API integration tests where useful
- frontend smoke tests if feasible
- idempotency/retry coverage for critical flows

6. DOCS POLISH
Update:
- README
- local run instructions
- architecture docs
- ADRs
- operational notes
- environment setup docs
- how to swap agent runtimes
- how to switch durable runtimes
- how PostHog is used
- how Supabase is used
- how experiments are governed safely

FINAL DELIVERABLE EXPECTATIONS
By the end of this pass, the repo should feel like a serious starter platform, not a thin mock.

OUTPUT FORMAT
1. List modified/new files first.
2. Then print each file:
FILE: <path>
<full contents>

END WITH
- final implemented architecture summary
- what is production-ready
- what is intentionally stubbed
- recommended next 15 implementation steps