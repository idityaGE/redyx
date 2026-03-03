# Phase 3: Posts + Voting + Feeds - Research

**Researched:** 2026-03-03
**Domain:** Post creation, voting, feed algorithms, sharding — Go gRPC services + Kafka + Astro/Svelte frontend
**Confidence:** HIGH

## Summary

Phase 3 builds two backend gRPC services (post-service, vote-service), introduces Kafka for async event processing, implements post sharding with consistent hashing, and delivers frontend pages for post creation, feed browsing, voting, and post detail views. Protos for PostService and VoteService are already defined with HTTP annotations. The existing platform libraries, Docker Compose infrastructure, and Envoy configuration need extension but not replacement.

The hardest problem in this phase is **home feed aggregation** — querying posts across sharded databases for all communities a user has joined, sorted by ranking algorithms (Hot/New/Top/Rising). The second hardest is **coordinating Redis vote state with PostgreSQL persistence via Kafka**. Both problems have well-understood solutions, documented below.

**Primary recommendation:** Fan-out-on-read with aggressive caching for home feed. Two logical PostgreSQL databases (not separate instances) for post sharding in v1, with application-level routing via consistent hashing. Kafka in KRaft mode (no Zookeeper) via bitnami/kafka for async vote→score→karma pipeline. Redis for real-time vote state and score caching. franz-go as the Kafka client.

## Standard Stack

| Component | Library/Tool | Version | Why |
|-----------|-------------|---------|-----|
| Kafka client (Go) | `github.com/twmb/franz-go` | v1.20.7 | Pure Go, no CGO, full protocol support, fastest benchmarks. Already chosen in project stack. |
| Kafka admin | `github.com/twmb/franz-go/pkg/kadm` | v1.17.2 | Topic creation/management. Companion to franz-go. |
| Kafka broker | `bitnami/kafka:3.7` | 3.7 | KRaft mode (no Zookeeper). Architecture doc already specifies this. |
| Consistent hashing | `github.com/serialx/hashring` | latest | Lightweight consistent hash ring with virtual nodes. Well-maintained, minimal API. |
| Markdown rendering (frontend) | `marked` | v15+ | Fast, spec-compliant, works in browser. Small bundle (~40KB gzipped). No framework dependency. |
| Markdown sanitization | `DOMPurify` | v3+ | XSS prevention for rendered markdown HTML. Industry standard. |
| URL validation (Go) | `net/url` (stdlib) | — | Standard library URL parsing. No third-party needed. |
| Intersection Observer (scroll) | Browser API | — | Native API for infinite scroll trigger. No library needed. |

### Libraries NOT Needed

- **svelte-markdown / mdsvex**: Overkill. We're rendering user-submitted markdown at runtime, not authoring markdown components. `marked` + `DOMPurify` is simpler and more predictable.
- **hashicorp/consistent**: Overly complex for our use case. `serialx/hashring` is simpler with virtual node support.
- **segmentio/kafka-go**: Inferior to franz-go (slower rebalancing, less active maintenance).

## Architecture Patterns

### Home Feed Aggregation

**Recommendation: Fan-out-on-read with Redis caching**

**Why not fan-out-on-write (pre-computed feeds)?**
- Fan-out-on-write means every new post triggers writes to the feed list of every member of that community. A community with 10K members = 10K Redis/DB writes per post.
- Works at Twitter scale with dedicated infrastructure. Overkill for v1 with <10K users.
- Creates massive write amplification for popular communities.
- Harder to change feed algorithm — recomputing millions of feeds on algorithm change.

**Why fan-out-on-read works for v1:**
- User has N joined communities (typically 5-30). Query each community's recent posts, merge-sort, return top K.
- With 2 post shards, worst case is 2 parallel shard queries per feed load.
- Feed results cached in Redis per user (key: `feed:{user_id}:{sort}:{time_range}`, TTL: 2-5 minutes).
- Cache invalidated on: new post in joined community, vote on cached post, or TTL expiry.

**Implementation approach:**

```
ListHomeFeed(user_id, sort, cursor, limit):
  1. Check Redis cache: feed:{user_id}:{sort}:{time_range}:{cursor_hash}
  2. If cache hit → return cached page
  3. If cache miss:
     a. Get user's joined community IDs from community-service (gRPC call, cached in Redis 5min)
     b. Group community IDs by shard (consistent hash)
     c. Query each shard in parallel:
        SELECT * FROM posts WHERE community_id IN ($ids)
        ORDER BY {sort_column} {direction}
        LIMIT {limit * 2}  -- overfetch to allow merge
     d. Merge-sort results from all shards
     e. Apply cursor-based pagination to merged results
     f. Cache result page in Redis (2min TTL)
  4. Return page + next cursor
```

