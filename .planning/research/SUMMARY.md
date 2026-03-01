# Project Research Summary

**Project:** Redyx — Anonymous Community Discussion Platform
**Domain:** Social discussion platform (Reddit clone) with Go microservices backend
**Researched:** 2026-03-02
**Confidence:** HIGH

## Executive Summary

Redyx is an anonymous Reddit-style discussion platform built as 12 Go microservices communicating over gRPC, fronted by Envoy for REST-to-gRPC transcoding, with an Astro+Svelte SSR frontend. The platform's core value proposition is privacy-first pseudonymous discussion with optional anonymous posting. Experts building this type of system use a strict service-per-domain approach: separate PostgreSQL instances per core service (auth, user, community, post), ScyllaDB for the high-write comment tree, Redis for real-time state (votes, sessions, rate limiting, caching), and Kafka for asynchronous event fan-out across services. The stack is mature, all library versions are verified against Feb/Mar 2026 releases, and the patterns are well-established in Go production systems.

The recommended approach is to build in strict dependency order: shared platform libraries and proto definitions first, then auth → user → community → post → comment → vote, followed by supporting services (search, notifications, media, moderation). The Envoy gateway and Astro frontend should be built alongside the core services, not after. The single Go module monorepo structure eliminates multi-module complexity and enables atomic cross-service refactors. The key architectural decision — Envoy's gRPC-JSON transcoding — eliminates an entire custom REST gateway, but introduces a critical build artifact dependency (the proto descriptor file) that must be managed from day one.

The most dangerous risks are: (1) **ScyllaDB comment schema** — an unbounded partition design will break catastrophically on viral posts and is nearly impossible to migrate after data exists; (2) **Envoy transcoding field name mismatches** — silently drops data, causes days of debugging per service if not caught early; (3) **Kafka consumer idempotency** — without set-based vote storage, duplicate processing inflates counts permanently; (4) **Home feed aggregation** — cross-shard queries across joined communities is the single hardest feature in the platform. All four risks have well-documented prevention strategies detailed in the research.

## Key Findings

### Recommended Stack

The stack is a pure Go backend (Go 1.26) with gRPC for all inter-service communication and Envoy v1.37 as the API gateway performing REST-to-gRPC transcoding. All 12 services are in a single Go module monorepo. The frontend uses Astro 5 (SSR) with Svelte 5 interactive islands — shipping minimal JavaScript for a content-heavy discussion site.

**Core technologies:**
- **Go 1.26 + grpc-go v1.79.1**: All 12 microservices, native concurrency, zero-cost gRPC — verified Feb 2026 release
- **Envoy v1.37.0**: API gateway with gRPC-JSON transcoding, JWT validation, rate limiting — eliminates custom REST gateway code
- **pgx v5.8.0**: PostgreSQL driver for 5 separate PG instances (auth, user, community, post-sharded, platform)
- **ScyllaDB via scylladb/gocql v1.17.1**: Comment tree storage with shard-aware connection pooling — must use ScyllaDB fork, NOT upstream gocql
- **Redis via go-redis v9.18.0**: 6 logical databases (sessions, rate limiting, auth tokens, pub/sub, caching, spam scoring)
- **Kafka via franz-go v1.20.7**: Pure Go client (no CGO) for event-driven async communication between services
- **Meilisearch via meilisearch-go v0.36.1**: Full-text search indexed from Kafka events
- **Astro 5 + Svelte 5**: SSR frontend with hydrated interactive islands for votes, comments, search, notifications
- **buf v1.66.0**: Proto management — replaces raw protoc entirely for linting, generation, and breaking change detection
- **OpenTelemetry v1.40.0 + Zap v1.27.1**: Distributed tracing and structured logging across all services

