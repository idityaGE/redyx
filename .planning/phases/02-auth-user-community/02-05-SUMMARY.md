---
phase: 02-auth-user-community
plan: 05
subsystem: infra
tags: [docker-compose, envoy, grpc-transcoding, routing, cors]

# Dependency graph
requires:
  - phase: 02-auth-user-community
    provides: "auth, user, community gRPC services (plans 02-04)"
  - phase: 01-foundation-frontend-shell
    provides: "Dockerfile, Envoy base config, skeleton-service pattern"
provides:
  - "Docker Compose definitions for auth-service, user-service, community-service"
  - "Envoy route rules for /api/v1/auth/*, /api/v1/users/*, /api/v1/communities/*"
  - "gRPC transcoder registration for AuthService, UserService, CommunityService, RateLimitService"
  - "HTTP/2 upstream clusters for all Phase 2 services"
affects: [02-06, 02-07, 02-08, 02-09, 02-10]

# Tech tracking
tech-stack:
  added: []
  patterns: ["service-per-port Docker Compose pattern", "Envoy first-match route ordering", "shared JWT_SECRET across services"]

key-files:
  created: []
  modified:
    - docker-compose.yml
    - deploy/envoy/envoy.yaml

key-decisions:
  - "Redis DB isolation: auth=1, user=2, community=3 (skeleton=0)"
  - "Envoy specific routes before catch-all for first-match routing"
  - "CORS expose-headers includes retry-after for rate-limit visibility"

patterns-established:
  - "Service naming: {name}-service with matching cluster and Docker Compose service name"
  - "Port allocation: 50051 (skeleton), 50052 (auth), 50053 (user), 50054 (community)"

requirements-completed: [AUTH-04, RATE-04]

# Metrics
duration: 2min
completed: 2026-03-02
---

# Phase 2 Plan 5: Docker Compose + Envoy Summary

**Three Phase 2 services wired into Docker Compose with Envoy prefix-routing, gRPC transcoder registration, and CORS retry-after header exposure**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-02T20:01:35Z
- **Completed:** 2026-03-02T20:03:13Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Docker Compose defines auth-service (50052), user-service (50053), community-service (50054) with isolated databases and Redis DBs
- Envoy routes /api/v1/auth/*, /api/v1/users/*, /api/v1/communities/* to their respective services before the catch-all
- All five proto services registered in gRPC-JSON transcoder (Health, Auth, User, Community, RateLimit)
- CORS headers expose retry-after so frontends can read 429 rate-limit responses

## Task Commits

Each task was committed atomically:

1. **Task 1: Add auth, user, community services to Docker Compose** - `20437b1` (feat)
2. **Task 2: Update Envoy routes and transcoder for Phase 2 services** - `eb73042` (feat)

## Files Created/Modified
- `docker-compose.yml` - Added auth-service, user-service, community-service definitions with DB URLs, Redis DBs, shared JWT secret, and envoy depends_on
- `deploy/envoy/envoy.yaml` - Added prefix routes, transcoder service registrations, HTTP/2 upstream clusters, and CORS expose-headers

## Decisions Made
- Redis DB isolation per service: auth=DB1, user=DB2, community=DB3 (skeleton stays DB0) — avoids key collisions
- Envoy specific routes placed before catch-all — Envoy uses first-match, so /api/v1/auth/ must precede /api/v1/
- Added access-control-expose-headers for retry-after — enables frontend to read Retry-After on 429 responses

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Infrastructure fully wired for all Phase 2 services
- Ready for 02-06 (frontend foundation: API client, auth store, Header/Sidebar)
- All services accessible via REST through Envoy gateway once built

---
*Phase: 02-auth-user-community*
*Completed: 2026-03-02*
