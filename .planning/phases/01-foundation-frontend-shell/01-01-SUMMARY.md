---
phase: 01-foundation-frontend-shell
plan: 01
subsystem: infra
tags: [protobuf, buf, grpc, envoy, go, makefile]

# Dependency graph
requires: []
provides:
  - 14 proto definitions with google.api.http annotations (API contract for all 12 services)
  - Generated Go code in gen/ for type-safe gRPC service stubs
  - Envoy proto descriptor at deploy/envoy/proto.pb for REST-to-gRPC transcoding
  - Makefile with `make proto` single-command generation
  - Go module github.com/idityaGE/redyx with gRPC dependencies
affects: [02-platform-libs-docker-envoy, all future service phases]

# Tech tracking
tech-stack:
  added: [buf v1.66.0, protoc-gen-go v1.36.11, grpc-go v1.79.1, google.golang.org/protobuf]
  patterns: [buf v2 workspace config, managed mode go_package override, googleapis dependency exclusion]

key-files:
  created:
    - go.mod
    - buf.yaml
    - buf.gen.yaml
    - buf.lock
    - Makefile
    - proto/redyx/common/v1/common.proto
    - proto/redyx/health/v1/health.proto
    - proto/redyx/auth/v1/auth.proto
    - proto/redyx/user/v1/user.proto
    - proto/redyx/community/v1/community.proto
    - proto/redyx/post/v1/post.proto
    - proto/redyx/comment/v1/comment.proto
    - proto/redyx/vote/v1/vote.proto
    - proto/redyx/search/v1/search.proto
    - proto/redyx/media/v1/media.proto
    - proto/redyx/notification/v1/notification.proto
    - proto/redyx/moderation/v1/moderation.proto
    - proto/redyx/ratelimit/v1/ratelimit.proto
    - proto/redyx/spam/v1/spam.proto
    - deploy/envoy/proto.pb
    - gen/ (27 generated Go files)
  modified:
    - .gitignore

key-decisions:
  - "buf.gen.yaml uses per-file go_package override for googleapis to resolve import path mismatch"
  - "Generated Go code committed to gen/ (not gitignored) per architecture decision"
  - "Health proto renamed to CheckRequest/CheckResponse to pass STANDARD buf lint rules"

patterns-established:
  - "Proto package convention: redyx.<service>.v1 with snake_case fields"
  - "HTTP annotation convention: /api/v1/<resource> flat paths"
  - "Single make proto target for Go code + Envoy descriptor generation"
  - "buf v2 managed mode with googleapis go_package exclusion pattern"

requirements-completed: [FEND-02]

# Metrics
duration: 13min
completed: 2026-03-01
---

# Phase 1 Plan 1: Proto Definitions & Build Pipeline Summary

**14 proto files defining 58 REST-to-gRPC annotated endpoints across 13 services, compiled via buf v2 into Go code and Envoy descriptor with a single `make proto` command**

## Performance

- **Duration:** 13 min
- **Started:** 2026-03-01T22:21:20Z
- **Completed:** 2026-03-01T22:35:13Z
- **Tasks:** 3
- **Files modified:** 64

## Accomplishments
- Complete API contract defined for all 12 services plus health check — 58 HTTP-annotated RPCs with full request/response message types
- buf v2 workspace configured with managed mode for automatic go_package generation and googleapis dependency
- `make proto` generates both Go code (27 files across 14 packages) and Envoy descriptor (173KB) in one command
- All generated Go code compiles cleanly with `go build ./gen/...`

## Task Commits

Each task was committed atomically:

1. **Task 1: Initialize Go module and Buf configuration** - `1496af4` (chore)
2. **Task 2: Define all proto files with HTTP annotations** - `8c250b2` (feat)
3. **Task 3: Create Makefile and run proto generation** - `8422ee2` (feat)

