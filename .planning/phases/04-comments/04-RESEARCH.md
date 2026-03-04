# Phase 4: Comments (Full Stack) - Research

**Researched:** 2026-03-04
**Phase:** 04-comments
**Requirements:** CMNT-01 through CMNT-06

---

## 1. Standard Stack: What Exists, What to Add

### Already Exists (reuse directly)
| Asset | Location | Usage in Phase 4 |
|-------|----------|-------------------|
| `comment.proto` | `proto/redyx/comment/v1/comment.proto` | Fully defined: CRUD + ListComments + ListReplies, CommentSortOrder enum, materialized path field |
| Generated Go code | `gen/redyx/comment/v1/comment.pb.go`, `comment_grpc.pb.go` | Server interface ready to implement |
| `VoteButtons.svelte` | `web/src/components/post/VoteButtons.svelte` | Accepts `targetType` prop, pass `TARGET_TYPE_COMMENT` + comment ID as `postId` |
| `PostBody.svelte` | `web/src/components/post/PostBody.svelte` | marked + DOMPurify markdown rendering, reuse for comment body display |
| `SortBar.svelte` | `web/src/components/feed/SortBar.svelte` | Pattern to replicate for comment sort (Best/Top/New tabs) |
| `api.ts` | `web/src/lib/api.ts` | API client with auth, 401 retry, ApiError — all comment API calls use this |
| `auth.ts` | `web/src/lib/auth.ts` | `whenReady()`, `isAuthenticated()`, `getUser()`, `subscribe()` |
| `time.ts` | `web/src/lib/time.ts` | `relativeTime()` for comment timestamps |
| Vote service | `internal/vote/` | Already supports `TARGET_TYPE_COMMENT` (validated in `server.go:48-49`) |
| Kafka VoteEvent | `proto/redyx/common/v1/events.proto` | `target_type` field accepts "comment" string |
| Platform libraries | `internal/platform/` | grpcserver, config, auth, middleware, errors, pagination, redis |
| Docker Dockerfile | `deploy/docker/Dockerfile` | Multi-stage build, `SERVICE` build arg — works for `comment` service |
| Envoy config | `deploy/envoy/envoy.yaml` | Add cluster + routes for comment-service |
| Makefile | `Makefile` | `make proto` regenerates Go code + Envoy descriptor |

### Must Add (new for Phase 4)
| Component | Location | Notes |
|-----------|----------|-------|
| ScyllaDB container | `docker-compose.yml` | ScyllaDB image, healthcheck, volume |
| Comment service binary | `cmd/comment/main.go` | Follows vote-service pattern but with ScyllaDB instead of Redis/PostgreSQL |
| Comment service logic | `internal/comment/` | `server.go`, `scylla.go`, `wilson.go`, `kafka.go` (consumer for vote score updates) |
| ScyllaDB migration | `migrations/comment/` | CQL schema files (manually applied, no standard migration tool) |
| Comment components | `web/src/components/comment/` | CommentSection, CommentCard, CommentTree, CommentSortBar, CommentForm |
| Envoy comment routes | `deploy/envoy/envoy.yaml` | Routes for `/api/v1/posts/{post_id}/comments` and `/api/v1/comments/` |
| Config: ScyllaDB URL | `internal/platform/config/config.go` | Add `ScyllaDBHosts` field |

### Go Dependencies to Add
| Package | Purpose |
|---------|---------|
| `github.com/gocql/gocql` | ScyllaDB/Cassandra Go driver (the standard, most-used driver, 3600+ importers) |

**Note:** The `gocql` package is the standard Go driver for Cassandra/ScyllaDB. There is also `github.com/scylladb/gocql` (a fork with ScyllaDB-specific optimizations like shard-aware routing), but the upstream `gocql/gocql` is fully compatible with ScyllaDB and has broader community support. For v1 dev with a single-node ScyllaDB, either works — use `gocql/gocql` for simplicity and switch to the ScyllaDB fork later if shard-aware routing is needed.

---

## 2. ScyllaDB Schema Design

### Key Concept: Materialized Path

Materialized path encodes a comment's position in the tree as a string path. Each segment is a sortable, zero-padded identifier derived from a deterministic counter or the comment's creation order within its parent.

