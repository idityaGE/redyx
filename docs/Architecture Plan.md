---
title: Reddit Clone - Architecture Plan
tags: [type/concept, proj/reddit-clone, status/active, stack/go, stack/k8s, stack/kafka, stack/redis]
summary: System architecture, tech stack, service boundaries, database strategy, and infrastructure for the Reddit clone
---

## 1. High-Level Architecture

```
                        +------------------+
                        |   Cloudflare CDN |
                        +--------+---------+
                                 |
                        +--------v---------+
                        |   Astro Frontend  |  (SSR + Svelte Islands)
                        |   (Content-driven)|
                        +--------+---------+
                                 | REST/JSON
                        +--------v---------+
                        |   Envoy Gateway   |
                        |   - TLS termination
                        |   - Rate limiting (Redis-backed)
                        |   - REST -> gRPC translation
                        |   - JWT validation
                        +--------+---------+
                                 | gRPC (internal)
              +------------------+------------------+
              |         |        |        |         |
         +----v---+ +---v----+ +v------+ +v------+ +v--------+
         | Auth   | | User   | | Post  | | Vote  | | Comment |
         | Service| | Service| | Service| | Service| | Service |
         +----+---+ +---+----+ +---+---+ +---+---+ +----+----+
              |         |         |         |            |
         +----v---+ +---v----+ +--v----+ +-v------+ +---v-----+
         |Postgres| |Postgres| |Postgres| | Redis  | | ScyllaDB|
         | (auth) | | (user) | |(sharded)| | +Kafka | |         |
         +--------+ +--------+ +-------+ | +PG    | +---------+
                                          +--------+
              +------------------+------------------+
              |         |        |        |         |
         +----v-----+ +v------+ +v--------+ +------v------+
         | Community| | Search| | Media    | | Notification|
         | Service  | | Service| | Service | | Service     |
         +----+-----+ +---+---+ +----+----+ +------+------+
              |            |          |              |
         +----v---+  +----v-----+ +--v-----+  +----v----+
         |Postgres|  |Meilisearch| | AWS S3 |  | Redis   |
         |(commty)|  +----------+ +--------+  | +Kafka  |
         +--------+                            | +PG     |
                                               +---------+
         +-------------------+  +---------------------+
         | Rate Limit        |  | Spam/Abuse Detection |
         | (Envoy + Redis)   |  | Service              |
         +-------------------+  | Redis + PG           |
                                +---------------------+

         +-------------------+  +---------------------+
         | Moderation Service|  | WebSocket Gateway    |
         | PostgreSQL        |  | (Notifications)      |
         +-------------------+  +---------------------+
```

## 2. Tech Stack

| Layer              | Technology                  | Rationale                                                    |
| ------------------ | --------------------------- | ------------------------------------------------------------ |
| Frontend           | Astro (SSR + Islands)       | Content-driven, fast page loads, minimal JS shipped          |
| Islands Framework  | Svelte                      | Interactive components (vote buttons, comment forms, search) |
| API Gateway        | Envoy                       | REST-to-gRPC translation, rate limiting, TLS, load balancing |
| Backend Services   | Go (Golang)                 | High performance, strong concurrency, great gRPC support     |
| Service-to-Service | gRPC (Protocol Buffers)     | Type-safe, fast binary protocol, streaming support           |
| Frontend-to-Gateway| REST/JSON                   | Astro SSR uses fetch(), browser-friendly, easy debugging     |
| Real-Time          | WebSocket                   | Notification delivery to connected clients                   |
| Auth               | Email + Password + Google OAuth + OTP | Verified accounts, social login, anonymity preserved |
| Message Queue      | Apache Kafka                | Vote processing, feed updates, event sourcing, notifications |
| Cache              | Redis                       | Session store, vote counts, hot posts, rate limiting         |
| Primary Database   | PostgreSQL                  | ACID, relational integrity, mature ecosystem                 |
| Sharded Database   | PostgreSQL (consistent hashing) | App-level sharding on the post-service                  |
| Comment Store      | ScyllaDB                    | High-throughput wide-column store for nested comment trees    |
| Search Engine      | Meilisearch                 | Full-text search on posts, comments, communities             |
| Object Storage     | AWS S3                      | Image and video storage, CDN integration, durability         |
| Orchestration      | Kubernetes (k8s)            | Service deployment, auto-scaling, health checks              |
| Service Mesh       | Istio (optional)            | mTLS between services, traffic management                    |
| CI/CD              | GitHub Actions + ArgoCD     | Build, test, deploy to k8s via GitOps                        |
| Monitoring         | Prometheus + Grafana        | Metrics collection and dashboards                            |
| Logging            | Loki                        | Centralized log aggregation with Grafana integration         |
| Tracing            | OpenTelemetry + Jaeger      | Distributed request tracing across microservices             |
| Load Testing       | k6 or Locust                | Validate sharding, caching, and Kafka under load             |

