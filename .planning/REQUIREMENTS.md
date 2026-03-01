# Requirements: Redyx

**Defined:** 2026-03-02
**Core Value:** Users can anonymously create communities, post content, and have threaded discussions — with minimal personal data collected and maximum privacy preserved.

## v1.0 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Authentication (AUTH)

- [ ] **AUTH-01**: User can register with email, username, and password (argon2id hashed)
- [ ] **AUTH-02**: User receives 6-digit OTP via email to verify account (5min TTL, stored in Redis)
- [ ] **AUTH-03**: User can register via Google OAuth and choose a username
- [ ] **AUTH-04**: User can log in with email/password or Google OAuth
- [ ] **AUTH-05**: System issues JWT access token (15min) and refresh token (7 days)
- [ ] **AUTH-06**: User can log out, invalidating the refresh token
- [ ] **AUTH-07**: User can reset password via email link with token
- [ ] **AUTH-08**: Email and auth method are never exposed to other users through any API endpoint

### User Profiles (USER)

- [ ] **USER-01**: User has public profile showing username, karma, and cake day
- [ ] **USER-02**: Profile displays paginated post and comment history
- [ ] **USER-03**: Karma calculated from total upvotes received on posts and comments
- [ ] **USER-04**: User can update display name, bio (500 chars), and avatar
- [ ] **USER-05**: User can delete account, wiping all PII and replacing content with [deleted]

### Communities (COMM)

- [ ] **COMM-01**: User can create community with unique, immutable name (alphanumeric + underscores, 3-21 chars)
- [ ] **COMM-02**: Community has description (markdown), rules (ordered list), banner, and icon
- [ ] **COMM-03**: Community visibility: public, restricted (view-only for non-approved), or private (invite-only)
- [ ] **COMM-04**: User can join and leave communities (updates member count)
- [ ] **COMM-05**: Creator is automatically assigned owner and moderator roles
- [ ] **COMM-06**: Owner can assign and revoke moderator roles
- [ ] **COMM-07**: Community metadata cached in Redis (1hr TTL, invalidate on update)

### Posts & Feeds (POST)

- [ ] **POST-01**: User can create text post (title max 300 chars + markdown body max 40K chars) in a community
- [ ] **POST-02**: User can create link post (title + validated URL)
- [ ] **POST-03**: User can create media post (title + image/video upload via Media Service)
- [ ] **POST-04**: Post displays title, author, community, timestamp, vote score, and comment count
- [ ] **POST-05**: User can edit and delete own posts (edited shows timestamp, deleted shows [deleted])
- [ ] **POST-06**: Community feed sortable by Hot (Lemmy algorithm), New, Top (hour/day/week/month/year/all), Rising
- [ ] **POST-07**: Home feed aggregates posts from all joined communities with same sorting options
- [ ] **POST-08**: User can post anonymously as [anonymous] within a community (mods see real author)
- [ ] **POST-09**: User can save/bookmark posts to a private "Saved" list
- [ ] **POST-10**: Posts stored on shard determined by consistent hashing of community_id

### Comments (CMNT)

- [ ] **CMNT-01**: User can comment on posts (markdown, max 10K chars, stored in ScyllaDB)
- [ ] **CMNT-02**: User can reply to comments, forming nested threads (materialized path for tree ordering)
- [ ] **CMNT-03**: Comments display author, timestamp, vote score, and reply count
- [ ] **CMNT-04**: Comments sortable by Best (Wilson score confidence interval), Top, New, Controversial
- [ ] **CMNT-05**: Deleted comments show [deleted] but thread structure preserved (children remain visible)
- [ ] **CMNT-06**: Deep threads lazy-loaded (top 2-3 levels shown initially, rest on demand)

### Voting (VOTE)

- [ ] **VOTE-01**: User can upvote or downvote any post or comment
- [ ] **VOTE-02**: One vote per user per item; can change direction or remove (enforced in Redis)
- [ ] **VOTE-03**: Net score (upvotes minus downvotes) displayed on each item, updates within 500ms
- [ ] **VOTE-04**: Votes update author's karma asynchronously via Kafka (set-based idempotent processing)
- [ ] **VOTE-05**: Vote endpoints are idempotent (duplicate requests are safe)

### Search (SRCH)

- [ ] **SRCH-01**: User can search posts by title and body text via Meilisearch (results within 300ms)
- [ ] **SRCH-02**: User can search within a specific community or globally
- [ ] **SRCH-03**: Community name autocomplete in search bar (prefix-based, cached in Redis, triggers after 2+ chars)
- [ ] **SRCH-04**: Search results ranked by relevance, recency, and vote score

### Notifications (NOTF)

