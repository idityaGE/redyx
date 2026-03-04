# Phase 5: Search + Notifications + Media (Full Stack) - Research

**Researched:** 2026-03-05
**Domain:** Full-text search (Meilisearch), real-time WebSocket notifications, presigned-URL media uploads (S3/MinIO)
**Confidence:** HIGH

## Summary

Phase 5 introduces three independent feature verticals — search, notifications, and media — each requiring a new Go microservice, integration with existing infrastructure (Kafka, Redis, PostgreSQL, Envoy), and frontend Svelte components. All three proto definitions already exist with generated Go code in `gen/`. The existing Header.svelte has placeholder search input and notification bell that need replacement with interactive Svelte components. The PostSubmitForm.svelte has a media tab stub ready for replacement.

**Search** uses Meilisearch (already in tech stack decisions from init) with a Go search-service that indexes posts via Kafka consumption and serves queries. **Notifications** requires a new notification-service with PostgreSQL storage, Kafka consumption for events (comment replies, mentions), and a WebSocket endpoint for real-time delivery — notably, this is the first WebSocket in the project, so Envoy needs upgrade support config. **Media** uses a presigned-URL upload flow with MinIO (S3-compatible) in dev, with the media-service managing metadata in PostgreSQL and triggering thumbnail generation.

**Primary recommendation:** Implement the three backend services first (they're independent), then wire Envoy routing and Docker Compose, then build frontend components. Search and media are simpler; notifications is the most complex due to WebSocket + Kafka consumer + offline delivery.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Search experience:** Inline autocomplete dropdown + `/search?q=...` results page. Context-aware scoping (community page auto-scopes with removable pill). Feed-row style results reusing FeedRow layout pattern. Minimal empty state.
- **Notification panel:** Dropdown below bell icon (10-20 recent). Chronological list with type icon/prefix. Silent badge updates via WebSocket (no toasts, no sounds). Click to navigate + auto-mark as read. "Mark all read" button.
- **Media upload flow:** Multiple files (up to 4-5 images OR 1 video, no mixing). Drag-and-drop zone with terminal-style text. Per-file ASCII progress bar `[=========>   ] 78%`. Thumbnail previews with `[x]` remove. Submit during processing allowed (PENDING/PROCESSING status). Inline per-file errors. Stacked images with lightbox in post detail view.
- **No AWS in dev:** Use MinIO (S3-compatible, Docker) for presigned-URL flow locally.

### Claude's Discretion
- Notification preferences page layout and interaction patterns
- Search results page sort controls implementation (relevance, recency, score)
- Autocomplete dropdown styling and animation
- Lightbox component implementation details
- WebSocket reconnection strategy and offline notification delivery mechanism
- Loading skeletons and transition animations for all three features
- Exact terminal-aesthetic styling for new components

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| SRCH-01 | User can search posts by title and body text via Meilisearch (results within 300ms) | Meilisearch Go SDK + search-service architecture, indexing via Kafka consumer |
| SRCH-02 | User can search within a specific community or globally | Proto `community_name` filter on `SearchPostsRequest`, Meilisearch filterable attributes |
| SRCH-03 | Community name autocomplete in search bar (prefix-based, cached in Redis, triggers after 2+ chars) | Redis ZRANGEBYLEX for sorted set prefix matching, or Meilisearch search on community index |
| SRCH-04 | Search results ranked by relevance, recency, and vote score | Meilisearch ranking rules + sortable attributes configuration |
| NOTF-01 | User receives notification when someone replies to their post or comment | Kafka consumer on new topic `redyx.comments.v1` published by comment-service |
| NOTF-02 | User receives notification when mentioned with u/username | Regex mention detection in comment body during creation |
| NOTF-03 | Notifications delivered in real time via WebSocket (within 1 second) | gorilla/websocket or nhooyr.io/websocket, Envoy WebSocket upgrade config |
| NOTF-04 | Offline notifications stored in PostgreSQL and delivered on next WebSocket connection | Notification store with `is_read` + `delivered_at` tracking |
| NOTF-05 | User can mark individual or all notifications as read | Proto RPCs already defined: MarkRead, MarkAllRead |
| NOTF-06 | User can configure notification preferences (mute communities, mute reply types) | Proto RPCs: GetPreferences, UpdatePreferences with muted_communities |
| MDIA-01 | User can upload images and videos when creating a post | Presigned URL flow: InitUpload → client PUT → CompleteUpload |
| MDIA-02 | Uploaded files validated for type (JPEG, PNG, GIF, WebP) and size (20MB image, 100MB video) | Server-side validation in InitUpload + content-type verification in CompleteUpload |
| MDIA-03 | Thumbnails generated for image uploads (max 320px wide) | Go `imaging` library (disintegration/imaging) for resize, or `bimg` for libvips |
| MDIA-04 | Media stored in AWS S3 and served through CloudFront CDN | MinIO in dev (S3-compatible), AWS SDK v2 Go for presigned URLs |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| meilisearch-go | v0.29+ | Go SDK for Meilisearch full-text search engine | Official Go client, project tech stack decision |
| Meilisearch | v1.12+ (Docker) | Full-text search engine | Already in project tech stack; fast (<50ms), typo-tolerant, easy to operate |
| aws-sdk-go-v2 | v1.36+ | S3 presigned URL generation for media uploads | Standard Go AWS SDK v2 for MinIO/S3 compatible API |
| nhooyr.io/websocket | v1.8+ | WebSocket server for real-time notifications | Lightweight, idiomatic Go, context-aware, better than gorilla/websocket (archived) |
| disintegration/imaging | v1.6+ | Image thumbnail generation (resize, crop) | Pure Go, no CGO, simple API for resize operations |
| MinIO | RELEASE.2024+ (Docker) | S3-compatible object storage for local dev | S3 API compatible, single binary, user explicitly requested for dev |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| twmb/franz-go | (already in go.mod) | Kafka producer/consumer for notification + search indexing events | Comment-service publishes events, notification-service and search-service consume |
| redis/go-redis/v9 | (already in go.mod) | Autocomplete cache, notification unread counts, WebSocket session tracking | Community name prefix cache, unread count cache |
| jackc/pgx/v5 | (already in go.mod) | PostgreSQL for notification + media metadata storage | Notification store, media metadata store |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| nhooyr.io/websocket | gorilla/websocket | gorilla/websocket is archived/unmaintained; nhooyr.io is actively maintained, context-aware |
| disintegration/imaging | bimg (libvips) | bimg is faster but requires CGO + libvips C library; imaging is pure Go, simpler for thumbnails only |
| MinIO | LocalStack | LocalStack emulates more AWS services but is heavier; MinIO is lighter and S3-focused |
| Meilisearch | PostgreSQL full-text search | pg_tsvector works but lacks typo tolerance, ranking quality, and <300ms at scale. Meilisearch is a locked tech choice |

**Installation:**
```bash
# Go dependencies (run from project root)
go get github.com/meilisearch/meilisearch-go@latest
go get nhooyr.io/websocket@latest
go get github.com/disintegration/imaging@latest
go get github.com/aws/aws-sdk-go-v2@latest
go get github.com/aws/aws-sdk-go-v2/config@latest
go get github.com/aws/aws-sdk-go-v2/service/s3@latest
go get github.com/aws/aws-sdk-go-v2/feature/s3/manager@latest
```

## Architecture Patterns

### Recommended Project Structure
```
# Three new backend services following existing pattern
cmd/search/main.go              # Search service entry point
cmd/notification/main.go        # Notification service entry point
cmd/media/main.go               # Media service entry point

internal/search/
├── server.go                   # gRPC server (SearchPosts, AutocompleteCommunities)
├── indexer.go                  # Kafka consumer — indexes posts into Meilisearch
└── meili.go                    # Meilisearch client wrapper

internal/notification/
├── server.go                   # gRPC server (ListNotifications, MarkRead, etc.)
├── store.go                    # PostgreSQL notification + preferences storage
├── consumer.go                 # Kafka consumer — creates notifications from events
├── websocket.go                # WebSocket hub — manages connections, pushes notifications
└── mention.go                  # u/username mention detection regex

internal/media/
├── server.go                   # gRPC server (InitUpload, CompleteUpload, GetMedia)
├── store.go                    # PostgreSQL media metadata storage
├── s3.go                       # S3/MinIO presigned URL generation
└── thumbnail.go                # Image thumbnail generation

migrations/search/              # (empty — Meilisearch manages its own indexes)
migrations/notification/
└── 001_notifications.up.sql    # notifications + notification_preferences tables
migrations/media/
└── 001_media.up.sql            # media_items table

# Frontend components
web/src/components/search/
├── SearchBar.svelte            # Replaces Header search input, with autocomplete dropdown
├── SearchResults.svelte        # Search results page component
└── SearchResultRow.svelte      # Individual result row (FeedRow-like)

web/src/components/notification/
├── NotificationBell.svelte     # Bell icon with unread count badge
├── NotificationDropdown.svelte # Dropdown panel with notification list
├── NotificationItem.svelte     # Individual notification row
└── NotificationPreferences.svelte  # Preferences page component

web/src/components/media/
├── MediaUpload.svelte          # Drag-and-drop upload zone with progress
├── MediaPreview.svelte         # Thumbnail preview grid with remove buttons
├── MediaGallery.svelte         # Stacked images view in post detail
└── Lightbox.svelte             # Fullscreen image viewer with prev/next

web/src/pages/search.astro      # /search?q=... results page
web/src/pages/notifications.astro  # Full notifications page
web/src/pages/settings/notifications.astro  # Notification preferences page
web/src/lib/websocket.ts        # WebSocket client with reconnection logic
```

### Pattern 1: Kafka Event Pipeline (Comment → Notification + Search)
**What:** Comment-service publishes a new Kafka event when a comment is created. Notification-service and search-service consume independently.
**When to use:** Any cross-service event communication (already established in vote → post/user pipeline).
**Example:**
```go
// New event proto needed in events.proto
// CommentEvent published when a comment is created
message CommentEvent {
  string event_id = 1;
  string comment_id = 2;
  string post_id = 3;
  string author_id = 4;
  string author_username = 5;
  string parent_comment_id = 6;     // empty for top-level
  string parent_comment_author_id = 7;
  string post_author_id = 8;
  string community_name = 9;
  string body = 10;                 // for mention detection + search indexing
  google.protobuf.Timestamp created_at = 11;
}

// Also: PostEvent for search indexing (published by post-service)
message PostEvent {
  string event_id = 1;
  string post_id = 2;
  string title = 3;
  string body = 4;
  string author_username = 5;
  string community_name = 6;
  int32 vote_score = 7;
  google.protobuf.Timestamp created_at = 8;
}
```

### Pattern 2: Presigned URL Upload Flow
**What:** Client requests a presigned URL from media-service, uploads directly to MinIO/S3, then confirms completion. Media-service never handles file bytes.
**When to use:** All file uploads (images, videos).
**Flow:**
```
1. Client → POST /api/v1/media/upload (filename, content_type, size_bytes, media_type)
2. Server validates type/size → creates media record (PENDING) → generates presigned PUT URL
3. Server → returns { media_id, upload_url, expires_at }
4. Client → PUT upload_url (direct to MinIO/S3 with file bytes + Content-Type header)
5. Client → POST /api/v1/media/{media_id}/complete
6. Server → verifies object exists in S3 → updates status to PROCESSING → triggers thumbnail
7. Server → generates thumbnail → updates status to READY → returns { url, thumbnail_url }
```

### Pattern 3: WebSocket Hub for Real-Time Notifications
**What:** Notification-service runs a WebSocket hub that maps user_id → active connection(s). Kafka consumer pushes new notifications to connected users.
**When to use:** Real-time notification delivery.
**Key design:**
```go
// WebSocket hub manages active connections per user
type Hub struct {
    mu    sync.RWMutex
    conns map[string][]*websocket.Conn  // user_id → connections
}

// Register: when user connects via WebSocket
// Unregister: when connection closes
// Send: Kafka consumer calls hub.Send(userID, notification) 
//        for each connected user matching the notification target
```

### Pattern 4: Envoy WebSocket Support
**What:** Envoy needs `upgrade_configs` for WebSocket upgrade on the notification route.
**Critical:** This is the first WebSocket in the project — Envoy's gRPC-JSON transcoder does NOT handle WebSocket. The WebSocket endpoint must bypass the transcoder.
**Approach:** Add a separate HTTP route for `/api/v1/ws/notifications` that routes directly to notification-service WITHOUT the gRPC transcoder. The notification-service exposes both gRPC (for REST API calls) and a raw HTTP WebSocket handler.

### Pattern 5: Context-Aware Search Scoping (Frontend)
**What:** SearchBar component detects current route to auto-scope search queries.
**When to use:** Community pages auto-scope, global pages don't.
```typescript
// SearchBar.svelte detects if on community page
let communityScope = $derived(
  window.location.pathname.match(/^\/community\/([^/]+)/)?.[1] ?? null
);
// Removable pill shows community name, clicking X removes scope
```

### Anti-Patterns to Avoid
- **Proxying file uploads through the API gateway:** File bytes should go directly to MinIO/S3 via presigned URL, never through Envoy or the media-service
- **Polling for notifications:** Use WebSocket, not polling. No setInterval-based notification checks
- **Indexing posts synchronously:** Search indexing must be async via Kafka, never in the post-creation request path
- **Single WebSocket connection for all features:** Only notifications use WebSocket; don't over-engineer a generic real-time system
- **Storing notification read state in Redis only:** PostgreSQL is the source of truth for notification history; Redis caches unread counts

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Full-text search | Custom inverted index or pg_tsvector | Meilisearch | Typo tolerance, relevance ranking, sub-50ms queries, faceting |
| Image thumbnails | Manual pixel manipulation | disintegration/imaging | EXIF handling, format conversion, resize algorithms |
| S3 presigned URLs | Custom HMAC signing | AWS SDK v2 `s3.PresignClient` | Signature format changes, expiry handling, region scoping |
| WebSocket protocol | Raw TCP/HTTP upgrade handling | nhooyr.io/websocket | Ping/pong, close handshake, compression, context cancellation |
| Mention detection | Custom parser | `regexp.MustCompile(`(?:^|\s)u/([a-zA-Z0-9_]{3,20})`)` | Simple regex is sufficient for u/username pattern |
| Debounced search input | Manual setTimeout | Svelte `$effect` + timer pattern | Proper cleanup on component destroy |

**Key insight:** Each of these three domains (search, real-time, media) has well-established patterns and libraries. The complexity is in integration (Kafka pipelines, Envoy routing, frontend state), not in the individual features.

## Common Pitfalls

### Pitfall 1: Envoy WebSocket vs gRPC Transcoder Conflict
**What goes wrong:** WebSocket upgrade requests get intercepted by the gRPC-JSON transcoder filter, which doesn't understand WebSocket protocol and rejects the connection.
**Why it happens:** All routes currently go through the transcoder. WebSocket needs raw HTTP passthrough.
**How to avoid:** Define the WebSocket route BEFORE the transcoder filter, or use a separate listener/route that bypasses the transcoder entirely. The notification service should expose WebSocket on a separate HTTP port or use a path prefix that Envoy routes to a non-transcoded cluster.
**Warning signs:** WebSocket connection immediately closes with 400/404, or upgrade headers are stripped.
**Recommended approach:** Add the notification-service WebSocket endpoint as a raw HTTP route in Envoy with `upgrade_configs: [{upgrade_type: websocket}]` on the route, and ensure the cluster for WebSocket uses HTTP/1.1 (not HTTP/2) since WebSocket requires HTTP/1.1 upgrade.

### Pitfall 2: MinIO Presigned URL CORS
**What goes wrong:** Browser blocks direct PUT to MinIO because CORS headers aren't configured on the MinIO bucket.
**Why it happens:** Client-side uploads go directly to MinIO (different origin than the Envoy gateway).
**How to avoid:** Configure MinIO bucket CORS policy to allow PUT from the frontend origin. In docker-compose, MinIO needs `MINIO_BROWSER_REDIRECT_URL` and bucket CORS via `mc` CLI or startup script.
**Warning signs:** Browser console shows CORS errors on PUT to presigned URL.

### Pitfall 3: Kafka Consumer Group Conflicts
**What goes wrong:** Multiple services consume from the same topic but interfere with each other's offsets.
**Why it happens:** Using the same consumer group ID across services.
**How to avoid:** Each service uses a unique consumer group: `search-service.redyx.posts.v1`, `notification-service.redyx.comments.v1`, etc. This is already the pattern used by vote consumers.
**Warning signs:** Messages processed by wrong service, or messages skipped.

### Pitfall 4: WebSocket Connection Lifecycle with Auth
**What goes wrong:** WebSocket connections use stale or expired JWT tokens and can't refresh.
**Why it happens:** WebSocket doesn't support custom headers after initial handshake. Token refresh requires a new connection.
**How to avoid:** Pass JWT as query parameter on initial WebSocket connect (`ws://host/api/v1/ws/notifications?token=...`). Server validates token on upgrade. Client reconnects with fresh token when connection drops or server sends auth-expired message.
**Warning signs:** WebSocket connects but immediately disconnects after token expiry.

### Pitfall 5: Meilisearch Index Drift
**What goes wrong:** Meilisearch index gets out of sync with PostgreSQL post data (edited posts, deleted posts, score changes).
**Why it happens:** Only indexing on creation, not on updates or deletes.
**How to avoid:** Publish PostEvent on create, update, and delete. Search indexer handles all three operations. For vote scores, batch-update Meilisearch periodically (not on every vote — too noisy).
**Warning signs:** Search returns deleted posts, or outdated titles/bodies.

### Pitfall 6: Large File Upload Timeout
**What goes wrong:** Video uploads (up to 100MB) time out during direct PUT to MinIO.
**Why it happens:** Default presigned URL expiry too short, or MinIO connection timeout.
**How to avoid:** Set presigned URL expiry to 1 hour for video uploads. Client shows progress via XMLHttpRequest (not fetch, which doesn't support upload progress events).
**Warning signs:** Large uploads fail silently, or presigned URL expires mid-upload.

### Pitfall 7: Notification Storm on Popular Posts
**What goes wrong:** Post author gets hundreds of notifications for a viral post, overwhelming WebSocket and database.
**Why it happens:** Every reply generates a notification for the post author.
**How to avoid:** Rate-limit notifications per target: after N notifications in a time window, collapse into "X more replies" summary. Or defer to preference settings (mute communities).
**Warning signs:** WebSocket flood, notification table growing rapidly.

## Code Examples

### Meilisearch Go Client — Index and Search Posts
```go
// Source: meilisearch-go official README
import "github.com/meilisearch/meilisearch-go"

// Initialize client
client := meilisearch.New("http://meilisearch:7700", meilisearch.WithAPIKey("dev-master-key"))

// Configure index
index := client.Index("posts")
index.UpdateSettings(&meilisearch.Settings{
    SearchableAttributes: []string{"title", "body"},
    FilterableAttributes: []string{"communityName"},
    SortableAttributes:   []string{"createdAt", "voteScore"},
    RankingRules: []string{
        "words", "typo", "proximity", "attribute", "sort", "exactness",
    },
})

// Index a document
index.AddDocuments(map[string]interface{}{
    "id":              postID,
    "title":           title,
    "body":            body,
    "authorUsername":  authorUsername,
    "communityName":  communityName,
    "voteScore":      voteScore,
    "commentCount":   commentCount,
    "createdAt":      createdAt.Unix(),
})

// Search
searchRes, _ := index.Search("query text", &meilisearch.SearchRequest{
    Filter:               "communityName = 'golang'",
    Sort:                 []string{"voteScore:desc"},
    Limit:                25,
    AttributesToHighlight: []string{"title", "body"},
    HighlightPreTag:      "<mark>",
    HighlightPostTag:     "</mark>",
    AttributesToCrop:     []string{"body:50"},
})
```

### WebSocket Server with nhooyr.io/websocket
```go
// Source: nhooyr.io/websocket docs
import "nhooyr.io/websocket"
import "nhooyr.io/websocket/wsjson"

func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Validate JWT from query parameter
    token := r.URL.Query().Get("token")
    claims, err := h.jwtValidator.Validate(token)
    if err != nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
        OriginPatterns: []string{"*"}, // Tighten in production
    })
    if err != nil {
        return
    }
    defer conn.CloseNow()

    // Register connection
    h.register(claims.UserID, conn)
    defer h.unregister(claims.UserID, conn)

    // Deliver offline notifications
    h.deliverOffline(r.Context(), claims.UserID, conn)

    // Keep connection alive — read loop handles client pings
    for {
        _, _, err := conn.Read(r.Context())
        if err != nil {
            break
        }
    }
}

// Send notification to all active connections for a user
func (h *Hub) Send(userID string, notification *Notification) {
    h.mu.RLock()
    conns := h.conns[userID]
    h.mu.RUnlock()
    
    for _, conn := range conns {
        wsjson.Write(context.Background(), conn, notification)
    }
}
```

### MinIO Presigned URL Generation
```go
// Source: AWS SDK v2 docs (MinIO compatible)
import (
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

func NewS3Client(endpoint, accessKey, secretKey, region string) *s3.Client {
    return s3.New(s3.Options{
        BaseEndpoint: aws.String(endpoint),   // "http://minio:9000" in dev
        Region:       region,                  // "us-east-1"
        Credentials:  aws.NewCredentialsCache(
            credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
        ),
        UsePathStyle: true,  // CRITICAL for MinIO — virtual-hosted style won't work
    })
}

func (s *S3Store) GeneratePresignedPUT(ctx context.Context, key, contentType string, sizeBytes int64) (string, time.Time, error) {
    presigner := s3.NewPresignClient(s.client)
    
    req, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
        Bucket:        aws.String(s.bucket),
        Key:           aws.String(key),
        ContentType:   aws.String(contentType),
        ContentLength: aws.Int64(sizeBytes),
    }, s3.WithPresignExpires(1*time.Hour))
    if err != nil {
        return "", time.Time{}, err
    }
    
    return req.URL, time.Now().Add(1*time.Hour), nil
}
```

### Frontend Upload with XHR Progress
```typescript
// XMLHttpRequest for upload progress (fetch API doesn't support upload progress)
function uploadToPresignedURL(
  url: string,
  file: File,
  onProgress: (percent: number) => void
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('PUT', url);
    xhr.setRequestHeader('Content-Type', file.type);
    
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };
    
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new Error(`Upload failed: ${xhr.status}`));
    };
    xhr.onerror = () => reject(new Error('Upload failed'));
    
    xhr.send(file);
  });
}
```

### WebSocket Client with Reconnection (Frontend)
```typescript
// web/src/lib/websocket.ts
export function createNotificationSocket(token: string, onMessage: (data: any) => void) {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  let ws: WebSocket | null = null;
  let reconnectTimer: number | null = null;
  let reconnectDelay = 1000; // Start at 1s, exponential backoff

  function connect() {
    ws = new WebSocket(`${protocol}//${location.host}/api/v1/ws/notifications?token=${token}`);
    
    ws.onopen = () => { reconnectDelay = 1000; }; // Reset backoff
    ws.onmessage = (e) => onMessage(JSON.parse(e.data));
    ws.onclose = () => {
      // Exponential backoff with jitter, max 30s
      const jitter = Math.random() * 1000;
      reconnectTimer = window.setTimeout(connect, Math.min(reconnectDelay + jitter, 30000));
      reconnectDelay *= 2;
    };
  }
  
  connect();
  return {
    close: () => {
      if (reconnectTimer) clearTimeout(reconnectTimer);
      ws?.close();
    }
  };
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| gorilla/websocket | nhooyr.io/websocket | 2023 (gorilla archived) | gorilla is unmaintained; nhooyr.io is the standard replacement |
| AWS SDK v1 | AWS SDK v2 | 2023 | v2 has better modularity, presign API, context support |
| Meilisearch v0.x | Meilisearch v1.x+ | 2023 | Stable API, multi-index search, hybrid search |
| XMLHttpRequest for uploads | Still XHR for progress | Ongoing | Fetch API still lacks upload progress events in 2026 |

**Deprecated/outdated:**
- gorilla/websocket: Archived, no security patches. Use nhooyr.io/websocket
- AWS SDK v1 for Go: Maintenance mode. v2 is the standard

## Open Questions

1. **Meilisearch API key management**
   - What we know: Meilisearch requires a master key in production, which generates admin and search-only API keys
   - What's unclear: Whether search-service should use admin key (for indexing) and expose a separate search-only key, or use admin key for everything in v1
   - Recommendation: Use master key in dev, single admin key for the search-service (it's the only consumer). Defer key rotation to production hardening.

2. **Comment-service Kafka producer**
   - What we know: Comment-service currently has no Kafka producer — it only consumes vote events. It needs to publish CommentEvents for notification-service.
   - What's unclear: Whether to add producer to existing comment-service or create a separate CDC pipeline
   - Recommendation: Add Kafka producer directly to comment-service (same pattern as vote-service's producer). Simplest, follows existing pattern.

3. **Post events for search indexing**
   - What we know: Post-service currently publishes no Kafka events (vote-service publishes, post-service consumes)
   - What's unclear: Whether to add post event publishing to post-service or use a database CDC approach
   - Recommendation: Add Kafka producer to post-service for create/update/delete events. Same pattern as vote-service.

4. **Notification WebSocket endpoint routing through Envoy**
   - What we know: All current routes use gRPC-JSON transcoder. WebSocket needs HTTP/1.1 passthrough.
   - What's unclear: Exact Envoy configuration for mixed gRPC + WebSocket on same listener
   - Recommendation: Add WebSocket route BEFORE the transcoder filter routes. Use `upgrade_configs` on the route action. Notification-service runs a separate HTTP server (alongside gRPC) for the WebSocket endpoint. Envoy cluster for WebSocket uses HTTP/1.1 (not HTTP/2).

5. **Redis DB assignments for new services**
   - What we know: DB 0=skeleton, 1=auth, 2=user, 3=community, 4=post, 5=vote, 6=comment
   - What's unclear: N/A — clear pattern
   - Recommendation: DB 7=search, DB 8=notification, DB 9=media

6. **Video thumbnail generation**
   - What we know: Image thumbnails are straightforward with `imaging` library. Video first-frame extraction requires ffmpeg or similar
   - What's unclear: Whether to require ffmpeg in the Docker image for video thumbnails
   - Recommendation: For v1, skip video thumbnails (return empty thumbnail_url for videos). Video transcoding is deferred to v2 (CONT-01). User decision noted video shows "first frame" — this can be a future enhancement. The media tab already shows a generic video icon pattern.

## Infrastructure Changes Required

### Docker Compose Additions
```yaml
# New services needed in docker-compose.yml
meilisearch:
  image: getmeili/meilisearch:v1.12
  ports: ["7700:7700"]
  environment:
    MEILI_MASTER_KEY: "dev-master-key"
    MEILI_ENV: development
  volumes:
    - meili-data:/meili_data

minio:
  image: minio/minio:latest
  ports:
    - "9000:9000"    # S3 API
    - "9001:9001"    # Console
  environment:
    MINIO_ROOT_USER: minioadmin
    MINIO_ROOT_PASSWORD: minioadmin
  command: server /data --console-address ":9001"
  volumes:
    - minio-data:/data

minio-init:
  image: minio/mc:latest
  depends_on: [minio]
  entrypoint: >
    /bin/sh -c "
    sleep 3 &&
    mc alias set local http://minio:9000 minioadmin minioadmin &&
    mc mb local/redyx-media --ignore-existing &&
    mc anonymous set download local/redyx-media &&
    exit 0
    "

search-service:
  # Port 50058, Redis DB 7, Meilisearch + Kafka
  
notification-service:
  # Port 50059, Redis DB 8, PostgreSQL (notifications DB) + Kafka
  # Also exposes HTTP port 8081 for WebSocket endpoint
  
media-service:
  # Port 50060, Redis DB 9, PostgreSQL (media DB) + MinIO
```

### Envoy Configuration Changes
- Add clusters: search-service (:50058), notification-service (:50059), media-service (:50060), notification-ws (:8081, HTTP/1.1)
- Add routes: `/api/v1/search/` → search-service, `/api/v1/notifications` → notification-service, `/api/v1/media/` → media-service
- Add WebSocket route: `/api/v1/ws/notifications` → notification-ws cluster with `upgrade_configs: [{upgrade_type: websocket}]`
- Add services to transcoder: `redyx.search.v1.SearchService`, `redyx.notification.v1.NotificationService`, `redyx.media.v1.MediaService`

### PostgreSQL Databases
Add to `init-databases.sql`:
```sql
CREATE DATABASE notifications;
CREATE DATABASE media;
GRANT ALL PRIVILEGES ON DATABASE notifications TO redyx;
GRANT ALL PRIVILEGES ON DATABASE media TO redyx;
```

### New Kafka Topics
- `redyx.comments.v1` — published by comment-service on create, consumed by notification-service
- `redyx.posts.v1` — published by post-service on create/update/delete, consumed by search-service

### Config.go Updates
Add to platform config: `MeilisearchURL`, `MeilisearchAPIKey`, `MinIOEndpoint`, `MinIOAccessKey`, `MinIOSecretKey`, `MinIOBucket`, `WebSocketPort`

## Sources

### Primary (HIGH confidence)
- Codebase analysis: proto definitions (`proto/redyx/{search,notification,media}/v1/*.proto`) — verified API contracts
- Codebase analysis: existing service patterns (cmd/*/main.go, internal/*/server.go) — verified bootstrap patterns
- Codebase analysis: Kafka producer/consumer patterns (internal/vote/kafka.go, internal/comment/kafka.go) — verified event pipeline
- Codebase analysis: Envoy configuration (deploy/envoy/envoy.yaml) — verified routing patterns
- Codebase analysis: Docker Compose (docker-compose.yml) — verified service orchestration
- Codebase analysis: Frontend components (Header.svelte, PostSubmitForm.svelte, FeedRow.svelte) — verified integration points

### Secondary (MEDIUM confidence)
- meilisearch-go SDK: API patterns for Go client (standard usage, well-documented)
- nhooyr.io/websocket: Go WebSocket library API (actively maintained, well-documented)
- AWS SDK v2 Go: Presigned URL generation with S3 (standard AWS pattern)
- MinIO: S3-compatible local dev (widely used, well-documented Docker setup)
- Envoy WebSocket support: `upgrade_configs` on route level (documented in Envoy docs)

### Tertiary (LOW confidence)
- Video thumbnail generation approach — deferred to future phase, pure Go solutions are limited without ffmpeg

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries are established, versions current, no ambiguity
- Architecture: HIGH — follows existing service patterns exactly, proto contracts already defined
- Pitfalls: HIGH — based on concrete codebase analysis (Envoy config, Kafka patterns, Redis DB assignments)
- Frontend: HIGH — integration points clearly identified (Header.svelte, PostSubmitForm.svelte, FeedRow.svelte)
- WebSocket/Envoy integration: MEDIUM — first WebSocket in project, Envoy config needs careful handling

**Research date:** 2026-03-05
**Valid until:** 2026-04-04 (30 days — stable domain, established libraries)
