---
phase: 06-moderation-spam-full-stack
plan: 03
subsystem: api
tags: [grpc, protobuf, spam, moderation, post, comment, scylladb, postgresql]

# Dependency graph
requires:
  - phase: 06-moderation-spam-full-stack
    provides: "moderation service with CheckBan RPC, spam service with CheckContent RPC"
provides:
  - "Post-service moderator internal RPCs (remove, restore, pin, count-pins, remove-by-user)"
  - "Comment-service moderator internal RPCs (remove, restore, remove-by-user)"
  - "Spam and ban checks integrated into post and comment creation flows"
affects: [06-moderation-spam-full-stack, 07-deployment-observability]

# Tech tracking
tech-stack:
  added: []
  patterns: ["fail-open service integration", "ServerOption pattern for optional gRPC clients", "internal RPCs without HTTP annotations"]

key-files:
  created:
    - "internal/post/moderator.go"
    - "internal/comment/moderator.go"
  modified:
    - "proto/redyx/post/v1/post.proto"
    - "proto/redyx/comment/v1/comment.proto"
    - "internal/post/server.go"
    - "internal/comment/server.go"
    - "cmd/post/main.go"
    - "cmd/comment/main.go"

key-decisions:
  - "ServerOption pattern for spam/moderation clients (consistent with existing comment-service WithPostClient pattern)"
  - "Fail-open on service errors for both spam and ban checks (consistent with existing fail-open rate limiting pattern)"
  - "Comment ban check resolves community name via post-service GetPost before checking moderation-service"
  - "RemoveCommentsByUser uses ALLOW FILTERING on author_id — acceptable for infrequent moderation actions"

patterns-established:
  - "Fail-open service integration: log warning and allow content through when external services are unavailable"
  - "Internal RPCs: no google.api.http annotations for service-to-service gRPC calls"

requirements-completed: [MOD-01, MOD-02, MOD-03, SPAM-01, SPAM-02, SPAM-03]

# Metrics
duration: 10min
completed: 2026-03-06
---

# Phase 6 Plan 3: Post/Comment Moderator RPCs + Spam/Ban Integration Summary

**Internal moderator RPCs for post-service (5) and comment-service (3) plus spam/ban checks integrated into content creation flows with fail-open resilience**

## Performance

- **Duration:** 10 min
- **Started:** 2026-03-06T10:34:06Z
- **Completed:** 2026-03-06T10:44:22Z
- **Tasks:** 2
- **Files modified:** 14 (2 protos, 4 generated, 2 new Go files, 4 modified Go files, 2 cmd mains)

## Accomplishments
- Post-service now exposes 5 internal moderator RPCs: ModeratorRemovePost, ModeratorRestorePost, SetPostPinned, CountPinnedPosts, RemovePostsByUser
- Comment-service now exposes 3 internal moderator RPCs: ModeratorRemoveComment, ModeratorRestoreComment, RemoveCommentsByUser
- CreatePost checks ban status and spam content before persisting, with fail-open on service errors
- CreateComment checks ban status and spam content before persisting, with fail-open on service errors
- Vague rejection messages prevent leaking spam detection internals

## Task Commits

Each task was committed atomically:

1. **Task 1: Add moderator internal RPCs to post and comment protos** - `6e5b57e` (feat)
2. **Task 2: Implement moderator RPCs + spam/ban integration** - `fd439d7` (feat)

## Files Created/Modified
- `proto/redyx/post/v1/post.proto` - Added 5 internal RPCs + request/response messages
- `proto/redyx/comment/v1/comment.proto` - Added 3 internal RPCs + request/response messages
- `gen/redyx/post/v1/post.pb.go` - Regenerated protobuf code
- `gen/redyx/post/v1/post_grpc.pb.go` - Regenerated gRPC code
- `gen/redyx/comment/v1/comment.pb.go` - Regenerated protobuf code
- `gen/redyx/comment/v1/comment_grpc.pb.go` - Regenerated gRPC code
- `internal/post/moderator.go` - 5 moderator RPCs using shard router for DB access
- `internal/post/server.go` - Added spamClient/moderationClient fields, ServerOption pattern, ban+spam checks in CreatePost
- `internal/comment/moderator.go` - 3 moderator RPCs using ScyllaDB dual-table pattern
- `internal/comment/server.go` - Added spamClient/moderationClient fields, ServerOption pattern, ban+spam checks in CreateComment
- `cmd/post/main.go` - Wired spam-service and moderation-service gRPC clients
- `cmd/comment/main.go` - Wired spam-service and moderation-service gRPC clients
- `deploy/envoy/proto.pb` - Regenerated Envoy descriptor

## Decisions Made
- Used ServerOption pattern for spam/moderation clients (consistent with existing comment-service WithPostClient pattern)
- Fail-open on service errors: log warning, allow content through if spam/moderation service unreachable
- Comment ban check resolves community name via post-service GetPost before calling moderation-service CheckBan
- RemoveCommentsByUser uses ALLOW FILTERING on author_id in ScyllaDB — acceptable for infrequent moderation actions in v1

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Post-service and comment-service are extended with moderator RPCs and spam/ban integration
- Ready for Plan 04 (moderation-service cross-service integration to call these internal RPCs)

---
*Phase: 06-moderation-spam-full-stack*
*Completed: 2026-03-06*