**Cursor encoding for cross-shard pagination:**
Extend the existing `pagination.EncodeCursor` to include the sort value (hot_score, created_at, etc.) alongside the post ID. On subsequent pages, each shard query uses `WHERE (sort_col, id) < ($cursor_sort, $cursor_id)` for stable keyset pagination.

**Performance characteristics (v1 scale):**
- 2 shards × 1 query each = 2 parallel DB queries (~5-10ms each)
- 1 community-service gRPC call (~2-5ms, cached)
- Merge-sort in memory (~0.1ms for 50 items)
- Total: ~15-20ms uncached, ~1ms cached
- Acceptable up to ~10K concurrent users before needing write-side caching

**When to switch to fan-out-on-write:**
If home feed latency exceeds 200ms uncached, or if >50% of feed requests are cache misses. This would be a v2 concern at 100K+ users.

### Post Sharding

**Recommendation: 2 logical PostgreSQL databases in the same PostgreSQL instance, with application-level routing via consistent hash ring**

**Why not separate PostgreSQL instances?**
- The architecture doc shows separate pg-post instances, but the current Docker Compose uses a single PostgreSQL instance with multiple databases (`init-databases.sql`).
- For v1 dev, separate instances add Docker Compose complexity with no benefit. Sharding logic is the same whether databases are on the same or different hosts.
- The application-level routing code is identical — just different connection strings.

**Approach:**

1. **Two databases**: `posts_shard_0` and `posts_shard_1` in the same PostgreSQL instance. Added to `init-databases.sql`.
2. **Shard router**: Consistent hash ring with virtual nodes mapping `community_id → shard_index`.
3. **Shard mapping table**: A `community_shards` table in the post-service's primary database (or the first shard) that records the actual shard assignment for each community. Hash determines initial placement; the table is the source of truth (per Pitfall 8 mitigation).
4. **Connection pool per shard**: Post service creates a `*pgxpool.Pool` for each shard database.

```go
// internal/post/shard.go
type ShardRouter struct {
    ring     *hashring.HashRing
    pools    map[string]*pgxpool.Pool  // shard_name -> pool
    shardIDs []string                   // ["shard_0", "shard_1"]
}

func (r *ShardRouter) GetPool(communityID string) *pgxpool.Pool {
    node, _ := r.ring.GetNode(communityID)
    return r.pools[node]
}

func (r *ShardRouter) AllPools() []*pgxpool.Pool {
    // For cross-shard queries (home feed)
    pools := make([]*pgxpool.Pool, 0, len(r.pools))
    for _, p := range r.pools {
        pools = append(pools, p)
    }
    return pools
}
```

**Community feed** (single community) is always a single-shard query — all posts for a community live on the same shard by design. This is the major benefit of sharding by `community_id`.

**Home feed** requires querying all shards (see aggregation section above).

**Schema per shard** (identical on each):

```sql
CREATE TABLE posts (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    title           TEXT        NOT NULL CHECK (char_length(title) <= 300),
    body            TEXT        NOT NULL DEFAULT '' CHECK (char_length(body) <= 40000),
    url             TEXT        NOT NULL DEFAULT '',
    post_type       SMALLINT    NOT NULL DEFAULT 1,  -- 1=text, 2=link, 3=media
    author_id       UUID        NOT NULL,
    author_username TEXT        NOT NULL,
    community_id    UUID        NOT NULL,
    community_name  TEXT        NOT NULL,
    vote_score      INTEGER     NOT NULL DEFAULT 0,
    upvotes         INTEGER     NOT NULL DEFAULT 0,
    downvotes       INTEGER     NOT NULL DEFAULT 0,
    comment_count   INTEGER     NOT NULL DEFAULT 0,
    hot_score       DOUBLE PRECISION NOT NULL DEFAULT 0,
    is_edited       BOOLEAN     NOT NULL DEFAULT false,
    is_deleted      BOOLEAN     NOT NULL DEFAULT false,
    is_pinned       BOOLEAN     NOT NULL DEFAULT false,
    is_anonymous    BOOLEAN     NOT NULL DEFAULT false,
    thumbnail_url   TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at       TIMESTAMPTZ
);

-- Community feed (single shard) — all sort orders
CREATE INDEX idx_posts_community_hot ON posts (community_id, hot_score DESC, id DESC)
    WHERE is_deleted = false;
CREATE INDEX idx_posts_community_new ON posts (community_id, created_at DESC, id DESC)
    WHERE is_deleted = false;
CREATE INDEX idx_posts_community_top ON posts (community_id, vote_score DESC, id DESC)
    WHERE is_deleted = false;

-- Home feed cross-shard queries need efficient community_id IN (...) filtering
CREATE INDEX idx_posts_community_id ON posts (community_id);

-- Author's posts (for profile page, cross-shard)
CREATE INDEX idx_posts_author ON posts (author_id, created_at DESC);
```