**Example tree:**
```
post_abc
├── comment_1 (path: "001")
│   ├── comment_3 (path: "001.001")
│   │   └── comment_6 (path: "001.001.001")
│   └── comment_4 (path: "001.002")
└── comment_2 (path: "002")
    └── comment_5 (path: "002.001")
```

Path properties:
- Lexicographic sort of paths = depth-first tree order (parent always before children)
- `depth` = number of segments = `strings.Count(path, ".") + 1`
- All descendants of comment "001" have paths starting with "001."
- Zero-padding to 3 digits supports 999 children per parent (sufficient for v1)

### Primary Table: `comments_by_post`

This is the main query table. Partition by `post_id`, cluster by `path` for efficient tree-ordered retrieval.

```cql
CREATE KEYSPACE IF NOT EXISTS redyx_comments
WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

CREATE TABLE IF NOT EXISTS redyx_comments.comments_by_post (
    post_id       UUID,
    comment_id    UUID,
    parent_id     UUID,          -- NULL (as empty UUID) for top-level
    author_id     UUID,
    author_username TEXT,
    body          TEXT,
    path          TEXT,           -- materialized path: "001.002.001"
    depth         INT,
    vote_score    INT,
    upvotes       INT,
    downvotes     INT,
    reply_count   INT,
    is_edited     BOOLEAN,
    is_deleted    BOOLEAN,
    created_at    TIMESTAMP,
    edited_at     TIMESTAMP,
    PRIMARY KEY (post_id, path)
) WITH CLUSTERING ORDER BY (path ASC);
```

**Why this works:**
- All comments for a post in one partition — single-partition query for ListComments
- Clustering by `path ASC` gives depth-first tree order by default
- Path prefix scan: `WHERE post_id = ? AND path >= '001.' AND path < '001/'` retrieves all descendants of comment "001" (since '/' is the character after '.' in ASCII)
- Top-level comments: `WHERE post_id = ? AND depth = 1` — but CQL doesn't allow filtering on non-clustering columns efficiently. Instead, use path segment count or a separate approach (see below).

### Lookup Table: `comments_by_id`

For GetComment, UpdateComment, DeleteComment by comment_id (without knowing post_id).

```cql
CREATE TABLE IF NOT EXISTS redyx_comments.comments_by_id (
    comment_id    UUID,
    post_id       UUID,
    parent_id     UUID,
    author_id     UUID,
    author_username TEXT,
    body          TEXT,
    path          TEXT,
    depth         INT,
    vote_score    INT,
    upvotes       INT,
    downvotes     INT,
    reply_count   INT,
    is_edited     BOOLEAN,
    is_deleted    BOOLEAN,
    created_at    TIMESTAMP,
    edited_at     TIMESTAMP,
    PRIMARY KEY (comment_id)
);
```

### Counter Table: `comment_path_counters`

For generating the next child path segment (atomic increment per parent).

```cql
CREATE TABLE IF NOT EXISTS redyx_comments.comment_path_counters (
    post_id    UUID,
    parent_path TEXT,   -- "" for top-level, "001" for children of comment "001"
    counter    COUNTER,
    PRIMARY KEY (post_id, parent_path)
);
```

**Path generation flow (CreateComment):**
1. Increment counter for `(post_id, parent_path)` — atomic via ScyllaDB counter
2. New path segment = zero-padded counter value: `fmt.Sprintf("%03d", counter)`
3. Full path = `parent_path + "." + segment` (or just `segment` for top-level)

### Query Patterns

| Operation | Query | Efficiency |
|-----------|-------|------------|
| ListComments (top-level, New sort) | `SELECT * FROM comments_by_post WHERE post_id = ? AND path > '' AND path < '~' ALLOW FILTERING` with depth=1 filter | Single partition scan. **Better approach:** query all, filter depth=1 in application |
| ListComments (all for tree) | `SELECT * FROM comments_by_post WHERE post_id = ?` | Single partition, full tree in path order |
| ListReplies (children of X) | `SELECT * FROM comments_by_post WHERE post_id = ? AND path > '{parent_path}.' AND path < '{parent_path}/'` | Partition scan with clustering range |
| GetComment by ID | `SELECT * FROM comments_by_id WHERE comment_id = ?` | Direct lookup |
| UpdateComment | Batch update both tables | 2-table update |
| DeleteComment (soft) | Batch update `is_deleted = true, body = '[deleted]', author_username = '[deleted]'` on both tables | 2-table update |
| CreateComment | Batch insert into both tables + increment counter | 3 operations (can use BATCH for inserts) |