**Critical version/library warnings:**
- Use `scylladb/gocql` NOT `gocql/gocql` — 30-50% performance difference on ScyllaDB
- Use `coder/websocket` NOT `gorilla/websocket` — gorilla is archived/unmaintained
- Use `golang-jwt/jwt/v5` NOT `dgrijalva/jwt-go` — security vulnerabilities
- Use `google.golang.org/protobuf` NOT `github.com/golang/protobuf` — deprecated since 2020
- Do NOT use HTTP frameworks (gin, echo, fiber) — services expose gRPC only, Envoy handles REST
- Do NOT use ORMs (GORM, ent) — raw SQL with pgx for sharding compatibility

### Expected Features

**Must have (table stakes):**
- Email/password auth with JWT tokens + email OTP verification
- **Password reset flow** (NOT in SRS — must add; users will forget passwords)
- Google OAuth social login
- Username-only public identity (core anonymity value)
- Communities: create, join/leave, public/restricted/private visibility, roles (owner/mod/member)
- Text and link posts with markdown rendering
- Nested/threaded comments (THE defining feature of Reddit-like platforms)
- Upvote/downvote with toggle behavior and optimistic UI updates
- Community feed with Hot/New/Top/Rising sort + time filters
- Home feed aggregated from joined communities (hardest feature)
- Basic moderation: remove posts/comments, temp/perm bans, mod log
- Per-user rate limiting with new account restrictions

