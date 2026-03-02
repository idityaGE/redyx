---
phase: 02-auth-user-community
plan: 01
subsystem: auth, infra
tags: [jwt, grpc-interceptor, rate-limit, redis, lua, postgresql, migrations, docker]

# Dependency graph
requires:
  - phase: 01-foundation-frontend-shell
    provides: platform libraries (grpcserver, config, middleware, errors, redis), Docker Compose base
provides:
  - JWT auth gRPC interceptor with claims context injection
  - Redis token bucket rate limiter with tier-based limits
  - Extended config with auth/OAuth/rate-limit environment loading
  - Database migrations for auth, user_profiles, community schemas
  - Docker Compose multi-database init script
affects: [02-02, 02-03, 02-04, 02-05]

# Tech tracking
tech-stack:
  added: [golang-jwt/jwt/v5, x/crypto, x/oauth2, google/uuid]
  patterns: [gRPC unary interceptor, Redis Lua scripting, token bucket rate limiting, context-based claims propagation]

key-files:
  created:
    - internal/platform/auth/context.go
    - internal/platform/auth/interceptor.go
    - internal/platform/ratelimit/limiter.go
    - internal/platform/ratelimit/interceptor.go
    - deploy/docker/init-databases.sql
    - migrations/auth/001_users.up.sql
    - migrations/auth/001_users.down.sql
    - migrations/auth/002_refresh_tokens.up.sql
    - migrations/auth/002_refresh_tokens.down.sql
    - migrations/user/001_profiles.up.sql
    - migrations/user/001_profiles.down.sql
    - migrations/community/001_communities.up.sql
    - migrations/community/001_communities.down.sql
  modified:
    - go.mod
    - go.sum
    - internal/platform/config/config.go
    - docker-compose.yml

key-decisions:
  - "Fail-open rate limiting: Redis errors allow requests through to avoid blocking all traffic"
  - "Public methods still attempt optional JWT extraction for rate limit tier differentiation"
  - "Token bucket via Lua script for atomic Redis operations — no race conditions"

patterns-established:
  - "gRPC interceptor pattern: auth → rate-limit → error-mapping in middleware chain"
  - "Context-based claims propagation: WithClaims/ClaimsFromContext for downstream handlers"
  - "Database-per-service: init-databases.sql creates auth, user_profiles, community DBs on first startup"

requirements-completed: [AUTH-05, AUTH-08, RATE-01, RATE-02, RATE-03, RATE-04]

# Metrics
duration: 5min
completed: 2026-03-03
---

# Phase 2 Plan 1: Platform Infrastructure Summary

**JWT auth interceptor with claims context, Redis Lua token bucket rate limiter with 3 tiers, extended config, and database migrations for auth/user/community services**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-02T19:43:22Z
- **Completed:** 2026-03-02T19:48:28Z
- **Tasks:** 3
- **Files modified:** 17

## Accomplishments
- JWT validation gRPC interceptor with public method allow-list and Bearer token extraction from metadata
- Redis-backed token bucket rate limiter with tiered limits (anonymous: 10/min, authenticated: 100/min, trusted: 300/min) and per-action limits
- Extended platform Config with JWT, OAuth, OTP, and rate limit fields with env-based loading and dev defaults
- Complete database migration set: users, refresh_tokens, profiles, communities, community_members tables
- Docker Compose multi-database init script for auth, user_profiles, community databases

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Go dependencies and extend platform config** - `b06e01e` (feat)
2. **Task 2: Create auth interceptor and rate limit interceptor platform packages** - `9c99789` (feat)
3. **Task 3: Create database migrations, Docker init script, and update Docker Compose** - `fc3de86` (feat)

## Files Created/Modified
- `internal/platform/auth/context.go` - Claims struct, WithClaims/ClaimsFromContext/UserIDFromContext helpers
- `internal/platform/auth/interceptor.go` - JWTValidator (HS256), UnaryInterceptor with public method allow-list
- `internal/platform/ratelimit/limiter.go` - Redis Lua token bucket script, tiered/action limit configs
- `internal/platform/ratelimit/interceptor.go` - Rate limit gRPC interceptor with tier detection, retry-after metadata
- `internal/platform/config/config.go` - Extended with JWT, OAuth, OTP, rate limit fields + envDuration/envBool helpers
- `deploy/docker/init-databases.sql` - Creates auth, user_profiles, community databases with GRANT
- `docker-compose.yml` - Mounts init-databases.sql into postgres entrypoint
- `migrations/auth/001_users.up.sql` - Users table with email/username/password_hash/google_id, partial unique index
- `migrations/auth/002_refresh_tokens.up.sql` - Refresh tokens with SHA-256 hash, expiry, cascade delete
- `migrations/user/001_profiles.up.sql` - Profiles with bio length check, karma, soft delete
- `migrations/community/001_communities.up.sql` - Communities with name regex, JSONB rules, visibility; members with composite PK
- `go.mod` / `go.sum` - Added golang-jwt/jwt/v5, x/crypto, x/oauth2, google/uuid

## Decisions Made
- **Fail-open rate limiting:** Redis errors allow requests through rather than blocking all traffic — availability over strictness for rate limiting
- **Optional JWT on public methods:** Public endpoints still try to parse JWT for rate limit tier differentiation (anonymous vs authenticated)
- **Atomic Lua script:** Token bucket implemented as single Lua script to avoid race conditions in concurrent Redis operations
- **envBool/envDuration helpers:** Added to config package for clean boolean and duration environment variable parsing

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Auth/rate-limit interceptors ready for all Phase 2 services to import
- Database schemas defined for auth (plan 02), user (plan 03), community (plan 04) services
- Docker Compose will create all 3 databases on fresh `docker compose up`
- Ready for 02-02-PLAN.md (Auth gRPC service implementation)

## Self-Check: PASSED

All 13 created files verified on disk. All 3 task commits verified in git log.

---
*Phase: 02-auth-user-community*
*Completed: 2026-03-03*
