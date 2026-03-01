# Feature Landscape

**Domain:** Anonymous community-driven discussion platform (Reddit clone)
**Researched:** 2026-03-02
**Overall confidence:** HIGH — grounded in existing SRS, real Reddit/Lemmy UX patterns, and established ranking algorithm research

---

## Table Stakes

Features users expect from any Reddit-like platform. Missing any of these = users leave immediately because the product feels broken or unfinished.

### Authentication & Account

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Email/password registration | Baseline auth on every platform | Low | argon2id hashing is correct choice. Users expect immediate feedback on duplicate email/username |
| Email verification (OTP) | Prevents throwaway spam accounts | Low | 6-digit, 5min TTL, Redis. Standard pattern. Must allow resend |
| Social login (Google OAuth) | Users expect at least one social login option | Medium | Google is sufficient for v1. OAuth code exchange + username selection flow is well-established |
| Login/logout with JWT tokens | Stateless auth is expected behavior | Low | 15min access / 7day refresh is standard. Users expect "remember me" to work |
| Password reset flow | Every platform has this; missing = locked out users | Low | **NOT in SRS — add it.** Users will forget passwords. Email-based reset link with token is table stakes |
| Username-only public identity | Reddit established this pattern; users expect pseudonymity | Low | Never expose email, auth method, or IP. This is Redyx's core value prop |

### User Profiles

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Public profile page (username, karma, cake day) | Reddit users check profiles constantly to assess commenter credibility | Low | Paginated post/comment history is critical — not just a vanity page |
| Karma score | Reputation signal users rely on to judge trustworthiness | Low | Sum of upvotes received. Must be visible on every post/comment author |
| Account deletion with data wipe | Privacy/legal requirement (GDPR-style). Users expect this | Medium | Replace content with `[deleted]`, anonymize votes. The 30-day username cooldown in the SRS is a good touch |

### Communities

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Create communities with unique names | Core organizational unit. Reddit's `r/` prefix is universally understood | Low | Immutable names are correct. Validate: alphanumeric + underscores, 3-21 chars |
| Join/leave communities | Subscription model drives the home feed | Low | Must update member count atomically. Private communities need invite flow |
| Community description and rules | Users need to understand what a community is about before joining | Low | Markdown support for description. Rules as ordered list |
| Public/restricted/private visibility | Community owners expect control over who can view and participate | Medium | Restricted = view-only for non-approved users. Private = invite-only. Must enforce at post creation, not just display |
| Community feed with sorting | Browsing a single community's posts is the primary content discovery pattern | Medium | Hot/New/Top/Rising. Pinned posts at top regardless of sort |
| Member roles (owner, moderator, member) | Community governance is expected | Low | Hierarchical: owner > mod > member. Owner appoints mods |

### Posts

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Text posts (title + markdown body) | The fundamental content unit | Medium | Max 300 char title, 40K char body. Markdown rendering must handle edge cases (XSS prevention) |
| Link posts (title + URL) | Reddit's origin was link aggregation; users expect to share URLs | Low | URL validation (http/https only). Link preview is a differentiator, not table stakes |
| Post metadata display (author, community, timestamp, score, comment count) | Users scan metadata to decide what to read | Low | All of these must be visible in the feed card. Missing any one looks broken |
| Edit and delete own posts | Basic content ownership | Low | Show "edited" indicator with timestamp. Deletion replaces content with `[deleted]` |
| Community feed (sorted: Hot, New, Top with time filters) | The primary way users consume content in a community | Medium | Time filters on Top (hour/day/week/month/year/all) are expected, not optional |
| Home feed (aggregated from joined communities) | The "front page" experience. Without this, there's no reason to join communities | High | Cross-shard aggregation is the hard part. Cursor-based pagination required |
| Auto-upvote own post (score starts at 1) | Reddit does this; users expect it. Starting at 0 looks wrong | Low | Automatic on post creation |

### Comments

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Comment on posts (markdown body) | The entire discussion model depends on this | Medium | Max 10K chars. Store in ScyllaDB partitioned by post_id |
| Nested/threaded replies (tree structure) | **This is THE feature** that defines Reddit vs. flat forums. Non-negotiable | High | Materialized path for ordering. Visual indentation communicates hierarchy. Must support at least 5+ levels |
| Deleted comments preserve thread structure | Reddit shows `[deleted]` but keeps children visible. Users expect this | Low | Critical for conversation continuity. If all children also deleted, collapse subtree |
| Lazy-load deep threads ("load more replies") | Prevents massive page loads on popular posts | Medium | Show top 2-3 levels, "load more" fetches children. Cursor-based pagination within thread levels |
| Comment sorting (Best, Top, New, Controversial) | Users switch sort modes constantly. Missing = feels limited | Medium | "Best" is the Wilson score confidence interval (not simple upvotes-downvotes). "Controversial" = high total votes, close to 50/50 split |

