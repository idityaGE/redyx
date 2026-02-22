---
title: "Redyx"
subtitle: "Software Requirement Specification Document"
author: "Aditya"
date: "February 2026"
titlepage: true
titlepage-color: "1a365d"
titlepage-text-color: "FFFFFF"
titlepage-rule-color: "FFFFFF"
titlepage-rule-width: 2
toc: true
toc-own-page: true
colorlinks: true
linkcolor: "blue"
numbersections: false
table-use-row-colors: true
header-left: "Redyx - SRS"
header-right: "February 2026"
footer-left: "Aditya"
footer-center: "Confidential"
code-block-font-size: "\\small"
footnotes-pretty: true
---

# 1. Introduction

Redyx is an anonymous, community-driven discussion platform modeled after Reddit. Users create communities, post content, comment in nested threads, and vote on everything. The system runs on a microservice architecture built with Go, communicating over gRPC, and deployed on Kubernetes.

The distinguishing goal of this project is anonymity. Users interact through pseudonymous usernames. The platform collects minimal personal information and never exposes private data (email, IP, auth method) to other users.

## 1.1 Purpose

This document specifies the functional and non-functional requirements for `Redyx`. It is intended for the development team, the course instructor, and anyone evaluating the project's scope and design decisions.

## 1.2 Scope

Redyx covers:

- User registration and authentication (email/password and Google OAuth with OTP verification)
- Community creation and management
- Text, link, and media posts
- Nested, threaded comments
- Upvote/downvote system with karma tracking
- Feed generation with multiple sorting algorithms
- Full-text search
- Real-time notifications over WebSocket
- Rate limiting, spam detection, and moderation tools
- Media upload and storage

The system is deployed as 12 microservices on Kubernetes, with PostgreSQL (sharded for posts), ScyllaDB (comments), Redis (caching and real-time state), Meilisearch (search), Kafka (event processing), and AWS S3 (media storage).

## 1.3 Definitions and acronyms

| Term               | Definition                                                                           |
| ------------------ | ------------------------------------------------------------------------------------ |
| Community          | A topic-based group where users post content (similar to a subreddit)                |
| Karma              | A user's reputation score, calculated from upvotes received                          |
| OTP                | One-Time Password, a 6-digit code sent via email for verification                    |
| gRPC               | Google's Remote Procedure Call framework using Protocol Buffers                      |
| JWT                | JSON Web Token, used for stateless authentication                                    |
| SSR                | Server-Side Rendering                                                                |
| HPA                | Horizontal Pod Autoscaler in Kubernetes                                              |
| mTLS               | Mutual TLS, both client and server authenticate each other                           |
| Shadow-ban         | A ban where the user's content is hidden from others but still visible to themselves |
| Shard              | A horizontal partition of a database                                                 |
| Consistent hashing | A hashing scheme that minimizes data movement when shards are added or removed       |

## 1.4 References

