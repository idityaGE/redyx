# Phase 6: Moderation + Spam (Full Stack) - Research

**Researched:** 2026-03-06
**Domain:** Content moderation, spam detection, Kafka async analysis, Svelte moderation UI
**Confidence:** HIGH

## Summary

Phase 6 introduces two new backend microservices (moderation-service and spam-service) plus substantial frontend components. The moderation service handles content removal, user bans, post pinning, mod log, and report queue management — all requiring moderator role verification via cross-service gRPC calls to community-service. The spam service provides synchronous content checking (blocklist + URL + duplicate detection) and asynchronous behavior analysis via Kafka consumers.

The project has significant existing infrastructure to build upon: protos are already defined and compiled (`proto/redyx/moderation/v1/moderation.proto`, `proto/redyx/spam/v1/spam.proto`), generated Go code exists in `gen/`, the `is_pinned` column exists on posts, the community role system (`member/moderator/owner`) is operational, and Kafka producer/consumer patterns are well-established across vote-service, post-service, and search-service. The Dockerfile, docker-compose, and envoy routing patterns are consistent and repeatable.

**Primary recommendation:** Build moderation-service and spam-service following the exact patterns established by existing services (vote, post, notification). Extend CommunitySettings.svelte with mod tools tabs. The proto changes needed are minimal — add `SubmitReport`, `DismissReport`, `ListBans`, and `RestoreContent` RPCs plus the ban check endpoint used by post/comment services. The spam service needs a new Kafka topic for behavior events and a consumer for async analysis.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Report action triggered via three-dot overflow menu on posts and comments
- Report dialog shows a predefined reason picker — no free text field
- Reasons: Spam, Harassment, Misinformation, Breaks community rules, Other
- After submitting: confirmation toast ("Report submitted"), no status tracking or "My Reports" page
- Reports are anonymous to the reported user
- Report queue is the primary/landing view when mod opens mod tools
- Mod tools accessed as a tab within existing CommunitySettings (already gated to moderators)
- Inline action buttons on each queue item: [remove] [dismiss] [ban user] — no detail view expansion
- Inline confirmation pattern before destructive actions (button changes to [confirm?] / [cancel]) — matches existing CommentCard delete pattern
- No pending report count badge or indicator — mods check manually
- Resolved items move to a separate "Resolved" tab — mods can undo ALL actions (remove, dismiss, ban) from there
- Undoing an action moves the item back to the active queue
- Mod log shows full entries: mod name, action taken, target content/user, timestamp, reason
- Mod log filterable by action type (remove, ban, pin, dismiss) using dropdown — reuse SortBar pattern
- Banned user sees a banner at top of community page: "You are banned from this community. Reason: [reason]. Expires: [date or Permanent]."
- Post/comment forms disabled for banned users — can still view/read all content
- Preset ban durations only: 1 day, 3 days, 7 days, 30 days, Permanent
- Ban reason is required — mod must provide a reason
- Ban reason is visible to the banned user in the ban banner
- Ban list tab in mod tools showing all active bans: username, reason, duration, date banned, [unban] button
- Expired bans auto-removed from active list
- No warning system or appeal mechanism — direct bans only (v1)
- Ban dialog includes checkbox: "Also remove all posts/comments by this user" — optional, not default
- Mods can ban directly from report queue inline actions (not just from ban management section)
- Keyword/URL blocklist check happens at publish time — content is rejected, not silently held
- User sees vague rejection message: "Your post couldn't be published — it may contain restricted content." Does not reveal exact blocklist match
- Async behavior analysis (Kafka: rapid posting, link spam patterns) flags to mod queue — no auto-punishment or auto-restriction
- Spam-detected items show a distinct [spam-detection] tag in the report queue, separate from [user-report] tags
- Same queue, same actions — mods can filter by source type
- Blocklists are platform-level only (global) — no per-community customization in v1
- Duplicate content from same user is rejected at publish time (same mechanism as blocklist)

