---
phase: 02-auth-user-community
plan: 02
subsystem: auth
tags: [argon2id, jwt, otp, google-oauth, grpc, redis, postgresql, password-reset]

# Dependency graph
requires:
  - phase: 02-auth-user-community
    provides: JWT interceptor, rate limiter, config, database migrations, Docker Compose
provides:
  - AuthServiceServer gRPC implementation with 7 RPCs
  - Argon2id password hashing (RFC 9106 params)
  - JWT access token creation (HS256, 15min TTL)
  - OTP generation with Redis storage and constant-time verification
  - Google OAuth code exchange with user info retrieval
  - Dev email stub (log-based OTP/reset token delivery)
  - Auth service binary (cmd/auth)
affects: [02-03, 02-04, 02-05, 02-06]

# Tech tracking
tech-stack:
  added: [argon2id, crypto/subtle, crypto/sha256, google/uuid, oauth2/google]
  patterns: [gRPC service server implementation, refresh token rotation with SHA-256 hashing, two-step password reset via Redis, create-on-first-access cross-service pattern]

key-files:
  created:
    - internal/auth/hasher.go
    - internal/auth/jwt.go
    - internal/auth/otp.go
    - internal/auth/oauth.go
    - internal/auth/email.go
    - internal/auth/server.go
    - cmd/auth/main.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Auth-service creates auth user record only; user-service handles profile creation (create-on-first-access pattern)"
  - "Refresh tokens stored as SHA-256 hash in PostgreSQL, rotated on every refresh"
  - "Password reset uses Redis-stored tokens with 1hr TTL, prevents email enumeration"
  - "Google OAuth returns is_new_user=true when username not yet chosen, requiring second call"
  - "Dev mode uses LogSender to log OTP codes and reset tokens to stdout instead of real email"

patterns-established:
  - "gRPC service server pattern: embed UnimplementedServer, inject deps via NewServer constructor"
  - "Refresh token rotation: UUID → SHA-256 hash → PostgreSQL lookup → delete old → issue new"
  - "Two-step password reset: step 1 initiates (Redis token), step 2 completes (verify + update)"
  - "OAuth registration flow: code exchange → user lookup → new user needs username → second call with username"

requirements-completed: [AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06, AUTH-07, AUTH-08]

# Metrics
duration: 5min
completed: 2026-03-03
---

# Phase 2 Plan 2: Auth Service Summary

**gRPC auth service with 7 RPCs: Register (argon2id+OTP), Login (JWT+refresh rotation), VerifyOTP, RefreshToken, Logout, ResetPassword (2-step Redis flow), GoogleOAuth (code exchange with is_new_user)**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-02T19:53:52Z
- **Completed:** 2026-03-02T19:58:35Z
- **Tasks:** 2
- **Files modified:** 9

## Accomplishments
- Complete auth service implementing all 7 AuthService RPCs with full validation, security, and error handling
- Argon2id password hashing with RFC 9106 recommended parameters and constant-time verification
- JWT access token creation (HS256, configurable TTL) with refresh token rotation via SHA-256 hashed PostgreSQL storage
- OTP verification with crypto/rand generation, Redis TTL storage, and constant-time comparison
- Google OAuth integration with graceful disabled-mode (nil manager) and two-phase registration
- Auth service entry point with migration runner, full interceptor chain, and complete dependency wiring

## Task Commits

Each task was committed atomically:

1. **Task 1: Create auth service helper modules** - `f510649` (feat)
2. **Task 2: Implement AuthServiceServer and auth service entry point** - `77cd42d` (feat)

## Files Created/Modified
- `internal/auth/hasher.go` - Argon2id password hashing/verification with RFC 9106 params
- `internal/auth/jwt.go` - JWTManager for HS256 access token creation
- `internal/auth/otp.go` - OTPManager for 6-digit codes with Redis storage and constant-time compare
- `internal/auth/oauth.go` - OAuthManager for Google OAuth code exchange and userinfo retrieval
- `internal/auth/email.go` - EmailSender interface + LogSender dev stub
- `internal/auth/server.go` - AuthServiceServer implementing all 7 RPCs
- `cmd/auth/main.go` - Service entry point with migration runner and interceptor chain
- `go.mod` / `go.sum` - Added cloud.google.com/go/compute/metadata (oauth2/google transitive dep)

## Decisions Made
- **Create-on-first-access for profiles:** Auth service only creates auth user records; user-service handles profile creation to avoid tight coupling between services
- **SHA-256 refresh token hashing:** Raw UUID tokens sent to client, SHA-256 hash stored in DB — prevents token theft via DB compromise
- **Anti-enumeration on password reset:** Returns success even for non-existent emails to prevent email existence discovery
- **Two-phase Google OAuth registration:** Returns is_new_user=true without tokens when username not provided, requiring a second call with chosen username

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Auth service binary compiles and is ready for integration testing
- All 7 RPCs implement the proto contract with proper error handling
- JWT tokens compatible with the platform auth interceptor (same HS256 secret, same claims structure)
- Ready for 02-03-PLAN.md (User service) which depends on auth user creation

## Self-Check: PASSED

All 7 created files verified on disk. All 2 task commits verified in git log.

---
*Phase: 02-auth-user-community*
*Completed: 2026-03-03*
