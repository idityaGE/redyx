# State: Redyx

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-02)

**Core value:** Users can anonymously create communities, post content, and have threaded discussions — with minimal personal data collected and maximum privacy preserved.
**Current focus:** Phase 1: Foundation + Frontend Shell

## Current Position

Phase: 1 of 7 (Foundation + Frontend Shell)
Plan: 0 of ? in current phase
Status: Ready to plan
Last activity: 2026-03-02 — Roadmap revised: frontend built progressively alongside each backend phase

Progress: [░░░░░░░░░░] 0%

## Performance Metrics

**Velocity:**
- Total plans completed: 0
- Average duration: —
- Total execution time: 0 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| - | - | - | - |

*Updated after each plan completion*

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- [Roadmap]: 7-phase structure following strict dependency order (foundation → auth → content → comments → engagement → safety → deployment)
- [Roadmap]: Frontend built progressively — Astro+Svelte project initialized in Phase 1, each subsequent phase builds corresponding frontend pages alongside backend services
- [Roadmap]: Phase 7 is deployment + observability only (no frontend feature work, only performance optimization via FEND-04)
- [Roadmap]: ScyllaDB comment schema deferred to Phase 4 — design before coding, schema is irreversible
- [Roadmap]: Home feed cross-shard aggregation is in Phase 3 — hardest feature, needs research spike during planning
- [Roadmap]: Rate limiting placed in Phase 2 with auth (Envoy-level enforcement)

### Context from Init

- Extensive upfront design exists: SRS, architecture plan, 45 user stories, UML diagrams
- Tech stack locked: Go + gRPC + Envoy + Astro/Svelte + PostgreSQL + ScyllaDB + Redis + Kafka + Meilisearch + S3
- Database-per-service pattern with application-level sharding on post service
- Privacy/anonymity is a core architectural constraint, not a feature toggle

### Pending Todos

None yet.

### Blockers/Concerns

- Home feed aggregation strategy not yet decided (fan-out-on-write vs fan-out-on-read) — needs Phase 3 research
- ScyllaDB migration tooling gap — no standard tool, needs custom version tracking
- Envoy rate limiting integration approach (local vs external) — needs Phase 2 implementation research
- Post shard count (2 vs 4 for v1) — affects consistent hash ring, decide during Phase 3 planning

## Session Continuity

Last session: 2026-03-02
Stopped at: Roadmap revised (progressive frontend), ready to plan Phase 1
Resume file: None

---
*Last updated: 2026-03-02 — Roadmap revised (progressive frontend)*
