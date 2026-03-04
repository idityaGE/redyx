---
phase: 04-comments
plan: 04
subsystem: comments
tags: [scylladb, astro, svelte, envoy, docker, e2e, integration-test]

# Dependency graph
requires:
  - phase: 04-comments/04-01
    provides: Comment service backend with ScyllaDB store and 6 RPCs
  - phase: 04-comments/04-02
    provides: Docker/Envoy wiring for ScyllaDB and comment-service
  - phase: 04-comments/04-03
    provides: Frontend comment components (CommentSection, CommentCard, CommentSortBar, CommentForm)
provides:
  - CommentSection mounted in post detail page as Svelte island
  - Verified end-to-end comment lifecycle (create, reply, update, delete, list, sort, vote)
  - Fixed ScyllaDB keyspace initialization bug in comment-service startup
affects: [05-search-notifications-media]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "ScyllaDB two-phase connection: no-keyspace for migrations, then reconnect with keyspace"

key-files:
  created: []
  modified:
    - web/src/pages/post/[id].astro
    - cmd/comment/main.go

key-decisions:
  - "Split connectScyllaDB into two functions to fix keyspace creation race condition"

patterns-established:
  - "ScyllaDB migration-first startup: connect without keyspace → run CREATE KEYSPACE → reconnect with keyspace"

requirements-completed: [CMNT-01, CMNT-02, CMNT-03, CMNT-04, CMNT-05, CMNT-06]

# Metrics
duration: 24min
completed: 2026-03-04
---

# Phase 4 Plan 4: E2E Integration Summary

**CommentSection mounted in post detail page with all 14 API tests passing: create, reply (depth 2 and 3), get, update (isEdited), delete (soft-delete [deleted]), list with Best/New/Top sort, list replies, and vote on comment**

## Performance

- **Duration:** 24 min
- **Started:** 2026-03-04T19:19:13Z
- **Completed:** 2026-03-04T19:42:47Z
- **Tasks:** 1 auto + 1 checkpoint (described, not blocked)
- **Files modified:** 2

## Accomplishments
- CommentSection mounted below PostDetail in post/[id].astro as a separate Svelte island with client:load
- Fixed critical ScyllaDB keyspace initialization bug — migrations now run before keyspace connection
- All 14 API curl tests verified: comment CRUD, nested replies, sort orders, lazy-load replies, vote on comment
- Full Docker stack (12 services including ScyllaDB and comment-service) running and healthy

## Task Commits

Each task was committed atomically:

1. **Task 1: Mount CommentSection + API verification** - `9775578` (feat)

**Plan metadata:** (to be added below)

## Files Created/Modified
- `web/src/pages/post/[id].astro` - Added CommentSection import and mount with client:load
- `cmd/comment/main.go` - Fixed ScyllaDB connection: split into two-phase migration-first startup

## Decisions Made
- Split `connectScyllaDB` into `connectScyllaDBNoKeyspace` + `connectScyllaDBWithKeyspace` to fix race condition where keyspace didn't exist yet when the service tried to connect with it

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed ScyllaDB keyspace initialization ordering**
- **Found during:** Task 1 (Docker stack startup)
- **Issue:** `connectScyllaDB` tried to connect WITH keyspace before `RunMigrations` (which creates the keyspace) had a chance to run. The original code connected without keyspace, closed that session, then tried keyspace — failing because keyspace didn't exist yet. Migrations that CREATE the keyspace needed to run first.
- **Fix:** Split into `connectScyllaDBNoKeyspace` (for migrations) and `connectScyllaDBWithKeyspace` (after migrations). The main function now: (1) connects without keyspace, (2) runs migrations (creates keyspace + tables), (3) closes migration session, (4) reconnects with keyspace.
- **Files modified:** `cmd/comment/main.go`
- **Verification:** Comment service starts successfully, logs show "scylladb migrations applied" then "connected to scylladb" with keyspace
- **Committed in:** `9775578` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Essential fix — comment-service couldn't start without it. No scope creep.

## Issues Encountered
- Docker compose build timed out initially due to ScyllaDB image pull (240MB) — resolved by running build separately before `up -d`

## Checkpoint: Human Verification (Task 2)

**What was built:** Complete comment system with backend service (ScyllaDB store, materialized path, Wilson score sort, Kafka vote consumer), Docker/Envoy wiring (ScyllaDB container, comment-service, route config), and frontend components (CommentSection, CommentCard, CommentSortBar, CommentForm) mounted in post detail page.

**What the human should verify:**
1. Open http://localhost:4321 (Astro dev server) and log in
2. Navigate to a post — comment section should appear below the post
3. Create comment: Click [write comment] → type comment → submit → appears in tree
4. Reply: Click [reply] on comment → inline form → submit → nested reply appears
5. Nested reply: Reply to the reply → verify 3 levels of indentation
6. Vote on comment: Click upvote/downvote → score updates
7. Sort comments: Switch between Best, Top, New → comments reorder
8. Edit comment: Click [edit] → modify → [save] → "(edited)" appears
9. Delete comment: Click [delete] → confirm → shows [deleted] body, replies remain
10. Collapse/expand: Click [-] → subtree collapses → click [+] → expands
11. Deep threads: Verify [load N more replies] for depth > 3
12. Empty state: Post with no comments → "no comments yet" message

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Phase 4 complete — all 4 plans executed
- Core discussion loop (browse → read → vote → comment → reply) is fully working
- Ready for Phase 5: Search + Notifications + Media

## Self-Check: PASSED

All key files exist, commit verified, CommentSection mounted with client:load, ScyllaDB fix verified.

---
*Phase: 04-comments*
*Completed: 2026-03-04*