### Claude's Discretion
- Loading states and skeleton design for mod dashboard
- Exact spacing, typography, and terminal-aesthetic styling details
- Error state handling across mod tools
- How pinned posts are visually distinguished in the feed (pin icon, position, styling)
- Pin/unpin UI interaction details (inline on post vs from mod tools)
- Exact behavior analysis thresholds (what constitutes "rapid posting" or "link spam")
- Blocklist seed data (initial keywords and URLs)
- Database schema details (mod_log, reports, bans tables)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MOD-01 | Moderators can remove posts and comments from their community | Moderation service `RemoveContent` RPC, cross-service role check via community-service, sets `is_deleted` on posts/comments |
| MOD-02 | Moderators can ban users from community (temporary or permanent, with duration and reason) | Moderation service `BanUser`/`UnbanUser` RPCs, `bans` table with expires_at, ban check endpoint for post/comment services |
| MOD-03 | Moderators can pin up to 2 posts in their community | Moderation service `PinPost`/`UnpinPost` RPCs, `is_pinned` column already exists on posts table, enforce max 2 pins via count check |
| MOD-04 | All moderation actions recorded in mod log | `mod_log` table, every mod action writes entry before returning, `GetModLog` RPC with action_filter |
| MOD-05 | Moderators can view queue of reported/flagged content | `ListReportQueue` RPC with status filter (active/resolved), aggregated by content_id with report_count |
| MOD-06 | Users can report posts and comments with a reason | New `SubmitReport` RPC on moderation service (or spam service), `reports` table, predefined reason enum |
| SPAM-01 | Content checked against keyword blocklist before publishing | Spam service `CheckContent` RPC called by post-service and comment-service at create time, in-memory keyword trie/set |
| SPAM-02 | URLs in posts checked against known-bad domain list | Same `CheckContent` RPC, URL extraction + domain matching against blocklist set |
| SPAM-03 | Duplicate content from same user rejected | Content hash (SHA-256 of normalized text) stored in Redis per user, checked during `CheckContent` |
| SPAM-04 | Async behavior analysis via Kafka detects rapid posting and link spam | Kafka topic `redyx.behavior.v1`, spam-service consumes post/comment create events, sliding window counters in Redis, flags to mod queue |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| jackc/pgx/v5 | v5.x | PostgreSQL driver for moderation DB | Already used by all PostgreSQL services in project |
| twmb/franz-go | latest | Kafka producer/consumer for behavior analysis | Already used by vote, post, search, notification services |
| redis/go-redis/v9 | v9.x | Duplicate detection cache, behavior counters | Already used by all services; DB 10 for moderation, DB 11 for spam |
| google.golang.org/grpc | latest | Cross-service communication | Already used throughout; moderation calls community-service for role checks |
| Svelte 5 | 5.x | Frontend mod dashboard components | Already the project's interactive component framework with runes ($state, $derived) |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| crypto/sha256 (stdlib) | go1.26 | Content hash for duplicate detection | SPAM-03: hash post/comment body for dedup |
| strings/regexp (stdlib) | go1.26 | URL extraction from content | SPAM-02: extract URLs before blocklist check |
| net/url (stdlib) | go1.26 | URL domain extraction | SPAM-02: parse and normalize URL domains |
| twmb/franz-go/pkg/kadm | latest | Kafka admin client for topic creation | Behavior analysis topic setup on startup |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| In-memory keyword set | PostgreSQL LIKE queries | In-memory is O(1) lookup vs O(n) DB scan; blocklist is small and global |
| SHA-256 content hash | SimHash/MinHash | Exact match is sufficient for v1; fuzzy matching adds complexity |
| Redis sliding window | PostgreSQL time-series | Redis is faster for real-time counters; data is ephemeral |

**Installation:**
No new Go dependencies needed — all libraries already in go.mod. Frontend uses existing Svelte/Astro stack.

## Architecture Patterns

