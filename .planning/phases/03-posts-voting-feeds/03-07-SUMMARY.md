---
phase: 03-posts-voting-feeds
plan: 07
subsystem: testing
tags: [e2e, curl, integration, verification, kafka, voting, feeds, sharding]

# Dependency graph
requires:
  - phase: 03-posts-voting-feeds
    provides: All prior plans (01-06) — post service, vote service, Docker/Envoy wiring, frontend components, pages, saved posts
provides:
  - Verified end-to-end post lifecycle (create → feed → vote → detail → edit → delete)
  - Verified vote pipeline (Redis → Kafka → ScoreConsumer → PostgreSQL)
  - Verified cross-shard home feed aggregation
  - Public anonymous feed for non-authenticated users
  - SPA navigation via Astro ClientRouter (no sidebar/header remount)
  - Live vote score overlay from Redis on all feed endpoints
affects: [04-comments]

# Tech tracking
tech-stack:
  added: [astro-client-router, astro-view-transitions]
  patterns: [redis-pipeline-batch-score-overlay, transition-persist-spa, public-feed-anonymous-access]

key-files:
  created: []
  modified:
    - internal/post/server.go
    - internal/post/cache.go
    - internal/platform/auth/interceptor.go
    - internal/vote/kafka.go
    - cmd/post/main.go
    - web/src/layouts/BaseLayout.astro
    - web/src/components/Sidebar.svelte
    - web/src/components/FeedList.svelte
    - web/src/components/HomeFeed.svelte
    - web/src/components/CommunityFeed.svelte
    - web/src/components/PostDetail.svelte
    - web/src/components/PostSubmitForm.svelte
    - web/src/components/SavedPosts.svelte
    - web/src/components/CommunityDetail.svelte
    - web/src/lib/auth.ts

key-decisions:
  - "ListHomeFeed serves public feed for anonymous users (queries all shards, no community filter)"
  - "ListHomeFeed added to publicMethods in auth interceptor for anonymous access"
  - "Live vote scores overlaid from Redis vote-service (DB 5) on all feed endpoints via pipeline batch"
  - "Astro ClientRouter + transition:persist prevents sidebar/header remount on SPA navigation"
  - "Hardcoded communities removed from sidebar for anonymous users"
  - "Auth race condition fixed with whenReady() promise pattern"
  - "Svelte 5 $effect infinite loop fixed with untrack() for loadPage() calls"
  - "Kafka producer uses context.Background() instead of request context for fire-and-forget"
  - "ScoreConsumer instantiated and started in cmd/post/main.go"

patterns-established:
  - "whenReady() pattern: components await auth initialization before API calls or auth checks"
  - "Reactive auth pattern: $state + subscribe() for auth-dependent template rendering"
  - "untrack() pattern: wrap $state-mutating calls in $effect to prevent infinite loops"
  - "Redis pipeline batch score overlay: GetVoteScores fetches live scores for N posts in one round-trip"
  - "transition:persist for cross-page persistent components in Astro MPA"

requirements-completed: [POST-04, POST-06, POST-07, VOTE-01]

# Metrics
duration: ~120min
completed: 2026-03-03
---

# Phase 3 Plan 7: E2E Integration Verification Summary

**21/21 API curl tests passed, then extensive debugging of auth race conditions, Kafka pipeline, vote score caching, SPA navigation, and anonymous public feed**

## Performance

- **Duration:** ~120 min (across multiple debug sessions)
- **Started:** 2026-03-03T15:30:00Z
- **Completed:** 2026-03-03T17:30:00Z
- **Tasks:** 2 (API tests + human verification with bug fixes)
- **Files modified:** 15

## Accomplishments
- All 21 API curl tests passed: post CRUD, all sort orders, home feed, vote lifecycle, save/unsave, anonymous masking, delete
- Fixed auth race condition with `whenReady()` promise — prevents 401s and competing token refreshes on page load
- Fixed Kafka producer context cancellation — vote events now reliably reach Kafka
- Fixed ScoreConsumer never starting — vote_score now updates in PostgreSQL via Kafka pipeline
- Fixed FeedList infinite loop — `untrack()` prevents $effect re-trigger on $state mutations
- Fixed non-reactive auth in 5 components — subscribe pattern for auth-dependent UI
- Fixed sidebar showing only owned communities — now checks `isMember` per community
- Added SPA navigation via Astro ClientRouter — sidebar/header persist across page changes
- Added public anonymous feed — non-authenticated users see all posts on home page
- Added live vote score overlay — all feed endpoints return real-time scores from Redis

## Task Commits

Each task was committed atomically:

1. **Task 1: API curl tests + initial fixes** - `4d26de2` (fix)
2. **Task 2: Auth race condition, vote pipeline, SPA navigation, public feed** - `bd84b1e` (fix)

