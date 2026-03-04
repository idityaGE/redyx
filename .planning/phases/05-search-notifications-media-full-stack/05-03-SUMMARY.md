---
phase: 05-search-notifications-media-full-stack
plan: 03
subsystem: notification
tags: [websocket, kafka, postgresql, grpc, real-time, nhooyr-websocket]

requires:
  - phase: 04-comments
    provides: "Comment service with ScyllaDB storage"
  - phase: 01-foundation-frontend-shell
    provides: "gRPC server bootstrap, config, middleware, auth interceptor"
provides:
  - "Notification service with gRPC + WebSocket dual server"
  - "Kafka consumer for CommentEvent processing"
  - "PostgreSQL notification + preferences storage"
  - "WebSocket hub for real-time notification delivery"
  - "u/username mention detection"
affects: [05-search-notifications-media-full-stack, frontend-notifications]

tech-stack:
  added: [nhooyr.io/websocket v1.8.17]
  patterns: [dual-server gRPC+HTTP, websocket-hub-per-user, kafka-consumer-group-per-service, jwt-query-param-websocket]

key-files:
  created:
    - cmd/notification/main.go
    - internal/notification/server.go
    - internal/notification/store.go
    - internal/notification/consumer.go
    - internal/notification/websocket.go
    - internal/notification/mention.go
    - internal/notification/events.go
    - migrations/notification/001_notifications.up.sql
  modified:
    - proto/redyx/common/v1/events.proto
    - go.mod
    - go.sum

key-decisions:
  - "JSON encoding for CommentEvent Kafka messages (proto definition added but not yet generated; Go struct used directly)"
  - "nhooyr.io/websocket v1.8.17 over gorilla/websocket (gorilla is archived per research)"
  - "JWT token via query parameter for WebSocket auth (WebSocket doesn't support custom headers post-handshake)"
  - "Redis DB 8 reserved for notification-service unread count cache"
  - "Mention-based notifications use username as target (no cross-service user lookup in v1)"

patterns-established:
  - "Dual-server pattern: gRPC on one port + HTTP/WebSocket on another, both started from same main.go"
  - "WebSocket hub pattern: per-user connection map with RWMutex, register/unregister lifecycle"
  - "Offline notification delivery: query PostgreSQL on WebSocket connect for recent unread notifications"

requirements-completed: [NOTF-01, NOTF-02, NOTF-03, NOTF-04, NOTF-05, NOTF-06]

duration: 7min
completed: 2026-03-04
---

# Phase 5 Plan 3: Notification Service Summary

**Notification-service with Kafka consumer, PostgreSQL storage, WebSocket hub, mention detection, and 5 gRPC RPCs using nhooyr.io/websocket**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-04T20:54:24Z
- **Completed:** 2026-03-04T21:02:04Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments
- Full notification-service with dual gRPC (50059) + HTTP/WebSocket (8081) servers
- 5 gRPC RPCs: ListNotifications, MarkRead, MarkAllRead, GetPreferences, UpdatePreferences
- Kafka consumer processes CommentEvents with preference checking and mention detection
- WebSocket hub manages per-user connections with offline notification delivery on connect
- PostgreSQL schema with notifications + preferences tables and proper indexes

## Task Commits

Each task was committed atomically:

1. **Task 1: Create PostgreSQL store, mention detector, and Kafka consumer** - `9c52ca5` (feat)
2. **Task 2: Create WebSocket hub, gRPC server, and service entry point** - `a9c55f4` (feat)

## Files Created/Modified
- `cmd/notification/main.go` - Service entry point with dual gRPC + WebSocket servers
- `internal/notification/server.go` - gRPC server implementing 5 NotificationService RPCs
- `internal/notification/store.go` - PostgreSQL notification + preferences CRUD
- `internal/notification/consumer.go` - Kafka consumer for CommentEvent processing
- `internal/notification/websocket.go` - WebSocket hub with per-user connection management
- `internal/notification/mention.go` - u/username mention extraction regex
- `internal/notification/events.go` - CommentEvent struct for Kafka deserialization
- `migrations/notification/001_notifications.up.sql` - PostgreSQL schema for notifications + preferences
- `proto/redyx/common/v1/events.proto` - Added CommentEvent message definition
- `go.mod` / `go.sum` - Added nhooyr.io/websocket dependency

## Decisions Made
- Used JSON encoding for CommentEvent Kafka messages — proto definition added to events.proto but Go code generated struct used directly since `buf generate` not run. Can switch to proto.Unmarshal when proto codegen is executed.
- nhooyr.io/websocket v1.8.17 chosen over gorilla/websocket (archived per research)
- JWT token passed as query parameter `?token=...` for WebSocket auth since WebSocket doesn't support custom headers post-handshake
- Redis DB 8 reserved for notification-service
- Mention notifications use username as target_id (no cross-service user-id lookup in v1)
- Cursor-based pagination for ListNotifications using offset internally

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added CommentEvent proto definition and Go struct**
- **Found during:** Task 1 (Consumer implementation)
- **Issue:** Plan references `proto/redyx/events/v1/events.proto` with CommentEvent but this proto file doesn't exist. CommentEvent was planned in Plan 01 but never materialized.
- **Fix:** Added CommentEvent message to `proto/redyx/common/v1/events.proto` (where VoteEvent and PostEvent already live) and created a Go struct in `internal/notification/events.go` for JSON deserialization since proto codegen isn't available.
- **Files modified:** proto/redyx/common/v1/events.proto, internal/notification/events.go
- **Verification:** `go build ./internal/notification/...` succeeds
- **Committed in:** 9c52ca5 (Task 1 commit)

**2. [Rule 1 - Bug] Fixed PaginationRequest field names**
- **Found during:** Task 2 (gRPC server)
- **Issue:** Plan implied page_size/page_number but PaginationRequest proto uses cursor/limit fields
- **Fix:** Updated ListNotifications to use cursor-based pagination with limit field, converting cursor to offset internally
- **Files modified:** internal/notification/server.go
- **Verification:** `go build ./internal/notification/...` succeeds
- **Committed in:** a9c55f4 (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug)
**Impact on plan:** Both fixes were necessary for compilation. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Notification service backend complete and compiling
- Ready for frontend notification integration and Docker Compose wiring
- Comment service needs to produce CommentEvents to Kafka for end-to-end flow

---
*Phase: 05-search-notifications-media-full-stack*
*Completed: 2026-03-04*
