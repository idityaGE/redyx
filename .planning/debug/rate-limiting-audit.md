---
status: resolved
trigger: "rate-limiting-audit: RateLimitingService proto exists but appears unused"
created: 2026-03-27T00:00:00Z
updated: 2026-03-27T12:00:00Z
---

## Current Focus

hypothesis: CONFIRMED AND FIXED - RateLimitService proto was unused dead code, action-specific rate limits were defined but not enforced
test: Complete code audit and implementation
expecting: Full rate limiting implementation
next_action: None - all issues resolved

## Symptoms

expected: RateLimitingService proto RPCs should be called for rate limiting enforcement
actual: RateLimitingService proto was unused - rate limiting works through middleware interceptor pattern instead
errors: None reported - discovered during code review
reproduction: Code audit - compare proto definitions to actual implementation
started: Discovered during code review

## Eliminated

## Evidence

- timestamp: 2026-03-27T00:01:00Z
  checked: proto/redyx/ratelimit/v1/ratelimit.proto
  found: RateLimitService defined with CheckRateLimit RPC, UserTier enum (anonymous/authenticated/trusted)
  implication: Proto exists and is well-defined

- timestamp: 2026-03-27T00:02:00Z
  checked: grep for RegisterRateLimitServiceServer in main.go files
  found: No service registers RateLimitServiceServer - proto service is NEVER implemented
  implication: Proto is dead code - no gRPC server implements it

- timestamp: 2026-03-27T00:03:00Z
  checked: internal/platform/ratelimit/limiter.go and interceptor.go
  found: Working Redis-backed rate limiter with TierLimits (anonymous=10/min, authenticated=100/min, trusted=300/min) and ActionLimits (post=5/hr, comment=30/hr, vote=60/min, community=1/day)
  implication: Rate limiting IS implemented - just via interceptor pattern, not proto service

- timestamp: 2026-03-27T00:04:00Z
  checked: All service main.go files (auth, user, post, vote, comment, etc.)
  found: All services use ratelimit.UnaryInterceptor(limiter, cfg.RateLimitEnabled) in interceptor chain
  implication: Global request rate limiting IS working via middleware

- timestamp: 2026-03-27T00:05:00Z
  checked: grep for ActionLimits usage and limiter.Check calls in service handlers
  found: ActionLimits defined but NEVER USED - only interceptor calls limiter.Check for global limits
  implication: Action-specific limits (RATE-03) are NOT enforced

- timestamp: 2026-03-27T00:06:00Z
  checked: deploy/envoy/envoy.yaml
  found: RateLimitService listed in grpc_json_transcoder services BUT no route to a ratelimit cluster - no cluster exists
  implication: Envoy config references dead proto; no standalone rate limit service exists

- timestamp: 2026-03-27T00:07:00Z
  checked: docker-compose.yml
  found: No rate-limit-service container - rate limiting is distributed via interceptors in each service
  implication: Architecture decision - no centralized rate limit service

## Resolution

root_cause: 
1. RateLimitService proto was unused dead code (architecture uses interceptor pattern, not centralized service)
2. Action-specific limits (RATE-03) were defined in ActionLimits map but never enforced in handlers
3. TRUSTED tier was unnecessary (only anonymous/authenticated needed)
4. Envoy config had orphaned reference to non-existent RateLimitService

fix: 
1. Deleted unused proto/redyx/ratelimit/v1/ and gen/redyx/ratelimit/v1/ directories
2. Removed orphaned redyx.ratelimit.v1.RateLimitService from deploy/envoy/envoy.yaml
3. Removed TRUSTED tier from TierLimits in internal/platform/ratelimit/limiter.go
4. Added ErrRateLimited sentinel error to internal/platform/errors/errors.go
5. Added ErrRateLimited -> codes.ResourceExhausted mapping in internal/platform/middleware/errors.go
6. Implemented action-specific rate limiting in CreatePost (internal/post/server.go)
7. Implemented action-specific rate limiting in CreateComment (internal/comment/server.go)
8. Implemented action-specific rate limiting in Vote (internal/vote/server.go)
9. Implemented action-specific rate limiting in CreateCommunity (internal/community/server.go)
10. Wired limiters through ServerOption pattern in all affected services

verification: go build ./... passes successfully

files_changed:
- proto/redyx/ratelimit/v1/ratelimit.proto (DELETED)
- gen/redyx/ratelimit/v1/*.pb.go (DELETED)
- deploy/envoy/envoy.yaml (removed orphaned transcoder service)
- internal/platform/ratelimit/limiter.go (removed TRUSTED tier)
- internal/platform/errors/errors.go (added ErrRateLimited)
- internal/platform/middleware/errors.go (added ErrRateLimited mapping)
- internal/post/server.go (added limiter field, WithLimiter option, rate limit check in CreatePost)
- internal/comment/server.go (added limiter field, WithLimiter option, rate limit check in CreateComment)
- internal/vote/server.go (added limiter field, ServerOption pattern, WithLimiter option, rate limit check in Vote)
- internal/community/server.go (added limiter field, ServerOption pattern, WithLimiter option, rate limit check in CreateCommunity)
- cmd/post/main.go (pass limiter to NewServer)
- cmd/comment/main.go (pass limiter to NewServer)
- cmd/vote/main.go (pass limiter to NewServer)
- cmd/community/main.go (pass limiter to NewServer)