### Voting

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Upvote/downvote on posts and comments | The core engagement mechanic. Without this, content ranking is impossible | Medium | One vote per user per item. Idempotent. Toggle behavior: click same direction = remove vote |
| Net score display (upvotes - downvotes) | Users need the signal to assess content quality | Low | Must update within 500ms of voting for perceived responsiveness |
| User's own vote direction highlighted | Reddit highlights the arrow you clicked. Missing = confusing UX | Low | Up arrow orange/highlighted, down arrow blue/highlighted, neutral = gray |
| Vote changes and removal | Users change their minds. Must allow switching from up to down seamlessly | Low | Score adjustment: up->down = -2, down->up = +2, remove = +/-1 |

### Moderation (Basic)

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Remove posts/comments | Moderators must be able to enforce rules | Low | Hidden from regular users, visible to mods. Different from author deletion |
| Ban users from community (temp/permanent) | Standard community governance | Medium | Must check ban on every write operation. Expired bans cleaned by background job |
| Mod log (transparency) | Reddit has public mod logs. Users expect accountability | Low | Log every action: who, what, when, reason |

### Rate Limiting

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Per-user API rate limits | Without this, bots destroy the platform on day one | Medium | Token bucket at gateway level. 429 + Retry-After header |
| Action-specific limits (posts/hour, comments/hour, votes/min) | Prevents content spam even from authenticated users | Medium | Sliding window counters in Redis. Must be tuned post-launch |
| New account restrictions | Standard anti-spam: new accounts can't post immediately | Low | <24h can't post, <1h can't comment. Harsh but necessary |

---

## Differentiators

Features that set Redyx apart. Not expected by default, but valued. These are what make users say "this is better than Reddit."

### Anonymity-First Design (Redyx Core Value)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Anonymous posting mode (`[anonymous]` author) | **THE differentiator.** Reddit doesn't have this natively. Lets people share sensitive content without reputation risk | Medium | Mods see real author. Anonymous posts follow same rules/rate limits. Must not leak identity through timing or metadata |
| Zero PII exposure through any API | Privacy by design, not privacy by policy | Low | Audit every endpoint. Emails encrypted at rest (AES-256). IPs never in app DB. This must be verifiable |
| Minimal data collection (no phone, no real name) | Contrast to Reddit requiring email. Redyx collects less | Low | Marketing angle: "we know less about you than anyone" |
| True account deletion (PII purge) | Reddit's account deletion is incomplete. Redyx does it right | Medium | Delete all PII, anonymize votes, replace content with `[deleted]`. Username available after 30 days |

### Real-Time Features

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| WebSocket notifications (reply, mention) | Real-time delivery is delightful. Polling feels sluggish | High | Redis-backed connection registry. Pod-to-pod routing. Offline storage + delivery on reconnect |
| `u/username` mentions with notifications | Reddit has this; smaller platforms often don't | Low | Parse content for `u/` prefix, trigger notification to mentioned user |
| Notification preferences (mute communities, mute replies) | Power users need this immediately on an active platform | Low | Check preferences before sending, not after |

### Search & Discovery

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Full-text search (posts by title + body) | Reddit's search was famously bad for years. Good search = major differentiator | Medium | Meilisearch handles this well. Index via Kafka events. <300ms response |
| Community autocomplete in search bar | Quick navigation is a UX win. Reduces friction to finding communities | Low | Prefix search cached in Redis. Trigger after 2+ chars |
| Search within specific community | Scoped search is extremely useful for large communities | Low | Filter parameter on search query. Meilisearch supports this natively |

### Content Richness

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Media posts (image/video upload) | Visual content drives engagement | Medium | Validation, ClamAV scan, S3 storage, thumbnail generation, CDN delivery. 20MB image / 100MB video limits |
| Pin posts (up to 2 per community) | Community moderators need to highlight announcements | Low | Display at top of feed regardless of sort. Visually marked |
| Save/bookmark posts | Convenience feature that power users love | Low | Private to user. Simple toggle. Dedicated "Saved" page |