## 3. Why API Gateway Translation over gRPC-Web

gRPC is used **internally** between Go microservices for speed and type safety. But the frontend communicates via **REST/JSON** through the API Gateway. Reasons:

- Astro SSR uses standard `fetch()` for server-side rendering — REST/JSON is native to this
- Browser devtools can inspect JSON responses (gRPC-Web is binary, hard to debug)
- No need to bundle a protobuf runtime in the browser
- The entire web tooling ecosystem (Postman, curl, fetch) speaks REST/JSON
- gRPC-Web does not support client-side streaming in browsers
- JSON serialization overhead at the gateway is negligible — the bottleneck is DB and Kafka, not edge serialization

Envoy handles the translation: incoming REST/JSON requests are converted to gRPC calls to the appropriate backend service using Envoy's gRPC-JSON transcoder filter.

## 4. Microservice Boundaries

### 4.1 Auth Service

- **Responsibility**: Registration, login, logout, OAuth, OTP verification, token management
- **Database**: PostgreSQL (isolated, stores only credentials and OAuth tokens)
- **Key details**:
  - **Registration methods**:
    - Email + username + password (email verified via OTP before account is active)
    - Google OAuth (fetches email from Google, user still picks a username)
  - OTP delivery via email (6-digit code, 5 min TTL, stored in Redis)
  - Issues JWTs (short-lived access token 15 min + long-lived refresh token 7 days)
  - Password hashing with **argon2id** (memory-hard, GPU-resistant)
  - **Other users only see the username** — email and auth method are never exposed publicly
  - Tokens are validated at the Envoy API Gateway level (no round-trip to auth-service per request)
  - Google OAuth flow: Astro redirects to Google -> Google returns auth code -> Auth Service exchanges for tokens -> creates/links account

### 4.2 User Service

- **Responsibility**: User profiles, karma calculation, account settings, avatar
- **Database**: PostgreSQL
- **Key details**:
  - Separated from auth-service — profile data is not credential data
  - Karma is eventually consistent (aggregated from vote events via Kafka)
  - Public profile shows: username, karma, cake day, post/comment history

### 4.3 Community Service

- **Responsibility**: CRUD communities, membership (join/leave), roles (member/mod/owner), community rules and settings
- **Database**: PostgreSQL
- **Key details**:
  - Community names are unique and immutable
  - Membership is a many-to-many relationship (user <-> community)
  - Community metadata is heavily cached in Redis (high read, low write)
  - Visibility levels: public, restricted (anyone can view, only approved can post), private

### 4.4 Post Service

- **Responsibility**: CRUD posts, feed generation, post ranking
- **Database**: PostgreSQL (sharded — see Section 7)
- **Key details**:
  - Posts belong to exactly one community
  - Supports text, link, and media post types
  - Feed generation queries across subscribed communities
  - Hot/Top/New/Rising sorting algorithms
  - Publishes events to Kafka on create/update/delete (consumed by search, notification, feed services)

### 4.5 Comment Service

- **Responsibility**: CRUD comments, nested threading, comment tree retrieval
- **Database**: ScyllaDB
- **Key details**:
  - ScyllaDB chosen for high write throughput and wide-column model suited for comment trees
  - Partition key: `post_id` (all comments for a post in one partition)
  - Clustering key: `path` (materialized path like `root/parent/child` for tree ordering)
  - Comments form a tree structure (parent_id references another comment or null for top-level)
  - Deleted comments become `[deleted]` but preserve tree structure
  - Lazy-load deep threads (fetch top 2-3 levels, load more on demand)
  - Denormalize author username into comment row (avoid cross-service join on read)