### Important: Sorting Strategy

**ScyllaDB cannot sort dynamically.** The clustering order is fixed at table creation. For the `comments_by_post` table, path ASC gives us tree order — but we need to sort top-level comments by Best/Top/New.

**Application-level sort approach (recommended for v1):**
1. Fetch all top-level comments for the post from `comments_by_post` where `depth = 1`
2. Sort in the Go application by Wilson score (Best), vote_score (Top), or created_at (New)
3. Apply pagination in-memory after sorting
4. For each top-level comment, fetch first 2-3 levels of replies (limited to N per parent) — either in a second query or by fetching the full tree and trimming

**Why application-level sort is acceptable:**
- Comments per post rarely exceed 10,000 (even popular posts)
- Fetching depth=1 comments only = typically <500 rows
- Wilson score requires upvotes + downvotes which are already in the row
- ScyllaDB reads from a single partition are fast (~1-5ms for <10K rows)
- Alternative (materialized views per sort) would 3x storage and add write latency

### Lazy-Loading Deep Threads (CMNT-06)

**Strategy:**
1. `ListComments` fetches top-level comments (sorted by Best/Top/New) + first 2 levels of replies per top-level comment
2. `ListReplies` fetches next page of replies for a specific comment (on-demand when user clicks "load N more replies")
3. Frontend initially renders 3 levels (top + 2 reply levels), shows `[load N more replies]` for deeper threads
4. The `reply_count` field enables accurate "N replies" display without fetching children

**Implementation:**
- `ListComments` RPC: Query `comments_by_post` for the post, filter depth <= 3 in application, sort top-level by requested order, paginate top-level comments, include child comments up to depth 3 for each returned top-level comment
- `ListReplies` RPC: Query `comments_by_post` with path prefix range scan for the parent comment's direct children + their children (2 more levels)

---

## 3. Architecture Patterns

### Existing Service Structure (follow exactly)

**cmd/comment/main.go pattern** (based on `cmd/vote/main.go` and `cmd/post/main.go`):
```
1. Initialize zap.Logger
2. config.Load("comment")
3. Connect to ScyllaDB (gocql.ClusterConfig → gocql.Session)
4. Connect to Redis (for vote score overlay, DB 6)
5. Connect to vote-service Redis DB 5 (read-only, for vote state)
6. Create rate limiter
7. Create JWT validator
8. Create gRPC server with middleware chain:
   Recovery → Logging → Auth → RateLimit → ErrorMapping
9. Create and register CommentServiceServer
10. Start Kafka consumer goroutine (vote score updates for comments)
11. srv.Run()
```

**internal/comment/ package structure** (based on `internal/vote/` and `internal/post/`):
```
internal/comment/
├── server.go       # CommentServiceServer implementation (all RPCs)
├── scylla.go       # ScyllaDB store (CRUD operations, queries)
├── wilson.go       # Wilson score calculation
├── kafka.go        # Kafka consumer for comment vote score updates
└── path.go         # Materialized path generation utilities
```

### Key Architecture Decisions

1. **Comment service is a separate gRPC microservice** (like vote-service, post-service)
2. **ScyllaDB for storage** (NOT PostgreSQL) — this is a hard architectural constraint from PROJECT.md
3. **Redis for vote score overlay** — comment service reads from vote-service Redis DB 5 for live scores (same pattern as post-service `cache.go`)
4. **Kafka consumer for score persistence** — consumes vote events where `target_type == "comment"`, updates `vote_score`/`upvotes`/`downvotes` in ScyllaDB
5. **Wilson score computed at query time** — not precomputed (unlike hot_score for posts), because comment vote counts change frequently and there are fewer comments per page than posts per feed
6. **Auth interceptor with public methods** — `ListComments` and `ListReplies` should be accessible without auth (like `GetPost` and `ListPosts` in post-service), but auth is optional for user_vote state