**Saved posts table** (in the first shard or a separate table):

```sql
CREATE TABLE saved_posts (
    user_id    UUID NOT NULL,
    post_id    UUID NOT NULL,
    saved_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, post_id)
);

CREATE INDEX idx_saved_posts_user ON saved_posts (user_id, saved_at DESC);
```

Note: Saved posts only store `post_id` references. When listing saved posts, the service fetches the actual post data from the appropriate shard. This avoids data duplication but requires shard-aware fetching.

### Hot Ranking Algorithm

**Recommendation: Lemmy's Hot algorithm, precomputed and stored in a `hot_score` column**

**Formula (from FEATURES.md research):**

```
rank = scale_factor * log(max(1, 3 + score)) / (time_hours + 2) ^ gravity
```

Where:
- `score` = `upvotes - downvotes` (net vote score)
- `time_hours` = hours since post creation
- `gravity` = 1.8 (default decay rate, configurable)
- `scale_factor` = 10000 (to produce sort-friendly integer-range values)

**Go implementation:**

```go
func HotScore(score int, createdAt time.Time) float64 {
    const gravity = 1.8
    const scaleFactor = 10000.0

    hoursAge := time.Since(createdAt).Hours()
    numerator := scaleFactor * math.Log(math.Max(1, float64(3+score)))
    denominator := math.Pow(hoursAge+2, gravity)
    return numerator / denominator
}
```

**Computation strategy: Precompute on write + periodic refresh**

1. **On post creation:** Compute initial `hot_score` (score=0 → `log(max(1,3)) / 2^1.8 ≈ 0.315`). Store in `hot_score` column.
2. **On vote:** Kafka consumer in post-service receives vote event → recompute `hot_score` with new score → `UPDATE posts SET hot_score = $1, vote_score = $2 WHERE id = $3`.
3. **Periodic refresh:** A background goroutine in post-service runs every 15 minutes: `UPDATE posts SET hot_score = [formula] WHERE created_at > now() - interval '48 hours' AND is_deleted = false`. This handles the time-decay component for posts that aren't receiving votes.

**Why precompute (not compute-on-query)?**
- Computing `hot_score` for every post on every feed request is expensive (log, pow operations).
- With a precomputed column, the community feed query is a simple `ORDER BY hot_score DESC` using the index.
- The periodic refresh ensures time decay is reflected even without new votes.
- 48-hour window for refresh because posts older than ~48h with no new votes are effectively off the Hot feed anyway.

**Other sort orders:**
- **New**: `ORDER BY created_at DESC` — trivial, just the index.
- **Top**: `ORDER BY vote_score DESC` — trivial with index. Time range filter: `WHERE created_at > now() - interval '1 hour/day/week/...'`.
- **Rising**: `vote_score / max(1, hours_since_creation)` — velocity. Can be computed on query for the small result set (rising only cares about recent posts, last 12-24h). No need to precompute.

### Kafka Setup for Karma

**Recommendation: Single Kafka broker in KRaft mode, franz-go producer in vote-service, franz-go consumer in post-service and user-service**

**Docker Compose addition:**

```yaml
kafka:
  image: bitnami/kafka:3.7
  ports: ["9092:9092"]
  environment:
    KAFKA_CFG_NODE_ID: 1
    KAFKA_CFG_PROCESS_ROLES: controller,broker
    KAFKA_CFG_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
    KAFKA_CFG_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
    KAFKA_CFG_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
    KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT
    KAFKA_CFG_CONTROLLER_LISTENER_NAMES: CONTROLLER
    KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE: "true"
  healthcheck:
    test: ["CMD-SHELL", "kafka-topics.sh --bootstrap-server localhost:9092 --list || exit 1"]
    interval: 10s
    timeout: 10s
    retries: 5
    start_period: 30s
```

**Topic: `redyx.votes.v1`**

- Partitions: 6 (sufficient for v1; allows up to 6 parallel consumers per group)
- Key: `target_id` (post or comment ID) — ensures all votes for one post land on the same partition for ordered processing
- Retention: 7 days
- Consumer groups:
  - `post-service.redyx.votes.v1` — updates `vote_score` and `hot_score` on posts
  - `user-service.redyx.votes.v1` — updates author karma

**Vote event message (protobuf-serialized):**

