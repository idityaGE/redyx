# Requirements: Redyx

**Defined:** 2026-03-02
**Core Value:** Users can anonymously create communities, post content, and have threaded discussions — with minimal personal data collected and maximum privacy preserved.

## v1.0 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Authentication (AUTH)

- [x] **AUTH-01**: User can register with email, username, and password (argon2id hashed)
- [x] **AUTH-02**: User receives 6-digit OTP via email to verify account (5min TTL, stored in Redis)
- [x] **AUTH-03**: User can register via Google OAuth and choose a username
- [x] **AUTH-04**: User can log in with email/password or Google OAuth
- [x] **AUTH-05**: System issues JWT access token (15min) and refresh token (7 days)
- [x] **AUTH-06**: User can log out, invalidating the refresh token
- [x] **AUTH-07**: User can reset password via email link with token
- [x] **AUTH-08**: Email and auth method are never exposed to other users through any API endpoint

### User Profiles (USER)

- [x] **USER-01**: User has public profile showing username, karma, and cake day
- [x] **USER-02**: Profile displays paginated post and comment history
- [x] **USER-03**: Karma calculated from total upvotes received on posts and comments
- [x] **USER-04**: User can update display name, bio (500 chars), and avatar
- [x] **USER-05**: User can delete account, wiping all PII and replacing content with [deleted]

### Communities (COMM)

- [x] **COMM-01**: User can create community with unique, immutable name (alphanumeric + underscores, 3-21 chars)
- [x] **COMM-02**: Community has description (markdown), rules (ordered list), banner, and icon
- [x] **COMM-03**: Community visibility: public, restricted (view-only for non-approved), or private (invite-only)
- [x] **COMM-04**: User can join and leave communities (updates member count)
- [x] **COMM-05**: Creator is automatically assigned owner and moderator roles
- [x] **COMM-06**: Owner can assign and revoke moderator roles
- [x] **COMM-07**: Community metadata cached in Redis (1hr TTL, invalidate on update)

### Posts & Feeds (POST)

- [x] **POST-01**: User can create text post (title max 300 chars + markdown body max 40K chars) in a community ✓
- [x] **POST-02**: User can create link post (title + validated URL) ✓
- [x] **POST-03**: User can create media post (title + image/video upload via Media Service) ✓ (stubbed UNIMPLEMENTED)
- [x] **POST-04**: Post displays title, author, community, timestamp, vote score, and comment count ✓
- [x] **POST-05**: User can edit and delete own posts (edited shows timestamp, deleted shows [deleted]) ✓
- [x] **POST-06**: Community feed sortable by Hot (Lemmy algorithm), New, Top (hour/day/week/month/year/all), Rising ✓
- [x] **POST-07**: Home feed aggregates posts from all joined communities with same sorting options ✓
- [x] **POST-08**: User can post anonymously as [anonymous] within a community (mods see real author) ✓
- [x] **POST-09**: User can save/bookmark posts to a private "Saved" list ✓
- [x] **POST-10**: Posts stored on shard determined by consistent hashing of community_id ✓

### Comments (CMNT)

- [x] **CMNT-01**: User can comment on posts (markdown, max 10K chars, stored in ScyllaDB) ✓
- [x] **CMNT-02**: User can reply to comments, forming nested threads (materialized path for tree ordering) ✓
- [x] **CMNT-03**: Comments display author, timestamp, vote score, and reply count ✓
- [x] **CMNT-04**: Comments sortable by Best (Wilson score confidence interval), Top, New ✓ (Controversial deferred per user decision)
- [x] **CMNT-05**: Deleted comments show [deleted] but thread structure preserved (children remain visible) ✓
- [x] **CMNT-06**: Deep threads lazy-loaded (top 2-3 levels shown initially, rest on demand) ✓

### Voting (VOTE)

