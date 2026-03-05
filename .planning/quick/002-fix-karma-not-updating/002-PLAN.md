---
phase: quick
plan: 002
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/vote/consumer.go
  - cmd/user/main.go
  - docker-compose.yml
autonomous: false
requirements: [QUICK-002]
must_haves:
  truths:
    - "Voting on a post increments the post author's karma in the profiles table"
    - "Duplicate vote events do not double-count karma (Redis SADD dedup preserved)"
    - "Karma consumer handles events with empty author_id by looking up from post DB"
  artifacts:
    - path: "internal/vote/consumer.go"
      provides: "Karma consumer with post-shard author lookup and correct SQL"
    - path: "cmd/user/main.go"
      provides: "User service wiring post shard pools into karma consumer"
    - path: "docker-compose.yml"
      provides: "POST_SHARD_DSNS env var for user-service container"
  key_links:
    - from: "internal/vote/consumer.go"
      to: "profiles table"
      via: "UPDATE profiles SET karma = karma + $1 WHERE user_id = $2"
      pattern: "UPDATE profiles.*karma.*WHERE user_id"
    - from: "internal/vote/consumer.go"
      to: "post shard DBs"
      via: "SELECT author_id FROM posts WHERE id = $1"
      pattern: "SELECT author_id FROM posts"
---

<objective>
Fix karma never updating — user profiles show karma: 0 despite voting activity.

**Root cause (3-link bug chain):**
1. Vote-service publishes VoteEvent with `AuthorId: ""` (line 84 of server.go) — comment says "Consumer looks up from post data" but the karma consumer never does this lookup
2. Karma consumer skips all events where `GetAuthorId() == ""` (line 92 of consumer.go) — so every event is skipped
3. Even if author_id were populated, the SQL is wrong: `UPDATE users SET karma = karma + $1 WHERE id = $2` — table is `profiles`, column is `user_id`

**Fix strategy:** Keep vote RPC fast (per decision [03-02] — fire-and-forget). Fix the karma consumer to:
- Look up `author_id` from post shard DBs when the event has empty author_id
- Use correct table name (`profiles`) and column name (`user_id`) in the UPDATE SQL

Purpose: Users see their karma reflect voting activity on their posts
Output: Working karma pipeline from vote → Kafka → karma consumer → profiles.karma
</objective>

<execution_context>
@./.opencode/get-shit-done/workflows/execute-plan.md
@./.opencode/get-shit-done/templates/summary.md
</execution_context>

<context>
@internal/vote/consumer.go (karma consumer — has the bugs)
@internal/vote/server.go (vote RPC — publishes empty author_id)
@internal/vote/kafka.go (Kafka producer — working correctly)
@cmd/user/main.go (user-service entry — wires karma consumer)
@docker-compose.yml (service env vars)
@migrations/user/001_profiles.up.sql (correct table schema: `profiles`, PK `user_id`)
@migrations/post_shard_0/001_create_posts.up.sql (post schema has `author_id` column)
@internal/platform/config/config.go (Config struct already has PostShardDSNs field)

<interfaces>
<!-- Key contracts the executor needs -->

From internal/vote/consumer.go — current broken KarmaConsumer:
```go
type KarmaConsumer struct {
    client *kgo.Client
    rdb    *redis.Client
    db     *pgxpool.Pool    // user_profiles DB
    logger *zap.Logger
}

func NewKarmaConsumer(brokers []string, rdb *redis.Client, db *pgxpool.Pool, logger *zap.Logger) (*KarmaConsumer, error)
```

From internal/platform/config/config.go:
```go
type Config struct {
    // ...
    PostShardDSNs []string  // Already exists in config, loaded from POST_SHARD_DSNS env var
    // ...
}
```

From migrations/user/001_profiles.up.sql — actual table:
```sql
CREATE TABLE profiles (
    user_id      UUID        PRIMARY KEY,
    username     TEXT        NOT NULL,
    karma        INTEGER     NOT NULL DEFAULT 0,
    ...
);
```

From migrations/post_shard_0/001_create_posts.up.sql:
```sql
CREATE TABLE posts (
    id              UUID        PRIMARY KEY,
    author_id       UUID        NOT NULL,
    ...
);
```