- [ ] **NOTF-01**: User receives notification when someone replies to their post or comment
- [ ] **NOTF-02**: User receives notification when mentioned with u/username
- [ ] **NOTF-03**: Notifications delivered in real time via WebSocket (within 1 second)
- [ ] **NOTF-04**: Offline notifications stored in PostgreSQL and delivered on next WebSocket connection
- [ ] **NOTF-05**: User can mark individual or all notifications as read
- [ ] **NOTF-06**: User can configure notification preferences (mute communities, mute reply types)

### Media (MDIA)

- [ ] **MDIA-01**: User can upload images and videos when creating a post
- [ ] **MDIA-02**: Uploaded files validated for type (JPEG, PNG, GIF, WebP) and size (20MB image, 100MB video)
- [ ] **MDIA-03**: Thumbnails generated for image uploads (max 320px wide)
- [ ] **MDIA-04**: Media stored in AWS S3 and served through CloudFront CDN

### Moderation (MOD)

- [ ] **MOD-01**: Moderators can remove posts and comments from their community (hidden from regular users)
- [ ] **MOD-02**: Moderators can ban users from community (temporary or permanent, with duration and reason)
- [ ] **MOD-03**: Moderators can pin up to 2 posts in their community (displayed at top of feed)
- [ ] **MOD-04**: All moderation actions recorded in mod log (who, what, when, reason)
- [ ] **MOD-05**: Moderators can view queue of reported/flagged content (sorted by report count)
- [ ] **MOD-06**: Users can report posts and comments with a reason

### Rate Limiting (RATE)

- [ ] **RATE-01**: API gateway enforces per-user request rate limits via Redis token bucket
- [ ] **RATE-02**: Tiered limits: anonymous (10 req/min), authenticated (100 req/min), trusted (300 req/min)
- [ ] **RATE-03**: Action-specific limits: 5 posts/hour, 30 comments/hour, 60 votes/min, 1 community/day
- [ ] **RATE-04**: Exceeding limit returns HTTP 429 with Retry-After header

### Spam Detection (SPAM)

- [ ] **SPAM-01**: Content checked against keyword blocklist before publishing
- [ ] **SPAM-02**: URLs in posts checked against known-bad domain list
- [ ] **SPAM-03**: Duplicate content from same user rejected (content hash comparison)
- [ ] **SPAM-04**: Async behavior analysis via Kafka detects rapid posting and link spam across communities

### Infrastructure (INFRA)

- [ ] **INFRA-01**: Docker Compose configuration for local development with all services and data stores
- [ ] **INFRA-02**: Kubernetes deployment with HPA, readiness/liveness probes, namespace isolation
- [ ] **INFRA-03**: Prometheus metrics collection from every Go service via /metrics endpoint
- [ ] **INFRA-04**: Grafana dashboards per service and global overview dashboard
- [ ] **INFRA-05**: Loki centralized log aggregation with structured JSON logs from all services
- [ ] **INFRA-06**: OpenTelemetry distributed traces across service boundaries with Jaeger visualization

### Frontend (FEND)

- [x] **FEND-01**: Astro SSR frontend with Svelte interactive islands for dynamic components
- [ ] **FEND-02**: Envoy API gateway with REST-to-gRPC transcoding via proto descriptor set
- [x] **FEND-03**: Responsive layout for desktop, tablet, and mobile
- [ ] **FEND-04**: Page load under 2 seconds on fast 4G connection

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Security Hardening