- [x] **VOTE-01**: User can upvote or downvote any post or comment ✓
- [x] **VOTE-02**: One vote per user per item; can change direction or remove (enforced in Redis) ✓
- [x] **VOTE-03**: Net score (upvotes minus downvotes) displayed on each item, updates within 500ms ✓
- [x] **VOTE-04**: Votes update author's karma asynchronously via Kafka (set-based idempotent processing) ✓
- [x] **VOTE-05**: Vote endpoints are idempotent (duplicate requests are safe) ✓

### Search (SRCH)

- [x] **SRCH-01**: User can search posts by title and body text via Meilisearch (results within 300ms)
- [ ] **SRCH-02**: User can search within a specific community or globally
- [ ] **SRCH-03**: Community name autocomplete in search bar (prefix-based, cached in Redis, triggers after 2+ chars)
- [ ] **SRCH-04**: Search results ranked by relevance, recency, and vote score

### Notifications (NOTF)

- [x] **NOTF-01**: User receives notification when someone replies to their post or comment
- [x] **NOTF-02**: User receives notification when mentioned with u/username
- [x] **NOTF-03**: Notifications delivered in real time via WebSocket (within 1 second)
- [x] **NOTF-04**: Offline notifications stored in PostgreSQL and delivered on next WebSocket connection
- [x] **NOTF-05**: User can mark individual or all notifications as read
- [x] **NOTF-06**: User can configure notification preferences (mute communities, mute reply types)

### Media (MDIA)

- [x] **MDIA-01**: User can upload images and videos when creating a post
- [x] **MDIA-02**: Uploaded files validated for type (JPEG, PNG, GIF, WebP) and size (20MB image, 100MB video)
- [x] **MDIA-03**: Thumbnails generated for image uploads (max 320px wide)
- [x] **MDIA-04**: Media stored in AWS S3 and served through CloudFront CDN

### Moderation (MOD)

- [ ] **MOD-01**: Moderators can remove posts and comments from their community (hidden from regular users)
- [ ] **MOD-02**: Moderators can ban users from community (temporary or permanent, with duration and reason)
- [ ] **MOD-03**: Moderators can pin up to 2 posts in their community (displayed at top of feed)
- [ ] **MOD-04**: All moderation actions recorded in mod log (who, what, when, reason)
- [ ] **MOD-05**: Moderators can view queue of reported/flagged content (sorted by report count)
- [ ] **MOD-06**: Users can report posts and comments with a reason

### Rate Limiting (RATE)

- [x] **RATE-01**: API gateway enforces per-user request rate limits via Redis token bucket
- [x] **RATE-02**: Tiered limits: anonymous (10 req/min), authenticated (100 req/min), trusted (300 req/min)
- [x] **RATE-03**: Action-specific limits: 5 posts/hour, 30 comments/hour, 60 votes/min, 1 community/day
- [x] **RATE-04**: Exceeding limit returns HTTP 429 with Retry-After header

### Spam Detection (SPAM)

- [ ] **SPAM-01**: Content checked against keyword blocklist before publishing
- [ ] **SPAM-02**: URLs in posts checked against known-bad domain list
- [ ] **SPAM-03**: Duplicate content from same user rejected (content hash comparison)
- [ ] **SPAM-04**: Async behavior analysis via Kafka detects rapid posting and link spam across communities

### Infrastructure (INFRA)

- [x] **INFRA-01**: Docker Compose configuration for local development with all services and data stores
- [ ] **INFRA-02**: Kubernetes deployment with HPA, readiness/liveness probes, namespace isolation
- [ ] **INFRA-03**: Prometheus metrics collection from every Go service via /metrics endpoint
- [ ] **INFRA-04**: Grafana dashboards per service and global overview dashboard
- [ ] **INFRA-05**: Loki centralized log aggregation with structured JSON logs from all services
- [ ] **INFRA-06**: OpenTelemetry distributed traces across service boundaries with Jaeger visualization

### Frontend (FEND)