### 4.6 Vote Service

- **Responsibility**: Upvote/downvote on posts and comments, score aggregation
- **Database**: Redis (real-time counts) + Kafka (event log) + PostgreSQL (persistence)
- **Key details**:
  - Highest throughput service — must handle thousands of votes per second
  - Redis stores current vote state per user per item (ensures one vote per user)
  - Vote events published to Kafka, consumed by:
    - Post/Comment service (update score)
    - User service (update karma)
  - Idempotent design: duplicate vote requests are safely ignored
  - Vote record: `{user_id, target_id, target_type (post|comment), direction (up|down|none)}`

### 4.7 Search Service

- **Responsibility**: Full-text search across posts, comments, and communities
- **Database**: Meilisearch
- **Key details**:
  - Consumes Kafka events from post/comment/community services to keep index updated
  - Supports search within a specific community or globally
  - Autocomplete for community names
  - Ranked by relevance + recency + score

### 4.8 Media Service

- **Responsibility**: Image/video upload, processing, serving
- **Storage**: AWS S3
- **Key details**:
  - Handles file upload, validation (type, size limits), and storage to S3
  - Generates thumbnails for images (processed in-service before upload)
  - Returns an S3 URL that the post-service stores as a reference
  - S3 bucket policy: private by default, served via CloudFront CDN with signed URLs
  - Virus/malware scanning on upload (ClamAV)
  - S3 lifecycle rules: move infrequently accessed media to S3 Glacier after 90 days
  - CDN (CloudFront) caching for frequently accessed media

### 4.9 Notification Service

- **Responsibility**: Real-time and async notifications for user events
- **Database**: Redis (connection registry + unread counts) + Kafka (event ingestion) + PostgreSQL (notification history)
- **Key details**:
  - Consumes events from Kafka:
    - Someone replied to your post or comment
    - Someone mentioned your username (`u/username`)
    - Moderation action on your post (removed, approved, flaired)
    - New post in a community you follow (optional, configurable)
  - **Real-time delivery via WebSocket**:
    - Each authenticated user opens a persistent WebSocket connection on login
    - Notification Service maintains a Redis-backed registry of `user_id -> pod_id` mappings
    - When an event arrives from Kafka, the service looks up which pod holds the user's WebSocket and pushes the notification
    - If the user is offline, the notification is stored in PostgreSQL and delivered on next connection
    - WebSocket heartbeat (ping/pong every 30s) to detect stale connections
  - Email notifications for critical events (account security, mod actions) if user has email on file
  - Notification preferences per user (mute specific communities, mute replies, etc.)
  - Read/unread tracking (unread count cached in Redis)
  - Batch delivery to avoid spamming (aggregate multiple replies into one notification)

### 4.10 Rate Limiting Service

- **Responsibility**: Protect all services from abuse, bots, and traffic spikes
- **Database**: Redis
- **Key details**:
  - Runs at the **API Gateway level** (before requests hit backend services)
  - Algorithms:
    - **Token bucket** for general API rate limiting (e.g., 100 requests/min per user)
    - **Sliding window** for action-specific limits (e.g., 5 posts/hour, 1 community creation/day)
  - Rate limit tiers:
    - **Anonymous users**: Strict (10 req/min)
    - **Authenticated users**: Moderate (100 req/min)
    - **Trusted users** (high karma): Relaxed (300 req/min)
  - Per-endpoint limits:
    - POST `/posts`: 5 per hour
    - POST `/comments`: 30 per hour
    - POST `/votes`: 60 per minute
    - POST `/communities`: 1 per day
    - POST `/register`: 3 per hour per IP
  - Returns `429 Too Many Requests` with `Retry-After` header
  - Redis stores counters with TTL expiry (no manual cleanup needed)
  - Circuit breaker: if a single user exceeds limits repeatedly, temporarily ban the token

### 4.11 Spam and Abuse Detection Service

