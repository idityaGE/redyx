# Pitfalls Research

**Domain:** Go microservice platform (anonymous Reddit clone) — gRPC + Envoy + ScyllaDB + Kafka + Redis + PostgreSQL
**Researched:** 2026-03-02
**Confidence:** HIGH (stack-specific, verified against official docs)

---

## Critical Pitfalls

### Pitfall 1: Envoy gRPC-JSON Transcoding Field Name Mismatch

**What goes wrong:**
Proto field names use `snake_case` (e.g., `community_id`) but the Envoy transcoder converts them to `camelCase` in JSON by default (`communityId`). Your frontend sends `community_id`, Envoy silently ignores the field (doesn't error — just sets it to zero-value), and your Go service receives an empty string. You debug for hours because the gRPC service works fine when called directly.

**Why it happens:**
Envoy's `grpc_json_transcoder` follows proto3's canonical JSON mapping, which uses `lowerCamelCase` for field names. The `preserve_proto_field_names` option defaults to `false`. Developers test gRPC directly (works), then add the Envoy layer and everything breaks silently because proto3 zero-values look like valid empty requests.

**How to avoid:**
Set `preserve_proto_field_names: false` (the default) and document that the REST API uses camelCase. OR set `preserve_proto_field_names: true` in the Envoy config to accept snake_case. Pick one convention and enforce it project-wide. The critical step: **add an integration test that sends a JSON request through Envoy and verifies the Go service receives non-zero field values**.

```yaml
# envoy.yaml — transcoder config
typed_config:
  "@type": type.googleapis.com/envoy.extensions.filters.http.grpc_json_transcoder.v3.GrpcJsonTranscoder
  proto_descriptor: "/etc/envoy/proto.pb"
  services:
    - redyx.post.v1.PostService
  print_options:
    always_print_primitive_fields: true  # Don't omit zero values in responses
    preserve_proto_field_names: false     # Use camelCase (proto3 canonical)
```

**Warning signs:**
- Fields arrive as zero-values in Go handlers when sent from REST
- Works fine with `grpcurl` but breaks through Envoy
- Frontend team says "we sent the data" but backend logs show empty fields

**Phase to address:**
Phase 1 (Proto definitions + Envoy setup). Must be decided before any service is built. Changing field name convention later breaks all clients.

**Time impact if hit:** 1-2 days debugging per service, multiplied across 12 services = potential 3+ week time sink if caught late.

---

### Pitfall 2: Envoy Route Matching — REST Path vs gRPC Path Confusion

**What goes wrong:**
You define `google.api.http` annotations on your proto RPCs (e.g., `get: "/v1/communities/{community_id}"`), but your Envoy route config matches on the *gRPC* path (`/redyx.community.v1.CommunityService/GetCommunity`), not the REST path. Requests hit 404. Or worse: you set `match_incoming_request_route: false` (the default) and routes that match `/v1/communities/` don't work because the transcoder rewrites the path to the gRPC form before routing.

**Why it happens:**
Per Envoy docs: "The requests processed by the transcoder filter will have `/<package>.<service>/<method>` path and `POST` method." Routes must match the gRPC path, not the incoming HTTP path, unless `match_incoming_request_route` is set to `true`. This is counter-intuitive — you write REST URLs in your proto annotations but route on gRPC paths.

**How to avoid:**
Use `match_incoming_request_route: true` in the transcoder config. This makes route matching use the original HTTP path, which is far more intuitive. Then your route prefixes can be `/v1/` as expected.

```yaml
# Transcoder filter config
match_incoming_request_route: true

# Route config — now matches REST paths
routes:
  - match:
      prefix: "/v1/communities"
    route:
      cluster: community-service
  - match:
      prefix: "/v1/posts"
    route:
      cluster: post-service
```

Without this, you'd need:
```yaml
# Without match_incoming_request_route — confusing
routes:
  - match:
      prefix: "/redyx.community.v1.CommunityService"
    route:
      cluster: community-service
```

**Warning signs:**
- 404s on all REST endpoints even though gRPC works directly
- Routes only work when matching on `/<package>.<service>/` format

**Phase to address:**
Phase 1 (Envoy gateway setup). One-time decision, but getting it wrong means rewriting all route configs.

**Time impact if hit:** 1-2 days of confusion, plus route config rewrite.

---

### Pitfall 3: Proto Descriptor File Out of Sync with Go Services

**What goes wrong:**
Envoy's transcoder needs a compiled `.pb` descriptor file. You update your `.proto` files, regenerate Go code, deploy the Go service — but forget to regenerate and redeploy the descriptor file to Envoy. Now Envoy's transcoder has a stale schema: new fields are silently dropped, new RPCs return 404, and renamed fields break in both directions. Since proto3 is forward-compatible, there's no explicit error.

**Why it happens:**
The descriptor file is a separate build artifact from Go code generation. `protoc --go_out` and `protoc --descriptor_set_out` are separate invocations. Without a single build step that does both, they drift.

**How to avoid:**
Create a single `Makefile` target (or `buf` config) that generates Go code AND the descriptor set in one step. Include the descriptor file in the same Docker image or mount it from a shared volume.

```makefile
# Makefile — single target generates everything
.PHONY: proto
proto:
	protoc \
		-I$(GOOGLEAPIS_DIR) -I./proto \
		--include_imports --include_source_info \
		--descriptor_set_out=envoy/proto.pb \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(shell find proto -name '*.proto')
```

Better: use `buf` which handles both in a single config file. Also add a CI step that rebuilds descriptors and fails if the checked-in file differs.

**Warning signs:**
- New RPCs work via gRPC but 404 via REST
- Added proto fields not appearing in JSON responses
- Envoy logs show "unknown field" warnings

**Phase to address:**
Phase 1 (build tooling). Set up `buf.yaml` or Makefile before writing any proto files.

**Time impact if hit:** 30 minutes per incident, but happens repeatedly (every proto change) until fixed.

---

### Pitfall 4: ScyllaDB Comment Tree — Unbounded Partition Size

**What goes wrong:**
You model comments with `post_id` as the partition key and `path` (materialized path like `"root/parent/child"`) as a clustering key. Popular posts accumulate thousands of comments in a single partition. ScyllaDB partitions should stay under ~100MB. A viral post with 50K comments, each averaging 2KB, hits 100MB. Reads become slow, compaction stalls, and the node becomes a hotspot.

**Why it happens:**
The data model is optimized for the happy path (fetch all comments for a post in one query). It doesn't account for the 99th percentile — a post that goes viral. This is the classic wide-partition anti-pattern in Cassandra/ScyllaDB.

**How to avoid:**
Use a composite partition key: `(post_id, bucket)` where `bucket` is derived from the comment's parent or a time-based bucket. For a Reddit-like system, a practical approach:

```cql
CREATE TABLE comments (
    post_id     UUID,
    parent_id   UUID,       -- root comments have parent_id = post_id
    comment_id  TIMEUUID,   -- inherently ordered by creation time
    author_id   UUID,
    body        TEXT,
    path        TEXT,        -- materialized path: "root_id/parent_id/comment_id"
    depth       INT,
    created_at  TIMESTAMP,
    PRIMARY KEY ((post_id, parent_id), comment_id)
) WITH CLUSTERING ORDER BY (comment_id ASC);
```

This partitions by `(post_id, parent_id)`, so each parent's direct children are in one partition. To load a thread: first fetch root-level comments (`parent_id = post_id`), then lazily fetch child partitions. No single partition grows unbounded.

The tradeoff: you can't fetch the entire comment tree in one query. But Reddit doesn't either — it paginates and lazy-loads. Design for that from day one.

**Warning signs:**
- `nodetool tablestats` shows large partition warnings
- Comment fetches slow down for popular posts
- ScyllaDB logs: "Writing large partition" warnings

**Phase to address:**
Phase where comment service schema is designed. The schema is nearly impossible to change after data is written.

**Time impact if hit:** 2-5 day schema redesign + data migration if caught after data exists. Effectively a rewrite of the comment service.

---

### Pitfall 5: ScyllaDB Tombstone Accumulation from Comment Deletions

**What goes wrong:**
When users delete comments, ScyllaDB doesn't actually remove data — it writes a tombstone marker. Tombstones accumulate until `gc_grace_seconds` passes (default: 10 days). A moderation sweep that deletes 1000 comments from a post creates 1000 tombstones in that partition. Subsequent reads must scan through all tombstones, causing "tombstone overwhelm" — reads time out, and ScyllaDB may return errors or drop the query.

**Why it happens:**
ScyllaDB's distributed nature requires tombstones to propagate deletions to all replicas. Developers used to PostgreSQL `DELETE` expect deletions to be free. In ScyllaDB, deletions are the most expensive operation.

**How to avoid:**
1. **Soft-delete in the application**: Set a `deleted` boolean and `deleted_at` timestamp. Filter deleted comments in application code. Never issue CQL `DELETE` for comments.
2. **Use TTL for truly ephemeral data** (e.g., notification reads, rate limit counters) — TTL creates tombstones too, but they're predictable.
3. **Set `gc_grace_seconds` appropriately**: For the comments table, 10 days is fine if you run repairs weekly.
4. **Design the schema to avoid range deletes**: Never `DELETE FROM comments WHERE post_id = X` — this creates a partition-level tombstone that blocks ALL reads.

```go
// Soft-delete — DO THIS
func (r *CommentRepo) DeleteComment(ctx context.Context, postID, commentID uuid.UUID) error {
    return r.session.Query(
        `UPDATE comments SET deleted = true, deleted_at = ?, body = '[deleted]'
         WHERE post_id = ? AND parent_id = ? AND comment_id = ?`,
        time.Now(), postID, parentID, commentID,
    ).WithContext(ctx).Exec()
}

// Hard-delete — DON'T DO THIS
// DELETE FROM comments WHERE post_id = ? AND comment_id = ?
```

**Warning signs:**
- Read latency spikes after moderation actions
- ScyllaDB logs: "Read X live rows and Y tombstone cells"
- Monitoring shows increasing tombstone-to-live-cell ratio

**Phase to address:**
Phase where comment service is built. Must be a design decision before any delete logic is written.

**Time impact if hit:** 1-2 days to refactor delete logic, plus ongoing performance issues until tombstones are compacted.

---

### Pitfall 6: Kafka Consumer Group Rebalancing Storm During Deploys

**What goes wrong:**
You have 3 pods of the vote-service, each in the same consumer group, consuming vote events. During a rolling deploy, Kubernetes stops pod 1, Kafka detects the lost consumer and triggers a rebalance. While rebalancing, ALL consumers in the group pause processing. Pod 2 starts (new version), triggering another rebalance. Pod 1 (old) stops, another rebalance. Three rebalances in quick succession. During each rebalance (which takes `session.timeout.ms` + `max.poll.interval.ms` to settle), zero events are processed. Vote counts freeze for 30-60 seconds.

**Why it happens:**
Kafka's consumer group protocol requires all consumers to agree on partition assignments. Any membership change triggers a full "stop-the-world" rebalance. With Kubernetes rolling deploys, membership changes happen in rapid succession.

**How to avoid:**
1. **Use `CooperativeStickyAssignor`** (Kafka 2.4+): This uses incremental rebalancing — only the affected partitions are reassigned, and other consumers continue processing.
2. **Set `session.timeout.ms` and `heartbeat.interval.ms` correctly**: Session timeout 30s (not the default 10s), heartbeat 10s. This gives pods time to shut down gracefully.
3. **Implement graceful shutdown**: On SIGTERM, commit offsets and leave the group explicitly before the process exits. This triggers a single clean rebalance instead of a timeout-based one.

```go
// sarama consumer config
config := sarama.NewConfig()
config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
    sarama.NewBalanceStrategySticky(), // Use sticky assignment
}
config.Consumer.Group.Session.Timeout = 30 * time.Second
config.Consumer.Group.Heartbeat.Interval = 10 * time.Second

// Graceful shutdown
ctx, cancel := context.WithCancel(context.Background())
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigChan
    cancel() // This triggers ConsumeClaim to return
}()
// consumer.Consume will return when ctx is cancelled
// sarama then commits offsets and cleanly leaves the group
```

4. **Set `terminationGracePeriodSeconds`** in K8s to 45s+ to give the consumer time to commit and leave.

**Warning signs:**
- Vote counts freeze during deploys
- Kafka consumer lag spikes during rolling updates
- Logs show repeated "rebalancing" messages in quick succession

**Phase to address:**
Phase where Kafka consumers are first implemented. Must be configured correctly from the start — changing rebalance strategy later causes a full rebalance.

**Time impact if hit:** 1-2 days to diagnose + fix. Meanwhile, every deploy causes 30-60s event processing outage.

---

### Pitfall 7: Kafka Consumer Offset Commit — At-Least-Once Duplication Without Idempotency

**What goes wrong:**
Your vote consumer reads a vote event, increments the post's vote count in Redis, then commits the offset. If the consumer crashes after incrementing but before committing, the event is reprocessed on restart — the vote is counted twice. Over time, popular posts have inflated vote counts, and you can't tell which increments were duplicates.

**Why it happens:**
Kafka guarantees at-least-once delivery by default. Developers familiar with traditional message queues (which often provide exactly-once) don't realize they need to handle duplicates. Auto-commit makes this worse: offsets are committed on a timer, not after successful processing.

**How to avoid:**
1. **Disable auto-commit**: `config.Consumer.Offsets.AutoCommit.Enable = false`. Commit manually after processing.
2. **Make consumers idempotent**: Use the event's unique ID (vote_id) as an idempotency key.
3. **For vote counts**: Use a Redis set to track which votes have been counted, then derive the count from the set size. `SADD post:{id}:upvotes {user_id}` is naturally idempotent.

```go
// WRONG: counter-based (not idempotent)
// INCR post:{post_id}:score

// RIGHT: set-based (naturally idempotent)
func (h *VoteHandler) HandleVote(msg *sarama.ConsumerMessage) error {
    var vote VoteEvent
    if err := json.Unmarshal(msg.Value, &vote); err != nil {
        return err // Dead-letter this
    }

    // SADD is idempotent — adding same user_id twice has no effect
    if vote.Direction == 1 {
        h.redis.SAdd(ctx, fmt.Sprintf("post:%s:upvotes", vote.PostID), vote.UserID)
        h.redis.SRem(ctx, fmt.Sprintf("post:%s:downvotes", vote.PostID), vote.UserID)
    } else {
        h.redis.SAdd(ctx, fmt.Sprintf("post:%s:downvotes", vote.PostID), vote.UserID)
        h.redis.SRem(ctx, fmt.Sprintf("post:%s:upvotes", vote.PostID), vote.UserID)
    }

    // Score = |upvotes| - |downvotes| — always correct regardless of replay
    return nil
}
```

**Warning signs:**
- Vote counts don't match the number of distinct voters
- Counts drift higher over time, especially after deploys (which cause reprocessing)
- `kafka_consumer_group_lag` metric shows offset resets

**Phase to address:**
Phase where vote/karma Kafka consumers are built. Must be a design decision for the data model.

**Time impact if hit:** 2-3 days to redesign from counter-based to set-based votes, plus data reconciliation.

---

### Pitfall 8: Application-Level Consistent Hashing — Hash Ring Rebalancing on Shard Change

**What goes wrong:**
You implement consistent hashing on `community_id` to route posts to sharded PostgreSQL instances. With 2 shards, community "gaming" maps to shard 1. You add shard 3 later, and now "gaming" maps to shard 2. All existing posts for "gaming" are on shard 1 but new posts go to shard 2. Queries that need all posts for a community now need to check multiple shards, or you need a data migration.

**Why it happens:**
Basic hash-modulo (`hash(community_id) % num_shards`) remaps most keys when shard count changes. Even true consistent hashing remaps ~1/N keys on average. Developers plan the initial sharding but don't plan for shard addition.

**How to avoid:**
1. **Use consistent hashing with virtual nodes**, not hash-modulo. This minimizes key remapping when shards change.
2. **Store the shard mapping explicitly**: Maintain a `community_shards` table in a central PostgreSQL (the "platform" DB) that records which shard each community lives on. Hash determines initial placement; the lookup table is the source of truth.
3. **Design for a fixed shard count from the start**: For a v1.0 demo, use 2-4 shards and accept this as fixed. Don't over-engineer shard addition — it's a v2 problem.

```go
// shard_router.go
type ShardRouter struct {
    ring     *hashring.HashRing  // consistent hash ring
    shardMap sync.Map            // community_id -> shard override (for migrations)
}

func (r *ShardRouter) GetShard(communityID string) string {
    // Check override map first (for communities mid-migration)
    if shard, ok := r.shardMap.Load(communityID); ok {
        return shard.(string)
    }
    // Fall back to consistent hash
    shard, _ := r.ring.GetNode(communityID)
    return shard
}
```

4. **Critical: never store shard identifiers in the data itself**. Route at the application layer; the data doesn't know which shard it's on.

**Warning signs:**
- Planning to "add shards later" without a migration strategy
- Using `hash % N` instead of consistent hashing
- No integration tests for shard routing

**Phase to address:**
Phase where post service sharding is designed. The routing algorithm is foundational — changing it later requires data migration.

**Time impact if hit:** 3-5 days for data migration if shard count changes. With proper design, shard addition is a controlled operation rather than an emergency.

---

### Pitfall 9: WebSocket Notification Delivery Across Pods — User Connected to Wrong Pod

**What goes wrong:**
User A is connected via WebSocket to pod 2 of the notification service. User B upvotes User A's post. The Kafka consumer processing the vote event runs on pod 1. Pod 1 tries to deliver the notification to User A, but User A's WebSocket is on pod 2. The notification is lost.

**Why it happens:**
WebSocket connections are stateful — they're bound to a specific pod. Without a cross-pod pub/sub mechanism, notifications can only be delivered to locally connected users.

**How to avoid:**
Use Redis Pub/Sub as a sidecar broadcast channel. When a Kafka consumer on any pod creates a notification, it publishes to a Redis channel. All notification service pods subscribe to these channels and deliver to their local WebSocket connections.

```go
// notification_service.go

// On Kafka event: publish to Redis (any pod can do this)
func (s *NotifService) HandleKafkaEvent(event NotificationEvent) {
    // Store notification in DB (for later retrieval)
    s.repo.SaveNotification(ctx, event)

    // Broadcast via Redis Pub/Sub to all pods
    payload, _ := json.Marshal(event)
    s.redis.Publish(ctx, fmt.Sprintf("notif:%s", event.UserID), payload)
}

// On startup: each pod subscribes to notifications for its connected users
func (s *NotifService) SubscribeUser(userID string, ws *websocket.Conn) {
    sub := s.redis.Subscribe(ctx, fmt.Sprintf("notif:%s", userID))
    go func() {
        for msg := range sub.Channel() {
            ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
        }
    }()
}
```

**Alternative**: Use a Redis Pub/Sub broadcast channel per pod, and fan out on-publish rather than on-subscribe. The tradeoff depends on user count vs. pod count.

**Key consideration**: Redis Pub/Sub is fire-and-forget. If a pod is briefly disconnected from Redis, notifications are lost. For guaranteed delivery, also persist notifications to the DB and have the client fetch unread on reconnect.

**Warning signs:**
- Notifications work in dev (1 pod) but fail intermittently in staging (multiple pods)
- Users report missing notifications
- Notification delivery rate < 100% in metrics

**Phase to address:**
Phase where WebSocket notification service is built. Must be designed in from the start — adding Redis Pub/Sub later requires refactoring the entire notification flow.

**Time impact if hit:** 2-3 days to add Redis Pub/Sub after the fact, plus debugging time for the "works on one pod" mystery.

---

### Pitfall 10: JWT Token Revocation — Logout Doesn't Actually Log Out

**What goes wrong:**
User changes password or gets banned. You delete their refresh token from the DB. But their current access token (JWT) is still valid for up to 15 minutes. The banned user continues accessing the platform until the token expires. With 15-minute access tokens, this is a significant security gap for a platform that needs to ban abusive users quickly.

**Why it happens:**
JWTs are stateless by design — they can't be revoked without some server-side state. Developers choose JWTs for scalability but don't realize they've traded away immediate revocation.

**How to avoid:**
Implement a lightweight token blocklist in Redis with TTL matching the access token lifetime:

```go
// On ban/password change/logout:
func (s *AuthService) RevokeAccessToken(tokenID string, expiresAt time.Time) error {
    ttl := time.Until(expiresAt)
    if ttl <= 0 {
        return nil // Already expired
    }
    return s.redis.Set(ctx, fmt.Sprintf("revoked:%s", tokenID), "1", ttl).Err()
}

// In auth middleware (every request):
func (m *AuthMiddleware) IsRevoked(tokenID string) bool {
    exists, _ := m.redis.Exists(ctx, fmt.Sprintf("revoked:%s", tokenID)).Result()
    return exists > 0
}
```

This costs one Redis `EXISTS` per request (sub-millisecond) and the blocklist is self-cleaning via TTL. Include a unique `jti` (JWT ID) claim in every access token to enable per-token revocation.

**Warning signs:**
- No `jti` claim in JWTs
- No revocation mechanism planned
- "We'll handle it with short token lifetimes" — 15 minutes is still too long for banning

**Phase to address:**
Phase where auth service is built. Must be designed into the JWT claims structure and middleware from day one.

**Time impact if hit:** 1 day to add blocklist, but if JWT claims don't include `jti`, all existing tokens are unrevocable.

---

### Pitfall 11: gRPC Error Handling — Returning Go Errors Instead of gRPC Status

**What goes wrong:**
You return a plain Go error from a gRPC handler: `return nil, fmt.Errorf("community not found")`. The client receives `UNKNOWN` status code with the error message. Your Envoy transcoder translates this to HTTP 500 (Internal Server Error). Every "not found" looks like a server crash to the frontend. Your error monitoring fires alerts on normal 404s.

**Why it happens:**
In Go, the natural idiom is to return `error`. But gRPC expects `status.Error` with a specific status code. The gRPC library wraps plain errors as `codes.Unknown`. Developers write Go-idiomatic code that is gRPC-unaware.

**How to avoid:**
Always use `google.golang.org/grpc/status` and `google.golang.org/grpc/codes`:

```go
import (
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// WRONG
func (s *Server) GetCommunity(ctx context.Context, req *pb.GetCommunityRequest) (*pb.Community, error) {
    c, err := s.repo.Find(req.Id)
    if err != nil {
        return nil, fmt.Errorf("community not found: %w", err) // -> HTTP 500!
    }
    return c, nil
}

// RIGHT
func (s *Server) GetCommunity(ctx context.Context, req *pb.GetCommunityRequest) (*pb.Community, error) {
    c, err := s.repo.Find(req.Id)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            return nil, status.Error(codes.NotFound, "community not found") // -> HTTP 404
        }
        return nil, status.Error(codes.Internal, "failed to fetch community") // -> HTTP 500
    }
    return c, nil
}
```

Create a shared error-mapping interceptor early:

```go
// interceptor that maps domain errors to gRPC status codes
func ErrorMappingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
    resp, err := handler(ctx, req)
    if err != nil {
        if _, ok := status.FromError(err); ok {
            return resp, err // Already a gRPC status, pass through
        }
        // Map domain errors
        switch {
        case errors.Is(err, ErrNotFound):
            return nil, status.Error(codes.NotFound, err.Error())
        case errors.Is(err, ErrForbidden):
            return nil, status.Error(codes.PermissionDenied, err.Error())
        case errors.Is(err, ErrConflict):
            return nil, status.Error(codes.AlreadyExists, err.Error())
        default:
            return nil, status.Error(codes.Internal, "internal error")
        }
    }
    return resp, nil
}
```

**Warning signs:**
- Frontend gets HTTP 500 for not-found resources
- Error monitoring has high alert noise
- All gRPC errors show `UNKNOWN` code in traces

**Phase to address:**
Phase 1 (shared libraries). Build the error interceptor before any service handler is written.

**Time impact if hit:** 30 minutes to build the interceptor, but retrofitting across 12 services takes 1-2 days.

---

### Pitfall 12: Proto API Design — Changing Field Numbers After Deployment

**What goes wrong:**
You have `message Post { string title = 1; string body = 2; }`. You decide to rename the field and accidentally change the field number: `message Post { string headline = 1; string content = 3; }`. Field 3 never existed before — that's fine. But what if you reuse field 2 for something else? Old serialized data maps the old `body` field to the new field 2. Proto uses field numbers for wire encoding, not names.

**Why it happens:**
Proto field numbers are the stable wire identifiers. Field names only matter for JSON encoding. Developers used to SQL schema migrations think renaming a column is safe. In proto, renaming the Go accessor is safe, but changing the number breaks backward compatibility.

**How to avoid:**
1. **Never reuse field numbers**. If you remove a field, `reserve` its number:
```protobuf
message Post {
    string title = 1;
    reserved 2; // was: string old_field = 2;
    string body = 3;
}
```
2. **Use `buf lint`** to enforce proto best practices automatically (it catches field number reuse).
3. **Treat proto files as append-only**: New fields get new (higher) numbers. Removed fields get reserved.

**Warning signs:**
- Data corruption after proto changes
- Old messages deserialize incorrectly
- No `reserved` statements in proto files

**Phase to address:**
Phase 1 (proto definitions). Establish conventions before any proto is written.

**Time impact if hit:** Data corruption is catastrophic. Could require database wipe and re-seed in development, or complex migration in production.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Skip proto linting (`buf lint`) | Faster proto iteration | Inconsistent APIs, transcoding bugs, breaking changes slip through | Never — takes 5 min to set up, saves days |
| Single PostgreSQL for all services | Simpler Docker Compose, fewer ports | Tight coupling, migration conflicts, can't independently scale | Only in first week of dev for bootstrapping, migrate to per-service DBs within the phase |
| Hardcoded shard count (`hash % 2`) | Simpler initial implementation | Impossible to add shards without data migration | Only if you're certain you'll never need more shards (you're not certain) |
| Store vote counts as counters, not sets | Simpler code, one Redis key per post | Can't deduplicate, can't list who voted, can't undo votes correctly | Never — set-based is only marginally more complex |
| Auto-commit Kafka offsets | Less code, no manual commit logic | At-least-once guarantees broken (can lose messages), can't implement backpressure | Early prototyping only; switch to manual before any real data flows |
| `SELECT *` from ScyllaDB | Faster development, less typing | Reads all columns including large `body` text for listing views, wastes bandwidth | Never — always specify needed columns in CQL |
| HTTP polling instead of WebSocket | Avoids WebSocket complexity entirely | High server load from polling, poor UX for notifications | MVP only if WebSocket is behind schedule — plan to replace within same milestone |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Envoy + gRPC | Not setting `http2_protocol_options` on the upstream cluster | Envoy must use HTTP/2 to talk to gRPC backends. Set `explicit_http_config.http2_protocol_options: {}` on every gRPC cluster. Without this, Envoy sends HTTP/1.1 and gRPC fails silently. |
| Envoy + gRPC | Not including `google/api/annotations.proto` and `google/api/http.proto` in protoc include path | The `google.api.http` option requires these imports. Clone `googleapis/googleapis` and add to `-I` path. Without them, protoc compiles but annotations are ignored. |
| ScyllaDB + Go (gocql) | Using `gocql.Quorum` consistency for all reads | Use `gocql.One` or `gocql.LocalOne` for comment tree reads (latency-sensitive, eventual consistency is fine). Reserve `Quorum` for vote-count reads where accuracy matters. |
| ScyllaDB + Go (gocql) | Not configuring retry policy | Default retry policy retries on `NextHost` which can cascade timeouts. Use `gocql.SimpleRetryPolicy{NumRetries: 2}` and handle errors in application code. |
| Kafka + Go (sarama) | Using `sarama.OffsetNewest` for new consumer groups | New consumers start from the latest offset and miss all historical events. Use `sarama.OffsetOldest` for the first run, then rely on committed offsets for subsequent runs. `config.Consumer.Offsets.Initial = sarama.OffsetOldest` |
| Kafka + Go (sarama) | Forgetting to handle `ConsumerGroupHandler.Setup()` and `Cleanup()` | These are called on every rebalance. If you initialize resources in `Setup()` but don't clean up in `Cleanup()`, you leak goroutines and connections on every rebalance. |
| Redis + Go | Not setting read/write timeouts on the Redis client | Default is no timeout. A single slow Redis command blocks the goroutine forever. Set `ReadTimeout: 3s, WriteTimeout: 3s, DialTimeout: 5s` on client options. |
| PostgreSQL + Go | Opening a new connection per request instead of using `sql.DB` pool | `sql.Open()` returns a pool. Don't create new pools per request. Set `SetMaxOpenConns(25)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(5 * time.Minute)`. |
| Meilisearch + Go | Treating `AddDocuments` as synchronous | Meilisearch indexing is async. `AddDocuments` returns a task ID. If you need confirmation, poll the task status. Don't assume the document is searchable immediately after the call returns. |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Loading full comment trees in one query | Initial page load takes 2-3 seconds for popular posts | Paginate: fetch top-level comments first (20), lazy-load children on expand. Never `SELECT * FROM comments WHERE post_id = ?` without `LIMIT`. | Posts with > 200 comments |
| N+1 queries for post listings (fetch post, then author, then community, then vote count) | Post feed API takes 500ms+, linear with page size | Batch fetches: fetch all post IDs, then batch-fetch authors, communities, and vote counts. Use Redis pipeline for multi-key operations. | Any listing page with > 10 items |
| Single Redis instance for all 6 logical DBs | One slow operation (e.g., large SMEMBERS on vote set) blocks all other DBs | Use Redis Cluster or separate Redis instances for latency-sensitive uses (sessions, rate limits) vs. bulk operations (vote sets, search cache). | When any single key exceeds 10K members |
| Synchronous Kafka produce in request handlers | Request latency includes Kafka write (5-50ms per message) | Use async producer with channel-based send. Don't wait for ack in the request path. Accept the message, enqueue to Kafka, return to client. | Under any meaningful load — it adds latency to every vote/action |
| gRPC connection per request | High connection overhead, port exhaustion | Create gRPC client connections at service startup, reuse across requests. gRPC multiplexes RPCs over a single HTTP/2 connection. One `grpc.ClientConn` per target service is sufficient. | > 100 RPS per service |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Storing JWT secret in code or environment variable without rotation | Single key compromise exposes all users | Use asymmetric keys (RS256/ES256): sign with private key, verify with public key. Store private key in secrets manager. Public key can be distributed to all services for verification without risk. |
| Not validating `aud` (audience) claim in JWTs | Token issued for service A can be replayed to service B | Include service-specific `aud` claim. Auth service issues tokens with `aud: "redyx-api"`. Each service validates `aud` matches expected value. |
| Argon2id with low memory parameter | GPU brute-forcing becomes feasible | Use `time=1, memory=64*1024 (64MB), threads=4` minimum. The whole point of argon2id is memory-hardness; skimping on memory defeats the purpose. |
| Refresh token stored in Redis without binding to user/device | Stolen refresh token works from any IP/device | Bind refresh tokens to a fingerprint (user-agent + IP subnet). On refresh, verify the fingerprint matches. Invalidate all tokens on password change. |
| No rate limiting on auth endpoints | Credential stuffing, brute force attacks | Rate limit by IP: 5 login attempts per minute per IP. Rate limit by account: 10 attempts per hour per email. Use sliding window in Redis. Implement before the auth service is exposed. |
| OAuth2 state parameter not validated | CSRF attacks on OAuth login flow | Generate a random `state`, store in a short-lived cookie (5 min TTL), validate on callback. Don't store in session (defeats the purpose — attacker can set session). |
| Exposing internal gRPC errors to REST clients | Information disclosure (stack traces, DB errors, internal service names) | The error-mapping interceptor should sanitize: map to generic messages for `codes.Internal`. Never pass raw error strings from downstream services to REST responses. |
| Not encrypting emails at rest (project constraint) | PII exposure on database breach | Use AES-256-GCM encryption before storing. Store the encryption key in a secrets manager, not in the database or code. Use a separate key per user (derived from master key + user_id) for defense in depth. |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Showing stale vote counts (Redis cache not updated after vote) | User votes, count doesn't change, votes again thinking it didn't register | Optimistically update the count in the frontend on vote, then reconcile with the server-side count when the async Kafka pipeline updates Redis. |
| Comment tree loads all-or-nothing | Post with 500 comments takes 3s to load; user sees blank page | Load top 20 root comments with first 3 replies each. "Load more" button for additional roots. "Show N more replies" for child threads. |
| No loading states during cross-service calls | User clicks "Create Post", nothing happens for 2 seconds | Frontend shows optimistic UI immediately. Backend returns 202 Accepted for async operations. Use WebSocket to push the final result. |
| Notification WebSocket reconnection not handled | User's phone sleeps, wakes up, no more notifications until refresh | Client-side: implement exponential backoff reconnection. On reconnect, fetch unread notifications from REST API to catch up on missed real-time events. |
| Search results showing deleted/removed content | User finds content in search that's been removed by moderator | Meilisearch indexing is async, so there's a delay. On the search results page, validate visibility before rendering (check `deleted` flag). Also publish a delete event to trigger Meilisearch document removal. |

## "Looks Done But Isn't" Checklist

- [ ] **Envoy transcoding:** Test every RPC through Envoy (not just gRPC directly). Verify field names map correctly in both request and response.
- [ ] **gRPC error codes:** Verify that `NotFound` returns HTTP 404, `PermissionDenied` returns 403, `InvalidArgument` returns 400. Envoy auto-maps gRPC codes to HTTP codes, but only if you use the right gRPC codes.
- [ ] **JWT auth:** Test token expiration (fast-forward time), test revocation (ban user, verify immediate lockout), test refresh flow (expired access + valid refresh = new tokens).
- [ ] **Kafka consumers:** Verify at-least-once by killing consumers mid-batch and checking no events are lost. Verify idempotency by replaying the same batch and checking no duplicates.
- [ ] **ScyllaDB comments:** Test with 1000+ comments on a single post. Verify pagination works. Verify deleted comment tombstones don't degrade read performance.
- [ ] **WebSocket across pods:** Deploy 2+ notification pods. Send notification to a user connected to pod 1 via an event processed on pod 2. Verify delivery.
- [ ] **Post sharding:** Create posts in community A (shard 1) and community B (shard 2). Verify listing by community queries the correct shard. Verify cross-shard operations (user's post history across communities) aggregate correctly.
- [ ] **Rate limiting:** Test from the same IP with burst traffic. Verify limits are enforced at the Envoy/Redis level, not just in application code (which can be bypassed by hitting different pods).
- [ ] **OAuth2 flow:** Test full redirect flow including error cases: user denies consent, Google is down, state parameter mismatch.
- [ ] **Media upload:** Upload a 10MB image. Verify it's virus-scanned, resized, stored in S3, and CDN URL returned. Test with malicious file (rename .exe to .jpg).
- [ ] **Graceful shutdown:** During a rolling deploy, verify in-flight requests complete, Kafka offsets are committed, WebSocket connections are cleanly closed, and no data is lost.
- [ ] **Distributed tracing:** Verify a single user request generates a trace that spans Envoy -> Go service -> Kafka -> consumer -> Redis/DB. If trace context isn't propagated across Kafka, you have blind spots.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Envoy field name mismatch | LOW | Update `preserve_proto_field_names` in Envoy config. Frontend may need to update field names in API calls. No data loss. |
| Envoy route matching | LOW | Change `match_incoming_request_route` to true, rewrite route prefixes. No data loss. |
| Proto descriptor out of sync | LOW | Rebuild descriptor file and redeploy Envoy. No data loss. |
| ScyllaDB unbounded partitions | HIGH | Schema redesign requires new table + data migration script. Must run dual-write during migration. May need to drain the comments table and re-insert. |
| ScyllaDB tombstone accumulation | MEDIUM | Run `nodetool repair`, lower `gc_grace_seconds` temporarily, run `nodetool compact`. Then refactor to soft-delete. |
| Kafka rebalancing storm | LOW | Update consumer config (sticky assignor, session timeout) and redeploy. No data loss — consumers resume from committed offsets. |
| Kafka offset duplication | MEDIUM | If using set-based votes: no recovery needed (idempotent). If using counters: reconcile by re-deriving counts from the vote event log (replay from offset 0). |
| Consistent hash shard change | HIGH | Write a migration script that reads from old shard, writes to new shard, switches routing. Requires downtime or dual-write period. |
| WebSocket cross-pod delivery | MEDIUM | Add Redis Pub/Sub layer. Persist undelivered notifications to DB. Client fetches missed notifications on reconnect. |
| JWT without revocation | MEDIUM | Add Redis blocklist. Requires adding `jti` claim — issue new tokens to all users (effectively a mass logout). |
| gRPC error code misuse | LOW | Add error-mapping interceptor. Existing client code may need updates if it was handling HTTP 500 for not-found. |
| Proto field number reuse | HIGH | If data was serialized with conflicting field numbers, it's corrupted. Must identify affected records and re-create them. Prevention is the only real strategy. |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Envoy field name mismatch | Proto + Envoy setup | Integration test: JSON request through Envoy, verify Go handler receives correct values |
| Envoy route matching | Envoy gateway config | Test each route returns 200 (not 404) through Envoy |
| Proto descriptor sync | Build tooling (Makefile/buf) | CI step: rebuild descriptor, `diff` against checked-in version, fail if different |
| ScyllaDB unbounded partitions | Comment service schema design | Load test: insert 10K comments on one post, verify read latency < 100ms |
| ScyllaDB tombstone accumulation | Comment service delete logic | After deleting 100 comments, verify read latency unchanged |
| Kafka rebalancing storm | Kafka consumer config (first consumer) | Simulate rolling deploy in staging, measure event processing gap |
| Kafka offset duplication | Vote/karma consumer design | Replay same event batch, verify vote count unchanged |
| Consistent hash shard routing | Post service sharding design | Add a third shard to test config, verify existing community lookups unchanged |
| WebSocket cross-pod delivery | Notification service architecture | Deploy 2+ pods, verify cross-pod notification delivery |
| JWT revocation | Auth service design | Ban user, immediately verify API access rejected |
| gRPC error codes | Shared library (interceptors) | Test each error type maps to correct HTTP status code through Envoy |
| Proto field number safety | Proto linting (buf) | `buf lint` in CI, `buf breaking` against main branch |

## Sources

- Envoy gRPC-JSON Transcoder docs: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/grpc_json_transcoder_filter (verified 2026-03-02)
- AIP-127: HTTP and gRPC Transcoding: https://aip.dev/127 (verified 2026-03-02)
- gRPC Error Handling guide: https://grpc.io/docs/guides/error/ (verified 2026-03-02)
- gRPC Go Basics Tutorial: https://grpc.io/docs/languages/go/basics/ (verified 2026-03-02)
- ScyllaDB Data Types (collections, tombstone behavior): https://opensource.docs.scylladb.com/stable/cql/types.html (verified 2026-03-02)
- ScyllaDB Anti-Entropy (repair, consistency): https://opensource.docs.scylladb.com/stable/architecture/anti-entropy/ (verified 2026-03-02)
- ScyllaDB Tombstone Flush KB: https://opensource.docs.scylladb.com/stable/kb/tombstones-flush.html (verified 2026-03-02)
- IBM/sarama (Go Kafka client) documentation: https://pkg.go.dev/github.com/IBM/sarama (verified 2026-03-02)
- Proto3 JSON Mapping: https://developers.google.com/protocol-buffers/docs/proto3#json (MEDIUM confidence — training data)
- Consistent hashing virtual nodes: standard distributed systems practice (HIGH confidence — well-established pattern)
- Argon2id parameters: OWASP recommendations (HIGH confidence — industry standard)
- Redis Pub/Sub for WebSocket fan-out: common architectural pattern (HIGH confidence — widely documented)

---
*Pitfalls research for: Redyx — Go microservice platform (anonymous Reddit clone)*
*Researched: 2026-03-02*