- [x] **FEND-01**: Astro SSR frontend with Svelte interactive islands for dynamic components
- [x] **FEND-02**: Envoy API gateway with REST-to-gRPC transcoding via proto descriptor set
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
| AUTH-01 | Phase 2 | Complete |
| AUTH-02 | Phase 2 | Complete |
| AUTH-03 | Phase 2 | Complete |
| AUTH-04 | Phase 2 | Complete |
| AUTH-05 | Phase 2 | Complete |
| AUTH-06 | Phase 2 | Complete |
| AUTH-07 | Phase 2 | Complete |
| AUTH-08 | Phase 2 | Complete |
| USER-01 | Phase 2 | Complete |
| USER-02 | Phase 2 | Complete |
| USER-03 | Phase 2 | Complete |
| USER-04 | Phase 2 | Complete |
| USER-05 | Phase 2 | Complete |
| COMM-01 | Phase 2 | Complete |
| COMM-02 | Phase 2 | Complete |
| COMM-03 | Phase 2 | Complete |
| COMM-04 | Phase 2 | Complete |
| COMM-05 | Phase 2 | Complete |
| COMM-06 | Phase 2 | Complete |
| COMM-07 | Phase 2 | Complete |
| POST-01 | Phase 3 | Complete |
| POST-02 | Phase 3 | Complete |
| POST-03 | Phase 3 | Complete |
| POST-04 | Phase 3 | Complete |
| POST-05 | Phase 3 | Complete |
| POST-06 | Phase 3 | Complete |
| POST-07 | Phase 3 | Complete |
| POST-08 | Phase 3 | Complete |
| POST-09 | Phase 3 | Complete |
| POST-10 | Phase 3 | Complete |
| CMNT-01 | Phase 4 | ✓ Complete |
| CMNT-02 | Phase 4 | ✓ Complete |
| CMNT-03 | Phase 4 | ✓ Complete |
| CMNT-04 | Phase 4 | ✓ Complete |
| CMNT-05 | Phase 4 | ✓ Complete |
| CMNT-06 | Phase 4 | ✓ Complete |
| VOTE-01 | Phase 3, Plan 02 | ✓ Complete |
| VOTE-02 | Phase 3, Plan 02 | ✓ Complete |
| VOTE-03 | Phase 3, Plan 02 | ✓ Complete |
| VOTE-04 | Phase 3, Plan 02 | ✓ Complete |
| VOTE-05 | Phase 3, Plan 02 | ✓ Complete |
| SRCH-01 | Phase 5 | Complete |
| SRCH-02 | Phase 5 | Pending |
| SRCH-03 | Phase 5 | Pending |
| SRCH-04 | Phase 5 | Pending |
| NOTF-01 | Phase 5 | Complete |
| NOTF-02 | Phase 5 | Complete |
| NOTF-03 | Phase 5 | Complete |
| NOTF-04 | Phase 5 | Complete |
| NOTF-05 | Phase 5 | Complete |
| NOTF-06 | Phase 5 | Complete |
| MDIA-01 | Phase 5 | Complete |
| MDIA-02 | Phase 5 | Complete |
| MDIA-03 | Phase 5 | Complete |
| MDIA-04 | Phase 5 | Complete |
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
| RATE-01 | Phase 2 | Complete |
| RATE-02 | Phase 2 | Complete |
| RATE-03 | Phase 2 | Complete |
| RATE-04 | Phase 2 | Complete |
| INFRA-01 | Phase 1 | Complete |
| INFRA-02 | Phase 7 | Pending |
| INFRA-03 | Phase 7 | Pending |
| INFRA-04 | Phase 7 | Pending |
| INFRA-05 | Phase 7 | Pending |
| INFRA-06 | Phase 7 | Pending |
| FEND-01 | Phase 1 | Complete |
| FEND-02 | Phase 1 | Complete |
| FEND-03 | Phase 1 | Complete |
| FEND-04 | Phase 7 | Pending |

**Coverage:**
- v1.0 requirements: 79 total
- Mapped to phases: 79
- Unmapped: 0

---
*Requirements defined: 2026-03-02*
*Last updated: 2026-03-02 after roadmap revision (progressive frontend)*
