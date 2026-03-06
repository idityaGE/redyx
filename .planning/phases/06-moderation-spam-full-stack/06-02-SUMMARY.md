---
phase: 06-moderation-spam-full-stack
plan: 02
subsystem: spam
tags: [grpc, redis, kafka, sha256, blocklist, dedup, spam-detection]

# Dependency graph
requires:
  - phase: 01-foundation-frontend-shell
    provides: "Platform libs (grpcserver, config, middleware, redis)"
  - phase: 03-posts-voting-feeds
    provides: "PostEvent proto and Kafka topic (redyx.posts.v1)"
provides:
  - "SpamServiceServer with CheckContent and ReportSpam RPCs"
  - "In-memory keyword + URL blocklist from JSON seed"
  - "Redis-based SHA-256 content dedup with 24h TTL"
  - "Kafka BehaviorConsumer for rapid posting and link spam detection"
  - "Spam service bootstrap (cmd/spam/main.go)"
affects: [06-moderation-spam-full-stack, post-service, comment-service]

# Tech tracking
tech-stack:
  added: [miniredis/v2 (test)]
  patterns: [redis-sliding-window, kafka-consumer-group, json-seed-data, content-hash-dedup]

key-files:
  created:
    - cmd/spam/main.go
    - internal/spam/blocklist.go
    - internal/spam/dedup.go
    - internal/spam/server.go
    - internal/spam/consumer.go
    - internal/spam/spam_test.go
    - internal/spam/data/blocklist.json
  modified: []

key-decisions:
  - "Redis DB 11 for spam service — follows per-service DB isolation pattern"
  - "Content normalization (lowercase, trim, collapse whitespace) before SHA-256 hashing"
  - "Vague reasons in CheckContentResponse (blocked_content/blocked_url) — never leak specific matched keywords"
  - "BehaviorConsumer logs flags when moderation SubmitReport RPC not yet available (Plan 06-01 dependency)"
  - "Behavior topic (redyx.behavior.v1) created for future extensibility"

patterns-established:
  - "Redis sliding window: INCR + EXPIRE for rate-based behavior detection"
  - "JSON seed data for blocklist — loaded at startup, not from database"
  - "Content hash dedup: SHA-256 of normalized text with Redis SET NX + TTL"

requirements-completed: [SPAM-01, SPAM-02, SPAM-03, SPAM-04]

# Metrics
duration: 8min
completed: 2026-03-06
---

# Phase 6 Plan 2: Spam Detection Service Summary

**Spam gRPC service with keyword/URL blocklist, SHA-256 content dedup via Redis, and Kafka behavior consumer for rapid posting and link spam detection**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-06T10:14:27Z
- **Completed:** 2026-03-06T10:23:00Z
- **Tasks:** 2 (Task 1 with TDD)
- **Files created:** 7

## Accomplishments
- CheckContent RPC with keyword blocklist, URL domain blocklist, and duplicate detection
- SHA-256 content hashing with normalization (lowercase, trim, whitespace collapse) and 24h Redis TTL
- Kafka BehaviorConsumer detecting rapid posting (>10/5min) and link spam (>5 URL posts/1hr)
- Full test suite (20 tests) using miniredis for isolated Redis testing
- Service bootstrap with Redis DB 11, Kafka consumer, and standard middleware chain

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement spam service (TDD RED)** - `c123d87` (test)
2. **Task 1: Implement spam service (TDD GREEN)** - `33cc867` (feat)
3. **Task 2: Kafka behavior consumer and service bootstrap** - `c7eee66` (feat)

## Files Created/Modified
- `internal/spam/data/blocklist.json` - JSON seed data with spam keywords and blocked domains
- `internal/spam/blocklist.go` - In-memory keyword + URL domain blocklist with case-insensitive matching
- `internal/spam/dedup.go` - Redis SHA-256 content hash deduplication with 24h TTL
- `internal/spam/server.go` - SpamServiceServer with CheckContent and ReportSpam RPCs
- `internal/spam/consumer.go` - Kafka BehaviorConsumer for rapid posting and link spam detection
- `internal/spam/spam_test.go` - 20 tests covering blocklist, dedup, URL extraction, and CheckContent
- `cmd/spam/main.go` - Service bootstrap: Redis DB 11, Kafka consumer, blocklist loading

## Decisions Made
- Used vague reasons ("blocked_content", "blocked_url") in CheckContentResponse — never expose specific matched keywords to prevent evasion
- Content normalization before hashing ensures "  Hello   World  " and "hello world" produce same hash
- BehaviorConsumer logs spam flags instead of calling SubmitReport since moderation proto doesn't have that RPC yet (Plan 06-01 will add it)
- Created redyx.behavior.v1 topic with 3 partitions for future behavior event extensibility
- Redis DB 11 per the project's per-service DB isolation convention

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] SubmitReport RPC not available in moderation proto**
- **Found during:** Task 2 (Kafka behavior consumer)
- **Issue:** Plan references calling moderation service SubmitReport via gRPC, but moderation proto only has ListReportQueue — SubmitReport will be added by Plan 06-01
- **Fix:** Consumer logs spam detections with all relevant fields (post_id, author, community, reason, source). Ready to call SubmitReport once the RPC exists.
- **Files modified:** internal/spam/consumer.go
- **Verification:** Service compiles successfully, consumer processes events
- **Committed in:** c7eee66

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Minimal — consumer is fully functional for detection, just logs instead of making gRPC call. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Spam service ready for integration with post-service and comment-service (Plan 06-03+)
- BehaviorConsumer ready to call SubmitReport once moderation proto is extended (Plan 06-01)
- Ready for Plan 06-03: Frontend moderation pages

---
*Phase: 06-moderation-spam-full-stack*
*Completed: 2026-03-06*