### Moderation (Advanced)

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Mod queue (reported/flagged content) | Efficient moderation at scale | Medium | Sorted by report count. Shows content snippet, author, report reasons |
| Content reporting by users | The primary way communities self-moderate | Low | Flag icon on posts/comments. Multiple report reasons |
| Pre-publish spam filtering (keyword blocklist, URL check, duplicate detection) | Keeps spam out before users see it. Proactive vs. reactive | Medium | Synchronous checks before save. Keyword regex, domain blocklist, content hash for duplicates |
| Post-publish behavior analysis | Catches patterns that pre-publish misses (e.g., same link across 5 communities) | High | Async via Kafka. Behavior scoring in Redis. Flags for mod review |

### Feed Algorithms

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Hot ranking (time-decay + score) | The default sort that makes Reddit work. Based on: `log(score) + age_factor` | Medium | Reddit's original algorithm: `log10(max(abs(score),1)) + sign(score) * seconds_since_epoch / 45000`. Lemmy improved it with `ScaleFactor * log(Max(1, 3+Score)) / (Time + 2)^Gravity`. Use Lemmy's approach — it handles score=0 better |
| Rising sort (gaining upvotes quickly) | Surfaces emerging content before it hits Hot | Medium | Calculate velocity: votes per time unit. Useful for active communities |
| Top sort with time filters | Standard expectation for content discovery | Low | Filter by hour/day/week/month/year/all-time. Simple query |
| Wilson score for "Best" comment sorting | Reddit adopted this in 2009 (based on Evan Miller's article). It's the correct way to rank comments | Medium | Uses lower bound of Wilson score confidence interval. Balances proportion of positive ratings with uncertainty from small sample size. Avoids the "first comment wins" problem of simple scoring |

---

## Anti-Features

Features that seem good but actively hurt the product. Explicitly do NOT build these.

| Anti-Feature | Why It Seems Good | Why It Hurts | What to Do Instead |
|--------------|-------------------|-------------|-------------------|
| Direct messaging / chat | "Reddit has it" | Massive scope creep. Requires abuse prevention, media scanning, blocking, reporting — essentially a second product. Reddit's chat is widely disliked and underused. Attracts harassment | Out of scope (per SRS). Focus on public threaded discussion. Users who want DMs can share contact info voluntarily |
| Flair/tag system | Helps organize posts within communities | Medium complexity for low value in v1. Requires per-community configuration UI, tag management, tag-based filtering. Reddit's flair system is confusing to new users | Defer to v1.1+. Communities function fine without it initially |
| Multiple OAuth providers (Apple, GitHub, Twitter) | "More login options = more users" | Each provider adds maintenance burden, edge cases, and security surface. Google covers >80% of social login needs | Google OAuth only for v1. Add others only if users specifically request them |
| Infinite scroll without pagination controls | "Modern UX is infinite scroll" | Kills browser performance on long feeds. Makes it impossible to find content you scrolled past. Breaks "back" button. Accessibility nightmare | Cursor-based pagination with "Load More" or numbered pages. User controls the pace |
| Realtime vote count updates on feed (WebSocket) | "Watch scores change live!" | Enormous WebSocket overhead broadcasting every vote to every viewer. Reddit doesn't do this — scores update on page refresh. The visual jitter is distracting | Update vote scores on user action only (their own votes). Refresh-based for others. WebSocket only for notifications |
| Email notifications for every reply | "Keep users engaged" | Spam. Users get annoyed, unsubscribe from everything, and leave. Reddit's email notifications are overwhelmingly negative | Notifications via WebSocket + in-app only for v1. Email only for account security (password reset, ban notice) |
| Vote manipulation detection (cluster analysis) | "Prevent gaming the system" | P3 for good reason: requires significant data volume before it's useful. False positives alienate legitimate users. Complex ML pipeline with no training data | Defer to v1.1+. Use simple heuristics: new account restrictions, rate limits on votes, duplicate detection |
| Shadow-banning | "Effective against spammers" | Ethically questionable. Creates trust issues if users discover it. Complex to implement correctly (must filter from all views without the user noticing). Reddit moved away from it for sitewide use | Defer to v1.1+ (per SRS P3). Use visible bans + content removal for v1 |
| User-to-user blocking | "Let users control their experience" | Seems simple but creates complex edge cases: what happens when blocked user replies to a thread you're in? What about mod actions? Does blocking a mod work? | Defer to v1.1+. Focus on community-level moderation and muting notification preferences for v1 |
| Crossposting | "Share content across communities" | Adds complexity to post ownership, moderation, and feed deduplication. Reddit's crossposting creates confusion about where discussion happens | Not in SRS. Don't add it. Users can manually post in multiple communities |
| Awards / coins / premium system | "Monetization and engagement" | Massive scope creep. Reddit's awards system was so complex they ripped it out and rebuilt it multiple times. Distracts from core discussion experience | Not in scope. Focus on the discussion platform being good |
| Custom CSS / themes per community | "Community identity" | Security nightmare (CSS injection). Maintenance burden. Inconsistent UX across communities. Reddit dropped custom CSS in their redesign | Community banners + icons are sufficient for identity. No custom styling |
| NSFW content tagging system | "Content control" | Requires content classification, user preference systems, age verification considerations, and per-community NSFW settings. Significant complexity | Defer entirely. Communities can use rules to self-moderate content |
| Push notifications (browser) | "Re-engage users" | Users hate browser notification prompts. The permission prompt has one of the lowest acceptance rates of any web feature. Creates negative first impression | Never prompt for push notifications. WebSocket in-app notifications only |

---

## Feature Dependencies

Critical path: features that block other features. Build order must respect these.

```
Auth Service (registration, login, JWT)
  └─> User Service (profiles, karma) — needs authenticated user_id
  └─> Community Service (create, join) — needs authenticated user_id
        └─> Post Service (create posts in community) — needs community_id + user_id
              └─> Comment Service (comment on posts) — needs post_id + user_id
              └─> Vote Service (vote on posts/comments) — needs target_id + user_id
                    └─> Karma updates (async via Kafka) — needs vote events
              └─> Search Service (index posts) — needs PostCreated events from Kafka
              └─> Feed algorithms (Hot/Top/New/Rising) — needs posts + vote scores
        └─> Moderation Service — needs community_id + user roles
              └─> Mod queue — needs content reporting
              └─> Ban enforcement — needs membership checks on write
        └─> Media Service — needs S3 infra + post creation flow
  └─> Notification Service — needs Kafka events from Post, Comment, Vote services
        └─> WebSocket delivery — needs Redis connection registry
  └─> Rate Limiting — needs Redis, runs at Envoy gateway level (can be built early)
  └─> Spam Detection — needs content creation events (pre-publish sync + post-publish async)
```

### Key dependency insights:

1. **Auth must come first.** Every other service depends on authenticated user identity.
2. **Communities before posts.** Posts belong to communities. Can't test posting without at least one community.
3. **Posts before comments, votes, search.** All three consume post events.
4. **Votes before meaningful feeds.** Hot/Top sorting requires vote scores. Without votes, only "New" sort works.
5. **Kafka infrastructure before async features.** Karma updates, search indexing, notifications, and spam analysis all consume Kafka events.
6. **Redis infrastructure before rate limiting and voting.** Both need Redis from day one.
7. **Rate limiting should be early.** Protects all other services during development and testing.
8. **Moderation and spam detection can be later.** They enhance the platform but aren't required for basic functionality.
9. **Notifications are a late-stage feature.** Depend on almost everything else being in place first.
10. **Media is independent.** Can be built in parallel with post/comment services, just needs S3.

---

## Ranking Algorithm Details

These are critical implementation details that determine whether feeds "feel right."

### Post Ranking: Hot Sort

Reddit's original hot algorithm (simplified):
```
hot_score = log10(max(|score|, 1)) * sign(score) + age_seconds / 45000
```
Where `age_seconds` = seconds since Reddit's epoch (Dec 8, 2005).

**Problem:** This gives an inherent advantage to newer posts regardless of quality, and makes the time component a monotonically increasing number. It works at Reddit's scale but has quirks.

**Recommendation:** Use Lemmy's improved version:
```
rank = scale_factor * log(max(1, 3 + score)) / (time_hours + 2) ^ gravity
```
Where `gravity = 1.8` (default decay rate).

- Adding 3 to score means content needs 3+ downvotes to appear penalized (new content gets a fair chance)
- Log scale means first 10 votes matter as much as next 100 (prevents snowball)
- Time decay means old content naturally drops off

### Comment Ranking: "Best" Sort

Use Wilson score confidence interval (lower bound):
```
wilson_score = (p_hat + z²/2n - z * sqrt((p_hat*(1-p_hat) + z²/4n) / n)) / (1 + z²/n)
```
Where `p_hat = positive_votes / total_votes`, `z = 1.96` (95% confidence), `n = total_votes`.

**Why Wilson:** Balances the proportion of positive ratings against the uncertainty of a small sample. A comment with 5 upvotes and 0 downvotes ranks appropriately against one with 100 upvotes and 20 downvotes.

**Reddit adopted this in 2009** and it's now the standard for "Best" comment sorting. Evan Miller's original analysis (evanmiller.org) is the authoritative source.

### "Controversial" Sort

```
controversial = (upvotes + downvotes) ^ (min(upvotes, downvotes) / max(upvotes, downvotes))
```
High total votes + close to 50/50 split = high controversy score.

---

## UX Patterns Users Expect

Drawn from actual Reddit behavior, not speculation.

### Feed UX
- **Default sort is Hot**, not New. New users who see chronological feeds get confused by irrelevant old content at top
- **Feed cards show:** title, author, community name, timestamp (relative: "3h ago"), score, comment count. Optional: thumbnail for link/media posts
- **Clicking title opens post detail page.** Clicking community name navigates to community. Clicking author navigates to profile
- **Vote arrows are on the left side** of each feed card. Orange up, blue down, gray neutral. This layout is burned into Reddit user muscle memory
- **"No posts yet" empty state** for new communities. Don't show a blank page

### Comment UX
- **Indentation visually communicates reply depth.** Each level indented ~20-24px with a colored vertical line on the left (different color per depth level)
- **Collapse/expand thread toggle.** Clicking the vertical line or a [-] button collapses that subtree. Critical for navigating large threads
- **Reply box opens inline** below the comment being replied to. Don't navigate away from the page
- **"Continue this thread" link** at max display depth (3 levels). Links to a new page rooted at that comment
- **Deleted comments show `[deleted]`** but children remain visible. If a parent is deleted and ALL children are also deleted, collapse the entire subtree

### Voting UX
- **Click to vote, click again to un-vote.** Not a separate "remove vote" button
- **Score updates immediately on client** (optimistic update). Don't wait for server response
- **Vote state persists across page loads.** User's own vote direction is always highlighted when they see content they've voted on

### Community UX
- **Sidebar shows:** community description, rules, member count, creation date, moderator list, join/leave button
- **Community header:** banner image + icon + name
- **Rules displayed prominently.** Numbered list, expandable

### Auth UX
- **Registration is a single form**, not multi-step. Email, username, password, submit. OTP verification on next page
- **Error messages are specific:** "Username already taken", "Password must be at least 8 characters", not generic "Registration failed"
- **After login, redirect to the page the user was on**, not a generic home page
- **Token refresh is invisible** to the user. They should never see "session expired, please log in again" during active use

### Navigation UX
- **Global search bar** in the header/nav
- **Community sidebar** on desktop, collapsible on mobile
- **Breadcrumb-style navigation:** Home > Community Name > Post Title
- **Back button works correctly.** This means proper browser history management, not SPA state hacks that break the back button
- **Mobile-responsive.** Single-column layout on mobile, cards stack vertically, vote buttons accessible with thumb

---

## Complexity Assessment by Feature Area

Honest assessment of where the real difficulty lies.

| Feature Area | Nominal Complexity | Actual Complexity | Why the Gap? |
|-------------|-------------------|-------------------|-------------|
| Auth (email/password, JWT) | Low | Low | Standard patterns, well-documented |
| Auth (Google OAuth) | Medium | Medium | OAuth flow has edge cases (token refresh, account linking, email conflicts) |
| User profiles | Low | Low | CRUD with pagination |
| Communities (basic CRUD) | Low | Low | Standard relational model |
| Communities (visibility enforcement) | Medium | **High** | Must enforce at every write endpoint across Post, Comment services. Cross-service authorization check |
| Text/link posts | Medium | Medium | Markdown rendering + XSS prevention is the tricky part |
| **Home feed aggregation** | Medium | **Very High** | Cross-shard query across all joined communities. Requires caching strategy, pagination across shards, merge-sort of results. **This is the hardest feature in the platform** |
| Community feed | Medium | Low | Single shard query (all posts for a community on same shard by design) |
| **Nested comments (ScyllaDB)** | High | **Very High** | Materialized path model in ScyllaDB. Lazy-load with correct ordering. Tree reconstruction on read. Pagination within a tree is non-trivial. Denormalization trade-offs |
| Voting (Redis + Kafka) | Medium | Medium | Redis for speed, Kafka for durability. Idempotency is the key concern |
| **Ranking algorithms** | Medium | **High** | Hot, Best (Wilson), Controversial, Rising all need correct implementation. Wrong algorithm = bad UX. Must be efficient (precomputed or indexed, not calculated per-request) |
| Search (Meilisearch) | Medium | Low-Medium | Meilisearch handles the hard parts. Keeping index in sync via Kafka is the main concern |
| WebSocket notifications | High | **Very High** | Multi-pod connection routing via Redis. Heartbeat management. Offline storage + reconnect delivery. Connection lifecycle management across deploys |
| Media upload pipeline | Medium | Medium | Validation + ClamAV + S3 + thumbnail generation. Well-understood pipeline |
| Rate limiting | Medium | Low-Medium | Token bucket + sliding window in Redis. Envoy integration is the tricky part |
| Spam detection (pre-publish) | Medium | Medium | Synchronous checks are latency-sensitive. Must not slow down post creation noticeably |
| Spam detection (post-publish) | High | High | Async analysis, behavior scoring. High false-positive risk |
| Moderation tools | Medium | Medium | Standard CRUD with authorization. Mod queue requires aggregation of reports |
| **Account deletion (true PII purge)** | Medium | **High** | Must update records across ALL services: Auth, User, Post, Comment, Vote, Notification, Moderation. Distributed transaction problem. Must not miss any PII |

---

## MVP Recommendation

### Priority 1: Core Loop (must work or nothing works)
1. **Auth** (email/password registration + login + JWT) — gate to everything
2. **Communities** (create + join + basic settings) — content organization
3. **Text posts** (create in community + community feed with New sort) — first content
4. **Nested comments** (create + reply + tree display) — first discussion
5. **Voting** (upvote/downvote on posts + comments) — first engagement signal
6. **Rate limiting** (basic per-user limits) — protection from day one

### Priority 2: Makes It Usable
7. **Home feed** (aggregated from joined communities + Hot sort) — the "front page"
8. **Feed sorting** (Hot, New, Top with time filters) — content discovery
9. **Comment sorting** (Best using Wilson score, Top, New) — thread quality
10. **User profiles** (username, karma, post history) — user identity
11. **Google OAuth** — lower friction registration
12. **Link posts** — content variety

### Priority 3: Makes It Good
13. **Moderation** (remove, ban, pin, mod log) — community governance
14. **Search** (full-text + community autocomplete) — content discovery at scale
15. **Notifications** (WebSocket + offline storage) — engagement loop
16. **Spam detection** (pre-publish filtering + new account restrictions) — quality control
17. **Media posts** (image upload + CDN) — visual content

### Defer to v1.1+
- Post-publish behavior analysis (needs data volume)
- Shadow-banning (complex, ethically questionable)
- Vote manipulation detection (needs data volume)
- Flair/tag system (low value for effort)
- Anonymous posting mode (differentiator but can wait for core to stabilize)
- Advanced notification preferences
- Profile customization (avatar, bio)
- Video upload (complexity of transcoding)

**Note on anonymous posting:** This is Redyx's core differentiator per the project vision, but it's P2 in the SRS and depends on all the basic post/comment/mod infrastructure being solid first. Ship the basic platform, then layer anonymity on top.

---

## Sources

- **SRS (primary):** `/docs/Software Requirement Specification Document.md` — 45 user stories, full requirements
- **Architecture Plan:** `/docs/Architecture Plan.md` — service boundaries, database strategy
- **Core Concepts:** `/docs/Core Concepts.md` — domain model, feature priorities
- **Reddit ranking:** Evan Miller, "How Not To Sort By Average Rating" (evanmiller.org, 2009) — Wilson score confidence interval, adopted by Reddit for "Best" comment sort. HIGH confidence
- **Lemmy ranking algorithm:** Lemmy docs, `contributors/07-ranking-algo.html` — improved Hot sort with logarithmic scale and time decay. HIGH confidence
- **Lemmy moderation model:** Lemmy docs, `users/04-moderation.html` — moderation hierarchy, action types. HIGH confidence
- **Lemmy vote/sort patterns:** Lemmy docs, `users/03-votes-and-ranking.html` — sort types (Active, Hot, Scaled, New, Top). HIGH confidence
- **Reddit site overview:** Wikipedia, "Reddit" — features, user patterns, moderation history, technology. MEDIUM confidence (secondary source but comprehensive)
- **Reddit UX patterns:** Based on established Reddit interface conventions widely documented and observed. MEDIUM confidence (training data + cross-referenced with Lemmy implementation)