### Recommended Project Structure
```
cmd/moderation/main.go          # Service bootstrap (follows cmd/vote/main.go pattern)
cmd/spam/main.go                # Spam service bootstrap
internal/moderation/
├── server.go                   # ModerationServiceServer implementation
├── store.go                    # PostgreSQL queries (reports, bans, mod_log)
└── producer.go                 # Kafka producer for mod events (optional, for audit)
internal/spam/
├── server.go                   # SpamServiceServer implementation
├── blocklist.go                # In-memory keyword + URL blocklist
├── dedup.go                    # Redis-based content hash dedup
├── consumer.go                 # Kafka consumer for async behavior analysis
└── producer.go                 # Kafka producer for flagging to mod queue
migrations/moderation/
├── 001_moderation.up.sql       # reports, bans, mod_log tables
web/src/components/moderation/
├── ReportDialog.svelte         # Report reason picker modal
├── ReportQueue.svelte          # Mod queue with inline actions
├── ModLog.svelte               # Filterable mod log entries
├── BanList.svelte              # Active bans management
├── BanDialog.svelte            # Ban form (duration, reason, remove content checkbox)
├── PinnedPostControls.svelte   # Pin/unpin UI
```

### Pattern 1: Cross-Service Role Verification
**What:** Moderation service calls community-service via gRPC to verify caller is moderator/owner before executing any mod action.
**When to use:** Every moderation RPC (RemoveContent, BanUser, PinPost, etc.)
**Example:**
```go
// Source: Established pattern in cmd/post/main.go (community membership check)
func (s *Server) verifyModerator(ctx context.Context, communityName string) (*auth.Claims, error) {
    claims := auth.ClaimsFromContext(ctx)
    if claims == nil {
        return nil, fmt.Errorf("unauthenticated: %w", perrors.ErrUnauthenticated)
    }
    // Call community-service to get community + verify role
    resp, err := s.communityClient.GetCommunity(ctx, &commv1.GetCommunityRequest{
        Name: communityName,
        UserId: claims.UserID,
    })
    if err != nil {
        return nil, fmt.Errorf("verify community: %w", err)
    }
    if !resp.IsModerator {
        return nil, fmt.Errorf("not a moderator: %w", perrors.ErrForbidden)
    }
    return claims, nil
}
```

### Pattern 2: Inline Confirmation (Frontend)
**What:** Destructive actions show [confirm?] / [cancel] instead of modal dialogs.
**When to use:** Remove, dismiss, ban actions in report queue and ban list.
**Example:**
```svelte
<!-- Source: CommentCard.svelte confirmingDelete pattern (line 49) -->
<script lang="ts">
  let confirmingRemove = $state(false);
</script>

{#if confirmingRemove}
  <button onclick={executeRemove} class="text-red-500">[confirm?]</button>
  <button onclick={() => confirmingRemove = false} class="text-terminal-dim">[cancel]</button>
{:else}
  <button onclick={() => confirmingRemove = true} class="text-terminal-fg">[remove]</button>
{/if}
```

### Pattern 3: Kafka Async Behavior Analysis
**What:** Spam service consumes post/comment create events, maintains sliding window counters in Redis, and creates report queue entries when thresholds are exceeded.
**When to use:** SPAM-04 — detecting rapid posting and link spam patterns.
**Example:**
```go
// Source: internal/vote/consumer.go PollFetches pattern
func (c *BehaviorConsumer) Run(ctx context.Context) {
    for {
        fetches := c.client.PollFetches(ctx)
        if ctx.Err() != nil {
            return
        }
        fetches.EachRecord(func(r *kgo.Record) {
            var event eventsv1.PostEvent
            if err := proto.Unmarshal(r.Value, &event); err != nil {
                c.logger.Error("unmarshal event", zap.Error(err))
                return
            }
            c.analyzePostBehavior(ctx, &event)
        })
    }
}
```