### gRPC Port Assignment
Following the existing pattern:
- skeleton: 50051
- auth: 50052
- user: 50053
- community: 50054
- post: 50055
- vote: 50056
- **comment: 50057**

### Redis DB Assignment
Following `[02-05]` decision (Redis DB isolation):
- auth=1, user=2, community=3, post=4, vote=5
- **comment=6**

---

## 4. Wilson Score Algorithm

### What It Is

Wilson score lower bound gives a confidence interval for the "true" upvote ratio of a comment, accounting for sample size. A comment with 5 upvotes and 0 downvotes should not rank higher than one with 100 upvotes and 10 downvotes, even though the first has a "perfect" ratio.

### Formula

The Wilson score lower bound (for a 95% confidence interval) is:

```
wilson_lower = (p_hat + z²/(2n) - z * sqrt((p_hat*(1-p_hat) + z²/(4n)) / n)) / (1 + z²/n)
```

Where:
- `p_hat` = upvotes / (upvotes + downvotes) — observed success ratio
- `n` = upvotes + downvotes — total votes
- `z` = 1.96 for 95% confidence (standard, used by Reddit)

### Go Implementation

```go
package comment

import "math"

// WilsonScore computes the lower bound of the Wilson score confidence interval.
// Used for "Best" comment sorting — surfaces quality comments by upvote ratio
// while accounting for sample size.
//
// Returns 0.0 for comments with no votes (these sort to bottom).
// z = 1.96 for 95% confidence interval (same as Reddit's original algorithm).
func WilsonScore(upvotes, downvotes int) float64 {
    n := float64(upvotes + downvotes)
    if n == 0 {
        return 0
    }

    const z = 1.96 // 95% confidence
    phat := float64(upvotes) / n
    z2 := z * z

    numerator := phat + z2/(2*n) - z*math.Sqrt((phat*(1-phat)+z2/(4*n))/n)
    denominator := 1 + z2/n

    return numerator / denominator
}
```

