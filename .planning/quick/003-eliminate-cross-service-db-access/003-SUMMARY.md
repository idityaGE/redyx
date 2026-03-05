# Cross-Service Database Access Elimination Summary

**Commit:** `89e0af3` — `refactor: eliminate cross-service database access — use gRPC for all inter-service communication`

## What Changed

Four cross-service database access violations were eliminated, replacing direct database queries with proper gRPC inter-service communication.

### Violation #1: post-service -> community DB (LARGEST)

**Before:** `internal/post/server.go` had `communityDB *pgxpool.Pool` used for:
- `resolveCommunity()` — direct SQL `SELECT id FROM communities WHERE name = $1`
- Membership check — direct SQL `SELECT EXISTS(... FROM community_members ...)`
- `getUserCommunityIDs()` — direct SQL joining `community_members` and `communities`

**After:**
- Added `communityv1.CommunityServiceClient` gRPC field to post-service Server
- `resolveCommunity()` calls `communityClient.GetCommunity()` — gets community_id + is_member from response
- Membership check uses `is_member` field from GetCommunity response (no separate query needed)
- `getUserCommunityIDs()` calls `communityClient.ListUserCommunities()` — new RPC added to community-service
- Added `ListUserCommunities` RPC + implementation to community-service proto and server

### Violation #2: post-service -> media DB (EASY)

**Before:** `internal/post/server.go` had `mediaDB *pgxpool.Pool` querying `media_items` table directly in `CreatePost()`.

**After:**
- Added `mediav1.MediaServiceClient` gRPC field to post-service Server
- `CreatePost()` calls `mediaClient.GetMedia()` for each media ID instead of direct SQL
- `cmd/post/main.go` creates gRPC client connection to `media-service:50060`

### Violation #3+4: search-service backfills (ACCEPT KAFKA-ONLY)

**Before:** `cmd/search/main.go` had:
- `seedPostsFromShards()` — connected to post shard DBs directly
- `seedCommunityAutocomplete()` — connected to community DB directly

**After:**
- Removed `seedPostsFromShards()` entirely — Kafka indexer handles post indexing
- Replaced `seedCommunityAutocomplete()` with gRPC call to `communityClient.ListCommunities()` — paginated iteration
- No more direct DB connections in search-service

### Violation #5: user-service -> ScyllaDB comments (MEDIUM)

**Before:** `internal/user/server.go` had `scyllaDB *gocql.Session` querying `comments_by_author` table directly in `GetUserComments()`.

**After:**
- Added `ListCommentsByAuthor` RPC to comment-service proto with `CommentSummary` message type
- Implemented `ListCommentsByAuthor` in comment-service — queries `comments_by_author` ScyllaDB table (owns this data)
- Added post-service gRPC client to comment-service for enrichment (post_title, community_name)
- Replaced `scyllaDB` with `commentv1.CommentServiceClient` in user-service
- `GetUserComments()` delegates to `commentClient.ListCommentsByAuthor()`
- Removed ScyllaDB connection from `cmd/user/main.go`

## Files Changed (23 files, +1522/-663)

### Proto
- `proto/redyx/comment/v1/comment.proto` — Added `ListCommentsByAuthor` RPC + request/response/CommentSummary messages
- `proto/redyx/community/v1/community.proto` — Added `ListUserCommunities` RPC + request/response/UserCommunity messages
- `proto/redyx/post/v1/post.proto` — (from prior work) Added `ListUserPosts` RPC
- `proto/redyx/vote/v1/vote.proto` — (from prior work) Added `author_id` to VoteRequest

### Generated
- `gen/redyx/comment/v1/comment.pb.go`, `comment_grpc.pb.go`
- `gen/redyx/community/v1/community.pb.go`, `community_grpc.pb.go`
- `gen/redyx/post/v1/post.pb.go`, `post_grpc.pb.go`
- `gen/redyx/vote/v1/vote.pb.go`
- `deploy/envoy/proto.pb`

### Service implementations
- `internal/post/server.go` — Replaced `communityDB`/`mediaDB` with gRPC clients
- `internal/user/server.go` — Replaced `scyllaDB` with comment-service gRPC client
- `internal/comment/server.go` — Added `ListCommentsByAuthor` implementation + post enrichment
- `internal/community/server.go` — Added `ListUserCommunities` implementation

### Service main files
- `cmd/post/main.go` — gRPC clients for community-service + media-service (replaces DB pools)
- `cmd/user/main.go` — gRPC client for comment-service (replaces ScyllaDB connection)
- `cmd/search/main.go` — gRPC client for community-service (replaces DB seeding)
- `cmd/comment/main.go` — gRPC client for post-service (comment enrichment)

### Config/infra
- `internal/platform/config/config.go` — Removed `CommunityDatabaseURL` and `MediaDatabaseURL` fields
- `internal/platform/auth/interceptor.go` — Added `ListCommentsByAuthor` and `ListUserCommunities` to public methods
- `docker-compose.yml` — Replaced DB env vars with gRPC service addresses, updated depends_on

## Verification

- `go build ./...` — PASS
- `go vet ./...` — PASS

## Not Changed (by design)

- `post-service` cache.go reading from vote-service Redis — conscious performance trade-off
- Envoy routing for `/api/v1/users/{username}/comments` — still routes to user-service (which now delegates to comment-service gRPC internally)