### Pattern 4: Mod Tools as CommunitySettings Tab Extension
**What:** Extend existing CommunitySettings.svelte with additional tabs for mod tools (queue, log, bans, pins).
**When to use:** All mod dashboard views.
**Example:**
```svelte
<!-- Extend CommunitySettings.svelte with tab navigation -->
<script lang="ts">
  let activeTab = $state<'settings' | 'queue' | 'log' | 'bans'>('queue');
</script>

<!-- Tab bar using terminal bracket style -->
<div class="flex gap-2 text-xs font-mono mb-4">
  {#each tabs as tab}
    <button
      onclick={() => activeTab = tab.id}
      class={activeTab === tab.id ? 'text-accent-500' : 'text-terminal-dim'}
    >
      [{tab.label}]
    </button>
  {/each}
</div>
```

### Anti-Patterns to Avoid
- **Don't call community-service DB directly:** Quick-task 003 eliminated all cross-service DB access. Use gRPC calls to community-service for role checks.
- **Don't use browser confirm() for mod actions:** Project uses inline confirmation pattern (confirmingDelete state), not modal dialogs.
- **Don't auto-punish from spam detection:** User decision — async analysis flags to mod queue only, no auto-bans or auto-removal.
- **Don't reveal blocklist matches to users:** Vague rejection message only — prevents gaming the filter.
- **Don't create separate mod tools page:** Mod tools are tabs within existing CommunitySettings, not a new URL.
- **Don't use `context.Background()` for synchronous spam checks:** Unlike fire-and-forget Kafka publishes, `CheckContent` is synchronous and should use the request context.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Content hashing | Custom hash function | `crypto/sha256` from stdlib | Standard, collision-resistant, no edge cases |
| URL extraction | Regex from scratch | `net/url` + simple regex for href/markdown links | stdlib handles edge cases (encoded chars, ports, etc.) |
| Kafka topic management | Manual topic creation scripts | `kadm.CreateTopics` on startup (existing pattern) | Idempotent, handles "already exists", same as all other services |
| Rate/frequency detection | Custom time-window logic | Redis `INCR` + `EXPIRE` sliding window | Atomic, distributed, survives restarts |
| gRPC service bootstrap | Custom server wiring | `internal/platform/grpcserver.New()` | Handles health checks, reflection, graceful shutdown, interceptor chain |
| Database migrations | Migration framework | `ReadDir` + `pool.Exec` pattern (existing) | Every service uses this exact pattern — simple, idempotent with IF NOT EXISTS |

**Key insight:** This project has strong established patterns for every infrastructure concern. Every new service should be a near-copy of an existing one structurally.

## Common Pitfalls

### Pitfall 1: Envoy Route Ordering for Moderation Paths
**What goes wrong:** Moderation routes `/api/v1/communities/{name}/moderation/*` get caught by the existing community-service catch-all route (`prefix: "/api/v1/communities"`).
**Why it happens:** Envoy uses first-match routing. The community catch-all at line 110 of envoy.yaml matches before moderation-specific routes.
**How to avoid:** Add moderation routes BEFORE the community catch-all, using regex or prefix matching. Pattern established by post-service community posts route (lines 104-109).
**Warning signs:** 404 or wrong service handling moderation requests.

### Pitfall 2: Ban Check Must Be Synchronous and Fast
**What goes wrong:** Ban enforcement on post/comment creation adds latency if it requires a DB round-trip to the moderation service on every create.
**Why it happens:** Every CreatePost and CreateComment now needs to check ban status.
**How to avoid:** Use Redis to cache active bans per community+user with TTL matching shortest ban duration. Moderation service updates Redis on ban/unban. Post/comment services check Redis directly or call moderation service (which checks Redis).
**Warning signs:** Post creation latency increases noticeably.

