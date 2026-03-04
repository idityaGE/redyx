---
gsd_state_version: 1.0
milestone: v1.0
milestone_name: milestone
status: planning
stopped_at: Phase 5 context gathered
last_updated: "2026-03-04T20:29:26.332Z"
last_activity: 2026-03-04 — Completed 04-04-PLAN.md (E2E integration verification)
progress:
  total_phases: 7
  completed_phases: 4
  total_plans: 24
  completed_plans: 24
  percent: 100
---

# State: Redyx

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-02)

**Core value:** Users can anonymously create communities, post content, and have threaded discussions — with minimal personal data collected and maximum privacy preserved.
**Current focus:** Phase 5 next — Search + Notifications + Media (Full Stack)

## Current Position

Phase: 4 of 7 — Complete
Plan: 4 of 4 complete in Phase 4
Status: Phase 4 complete — ready for Phase 5 planning
Last activity: 2026-03-04 — Completed 04-04-PLAN.md (E2E integration verification)

Progress: [██████████] 100% (Plan 24/24 overall, Phase 4: 4/4)

## Performance Metrics

**Velocity:**
- Total plans completed: 23
- Average duration: ~9 min
- Total execution time: ~3.4 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 01-foundation-frontend-shell | 3/3 | ~35 min | ~12 min |
| 02-auth-user-community | 10/10 | ~123 min | ~12 min |
| 03-posts-voting-feeds | 7/7 | ~159 min | ~23 min |
| 04-comments | 4/4 | ~34 min | ~9 min |

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
- [01-02]: TailwindCSS v4 CSS-first config with @theme directive, not tailwind.config.js
- [01-02]: JetBrains Mono via Bunny Fonts CDN (privacy-friendly)
- [01-02]: Dark mode default, inline script in head prevents flash
- [01-02]: Svelte 5 runes ($state, $derived) for all interactive components
- [01-01]: buf.gen.yaml uses per-file go_package override for googleapis to resolve import path
- [01-01]: Generated Go code committed to gen/ (not gitignored) per architecture decision
- [01-01]: Health proto renamed to CheckRequest/CheckResponse for buf STANDARD lint compliance
- [01-03]: Envoy match_incoming_request_route: true for REST path routing (Pitfall 2 prevention)
- [01-03]: preserve_proto_field_names: false — camelCase JSON everywhere (Pitfall 1)
- [01-03]: Platform libs in internal/platform/ with grpcserver bootstrap, config, database, redis, middleware, errors, pagination
- [01-03]: Middleware chain order: Recovery → Logging → Auth → RateLimit → ErrorMapping (outermost catches panics)
- [01-03]: Error mapping interceptor sanitizes internal errors — never leaks raw messages to clients
- [02-01]: Fail-open rate limiting — Redis errors allow requests through to preserve availability
- [02-01]: Public methods still attempt optional JWT extraction for rate limit tier differentiation
- [02-01]: Token bucket via Lua script for atomic Redis operations — no race conditions
- [02-05]: Redis DB isolation per service: auth=1, user=2, community=3 (skeleton=0)
- [02-05]: Envoy specific routes before catch-all for first-match routing
- [02-05]: CORS expose-headers includes retry-after for frontend rate-limit visibility
- [02-07]: Auth page pattern: Astro shell + Svelte form with client:load using AuthForm wrapper
- [02-07]: URL query params for cross-page state passing (email, OAuth code, reset token)
- [02-07]: Field-level inline errors in terminal style: "> error: message" in red
- [02-08]: Profile avatar uses Unicode box-drawing chars for authentic terminal frame
- [02-08]: $effect() for prop-to-state sync pattern in Svelte 5 editable components
- [02-08]: Type-to-confirm pattern for destructive actions (account deletion)
- [02-09]: Two-step community creation: POST creates, PATCH adds rules (proto constraint)
- [02-09]: Independent save per settings section, not one big form
- [02-09]: ls -la style table format for community browse page
- [02-10]: Auth interceptor before rate limiter in middleware chain for correct tier differentiation
- [02-10]: Envoy community route uses /api/v1/communities (no trailing slash) for bare path matching
- [02-10]: Refresh token persisted in localStorage — acceptable for dev, revisit for production
- [02-10]: Profile fetch uses JWT-decoded username (route is /users/{username}, not /users/{uuid})
- [02-10]: No dedicated "my communities" RPC — sidebar filters ListCommunities by ownerId client-side
- [02-10]: Logout uses pub/sub instead of page reload for instant [anonymous] UI update
- [03-01]: 2 post shards in same PostgreSQL instance for v1 simplicity
- [03-01]: Community name as shard routing key via consistent hash (serialx/hashring)
- [03-01]: saved_posts centralized on shard_0 to avoid cross-shard coordination
- [03-01]: Hot score precomputed in column, refreshed every 15min for recent posts
- [03-01]: Fan-out-on-read for home feed with 2min Redis cache
- [03-01]: Anonymous posts store real author_id in DB but mask in API responses
- [03-01]: GetPost and ListPosts as public auth methods for anonymous browsing
- [03-02]: Async Kafka publish in Vote RPC — fire-and-forget to keep <50ms response
- [03-02]: Redis-only vote service (no PostgreSQL) — Kafka provides durability
- [03-02]: Redis SADD deduplication for karma consumer — 24h TTL on processed set
- [03-02]: 6-partition Kafka topic for votes — explicit creation on startup prevents wrong defaults
- [03-03]: ScoreConsumer reads Redis SCARD for idempotent set-based score derivation (not delta-based)
- [03-03]: Kafka KRaft mode (bitnami/kafka:3.7) — no Zookeeper dependency
- [03-03]: Regex route for /api/v1/communities/{name}/posts before community catch-all
- [03-03]: Karma consumer as goroutine in user-service (not a separate container)
- [03-04]: Component-local $state for vote state (not global store) — each post card owns its own vote state
- [03-04]: marked + DOMPurify for client-side markdown rendering (user content changes, preview needs client rendering)
- [03-04]: IntersectionObserver with 200px rootMargin for infinite scroll trigger
- [03-04]: $effect() for sort/timeRange reactive reset in FeedList
- [03-05]: HomeFeed and CommunityFeed compose SortBar+FeedList with local $state sort management
- [03-05]: Post submit uses tab bar (Text/Link/Media) with Write/Preview sub-tabs for markdown body
- [03-05]: PostDetail inline edit mode toggles form fields in-place (not separate page)
- [03-05]: Delete uses inline yes/no confirmation pattern (not browser confirm dialog)
- [03-06]: Saved tab only on own profile via $derived tabs with isOwnProfile conditional
- [03-06]: Auth guard redirects to /login?redirect=/saved for unauthenticated users
- [03-06]: Optimistic unsave removes post immediately, restores on API failure
- [03-07]: ListHomeFeed serves public feed for anonymous users (queries all shards)
- [03-07]: Live vote scores overlaid from Redis vote-service on all feed endpoints via pipeline batch
- [03-07]: Astro ClientRouter + transition:persist prevents sidebar/header remount on navigation
- [03-07]: whenReady() promise pattern for auth initialization — all components await before API calls
- [03-07]: untrack() pattern for $effect + $state mutation — prevents infinite loops in Svelte 5
- [03-07]: Kafka producer uses context.Background() for fire-and-forget produce (not request context)
- [04-01]: ScyllaDB dual-table write pattern: comments_by_post (query) + comments_by_id (lookup), both updated on every mutation
- [04-01]: Counter table for atomic materialized path generation (avoids race conditions)
- [04-01]: Read-increment-write for reply_count (acceptable minor race risk for v1)
- [04-01]: In-memory sort for ListComments (fetch all, sort, paginate) — acceptable for v1 comment volumes
- [04-01]: ScyllaDB connection retry loop (30 attempts, 2s apart) for slow container startup
- [04-01]: Separate Kafka consumer group (comment-service.redyx.votes.v1) on same votes topic
- [04-02]: Comment regex route BEFORE /api/v1/posts prefix to prevent post-service catching comment requests
- [04-02]: ScyllaDB dev mode (--smp 1 --memory 512M --developer-mode 1) for minimal resource usage
- [04-02]: 60s start_period on ScyllaDB healthcheck to accommodate slow cold starts
- [04-03]: Flat list rendering with depth-based padding (no recursive tree-building) — server provides display order
- [04-03]: Auto-collapse comments with voteScore < -5 as initial collapsed state
- [04-03]: localStorage key 'commentSort' for sort preference persistence across posts
- [04-03]: shouldShowLoadMore checks if children already in flat list before showing trigger
- [04-04]: ScyllaDB two-phase connection: connect without keyspace for migrations, then reconnect with keyspace after creation

### Context from Init

- Extensive upfront design exists: SRS, architecture plan, 45 user stories, UML diagrams
- Tech stack locked: Go + gRPC + Envoy + Astro/Svelte + PostgreSQL + ScyllaDB + Redis + Kafka + Meilisearch + S3
- Database-per-service pattern with application-level sharding on post service
- Privacy/anonymity is a core architectural constraint, not a feature toggle

### Pending Todos

None.

### Blockers/Concerns

- ScyllaDB migration tooling gap — solved with simple RunMigrations reading .cql files (CREATE IF NOT EXISTS is idempotent)
- Home feed aggregation: fan-out-on-read with Redis caching (implemented in 03-01)
- Post shard count: 2 shards for v1 (implemented in 03-01)

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 001 | Categorize frontend components into domain subfolders | 2026-03-03 | d51e48b | [001-categorize-frontend-components](./quick/001-categorize-frontend-components/) |

## Session Continuity

Last session: 2026-03-04T20:29:26.329Z
Stopped at: Phase 5 context gathered
Resume file: .planning/phases/05-search-notifications-media-full-stack/05-CONTEXT.md

---
*Last updated: 2026-03-05 — Phase 4 complete (4/4 plans, verified), ready for Phase 5*