- **Responsibility**: Detect and prevent spam, bot activity, and abusive content
- **Database**: Redis (real-time scoring) + PostgreSQL (ban records, audit log)
- **Key details**:
  - **Pre-publish checks** (synchronous, before content is saved):
    - Keyword/regex blocklist filtering (slurs, known spam patterns)
    - URL reputation check (block known phishing/malware domains)
    - Duplicate content detection (same user posting identical content)
    - New account restrictions: accounts < 24h old cannot post, < 1h cannot comment
  - **Post-publish analysis** (asynchronous, via Kafka events):
    - Pattern detection: rapid posting, same link spammed across communities
    - Vote manipulation detection: coordinated upvote rings (cluster analysis on vote timing)
    - Account behavior scoring: assign a trust score based on age, karma, report history
  - **Actions**:
    - Auto-remove content that fails pre-publish checks
    - Flag suspicious content for moderator review
    - Shadow-ban: user's posts are visible only to themselves (they don't know they're banned)
    - IP-level temporary ban for severe abuse
  - **Moderator integration**:
    - Moderators can report users to the global abuse system
    - AutoMod-style rules per community (configurable regex patterns, account age requirements)
  - **Audit log**: Every automated action is logged for review and appeal

### 4.12 Moderation Service

- **Responsibility**: Community-level content moderation, user bans, mod tools
- **Database**: PostgreSQL
- **Key details**:
  - Remove/approve posts and comments
  - Ban/mute users from specific communities (with duration and reason)
  - Pin/sticky up to 2 posts per community
  - Mod queue: list of reported/flagged content for review
  - Mod log: all moderation actions are logged (transparency)

## 5. User Anonymity Design

Anonymity is a core design principle, not an afterthought.

### 5.1 What Other Users See

- Users are identified **only by username** — no real name, no avatar by default
- No email, phone, or location is ever shown publicly
- Optional **anonymous posting**: a user can post/comment as `[anonymous]` within a community
  - The username is hidden from all other users
  - Only the community moderators can see the real username (for moderation purposes)
  - Anonymous posts still follow all rules and rate limits

### 5.2 What the System Stores

- **Registration requires**: email + username + password (or Google OAuth + username)
- Email is **verified via OTP** but **never shown publicly** — used only for auth, OTP, and account recovery
- **No phone number**, no real name, no date of birth
- **IP addresses are NOT stored** in application databases
  - If needed for abuse detection, IPs are hashed (SHA-256 + salt) and stored temporarily (TTL 24h in Redis)
  - Raw IPs exist only in transient request logs, rotated and deleted after 7 days
- **User-agent strings** are not persisted

### 5.3 Data Protection

- Passwords hashed with **argon2id** (memory-hard, GPU-resistant)
- Email encrypted at rest with AES-256
- Database-level encryption at rest for all PostgreSQL instances
- TLS everywhere (mTLS between microservices via Istio)
- **Account deletion** is a true purge: all posts become `[deleted]`, all PII is wiped, vote records are anonymized

## 6. Communication Patterns

### 6.1 Synchronous (gRPC, request-response)

Used when the caller **needs an immediate answer**:
- User requests a page -> API Gateway -> Post Service -> returns posts
- User submits a comment -> API Gateway -> Comment Service -> returns created comment
- Auth token validation -> API Gateway -> Auth Service -> returns valid/invalid

### 6.2 Asynchronous (Kafka, event-driven)

Used when actions can be **processed later** or need to **fan out to multiple consumers**:
- User votes -> Vote Service publishes `VoteCreated` event:
  - Post Service consumes it -> updates post score
  - User Service consumes it -> updates karma
  - Spam Detection consumes it -> checks for vote manipulation
- User creates a post -> Post Service publishes `PostCreated` event:
  - Search Service consumes it -> indexes the post
  - Notification Service consumes it -> notifies community followers
  - Spam Detection consumes it -> analyzes for spam
- Moderator removes a post -> Moderation Service publishes `PostRemoved` event:
  - Post Service consumes it -> marks post as removed
  - Notification Service consumes it -> notifies the author
  - Search Service consumes it -> removes from index

### 6.3 Real-Time (WebSocket)

Used for **live updates** pushed to the browser:
- User authenticates -> opens persistent WebSocket connection to Notification Service
- Notification Service pushes events to the WebSocket:
  - New notification badge count
  - Notification content (reply, mention, mod action)
- Optional: live vote count updates on posts currently being viewed
- WebSocket connections are load-balanced across Notification Service pods
- Redis pub/sub used internally so any pod can route a message to the correct WebSocket-holding pod