### Pitfall 3: Proto Descriptor Must Include New Services
**What goes wrong:** Envoy's gRPC-JSON transcoder doesn't recognize moderation/spam service RPCs — returns 404 or "unknown service."
**Why it happens:** The `deploy/envoy/proto.pb` descriptor file must be regenerated to include the new services. Also, the `services` list in envoy.yaml (line 124-135) must include `redyx.moderation.v1.ModerationService` and `redyx.spam.v1.SpamService`.
**How to avoid:** Regenerate proto.pb after proto changes. Add services to envoy.yaml transcoder config.
**Warning signs:** REST calls return 404 despite correct routing.

### Pitfall 4: Report Queue Aggregation vs Raw Reports
**What goes wrong:** Showing one queue entry per report instead of one per content item (aggregated by content_id with report_count).
**Why it happens:** The Report proto has `report_count` but the DB stores individual reports.
**How to avoid:** Query should GROUP BY content_id with COUNT(*) for report_count, and return one row per unique content item, sorted by report_count DESC.
**Warning signs:** Queue shows duplicate entries for the same post/comment.

### Pitfall 5: Pin Enforcement Across Shards
**What goes wrong:** Max 2 pins enforcement fails when posts are on different shards.
**Why it happens:** Posts are sharded by community_id, but a community's posts could theoretically span shards (though in practice with consistent hashing, one community maps to one shard).
**How to avoid:** Since consistent hashing maps one community to one shard, the pin count check (`SELECT COUNT(*) FROM posts WHERE community_id = $1 AND is_pinned = true`) works on a single shard. The moderation service needs to know which shard to query — use the same consistent hash the post-service uses, or call post-service via gRPC.
**Warning signs:** More than 2 pinned posts in a community, or pin operations fail on wrong shard.

### Pitfall 6: Auth Interceptor publicMethods Map
**What goes wrong:** New moderation/spam service methods require auth but aren't recognized by the auth interceptor, resulting in 401 on valid requests.
**Why it happens:** The auth interceptor in `internal/platform/auth/interceptor.go` has a hardcoded `publicMethods` map. Methods NOT in this map require auth (which is correct for mod actions). But GetModLog and ListReportQueue might need to be listed as requiring auth explicitly — they already do by default since they're not in the public map.
**How to avoid:** Only add methods that should be publicly accessible (like CheckContent for pre-publish validation, if called from post-service internally). Most moderation RPCs should require auth (not in publicMethods). The spam service's CheckContent is called service-to-service, not from frontend.
**Warning signs:** 401 errors on mod actions, or worse, unauthenticated access to mod-only endpoints.

### Pitfall 7: Community Members Role Check Has 'admin' Not 'owner'
**What goes wrong:** Role check assumes `role IN ('moderator', 'owner')` but the community_members table CHECK constraint uses `'admin'` not `'owner'`.
**Why it happens:** The migration at `migrations/community/001_communities.up.sql:23` uses `CHECK (role IN ('member', 'moderator', 'admin'))` — but the code at `internal/community/server.go:617` uses `getMemberRole()` which returns the raw role string. The second migration `002_add_username_and_owner_role.up.sql` may have changed this.
**How to avoid:** Check the actual role values in the database. Read migration 002 to see if 'owner' was added. The code refers to 'owner' role (CommunitySettings checks `mod.role === 'owner'`), so the second migration likely added it.
**Warning signs:** Moderator verification fails because role string doesn't match expectations.

## Code Examples

