---
phase: 04-comments
plan: 02
subsystem: infra
tags: [docker-compose, envoy, scylladb, grpc-transcoding, routing]

# Dependency graph
requires:
  - phase: 04-comments
    provides: "comment-service binary, CommentService proto definition, ScyllaDB schema"
provides:
  - "ScyllaDB container in Docker Compose stack (scylladb/scylla:6.2, dev mode)"
  - "comment-service container wired to ScyllaDB, Redis DB 6, Kafka"
  - "Envoy routes for all 6 comment REST endpoints with correct ordering"
  - "comment-service cluster in Envoy with HTTP/2"
  - "CommentService registered in grpc_json_transcoder"
affects: [04-comments, 05-search-notifications-media, 07-deployment-observability]

# Tech tracking
tech-stack:
  added: [scylladb/scylla:6.2]
  patterns: [envoy-regex-before-prefix-routing, scylladb-dev-mode-container]

key-files:
  created: []
  modified:
    - docker-compose.yml
    - deploy/envoy/envoy.yaml

key-decisions:
  - "Comment regex route placed BEFORE /api/v1/posts prefix to prevent post-service catching comment requests"
  - "ScyllaDB dev mode with --smp 1 --memory 512M for minimal resource usage in development"
  - "60s start_period on ScyllaDB healthcheck to accommodate slow cold starts"

patterns-established:
  - "Regex-before-prefix route pattern: more specific regex routes must precede broader prefix routes in Envoy first-match"
  - "ScyllaDB container pattern: dev mode with constrained resources and generous healthcheck timing"

requirements-completed: [CMNT-01, CMNT-03]

# Metrics
duration: 2min
completed: 2026-03-04
---

# Phase 4 Plan 2: Comments Infrastructure Wiring Summary

**ScyllaDB + comment-service containers in Docker Compose, Envoy routes with regex-before-prefix ordering for all 6 comment endpoints, CommentService transcoder registration**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-04T19:12:40Z
- **Completed:** 2026-03-04T19:14:43Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- ScyllaDB 6.2 container with dev mode, 512M memory cap, and 60s start_period healthcheck added to Docker Compose
- comment-service container on port 50057 with ScyllaDB, Redis DB 6, and Kafka dependencies
- Envoy regex route for `/api/v1/posts/{post_id}/comments` placed BEFORE `/api/v1/posts` catch-all — prevents post-service from intercepting comment requests
- Envoy prefix route for `/api/v1/comments` handles CRUD and reply endpoints
- comment-service cluster with HTTP/2 protocol and CommentService in transcoder services list

## Task Commits

Each task was committed atomically:

1. **Task 1: ScyllaDB container, comment-service container in Docker Compose** - `ed52296` (feat)
2. **Task 2: Envoy comment routes, proto descriptor rebuild, cluster config** - `1e4f168` (feat)

## Files Created/Modified
- `docker-compose.yml` - Added scylladb container, comment-service container, scylla-data volume, envoy depends_on comment-service
- `deploy/envoy/envoy.yaml` - Comment regex + prefix routes before post catch-all, comment-service cluster, CommentService in transcoder

## Decisions Made
- Comment regex route `/api/v1/posts/[^/]+/comments.*` placed before `/api/v1/posts` prefix — Envoy first-match routing would otherwise send comment requests to post-service
- ScyllaDB dev mode (`--smp 1 --memory 512M --developer-mode 1`) keeps resource usage minimal for local development
- 60s start_period on ScyllaDB healthcheck — ScyllaDB cold start takes 30-60s in Docker, matching retry loop in comment-service (30 attempts * 2s)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Full infrastructure stack ready: ScyllaDB + comment-service + Envoy routing
- Ready for frontend comment components (04-03) which will consume comment API via Envoy
- All 6 comment REST endpoints routable through Envoy transcoder

---
*Phase: 04-comments*
*Completed: 2026-03-04*