```protobuf
// proto/redyx/common/v1/events.proto (new file)
message VoteEvent {
  string event_id = 1;           // UUID for deduplication
  string user_id = 2;            // Voter
  string target_id = 3;          // Post or comment ID
  string target_type = 4;        // "post" or "comment"
  string author_id = 5;          // Author of the voted-on content (for karma)
  int32 score_delta = 6;         // -2, -1, 0, +1, +2 (net change)
  string community_id = 7;       // For shard routing in post-service consumer
  google.protobuf.Timestamp occurred_at = 8;
}
```

**Producer (vote-service):**

```go
// After recording vote in Redis:
event := &commonv1.VoteEvent{
    EventId:    uuid.New().String(),
    UserId:     claims.UserID,
    TargetId:   req.TargetId,
    TargetType: "post",
    AuthorId:   authorID,  // looked up from Redis cache or post data
    ScoreDelta: delta,      // computed from old vote vs new vote
    CommunityId: communityID,
    OccurredAt: timestamppb.Now(),
}
producer.Publish(ctx, "redyx.votes.v1", req.TargetId, event)
```

Use async produce (fire-and-forget with franz-go's default behavior) — don't block the vote API response on Kafka write. The vote state is already in Redis; Kafka is for eventual consistency.

**Consumer (post-service) — idempotent score update:**

The consumer receives vote events and updates the post's `vote_score` and `hot_score`. Idempotency is guaranteed because:
1. The vote-service sends the **net delta** (not the absolute score).
2. The post-service consumer uses `UPDATE posts SET vote_score = vote_score + $delta, upvotes = upvotes + $up_delta, downvotes = downvotes + $down_delta WHERE id = $id`.
3. Duplicate events are acceptable because the delta is idempotent when computed from the vote state change (e.g., upvote→downvote = delta of -2, not "set score to X").

**Wait — delta-based updates are NOT idempotent.** If the same event is processed twice, the score is updated twice. The architecture research explicitly warns against this (Pitfall 7).

**Corrected approach: Set-based score derivation**

The vote-service stores the authoritative vote state in Redis as sets:
- `votes:up:{target_id}` — set of user_ids who upvoted
- `votes:down:{target_id}` — set of user_ids who downvoted

The Kafka consumer in post-service does NOT apply deltas. Instead, it receives the event as a trigger to recalculate from Redis:

```go
// Consumer: on vote event
upCount, _ := redis.SCard(ctx, fmt.Sprintf("votes:up:%s", event.TargetId))
downCount, _ := redis.SCard(ctx, fmt.Sprintf("votes:down:%s", event.TargetId))
newScore := int(upCount) - int(downCount)
// UPDATE posts SET vote_score = $newScore, upvotes = $upCount, downvotes = $downCount WHERE id = $id
```

This is naturally idempotent — processing the same event twice yields the same score.

**Consumer (user-service) — karma update:**

Karma updates are trickier because they're cumulative across all of a user's posts. The user-service consumer:
1. Receives vote event with `author_id` and `score_delta`.
2. Uses a Redis set to track which events have been processed: `SADD karma:processed:{author_id} {event_id}`.
3. If SADD returns 1 (new), apply delta: `UPDATE users SET karma = karma + $delta WHERE id = $author_id`.
4. If SADD returns 0 (duplicate), skip.
5. TTL on the processed set: 24h (events older than 24h won't be replayed).

This is VOTE-04's "set-based idempotent processing."

**Config additions:**

```go
// internal/platform/config/config.go additions
type Config struct {
    // ... existing fields ...

    // Kafka fields (Phase 3)
    KafkaBrokers string // comma-separated, e.g., "kafka:9092"
}
```

### Vote State Management

**Recommendation: Redis as primary vote state store, PostgreSQL for durability via Kafka, score updates within 500ms via Redis INCRBY**

**Vote flow (VOTE-01 through VOTE-05):**

```
1. Client POSTs /api/v1/votes { targetId, targetType, direction }
2. Vote-service:
   a. Get current vote: GET votes:state:{user_id}:{target_id}
      - Returns "up", "down", or nil (no vote)
   b. Compute delta:
      - nil → up:     delta = +1
      - nil → down:   delta = -1
      - up  → down:   delta = -2
      - down → up:    delta = +2
      - up  → none:   delta = -1
      - down → none:  delta = +1
      - same → same:  delta = 0 (idempotent, VOTE-05)
   c. If delta == 0: return current score (no-op, idempotent)
   d. Update vote state atomically (Lua script):
      - If direction == none: DEL votes:state:{user_id}:{target_id}
        + SREM votes:up:{target_id} {user_id} or SREM votes:down:{target_id} {user_id}
      - If direction == up: SET votes:state:{user_id}:{target_id} "up"
        + SADD votes:up:{target_id} {user_id}
        + SREM votes:down:{target_id} {user_id}
      - If direction == down: SET votes:state:{user_id}:{target_id} "down"
        + SADD votes:down:{target_id} {user_id}
        + SREM votes:up:{target_id} {user_id}
      - INCRBY votes:score:{target_id} {delta}
   e. GET votes:score:{target_id} → return new_score in response
   f. Publish VoteEvent to Kafka (async, non-blocking)
3. Response: { newScore: 42 } — within <50ms, well under 500ms (VOTE-03)
```

**Redis key patterns (vote-service, Redis DB 4):**

| Key | Type | Purpose | TTL |
|-----|------|---------|-----|
| `votes:state:{user_id}:{target_id}` | STRING | User's current vote direction ("up"/"down") | None (permanent) |
| `votes:up:{target_id}` | SET | All user_ids who upvoted this item | None |
| `votes:down:{target_id}` | SET | All user_ids who downvoted this item | None |
| `votes:score:{target_id}` | STRING (integer) | Current net score (upvotes - downvotes) | None |

**Lua script for atomicity (VOTE-02 enforcement):**

The vote state update (step 2d) MUST be a single Lua script to prevent race conditions where two concurrent votes from the same user could both succeed. The Lua script checks current state and applies the change atomically.

**PostgreSQL persistence:**

Vote-service does NOT write to PostgreSQL directly. The Kafka consumers in post-service and user-service handle durable storage:
- Post-service consumer: Updates `posts.vote_score`, `posts.upvotes`, `posts.downvotes`, `posts.hot_score`
- User-service consumer: Updates `users.karma`

If Redis data is lost, scores can be reconstructed from the Kafka event log (replay from offset 0). This is acceptable for v1 — Redis persistence (`save 60 1`) provides sufficient durability for dev.

**Vote-service database:**

The vote-service itself needs minimal PostgreSQL storage. Per the architecture doc, votes persist in `pg-platform`. However, for v1 simplicity, the vote-service can be Redis-only for vote state (the sets ARE the source of truth) with Kafka as the durability layer. If needed later, a `votes` table in pg-platform can store the vote log for audit/analytics.

**Redis DB assignment:**

Following the existing pattern (auth=1, user=2, community=3), assign:
- post-service: Redis DB 4
- vote-service: Redis DB 5

## Frontend Patterns

### Markdown Rendering

**Recommendation: `marked` + `DOMPurify` for client-side rendering**

**Why client-side, not server-side?**
- Text post bodies are user content that changes (edits). Rendering on the server would require re-rendering cached content on every view.
- Client-side rendering with `marked` is fast (~1ms for 40KB text).
- Preview tab in the post creation form requires client-side rendering anyway.

**Implementation:**

```svelte
<!-- PostBody.svelte -->
<script lang="ts">
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';

  interface Props {
    body: string;
  }

  let { body }: Props = $props();

  let html = $derived(DOMPurify.sanitize(marked.parse(body)));
</script>

<div class="prose prose-terminal font-mono text-sm">
  {@html html}
</div>
```

**marked configuration:**

```ts
marked.setOptions({
  breaks: true,       // GFM line breaks
  gfm: true,          // GitHub Flavored Markdown
  headerIds: false,    // No auto-generated IDs (security)
});
```

**Styling:** Use Tailwind's `@apply` in a `prose-terminal` class to style markdown output with the terminal aesthetic (monospace code blocks, subdued headings, accent-colored links).

**Bundle size:** marked (~40KB) + DOMPurify (~15KB) = ~55KB total gzipped. Acceptable for a Svelte island that's only loaded on pages with markdown content.

### Optimistic Voting

**Recommendation: Component-local `$state` with immediate update and async API call with rollback on failure**

**Pattern:**

```svelte
<!-- VoteButtons.svelte -->
<script lang="ts">
  import { api, ApiError } from '../lib/api';
  import { isAuthenticated } from '../lib/auth';

  interface Props {
    postId: string;
    initialScore: number;
    initialVote: number; // -1, 0, 1
  }

  let { postId, initialScore, initialVote }: Props = $props();

  let score = $state(initialScore);
  let userVote = $state(initialVote); // -1=down, 0=none, 1=up

  function formatScore(n: number): string {
    if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'm';
    if (n >= 1_000) return (n / 1_000).toFixed(1) + 'k';
    return n.toString();
  }

  async function vote(direction: 'up' | 'down') {
    if (!isAuthenticated()) {
      // Show login prompt
      return;
    }

    const prevScore = score;
    const prevVote = userVote;

    // Compute new state optimistically
    const newDirection = (direction === 'up' && userVote === 1) || (direction === 'down' && userVote === -1)
      ? 0  // Toggle off
      : (direction === 'up' ? 1 : -1);

    // Apply optimistic update
    score = prevScore + (newDirection - prevVote);
    userVote = newDirection;

    try {
      const directionEnum = newDirection === 1 ? 'VOTE_DIRECTION_UP'
        : newDirection === -1 ? 'VOTE_DIRECTION_DOWN'
        : 'VOTE_DIRECTION_NONE';

      const res = await api<{ newScore: number }>('/votes', {
        method: 'POST',
        body: JSON.stringify({
          targetId: postId,
          targetType: 'TARGET_TYPE_POST',
          direction: directionEnum,
        }),
      });

      // Reconcile with server score (in case of concurrent votes)
      score = res.newScore;
    } catch (e) {
      // Rollback on failure
      score = prevScore;
      userVote = prevVote;

      if (e instanceof ApiError && e.status === 401) {
        // Session expired
      }
    }
  }
</script>

<div class="flex flex-col items-center w-10 shrink-0 text-terminal-dim">
  <button
    onclick={() => vote('up')}
    class={userVote === 1 ? 'text-accent-500' : 'hover:text-accent-500'}
  >&#9650;</button>
  <span class={userVote === 1 ? 'text-accent-500' : userVote === -1 ? 'text-red-500' : 'text-terminal-fg'}>
    {formatScore(score)}
  </span>
  <button
    onclick={() => vote('down')}
    class={userVote === -1 ? 'text-red-500' : 'hover:text-red-500'}
  >&#9660;</button>
</div>
```

**Key design decisions:**
- Score and vote state are component-local `$state`, not a global store. Each post card owns its own vote state. This prevents cross-component interference.
- Optimistic update is synchronous: `score` and `userVote` change before the API call.
- Server response reconciles the score (handles concurrent votes from other users).
- On failure, state rolls back to the exact previous values.
- `formatScore` handles the compact number display (CONTEXT.md: "exact up to 999, then 1.4k, 15.8k, 1.2m").

### Infinite Scroll

**Note:** REQUIREMENTS.md explicitly lists infinite scroll as out-of-scope, but the Phase 3 CONTEXT.md (user's latest decisions) specifies "Infinite scroll for pagination (auto-load as user approaches bottom)." The CONTEXT.md represents the user's Phase 3 decision, so we implement it — but use a cursor-based approach that doesn't break the back button.

**Recommendation: Intersection Observer + cursor-based auto-loading**

```svelte
<!-- FeedList.svelte -->
<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';

  interface Props {
    endpoint: string;  // e.g., "/communities/programming/posts" or "/feed"
    sort: string;
    timeRange?: string;
  }

  let { endpoint, sort, timeRange }: Props = $props();

  type Post = { /* ... */ };

  let posts = $state<Post[]>([]);
  let nextCursor = $state<string | null>(null);
  let hasMore = $state(true);
  let loading = $state(false);
  let initialLoading = $state(true);
  let sentinel: HTMLElement;

  async function loadPage() {
    if (loading || !hasMore) return;
    loading = true;

    try {
      const params = new URLSearchParams({ 'pagination.limit': '25' });
      if (nextCursor) params.set('pagination.cursor', nextCursor);
      if (sort) params.set('sort', sort);
      if (timeRange) params.set('timeRange', timeRange);

      const res = await api<{
        posts: Post[];
        pagination: { nextCursor: string; hasMore: boolean };
      }>(`${endpoint}?${params}`);

      posts = [...posts, ...(res.posts ?? [])];
      nextCursor = res.pagination?.nextCursor ?? null;
      hasMore = res.pagination?.hasMore ?? false;
    } catch {
      hasMore = false;
    } finally {
      loading = false;
      initialLoading = false;
    }
  }

  onMount(() => {
    loadPage();

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && hasMore && !loading) {
          loadPage();
        }
      },
      { rootMargin: '200px' }  // Trigger 200px before sentinel is visible
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  });
</script>

{#if initialLoading}
  <div class="text-xs text-terminal-dim animate-pulse p-4">[loading feed...]</div>
{:else if posts.length === 0}
  <div class="text-xs text-terminal-dim p-4">no posts yet</div>
{:else}
  {#each posts as post (post.postId)}
    <FeedRow {post} />
  {/each}
{/if}

{#if loading && !initialLoading}
  <div class="text-xs text-terminal-dim animate-pulse p-2">[loading more...]</div>
{/if}

<div bind:this={sentinel} class="h-1"></div>
```

**Back button concern:** Since we're using cursor-based pagination (not page numbers), scrolling back to a previous position is handled by the browser's scroll restoration. The URL doesn't change during infinite scroll — only on sort change or navigation. This is acceptable for the terminal aesthetic where users primarily scroll forward.

**Reset on sort change:** When the user changes sort order or time range, the component should reset: `posts = []; nextCursor = null; hasMore = true; loadPage()`. Use `$effect` to watch for sort/timeRange prop changes.

### Feed Row Component

```svelte
<!-- FeedRow.svelte (compact terminal style matching index.astro mockup) -->
<div class="flex items-center gap-3 px-2 py-1.5 border-b border-terminal-border
            hover:bg-terminal-surface transition-colors text-xs font-mono group">
  <VoteButtons postId={post.postId} initialScore={post.voteScore} initialVote={post.userVote ?? 0} />

  <div class="flex-1 min-w-0">
    <a href="/post/{post.postId}" class="text-terminal-fg group-hover:text-accent-500 transition-colors truncate block text-sm">
      {post.title}
      {#if post.postType === 'POST_TYPE_LINK'}
        <span class="text-terminal-dim text-xs ml-1">({new URL(post.url).hostname})</span>
      {/if}
    </a>
    <div class="text-terminal-dim mt-0.5">
      <a href="/community/{post.communityName}" class="text-accent-600 hover:text-accent-500">
        r/{post.communityName}
      </a>
      <span class="mx-1">&middot;</span>
      <span>{post.isAnonymous ? '[anonymous]' : `u/${post.authorUsername}`}</span>
      <span class="mx-1">&middot;</span>
      <span>{relativeTime(post.createdAt)}</span>
      <span class="mx-1">&middot;</span>
      <span>{post.commentCount} comments</span>
    </div>
  </div>
</div>
```

## Don't Hand-Roll

1. **Consistent hashing**: Use `serialx/hashring` — don't implement your own hash ring. Virtual node support is critical for even distribution.
2. **Kafka client**: Use `franz-go` — don't use raw TCP or a wrapper. franz-go handles consumer group rebalancing, offset commits, and partition assignment correctly.
3. **Markdown rendering**: Use `marked` — don't write a markdown parser. Markdown is deceptively complex (edge cases in nested lists, code fences, etc.).
4. **HTML sanitization**: Use `DOMPurify` — absolutely do not attempt to sanitize HTML with regex or string replacement. XSS prevention requires a proper DOM parser.
5. **Intersection Observer**: Use the browser's native `IntersectionObserver` API — don't use scroll event listeners with manual offset calculations (performance nightmare).
6. **URL validation**: Use Go's `net/url.Parse()` — don't regex-validate URLs. URLs are complex (internationalized domains, ports, fragments, query strings).
7. **Cursor encoding**: Use the existing `pagination.EncodeCursor/DecodeCursor` from `internal/platform/pagination/` — it already handles base64 encoding with id+timestamp pairs.
8. **Lua scripts for Redis atomicity**: Use Redis Lua scripts for multi-key vote operations — don't use MULTI/EXEC transactions (they don't work across keys in Redis Cluster, and even in standalone they have WATCH race conditions).

## Common Pitfalls

### 1. Kafka Consumer Not Starting (Silent Failure)
**Problem:** franz-go consumers with `auto.create.topics.enable=false` on the broker will silently wait forever if the topic doesn't exist.
**Fix:** Use `kadm` to create topics on service startup before starting consumers. Or ensure `KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE=true` in Docker Compose (already set in our config).

### 2. Hot Score Staleness
**Problem:** If the periodic hot_score refresh goroutine doesn't run (crashed, long GC pause), feed ordering becomes increasingly stale.
**Fix:** Always recalculate hot_score when processing vote events. The periodic refresh is a safety net for time-decay, not the primary mechanism. Monitor with a metric: `post_service_hot_score_refresh_last_run_timestamp`.

### 3. Redis Vote State vs PostgreSQL Score Drift
**Problem:** If Kafka consumers fall behind, the Redis score (updated synchronously on vote) diverges from the PostgreSQL score (updated asynchronously via Kafka). Feed sorting uses PostgreSQL; vote display uses Redis.
**Fix:** Accept eventual consistency. The Redis score is always correct for display (it's updated atomically on every vote). The PostgreSQL score may lag by seconds — acceptable for feed sorting. Add a reconciliation job that runs hourly: compare Redis scores with PostgreSQL scores for recently active posts.

### 4. Cross-Shard Cursor Instability
**Problem:** When paginating across shards, a cursor that encodes a sort value + id from one shard may not produce correct results on the next request if the sort values change between requests (e.g., hot scores update).
**Fix:** For Hot sort, the cursor encodes `(hot_score, post_id)`. Between requests, hot_score may change slightly due to time decay — but this only affects borderline posts. Acceptable for UX. For New and Top sorts, the cursor values (created_at, vote_score) are stable enough.

### 5. Kafka Topic Auto-Creation with Wrong Partition Count
**Problem:** `auto.create.topics.enable=true` creates topics with the broker's default partition count (1), not the desired count (6).
**Fix:** Create topics explicitly on first service startup using `kadm.CreateTopics()` with desired partition count. Auto-create is a fallback, not the primary mechanism.

### 6. Shard Router Initialization Race
**Problem:** Post-service starts and tries to route a request before the shard router has loaded the community→shard mapping from the database.
**Fix:** Load the shard mapping during service initialization (before gRPC server starts accepting requests). Use the consistent hash ring for communities not yet in the mapping table (new communities).

### 7. Anonymous Post Author ID Leaking
**Problem:** POST-08 requires anonymous posts to display `[anonymous]` but mods can see the real author. If `author_id` is always populated in the Post proto response, the frontend could expose it.
**Fix:** In the `GetPost` and `ListPosts` RPCs, if `is_anonymous` is true AND the requesting user is NOT a moderator of the community, set `author_id = ""` and `author_username = "[anonymous]"` in the response. The database always stores the real author_id.

### 8. Envoy Route Ordering for New Services
**Problem:** Adding `/api/v1/posts/` and `/api/v1/votes/` routes. If `/api/v1/posts/{post_id}/save` matches the catch-all before the post-service route, saves go to the wrong backend.
**Fix:** Add specific routes BEFORE the catch-all (existing pattern). Post-service routes: `/api/v1/posts`, `/api/v1/communities/` (for ListPosts), `/api/v1/feed`, `/api/v1/saved`. Vote-service routes: `/api/v1/votes`.

### 9. Infinite Scroll Memory Accumulation
**Problem:** As the user scrolls, `posts` array grows unbounded. After 500+ posts, DOM node count causes jank.
**Fix:** For v1, accept this limitation (most users won't scroll past 100-200 posts). For v2, implement virtualized scrolling or windowing. The terminal aesthetic's compact rows (1-2 lines per post) mean ~50 rows visible at a time, so 500 posts = 500 DOM nodes — still manageable.

## Open Questions

### 1. Vote-Service Database: Redis-Only or Redis + PostgreSQL?
The architecture doc places vote persistence in `pg-platform`. The research above recommends Redis as the sole source of truth for vote state (with Kafka for durability). **Decision needed:** Should we add a `votes` table in pg-platform for persistence, or rely on Redis + Kafka replay? Redis-only is simpler but riskier (Redis crash before Kafka publish = lost vote). Recommendation: Redis-only for v1, add PG persistence if Redis durability proves insufficient.

### 2. Post-Service Inter-Service Calls: gRPC or Database Lookup?
When creating a post, the post-service needs to verify the community exists and the user is a member. Options:
- **A) gRPC call to community-service** (architecturally clean, but adds latency + dependency)
- **B) Cache community data in post-service Redis** (faster, but stale data risk)
- Recommendation: **A** with Redis caching on the post-service side (cache community membership for 5min). Follow the pattern from the architecture doc.

### 3. User's Vote State in Feed Responses
The `GetPostResponse` includes `user_vote` (user's vote on that post). But `ListPostsResponse` and `ListHomeFeedResponse` only return `repeated Post` — no user vote state. Should we:
- **A) Add `user_vote` to the `Post` message** (changes proto, adds per-post Redis lookup)
- **B) Batch-fetch user votes client-side after feed loads** (separate API call)
- **C) Add a `map<string, int32> user_votes` field to list responses** (map of post_id → vote direction)
- Recommendation: **C** — a parallel field in the list response. The vote-service can batch-lookup user votes for multiple post IDs in one Redis MGET. This avoids N+1 and doesn't change the Post message.

### 4. Post Shard Count: 2 or 4?
STATE.md lists this as an open question. With 2 shards:
- Simpler configuration, fewer connection pools
- Sufficient for v1 (thousands of communities)
- Can add more shards later with migration

With 4 shards:
- Better distribution from the start
- More parallel query capacity for home feed
- Harder to manage in development

Recommendation: **2 shards for v1.** The consistent hash ring makes adding shards a controlled operation later. Starting with more shards than needed adds complexity with no benefit at v1 scale.

### 5. REQUIREMENTS.md vs CONTEXT.md: Infinite Scroll
REQUIREMENTS.md "Out of Scope" table explicitly excludes infinite scroll. CONTEXT.md for Phase 3 specifies "Infinite scroll for pagination." These contradict. The CONTEXT.md is the user's latest Phase 3 decision, so the implementation plan should follow CONTEXT.md (infinite scroll with cursor-based loading). The planner should confirm this with the user or update REQUIREMENTS.md.

---

*Phase: 03-posts-voting-feeds*
*Research completed: 2026-03-03*