### Database Schema: Moderation Tables
```sql
-- migrations/moderation/001_moderation.up.sql

-- Reports from users and spam detection
CREATE TABLE IF NOT EXISTS reports (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    community_id    UUID        NOT NULL,
    community_name  TEXT        NOT NULL,
    content_id      UUID        NOT NULL,
    content_type    SMALLINT    NOT NULL,  -- 1=post, 2=comment
    reporter_id     UUID        NOT NULL,
    reason          TEXT        NOT NULL,
    source          TEXT        NOT NULL DEFAULT 'user',  -- 'user' or 'spam-detection'
    status          TEXT        NOT NULL DEFAULT 'active', -- 'active', 'resolved'
    resolved_action TEXT,       -- 'removed', 'dismissed', 'banned'
    resolved_by     UUID,
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reports_community_active
    ON reports (community_id, status, created_at DESC)
    WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_reports_content
    ON reports (content_id, content_type);

-- User bans per community
CREATE TABLE IF NOT EXISTS bans (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    community_id    UUID        NOT NULL,
    community_name  TEXT        NOT NULL,
    user_id         UUID        NOT NULL,
    username        TEXT        NOT NULL,
    reason          TEXT        NOT NULL,
    banned_by       UUID        NOT NULL,
    duration_seconds BIGINT     NOT NULL DEFAULT 0,  -- 0 = permanent
    expires_at      TIMESTAMPTZ,  -- NULL = permanent
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(community_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_bans_community_active
    ON bans (community_id)
    WHERE expires_at IS NULL OR expires_at > now();
CREATE INDEX IF NOT EXISTS idx_bans_user
    ON bans (user_id);

-- Moderation action log
CREATE TABLE IF NOT EXISTS mod_log (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    community_id    UUID        NOT NULL,
    community_name  TEXT        NOT NULL,
    moderator_id    UUID        NOT NULL,
    moderator_username TEXT     NOT NULL,
    action          TEXT        NOT NULL,  -- 'remove_post', 'remove_comment', 'ban_user', 'unban_user', 'pin_post', 'unpin_post', 'dismiss_report', 'restore_content'
    target_id       TEXT        NOT NULL,
    target_type     TEXT        NOT NULL,  -- 'post', 'comment', 'user'
    reason          TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mod_log_community
    ON mod_log (community_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mod_log_action
    ON mod_log (community_id, action, created_at DESC);
```

### Service Bootstrap Pattern
```go
// Source: cmd/vote/main.go — exact pattern to replicate for moderation-service
func main() {
    logger, _ := zap.NewProduction()
    cfg := config.Load("moderation")

    // PostgreSQL (moderation database)
    pool, err := database.NewPostgres(ctx, cfg.DatabaseURL)
    // Redis (DB 10 for moderation cache)
    rdb, err := platformredis.NewClient(redisURL)
    // Community-service gRPC client (for role verification)
    communityConn, _ := grpc.NewClient(communityServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    communityClient := commv1.NewCommunityServiceClient(communityConn)
    // Post-service gRPC client (for pin/remove operations)
    postConn, _ := grpc.NewClient(postServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    postClient := postv1.NewPostServiceClient(postConn)

    // gRPC server with standard middleware chain
    srv := grpcserver.New(cfg.GRPCPort, logger,
        grpcserver.WithUnaryInterceptors(
            middleware.Recovery(logger),
            middleware.Logging(logger),
            auth.UnaryInterceptor(jwtValidator),
            ratelimit.UnaryInterceptor(limiter, cfg.RateLimitEnabled),
            middleware.ErrorMapping(),
        ),
    )
    modv1.RegisterModerationServiceServer(srv.Server(), moderationServer)
    srv.Run()
}
```

### Spam Check Integration in Post Service
```go
// Called in CreatePost BEFORE persisting — synchronous check
func (s *Server) checkSpam(ctx context.Context, userID, title, body string, urls []string) error {
    resp, err := s.spamClient.CheckContent(ctx, &spamv1.CheckContentRequest{
        UserId:      userID,
        ContentType: "post",
        Content:     title + "\n" + body,
        Urls:        urls,
    })
    if err != nil {
        // Fail-open: log error but allow content through
        s.logger.Warn("spam check failed, allowing content", zap.Error(err))
        return nil
    }
    if resp.Result == spamv1.SpamCheckResult_SPAM_CHECK_RESULT_SPAM {
        return fmt.Errorf("content restricted: %w", perrors.ErrInvalidInput)
    }
    if resp.IsDuplicate {
        return fmt.Errorf("duplicate content: %w", perrors.ErrAlreadyExists)
    }
    return nil
}
```