- IEEE 830-1998: Recommended Practice for Software Requirements Specifications
- Reddit API documentation (https://www.reddit.com/dev/api)
- gRPC documentation (https://grpc.io/docs/)
- Envoy proxy documentation (https://www.envoyproxy.io/docs)
- ScyllaDB documentation (https://docs.scylladb.com)
- Kafka documentation (https://kafka.apache.org/documentation/)

## 1.5 Stakeholders

| Stakeholder       | Role                                               |
| ----------------- | -------------------------------------------------- |
| Aditya            | Developer, project owner                           |
| Course Instructor | Evaluator, approves the SRS and final submission   |
| End Users         | People who will register, post, vote, and moderate |

# 2. System overview

## 2.1 Architecture summary

Redyx uses a microservice architecture. The frontend is an Astro application with server-side rendering and Svelte islands for interactive components. It communicates with an Envoy API gateway over REST/JSON. Envoy translates those requests into gRPC calls to the appropriate backend service.

All backend services are written in Go and communicate with each other over gRPC. Asynchronous workflows (vote processing, search indexing, notifications, spam analysis) are handled through Apache Kafka. Redis is used for caching, rate limiting, real-time vote counts, and WebSocket connection tracking.

The system runs on Kubernetes with Horizontal Pod Autoscalers, namespace isolation per environment, and optional Istio service mesh for mTLS between services.

## 2.2 Tech stack

| Layer               | Technology                     |
| ------------------- | ------------------------------ |
| Frontend            | Astro (SSR) + Svelte (islands) |
| API Gateway         | Envoy                          |
| Backend Services    | Go (Golang)                    |
| Service-to-Service  | gRPC with Protocol Buffers     |
| Frontend-to-Gateway | REST/JSON                      |
| Real-Time           | WebSocket                      |
| Message Queue       | Apache Kafka                   |
| Cache               | Redis                          |
| Primary Database    | PostgreSQL                     |
| Comment Store       | ScyllaDB                       |
| Search Engine       | Meilisearch                    |
| Object Storage      | AWS S3                         |
| Orchestration       | Kubernetes                     |
| Monitoring          | Prometheus + Grafana           |
| Logging             | Loki                           |
| Tracing             | OpenTelemetry + Jaeger         |
| CI/CD               | GitHub Actions + ArgoCD        |

## 2.3 Services

The backend is split into 12 services:

1. Auth Service -- registration, login, OAuth, OTP, token management
2. User Service -- profiles, karma, settings
3. Community Service -- community CRUD, membership, roles
4. Post Service -- post CRUD, feed generation, ranking
5. Comment Service -- nested comment threads
6. Vote Service -- upvotes/downvotes, score aggregation
7. Search Service -- full-text search indexing and queries
8. Media Service -- image/video upload and processing
9. Notification Service -- real-time WebSocket notifications
10. Rate Limiting Service -- request throttling at the gateway
11. Spam/Abuse Detection Service -- content filtering, behavior scoring
12. Moderation Service -- community-level mod tools, bans, mod logs

# 3. Functional requirements

## 3.1 Authentication (FR-AUTH)

| ID        | Requirement                                                                        | Priority |
| --------- | ---------------------------------------------------------------------------------- | -------- |
| FR-AUTH-1 | Users can register with email, username, and password                              | P0       |
| FR-AUTH-2 | Users can register via Google OAuth, then choose a username                        | P0       |
| FR-AUTH-3 | Email is verified through a 6-digit OTP before account activation                  | P0       |
| FR-AUTH-4 | Users can log in with email/password or Google OAuth                               | P0       |
| FR-AUTH-5 | The system issues a short-lived access token (15 min) and a refresh token (7 days) | P0       |
| FR-AUTH-6 | Users can log out, which invalidates the refresh token                             | P0       |
| FR-AUTH-7 | Passwords are hashed with `argon2id` before storage                                | P0       |
| FR-AUTH-8 | Email and auth method are never exposed to other users                             | P0       |

## 3.2 User profiles (FR-USER)

| ID         | Requirement                                                    | Priority |
| ---------- | -------------------------------------------------------------- | -------- |
| FR-USER-1  | Each user has a public profile showing username, karma, and cake day | P1  |
| FR-USER-2  | The profile displays the user's post and comment history        | P1       |
| FR-USER-3  | Karma is calculated from the total upvotes received on posts and comments | P1 |
| FR-USER-4  | Users can update their display settings and avatar              | P2       |
| FR-USER-5  | Users can delete their account, which wipes all PII and replaces their posts/comments with `[deleted]` | P1 |

## 3.3 Communities (FR-COMM)

| ID        | Requirement                                                         | Priority |
| --------- | ------------------------------------------------------------------- | -------- |
| FR-COMM-1 | Any authenticated user can create a community with a unique name    | P0       |
| FR-COMM-2 | Community names are immutable once created                          | P0       |
| FR-COMM-3 | Communities have a description, rules, banner, and icon             | P1       |
| FR-COMM-4 | Communities can be set to public, restricted, or private visibility | P1       |
| FR-COMM-5 | Users can join and leave communities                                | P0       |
| FR-COMM-6 | The creator of a community is automatically a moderator             | P0       |
| FR-COMM-7 | Moderators can assign other users as moderators                     | P1       |

## 3.4 Posts (FR-POST)

| ID        | Requirement                                                                   | Priority |
| --------- | ----------------------------------------------------------------------------- | -------- |
| FR-POST-1 | Users can create text posts (title + markdown body) in a community            | P0       |
| FR-POST-2 | Users can create link posts (title + URL)                                     | P1       |
| FR-POST-3 | Users can create image/video posts (title + media upload)                     | P2       |
| FR-POST-4 | Each post belongs to exactly one community                                    | P0       |
| FR-POST-5 | Posts display: title, author, community, timestamp, vote score, comment count | P0       |
| FR-POST-6 | Users can edit and delete their own posts                                     | P0       |
| FR-POST-7 | The home feed aggregates posts from all communities the user has joined       | P1       |
| FR-POST-8 | Posts can be sorted by Hot, New, Top (with time filter), and Rising           | P1       |
| FR-POST-9 | Users can optionally post as `[anonymous]` within a community                 | P2       |

## 3.5 Comments (FR-CMNT)

| ID         | Requirement                                                    | Priority |
| ---------- | -------------------------------------------------------------- | -------- |
| FR-CMNT-1  | Users can comment on posts                                      | P0       |
| FR-CMNT-2  | Users can reply to comments, forming nested threads             | P0       |
| FR-CMNT-3  | Comments display: author, timestamp, vote score, reply count    | P0       |
| FR-CMNT-4  | Comments can be sorted by Best, Top, New, Controversial         | P1       |
| FR-CMNT-5  | Deleted comments show `[deleted]` but the thread structure remains intact | P0 |
| FR-CMNT-6  | Deep threads are lazy-loaded (top 2-3 levels shown, rest on demand) | P1  |

## 3.6 Voting (FR-VOTE)

| ID         | Requirement                                                    | Priority |
| ---------- | -------------------------------------------------------------- | -------- |
| FR-VOTE-1  | Users can upvote or downvote any post or comment                | P0       |
| FR-VOTE-2  | Each user gets one vote per item; they can change or remove it  | P0       |
| FR-VOTE-3  | Net score (upvotes minus downvotes) is displayed on each item   | P0       |
| FR-VOTE-4  | Votes update the author's karma asynchronously via Kafka        | P1       |
| FR-VOTE-5  | Vote endpoints are idempotent (duplicate requests are safe)     | P0       |

## 3.7 Search (FR-SRCH)

| ID         | Requirement                                                    | Priority |
| ---------- | -------------------------------------------------------------- | -------- |
| FR-SRCH-1  | Users can search posts by title and body text                   | P2       |
| FR-SRCH-2  | Users can search within a specific community or globally        | P2       |
| FR-SRCH-3  | Community name autocomplete is available in the search bar      | P2       |
| FR-SRCH-4  | Search results are ranked by relevance, recency, and vote score | P2      |

## 3.8 Notifications (FR-NOTF)

| ID         | Requirement                                                    | Priority |
| ---------- | -------------------------------------------------------------- | -------- |
| FR-NOTF-1  | Users receive a notification when someone replies to their post or comment | P1 |
| FR-NOTF-2  | Users receive a notification when mentioned with `u/username`   | P2       |
| FR-NOTF-3  | Notifications are delivered in real time via WebSocket           | P1       |
| FR-NOTF-4  | If the user is offline, notifications are stored and delivered on reconnect | P1 |
| FR-NOTF-5  | Users can mark notifications as read                            | P1       |
| FR-NOTF-6  | Users can configure notification preferences (mute communities, mute replies) | P2 |

## 3.9 Media (FR-MDIA)

| ID         | Requirement                                                    | Priority |
| ---------- | -------------------------------------------------------------- | -------- |
| FR-MDIA-1  | Users can upload images and videos when creating a post         | P2       |
| FR-MDIA-2  | Uploaded files are validated for type and size before storage   | P2       |
| FR-MDIA-3  | Thumbnails are generated for image uploads                      | P2       |
| FR-MDIA-4  | Media is stored in AWS S3 and served through CloudFront CDN     | P2       |

## 3.10 Moderation (FR-MOD)

| ID        | Requirement                                                     | Priority |
| --------- | --------------------------------------------------------------- | -------- |
| FR-MOD-1  | Moderators can remove posts and comments from their community    | P2       |
| FR-MOD-2  | Moderators can ban users from their community (with duration and reason) | P2 |
| FR-MOD-3  | Moderators can pin up to 2 posts in their community              | P2       |
| FR-MOD-4  | All moderation actions are recorded in a mod log                 | P2       |
| FR-MOD-5  | Moderators can view a queue of reported/flagged content          | P2       |

## 3.11 Rate limiting (FR-RATE)

| ID         | Requirement                                                    | Priority |
| ---------- | -------------------------------------------------------------- | -------- |
| FR-RATE-1  | The API gateway enforces per-user request rate limits            | P1       |
| FR-RATE-2  | Rate limits are tiered: anonymous (10/min), authenticated (100/min), trusted (300/min) | P1 |
| FR-RATE-3  | Action-specific limits apply: 5 posts/hour, 30 comments/hour, 60 votes/min | P1 |
| FR-RATE-4  | Exceeding the limit returns HTTP 429 with a Retry-After header  | P1       |

## 3.12 Spam and abuse detection (FR-SPAM)

| ID         | Requirement                                                    | Priority |
| ---------- | -------------------------------------------------------------- | -------- |
| FR-SPAM-1  | Content is checked against a keyword blocklist before publishing | P2      |
| FR-SPAM-2  | URLs in posts are checked against a known-bad domain list       | P2       |
| FR-SPAM-3  | Duplicate content from the same user is rejected                | P2       |
| FR-SPAM-4  | New accounts (< 24h old) cannot create posts; accounts < 1h old cannot comment | P1 |
| FR-SPAM-5  | Asynchronous analysis detects vote manipulation patterns         | P3      |
| FR-SPAM-6  | Shadow-banning is available as a moderation action               | P3      |

# 4. Non-functional requirements

## 4.1 Performance

| ID        | Requirement                                                      |
| --------- | ---------------------------------------------------------------- |
| NFR-PERF-1| API response time for read operations (feed, post, comments) should be under 200ms at the 95th percentile |
| NFR-PERF-2| Vote processing latency from click to updated score display should be under 500ms |
| NFR-PERF-3| The system should handle at least 10,000 concurrent users without degradation |
| NFR-PERF-4| WebSocket notification delivery should occur within 1 second of the triggering event |
| NFR-PERF-5| Search queries should return results within 300ms                 |

## 4.2 Scalability

| ID         | Requirement                                                                                      |
| ---------- | ------------------------------------------------------------------------------------------------ |
| NFR-SCAL-1 | The Post Service database is sharded using consistent hashing on `community_id`                  |
| NFR-SCAL-2 | New shards can be added with minimal data migration (~1/N of total data)                         |
| NFR-SCAL-3 | Each microservice can be independently scaled via Kubernetes HPA                                 |
| NFR-SCAL-4 | Kafka consumer groups allow parallel event processing per service                                |
| NFR-SCAL-5 | Redis caching reduces direct database load for hot data (community metadata, feeds, vote counts) |

## 4.3 Security

| ID         | Requirement                                                     |
| ---------- | --------------------------------------------------------------- |
| NFR-SEC-1  | All external traffic is encrypted with TLS; internal traffic uses mTLS via Istio |
| NFR-SEC-2  | Passwords are hashed with argon2id                               |
| NFR-SEC-3  | Emails are encrypted at rest with AES-256                        |
| NFR-SEC-4  | IP addresses are never stored in application databases; hashed IPs for abuse detection expire after 24 hours |
| NFR-SEC-5  | JWT access tokens expire after 15 minutes; refresh tokens after 7 days |
| NFR-SEC-6  | CORS is restricted to the frontend origin; CSRF tokens protect state-changing requests |
| NFR-SEC-7  | All database queries use parameterized statements (no string concatenation) |
| NFR-SEC-8  | Secrets are managed through Kubernetes Secrets or HashiCorp Vault |

## 4.4 Anonymity and privacy

| ID         | Requirement                                                     |
| ---------- | --------------------------------------------------------------- |
| NFR-PRIV-1 | Other users can only see a user's username, karma, and cake day  |
| NFR-PRIV-2 | Email, authentication method, and IP are never exposed through any API endpoint |
| NFR-PRIV-3 | Account deletion removes all PII; posts and comments are replaced with `[deleted]`; vote records are anonymized |
| NFR-PRIV-4 | Anonymous posting hides the username from all users except community moderators |
| NFR-PRIV-5 | Raw IP addresses in request logs are rotated and deleted after 7 days |

## 4.5 Reliability and availability

| ID         | Requirement                                                     |
| ---------- | --------------------------------------------------------------- |
| NFR-REL-1  | The system targets 99.9% uptime (roughly 8.7 hours of downtime per year) |
| NFR-REL-2  | Each service has readiness and liveness probes; Kubernetes restarts unhealthy pods automatically |
| NFR-REL-3  | Circuit breakers (sony/gobreaker) prevent cascade failures between services |
| NFR-REL-4  | Kafka provides durability for events; if a consumer goes down, it resumes from the last committed offset |
| NFR-REL-5  | PostgreSQL instances have automated daily backups with point-in-time recovery |

## 4.6 Usability

| ID         | Requirement                                                     |
| ---------- | --------------------------------------------------------------- |
| NFR-USE-1  | Page load time should be under 2 seconds on a fast 4G connection |
| NFR-USE-2  | The Astro frontend ships minimal JavaScript; interactive elements load as Svelte islands |
| NFR-USE-3  | The application is responsive and usable on mobile, tablet, and desktop |
| NFR-USE-4  | Error messages are specific (e.g., "Username already taken" rather than "Registration failed") |

## 4.7 Observability

| ID         | Requirement                                                     |
| ---------- | --------------------------------------------------------------- |
| NFR-OBS-1  | Prometheus collects metrics from every service; Grafana provides per-service dashboards |
| NFR-OBS-2  | All services emit structured JSON logs, aggregated in Loki and queryable through Grafana |
| NFR-OBS-3  | OpenTelemetry traces span across service boundaries; Jaeger visualizes end-to-end request flows |
| NFR-OBS-4  | Grafana alerts fire on error rate spikes, P99 latency exceeding thresholds, pod restarts, and Kafka consumer lag |

# 5. Database design

## 5.1 Database inventory

The system uses 9 data stores in total:

| Database     | Instances/Clusters | Services                                                      |
| ------------ | ------------------ | ------------------------------------------------------------- |
| PostgreSQL   | 5                  | Auth, User, Community, Post (sharded), Platform (Mod + Spam + Notification + Vote) |
| ScyllaDB     | 1                  | Comment                                                       |
| Redis        | 1 (6 logical DBs)  | Vote, Rate Limit, Auth OTP, Notification, Cache, Spam        |
| Meilisearch  | 1                  | Search                                                        |
| AWS S3       | 1 bucket           | Media                                                         |

## 5.2 PostgreSQL instances

| Instance       | Owner                                | Sharded | Contents                                                              |
| -------------- | ------------------------------------ | ------- | --------------------------------------------------------------------- |
| `pg-auth`      | Auth Service                         | No      | Hashed passwords, OAuth tokens, OTP records, encrypted emails         |
| `pg-user`      | User Service                         | No      | Profiles, karma, settings, avatars                                    |
| `pg-community` | Community Service                    | No      | Communities, memberships, roles, rules                                |
| `pg-post`      | Post Service                         | Yes     | Posts, sharded by `community_id` via consistent hashing               |
| `pg-platform`  | Moderation, Spam, Notification, Vote | No      | Mod logs, ban records, audit logs, notification history, vote records |

The `pg-platform` instance is shared because these four services have low-volume, non-overlapping storage needs. Each service uses a separate schema within the instance for isolation.

## 5.3 ScyllaDB

A single cluster (`scylla-comments`) stores all comments. The partition key is `post_id`, so all comments for a given post live in the same partition. The clustering key is a materialized path (e.g., `root/parent/child`) that determines tree ordering.

## 5.4 Redis logical databases

| DB   | Owner              | Contents                                          |
| ---- | ------------------ | ------------------------------------------------- |
| db0  | Vote Service       | Vote state per user per item, real-time vote counts |
| db1  | Rate Limiting      | Token bucket / sliding window counters with TTL   |
| db2  | Auth Service       | OTP codes (5 min TTL), refresh token blacklist    |
| db3  | Notification       | WebSocket connection registry, unread counts      |
| db4  | Cache (shared)     | Community metadata, hot feeds, karma, autocomplete |
| db5  | Spam Detection     | Behavior scores, hashed IPs (24h TTL)             |

## 5.5 Sharding strategy

The Post Service database is sharded using application-level consistent hashing. Each `community_id` is hashed onto a ring with virtual nodes. All posts for a community land on the same shard, keeping feed queries local to one shard.

When adding a new shard:

1. Add the new node to the hash ring
2. Identify which `community_id` values now map to the new shard
3. Copy affected posts in the background
4. Switch routing for those communities
5. Clean up old copies

Virtual nodes (multiple ring positions per physical shard) keep the distribution even. Hot shards are mitigated with read replicas and aggressive Redis caching.

# 6. Communication patterns

## 6.1 Synchronous (gRPC)

Request-response calls for operations where the client needs an immediate answer:

- Loading a page: Astro SSR -> Envoy -> Post Service -> returns posts as JSON
- Submitting a comment: Browser -> Envoy -> Comment Service -> returns the created comment
- Token validation: Envoy checks the JWT signature; only calls Auth Service for refresh token rotation

## 6.2 Asynchronous (Kafka)

Events published to Kafka topics, consumed by multiple services independently:

- `VoteCreated`: consumed by Post Service (update score), User Service (update karma), Spam Detection (check for manipulation)
- `PostCreated`: consumed by Search Service (index), Notification Service (alert followers), Spam Detection (analyze)
- `PostRemoved`: consumed by Post Service (mark removed), Notification Service (alert author), Search Service (remove from index)

## 6.3 Real-time (WebSocket)

Each authenticated user opens a persistent WebSocket connection. The Notification Service maintains a Redis-backed registry mapping each user to the pod holding their connection. When a Kafka event arrives, the service routes the notification to the correct pod, which pushes it to the user's WebSocket. Offline users receive their notifications on the next connection.

# 7. Time frame

| Phase   | Duration   | Deliverables                                                        |
| ------- | ---------- | ------------------------------------------------------------------- |
| Phase 1 | Weeks 1-3  | Auth Service, User Service, Community Service, basic Astro frontend |
| Phase 2 | Weeks 4-6  | Post Service (with sharding), Comment Service, Vote Service         |
| Phase 3 | Weeks 7-9  | Search Service, Media Service, Notification Service (WebSocket)     |
| Phase 4 | Weeks 10-11| Rate Limiting, Spam Detection, Moderation Service                   |
| Phase 5 | Week 12    | Kubernetes deployment, monitoring (Prometheus + Grafana + Loki), load testing, documentation |

# 8. Acceptance criteria

| ID    | Criterion                                                                   |
| ----- | --------------------------------------------------------------------------- |
| AC-1  | A user can register with email/password, verify via OTP, and log in         |
| AC-2  | A user can register via Google OAuth and choose a username                   |
| AC-3  | A user can create a community and another user can join it                   |
| AC-4  | A user can create a text post in a community and it appears in the feed      |
| AC-5  | A user can comment on a post and reply to existing comments (at least 3 levels deep) |
| AC-6  | Upvoting/downvoting updates the score and the author's karma                |
| AC-7  | The home feed aggregates posts from joined communities, sorted by Hot/New/Top |
| AC-8  | Search returns relevant posts by title and body within 300ms                 |
| AC-9  | A WebSocket notification is received within 1 second when someone replies to your post |
| AC-10 | Rate limiting returns HTTP 429 when the threshold is exceeded                |
| AC-11 | A moderator can remove a post, ban a user, and pin a post in their community |
| AC-12 | Account deletion wipes PII and replaces content with `[deleted]`             |
| AC-13 | The post database is sharded; adding a shard migrates only a fraction of the data |
| AC-14 | All services run on Kubernetes with health checks and auto-scaling           |

# 9. References

- IEEE 830-1998, IEEE Recommended Practice for Software Requirements Specifications
- Reddit API documentation: https://www.reddit.com/dev/api
- gRPC official documentation: https://grpc.io/docs/
- Envoy proxy documentation: https://www.envoyproxy.io/docs
- ScyllaDB documentation: https://docs.scylladb.com
- Apache Kafka documentation: https://kafka.apache.org/documentation/
- Meilisearch documentation: https://www.meilisearch.com/docs
- Kubernetes documentation: https://kubernetes.io/docs/
- Astro documentation: https://docs.astro.build
- System Design concepts: https://medium.com/@shivambhadani_/system-design-for-beginners-everything-you-need-in-one-article-c74eb702540b#25c0

# Revision history

| Version | Date       | Author | Changes         |
| ------- | ---------- | ------ | --------------- |
| 1.0     | 2026-02-11 | Aditya | Initial draft   |
