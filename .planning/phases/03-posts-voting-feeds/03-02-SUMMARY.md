---
phase: 03-posts-voting-feeds
plan: 02
subsystem: vote
tags: [redis, kafka, grpc, lua, franz-go, voting, idempotent]

# Dependency graph
requires:
  - phase: 02-auth-user-community
    provides: auth interceptor, JWT validator, platform middleware, Redis client
provides:
  - VoteServiceServer gRPC implementation (Vote, GetVoteState RPCs)
  - Redis vote store with atomic Lua script for all 9 vote transitions
  - Kafka producer for async VoteEvent publishing
  - KarmaConsumer module for user-service embedding
  - BatchGetVoteStates for feed-level vote state lookup
affects: [03-posts-voting-feeds, user-service-karma]

# Tech tracking
tech-stack:
  added: [franz-go, franz-go/kadm]
  patterns: [lua-atomic-redis, async-kafka-publish, redis-sadd-deduplication]

key-files:
  created:
    - internal/vote/redis.go
    - internal/vote/server.go
    - internal/vote/kafka.go
    - internal/vote/consumer.go
    - cmd/vote/main.go
  modified:
    - go.mod
    - go.sum

key-decisions:
  - "Async Kafka publish in Vote RPC — fire-and-forget to keep <50ms response"
  - "Redis-only vote service (no PostgreSQL) — Kafka provides durability"
  - "Redis SADD deduplication for karma consumer — 24h TTL on processed set"
  - "6-partition Kafka topic for votes — explicit creation on startup"

patterns-established:
  - "Lua atomic Redis: Single Lua script for multi-key atomic operations (prevents race conditions)"
  - "Async event publish: Kafka produce callback logs errors, doesn't fail the RPC"
  - "Idempotent processing: Redis SADD for exactly-once consumer semantics"

requirements-completed: [VOTE-01, VOTE-02, VOTE-03, VOTE-04, VOTE-05]

# Metrics
duration: 10min
completed: 2026-03-03
---

# Phase 3 Plan 2: Vote Service Summary

**Redis-backed vote service with Lua atomic state management, async Kafka event publishing via franz-go, and idempotent karma consumer with SADD deduplication**

## Performance

- **Duration:** 10 min
- **Started:** 2026-03-03T14:13:05Z
- **Completed:** 2026-03-03T14:23:18Z
- **Tasks:** 3
- **Files created:** 5

## Accomplishments
- VoteStore with Lua script atomically handling all 9 vote state transitions (nil/up/down × up/down/none)
- Vote gRPC server returning new_score within Redis round-trip (<50ms)
- Kafka producer publishing VoteEvent async with target_id partition key for ordering
- KarmaConsumer with Redis SADD deduplication for idempotent karma updates
- Vote service binary (Redis-only, no PostgreSQL dependency)

## Task Commits

Each task was committed atomically:

1. **Task 1: Redis vote store with Lua atomic operations** - `a912b84` (feat)
2. **Task 2: Vote gRPC server + Kafka producer + karma consumer** - `e1aa671` (feat)
3. **Task 3: Vote service entrypoint** - `5105d4c` (feat)

## Files Created/Modified
- `internal/vote/redis.go` - VoteStore with Lua atomic vote script, CastVote, GetVoteState, GetScore, BatchGetVoteStates
- `internal/vote/server.go` - VoteServiceServer implementing Vote and GetVoteState RPCs
- `internal/vote/kafka.go` - Kafka Producer with EnsureTopic (6 partitions, 7-day retention) and async PublishVoteEvent
- `internal/vote/consumer.go` - KarmaConsumer for user-service embedding, processes events with SADD deduplication
- `cmd/vote/main.go` - Vote service entrypoint (Redis DB 5, Kafka producer, gRPC server)
- `go.mod` / `go.sum` - Added franz-go, franz-go/kadm dependencies

## Decisions Made
- Async Kafka publish: fire-and-forget via callback — errors logged but don't fail the RPC, keeping vote response under 50ms
- Redis-only architecture: vote-service has no PostgreSQL connection; Redis sets are source of truth, Kafka provides durability
- Redis SADD deduplication: karma consumer uses `SADD karma:processed:{author_id} {event_id}` with 24h TTL for exactly-once processing
- AuthorId/CommunityId empty in VoteEvent: consumers look up post metadata rather than adding latency to Vote RPC
- 6 Kafka partitions: explicit topic creation on startup prevents Kafka's default 1-partition auto-creation (Pitfall 5)

## Deviations from Plan

None — plan executed exactly as written. The KafkaBrokers config field and VoteEvent proto already existed from prior work (uncommitted Plan 01 partial execution). franz-go dependency was added as part of Task 2.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Vote service ready for Docker Compose integration (Plan 03 or later wiring plan)
- KarmaConsumer module ready to embed in user-service as a goroutine
- BatchGetVoteStates ready for feed response enrichment (open question 3, option C from research)
- Envoy route configuration needed for `/api/v1/votes` endpoint

## Self-Check: PASSED

All 5 created files verified on disk. All 3 task commits found in git history.

---
*Phase: 03-posts-voting-feeds*
*Completed: 2026-03-03*