### Report Dialog Component Pattern
```svelte
<!-- Terminal-aesthetic report reason picker -->
<script lang="ts">
  import { api } from '../../lib/api';

  const REASONS = ['Spam', 'Harassment', 'Misinformation', 'Breaks community rules', 'Other'];
  let selectedReason = $state('');
  let submitting = $state(false);
  let showToast = $state(false);

  async function submitReport() {
    if (!selectedReason || submitting) return;
    submitting = true;
    try {
      await api(`/communities/${communityName}/moderation/reports`, {
        method: 'POST',
        body: JSON.stringify({
          contentId: contentId,
          contentType: contentType,
          reason: selectedReason,
        }),
      });
      showToast = true;
      setTimeout(() => { showToast = false; onClose(); }, 2000);
    } finally {
      submitting = false;
    }
  }
</script>
```

### Docker-Compose Service Entry
```yaml
# Pattern: matches existing services exactly
moderation-service:
  build:
    context: .
    dockerfile: deploy/docker/Dockerfile
    args:
      SERVICE: moderation
  environment:
    DATABASE_URL: postgres://redyx:dev@postgres:5432/moderation?sslmode=disable
    REDIS_URL: redis://redis:6379/10
    GRPC_PORT: "50061"
    JWT_SECRET: "dev-jwt-secret-32-bytes-minimum!!"
    KAFKA_BROKERS: "kafka:9092"
    COMMUNITY_SERVICE_ADDR: "community-service:50054"
    POST_SERVICE_ADDR: "post-service:50055"
    COMMENT_SERVICE_ADDR: "comment-service:50057"
  ports: ["50061:50061"]
  depends_on:
    postgres: { condition: service_healthy }
    redis: { condition: service_healthy }
    kafka: { condition: service_healthy }
    community-service: { condition: service_started }

spam-service:
  build:
    context: .
    dockerfile: deploy/docker/Dockerfile
    args:
      SERVICE: spam
  environment:
    REDIS_URL: redis://redis:6379/11
    GRPC_PORT: "50062"
    JWT_SECRET: "dev-jwt-secret-32-bytes-minimum!!"
    KAFKA_BROKERS: "kafka:9092"
    MODERATION_SERVICE_ADDR: "moderation-service:50061"
  ports: ["50062:50062"]
  depends_on:
    redis: { condition: service_healthy }
    kafka: { condition: service_healthy }
```

