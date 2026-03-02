---
phase: 02-auth-user-community
plan: 04
subsystem: community
tags: [grpc, postgresql, redis, caching, pagination, community, membership, moderation]

# Dependency graph
requires:
  - phase: 02-auth-user-community
    provides: platform auth interceptor, rate limiter, community DB migration, config
provides:
  - CommunityServiceServer gRPC implementation with all 9 RPCs
  - Redis community metadata cache with 1hr TTL
  - Community service binary (cmd/community)
  - Community members migration with username and owner role
affects: [02-05, 03-content-posts]

# Tech tracking
tech-stack:
  added: []
  patterns: [cache-aside pattern with Redis, denormalized username in community_members, cursor pagination with member_count DESC sort]

key-files:
  created:
    - internal/community/server.go
    - internal/community/cache.go
    - cmd/community/main.go
    - migrations/community/002_add_username_and_owner_role.up.sql
    - migrations/community/002_add_username_and_owner_role.down.sql
  modified: []

key-decisions:
  - "Cache-aside pattern: check Redis first, fallback to DB, populate cache on miss"
  - "Denormalized username in community_members to avoid cross-service calls for ListMembers"
  - "Owner role in community_members CHECK constraint (replacing 'admin' from original migration)"
  - "COALESCE(NULLIF()) pattern for partial updates in UpdateCommunity"
  - "Idempotent JoinCommunity using ON CONFLICT DO NOTHING"
  - "Member count synced from actual COUNT(*) subquery to prevent drift"

patterns-established:
  - "Cache-aside with Redis: Get → miss → DB query → Set. Invalidate on mutation."
  - "Denormalized usernames: stored at join time from auth claims, avoids cross-service RPC"
  - "Owner-only operations: AssignModerator/RevokeModerator check caller role == 'owner'"

requirements-completed: [COMM-01, COMM-02, COMM-03, COMM-04, COMM-05, COMM-06, COMM-07]

# Metrics
duration: 5min
completed: 2026-03-03
---

# Phase 2 Plan 4: Community Service Summary

**Complete CommunityService gRPC server with 9 RPCs (CRUD, membership, moderation), Redis cache-aside caching, cursor pagination, and owner/moderator role authorization**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-02T19:54:06Z
- **Completed:** 2026-03-02T19:58:51Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- All 9 CommunityService RPCs implemented: CreateCommunity, GetCommunity, UpdateCommunity, ListCommunities, JoinCommunity, LeaveCommunity, ListMembers, AssignModerator, RevokeModerator
- Redis cache-aside pattern for community metadata with 1hr TTL and automatic invalidation on mutations
- Owner/moderator authorization: only owners can assign/revoke moderators, owners cannot leave
- Community service entry point with full middleware chain (Recovery → Logging → RateLimit → Auth → ErrorMapping)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Redis community cache and implement CommunityServiceServer** - `b349064` (feat)
2. **Task 2: Create community service entry point** - `76eb6aa` (feat)

## Files Created/Modified
- `internal/community/cache.go` - Redis community metadata cache (Get/Set/Invalidate with 1hr TTL)
- `internal/community/server.go` - All 9 CommunityService RPCs with DB queries, validation, auth checks
- `cmd/community/main.go` - Service entry point wiring DB, Redis, auth, rate limiter, gRPC server
- `migrations/community/002_add_username_and_owner_role.up.sql` - Adds username column, changes role CHECK to include 'owner'
- `migrations/community/002_add_username_and_owner_role.down.sql` - Reverts username and role changes

## Decisions Made
- **Cache-aside pattern:** Check Redis cache first, fall back to PostgreSQL on miss, cache result. Invalidate on UpdateCommunity, LeaveCommunity, AssignModerator, RevokeModerator.
- **Denormalized username:** Stored in community_members at join/create time from auth claims to avoid cross-service calls for ListMembers.
- **Owner role replaces admin:** Migration 002 updates CHECK constraint from ('member', 'moderator', 'admin') to ('member', 'moderator', 'owner') for clarity.
- **Idempotent joins:** ON CONFLICT DO NOTHING prevents duplicate membership errors.
- **Member count from COUNT(*):** Uses subquery counting actual members instead of increment/decrement to prevent drift.
- **COALESCE(NULLIF()) for partial updates:** Empty strings in UpdateCommunity request are treated as "keep existing value".

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added username column and owner role migration**
- **Found during:** Task 1
- **Issue:** Original migration (001) did not include `username` column on community_members and used 'admin' role instead of 'owner'. Plan explicitly noted to check and add if missing.
- **Fix:** Created migration 002 adding username column and updating role CHECK constraint to include 'owner'.
- **Files modified:** migrations/community/002_add_username_and_owner_role.up.sql, migrations/community/002_add_username_and_owner_role.down.sql
- **Verification:** go build succeeds, migration SQL is valid
- **Committed in:** b349064 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Migration addition was explicitly anticipated in the plan. No scope creep.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Community service fully implements COMM-01 through COMM-07
- Ready for content/post service (Phase 3) to reference community_id for post creation
- Community membership checks available for downstream services via community_members table

## Self-Check: PASSED

All 5 created files verified on disk. Both task commits verified in git log.

---
*Phase: 02-auth-user-community*
*Completed: 2026-03-03*