## Files Created/Modified
- `go.mod` - Go module definition (github.com/idityaGE/redyx)
- `buf.yaml` - Buf v2 workspace config with STANDARD lint, WIRE_JSON breaking, googleapis dep
- `buf.gen.yaml` - Code generation config with managed mode and googleapis go_package override
- `buf.lock` - Locked googleapis dependency
- `Makefile` - Build targets: proto, proto-lint, proto-breaking, proto-descriptor, build, test, clean
- `proto/redyx/common/v1/common.proto` - Shared PaginationRequest/PaginationResponse types
- `proto/redyx/health/v1/health.proto` - HealthService.Check with GET /api/v1/health
- `proto/redyx/auth/v1/auth.proto` - 7 RPCs: Register, Login, RefreshToken, Logout, VerifyOTP, ResetPassword, GoogleOAuth
- `proto/redyx/user/v1/user.proto` - 5 RPCs: GetProfile, UpdateProfile, DeleteAccount, GetUserPosts, GetUserComments
- `proto/redyx/community/v1/community.proto` - 9 RPCs: CRUD + membership + moderator management
- `proto/redyx/post/v1/post.proto` - 8 RPCs: CRUD + feeds + save/bookmark
- `proto/redyx/comment/v1/comment.proto` - 6 RPCs: CRUD + list comments/replies
- `proto/redyx/vote/v1/vote.proto` - 2 RPCs: Vote, GetVoteState
- `proto/redyx/search/v1/search.proto` - 2 RPCs: SearchPosts, AutocompleteCommunities
- `proto/redyx/media/v1/media.proto` - 3 RPCs: InitUpload, CompleteUpload, GetMedia
- `proto/redyx/notification/v1/notification.proto` - 5 RPCs: ListNotifications, MarkRead, MarkAllRead, GetPreferences, UpdatePreferences
- `proto/redyx/moderation/v1/moderation.proto` - 7 RPCs: RemoveContent, BanUser, UnbanUser, PinPost, UnpinPost, GetModLog, ListReportQueue
- `proto/redyx/ratelimit/v1/ratelimit.proto` - 1 RPC: CheckRateLimit
- `proto/redyx/spam/v1/spam.proto` - 2 RPCs: CheckContent, ReportSpam
- `deploy/envoy/proto.pb` - Compiled proto descriptor set for Envoy transcoding
- `gen/` - 27 generated Go files (.pb.go and _grpc.pb.go for each service)
- `.gitignore` - Updated with Go patterns, gen/ and proto.pb not ignored

## Decisions Made
- **buf.gen.yaml googleapis go_package override:** buf managed mode rewrites all go_package prefixes, including googleapis. Required per-file go_package override for `google/api/annotations.proto` and `google/api/http.proto` to map to `google.golang.org/genproto/googleapis/api/annotations` instead of the gen/ prefix.
- **Health proto message naming:** Renamed `HealthCheckRequest`/`HealthCheckResponse` to `CheckRequest`/`CheckResponse` to comply with buf STANDARD lint rule (RPC request type should match RPC name pattern).
- **Generated code committed:** gen/ directory is not gitignored — generated Go code is committed per architecture decision for reproducible builds without buf.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Installed buf CLI**
- **Found during:** Task 1
- **Issue:** buf was not installed on the system
- **Fix:** Downloaded buf v1.66.0 binary to ~/bin and added to PATH
- **Files modified:** ~/bin/buf (user-local)
- **Verification:** `buf --version` returns 1.66.0
- **Committed in:** N/A (not a repo file)

**2. [Rule 1 - Bug] Fixed buf.gen.yaml managed mode googleapis import path**
- **Found during:** Task 3
- **Issue:** buf managed mode rewrote googleapis go_package to `github.com/idityaGE/redyx/gen/google/api`, causing `go build` to fail (package not found)
- **Fix:** Added per-file go_package overrides in buf.gen.yaml for `google/api/annotations.proto` and `google/api/http.proto` to use `google.golang.org/genproto/googleapis/api/annotations`
- **Files modified:** buf.gen.yaml
- **Verification:** `go build ./gen/...` compiles successfully
- **Committed in:** 8422ee2 (Task 3 commit)

**3. [Rule 1 - Bug] Renamed health proto message types for buf lint compliance**
- **Found during:** Task 2
- **Issue:** buf STANDARD lint requires RPC request/response types to match the RPC name pattern (Check → CheckRequest, not HealthCheckRequest)
- **Fix:** Renamed `HealthCheckRequest` → `CheckRequest` and `HealthCheckResponse` → `CheckResponse`
- **Files modified:** proto/redyx/health/v1/health.proto
- **Verification:** `buf lint` passes clean
- **Committed in:** 8c250b2 (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (1 blocking, 2 bug)
**Impact on plan:** All auto-fixes were necessary for the build pipeline to function correctly. No scope creep.

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Proto definitions complete — all 12 service APIs defined with full message types
- Ready for Plan 02 (Astro+Svelte frontend shell) and Plan 03 (platform libs + Docker + Envoy)
- Envoy descriptor ready for transcoding configuration in Plan 03

## Self-Check: PASSED

All key files verified on disk. All 3 task commits found. 14 proto files, 27 generated Go files confirmed.

---
*Phase: 01-foundation-frontend-shell*
*Completed: 2026-03-01*