## Files Created/Modified
- `internal/post/server.go` — ListHomeFeed anonymous support, live score overlay on all feed endpoints
- `internal/post/cache.go` — GetVoteScores batch Redis pipeline method
- `internal/platform/auth/interceptor.go` — ListHomeFeed added to publicMethods
- `internal/vote/kafka.go` — context.Background() for fire-and-forget Kafka produce
- `cmd/post/main.go` — ScoreConsumer instantiation + goroutine startup
- `web/src/layouts/BaseLayout.astro` — ClientRouter + transition:persist on Header/Sidebar/footer
- `web/src/components/Sidebar.svelte` — Membership-based community list, removed hardcoded communities
- `web/src/components/FeedList.svelte` — whenReady(), untrack() in $effect
- `web/src/components/HomeFeed.svelte` — Reactive auth via subscribe()
- `web/src/components/CommunityFeed.svelte` — Reactive auth via subscribe()
- `web/src/components/PostDetail.svelte` — Reactive auth, whenReady() for fetch
- `web/src/components/PostSubmitForm.svelte` — whenReady() before auth guard
- `web/src/components/SavedPosts.svelte` — whenReady() auth guard
- `web/src/components/CommunityDetail.svelte` — whenReady(), breadcrumb backlink
- `web/src/lib/auth.ts` — whenReady() promise, idempotent initialize()

## Decisions Made
- **Public feed for anonymous users:** ListHomeFeed queries all shards with no community filter when unauthenticated, matching Reddit's behavior
- **Redis pipeline for batch scores:** Single round-trip to fetch live vote scores for all posts in a feed page
- **Astro ClientRouter:** Converts MPA to SPA-like navigation, persisting Header/Sidebar across page changes
- **whenReady() promise:** Single shared promise that resolves when auth initialize() completes — all components await it before API calls

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Kafka producer context canceled**
- **Found during:** Task 1 (vote pipeline testing)
- **Issue:** Vote RPC context canceled before async Kafka produce completed
- **Fix:** Use `context.Background()` for fire-and-forget produce calls
- **Files modified:** `internal/vote/kafka.go`

**2. [Rule 1 - Bug] ScoreConsumer never started**
- **Found during:** Task 1 (vote_score verification)
- **Issue:** Consumer defined but never instantiated in cmd/post/main.go
- **Fix:** Added ScoreConsumer creation + goroutine startup
- **Files modified:** `cmd/post/main.go`

**3. [Rule 1 - Bug] Auth race condition on page load**
- **Found during:** Task 2 (human verification)
- **Issue:** Components making API calls before initialize() completed → 401s, competing refresh calls
- **Fix:** Added whenReady() promise to auth.ts, all API-calling components await it
- **Files modified:** `web/src/lib/auth.ts`, 6 component files

**4. [Rule 1 - Bug] FeedList $effect infinite loop**
- **Found during:** Task 2 (human verification)
- **Issue:** $effect tracked loading/hasMore $state vars via loadPage() → infinite re-trigger
- **Fix:** Wrapped loadPage() call in untrack() from 'svelte'
- **Files modified:** `web/src/components/FeedList.svelte`

**5. [Rule 1 - Bug] Non-reactive auth in Svelte templates**
- **Found during:** Task 2 (human verification)
- **Issue:** isAuthenticated() evaluates once at render time, never updates
- **Fix:** Added $state + subscribe() pattern in 5 components
- **Files modified:** HomeFeed, CommunityFeed, PostDetail, PostSubmitForm, SavedPosts

**6. [Rule 2 - Missing Critical] No public feed for anonymous users**
- **Found during:** Task 2 (human verification)
- **Issue:** ListHomeFeed returned 401 for unauthenticated users — empty home page
- **Fix:** Made ListHomeFeed serve all-posts public feed when claims == nil
- **Files modified:** `internal/post/server.go`, `internal/platform/auth/interceptor.go`

**7. [Rule 1 - Bug] Feed cache returns stale vote scores**
- **Found during:** Task 2 (human verification)
- **Issue:** 2-min Redis feed cache returned vote_score=0 after voting
- **Fix:** Overlay live scores from vote-service Redis via pipeline batch on all feed endpoints
- **Files modified:** `internal/post/cache.go`, `internal/post/server.go`

**8. [Rule 1 - Bug] Sidebar/Header remount on every navigation**
- **Found during:** Task 2 (human verification)
- **Issue:** Astro MPA full-page reloads caused re-fetch of auth + communities on every click
- **Fix:** Added ClientRouter + transition:persist to keep components mounted
- **Files modified:** `web/src/layouts/BaseLayout.astro`

---

**Total deviations:** 8 auto-fixed (6 bugs, 1 missing critical, 1 blocking)
**Impact on plan:** All fixes essential for correct operation. No scope creep — all issues were direct consequences of Phase 3 feature integration.

## Issues Encountered
- Kafka producer context cancellation was subtle — only visible in server logs as "context canceled" on produce
- Svelte 5 $effect dependency tracking is implicit — calling any function that reads $state creates a dependency
- Auth initialization race required a new shared primitive (whenReady promise) that all components must adopt

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All Phase 3 requirements verified: POST-01 through POST-10, VOTE-01 through VOTE-05
- Vote pipeline fully operational: vote → Redis → Kafka → ScoreConsumer → PostgreSQL
- Frontend SPA navigation working with persistent components
- Anonymous public feed serving all posts
- Ready for Phase 4: Comments (Full Stack)

---
*Phase: 03-posts-voting-feeds*
*Completed: 2026-03-03*
