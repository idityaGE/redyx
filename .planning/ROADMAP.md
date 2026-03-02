# Roadmap: Redyx

## Overview

Redyx is built in 7 phases following strict dependency order, with frontend pages and components built progressively alongside each backend phase. Phase 1 establishes shared infrastructure AND the Astro+Svelte project with responsive layout shell. Phases 2-6 each deliver backend services AND their corresponding frontend pages/components. Phase 7 is purely deployment and observability. The core discussion loop (browse → read → vote → comment → reply) is complete — both backend and frontend — by Phase 4. Everything after is enhancement and production readiness.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation + Frontend Shell** - Proto definitions, shared platform libraries, Envoy gateway, Docker Compose, Astro+Svelte project init with responsive layout shell ✓ (2026-03-02)
- [ ] **Phase 2: Auth + User + Community (Full Stack)** - Identity services + registration/login/profile/community frontend pages
- [ ] **Phase 3: Posts + Voting + Feeds (Full Stack)** - Content creation, voting, feed algorithms + post/feed/voting frontend components
- [ ] **Phase 4: Comments (Full Stack)** - ScyllaDB threaded discussion + comment tree frontend with lazy-loading UI
- [ ] **Phase 5: Search + Notifications + Media (Full Stack)** - Engagement services + search bar, notification panel, media upload frontend
- [ ] **Phase 6: Moderation + Spam (Full Stack)** - Moderation tools + mod dashboard, report UI, spam feedback frontend
- [ ] **Phase 7: Deployment + Observability** - Kubernetes deployment, monitoring stack, performance optimization

## Phase Details

### Phase 1: Foundation + Frontend Shell
**Goal**: Every service can be scaffolded from shared libraries with consistent gRPC patterns, the Envoy gateway transcodes REST to gRPC correctly, and the Astro+Svelte frontend project is initialized with a responsive layout shell
**Depends on**: Nothing (first phase)
**Requirements**: INFRA-01, FEND-01, FEND-02, FEND-03
**Success Criteria** (what must be TRUE):
  1. A skeleton gRPC service can be created using shared platform libraries (grpcserver bootstrap, middleware, config, database helpers) and responds to health checks
  2. Proto definitions compile with `buf` and generate Go code + Envoy descriptor set from a single `make proto` command
  3. Envoy transcodes a REST JSON request to gRPC and returns a correct JSON response for at least one test RPC
  4. Docker Compose brings up all infrastructure services (PostgreSQL, Redis, Envoy) and the skeleton service connects to them
  5. Astro SSR project with Svelte integration is initialized, builds, and serves a layout shell (header, sidebar, content area, footer) that is responsive across desktop, tablet, and mobile viewports
**Plans**: 3 plans

Plans:
- [ ] 01-01-PLAN.md — Proto definitions, buf config, Makefile, Go code generation + Envoy descriptor
- [ ] 01-02-PLAN.md — Astro SSR + Svelte frontend project with responsive terminal-aesthetic layout shell
- [ ] 01-03-PLAN.md — Shared Go platform libraries, skeleton gRPC service, Docker Compose, Envoy transcoding

### Phase 2: Auth + User + Community (Full Stack)
**Goal**: Users can create accounts, log in, manage profiles, and create/join communities — with working frontend pages for all auth flows, profile views, and community management
**Depends on**: Phase 1
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-06, AUTH-07, AUTH-08, USER-01, USER-02, USER-03, USER-04, USER-05, COMM-01, COMM-02, COMM-03, COMM-04, COMM-05, COMM-06, COMM-07, RATE-01, RATE-02, RATE-03, RATE-04
**Success Criteria** (what must be TRUE):
  1. User can register with email/password, receive OTP, verify account, and log in — staying authenticated across browser sessions via JWT refresh tokens
  2. User can register/login via Google OAuth, choose a username, and no private data (email, auth method) is exposed through any API
  3. User can view any profile (username, karma, cake day, post/comment history) and edit their own display name, bio, and avatar
  4. User can create a community, set visibility (public/restricted/private), assign moderators, and other users can join/leave
  5. API requests are rate-limited per user tier (anonymous/authenticated/trusted) with action-specific limits enforced, returning 429 with Retry-After when exceeded
  6. Frontend pages exist for: registration, login, OAuth callback, OTP verification, password reset, user profile (view/edit), community creation, community settings, and community detail (member list, rules, description)