- **SEC-01**: Shadow-banning as moderation action (user's content invisible to others)
- **SEC-02**: Vote manipulation detection via cluster analysis on vote timing
- **SEC-03**: New account restrictions (accounts < 24h cannot post, < 1h cannot comment)

### Social Features

- **SOCL-01**: User-to-user blocking with content filtering
- **SOCL-02**: Flair/tag system per community with tag-based filtering

### Content

- **CONT-01**: Video transcoding pipeline for uploaded videos
- **CONT-02**: Link preview generation (Open Graph metadata extraction)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Direct messaging / chat | Essentially a second product. Massive scope, abuse prevention, separate infrastructure. Reddit's chat is widely disliked |
| Mobile native app | Web-first approach. Mobile responsive layout is sufficient for v1 |
| OAuth beyond Google | Google covers >80% of social login needs. Each provider adds maintenance burden |
| Infinite scroll | Kills browser performance, breaks back button, accessibility nightmare. Use cursor-based pagination |
| Live vote count broadcasts | Enormous WebSocket overhead for minimal value. Scores update on user action only |
| Email notifications for replies | Spam. Users get annoyed and leave. WebSocket + in-app notifications only for v1 |
| Crossposting | Complex post ownership and feed deduplication. Users can post in multiple communities manually |
| Awards / coins / premium | Massive scope creep. Distracts from core discussion experience |
| Custom CSS per community | Security nightmare (CSS injection). Banners + icons are sufficient for identity |
| NSFW content tagging | Requires classification, preferences, age verification. Communities can self-moderate via rules |
| Browser push notifications | Users hate notification prompts. Lowest acceptance rate of any web feature |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| AUTH-01 | Phase 2 | Pending |
| AUTH-02 | Phase 2 | Pending |
| AUTH-03 | Phase 2 | Pending |
| AUTH-04 | Phase 2 | Pending |
| AUTH-05 | Phase 2 | Pending |
| AUTH-06 | Phase 2 | Pending |
| AUTH-07 | Phase 2 | Pending |
| AUTH-08 | Phase 2 | Pending |
| USER-01 | Phase 2 | Pending |
| USER-02 | Phase 2 | Pending |
| USER-03 | Phase 2 | Pending |
| USER-04 | Phase 2 | Pending |
| USER-05 | Phase 2 | Pending |
| COMM-01 | Phase 2 | Pending |
| COMM-02 | Phase 2 | Pending |
| COMM-03 | Phase 2 | Pending |
| COMM-04 | Phase 2 | Pending |
| COMM-05 | Phase 2 | Pending |
| COMM-06 | Phase 2 | Pending |
| COMM-07 | Phase 2 | Pending |
| POST-01 | Phase 3 | Pending |
| POST-02 | Phase 3 | Pending |
| POST-03 | Phase 3 | Pending |
| POST-04 | Phase 3 | Pending |
| POST-05 | Phase 3 | Pending |
| POST-06 | Phase 3 | Pending |
| POST-07 | Phase 3 | Pending |
| POST-08 | Phase 3 | Pending |
| POST-09 | Phase 3 | Pending |
| POST-10 | Phase 3 | Pending |
| CMNT-01 | Phase 4 | Pending |
| CMNT-02 | Phase 4 | Pending |
| CMNT-03 | Phase 4 | Pending |
| CMNT-04 | Phase 4 | Pending |
| CMNT-05 | Phase 4 | Pending |
| CMNT-06 | Phase 4 | Pending |
| VOTE-01 | Phase 3 | Pending |
| VOTE-02 | Phase 3 | Pending |
| VOTE-03 | Phase 3 | Pending |
| VOTE-04 | Phase 3 | Pending |
| VOTE-05 | Phase 3 | Pending |
| SRCH-01 | Phase 5 | Pending |
| SRCH-02 | Phase 5 | Pending |
| SRCH-03 | Phase 5 | Pending |
| SRCH-04 | Phase 5 | Pending |
| NOTF-01 | Phase 5 | Pending |
| NOTF-02 | Phase 5 | Pending |
| NOTF-03 | Phase 5 | Pending |
| NOTF-04 | Phase 5 | Pending |
| NOTF-05 | Phase 5 | Pending |
| NOTF-06 | Phase 5 | Pending |
| MDIA-01 | Phase 5 | Pending |
| MDIA-02 | Phase 5 | Pending |
| MDIA-03 | Phase 5 | Pending |
| MDIA-04 | Phase 5 | Pending |
| MOD-01 | Phase 6 | Pending |
| MOD-02 | Phase 6 | Pending |
| MOD-03 | Phase 6 | Pending |
| MOD-04 | Phase 6 | Pending |
| MOD-05 | Phase 6 | Pending |
| MOD-06 | Phase 6 | Pending |
| SPAM-01 | Phase 6 | Pending |
| SPAM-02 | Phase 6 | Pending |
| SPAM-03 | Phase 6 | Pending |
| SPAM-04 | Phase 6 | Pending |
| RATE-01 | Phase 2 | Pending |
| RATE-02 | Phase 2 | Pending |
| RATE-03 | Phase 2 | Pending |
| RATE-04 | Phase 2 | Pending |
| INFRA-01 | Phase 1 | Pending |
| INFRA-02 | Phase 7 | Pending |
| INFRA-03 | Phase 7 | Pending |
| INFRA-04 | Phase 7 | Pending |
| INFRA-05 | Phase 7 | Pending |
| INFRA-06 | Phase 7 | Pending |
| FEND-01 | Phase 1 | Complete |
| FEND-02 | Phase 1 | Pending |
| FEND-03 | Phase 1 | Complete |
| FEND-04 | Phase 7 | Pending |

**Coverage:**
- v1.0 requirements: 79 total
- Mapped to phases: 79
- Unmapped: 0

---
*Requirements defined: 2026-03-02*
*Last updated: 2026-03-02 after roadmap revision (progressive frontend)*
