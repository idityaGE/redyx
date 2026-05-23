---
title: "Redyx"
subtitle: "Feature Completion Tracker"
author: "Aditya"
date: "May 2026"
titlepage: true
titlepage-color: "1a365d"
titlepage-text-color: "FFFFFF"
titlepage-rule-color: "FFFFFF"
titlepage-rule-width: 2
toc: true
toc-own-page: true
colorlinks: true
linkcolor: "blue"
numbersections: true
table-use-row-colors: true
header-left: "Redyx - Features"
header-right: "May 2026"
footer-left: "Aditya"
footer-center: "Confidential"
footnotes-pretty: true
---

# Feature Completion Tracker

This document lists all features and requirements promised in the Redyx Software Requirement Specification (SRS) and tracks their completion status.

---

## 1. Functional Requirements

### 1.1 Authentication (FR-AUTH)

| ID        | Requirement                                                                        | Priority | Status      |
| --------- | ---------------------------------------------------------------------------------- | -------- | ----------- |
| FR-AUTH-1 | Users can register with email, username, and password                              | P0       | Completed   |
| FR-AUTH-2 | Users can register via Google OAuth, then choose a username                        | P0       | Completed   |
| FR-AUTH-3 | Email is verified through a 6-digit OTP before account activation                  | P0       | Completed   |
| FR-AUTH-4 | Users can log in with email/password or Google OAuth                               | P0       | Completed   |
| FR-AUTH-5 | The system issues a short-lived access token (15 min) and a refresh token (7 days) | P0       | Completed   |
| FR-AUTH-6 | Users can log out, which invalidates the refresh token                             | P0       | Completed   |
| FR-AUTH-7 | Passwords are hashed with `argon2id` before storage                                | P0       | Completed   |
| FR-AUTH-8 | Email and auth method are never exposed to other users                             | P0       | Completed   |

### 1.2 User Profiles (FR-USER)

| ID        | Requirement                                                                                               | Priority | Status      |
| --------- | --------------------------------------------------------------------------------------------------------- | -------- | ----------- |
| FR-USER-1 | Each user has a public profile showing username, karma, and cake day                                      | P1       | Completed   |
| FR-USER-2 | The profile displays the user's post and comment history                                                  | P1       | Completed   |
| FR-USER-3 | Karma is calculated from the total upvotes received on posts and comments                                  | P1       | Completed   |
| FR-USER-4 | Users can update their display settings and avatar                                                        | P2       | Completed   |
| FR-USER-5 | Users can delete their account, which wipes all PII and replaces their posts/comments with `[deleted]`    | P1       | Completed   |

### 1.3 Communities (FR-COMM)

| ID        | Requirement                                                          | Priority | Status      |
| --------- | -------------------------------------------------------------------- | -------- | ----------- |
| FR-COMM-1 | Any authenticated user can create a community with a unique name     | P0       | Completed   |
| FR-COMM-2 | Community names are immutable once created                           | P0       | Completed   |
| FR-COMM-3 | Communities have a description, rules, banner, and icon              | P1       | Completed   |
| FR-COMM-4 | Communities can be set to public, restricted, or private visibility  | P1       | Completed   |
| FR-COMM-5 | Users can join and leave communities                                 | P0       | Completed   |
| FR-COMM-6 | The creator of a community is automatically a moderator              | P0       | Completed   |
| FR-COMM-7 | Moderators can assign other users as moderators                      | P1       | Completed   |

### 1.4 Posts (FR-POST)

| ID         | Requirement                                                                    | Priority | Status      |
| ---------- | ------------------------------------------------------------------------------ | -------- | ----------- |
| FR-POST-1  | Users can create text posts (title + markdown body) in a community             | P0       | Completed   |
| FR-POST-2  | Users can create link posts (title + URL)                                      | P1       | Completed   |
| FR-POST-3  | Users can create image/video posts (title + media upload)                      | P2       | Completed   |
| FR-POST-4  | Each post belongs to exactly one community                                     | P0       | Completed   |
| FR-POST-5  | Posts display: title, author, community, timestamp, vote score, comment count  | P0       | Completed   |
| FR-POST-6  | Users can edit and delete their own posts                                      | P0       | Completed   |
| FR-POST-7  | The home feed aggregates posts from all communities the user has joined        | P1       | Completed   |
| FR-POST-8  | Posts can be sorted by Hot, New, Top (with time filter), and Rising            | P1       | Completed   |
| FR-POST-9  | Users can optionally post as `[anonymous]` within a community                  | P2       | Completed   |

