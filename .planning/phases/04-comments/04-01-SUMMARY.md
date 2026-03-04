---
phase: 04-comments
plan: 01
subsystem: comments
tags: [scylladb, gocql, grpc, kafka, wilson-score, materialized-path]

# Dependency graph
requires:
  - phase: 03-posts-voting-feeds
    provides: "Vote-service Redis (DB 5) for SCARD counts and user_vote state, Kafka votes topic, platform middleware"
provides:
  - "CommentService gRPC with 6 RPCs (CreateComment, GetComment, UpdateComment, DeleteComment, ListComments, ListReplies)"
  - "ScyllaDB store with dual-table writes and materialized path ordering"
  - "Wilson score algorithm for Best comment sorting"
  - "Kafka consumer for comment vote score updates"
  - "comment-service binary (cmd/comment/main.go)"
affects: [04-comments, 05-search-notifications-media, 06-moderation-spam]

# Tech tracking
tech-stack:
  added: [gocql, scylladb]
  patterns: [materialized-path-tree, dual-table-writes, counter-table-path-gen, scylladb-retry-loop]

key-files:
  created:
    - migrations/comment/001_create_comments.cql
    - internal/comment/path.go
    - internal/comment/wilson.go
    - internal/comment/scylla.go
    - internal/comment/kafka.go
    - internal/comment/server.go
    - cmd/comment/main.go
  modified:
    - internal/platform/config/config.go
    - internal/platform/auth/interceptor.go
    - go.mod
    - go.sum

key-decisions:
  - "ScyllaDB dual-table approach: comments_by_post (query) + comments_by_id (lookup) for efficient access patterns"
  - "Counter table for atomic materialized path generation (avoids race conditions)"
  - "Read-increment-write for reply_count (acceptable minor race risk for v1)"
  - "In-memory sort for ListComments (fetch all, sort, paginate) — acceptable for v1 comment volumes"
  - "ScyllaDB connection retry loop (30 attempts, 2s apart) for slow container startup"
  - "Separate Kafka consumer group (comment-service.redyx.votes.v1) on same topic as post-service"

patterns-established:
  - "ScyllaDB service pattern: gocql session with retry loop, RunMigrations from .cql files"
  - "Dual-table write pattern: write to both query table and lookup table on every mutation"
  - "Materialized path pattern: counter table + NextPath() for deterministic tree ordering"

requirements-completed: [CMNT-01, CMNT-02, CMNT-03, CMNT-04, CMNT-05, CMNT-06]

# Metrics
duration: 5min
completed: 2026-03-04
---

# Phase 4 Plan 1: Comment Service Backend Summary

**ScyllaDB-backed comment service with materialized path tree ordering, Wilson score Best sort, 6 gRPC RPCs, Kafka vote consumer, and dual-table write pattern**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-04T19:03:48Z
- **Completed:** 2026-03-04T19:08:57Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments
- Complete ScyllaDB schema with 3 tables (comments_by_post, comments_by_id, comment_path_counters) for efficient query and lookup patterns
- All 6 CommentService RPCs implemented: CreateComment with materialized path generation, GetComment with user_vote, UpdateComment (author-only), DeleteComment (soft delete preserving threads), ListComments (Best/Top/New sort + depth 3), ListReplies (lazy-load deep threads)
- Wilson score algorithm for Best sort — same confidence interval approach as Reddit's original algorithm
- Kafka consumer processes comment vote events with idempotent SCARD-based score derivation from vote-service Redis
- Service entrypoint wires ScyllaDB (with 60s retry loop), Redis, Kafka, and full middleware chain

## Task Commits

Each task was committed atomically:

1. **Task 1: ScyllaDB schema, config extension, path utilities, Wilson score** - `f1285f2` (feat)
2. **Task 2: ScyllaDB store, Kafka consumer, gRPC server, service entrypoint** - `0704b23` (feat)

## Files Created/Modified
- `migrations/comment/001_create_comments.cql` - ScyllaDB schema with 3 tables
- `internal/comment/path.go` - Materialized path utilities (NextPath, ParentPath, Depth, IsDescendant)
- `internal/comment/wilson.go` - Wilson score lower bound confidence interval
- `internal/comment/scylla.go` - ScyllaDB store with CRUD, ListComments, ListReplies, UpdateVoteScore
- `internal/comment/kafka.go` - Kafka vote consumer filtering for comment target_type
- `internal/comment/server.go` - CommentServiceServer with all 6 RPCs
- `cmd/comment/main.go` - Service entrypoint with ScyllaDB retry, Redis, Kafka wiring
- `internal/platform/config/config.go` - Added ScyllaDBHosts and ScyllaDBKeyspace fields
- `internal/platform/auth/interceptor.go` - Registered GetComment, ListComments, ListReplies as public methods

## Decisions Made
- ScyllaDB dual-table approach: comments_by_post for efficient post-based queries (clustered by path ASC), comments_by_id for fast single-comment lookups — both updated on every write
- Counter table for materialized path generation: atomic counter increment guarantees unique path segments per parent, avoiding race conditions
- Read-increment-write for reply_count updates: acceptable minor race risk for v1 (alternative would be LWT which adds latency)
- In-memory sort for ListComments: fetch all comments, sort top-level by Wilson/vote_score/created_at, paginate — acceptable for v1 comment volumes before needing materialized views
- ScyllaDB connection retry: 30 attempts * 2 seconds = 60 seconds, matching ScyllaDB's typical cold start time in Docker

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Comment service backend complete, ready for Docker/Envoy wiring (04-02)
- ScyllaDB container and Envoy route configuration needed next
- Frontend comment tree components will consume ListComments and ListReplies RPCs

---
*Phase: 04-comments*
*Completed: 2026-03-04*