**Plans**: 10 plans

Plans:
- [x] 02-01-PLAN.md — Platform infrastructure: auth interceptor, rate limiter, config, DB migrations, Docker init ✓
- [ ] 02-02-PLAN.md — Auth gRPC service: Register, Login, OTP, OAuth, JWT, Refresh, Logout, Reset
- [ ] 02-03-PLAN.md — User gRPC service: GetProfile, UpdateProfile, DeleteAccount, stub post/comment history
- [ ] 02-04-PLAN.md — Community gRPC service: CRUD, membership, moderator roles, Redis cache
- [x] 02-05-PLAN.md — Docker Compose + Envoy: add 3 services, route config, transcoder registration ✓
- [ ] 02-06-PLAN.md — Frontend foundation: API client, auth store, Header/Sidebar Svelte conversion
- [x] 02-07-PLAN.md — Frontend auth pages: register, login, verify, choose-username, reset-password ✓
- [x] 02-08-PLAN.md — Frontend profile page: status line, tabs, inline editing, account deletion ✓
- [x] 02-09-PLAN.md — Frontend community pages: browse, create, detail with sidebar, settings ✓
- [x] 02-10-PLAN.md — End-to-end verification: API curl tests + human verification checkpoint ✓

### Phase 3: Posts + Voting + Feeds (Full Stack)
**Goal**: Users can create posts, vote on content, and browse community and home feeds — with frontend pages for post creation, feed browsing, voting interactions, and post detail views
**Depends on**: Phase 2
**Requirements**: POST-01, POST-02, POST-03, POST-04, POST-05, POST-06, POST-07, POST-08, POST-09, POST-10, VOTE-01, VOTE-02, VOTE-03, VOTE-04, VOTE-05
**Success Criteria** (what must be TRUE):
  1. User can create text, link, and media posts in a community — posts display title, author, community, timestamp, vote score, and comment count
  2. User can upvote/downvote any post, change or remove their vote, and see the updated score within 500ms — with author karma updating asynchronously via Kafka
  3. User can browse a community feed sorted by Hot, New, Top (with time filters), and Rising — and browse a home feed aggregating posts from all joined communities
  4. User can edit/delete own posts, post anonymously as [anonymous], and save posts to a private bookmarks list
  5. Posts are stored on shards determined by consistent hashing of community_id, and the system handles cross-shard queries for home feed aggregation
  6. Frontend pages exist for: post creation form (text/link/media tabs), community feed with sort controls, home feed, post detail view, user's saved posts list — with optimistic vote UI updates (score changes instantly on click)
**Plans**: TBD

Plans:
- [ ] 03-01: TBD
- [ ] 03-02: TBD
- [ ] 03-03: TBD

### Phase 4: Comments (Full Stack)
**Goal**: Users can have threaded discussions on posts — with a frontend comment tree component supporting nested replies, sorting, and lazy-loading of deep threads
**Depends on**: Phase 3
**Requirements**: CMNT-01, CMNT-02, CMNT-03, CMNT-04, CMNT-05, CMNT-06
**Success Criteria** (what must be TRUE):
  1. User can comment on a post and reply to existing comments, forming nested threads stored in ScyllaDB with materialized path ordering
  2. Comments display author, timestamp, vote score, and reply count — and are sortable by Best (Wilson score), Top, New, and Controversial
  3. Deleted comments show [deleted] but preserve thread structure (children remain visible), and deep threads lazy-load on demand (top 2-3 levels shown initially)
  4. Frontend comment tree component renders nested threads with indentation, collapse/expand controls, sort selector, inline reply form, and "load more replies" button for deep threads
**Plans**: TBD

Plans:
- [ ] 04-01: TBD
- [ ] 04-02: TBD