### 1.5 Comments (FR-CMNT)

| ID         | Requirement                                                                     | Priority | Status      |
| ---------- | ------------------------------------------------------------------------------- | -------- | ----------- |
| FR-CMNT-1  | Users can comment on posts                                                      | P0       | Completed   |
| FR-CMNT-2  | Users can reply to comments, forming nested threads                             | P0       | Completed   |
| FR-CMNT-3  | Comments display: author, timestamp, vote score, reply count                    | P0       | Completed   |
| FR-CMNT-4  | Comments can be sorted by Best, Top, New, Controversial                         | P1       | Completed   |
| FR-CMNT-5  | Deleted comments show `[deleted]` but the thread structure remains intact       | P0       | Completed   |
| FR-CMNT-6  | Deep threads are lazy-loaded (top 2-3 levels shown, rest on demand)             | P1       | Completed   |

### 1.6 Voting (FR-VOTE)

| ID         | Requirement                                                      | Priority | Status      |
| ---------- | ---------------------------------------------------------------- | -------- | ----------- |
| FR-VOTE-1  | Users can upvote or downvote any post or comment                 | P0       | Completed   |
| FR-VOTE-2  | Each user gets one vote per item; they can change or remove it   | P0       | Completed   |
| FR-VOTE-3  | Net score (upvotes minus downvotes) is displayed on each item    | P0       | Completed   |
| FR-VOTE-4  | Votes update the author's karma asynchronously via Kafka         | P1       | Completed   |
| FR-VOTE-5  | Vote endpoints are idempotent (duplicate requests are safe)      | P0       | Completed   |

### 1.7 Search (FR-SRCH)

| ID         | Requirement                                                      | Priority | Status      |
| ---------- | ---------------------------------------------------------------- | -------- | ----------- |
| FR-SRCH-1  | Users can search posts by title and body text                    | P2       | Completed   |
| FR-SRCH-2  | Users can search within a specific community or globally         | P2       | Completed   |
| FR-SRCH-3  | Community name autocomplete is available in the search bar       | P2       | Completed   |
| FR-SRCH-4  | Search results are ranked by relevance, recency, and vote score  | P2       | Completed   |

### 1.8 Notifications (FR-NOTF)

| ID         | Requirement                                                                      | Priority | Status      |
| ---------- | -------------------------------------------------------------------------------- | -------- | ----------- |
| FR-NOTF-1  | Users receive a notification when someone replies to their post or comment       | P1       | Completed   |
| FR-NOTF-2  | Users receive a notification when mentioned with `u/username`                    | P2       | Completed   |
| FR-NOTF-3  | Notifications are delivered in real time via WebSocket                           | P1       | Completed   |
| FR-NOTF-4  | If the user is offline, notifications are stored and delivered on reconnect      | P1       | Completed   |
| FR-NOTF-5  | Users can mark notifications as read                                             | P1       | Completed   |
| FR-NOTF-6  | Users can configure notification preferences (mute communities, mute replies)    | P2       | Completed   |

### 1.9 Media (FR-MDIA)

| ID         | Requirement                                                      | Priority | Status      |
| ---------- | ---------------------------------------------------------------- | -------- | ----------- |
| FR-MDIA-1  | Users can upload images and videos when creating a post          | P2       | Completed   |
| FR-MDIA-2  | Uploaded files are validated for type and size before storage    | P2       | Completed   |
| FR-MDIA-3  | Thumbnails are generated for image uploads                       | P2       | Completed   |
| FR-MDIA-4  | Media is stored in AWS S3 and served through CloudFront CDN      | P2       | Completed   |

### 1.10 Moderation (FR-MOD)

| ID         | Requirement                                                       | Priority | Status      |
| ---------- | ----------------------------------------------------------------- | -------- | ----------- |
| FR-MOD-1   | Moderators can remove posts and comments from their community     | P2       | Completed   |
| FR-MOD-2   | Moderators can ban users from their community (with duration and reason) | P2 | Completed   |
| FR-MOD-3   | Moderators can pin up to 2 posts in their community               | P2       | Completed   |
| FR-MOD-4   | All moderation actions are recorded in a mod log                  | P2       | Completed   |
| FR-MOD-5   | Moderators can view a queue of reported/flagged content           | P2       | Completed   |

### 1.11 Rate Limiting (FR-RATE)

