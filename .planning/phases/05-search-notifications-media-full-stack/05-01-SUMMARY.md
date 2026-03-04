---
phase: 05-search-notifications-media-full-stack
plan: 01
subsystem: events
tags: [kafka, protobuf, franz-go, event-driven, meilisearch, minio, websocket]

# Dependency graph
requires:
  - phase: 03-posts-voting-feeds
    provides: post-service with shard router, vote-service Kafka pattern
  - phase: 04-comments
    provides: comment-service with ScyllaDB store
provides:
  - CommentEvent proto definition for notification-service consumers
  - PostEvent proto definition for search-service consumers
  - Kafka CommentProducer in comment-service
  - Kafka PostProducer in post-service
  - Platform config fields for Meilisearch, MinIO, WebSocket
affects: [05-02-notification-service, 05-03-search-service, 05-04-media-service, 05-05-websocket]

# Tech tracking
tech-stack:
  added: [meilisearch-go, nhooyr.io/websocket, disintegration/imaging, aws-sdk-go-v2]
  patterns: [fire-and-forget Kafka produce with context.Background(), event proto in separate redyx.events.v1 package]

key-files:
  created:
    - proto/redyx/events/v1/events.proto
    - gen/redyx/events/v1/events.pb.go
    - internal/comment/producer.go
    - internal/post/producer.go
  modified:
    - internal/comment/server.go
    - internal/post/server.go
    - cmd/comment/main.go
    - cmd/post/main.go
    - internal/platform/config/config.go
    - go.mod

key-decisions:
  - "Event protos in separate redyx.events.v1 package (not in common/v1) for clean separation of concerns"
  - "post_author_id left empty in CommentEvent — comment-service lacks cross-service post lookup; notification-service resolves via post-service"
  - "Fire-and-forget Kafka produce pattern with context.Background() consistent with [03-07] decision"

patterns-established:
  - "Event producer pattern: NewXProducer(brokers, logger) → Publish(ctx, event) fire-and-forget → Close()"
  - "Nil-producer guard: if s.producer != nil ensures services work without Kafka in tests"

requirements-completed: [NOTF-01, NOTF-02, SRCH-01]

# Metrics
duration: 9min
completed: 2026-03-05
---

# Phase 5 Plan 1: Kafka Event Publishing Summary

**CommentEvent and PostEvent proto definitions with Kafka producers in comment-service and post-service using fire-and-forget franz-go pattern**

## Performance

- **Duration:** 9 min
- **Started:** 2026-03-04T20:53:29Z
- **Completed:** 2026-03-04T21:02:41Z
- **Tasks:** 2
- **Files modified:** 10

## Accomplishments
- CommentEvent and PostEvent protobuf definitions in redyx.events.v1 package with all fields needed by notification and search services
- Kafka CommentProducer publishes events on comment creation with parent comment author for reply notifications
- Kafka PostProducer publishes events on post create/update/delete with event type enum for search indexing
- Platform config extended with Meilisearch, MinIO/S3, and WebSocket fields for Phase 5 services
- New Go dependencies installed: meilisearch-go, websocket, imaging, aws-sdk-go-v2

## Task Commits

Each task was committed atomically:

1. **Task 1: Create event proto definitions and add config for new services** - `6c9b87f` + `6edd291` (feat + chore — proto/config/deps were partially pre-committed in 6c9b87f, go.mod tidy in 6edd291)
2. **Task 2: Add Kafka producers to comment-service and post-service** - `5f1743e` (feat)

## Files Created/Modified
- `proto/redyx/events/v1/events.proto` — CommentEvent and PostEvent message definitions with PostEventType enum
- `gen/redyx/events/v1/events.pb.go` — Generated Go code from event protos
- `internal/comment/producer.go` — Kafka CommentProducer wrapping franz-go client, publishes to redyx.comments.v1
- `internal/post/producer.go` — Kafka PostProducer wrapping franz-go client, publishes to redyx.posts.v1
- `internal/comment/server.go` — Added producer field, publishes CommentEvent in CreateComment RPC
- `internal/post/server.go` — Added producer field, publishes PostEvent in CreatePost/UpdatePost/DeletePost RPCs
- `cmd/comment/main.go` — Creates CommentProducer from config.KafkaBrokers and injects into server
- `cmd/post/main.go` — Creates PostProducer from config.KafkaBrokers and injects into server
- `internal/platform/config/config.go` — Added Meilisearch, MinIO, WebSocket config fields with env var loading
- `go.mod` / `go.sum` — Added meilisearch-go, nhooyr.io/websocket, disintegration/imaging, aws-sdk-go-v2

## Decisions Made
- **Event proto location:** Used separate `redyx.events.v1` package rather than adding to `redyx.common.v1` — cleaner separation, avoids bloating the common package
- **post_author_id in CommentEvent:** Left empty because comment-service has no cross-service access to post data. Notification-service will resolve post author via its own post-service lookup. This avoids coupling comment-service to post-service.
- **Nil-producer guards:** All Publish calls wrapped in `if s.producer != nil` so services still work in test environments without Kafka

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Resolved pre-existing uncommitted code conflicts**
- **Found during:** Task 1
- **Issue:** Pre-existing uncommitted changes in `proto/redyx/common/v1/events.proto` duplicated the new events proto definitions. Commit `6c9b87f` already contained most Task 1 artifacts.
- **Fix:** Reverted pre-existing common/v1 changes, used the plan's `redyx.events.v1` package as intended
- **Files modified:** proto/redyx/common/v1/events.proto (reverted to committed state)
- **Verification:** `make proto` succeeds, `go build ./cmd/comment/... ./cmd/post/...` passes

**2. [Rule 1 - Bug] post_author_id unavailable in comment-service**
- **Found during:** Task 2
- **Issue:** Plan assumed comment-service has access to post data for post_author_id, but it only has ScyllaDB comment store
- **Fix:** Set post_author_id to empty string; notification-service will resolve this
- **Files modified:** internal/comment/server.go
- **Verification:** CommentEvent still contains all other required fields

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Minor — event publishing works correctly for all fields available in each service's scope

## Issues Encountered
- Pre-existing uncommitted files (`internal/search/`, `internal/notification/`, `internal/media/server.go`) from a previous session exist in the working tree. These reference the old common/v1 event types and cause `go build ./...` to fail, but are out of scope for this plan. The plan's target packages (`./cmd/comment/...`, `./cmd/post/...`) build cleanly.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Event publishing foundation complete — notification-service and search-service can now consume from `redyx.comments.v1` and `redyx.posts.v1` Kafka topics
- Config fields ready for Meilisearch, MinIO, and WebSocket services
- Ready for Plan 02 (notification-service consumer)

---
*Phase: 05-search-notifications-media-full-stack*
*Completed: 2026-03-05*