## 7. Database Sharding Strategy (Post Service)

### 7.1 Shard Key: `community_id` with Consistent Hashing

All posts for a given community land on the same shard. This keeps feed queries (all posts in `r/golang`) hitting a single shard, avoiding cross-shard joins.

```
community_id -> hash(community_id) -> shard ring -> target shard
```

### 7.2 Adding Shards (Resharding)

When you add a new shard to the ring, consistent hashing ensures only ~1/N of the data migrates (where N = number of shards). Steps:
1. Add the new shard node to the hash ring
2. Identify which community_ids now map to the new shard
3. Copy those communities' posts from old shard to new shard (background migration)
4. Flip routing to new shard for those communities
5. Delete migrated data from old shard

Use **virtual nodes** (each physical shard has multiple positions on the ring) to ensure even distribution.

### 7.3 Hot Shard Mitigation

If a single community (e.g., `r/funny`) grows massive and dominates one shard:

- **More virtual nodes**: Increase virtual nodes per physical shard for better distribution
- **Shard splitting**: When a shard exceeds a size/traffic threshold, split it by adding a new physical node to the ring — consistent hashing ensures only ~1/N data migrates
- **Read replicas**: Add read replicas for hot shards to handle read traffic; writes still go to the primary
- **Caching layer**: Hot community feeds are aggressively cached in Redis (5 min TTL), reducing shard read pressure

### 7.4 Implementation

Application-level consistent hashing implemented in Go:
- A shard routing layer in the Post Service maps `community_id` -> shard via hash ring
- The routing config (ring topology, virtual node count) is stored in a shared config (etcd or ConfigMap)
- All Post Service pods use the same ring config, ensuring consistent routing
- Migration tooling: a background Go worker that copies data during resharding events

## 8. Caching Strategy (Redis)

| Cache Target         | TTL     | Invalidation Strategy              |
| -------------------- | ------- | ---------------------------------- |
| Community metadata   | 1 hour  | Invalidate on community update     |
| Hot post feed        | 5 min   | TTL-based, regenerate on expiry    |
| Post vote count      | Real-time | Write-through from Vote Service  |
| User session/token   | 15 min  | Invalidate on logout               |
| Rate limit counters  | Per-window | Auto-expire with TTL             |
| User karma           | 10 min  | Invalidate on Kafka vote event     |
| Search autocomplete  | 30 min  | TTL-based                          |

## 9. Kubernetes Deployment

Each microservice is deployed as a separate **k8s Deployment** with:
- **HPA** (Horizontal Pod Autoscaler) based on CPU/memory and custom metrics (request rate)
- **Resource limits** (CPU and memory per pod)
- **Readiness and liveness probes** (gRPC health checks)
- **ConfigMaps** for non-sensitive config, **Secrets** for credentials
- **Namespace isolation** per environment (dev, staging, prod)

### Pod scaling expectations:

| Service          | Min Pods | Scale Trigger               |
| ---------------- | -------- | --------------------------- |
| API Gateway      | 3        | Request rate > 1000/s       |
| Auth Service     | 2        | CPU > 70%                   |
| Post Service     | 3        | Request rate, DB connections |
| Vote Service     | 3        | Kafka consumer lag           |
| Comment Service  | 2        | Request rate                 |
| Search Service   | 2        | Query latency > 200ms       |
| Notification     | 2        | Kafka consumer lag           |
| Spam Detection   | 2        | Kafka consumer lag           |

## 10. Security Measures

- **TLS everywhere**: Envoy terminates external TLS, Istio provides mTLS between services
- **Auth**: JWT with short-lived access tokens (15 min) + refresh tokens (7 days)
- **CORS**: Strict origin allowlist at the API Gateway
- **CSRF**: Token-based protection for state-changing requests
- **Input validation**: All inputs validated at the gateway and at each service boundary
- **SQL injection**: Parameterized queries only (no string concatenation)
- **Secret management**: k8s Secrets or HashiCorp Vault
- **Circuit breakers**: `sony/gobreaker` in Go — prevent cascade failures
- **Idempotency**: Vote and post creation endpoints are idempotent (safe to retry)
- **Database encryption at rest**: PostgreSQL with TDE or volume-level encryption