| ID         | Requirement                                                                                | Priority | Status      |
| ---------- | ------------------------------------------------------------------------------------------ | -------- | ----------- |
| FR-RATE-1  | The API gateway enforces per-user request rate limits                                      | P1       | Completed   |
| FR-RATE-2  | Rate limits are tiered: anonymous (10/min), authenticated (100/min), trusted (300/min)     | P1       | Completed   |
| FR-RATE-3  | Action-specific limits apply: 5 posts/hour, 30 comments/hour, 60 votes/min                 | P1       | Completed   |
| FR-RATE-4  | Exceeding the limit returns HTTP 429 with a Retry-After header                             | P1       | Completed   |

### 1.12 Spam and Abuse Detection (FR-SPAM)

| ID         | Requirement                                                        | Priority | Status      |
| ---------- | ------------------------------------------------------------------ | -------- | ----------- |
| FR-SPAM-1  | Content is checked against a keyword blocklist before publishing   | P2       | Completed   |
| FR-SPAM-2  | URLs in posts are checked against a known-bad domain list          | P2       | Completed   |
| FR-SPAM-3  | Duplicate content from the same user is rejected                   | P2       | Completed   |
| FR-SPAM-4  | New accounts (< 24h old) cannot create posts; accounts < 1h old cannot comment | P1 | Completed   |
| FR-SPAM-5  | Asynchronous analysis detects vote manipulation patterns           | P3       | Completed   |
| FR-SPAM-6  | Shadow-banning is available as a moderation action                 | P3       | Completed   |

---

## 2. Non-Functional Requirements

### 2.1 Performance (NFR-PERF)

| ID          | Requirement                                                                                               | Status      |
| ----------- | --------------------------------------------------------------------------------------------------------- | ----------- |
| NFR-PERF-1  | API response time for read operations (feed, post, comments) should be under 200ms at the 95th percentile   | Completed   |
| NFR-PERF-2  | Vote processing latency from click to updated score display should be under 500ms                         | Completed   |
| NFR-PERF-3  | The system should handle at least 10,000 concurrent users without degradation                             | Completed   |
| NFR-PERF-4  | WebSocket notification delivery should occur within 1 second of the triggering event                      | Completed   |
| NFR-PERF-5  | Search queries should return results within 300ms                                                         | Completed   |

### 2.2 Scalability (NFR-SCAL)

| ID          | Requirement                                                                                         | Status      |
| ----------- | --------------------------------------------------------------------------------------------------- | ----------- |
| NFR-SCAL-1  | The Post Service database is sharded using consistent hashing on `community_id`                     | Completed   |
| NFR-SCAL-2  | New shards can be added with minimal data migration (~1/N of total data)                            | Completed   |
| NFR-SCAL-3  | Each microservice can be independently scaled via Kubernetes HPA                                    | Completed   |
| NFR-SCAL-4  | Kafka consumer groups allow parallel event processing per service                                   | Completed   |
| NFR-SCAL-5  | Redis caching reduces direct database load for hot data (community metadata, feeds, vote counts)    | Completed   |

### 2.3 Security (NFR-SEC)

| ID          | Requirement                                                                                          | Status      |
| ----------- | ---------------------------------------------------------------------------------------------------- | ----------- |
| NFR-SEC-1   | All external traffic is encrypted with TLS; internal traffic uses mTLS via Istio                     | Completed   |
| NFR-SEC-2   | Passwords are hashed with argon2id                                                                   | Completed   |
| NFR-SEC-3   | Emails are encrypted at rest with AES-256                                                            | Completed   |
| NFR-SEC-4   | IP addresses are never stored in application databases; hashed IPs for abuse detection expire after 24 hours | Completed   |
| NFR-SEC-5   | JWT access tokens expire after 15 minutes; refresh tokens after 7 days                               | Completed   |
| NFR-SEC-6   | CORS is restricted to the frontend origin; CSRF tokens protect state-changing requests                | Completed   |
| NFR-SEC-7   | All database queries use parameterized statements (no string concatenation)                          | Completed   |
| NFR-SEC-8   | Secrets are managed through Kubernetes Secrets or HashiCorp Vault                                    | Completed   |

### 2.4 Anonymity and Privacy (NFR-PRIV)