### Envoy Route Configuration
```yaml
# BEFORE community catch-all (line ~110 in envoy.yaml)
# Moderation routes — regex to match /api/v1/communities/{name}/moderation/*
- match:
    safe_regex:
      regex: "/api/v1/communities/[^/]+/moderation.*"
  route:
    cluster: moderation-service
    timeout: 30s
# Spam routes
- match: { prefix: "/api/v1/spam" }
  route:
    cluster: spam-service
    timeout: 30s
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Modal confirm dialogs | Inline confirmation (confirmingX state) | Phase 3/4 | All mod actions use inline [confirm?]/[cancel] |
| Cross-service DB access | gRPC calls between services | Quick-task 003 | Moderation service MUST use gRPC to community/post/comment services |
| Zookeeper Kafka | KRaft mode (apache/kafka:3.7.2) | Phase 3 | No Zookeeper dependency for new Kafka topics |
| Single monolith DB | Database-per-service | Phase 1 | Moderation gets its own `moderation` database |

**Deprecated/outdated:**
- `gorilla/websocket`: Archived, project uses `nhooyr.io/websocket` — not relevant for this phase but noted for consistency

## Proto Changes Required

The existing moderation proto needs extensions for the report submission flow and resolved reports management. The current proto defines `ListReportQueue` and `GetModLog` but is missing:

1. **SubmitReport RPC** — Users submit reports (currently no RPC for this; `ReportSpam` on spam service is different)
2. **DismissReport RPC** — Mods dismiss a report without removing content
3. **RestoreContent RPC** — Undo a remove action (for resolved tab)
4. **ListBans RPC** — List active bans for ban management tab
5. **CheckBan RPC** — Called by post/comment services to check if user is banned before allowing create
6. **Report source field** — Distinguish user reports from spam-detection flags

Additionally, the `BanUserRequest` needs `username` field (for display in ban list) and the `BanUserRequest.duration_seconds` field semantics need to map to the preset durations (86400, 259200, 604800, 2592000, 0).

## Open Questions

1. **Spam service needs PostgreSQL or is Redis-only?**
   - What we know: Blocklists are global and relatively small. Duplicate detection uses content hashes with TTL.
   - What's unclear: Whether blocklists should be stored in PostgreSQL (survives restarts, admin-editable) or just hardcoded/config-file.
   - Recommendation: Use a config-loaded blocklist in spam-service memory. No PostgreSQL needed for spam — keep it Redis-only like vote-service. Blocklist data loaded from a JSON/text seed file at startup.

2. **How does moderation service update post `is_pinned` and `is_deleted`?**
   - What we know: Posts are sharded. Moderation service doesn't have direct DB access to post shards.
   - What's unclear: Whether to add new RPCs to post-service (e.g., `ModeratorRemovePost`, `SetPinned`) or have moderation service connect to post shard DBs directly.
   - Recommendation: Add internal gRPC RPCs to post-service and comment-service for moderator actions. This follows the established cross-service gRPC pattern (quick-task 003).

3. **Redis DB allocation for new services**
   - What we know: DBs 0-9 are allocated (skeleton=0, auth=1, user=2, community=3, post=4, vote=5, comment=6, search=7, notification=8, media=9).
   - Recommendation: moderation-service=DB 10, spam-service=DB 11.

4. **gRPC port allocation for new services**
   - What we know: Ports 50051-50060 are allocated to existing services.
   - Recommendation: moderation-service=50061, spam-service=50062.

## Sources

### Primary (HIGH confidence)
- Existing codebase: `proto/redyx/moderation/v1/moderation.proto` — already defined RPCs
- Existing codebase: `proto/redyx/spam/v1/spam.proto` — already defined RPCs
- Existing codebase: `gen/redyx/moderation/v1/` and `gen/redyx/spam/v1/` — compiled and ready
- Existing codebase: `internal/vote/kafka.go` — Kafka producer/consumer pattern
- Existing codebase: `internal/post/producer.go` — Kafka topic management pattern
- Existing codebase: `internal/platform/auth/interceptor.go` — Auth interceptor public methods map
- Existing codebase: `deploy/envoy/envoy.yaml` — Route ordering patterns
- Existing codebase: `docker-compose.yml` — Service definition patterns
- Existing codebase: `migrations/post_shard_0/001_create_posts.up.sql` — `is_pinned` column exists
- Existing codebase: `internal/community/server.go:617` — `getMemberRole()` helper
- Existing codebase: `web/src/components/community/CommunitySettings.svelte` — Mod-gated settings component
- Existing codebase: `web/src/components/comment/CommentCard.svelte:49` — Inline confirmation pattern

### Secondary (MEDIUM confidence)
- Existing codebase pattern analysis across 10 services for bootstrap, migration, and Kafka patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already used in project, no new dependencies
- Architecture: HIGH — follows identical patterns to 10 existing services, proto already defined
- Pitfalls: HIGH — identified from actual codebase inspection (envoy routing, role check values, shard concerns)
- Proto changes: HIGH — gap analysis against actual proto files and user requirements
- Frontend patterns: HIGH — CommunitySettings, CommentCard, SortBar all inspected for reuse

**Research date:** 2026-03-06
**Valid until:** 2026-04-06 (stable — all based on existing codebase patterns, no external dependency risk)