## 11. Observability

- **Metrics**: Prometheus scrapes each Go service via `/metrics` endpoint, Grafana dashboards for latency, error rates, throughput per service
- **Logging**: Structured JSON logs from all services -> Loki for centralized aggregation, queried via Grafana
- **Tracing**: OpenTelemetry SDK in each Go service -> Jaeger for distributed trace visualization
- **Alerting**: Grafana alerts on error rate spikes, latency P99 > thresholds, pod restarts, Kafka consumer lag
- **Dashboards**: Grafana dashboards per service + a global overview dashboard

## 12. Database Inventory

Total: **5 PostgreSQL instances + 1 ScyllaDB cluster + 1 Redis cluster + 1 Meilisearch instance + 1 AWS S3 bucket**

### 12.1 PostgreSQL Instances (5)

| #   | Instance       | Owned By                                              | Sharded | Data Stored                                                               |
| --- | -------------- | ----------------------------------------------------- | ------- | ------------------------------------------------------------------------- |
| 1   | `pg-auth`      | Auth Service                                          | No      | Credentials (hashed passwords, OAuth tokens, OTP records, emails)         |
| 2   | `pg-user`      | User Service                                          | No      | Profiles, karma, settings, avatars                                        |
| 3   | `pg-community` | Community Service                                     | No      | Communities, memberships, roles, rules                                    |
| 4   | `pg-post`      | Post Service                                          | Yes     | Posts (sharded by `community_id` via consistent hashing)                  |
| 5   | `pg-platform`  | Moderation + Spam + Notification + Vote (persistence) | No      | Mod logs, ban records, audit logs, notification history, vote persistence |

> [!note] `pg-platform` consolidation
> Moderation, Spam Detection, Notification, and Vote services share one PostgreSQL instance because their persistent storage needs are low-volume and non-overlapping. Each service uses its own **schema** within this instance for isolation. If any service outgrows this, it can be split into its own instance later.

### 12.2 ScyllaDB Cluster (1)

| # | Cluster          | Owned By        | Data Stored                                          |
| - | ---------------- | --------------- | ---------------------------------------------------- |
| 1 | `scylla-comments`| Comment Service | All comments, partitioned by `post_id`, clustered by materialized path |

### 12.3 Redis Cluster (1 cluster, multiple logical databases)

| Logical DB | Used By                        | Data Stored                                          |
| ---------- | ------------------------------ | ---------------------------------------------------- |
| db0        | Vote Service                   | Current vote state per user per item, real-time vote counts |
| db1        | Rate Limiting (Envoy + services) | Token bucket / sliding window counters with TTL     |
| db2        | Auth Service                   | OTP codes (TTL 5 min), refresh token blacklist       |
| db3        | Notification Service           | WebSocket connection registry, unread counts         |
| db4        | Caching (all services)         | Community metadata, hot feeds, user karma, search autocomplete |
| db5        | Spam Detection                 | Real-time behavior scoring, IP hashes (TTL 24h)     |

### 12.4 Meilisearch Instance (1)

| # | Instance           | Owned By       | Data Stored                                    |
| - | ------------------ | -------------- | ---------------------------------------------- |
| 1 | `meili-search`     | Search Service | Indexed posts, comments, community names       |

### 12.5 AWS S3 (1 bucket)

| # | Bucket             | Owned By      | Data Stored                                     |
| - | ------------------ | ------------- | ----------------------------------------------- |
| 1 | `reddit-media`     | Media Service | User-uploaded images, videos, thumbnails, community banners/icons |

### 12.6 Summary

| Database     | Instances/Clusters | Total Services Using It |
| ------------ | ------------------ | ----------------------- |
| PostgreSQL   | 5                  | 8 (Auth, User, Community, Post, Moderation, Spam, Notification, Vote) |
| ScyllaDB     | 1                  | 1 (Comment)             |
| Redis        | 1 (6 logical DBs)  | 6 (Vote, Rate Limit, Auth, Notification, Cache, Spam) |
| Meilisearch  | 1                  | 1 (Search)              |
| AWS S3       | 1 bucket           | 1 (Media)               |
| **Total**    | **9 data stores**  |                         |