**Should have (differentiators):**
- Anonymous posting mode (`[anonymous]` author) — Redyx's core differentiator
- WebSocket real-time notifications (replies, mentions)
- Full-text search with community autocomplete
- Media posts (image upload + CDN delivery)
- Wilson score "Best" comment sorting (Reddit adopted 2009)
- Lemmy-style hot ranking algorithm (improved over Reddit's original)
- Content reporting and mod queue
- Pre-publish spam filtering (keyword blocklist, URL check, duplicate detection)

**Defer (v2+):**
- DMs/chat (massive scope creep, essentially a second product)
- Flair/tag system, crossposting, awards/coins
- Post-publish behavior analysis, vote manipulation detection, shadow-banning
- User-to-user blocking, NSFW tagging, push notifications
- Custom CSS/themes per community
- Video upload (transcoding complexity)

### Architecture Approach

The system is a single Go module monorepo with 12 services following a strict server → service → repository layered pattern. Services communicate synchronously via gRPC and asynchronously via Kafka events. Each service owns its data store. A shared `internal/platform/` package provides common gRPC bootstrap, middleware (auth, logging, tracing, recovery), database connection helpers, and Kafka producer/consumer wrappers. Envoy sits at the edge, performing JWT validation, rate limiting, and REST-to-gRPC transcoding using compiled proto descriptor files.

**Major components:**
1. **Envoy API Gateway** — JWT validation, rate limiting, REST↔gRPC transcoding, CORS, TLS termination
2. **Auth Service** — Registration, login, JWT issuance, OAuth, token revocation via Redis blocklist
3. **User/Community/Post Services** — Core domain CRUD with strict ownership boundaries
4. **Post Service (sharded)** — Consistent hashing on `community_id` routing to multiple PostgreSQL instances
5. **Comment Service (ScyllaDB)** — Threaded comments with `(post_id, parent_id)` composite partition key for bounded partitions
6. **Vote Service** — Redis set-based voting (idempotent), Kafka event publishing for async score/karma propagation
7. **Kafka Event Bus** — 6 versioned topics enabling decoupled async processing (search indexing, notifications, spam analysis, karma updates)
8. **Notification Service** — WebSocket delivery with Redis Pub/Sub for cross-pod fan-out
9. **Astro+Svelte Frontend** — SSR page shells with hydrated Svelte islands for interactive UI

### Critical Pitfalls

1. **Envoy transcoding field name mismatch** — Proto snake_case silently becomes camelCase in JSON; fields arrive as zero-values in Go handlers. **Fix:** Decide convention in Phase 1, add integration tests through Envoy for every RPC. Time cost if missed: 1-2 days per service × 12 services.

2. **ScyllaDB unbounded comment partitions** — Single `post_id` partition key causes 100MB+ partitions on viral posts. **Fix:** Use composite `(post_id, parent_id)` partition key from day one. Schema is impossible to change after data exists. Time cost if missed: 2-5 day rewrite.

3. **Kafka consumer offset duplication without idempotency** — Counter-based vote storage double-counts on consumer restart. **Fix:** Use Redis sets (`SADD`) not counters (`INCR`) — sets are naturally idempotent. Time cost if missed: 2-3 day redesign + data reconciliation.

4. **JWT tokens unrevocable after ban** — Access tokens valid for 15min after user is banned. **Fix:** Redis blocklist with TTL matching token lifetime, `jti` claim in every token. Must be designed into JWT claims from day one.

5. **Proto descriptor file drift** — Update .proto files and Go code but forget to rebuild Envoy's descriptor file. New RPCs return 404 via REST. **Fix:** Single `make proto` target that generates Go code AND descriptor. CI check that descriptor matches checked-in version.

6. **Kafka consumer rebalancing storms during deploys** — Rolling deploys trigger cascading rebalances, freezing event processing for 30-60s. **Fix:** Use CooperativeStickyAssignor, 30s session timeout, graceful shutdown with explicit offset commit.

## Implications for Roadmap

Based on combined research, I recommend 7 phases structured around dependency ordering, architectural boundaries, and risk mitigation:

### Phase 1: Foundation — Proto, Platform Libraries, Build Tooling
**Rationale:** Every service depends on shared proto definitions, gRPC bootstrap, middleware (auth/logging/tracing/error-mapping), and database connection helpers. Building this first prevents 12 services from independently reinventing these patterns. Also establishes Envoy transcoding conventions that prevent the #1 pitfall.
**Delivers:** `proto/` definitions with `google.api.http` annotations for all core services, `gen/` generated code, `internal/platform/` shared libraries (grpcserver, middleware, config, database, kafka, redis, errors, pagination), buf.yaml + buf.gen.yaml, Makefile with `proto` target, Docker Compose infrastructure file, Envoy base configuration with transcoding.
**Addresses:** Build tooling, proto conventions, error handling standardization
**Avoids:** Envoy field name mismatch (Pitfall 1), route matching confusion (Pitfall 2), proto descriptor drift (Pitfall 3), gRPC error code misuse (Pitfall 11), proto field number safety (Pitfall 12)

### Phase 2: Auth + User + Community — Identity and Structure
**Rationale:** Auth is the universal dependency — every authenticated request needs tokens. User profiles are needed for denormalization into posts/comments. Communities must exist before posts can be created. These three form the identity and organizational foundation.
**Delivers:** Working auth flow (register, login, JWT, OAuth, token revocation), user profiles with karma display, community CRUD with visibility enforcement and role hierarchy. Rate limiting at Envoy level.
**Addresses:** Table stakes: registration, login, profiles, community creation, join/leave, rate limiting, password reset
**Avoids:** JWT revocation gap (Pitfall 10), auth endpoint brute-force (no rate limiting)

### Phase 3: Posts + Voting + Feeds — Core Content Loop
**Rationale:** Posts are the fundamental content unit; without them, there's nothing to discuss or vote on. Voting enables ranking algorithms. Community feeds and the home feed are the primary content consumption patterns. This is where the hardest feature (home feed cross-shard aggregation) lives.
**Delivers:** Text and link posts in communities, post sharding with consistent hashing, upvote/downvote with Redis set-based idempotent storage, Kafka event pipeline (votes → score updates → karma), community feeds with Hot/New/Top/Rising sort, home feed aggregated from subscriptions.
**Addresses:** Table stakes: posts, voting, feed sorting, auto-upvote. Differentiators: Lemmy hot ranking, Wilson score for "Best"
**Avoids:** Consistent hash rebalancing (Pitfall 8) — use virtual nodes + explicit shard mapping; Kafka offset duplication (Pitfall 7) — set-based voting from day one

### Phase 4: Comments — Threaded Discussion
**Rationale:** Comments are the second most complex feature after home feeds. ScyllaDB schema decisions are irreversible. The comment tree model (composite partition key, materialized path, lazy-loading) must be designed correctly before any data is written.
**Delivers:** Nested/threaded comments on posts, lazy-load deep threads ("load more replies"), comment sorting (Best/Top/New/Controversial), soft-delete preserving thread structure, comment score updates via Kafka.
**Addresses:** Table stakes: comments, threaded replies, collapsed threads, deleted comment handling
**Avoids:** ScyllaDB unbounded partitions (Pitfall 4), tombstone accumulation from deletions (Pitfall 5)

### Phase 5: Search + Notifications + Media — Engagement Layer
**Rationale:** These are Kafka consumers that react to content events. They're independent of each other and can be built in parallel. They transform the platform from functional to engaging.
**Delivers:** Full-text search (posts, communities, comments) via Meilisearch, community autocomplete, WebSocket real-time notifications with Redis Pub/Sub for cross-pod delivery, notification preferences, media upload pipeline (validation → S3 → CDN).
**Addresses:** Differentiators: search, real-time notifications, `u/mentions`, media posts, save/bookmark
**Avoids:** WebSocket cross-pod delivery failure (Pitfall 9), Meilisearch async indexing assumptions

### Phase 6: Moderation + Spam — Platform Safety
**Rationale:** Moderation tools depend on all content services being in place. Spam detection requires content creation events flowing through Kafka. These are essential for any public platform but don't block core functionality.
**Delivers:** Mod queue with report aggregation, content reporting by users, post/comment removal (distinct from author deletion), temp/permanent bans with enforcement on writes, mod log transparency, pre-publish spam filtering (keyword/URL/duplicate), anonymous posting mode.
**Addresses:** Table stakes: basic moderation. Differentiators: anonymous posting, spam filtering, new account restrictions

### Phase 7: Frontend + Deployment + Observability — Production Readiness
**Rationale:** The Astro+Svelte frontend should be developed progressively alongside backend phases, but the full production deployment (K8s manifests, Helm charts, monitoring stack) should come last once all services are stable and tested.
**Delivers:** Complete Astro SSR frontend with Svelte islands, Kubernetes manifests with HPA, Helm charts, Prometheus+Grafana dashboards, Loki+Promtail log aggregation, Jaeger distributed tracing, CI/CD pipeline, graceful shutdown verification.
**Addresses:** All UX patterns (feed cards, comment trees, vote arrows, community sidebars, mobile-responsive), production reliability

### Phase Ordering Rationale

- **Dependency chain:** Auth → User → Community → Post → Comment → Vote → Search/Notifications/Moderation. Each layer consumes the previous.
- **Risk front-loading:** Phase 1 eliminates 5 of 12 critical pitfalls. Phase 3-4 tackles the two hardest features (home feed, comment trees) early when pivoting is cheapest.
- **Kafka infrastructure** needed from Phase 3 onward. Redis from Phase 2. ScyllaDB only from Phase 4 — can be added to Docker Compose when needed.
- **Frontend development** should happen incrementally: basic page layouts in Phase 2, feed/voting UI in Phase 3, comment tree UI in Phase 4. Phase 7 is full polish and production deployment.
- **The "core loop" (browse → read → vote → comment → reply)** is complete by end of Phase 4. Everything after is enhancement.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 3 (Posts + Feeds):** Home feed cross-shard aggregation is the hardest feature. Needs research into caching strategies, merge-sort across shards, and cursor-based pagination across heterogeneous result sets.
- **Phase 4 (Comments):** ScyllaDB schema design is irreversible. Needs research into materialized path encoding, comment tree reconstruction algorithms, and ScyllaDB-specific pagination patterns.
- **Phase 5 (Notifications):** WebSocket lifecycle management across deployments, Redis Pub/Sub reliability, offline notification storage and catch-up on reconnect.

Phases with standard patterns (skip research-phase):
- **Phase 1 (Foundation):** Proto management with buf, gRPC middleware interceptors, and Envoy configuration are thoroughly documented in official sources.
- **Phase 2 (Auth/User/Community):** Standard JWT auth, OAuth2 code flow, PostgreSQL CRUD — well-established patterns with no novel challenges.
- **Phase 6 (Moderation):** CRUD operations with authorization checks. Content reporting aggregation is straightforward.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | **HIGH** | All versions verified from official GitHub releases/tags as of Feb/Mar 2026. Every library has been checked for compatibility and active maintenance. |
| Features | **HIGH** | Grounded in existing SRS (45 user stories), cross-referenced with Reddit/Lemmy UX patterns and ranking algorithm research (Wilson score, Lemmy hot sort). |
| Architecture | **HIGH** | Go monorepo patterns, Envoy transcoding, Kafka topic design, and per-service database isolation are well-established patterns verified against official Go module docs, Envoy docs, and buf.build docs. |
| Pitfalls | **HIGH** | Stack-specific pitfalls verified against official documentation (Envoy transcoder docs, ScyllaDB tombstone KB, Kafka consumer protocol). Includes concrete code examples and recovery strategies. |

**Overall confidence:** HIGH

### Gaps to Address

- **Home feed aggregation strategy:** Research identifies this as "the hardest feature" but doesn't prescribe a specific caching/materialization approach. Options range from fan-out-on-write (pre-compute feeds) to fan-out-on-read (query-time aggregation). Needs Phase 3 research spike.
- **ScyllaDB migration tooling:** No standard migration tool exists for ScyllaDB. Research suggests plain CQL scripts with a custom version tracking table. Needs validation during Phase 4 implementation.
- **Envoy rate limiting integration:** Research mentions both local rate limiting (per-connection) and external rate limiting (via the rate-limit service). The exact Envoy configuration for delegating to the Go rate-limit service needs Phase 2 implementation research.
- **Post shard count:** Research recommends "2-4 shards fixed for v1" but doesn't specify the exact number. This affects consistent hash ring configuration and should be decided during Phase 3 planning.
- **Password reset flow:** Identified as missing from the SRS but table stakes. Needs user stories written during Phase 2 requirements.
- **Anonymous posting implementation details:** Core differentiator but deferred to Phase 6. The exact mechanism for hiding identity from API responses while revealing to moderators needs design work.

## Sources

### Primary (HIGH confidence)
- grpc/grpc-go v1.79.1, jackc/pgx v5.8.0, redis/go-redis v9.18.0, twmb/franz-go v1.20.7, golang-jwt v5.3.1 — verified GitHub releases
- Envoy gRPC-JSON transcoder filter, JWT auth filter — official Envoy docs (envoyproxy.io)
- buf.build CLI quickstart, workspace configuration — official buf.build docs
- Go official module layout guidance — go.dev/doc/modules/layout
- ScyllaDB tombstone behavior, anti-entropy — official ScyllaDB docs
- gRPC error handling guide — grpc.io/docs/guides/error
- Project SRS, Architecture Plan, Core Concepts — `/docs/` directory (45 user stories)

### Secondary (MEDIUM confidence)
- Wilson score confidence interval — Evan Miller "How Not To Sort By Average Rating" (evanmiller.org, 2009), adopted by Reddit
- Lemmy ranking algorithm — Lemmy documentation (contributors/07-ranking-algo.html)
- Reddit UX patterns — Wikipedia, training data cross-referenced with Lemmy implementation
- golang-standards/project-layout — community convention, not official Go standard

### Tertiary (needs validation during implementation)
- Exact Envoy external rate limiting integration with Go service
- ScyllaDB comment tree pagination at scale (>10K comments per post)
- Cross-shard home feed aggregation performance characteristics

---
*Research completed: 2026-03-02*
*Ready for roadmap: yes*