### Phase 5: Search + Notifications + Media (Full Stack)
**Goal**: Users can search content, receive real-time notifications, and upload media — with frontend components for search, notification panel, and media upload
**Depends on**: Phase 4
**Requirements**: SRCH-01, SRCH-02, SRCH-03, SRCH-04, NOTF-01, NOTF-02, NOTF-03, NOTF-04, NOTF-05, NOTF-06, MDIA-01, MDIA-02, MDIA-03, MDIA-04
**Success Criteria** (what must be TRUE):
  1. User can search posts by title/body globally or within a community, with results ranked by relevance/recency/score and returned within 300ms
  2. Community name autocomplete works in the search bar after typing 2+ characters
  3. User receives real-time WebSocket notification within 1 second when someone replies to their post/comment or mentions them with u/username
  4. Offline notifications are stored and delivered on next connection, and user can mark notifications as read and configure notification preferences
  5. User can upload images (JPEG/PNG/GIF/WebP, 20MB max) and videos (100MB max) when creating a post, with thumbnails generated and media served via CDN
  6. Frontend includes: search bar with autocomplete dropdown, search results page, notification bell with unread count badge, notification dropdown/panel with mark-as-read, notification preferences page, and media upload component with drag-and-drop and progress indicator
**Plans**: TBD

Plans:
- [ ] 05-01: TBD
- [ ] 05-02: TBD
- [ ] 05-03: TBD

### Phase 6: Moderation + Spam (Full Stack)
**Goal**: Communities have moderation tools and the platform detects/prevents spam — with frontend moderation dashboard, report dialogs, and mod log views
**Depends on**: Phase 5
**Requirements**: MOD-01, MOD-02, MOD-03, MOD-04, MOD-05, MOD-06, SPAM-01, SPAM-02, SPAM-03, SPAM-04
**Success Criteria** (what must be TRUE):
  1. Moderators can remove posts/comments, ban users (temporary or permanent with reason), and pin up to 2 posts — with all actions recorded in a mod log
  2. Users can report posts/comments with a reason, and moderators can view a queue of flagged content sorted by report count
  3. Content is checked against keyword blocklist and known-bad URL list before publishing, and duplicate content from the same user is rejected
  4. Async behavior analysis via Kafka detects rapid posting and link spam patterns across communities
  5. Frontend includes: report dialog on posts/comments, mod dashboard with report queue and action buttons, mod log page, ban management UI, and pinned post controls — all accessible only to users with moderator role
**Plans**: TBD

Plans:
- [ ] 06-01: TBD
- [ ] 06-02: TBD

### Phase 7: Deployment + Observability
**Goal**: The platform runs in Kubernetes with full observability, and frontend performance is optimized to meet load time targets
**Depends on**: Phase 6
**Requirements**: FEND-04, INFRA-02, INFRA-03, INFRA-04, INFRA-05, INFRA-06
**Success Criteria** (what must be TRUE):
  1. Kubernetes deployment runs all services with HPA, readiness/liveness probes, and namespace isolation
  2. Prometheus collects metrics from every service, Grafana displays per-service and global dashboards, Loki aggregates structured logs, and Jaeger visualizes distributed traces across service boundaries
  3. Page loads complete under 2 seconds on fast 4G connection — verified with Lighthouse or equivalent performance audit
**Plans**: TBD

Plans:
- [ ] 07-01: TBD
- [ ] 07-02: TBD
- [ ] 07-03: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation + Frontend Shell | 3/3 | ✓ Complete | 2026-03-02 |
| 2. Auth + User + Community (Full Stack) | 9/10 | In Progress | - |
| 3. Posts + Voting + Feeds (Full Stack) | 0/? | Not started | - |
| 4. Comments (Full Stack) | 0/? | Not started | - |
| 5. Search + Notifications + Media (Full Stack) | 0/? | Not started | - |
| 6. Moderation + Spam (Full Stack) | 0/? | Not started | - |
| 7. Deployment + Observability | 0/? | Not started | - |
