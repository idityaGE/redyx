# Redyx

Redyx is an anonymous, community-driven discussion platform inspired by Reddit, built from scratch as a distributed microservice system. Users create communities, post content (text, links, media), comment in nested threads, and vote on everything -- all under pseudonymous usernames. The platform is designed around user privacy: no real names, no exposed emails, no stored IP addresses.

The backend consists of **12 Go microservices** communicating over **gRPC**, fronted by an **Envoy API gateway** that translates REST/JSON requests from the **Astro + Svelte** frontend. Asynchronous workflows (vote processing, search indexing, notifications, spam analysis) flow through **Apache Kafka**. The system is fully containerized and deployable on **Kubernetes** with a complete observability stack.

---

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Tech Stack](#tech-stack)
- [Services](#services)
  - [Auth Service](#1-auth-service)
  - [User Service](#2-user-service)
  - [Community Service](#3-community-service)
  - [Post Service](#4-post-service)
  - [Comment Service](#5-comment-service)
  - [Vote Service](#6-vote-service)
  - [Search Service](#7-search-service)
  - [Media Service](#8-media-service)
  - [Notification Service](#9-notification-service)
  - [Moderation Service](#10-moderation-service)
  - [Spam and Abuse Detection Service](#11-spam-and-abuse-detection-service)
  - [Rate Limiting](#12-rate-limiting)
- [Database Architecture](#database-architecture)
- [Post Sharding with Consistent Hashing](#post-sharding-with-consistent-hashing)
- [Communication Patterns](#communication-patterns)
- [Caching Strategy](#caching-strategy)
- [Security and Privacy](#security-and-privacy)
- [Observability](#observability)
- [Deployment](#deployment)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)

---

## Architecture Overview

```
                        +------------------+
                        |   Cloudflare CDN |
                        +--------+---------+
                                 |
                        +--------v---------+
                        |   Astro Frontend  |  (SSR + Svelte Islands)
                        +--------+---------+
                                 | REST/JSON
                        +--------v---------+
                        |   Envoy Gateway   |
                        |   - REST -> gRPC  |
                        |   - Rate limiting |
                        |   - JWT validation|
                        +--------+---------+
                                 | gRPC (internal)
              +------------------+------------------+
              |         |        |        |         |
         +----v---+ +---v----+ +v------+ +v------+ +v--------+
         | Auth   | | User   | | Post  | | Vote  | | Comment |
         | Service| | Service| | Svc   | | Svc   | | Service |
         +----+---+ +---+----+ +---+---+ +---+---+ +----+----+
              |         |         |         |            |
         +----v---+ +---v----+ +--v----+ +-v------+ +---v-----+
         |Postgres| |Postgres| |Postgres| | Redis  | | ScyllaDB|
         | (auth) | | (user) | |(shard) | | +Kafka | |         |
         +--------+ +--------+ +-------+ +--------+ +---------+

         +------------------+------------------+------------------+
         |         |        |        |         |                  |
    +----v-----+ +v------+ +v--------+ +------v-------+ +--------v-------+
    | Community| | Search| | Media    | | Notification | | Moderation/Spam|
    | Service  | | Svc   | | Service  | | Service      | | Services       |
    +----+-----+ +---+---+ +----+----+ +------+-------+ +--------+-------+
         |            |          |              |                  |
    +----v---+  +----v-----+ +--v-----+  +----v----+       +-----v----+
    |Postgres|  |Meilisearch| | MinIO  |  | Redis   |       | Postgres |
    |(commty)|  +----------+ | (S3)   |  | +Kafka  |       | +Redis   |
    +--------+               +--------+  | +PG     |       | +Kafka   |
                                          +---------+       +----------+
```

---

## Tech Stack

| Layer | Technology | Purpose |
|---|---|---|
| **Frontend** | Astro (SSR) + Svelte (Islands) | Content-driven pages with minimal JS; interactive components hydrate as Svelte islands |
| **API Gateway** | Envoy Proxy | REST-to-gRPC transcoding, rate limiting, CORS, load balancing, WebSocket upgrades |
| **Backend** | Go (Golang) | All 12 microservices; high concurrency, strong gRPC ecosystem |
| **Service Communication** | gRPC + Protocol Buffers | Type-safe, binary protocol for inter-service calls; 15 `.proto` definitions |
| **Frontend-to-Backend** | REST/JSON | Standard HTTP via Envoy's gRPC-JSON transcoder filter |
| **Message Queue** | Apache Kafka | Event-driven workflows: vote processing, search indexing, notifications, spam analysis |
| **Cache / Real-Time State** | Redis | Session store, vote counts, rate limiting, hot feeds, OTP codes, WebSocket registry |
| **Primary Database** | PostgreSQL 16 | ACID-compliant relational storage across 5 instances (one sharded) |
| **Comment Store** | ScyllaDB 6.2 | High-throughput wide-column store for nested comment trees |
| **Search Engine** | Meilisearch v1.12 | Full-text search with typo tolerance, ranked by relevance + recency + score |
| **Object Storage** | MinIO (S3-compatible) | Image/video uploads, thumbnails; production uses AWS S3 + CloudFront |
| **Orchestration** | Kubernetes (kind for local dev) | Helm charts, HPA, namespace isolation, NGINX Ingress |
| **Monitoring** | Prometheus + Grafana | Metrics collection via `/metrics` endpoints; per-service dashboards |
| **Logging** | Loki + Promtail | Centralized structured JSON log aggregation, queryable through Grafana |
| **Tracing** | OpenTelemetry + Jaeger | Distributed request tracing across all microservices |
| **Protobuf Tooling** | Buf | Linting, breaking change detection, code generation |
| **Password Hashing** | Argon2id | Memory-hard, GPU-resistant hashing (RFC 9106 parameters) |
| **Auth Tokens** | JWT (access 15 min + refresh 7 days) | Stateless authentication with short-lived access and long-lived refresh tokens |
| **OAuth** | Google OAuth 2.0 | Social login; user still picks a username |

---

## Services

### 1. Auth Service

**Port:** 50052 | **Database:** PostgreSQL (`auth`) | **Proto:** `redyx.auth.v1.AuthService`

Handles all authentication workflows. Stores only credentials -- no profile data.

```
Auth Service
|-- Registration
|   |-- Email + username + password registration
|   |-- Google OAuth registration (fetches email, user picks username)
|   |-- Email verification via 6-digit OTP (5 min TTL, stored in Redis)
|   |-- Account activation only after OTP verification
|
|-- Login / Logout
|   |-- Email + password login
|   |-- Google OAuth login
|   |-- JWT issuance: access token (15 min) + refresh token (7 days)
|   |-- Logout invalidates refresh token (blacklisted in Redis)
|
|-- Token Management
|   |-- Token refresh (rotate access token using valid refresh token)
|   |-- JWT validation at Envoy gateway level (no round-trip per request)
|
|-- Password Security
|   |-- Argon2id hashing (64 MiB memory, 1 iteration, 4 parallelism, 32-byte key)
|   |-- PHC string format storage ($argon2id$v=19$...)
|   |-- Constant-time comparison to prevent timing attacks
|
|-- Password Reset
|   |-- OTP-based password reset flow via email
|
|-- Privacy
    |-- Email and auth method never exposed through any API
    |-- Email encrypted at rest
```

**Key implementation files:**
- `internal/auth/server.go` -- gRPC handler implementations
- `internal/auth/hasher.go` -- Argon2id password hashing with RFC 9106 parameters
- `internal/auth/jwt.go` -- JWT token generation and validation
- `internal/auth/oauth.go` -- Google OAuth token exchange
- `internal/auth/otp.go` -- OTP generation and verification
- `internal/auth/email.go` -- Email delivery (OTP, password reset)

---

### 2. User Service

**Port:** 50053 | **Database:** PostgreSQL (`user_profiles`) | **Proto:** `redyx.user.v1.UserService`

Manages user profiles and karma. Separated from Auth to isolate profile data from credentials.

```
User Service
|-- Profile Management
|   |-- Public profile: username, karma, cake day (join date)
|   |-- Post and comment history aggregation (cross-service via gRPC)
|   |-- Display settings and avatar management
|
|-- Karma System
|   |-- Eventually consistent computation from Kafka vote events
|   |-- Aggregated from upvotes received on posts and comments
|   |-- Cached in Redis (10 min TTL)
|
|-- Account Lifecycle
|   |-- Account deletion: true PII purge
|   |-- Posts become [deleted], vote records anonymized
|
|-- Cross-Service Communication
    |-- Calls Post Service for user's post history
    |-- Calls Comment Service for user's comment history
    |-- Calls Community Service for user's community memberships
    |-- Consumes Kafka vote events for karma updates
```

---

### 3. Community Service

**Port:** 50054 | **Database:** PostgreSQL (`community`) | **Proto:** `redyx.community.v1.CommunityService`

CRUD for communities (subreddits), membership management, and role assignment.

```
Community Service
|-- Community CRUD
|   |-- Create community with unique, immutable name (r/name)
|   |-- Update description, rules, banner, icon
|   |-- Community metadata heavily cached in Redis (1 hour TTL)
|
|-- Membership
|   |-- Join / leave communities (many-to-many user <-> community)
|   |-- Member count tracking
|   |-- List user's subscribed communities
|
|-- Roles and Permissions
|   |-- Member: post, comment, vote
|   |-- Moderator: remove content, ban users, edit settings
|   |-- Creator/Owner: automatic moderator, can assign other moderators
|
|-- Visibility Levels
|   |-- Public: anyone can view and post
|   |-- Restricted: anyone can view, only approved users can post
|   |-- Private: only members can view
|
|-- Caching
    |-- Redis cache for community metadata (high read, low write)
    |-- Cache invalidation on community update
```

**Key implementation files:**
- `internal/community/server.go` -- gRPC handlers
- `internal/community/cache.go` -- Redis caching layer

---

### 4. Post Service

**Port:** 50055 | **Database:** PostgreSQL (sharded: `posts_shard_0`, `posts_shard_1`) | **Proto:** `redyx.post.v1.PostService`

Handles post CRUD, feed generation, and ranking. The database is **sharded by `community_id` using consistent hashing** -- the most architecturally significant service.

```
Post Service
|-- Post CRUD
|   |-- Create text posts (title + markdown body)
|   |-- Create link posts (title + URL)
|   |-- Create media posts (title + media upload via Media Service)
|   |-- Edit and delete own posts
|   |-- Anonymous posting ([anonymous] username, visible only to moderators)
|
|-- Feed Generation
|   |-- Community feed: all posts in a specific community (single-shard query)
|   |-- Home feed: aggregated from all subscribed communities (cross-shard)
|   |-- Saved posts feed
|   |-- Cursor-based pagination
|
|-- Ranking Algorithms
|   |-- Hot: Lemmy algorithm -- 10000 * log(max(1, 3+score)) / (hoursAge+2)^1.8
|   |-- New: reverse chronological
|   |-- Top: highest net score (filterable by time range: day, week, month, year, all)
|   |-- Rising: score / max(1, hoursAge) -- velocity-based ranking
|
|-- Sharding (Consistent Hashing)
|   |-- Shard key: community_id
|   |-- Hash ring with 40 virtual nodes per shard (serialx/hashring)
|   |-- community_id -> hash(community_id) -> shard ring -> target shard
|   |-- Each shard has its own pgxpool.Pool connection pool
|   |-- Community feed queries hit a single shard (no cross-shard joins)
|   |-- Home feed queries fan out to all shards in parallel
|
|-- Spam and Moderation Integration
|   |-- Pre-publish spam check via Spam Service (synchronous gRPC)
|   |-- Pre-publish moderation check (banned users, community rules)
|   |-- Moderator actions: pin/unpin, remove, approve
|
|-- Event Publishing (Kafka)
|   |-- PostCreated -> consumed by Search, Notification, Spam services
|   |-- PostUpdated -> consumed by Search service
|   |-- PostDeleted -> consumed by Search service
|
|-- Vote Score Updates
|   |-- Consumes VoteCreated events from Kafka
|   |-- Updates post score and recalculates hot/rising rankings
|
|-- Caching
    |-- Redis cache for hot post feeds (5 min TTL)
    |-- Cache invalidation on post create/update/delete
```

**Key implementation files:**
- `internal/post/server.go` -- gRPC handlers (46 KB, largest service)
- `internal/post/shard.go` -- `ShardRouter` with consistent hashing via `serialx/hashring`
- `internal/post/ranking.go` -- `HotScore` and `RisingScore` algorithms
- `internal/post/cache.go` -- Redis caching layer
- `internal/post/producer.go` -- Kafka event publisher
- `internal/post/vote_consumer.go` -- Kafka consumer for vote score updates
- `internal/post/moderator.go` -- Moderation integration (ban checks, pin, remove)

---

### 5. Comment Service

**Port:** 50057 | **Database:** ScyllaDB (`redyx_comments` keyspace) | **Proto:** `redyx.comment.v1.CommentService`

Nested, threaded comments stored in ScyllaDB. Uses materialized paths for tree ordering and Wilson score for "Best" sorting.

```
Comment Service
|-- Comment CRUD
|   |-- Create top-level comments on posts
|   |-- Reply to existing comments (nested threads)
|   |-- Edit and delete own comments
|   |-- Deleted comments display [deleted] but preserve tree structure
|   |-- Author username denormalized into comment row (no cross-service join on read)
|
|-- Thread Structure (Materialized Path)
|   |-- Partition key: post_id (all comments for a post in one partition)
|   |-- Clustering key: path (e.g., "001", "001.002", "001.002.003")
|   |-- NextPath() generates sequential child segments via ScyllaDB counters
|   |-- Depth calculated from path segment count
|   |-- Lazy-load deep threads (top 2-3 levels, then load more on demand)
|
|-- Sorting Algorithms
|   |-- Best: Wilson score confidence interval (z=1.96, 95% CI)
|   |-- Top: highest net score
|   |-- New: reverse chronological
|   |-- Controversial: high total votes with near-equal up/down ratio
|
|-- Spam and Moderation Integration
|   |-- Pre-publish spam check via Spam Service (synchronous gRPC)
|   |-- Pre-publish moderation check (banned users)
|
|-- Event Publishing (Kafka)
|   |-- CommentCreated -> consumed by Notification service (reply alerts)
|
|-- Vote Score Updates
    |-- Consumes VoteCreated events from Kafka
    |-- Updates comment score in ScyllaDB
```

**Key implementation files:**
- `internal/comment/server.go` -- gRPC handlers
- `internal/comment/scylla.go` -- ScyllaDB storage layer (19 KB)
- `internal/comment/path.go` -- Materialized path generation (`NextPath`, `ParentPath`, `Depth`)
- `internal/comment/wilson.go` -- Wilson score lower bound for "Best" sorting
- `internal/comment/kafka.go` -- Kafka vote event consumer
- `internal/comment/producer.go` -- Kafka event publisher
- `internal/comment/moderator.go` -- Moderation integration

---

### 6. Vote Service

**Port:** 50056 | **Database:** Redis (primary) + Kafka (event log) | **Proto:** `redyx.vote.v1.VoteService`

Highest throughput service. Manages upvotes/downvotes with atomic Redis Lua scripts and publishes events consumed by four other services.

```
Vote Service
|-- Voting
|   |-- Upvote / downvote posts and comments
|   |-- One vote per user per item (enforced atomically in Redis)
|   |-- Change vote direction or remove vote
|   |-- Idempotent: duplicate requests safely return current state
|
|-- Atomic Redis Operations (Lua Scripts)
|   |-- 9-state vote transition matrix handled in a single Lua script
|   |-- State keys: votes:state:{user_id}:{target_id}
|   |-- Set membership: votes:up:{target_id}, votes:down:{target_id}
|   |-- Atomic score update: votes:score:{target_id}
|   |-- Returns delta, new_score, old_direction
|
|-- Batch Operations
|   |-- BatchGetVoteStates: Redis pipelining for bulk vote state lookups
|   |-- Used by feed endpoints to show vote indicators
|
|-- Event Publishing (Kafka)
|   |-- VoteCreated events consumed by:
|       |-- Post Service (update post score)
|       |-- Comment Service (update comment score)
|       |-- User Service (update karma)
|       |-- Spam Service (vote manipulation detection)
|
|-- Score Queries
    |-- GetScore: real-time net score for any target
    |-- GetVoteState: current user's vote direction on a target
```

**Key implementation files:**
- `internal/vote/server.go` -- gRPC handlers
- `internal/vote/redis.go` -- `VoteStore` with atomic Lua scripts, batch pipelining
- `internal/vote/kafka.go` -- Kafka producer
- `internal/vote/consumer.go` -- Kafka consumer

---

### 7. Search Service

**Port:** 50058 | **Database:** Meilisearch | **Proto:** `redyx.search.v1.SearchService`

Full-text search powered by Meilisearch. Consumes Kafka events to keep the search index in sync.

```
Search Service
|-- Search Queries
|   |-- Full-text search across posts (title + body)
|   |-- Filter by community or search globally
|   |-- Community name autocomplete
|   |-- Ranked by relevance + recency + vote score
|
|-- Search Indexing (Kafka Consumer)
|   |-- Consumes PostCreated events -> indexes post in Meilisearch
|   |-- Consumes PostUpdated events -> updates index entry
|   |-- Consumes PostDeleted events -> removes from index
|   |-- Consumer group: search-service.redyx.posts.v1
|
|-- Meilisearch Integration
    |-- Index configuration: searchable attributes, ranking rules
    |-- Filterable attributes: community_name, author
    |-- Sortable attributes: vote_score, created_at
    |-- Typo tolerance enabled
```

**Key implementation files:**
- `internal/search/server.go` -- gRPC handlers
- `internal/search/indexer.go` -- Kafka consumer that routes events to Meilisearch
- `internal/search/meili.go` -- Meilisearch client wrapper (index/delete/search)

---

### 8. Media Service

**Port:** 50060 | **Database:** PostgreSQL (`media`) + MinIO (S3) | **Proto:** `redyx.media.v1.MediaService`

Handles file uploads, validation, thumbnail generation, and S3 storage.

```
Media Service
|-- Upload
|   |-- Image upload (JPEG, PNG, GIF, WebP)
|   |-- Video upload
|   |-- File type and size validation before storage
|   |-- Returns S3 URL stored as reference in Post Service
|
|-- Thumbnail Generation
|   |-- Resizes images to max 320px wide (Lanczos resampling)
|   |-- Maintains aspect ratio
|   |-- Encodes as JPEG (quality 85)
|   |-- Uploads thumbnail alongside original to S3
|   |-- Non-fatal: upload proceeds even if thumbnail generation fails
|
|-- Storage
|   |-- MinIO for local development (S3-compatible)
|   |-- AWS S3 for production with CloudFront CDN
|   |-- Bucket: redyx-media
|
|-- Metadata
    |-- Upload records stored in PostgreSQL (file metadata, status, URLs)
    |-- Status tracking: PENDING -> PROCESSING -> READY / FAILED
```

**Key implementation files:**
- `internal/media/server.go` -- gRPC handlers
- `internal/media/s3.go` -- S3/MinIO client wrapper
- `internal/media/thumbnail.go` -- Image resize and upload (uses `disintegration/imaging`)
- `internal/media/store.go` -- PostgreSQL metadata storage

---

### 9. Notification Service

**Port:** 50059 (gRPC) + 8081 (WebSocket) | **Database:** PostgreSQL (`notifications`) + Redis | **Proto:** `redyx.notification.v1.NotificationService`

Real-time notification delivery via WebSocket with offline persistence.

```
Notification Service
|-- Real-Time Delivery (WebSocket)
|   |-- Persistent WebSocket connection per authenticated user
|   |-- JWT authentication via query parameter (?token=...)
|   |-- Hub manages user_id -> active WebSocket connections map
|   |-- Pushes notifications as JSON over WebSocket
|   |-- Keep-alive read loop for connection health
|
|-- Offline Delivery
|   |-- Notifications stored in PostgreSQL when user is offline
|   |-- Delivered on next WebSocket connect (last 24h undelivered)
|
|-- Event Consumption (Kafka)
|   |-- Reply notifications (comment on your post/comment)
|   |-- Mention notifications (u/username in content)
|   |-- Moderation actions (post removed, approved, flaired)
|   |-- Community follow notifications (optional, configurable)
|
|-- Notification Management
|   |-- Read/unread tracking (unread count cached in Redis)
|   |-- Mark as read (individual or bulk)
|   |-- Notification preferences per user (mute communities, mute replies)
|
|-- Mention Parsing
    |-- Extracts u/username patterns from post and comment content
    |-- Generates mention notifications for matching users
```

**Key implementation files:**
- `internal/notification/server.go` -- gRPC handlers
- `internal/notification/websocket.go` -- `Hub` with register/unregister/send and WebSocket upgrade handler
- `internal/notification/consumer.go` -- Kafka consumer for notification events
- `internal/notification/store.go` -- PostgreSQL persistence
- `internal/notification/mention.go` -- u/username mention extraction
- `internal/notification/events.go` -- Event type definitions
- `internal/notification/post_resolver.go` -- Cross-service post title resolution

---

### 10. Moderation Service

**Port:** 50061 | **Database:** PostgreSQL (`moderation`) | **Proto:** `redyx.moderation.v1.ModerationService`

Community-level content moderation, user bans, and transparency tools.

```
Moderation Service
|-- Content Moderation
|   |-- Remove posts and comments from a community
|   |-- Approve posts (for communities requiring approval)
|   |-- Reports queue: list of user-reported/flagged content for review
|
|-- User Management
|   |-- Ban users from specific communities (with duration and reason)
|   |-- Mute users (prevent posting temporarily)
|   |-- Shadow-ban (content visible only to the banned user)
|
|-- Community Tools
|   |-- Pin/sticky up to 2 posts per community
|   |-- Assign flair to posts
|
|-- Transparency
|   |-- Mod log: all moderation actions recorded with actor, action, target, timestamp
|   |-- Queryable mod action history per community
|
|-- Event Publishing (Kafka)
|   |-- PostRemoved -> consumed by Post Service, Notification Service, Search Service
|
|-- Cross-Service Integration
    |-- Calls Community Service for role verification
    |-- Calls Post Service for post operations
    |-- Calls Comment Service for comment operations
```

**Key implementation files:**
- `internal/moderation/server.go` -- gRPC handlers (23 KB)
- `internal/moderation/store.go` -- PostgreSQL storage for mod logs, bans, reports

---

### 11. Spam and Abuse Detection Service

**Port:** 50062 | **Database:** Redis (real-time scoring) | **Proto:** `redyx.spam.v1.SpamService`

Multi-layer spam prevention with both synchronous pre-publish checks and asynchronous behavior analysis.

```
Spam Service
|-- Pre-Publish Checks (Synchronous, called before content is saved)
|   |-- Keyword blocklist filtering (case-insensitive substring matching)
|   |-- URL reputation check (blocked domain list, O(1) lookup)
|   |-- Duplicate content detection (SHA-256 hash + Redis SET NX, 24h TTL)
|   |-- URL extraction from markdown links and bare URLs
|   |-- New account restrictions:
|       |-- Accounts < 24h old cannot create posts
|       |-- Accounts < 1h old cannot comment
|
|-- Post-Publish Analysis (Asynchronous, via Kafka consumer)
|   |-- Rapid posting pattern detection
|   |-- Same link spammed across multiple communities
|   |-- Vote manipulation detection (coordinated upvote ring analysis)
|   |-- Account behavior scoring (trust score based on age, karma, report history)
|
|-- Actions
|   |-- Auto-remove content failing pre-publish checks
|   |-- Flag suspicious content for moderator review
|   |-- Shadow-ban: posts visible only to the author
|   |-- IP-level temporary ban for severe abuse
|
|-- Blocklist Management
|   |-- JSON-based keyword and domain blocklists
|   |-- Loaded at startup from data files
|   |-- Keywords normalized to lowercase
|   |-- Domains stored in hash map for O(1) lookup
|
|-- Content Deduplication
    |-- Normalize content: lowercase, trim, collapse whitespace
    |-- SHA-256 hash of normalized content
    |-- Redis key: dedup:{userID}:{hash} with 24h TTL
    |-- SET NX (set if not exists) for atomic duplicate detection
```

**Key implementation files:**
- `internal/spam/server.go` -- gRPC handlers (synchronous checks)
- `internal/spam/blocklist.go` -- Keyword/domain blocklist with URL extraction
- `internal/spam/dedup.go` -- SHA-256 content deduplication via Redis SET NX
- `internal/spam/consumer.go` -- Kafka consumer for async behavior analysis
- `internal/spam/data/` -- Blocklist seed data (keywords, domains)

---

### 12. Rate Limiting

**Database:** Redis | **Implementation:** `internal/platform/ratelimit/`

Redis-backed token bucket rate limiter with tiered limits and per-action quotas. Enforced at the gRPC interceptor level.

```
Rate Limiting
|-- Token Bucket Algorithm (Atomic Lua Script)
|   |-- Single Lua script for atomic check-and-decrement
|   |-- Redis key with TTL-based window expiry (no manual cleanup)
|   |-- Returns: allowed (bool), remaining, retry-after duration
|
|-- User Tiers
|   |-- Anonymous: 10 requests/minute
|   |-- Authenticated: 100 requests/minute
|
|-- Per-Action Limits
|   |-- Post creation: 5 per hour
|   |-- Comment creation: 30 per hour
|   |-- Vote: 60 per minute
|   |-- Community creation: 1 per day
|
|-- Response
    |-- HTTP 429 Too Many Requests with Retry-After header
    |-- gRPC ResourceExhausted status code
```

**Key implementation files:**
- `internal/platform/ratelimit/limiter.go` -- `Limiter` with atomic Lua token bucket script
- `internal/platform/ratelimit/interceptor.go` -- gRPC unary interceptor

---

## Database Architecture

The system uses **9 data stores** across 5 technologies:

### PostgreSQL (5 instances)

| Instance | Owner | Sharded | Data |
|---|---|---|---|
| `pg-auth` | Auth Service | No | Hashed passwords, OAuth tokens, OTP records, encrypted emails |
| `pg-user` | User Service | No | Profiles, karma, settings, avatars |
| `pg-community` | Community Service | No | Communities, memberships, roles, rules |
| `pg-post` | Post Service | Yes | Posts, sharded by `community_id` via consistent hashing |
| `pg-platform` | Moderation, Notification, Media | No | Mod logs, ban records, notification history, media metadata |

### ScyllaDB (1 cluster)

| Cluster | Owner | Data |
|---|---|---|
| `scylla-comments` | Comment Service | All comments, partitioned by `post_id`, clustered by materialized path |

### Redis (1 instance, 12 logical databases)

Each service uses a dedicated Redis logical database (db0 through db11) for isolation.

### Meilisearch (1 instance)

Full-text search index for posts, communities, and autocomplete.

### MinIO / S3 (1 bucket)

Object storage for user-uploaded images, videos, thumbnails, and community banners.

---

## Post Sharding with Consistent Hashing

The Post Service implements **application-level database sharding** using consistent hashing. This is the core scalability mechanism.

**How it works:**

1. Each `community_id` is hashed onto a ring using `serialx/hashring`
2. The ring has 40 virtual nodes per physical shard for even distribution
3. All posts for a community land on the same shard
4. Community feed queries hit a single shard (no cross-shard joins)
5. Home feed queries fan out to all shards in parallel

**Adding a new shard:**

1. Add the new node to the hash ring
2. Consistent hashing ensures only ~1/N of data migrates
3. Identify which `community_id` values now map to the new shard
4. Background migration copies affected posts
5. Flip routing, then clean up old copies

**Current deployment:** 2 shards (`posts_shard_0`, `posts_shard_1`), each in its own PostgreSQL database.

---

## Communication Patterns

### Synchronous (gRPC -- Request/Response)

Used when the caller needs an immediate answer:

- User requests a page -> Envoy -> Post Service -> returns posts
- User submits a comment -> Envoy -> Comment Service -> returns created comment
- Pre-publish spam check -> Post Service -> Spam Service -> returns allow/reject

### Asynchronous (Kafka -- Event-Driven)

Used for fan-out and eventual consistency:

```
VoteCreated event:
  -> Post Service: update post score
  -> Comment Service: update comment score
  -> User Service: update karma
  -> Spam Service: vote manipulation detection

PostCreated event:
  -> Search Service: index the post in Meilisearch
  -> Notification Service: notify community followers
  -> Spam Service: analyze for spam patterns

PostRemoved event:
  -> Post Service: mark post as removed
  -> Notification Service: notify the author
  -> Search Service: remove from index
```

### Real-Time (WebSocket)

- User authenticates and opens a persistent WebSocket connection
- Notification Service pushes events: replies, mentions, mod actions
- Offline users receive stored notifications on reconnect (last 24 hours)
- JWT authentication via query parameter (WebSocket limitation)

---

## Caching Strategy

| Cache Target | TTL | Invalidation |
|---|---|---|
| Community metadata | 1 hour | On community update |
| Hot post feed | 5 min | TTL-based, regenerate on expiry |
| Post vote count | Real-time | Write-through from Vote Service |
| User session/token | 15 min | On logout |
| Rate limit counters | Per-window | Auto-expire with TTL |
| User karma | 10 min | On Kafka vote event |
| Search autocomplete | 30 min | TTL-based |
| OTP codes | 5 min | Auto-expire |

---

## Security and Privacy

- **Anonymity by design:** Users identified only by username; no real name, phone, or location
- **Argon2id:** Password hashing with RFC 9106 parameters (64 MiB memory, GPU-resistant)
- **JWT:** Short-lived access tokens (15 min) with long-lived refresh tokens (7 days)
- **No IP storage:** IPs are never stored in application databases; hashed with SHA-256 + salt for abuse detection (24h TTL in Redis), raw IPs in request logs rotated after 7 days
- **TLS everywhere:** Envoy terminates external TLS; internal mTLS via Istio (optional)
- **CORS:** Strict origin allowlist at the API gateway
- **Parameterized queries:** No string concatenation in SQL (injection prevention)
- **Account deletion:** True PII purge; posts become `[deleted]`, vote records anonymized
- **Constant-time comparison:** Password verification uses `subtle.ConstantTimeCompare`

---

## Observability

The full observability stack is deployed in the `redyx-monitoring` Kubernetes namespace:

- **Prometheus:** Scrapes `/metrics` from each Go service via `go-grpc-prometheus` interceptors. Collects request latency histograms, error rates, and throughput per service.
- **Grafana:** Per-service dashboards and global overview. Configured with Prometheus, Loki, and Jaeger data sources.
- **Loki + Promtail:** Centralized structured JSON log aggregation from all services, queryable through Grafana.
- **Jaeger:** Distributed tracing via OpenTelemetry SDK (`otlptracegrpc` exporter). Health check spans are filtered out to reduce noise. Trace context propagated across gRPC calls.
- **Alerting:** Grafana alerts on error rate spikes, P99 latency thresholds, pod restarts, and Kafka consumer lag.

---

## Deployment

### Docker Compose (Local Development)

Start the full stack with all 12 services, databases, and Envoy gateway:

```bash
make docker-up          # Start all services
make docker-logs        # Tail logs
make docker-down        # Stop everything
```

### Kubernetes (kind)

Full Kubernetes deployment with Helm charts, NGINX Ingress, and observability stack:

```bash
make k8s-up             # Full deployment: cluster + ingress + storage + data + monitoring + app
make k8s-status         # Show cluster status (pods, ingresses, HPAs)
make k8s-logs           # Tail app service logs
make k8s-down           # Tear down everything
```

**K8s namespaces:**
- `redyx-app` -- All 12 microservices + Envoy gateway (deployed via Helm chart)
- `redyx-data` -- PostgreSQL, Redis, ScyllaDB, Kafka, Meilisearch, MinIO
- `redyx-monitoring` -- Prometheus, Grafana, Loki, Jaeger

**K8s features:**
- Helm chart: `deploy/k8s/charts/redyx-services/`
- HPA (Horizontal Pod Autoscaler) per service
- Readiness and liveness probes (gRPC health checks)
- ConfigMaps for configuration, Secrets for credentials
- NGINX Ingress Controller for external routing
- Local PersistentVolumes backed by `~/.redyx-data`

**Access URLs (local):**
```
API Gateway:  http://localhost:8080/api/v1/
Grafana:      http://localhost:8080/grafana
Prometheus:   http://localhost:8080/prometheus
Jaeger:       http://localhost:8080/jaeger
```

---

## Project Structure

```
redyx/
|-- cmd/                            # Service entry points (main.go per service)
|   |-- auth/
|   |-- comment/
|   |-- community/
|   |-- media/
|   |-- moderation/
|   |-- notification/
|   |-- post/
|   |-- search/
|   |-- skeleton/
|   |-- spam/
|   |-- user/
|   |-- vote/
|
|-- internal/                       # Service implementations
|   |-- auth/                       # Auth: hasher, JWT, OAuth, OTP, email, server
|   |-- comment/                    # Comment: ScyllaDB, materialized path, Wilson score
|   |-- community/                  # Community: CRUD, cache, server
|   |-- media/                      # Media: S3, thumbnails, store, server
|   |-- moderation/                 # Moderation: server, store, tests
|   |-- notification/               # Notification: WebSocket hub, Kafka consumer, store
|   |-- post/                       # Post: shard router, ranking, cache, Kafka
|   |-- search/                     # Search: Meilisearch client, Kafka indexer
|   |-- skeleton/                   # Skeleton: health check baseline service
|   |-- spam/                       # Spam: blocklist, dedup, Kafka consumer, tests
|   |-- user/                       # User: profiles, karma, server
|   |-- vote/                       # Vote: Redis Lua scripts, Kafka producer/consumer
|   |-- platform/                   # Shared platform libraries
|       |-- auth/                   # JWT validation
|       |-- config/                 # Environment-based configuration
|       |-- database/               # PostgreSQL connection helpers
|       |-- errors/                 # gRPC error mapping
|       |-- grpcserver/             # gRPC server bootstrap
|       |-- middleware/             # Logging, recovery, error interceptors
|       |-- observability/          # Prometheus metrics + OpenTelemetry tracing
|       |-- pagination/             # Cursor-based pagination
|       |-- ratelimit/              # Redis token bucket rate limiter
|       |-- redis/                  # Redis connection helpers
|
|-- proto/redyx/                    # Protocol Buffer definitions (15 .proto files)
|   |-- auth/v1/auth.proto
|   |-- comment/v1/comment.proto
|   |-- common/v1/common.proto
|   |-- common/v1/events.proto
|   |-- community/v1/community.proto
|   |-- events/v1/events.proto
|   |-- health/v1/health.proto
|   |-- media/v1/media.proto
|   |-- moderation/v1/moderation.proto
|   |-- notification/v1/notification.proto
|   |-- post/v1/post.proto
|   |-- search/v1/search.proto
|   |-- spam/v1/spam.proto
|   |-- user/v1/user.proto
|   |-- vote/v1/vote.proto
|
|-- gen/                            # Generated Go code from protobuf
|-- migrations/                     # SQL migrations per database
|   |-- auth/
|   |-- comment/
|   |-- community/
|   |-- media/
|   |-- moderation/
|   |-- notification/
|   |-- post_shard_0/
|   |-- post_shard_1/
|   |-- skeleton/
|   |-- user/
|
|-- deploy/
|   |-- docker/
|   |   |-- Dockerfile              # Multi-stage Go build (golang:1.26-alpine -> alpine:3.21)
|   |   |-- init-databases.sql      # PostgreSQL initialization (creates all databases)
|   |-- envoy/
|   |   |-- envoy.yaml              # Envoy config: REST->gRPC transcoding, routing, CORS
|   |   |-- envoy-k8s.yaml          # Envoy config for Kubernetes deployment
|   |   |-- proto.pb                # Compiled protobuf descriptor for Envoy transcoder
|   |-- k8s/
|       |-- kind-config.yaml        # kind cluster configuration
|       |-- storage/                # Local StorageClass and PersistentVolumes
|       |-- data/                   # StatefulSets: PostgreSQL, Redis, ScyllaDB, Kafka, Meilisearch, MinIO
|       |-- monitoring/             # Prometheus, Grafana, Loki, Jaeger configs
|       |-- ingress/                # NGINX Ingress Controller values
|       |-- charts/redyx-services/  # Helm chart for all 12 microservices
|
|-- web/                            # Astro + Svelte frontend
|   |-- src/
|   |   |-- pages/                  # Astro pages (SSR)
|   |   |-- components/             # Svelte interactive islands
|   |   |-- lib/                    # API client, utilities
|   |   |-- layouts/                # Page layouts
|   |   |-- styles/                 # CSS
|   |-- prisma/                     # Prisma schemas for DB introspection (dev tooling)
|
|-- diagrams/                       # UML diagrams
|   |-- activity-diagrams/
|   |-- class-diagrams/
|   |-- data-flow-diagrams/
|   |-- sequence-diagrams/
|   |-- use-case-diagram/
|
|-- docs/                           # Documentation
|   |-- Architecture Plan.md
|   |-- Core Concepts.md
|   |-- Software Requirement Specification Document.md
|
|-- scripts/                        # Validation and testing scripts
|-- docker-compose.yml              # Full local development stack
|-- Makefile                        # Build, test, deploy targets
|-- buf.yaml / buf.gen.yaml         # Protobuf tooling configuration
|-- go.mod / go.sum                 # Go module dependencies
```

---

## Getting Started

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- [Buf CLI](https://buf.build/docs/installation) (for protobuf generation)
- Node.js / Bun (for the frontend)
- kind + kubectl + Helm (for Kubernetes deployment)

### Local Development (Docker Compose)

```bash
# Generate protobuf code and Envoy descriptor
make proto

# Start all services
make docker-up

# The API is available at http://localhost:8080/api/v1/

# Start the frontend dev server
make web

# View logs
make docker-logs

# Stop
make docker-down
```

### Kubernetes Deployment

```bash
# Full stack: creates cluster, deploys everything
make k8s-up

# Check status
make k8s-status

# Validate deployment
make k8s-validate

# Tear down
make k8s-down
```

### Building

```bash
# Build all services
make build

# Run tests
make test

# Generate protobuf code
make proto

# Lint proto files
make proto-lint
```

---

## License

This project is licensed under the terms specified in the [LICENSE](LICENSE) file.
