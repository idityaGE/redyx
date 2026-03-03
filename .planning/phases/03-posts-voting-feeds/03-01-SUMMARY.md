---
phase: 03-posts-voting-feeds
plan: 01
subsystem: api
tags: [grpc, postgresql, sharding, consistent-hash, redis, ranking, pagination]

# Dependency graph
requires:
  - phase: 02-auth-user-community
    provides: "Auth interceptor, config loader, pagination helpers, community service pattern"
provides:
  - "PostService gRPC server with 8 RPCs (CRUD, feeds, save)"
  - "ShardRouter with consistent hash ring for community-based sharding"
  - "Hot/Rising ranking algorithms"
  - "VoteEvent protobuf message for Kafka vote pipeline"
  - "Post shard database schemas (posts + saved_posts)"
  - "Sort-aware cursor encoding for cross-shard pagination"
affects: [03-posts-voting-feeds, 04-comments-threading, 05-media-storage]

# Tech tracking
tech-stack:
  added: [serialx/hashring]
  patterns: [consistent-hash-sharding, cross-shard-merge-sort, fan-out-on-read, hot-score-precompute]

key-files:
  created:
    - proto/redyx/common/v1/events.proto
    - internal/post/shard.go
    - internal/post/ranking.go
    - internal/post/cache.go
    - internal/post/server.go
    - cmd/post/main.go
    - migrations/post_shard_0/001_create_posts.up.sql
    - migrations/post_shard_1/001_create_posts.up.sql
  modified:
    - internal/platform/config/config.go
    - internal/platform/pagination/cursor.go
    - internal/platform/auth/interceptor.go
    - deploy/docker/init-databases.sql
    - deploy/envoy/proto.pb
    - go.mod
    - go.sum
    - gen/redyx/common/v1/events.pb.go

key-decisions:
  - "2 post shards in same PostgreSQL instance for v1 simplicity"
  - "Community name used as shard routing key (consistent hash)"
  - "saved_posts centralized on shard_0 to avoid cross-shard coordination"
  - "Hot score precomputed in column, refreshed every 15min for recent posts"
  - "Rising sort computed on-the-fly for last 24h posts only"
  - "Fan-out-on-read for home feed with 2min Redis cache"
  - "Anonymous posts store real author_id in DB but mask in API responses"
  - "GetPost and ListPosts registered as public auth methods for anonymous browsing"

patterns-established:
  - "Shard router pattern: consistent hash ring → pool selection via community ID"
  - "Cross-shard parallel query: goroutine per shard → merge-sort results"
  - "Sort-aware cursor: encode (sort_value, id, created_at) for keyset pagination"
  - "Redis DB isolation: post-service=4, vote-service=5 (read-only from post-service)"

requirements-completed: [POST-01, POST-02, POST-03, POST-04, POST-05, POST-06, POST-07, POST-08, POST-09, POST-10]

# Metrics
duration: 13min
completed: 2026-03-03
---

# Phase 3 Plan 01: Post Service Backend Summary

**Complete post-service with ShardRouter (consistent hash, serialx/hashring), 8 PostService RPCs (CRUD + feeds + save), Lemmy hot ranking algorithm, VoteEvent proto, and cross-shard home feed aggregation via fan-out-on-read with Redis caching**

## Performance

- **Duration:** 13 min
- **Started:** 2026-03-03T14:12:21Z
- **Completed:** 2026-03-03T14:25:37Z
- **Tasks:** 3
- **Files modified:** 16

## Accomplishments
- ShardRouter with consistent hash ring distributes communities across 2 PostgreSQL shard databases
- All 8 PostService RPCs implemented: CreatePost (text/link, media stubbed), GetPost (cross-shard parallel lookup with user_vote/is_saved), UpdatePost, DeletePost, ListPosts (4 sort orders with cursor pagination), ListHomeFeed (cross-shard merge-sort), SavePost, ListSavedPosts
- Hot ranking algorithm (Lemmy formula) precomputed in column with 15-minute background refresh goroutine
- Anonymous post masking works correctly — author_id stored in DB, masked to "[anonymous]" in API responses for non-authors
- VoteEvent protobuf message ready for Kafka vote pipeline

## Task Commits

Each task was committed atomically:

1. **Task 1: VoteEvent proto, shard infrastructure, config extensions, database schemas** - `0bbb462` (feat)
2. **Task 2: PostService gRPC server — CRUD + community feed + anonymous posts + saved posts** - `e22a88b` (feat)
3. **Task 3: Post service entrypoint with shard pool initialization and hot score refresh goroutine** - `0060f33` (feat)

## Files Created/Modified
- `proto/redyx/common/v1/events.proto` - VoteEvent protobuf message for Kafka
- `internal/post/shard.go` - ShardRouter with consistent hash ring and per-shard pools
- `internal/post/ranking.go` - HotScore (Lemmy algorithm) and RisingScore functions
- `internal/post/cache.go` - Redis cache for feed pages, community membership, and vote state reads
- `internal/post/server.go` - PostServiceServer with all 8 RPCs (1120 lines)
- `cmd/post/main.go` - Post service entrypoint with shard pools, migrations, hot score refresh
- `migrations/post_shard_0/001_create_posts.up.sql` - Posts + saved_posts schema for shard 0
- `migrations/post_shard_1/001_create_posts.up.sql` - Posts + saved_posts schema for shard 1
- `internal/platform/config/config.go` - Added KafkaBrokers and PostShardDSNs fields
- `internal/platform/pagination/cursor.go` - Added EncodeSortCursor/DecodeSortCursor
- `internal/platform/auth/interceptor.go` - Added GetPost and ListPosts as public methods
- `deploy/docker/init-databases.sql` - Added posts_shard_0 and posts_shard_1 databases

## Decisions Made
- **2 shards for v1:** Consistent hash ring makes adding shards later a controlled operation. No benefit to starting with more at v1 scale.
- **Community name as shard key:** The community_name field is used for shard routing since we don't have a direct community-service gRPC call yet. In production, community_id would be resolved first.
- **saved_posts on shard_0:** Centralized to avoid cross-shard transaction complexity for bookmark operations.
- **Fan-out-on-read for home feed:** Queries all shards in parallel, merge-sorts results. Cached 2min in Redis. Acceptable for v1 scale (<10K concurrent users).
- **GetPost/ListPosts as public methods:** Anonymous users can browse posts and community feeds without authentication, matching Reddit's behavior.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added public auth methods for post browsing**
- **Found during:** Task 3 (Post service entrypoint)
- **Issue:** Plan didn't specify updating auth interceptor's public methods list for the post service
- **Fix:** Added `/redyx.post.v1.PostService/GetPost` and `/redyx.post.v1.PostService/ListPosts` to publicMethods map
- **Files modified:** internal/platform/auth/interceptor.go
- **Verification:** go vet ./... passes
- **Committed in:** 0060f33 (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Essential for anonymous post browsing. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Post service binary builds and all RPCs implemented
- VoteEvent proto ready for vote-service Kafka producer
- Shard infrastructure ready for deployment
- Ready for remaining Phase 3 plans (vote service, frontend, integration)

## Self-Check: PASSED

All 8 created files verified on disk. All 3 task commits verified in git log.

---
*Phase: 03-posts-voting-feeds*
*Completed: 2026-03-03*