From gen/redyx/common/v1 — VoteEvent proto:
```go
type VoteEvent struct {
    EventId    string
    UserId     string
    TargetId   string
    TargetType string   // "post" or "comment"
    AuthorId   string   // EMPTY when published by vote-service
    ScoreDelta int32
    // ...
}
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix karma consumer — add post-shard author lookup and correct SQL</name>
  <files>internal/vote/consumer.go, cmd/user/main.go, docker-compose.yml</files>
  <action>
**Fix 1 — Add post shard pools to KarmaConsumer (internal/vote/consumer.go):**

Update the `KarmaConsumer` struct to include post shard database pools for author lookup:

```go
type KarmaConsumer struct {
    client     *kgo.Client
    rdb        *redis.Client
    db         *pgxpool.Pool      // user_profiles DB
    postShards []*pgxpool.Pool    // post shard DBs for author_id lookup
    logger     *zap.Logger
}
```

Update `NewKarmaConsumer` signature to accept post shard pools:
```go
func NewKarmaConsumer(brokers []string, rdb *redis.Client, db *pgxpool.Pool, postShards []*pgxpool.Pool, logger *zap.Logger) (*KarmaConsumer, error)
```

Store `postShards` in the returned struct.

**Fix 2 — Add lookupAuthorID method (internal/vote/consumer.go):**

Add a method that queries all post shards to find the author of a post:

```go
func (c *KarmaConsumer) lookupAuthorID(ctx context.Context, postID string) string {
    for _, pool := range c.postShards {
        var authorID string
        err := pool.QueryRow(ctx, `SELECT author_id FROM posts WHERE id = $1`, postID).Scan(&authorID)
        if err == nil && authorID != "" {
            return authorID
        }
    }
    return ""
}
```

**Fix 3 — Update processEvent to look up author when missing (internal/vote/consumer.go):**

In `processEvent`, after getting `authorID := event.GetAuthorId()`, add a fallback:

```go
if authorID == "" {
    authorID = c.lookupAuthorID(ctx, event.GetTargetId())
    if authorID == "" {
        return nil // Cannot determine author — skip silently
    }
}
```

Remove the early return at lines 91-95 that skips events with empty author_id. Replace it with the lookup above inside `processEvent`.

**Fix 4 — Fix the SQL table and column names (internal/vote/consumer.go):**

Change line 153-154 from:
```go
`UPDATE users SET karma = karma + $1 WHERE id = $2`,
```
to:
```go
`UPDATE profiles SET karma = karma + $1 WHERE user_id = $2`,
```

**Fix 5 — Wire post shard pools in user-service main (cmd/user/main.go):**

After loading config and connecting to user DB, connect to post shard pools:

```go
// Connect to post shard databases for karma consumer author lookups
var postShards []*pgxpool.Pool
for _, dsn := range cfg.PostShardDSNs {
    pool, err := database.NewPostgres(ctx, dsn)
    if err != nil {
        logger.Warn("failed to connect to post shard, karma author lookup may be limited", zap.Error(err))
        continue
    }
    postShards = append(postShards, pool)
}
// defer close post shard pools (add a defer for each)
```

Pass `postShards` to `vote.NewKarmaConsumer`:
```go
kc, err := vote.NewKarmaConsumer(brokers, rdb, db, postShards, logger)
```

Add deferred cleanup for post shard pools before the Kafka consumer startup block.

**Fix 6 — Add POST_SHARD_DSNS to user-service in docker-compose.yml:**

In the `user-service` environment section, add:
```yaml
POST_SHARD_DSNS: "postgres://redyx:dev@postgres:5432/posts_shard_0?sslmode=disable,postgres://redyx:dev@postgres:5432/posts_shard_1?sslmode=disable"
```

**Also add `KarmaConsumer.Close()` to close post shard pools** — update the Close method to also close post shard pool connections, or handle it in main.go's defer block.
  </action>
  <verify>
    <automated>cd /home/idityage/github_repos/reddit && go build ./cmd/user/... && go vet ./internal/vote/... ./cmd/user/...</automated>
  </verify>
  <done>
- KarmaConsumer accepts post shard pools and looks up author_id from post DBs when event has empty author_id
- SQL correctly targets `profiles` table with `user_id` column
- user-service main.go connects to post shard DBs and passes them to KarmaConsumer
- docker-compose.yml provides POST_SHARD_DSNS to user-service
- Code compiles and passes vet
  </done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 2: Verify karma updates end-to-end</name>
  <files>none</files>
  <action>
  Human verifies the karma pipeline works end-to-end:
  - what-built: Fixed karma consumer pipeline — author lookup from post shards + correct SQL table/column targeting profiles.user_id
  - how-to-verify:
    1. Run `docker compose up -d --build user-service vote-service post-service kafka redis postgres`
    2. Wait for services to start: `docker compose logs user-service --tail 20` — look for "karma consumer started"
    3. Log in as a user (user A) and create a post in any community
    4. Log in as a different user (user B) and upvote user A's post
    5. Check user A's profile page — karma should show 1 (not 0)
    6. User B downvotes then re-upvotes — karma should still be 1 (dedup works)
    7. Check docker logs: `docker compose logs user-service --tail 50 | grep karma` — should show processing, not skipping
  - resume-signal: Type "approved" or describe issues
  </action>
  <verify>Human confirms karma > 0 on profile after receiving votes</verify>
  <done>Karma updates visible on user profile after voting activity</done>
</task>

</tasks>

<verification>
- `go build ./cmd/user/...` compiles successfully
- `go vet ./internal/vote/... ./cmd/user/...` passes
- Karma consumer SQL targets `profiles` table with `user_id` column (grep confirms)
- KarmaConsumer struct includes `postShards []*pgxpool.Pool` field
- docker-compose.yml user-service has POST_SHARD_DSNS env var
- No regression in existing vote score pipeline (ScoreConsumer untouched)
</verification>

<success_criteria>
- Voting on a post causes the post author's karma to increment in the profiles table
- User profile page reflects non-zero karma after receiving votes
- Karma consumer logs show events being processed (not skipped)
- Deduplication still works (Redis SADD pattern preserved)
</success_criteria>

<output>
After completion, create `.planning/quick/002-fix-karma-not-updating/002-SUMMARY.md`
</output>