| ID          | Requirement                                                                                          | Status      |
| ----------- | ---------------------------------------------------------------------------------------------------- | ----------- |
| NFR-PRIV-1  | Other users can only see a user's username, karma, and cake day                                       | Completed   |
| NFR-PRIV-2  | Email, authentication method, and IP are never exposed through any API endpoint                      | Completed   |
| NFR-PRIV-3  | Account deletion removes all PII; posts and comments are replaced with `[deleted]`; vote records are anonymized | Completed   |
| NFR-PRIV-4  | Anonymous posting hides the username from all users except community moderators                      | Completed   |
| NFR-PRIV-5  | Raw IP addresses in request logs are rotated and deleted after 7 days                                | Completed   |

### 2.5 Reliability and Availability (NFR-REL)

| ID          | Requirement                                                                                          | Status      |
| ----------- | ---------------------------------------------------------------------------------------------------- | ----------- |
| NFR-REL-1   | The system targets 99.9% uptime (roughly 8.7 hours of downtime per year)                             | Completed   |
| NFR-REL-2   | Each service has readiness and liveness probes; Kubernetes restarts unhealthy pods automatically     | Completed   |
| NFR-REL-3   | Circuit breakers (sony/gobreaker) prevent cascade failures between services                          | Completed   |
| NFR-REL-4   | Kafka provides durability for events; if a consumer goes down, it resumes from the last committed offset | Completed |
| NFR-REL-5   | PostgreSQL instances have automated daily backups with point-in-time recovery                        | Completed   |

### 2.6 Usability (NFR-USE)

| ID          | Requirement                                                                                          | Status      |
| ----------- | ---------------------------------------------------------------------------------------------------- | ----------- |
| NFR-USE-1   | Page load time should be under 2 seconds on a fast 4G connection                                     | Completed   |
| NFR-USE-2   | The Astro frontend ships minimal JavaScript; interactive elements load as Svelte islands             | Completed   |
| NFR-USE-3   | The application is responsive and usable on mobile, tablet, and desktop                              | Completed   |
| NFR-USE-4   | Error messages are specific (e.g., "Username already taken" rather than "Registration failed")       | Completed   |

### 2.7 Observability (NFR-OBS)

| ID          | Requirement                                                                                          | Status      |
| ----------- | ---------------------------------------------------------------------------------------------------- | ----------- |
| NFR-OBS-1   | Prometheus collects metrics from every service; Grafana provides per-service dashboards              | Completed   |
| NFR-OBS-2   | All services emit structured JSON logs, aggregated in Loki and queryable through Grafana             | Completed   |
| NFR-OBS-3   | OpenTelemetry traces span across service boundaries; Jaeger visualizes end-to-end request flows      | Completed   |
| NFR-OBS-4   | Grafana alerts fire on error rate spikes, P99 latency exceeding thresholds, pod restarts, and Kafka consumer lag | Completed |

---

## 3. Acceptance Criteria

| ID    | Criterion                                                                                    | Status      |
| ----- | -------------------------------------------------------------------------------------------- | ----------- |
| AC-1  | A user can register with email/password, verify via OTP, and log in                          | Completed   |
| AC-2  | A user can register via Google OAuth and choose a username                                   | Completed   |
| AC-3  | A user can create a community and another user can join it                                   | Completed   |
| AC-4  | A user can create a text post in a community and it appears in the feed                      | Completed   |
| AC-5  | A user can comment on a post and reply to existing comments (at least 3 levels deep)         | Completed   |
| AC-6  | Upvoting/downvoting updates the score and the author's karma                                 | Completed   |
| AC-7  | The home feed aggregates posts from joined communities, sorted by Hot/New/Top                | Completed   |
| AC-8  | Search returns relevant posts by title and body within 300ms                                 | Completed   |
| AC-9  | A WebSocket notification is received within 1 second when someone replies to your post       | Completed   |
| AC-10 | Rate limiting returns HTTP 429 when the threshold is exceeded                                | Completed   |
| AC-11 | A moderator can remove a post, ban a user, and pin a post in their community                 | Completed   |
| AC-12 | Account deletion wipes PII and replaces content with `[deleted]`                             | Completed   |
| AC-13 | The post database is sharded; adding a shard migrates only a fraction of the data            | Completed   |
| AC-14 | All services run on Kubernetes with health checks and auto-scaling                           | Completed   |

---

## 4. Summary

| Category                          | Total | Completed | Pending |
| --------------------------------- | ----- | --------- | ------- |
| Functional Requirements (FR)      | 75    | 75        | 0       |
| Non-Functional Requirements (NFR) | 36    | 36        | 0       |
| Acceptance Criteria (AC)          | 14    | 14        | 0       |
| **Grand Total**                   | **125**| **125**  | **0**   |

All promised features and requirements have been successfully implemented.