### Edge Cases
- 0 votes → score = 0.0 (sort to bottom)
- 1 upvote, 0 downvotes → score ≈ 0.207 (low confidence, doesn't dominate)
- 100 upvotes, 10 downvotes → score ≈ 0.849 (high confidence)
- 5 upvotes, 5 downvotes → score ≈ 0.337 (controversial, low ranking)

### Where Wilson Score Is Computed
- **At query time in `ListComments`**: After fetching top-level comments, compute Wilson score for each, sort by score descending, then paginate
- **NOT stored in the database**: Unlike hot_score for posts, Wilson score is cheap to compute and votes change frequently. Storing it would require constant updates
- Requires `upvotes` and `downvotes` columns in ScyllaDB (not just `vote_score` net), which the schema already includes

### Getting Upvotes/Downvotes

The vote service stores upvote/downvote sets in Redis:
- `votes:up:{target_id}` — SET of user IDs who upvoted
- `votes:down:{target_id}` — SET of user IDs who downvoted

The comment service can either:
1. **Read SCARD from vote Redis** (like post-service reads scores) — preferred, always up-to-date
2. **Maintain local counts via Kafka consumer** — eventually consistent, but works without Redis read access

**Recommended:** Read from vote Redis (DB 5) for Wilson sort, same as post-service reads live scores. Fall back to local ScyllaDB counts if Redis is unavailable.

---

## 5. Frontend Component Architecture

### Component Tree

```
PostDetail.svelte
└── CommentSection.svelte (client:load)
    ├── CommentSortBar.svelte
    ├── CommentForm.svelte (top-level, click-to-reveal "[write comment]")
    └── CommentTree.svelte
        └── CommentCard.svelte (recursive)
            ├── VoteButtons.svelte (targetType="TARGET_TYPE_COMMENT")
            ├── PostBody.svelte (for markdown body)
            ├── CommentForm.svelte (inline reply, toggleable)
            └── CommentTree.svelte (children, recursive)
```

### Component Breakdown

**CommentSection.svelte** — Top-level container, mounts below PostDetail
- Props: `postId: string`
- State: `comments`, `sort`, `loading`, `error`, `hasMore`, `cursor`
- On mount: `whenReady().then(() => fetchComments())`
- Manages sort state with localStorage persistence
- Renders CommentSortBar + top-level CommentForm + CommentTree

**CommentSortBar.svelte** — Sort selector (Best/Top/New)
- Props: `sort: string`, `onSortChange: (sort: string) => void`
- Pattern: replicate SortBar.svelte but with 3 tabs only, no time range
- Sort IDs: `COMMENT_SORT_ORDER_BEST`, `COMMENT_SORT_ORDER_TOP`, `COMMENT_SORT_ORDER_NEW`
- localStorage key: `commentSort`

**CommentCard.svelte** — Single comment with actions
- Props: `comment`, `postId`, `depth`, `onReplySubmitted`
- Component-local `$state` for: `collapsed`, `replying`, `editing`, `confirmingDelete`
- Renders: header (author, time, collapse toggle) → body (markdown) → action bar → inline reply form → children
- Collapse: `[-]` button toggles children/body visibility, shows `[+] u/username · N replies` when collapsed
- Depth-based indentation: `padding-left: {depth * 1.5}rem` with `border-left` per level (max 3 visual levels)
- For depth > 3: show `[load N more replies]` link instead of rendering inline

**CommentForm.svelte** — Textarea for creating/editing comments
- Props: `postId`, `parentId?`, `onSubmit`, `placeholder?`, `initialBody?`
- Top-level: click-to-reveal `[write comment]` button → textarea
- Reply: appears inline below comment when `[reply]` is clicked
- Submit: optimistic insert — close form, insert comment into tree immediately, revert on error
- Validation: max 10,000 chars

**CommentTree.svelte** — Recursive tree renderer
- Props: `comments: Comment[]`, `postId`, `depth: number`
- Renders each comment as CommentCard, passing incremented depth
- Handles `[load more replies]` at bottom for pagination

### Integration with PostDetail

The comment section mounts **inside PostDetail.svelte** below the post content, OR as a separate component in `web/src/pages/post/[id].astro`. Given PostDetail already owns the post fetch, the cleanest approach is:

**Option A (recommended):** Add `<CommentSection postId={postId} client:load />` below `<PostDetail>` in the Astro page. Comment section independently fetches its own data after auth is ready.

```astro
<!-- web/src/pages/post/[id].astro -->
<BaseLayout title="Post">
  <PostDetail postId={id!} client:load />
  <CommentSection postId={id!} client:load />
</BaseLayout>
```

This keeps PostDetail unchanged and comment section is a separate Svelte island.

### Recursive Rendering Approach

Svelte 5 supports recursive components via `{#snippet}` or by importing the component itself. The pattern:

```svelte
<!-- CommentCard.svelte -->
<script lang="ts">
  import CommentCard from './CommentCard.svelte'; // self-import for recursion
</script>

<!-- Render children -->
{#if !collapsed && comment.children?.length}
  {#each comment.children as child}
    <CommentCard comment={child} postId={postId} depth={depth + 1} />
  {/each}
{/if}
```

Svelte handles recursive component imports correctly. The tree terminates naturally when `children` is empty.

### Data Shape for Frontend

The API returns a flat list of comments. The frontend must **build the tree** from the flat list using `parent_id` relationships OR rely on the materialized path for ordering and use depth for indentation.

**Recommended approach:** Server returns comments in path order (already tree-ordered). Frontend renders them flat but with depth-based indentation. No tree-building required. This is simpler and avoids recursive data restructuring.

```typescript
type Comment = {
  commentId: string;
  postId: string;
  parentId: string;
  authorId: string;
  authorUsername: string;
  body: string;
  voteScore: number;
  replyCount: number;
  path: string;
  depth: number;
  isEdited: boolean;
  isDeleted: boolean;
  createdAt: string;
  editedAt: string;
};
```

**Flat rendering with depth:**
```svelte
{#each comments as comment}
  <div style="padding-left: {Math.min(comment.depth - 1, 2) * 1.5}rem">
    <CommentCard {comment} {postId} />
  </div>
{/each}
```

Visual nesting caps at 3 levels (depth 1, 2, 3), deeper comments render at level 3 indentation with `[continue thread →]` or `[load N more replies]`.

### Optimistic UI Patterns (from existing codebase)

Follow the VoteButtons and PostDetail patterns:
1. **Vote:** Same VoteButtons.svelte component, pass `targetType="TARGET_TYPE_COMMENT"` and `postId={comment.commentId}`
2. **Reply submit:** Optimistic insert — add comment to local array immediately, revert on API error
3. **Delete:** Inline `delete? yes / no` confirmation, then replace body with `[deleted]` optimistically
4. **Edit:** Inline textarea toggle (same pattern as PostDetail edit mode)

---

## 6. Integration Points

### Vote Service (Already Works)

The vote service (`internal/vote/server.go:48-49`) already validates `TARGET_TYPE_COMMENT`:
```go
if req.GetTargetType() != votev1.TargetType_TARGET_TYPE_POST &&
    req.GetTargetType() != votev1.TargetType_TARGET_TYPE_COMMENT {
```

The frontend `VoteButtons.svelte` already accepts `targetType` prop. Comment voting works by:
1. Passing `targetType="TARGET_TYPE_COMMENT"` and `postId={comment.commentId}` to VoteButtons
2. Vote service stores vote in Redis keyed by comment ID
3. Vote event published to Kafka with `target_type: "comment"`

### Kafka Events

**Vote score updates for comments:**
- Vote service publishes `VoteEvent` with `target_type: "comment"` and `target_id: commentId`
- Comment service runs a Kafka consumer (like `post/vote_consumer.go`) that:
  1. Reads from `redyx.votes.v1` topic (same topic as post votes)
  2. Filters events where `target_type == "comment"`
  3. Reads current up/down counts from vote Redis (`SCARD votes:up:{commentId}`, `SCARD votes:down:{commentId}`)
  4. Updates `vote_score`, `upvotes`, `downvotes` in ScyllaDB
- Consumer group: `comment-service.redyx.votes.v1`

**Post comment_count updates:**
- When a comment is created/deleted, the comment service needs to update `comment_count` on the post
- **Option A:** Publish a `CommentEvent` to Kafka, post-service consumes it and updates count
- **Option B:** Comment service directly increments/decrements via a gRPC call to post-service
- **Option C (simplest for v1):** Comment service directly updates `comment_count` in post's PostgreSQL shard (same pattern as post-service accessing community DB)
- **Recommended:** Option A (Kafka event) — maintains service boundaries. Or Option C with a read-only connection to post DB for simplicity.

### Docker/Envoy Wiring

**docker-compose.yml additions:**

1. **ScyllaDB container:**
```yaml
scylladb:
  image: scylladb/scylla:6.2
  ports: ["9042:9042"]
  command: --smp 1 --memory 512M --developer-mode 1
  volumes:
    - scylla-data:/var/lib/scylla
  healthcheck:
    test: ["CMD-SHELL", "cqlsh -e 'describe cluster'"]
    interval: 10s
    timeout: 10s
    retries: 10
    start_period: 60s
```

2. **Comment service container:**
```yaml
comment-service:
  build:
    context: .
    dockerfile: deploy/docker/Dockerfile
    args:
      SERVICE: comment
  environment:
    SCYLLADB_HOSTS: "scylladb:9042"
    SCYLLADB_KEYSPACE: "redyx_comments"
    REDIS_URL: redis://redis:6379/6
    GRPC_PORT: "50057"
    JWT_SECRET: "dev-jwt-secret-32-bytes-minimum!!"
    KAFKA_BROKERS: "kafka:9092"
  ports: ["50057:50057"]
  depends_on:
    scylladb:
      condition: service_healthy
    redis:
      condition: service_healthy
    kafka:
      condition: service_healthy
```

**envoy.yaml additions:**

Routes (add BEFORE the `/api/v1/posts` catch-all):
```yaml
# Comment routes — BEFORE post catch-all
# ListComments: GET /api/v1/posts/{post_id}/comments
# CreateComment: POST /api/v1/posts/{post_id}/comments
# These share the same path but different methods — Envoy routes by prefix
# Since post-service also has /api/v1/posts, we need regex for comments
- match:
    safe_regex:
      regex: "/api/v1/posts/[^/]+/comments.*"
  route:
    cluster: comment-service
    timeout: 30s
- match: { prefix: "/api/v1/comments" }
  route:
    cluster: comment-service
    timeout: 30s
```

Cluster:
```yaml
- name: comment-service
  type: STRICT_DNS
  lb_policy: ROUND_ROBIN
  typed_extension_protocol_options:
    envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
      "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
      explicit_http_config:
        http2_protocol_options: {}
  load_assignment:
    cluster_name: comment-service
    endpoints:
    - lb_endpoints:
      - endpoint:
          address:
            socket_address: { address: comment-service, port_value: 50057 }
```

Transcoder service registration:
```yaml
services:
  - redyx.comment.v1.CommentService  # Add to existing list
```

**CRITICAL Envoy routing note:** The comment routes for `/api/v1/posts/{post_id}/comments` MUST come before the existing `/api/v1/posts` prefix route. Envoy uses first-match routing. The existing post-service route `prefix: "/api/v1/posts"` would catch comment requests. Use a regex match for `posts/.../comments` placed above the post prefix route (same pattern as the existing community posts regex at `envoy.yaml:66-71`).

### Config Updates

Add to `internal/platform/config/config.go`:
```go
// ScyllaDB fields (Phase 4)
ScyllaDBHosts    string
ScyllaDBKeyspace string
```

With env vars: `SCYLLADB_HOSTS`, `SCYLLADB_KEYSPACE`

---

## 7. Common Pitfalls and Don't-Hand-Roll Warnings

### ScyllaDB Pitfalls

1. **No JOINs, no subqueries** — Every query pattern needs its own table. The `comments_by_post` and `comments_by_id` tables are the minimum viable set.

2. **No UPDATE ... SET counter = counter + 1 (except counter tables)** — Regular columns don't support increments. Use counter tables for path counters, and regular UPDATE with absolute values for vote scores.

3. **Counter tables have strict rules** — Counter tables can only have counter columns plus primary key columns. No mixing counter and non-counter columns. That's why `comment_path_counters` is a separate table.

4. **BATCH is not a transaction** — ScyllaDB BATCH provides atomicity only for a single partition. Cross-partition batches (like updating `comments_by_post` and `comments_by_id`) are NOT atomic. In practice this is acceptable — both writes usually succeed, and if one fails the data is self-healing on next read.

5. **ALLOW FILTERING is a full-table scan** — Never use it in production queries. The schema is designed so every query hits a partition key directly.

6. **Tombstones from deletes** — Soft-deleting (setting `is_deleted = true`) is better than CQL DELETE for comments. CQL DELETE creates tombstones that degrade read performance. Our soft-delete approach avoids this entirely.

7. **ScyllaDB startup is slow** — The container takes 30-60 seconds to become ready. The healthcheck `start_period: 60s` is essential. The comment service must handle connection retries on startup.

8. **No standard migration tool** — Unlike PostgreSQL's golang-migrate, ScyllaDB has no widely-used Go migration tool. Use a simple approach: read `.cql` files from `migrations/comment/` and execute them on startup (similar to `cmd/post/main.go` `runMigrations`), with a `schema_migrations` table for version tracking.

### Wilson Score Pitfalls

9. **Don't use net score for "Best" sort** — `vote_score` (upvotes - downvotes) is Top sort. Best sort requires Wilson score computed from separate upvotes and downvotes counts.

10. **Don't store Wilson score in the database** — It changes on every vote and is cheap to compute (one sqrt). Compute at query time.

### Frontend Pitfalls

11. **Infinite recursion in component rendering** — Cap visual depth at 3 levels. Svelte handles recursive imports but you need a depth check to terminate rendering.

12. **Don't build a tree from flat data** — Render flat with indentation based on `depth` field. Tree-building adds complexity with no visual benefit when you already have depth-first ordered data.

13. **Optimistic insert position** — When a user submits a reply, the new comment needs to appear at the correct position in the flat list (immediately after the parent's subtree). Use the parent's path to determine insertion index.

14. **VoteButtons reuse** — The `postId` prop name in VoteButtons.svelte is misleading for comments, but it works because the vote service uses `targetId` (generic). Pass comment ID as the `postId` prop.

### Integration Pitfalls

15. **Envoy route ordering** — Comment routes for `/api/v1/posts/{post_id}/comments` MUST be placed before the `/api/v1/posts` catch-all. First-match routing means the wrong service gets the request otherwise. This is the same issue solved for community posts (`envoy.yaml:66-71`).

16. **Proto descriptor must be rebuilt** — After any Envoy route change or proto change, run `make proto` to regenerate `deploy/envoy/proto.pb`. The transcoder needs the descriptor to map REST ↔ gRPC.

17. **Kafka consumer group naming** — Each consumer must have a unique group ID. Post-service uses `post-service.redyx.votes.v1`, comment-service should use `comment-service.redyx.votes.v1`. Both consume the same topic but process different `target_type` values.

---

## 8. Key Decisions for Planner

### Decisions That Are Locked (from CONTEXT.md)

| Decision | Detail |
|----------|--------|
| Visual nesting | 3 levels, `padding-left` + `border-left` lines |
| Collapse | `[-]`/`[+]` toggle, collapsed shows `[+] u/username · N replies` |
| Reply form | Inline below comment, optimistic insert |
| Top-level comment box | Click-to-reveal `[write comment]` |
| Markdown preview | None (comments are short) |
| Deleted state | `[deleted]` body + author, preserve children |
| Sort default | Best (Wilson), with SortBar |
| Sort options | Best/Top/New (Controversial deferred) |
| Sort persistence | localStorage |

### Decisions for Planner to Make

| Decision | Options | Recommendation |
|----------|---------|----------------|
| ScyllaDB Go driver | `gocql/gocql` vs `scylladb/gocql` | `gocql/gocql` — standard, sufficient for v1 single-node |
| Comment count updates | Kafka event vs direct DB vs gRPC call | Kafka event (new `CommentEvent` proto) or direct SQL update to post shard for simplicity |
| Flat rendering vs tree building | Flat list with depth indentation vs recursive tree build | Flat rendering — server returns path-ordered data, frontend uses depth for indentation |
| Deep thread continuation UX | Inline expansion vs separate thread view | Inline expansion with depth reset (Claude's discretion per CONTEXT.md) |
| Score-based auto-collapse | Collapse below -5 threshold | Claude's discretion — suggest threshold of -5 with `[score below threshold]` label |
| ScyllaDB migration approach | Custom Go migration runner vs manual CQL | Custom runner in main.go (match post-service pattern) reading `.cql` files |
| Wilson score data source | Vote Redis SCARD vs ScyllaDB stored counts | Vote Redis for real-time accuracy, ScyllaDB as fallback |
| Envoy comment route strategy | Regex before posts vs path restructuring | Regex `"/api/v1/posts/[^/]+/comments.*"` before `/api/v1/posts` prefix (proven pattern) |

### Plan Sequencing Suggestion

1. **Plan 04-01**: ScyllaDB setup + comment service backend (schema, CRUD RPCs, ScyllaDB store, gocql wiring)
2. **Plan 04-02**: Wilson score + sorting + Kafka consumer (Best/Top/New sort, vote score sync, reply count)
3. **Plan 04-03**: Docker/Envoy wiring (ScyllaDB container, comment-service container, Envoy routes, proto rebuild)
4. **Plan 04-04**: Frontend comment tree (CommentSection, CommentCard, CommentSortBar, CommentForm, recursive rendering)
5. **Plan 04-05**: Frontend integration + E2E verification (mount in PostDetail, voting, optimistic UI, lazy-loading, curl tests)

Alternative 4-plan structure (combine 01+02 backend, 03 infra, 04+05 frontend+E2E):
1. **Plan 04-01**: Comment service backend (ScyllaDB schema + store + all RPCs + Wilson score + Kafka consumer)
2. **Plan 04-02**: Docker/Envoy/infra (ScyllaDB container, comment-service container, Envoy routes, proto rebuild, config)
3. **Plan 04-03**: Frontend comment components (CommentSection, CommentCard, CommentSortBar, CommentForm, tree rendering, voting)
4. **Plan 04-04**: E2E integration + verification (mount in page, lazy-loading, optimistic UI, curl tests, human checkpoint)

---

*Research completed: 2026-03-04*
*Sources: Codebase exploration of all existing services, proto definitions, frontend components, Docker/Envoy config*
