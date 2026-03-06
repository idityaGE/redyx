---
phase: 06-moderation-spam-full-stack
plan: 04
subsystem: infra
tags: [docker-compose, envoy, grpc, auth-interceptor, moderation, spam]

# Dependency graph
requires:
  - phase: 06-01
    provides: moderation-service gRPC implementation and proto definitions
  - phase: 06-02
    provides: spam-service gRPC implementation and proto definitions
provides:
  - moderation-service and spam-service Docker Compose container definitions
  - Envoy routing for moderation and spam REST API endpoints
  - Proto descriptor with ModerationService and SpamService
  - Auth interceptor public methods for service-to-service calls
  - moderation database auto-creation via init-databases.sql
affects: [06-05, 06-06, 06-07]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Regex route before catch-all for sub-resource routing (moderation under communities)"
    - "Service-to-service RPCs marked as public methods in auth interceptor"

key-files:
  created: []
  modified:
    - docker-compose.yml
    - deploy/docker/init-databases.sql
    - deploy/envoy/envoy.yaml
    - internal/platform/auth/interceptor.go

key-decisions:
  - "Moderation regex route placed BEFORE community catch-all to prevent community-service catching moderation requests"
  - "CheckBan, SubmitReport, CheckContent, ReportSpam marked as public methods since they are service-to-service calls without JWT"
  - "post-service and comment-service given both SPAM_SERVICE_ADDR and MODERATION_SERVICE_ADDR for future integration"

patterns-established:
  - "Service-to-service RPCs added to publicMethods map for direct gRPC calls bypassing Envoy auth"

requirements-completed: [MOD-01, MOD-04, MOD-05, MOD-06, SPAM-04]

# Metrics
duration: 5min
completed: 2026-03-06
---

# Phase 6 Plan 4: Infrastructure Wiring Summary

**Docker Compose, Envoy routing, auth interceptor, and init-databases wired for moderation-service (50061) and spam-service (50062)**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-06T10:34:11Z
- **Completed:** 2026-03-06T10:39:56Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Moderation and spam service containers defined in Docker Compose with correct dependencies and environment variables
- Envoy routes moderation requests via regex BEFORE community catch-all (Pitfall 1 prevention)
- Auth interceptor updated with 4 public methods for service-to-service calls (CheckBan, SubmitReport, CheckContent, ReportSpam)
- Post-service and comment-service updated with SPAM_SERVICE_ADDR and MODERATION_SERVICE_ADDR

## Task Commits

Each task was committed atomically:

1. **Task 1: Docker Compose + init-databases for moderation and spam services** - `e7f44a9` (feat)
2. **Task 2: Envoy routing + proto descriptor + auth interceptor updates** - `ba2c6e0` (feat)

## Files Created/Modified
- `docker-compose.yml` - Added moderation-service and spam-service containers, updated post/comment/envoy service deps
- `deploy/docker/init-databases.sql` - Added moderation database creation
- `deploy/envoy/envoy.yaml` - Added moderation/spam routes, transcoder services, and cluster definitions
- `internal/platform/auth/interceptor.go` - Added public methods for service-to-service moderation/spam RPCs

## Decisions Made
- Moderation regex route `/api/v1/communities/[^/]+/moderation.*` placed AFTER community-posts regex but BEFORE community catch-all — first-match routing prevents community-service from swallowing moderation requests
- CheckBan, SubmitReport, CheckContent, ReportSpam added to publicMethods — these are service-to-service calls that bypass Envoy and don't carry JWT tokens; SubmitReport from frontend goes through Envoy with auth, but the handler uses optional claims extraction
- Spam-service has no PostgreSQL — Redis-only design per 06-02 architecture

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Infrastructure wiring complete for both moderation and spam services
- Ready for Plan 06-05 (frontend moderation UI) and beyond
- All services can be started with `docker compose up`

## Self-Check: PASSED

All files verified on disk, all commits found in git log, all content markers confirmed.

---
*Phase: 06-moderation-spam-full-stack*
*Completed: 2026-03-06*
