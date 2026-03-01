---
title: "Redyx"
subtitle: "User Stories"
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
header-left: "Redyx - User Stories"
header-right: "February 2026"
footer-left: "Aditya"
footer-center: "Confidential"
code-block-font-size: "\\small"
footnotes-pretty: true
tags: [type/project, proj/reddit-clone, status/active]
---

# 1. Authentication

### US-01: Email registration

**As a** visitor, **I want** to register with my email, a username, and a password **so that** I can create an account on the platform.

**Priority:** P0 | **Estimate:** 3 days

The system collects email, a unique username, and a password. The password is hashed with argon2id before storage. The account stays inactive until the user verifies their email through OTP.

**Acceptance criteria:**

- Registration form validates email format, username uniqueness, and password strength
- Password is hashed with argon2id and stored in pg-auth
- A 6-digit OTP is sent to the provided email
- Account is inactive until OTP verification completes
- Duplicate emails and usernames are rejected with specific error messages

---

### US-02: OTP email verification

**As a** newly registered user, **I want** to verify my email with a one-time code **so that** I can activate my account.

**Priority:** P0 | **Estimate:** 1 day

After registration, the user receives a 6-digit code by email. The code expires after 5 minutes. Entering the correct code activates the account and issues a JWT token pair.

**Acceptance criteria:**

- OTP is a 6-digit numeric code stored in Redis with a 5-minute TTL
- Entering the correct code activates the account
- Expired or incorrect codes return an error
- User can request a new OTP if the previous one expired
- After verification, the system returns an access token (15 min) and refresh token (7 days)

---

### US-03: Google OAuth registration

**As a** visitor, **I want** to sign up with my Google account **so that** I can skip filling out a registration form.

**Priority:** P0 | **Estimate:** 2 days

The user clicks "Sign in with Google," authorizes the app, and is redirected back. The system fetches their email from Google. They still pick a username. No separate OTP step is needed since Google already verified the email.

**Acceptance criteria:**

- Clicking "Sign in with Google" redirects to Google's OAuth consent screen
- After authorization, the system exchanges the auth code for Google tokens
- The user is prompted to choose a unique username
- Email from Google is stored (encrypted) but never shown to other users
- A JWT token pair is issued after username selection

---

### US-04: Login

**As a** registered user, **I want** to log in with my email and password (or Google) **so that** I can access my account.

**Priority:** P0 | **Estimate:** 1 day

The user provides credentials. The system verifies them and issues a short-lived access token and a longer-lived refresh token.

**Acceptance criteria:**

- Email/password login returns a token pair on success
- Google OAuth login works for existing accounts linked to Google
- Wrong password returns a generic "invalid credentials" error (no leaking whether the email exists)
- Access token expires in 15 minutes, refresh token in 7 days

---

### US-05: Logout and token refresh

**As a** logged-in user, **I want** to log out or refresh my session **so that** I can control my authentication state.

**Priority:** P0 | **Estimate:** 1 day

Logging out blacklists the refresh token. Token refresh issues a new access token using a valid refresh token.

**Acceptance criteria:**

- Logout invalidates the refresh token (added to Redis blacklist)
- Subsequent requests with the old access token fail after it expires
- Refresh endpoint issues a new access token if the refresh token is valid and not blacklisted
- Using a blacklisted refresh token returns 401

---

# 2. User profiles

### US-06: View public profile

**As a** user, **I want** to view another user's profile **so that** I can see their username, karma, and post history.

**Priority:** P1 | **Estimate:** 2 days

Public profiles show the username, karma score, cake day, and the user's post and comment history. No private information (email, auth method) is exposed.

**Acceptance criteria:**

- Profile page displays username, karma, cake day
- Post history and comment history are paginated
- Email, auth provider, and IP are never returned by the API
- Nonexistent usernames return 404

---

### US-07: Edit profile

**As a** user, **I want** to update my display name, bio, and avatar **so that** I can personalize my profile.

**Priority:** P2 | **Estimate:** 1 day

Users can set a display name, write a short bio, and upload an avatar image. The avatar goes through the Media Service.

**Acceptance criteria:**

- User can update display name and bio from settings
- Avatar upload goes through the Media Service and stores the URL in pg-user
- Changes are reflected on the public profile immediately
- Bio has a character limit (500 chars)

---

### US-08: Update account settings

**As a** user, **I want** to change my notification preferences and default sort order **so that** I can customize my experience.

**Priority:** P2 | **Estimate:** 1 day

Settings include email notification toggles, default feed sort order, and theme preference.

**Acceptance criteria:**

- User can toggle email notifications on/off
- User can set a default sort order (Hot, New, Top, Rising)
- Settings persist across sessions
- Changes take effect immediately

---

### US-09: Delete account

**As a** user, **I want** to permanently delete my account **so that** all my personal data is removed from the platform.

**Priority:** P1 | **Estimate:** 2 days

Account deletion wipes all PII. Posts and comments are replaced with `[deleted]`. Vote records are anonymized. This is irreversible.

**Acceptance criteria:**

- Deletion requires password confirmation (or re-auth for OAuth users)
- Email, password hash, and OAuth tokens are permanently deleted from pg-auth
- Profile data is wiped from pg-user
- All posts and comments show `[deleted]` as the author
- Vote records are anonymized (user_id set to null)
- The username becomes available again after 30 days

---

# 3. Communities

### US-10: Create a community

**As a** user, **I want** to create a community with a unique name **so that** people can gather around a shared topic.

**Priority:** P0 | **Estimate:** 2 days

Any authenticated user can create a community. The name must be unique and cannot be changed later. The creator becomes the first moderator.

**Acceptance criteria:**

- Community name is validated (alphanumeric + underscores, 3-21 chars, unique)
- Name is immutable after creation
- Creator is automatically assigned the OWNER role
- Community is created in pg-community with default settings (public visibility)
- Rate limit: 1 community creation per day

---

### US-11: Browse and join communities

**As a** user, **I want** to browse available communities and join ones that interest me **so that** I can see their posts in my feed.

**Priority:** P0 | **Estimate:** 1 day

Users can browse a list of communities, view their descriptions, and join or leave at will. Joining adds the community's posts to the user's home feed.

**Acceptance criteria:**

- Community list is paginated and sortable (by member count, name, date)
- Joining a community increments its member count
- Leaving decrements the member count
- Joined communities appear in the user's community list
- Private communities require an invite to join

---

### US-12: Configure community settings

**As a** community owner, **I want** to set the description, rules, banner, icon, and visibility **so that** the community is well-defined.

**Priority:** P1 | **Estimate:** 2 days

Owners and moderators can update the community's metadata. Visibility can be public, restricted, or private.

**Acceptance criteria:**

- Description, banner, and icon can be updated by moderators/owners
- Rules are an ordered list with title and description per rule
- Visibility options: public (anyone can view and post), restricted (anyone can view, approved users can post), private (invite-only)
- Changes to community metadata invalidate the Redis cache

---

### US-13: Manage community members

**As a** community owner, **I want** to assign moderators and manage membership **so that** the community is properly run.

**Priority:** P1 | **Estimate:** 1 day

Owners can promote members to moderator. Moderators cannot demote other moderators of equal rank.

**Acceptance criteria:**

- Owner can assign MODERATOR role to any community member
- Owner can revoke MODERATOR role
- Moderators cannot change other moderators' roles
- Member list is paginated and shows each user's role

---

# 4. Posts

### US-14: Create a text post

**As a** user, **I want** to write a text post in a community **so that** I can share my thoughts on a topic.

**Priority:** P0 | **Estimate:** 2 days

A text post has a title and a markdown body. It belongs to exactly one community. The post is stored on the correct shard based on the community_id.

**Acceptance criteria:**

- Post requires a title (max 300 chars) and a body (markdown, max 40,000 chars)
- Post is stored on the shard determined by consistent hashing of community_id
- A PostCreated event is published to Kafka
- Post displays author username, timestamp, community name, and initial score of 1
- Author's own upvote is automatically applied

---

### US-15: Create a link post

**As a** user, **I want** to submit a URL as a post **so that** I can share external content with the community.

**Priority:** P1 | **Estimate:** 1 day

A link post has a title and a URL. The URL is validated and stored alongside the post.

**Acceptance criteria:**

- Post requires a title and a valid URL
- URL is validated for format (must be http/https)
- Link posts display a clickable URL and optionally an embedded preview
- Post is published to Kafka for search indexing

---

### US-16: Create a media post

**As a** user, **I want** to upload an image or video as a post **so that** I can share visual content.

**Priority:** P2 | **Estimate:** 2 days

The user uploads media through the Media Service, which validates, scans, and stores it in S3. The returned URL is attached to the post.

**Acceptance criteria:**

- Upload goes through Media Service (validation, ClamAV scan, S3 storage)
- Thumbnail is generated for images
- Post stores the S3 media URL and thumbnail URL
- File size and type limits are enforced (e.g., 20MB images, 100MB video)
- Media is served through CloudFront CDN

---

### US-17: Edit and delete posts

**As a** post author, **I want** to edit or delete my own post **so that** I can correct mistakes or remove content.

**Priority:** P0 | **Estimate:** 1 day

Authors can edit the body of their posts and delete them entirely. Deleted posts show `[deleted]`.

**Acceptance criteria:**

- Only the author can edit or delete their own post
- Editing updates the body and sets an "edited" timestamp
- Deleting replaces the post body with `[deleted]` and hides the author
- Comment threads on deleted posts remain intact
- A PostUpdated/PostDeleted event is published to Kafka

---

### US-18: View home feed

**As a** user, **I want** to see a feed of posts from my joined communities **so that** I can catch up on new content.

**Priority:** P1 | **Estimate:** 3 days

The home feed aggregates posts across all communities the user has joined. It supports sorting by Hot, New, Top, and Rising.

**Acceptance criteria:**

- Feed pulls posts from all the user's joined communities
- Sorting options: Hot, New, Top (with time filter: hour/day/week/month/year/all), Rising
- Feed is paginated (cursor-based or offset)
- Hot feed is cached in Redis with a 5-minute TTL
- Anonymous visitors see a global feed of popular posts

---

### US-19: View community feed

**As a** user, **I want** to browse all posts in a specific community **so that** I can see what people are discussing there.

**Priority:** P0 | **Estimate:** 1 day

Each community has its own feed with the same sorting options. Since all posts for a community live on one shard, this query is efficient.

**Acceptance criteria:**

- Community feed shows all posts for that community, paginated
- Same sorting options as the home feed (Hot, New, Top, Rising)
- Pinned posts appear at the top regardless of sort
- Removed posts are hidden from non-moderators

---

### US-20: Save and unsave posts

**As a** user, **I want** to bookmark posts **so that** I can find them again later.

**Priority:** P2 | **Estimate:** 1 day

Users can save posts to a personal list and remove them. Saved posts are private.

**Acceptance criteria:**

- Save/unsave toggle on each post
- Saved posts are listed on a dedicated "Saved" page, paginated
- Saved list is private to the user
- Saving and unsaving is idempotent

---

### US-21: Anonymous posting

**As a** user, **I want** to post anonymously in a community **so that** I can share something without my username attached.

**Priority:** P2 | **Estimate:** 1 day

When creating a post, the user can check an "anonymous" option. The post shows `[anonymous]` as the author. Moderators can still see the real author for moderation purposes.

**Acceptance criteria:**

- Anonymous posts display `[anonymous]` as the author to all regular users
- Community moderators can see the real author's username
- Anonymous posts follow the same rules and rate limits as regular posts
- The author can still edit and delete their own anonymous post

---

# 5. Comments

### US-22: Comment on a post

**As a** user, **I want** to leave a comment on a post **so that** I can participate in the discussion.

**Priority:** P0 | **Estimate:** 2 days

Comments are stored in ScyllaDB, partitioned by post_id. Top-level comments have no parent. A CommentCreated event goes to Kafka for notifications and search indexing.

**Acceptance criteria:**

- Comment body is required (max 10,000 chars, markdown)
- Comment is stored in ScyllaDB with the correct partition (post_id) and path
- Post's comment count is incremented via gRPC to Post Service
- Author username is denormalized into the comment row
- CommentCreated event is published to Kafka

---

### US-23: Reply to a comment

**As a** user, **I want** to reply to an existing comment **so that** I can have a threaded conversation.

**Priority:** P0 | **Estimate:** 1 day

Replies reference a parent comment. The materialized path (e.g., `root/parent/child`) determines tree ordering.

**Acceptance criteria:**

- Reply stores parent_id and appends to the materialized path
- Nested replies are displayed in tree structure
- Reply count on the parent comment is updated
- The parent comment's author gets a notification

---

### US-24: Lazy-load deep threads

**As a** user, **I want** to load deeper replies on demand **so that** the page isn't overwhelmed by huge threads.

**Priority:** P1 | **Estimate:** 1 day

Only the top 2-3 levels of comments load initially. A "load more replies" link fetches the next level.

**Acceptance criteria:**

- Initial load returns comments up to depth 3
- "Load more" fetches the next batch of children for a given parent path
- Each batch is paginated (e.g., 20 replies per load)
- Deep threads don't block the initial page load

---

### US-25: Delete a comment

**As a** comment author, **I want** to delete my comment **so that** I can remove something I no longer want visible.

**Priority:** P0 | **Estimate:** 0.5 days

Deleted comments show `[deleted]` but the thread structure stays intact. Replies remain visible.

**Acceptance criteria:**

- Only the author (or a moderator) can delete a comment
- Deleted comment body is replaced with `[deleted]`, author is hidden
- Child replies remain visible and the tree structure is preserved
- If all children are also deleted, the thread collapses

---

# 6. Voting

### US-26: Vote on a post or comment

**As a** user, **I want** to upvote or downvote posts and comments **so that** I can surface good content and bury bad content.

**Priority:** P0 | **Estimate:** 3 days

Each user gets one vote per item. The vote state is stored in Redis for speed. A VoteCreated event goes to Kafka, which updates the item's score and the author's karma asynchronously.

**Acceptance criteria:**

- User can upvote, downvote, or remove their vote on any post or comment
- Only one vote per user per item (enforced in Redis)
- Vote updates the score displayed on the item within 500ms
- VoteCreated event is published to Kafka
- Post/Comment Service consumes the event and updates the score in its database
- User Service consumes the event and adjusts the author's karma

---

### US-27: Change or remove a vote

**As a** user, **I want** to change my vote from up to down (or remove it) **so that** I can correct a mistake.

**Priority:** P0 | **Estimate:** 0.5 days

Changing a vote adjusts the counts accordingly (e.g., switching from up to down is -2 net). Removing a vote restores the previous count.

**Acceptance criteria:**

- Switching from upvote to downvote publishes a single event with old and new direction
- Score adjusts correctly (up to down = -2, down to up = +2, remove = +/-1)
- The operation is idempotent (voting up twice has no additional effect)
- Vote state in Redis is updated atomically

---

### US-28: View vote counts

**As a** user, **I want** to see the net score on posts and comments **so that** I know what the community thinks.

**Priority:** P0 | **Estimate:** 0.5 days

The net score (upvotes minus downvotes) is displayed on every post and comment. The user's own vote direction is highlighted.

**Acceptance criteria:**

- Net score is fetched from Redis (real-time counts)
- User's own vote direction is indicated (highlighted up/down arrow)
- Batch fetching of vote counts for a feed page (avoid N+1 queries)
- Score updates are visible within 500ms of voting

---

# 7. Search

### US-29: Search for posts

**As a** user, **I want** to search posts by title or body text **so that** I can find specific content.

**Priority:** P2 | **Estimate:** 2 days

Full-text search is powered by Meilisearch. Results are ranked by relevance, recency, and score. Users can search globally or within a specific community.

**Acceptance criteria:**

- Search query matches against post title and body
- Results are ranked by relevance, with recency and score as tiebreakers
- Optional community filter narrows results to a single community
- Results return within 300ms
- Search results include title, snippet, author, community, score, and timestamp

---

### US-30: Search autocomplete

**As a** user, **I want** community name suggestions as I type in the search bar **so that** I can quickly navigate to a community.

**Priority:** P2 | **Estimate:** 1 day

Typing in the search bar triggers prefix-based autocomplete for community names.

**Acceptance criteria:**

- Autocomplete triggers after 2+ characters
- Results show community name, icon, and member count
- Cached in Redis with a 30-minute TTL
- Response time under 100ms

---

# 8. Notifications

### US-31: Receive real-time notifications

**As a** user, **I want** to get notified in real time when someone replies to my post or comment **so that** I don't miss conversations.

**Priority:** P1 | **Estimate:** 3 days

When the user is logged in, a persistent WebSocket connection delivers notifications as they happen. The Notification Service consumes Kafka events and routes them to the right user's WebSocket.

**Acceptance criteria:**

- WebSocket connection is established on login
- Notifications for replies appear within 1 second
- Notification includes: type, source username, target post/comment, timestamp
- Unread badge count updates in real time
- Heartbeat (ping/pong every 30s) keeps the connection alive

---

### US-32: Offline notification delivery

**As a** user, **I want** notifications I missed while offline to be waiting for me when I reconnect **so that** I don't lose any.

**Priority:** P1 | **Estimate:** 1 day

If the user is offline, notifications are stored in PostgreSQL. On the next WebSocket connection, unread notifications are delivered.

**Acceptance criteria:**

- Offline notifications are persisted in pg-platform (notification schema)
- On reconnect, unread notifications are pushed to the WebSocket
- Notifications are paginated (most recent first)
- Unread count is accurate on reconnect

---

### US-33: Mark notifications as read

**As a** user, **I want** to mark individual or all notifications as read **so that** I can manage my notification list.

**Priority:** P1 | **Estimate:** 0.5 days

Users can mark a single notification as read or mark all as read. The unread count in Redis updates accordingly.

**Acceptance criteria:**

- Single notification can be marked read via its ID
- "Mark all as read" sets all unread notifications to read
- Unread count in Redis is decremented/zeroed accordingly
- UI reflects the change immediately

---

### US-34: Configure notification preferences

**As a** user, **I want** to mute notifications from specific communities or disable certain notification types **so that** I only get alerts I care about.

**Priority:** P2 | **Estimate:** 1 day

Preferences are stored per user. The Notification Service checks preferences before delivering.

**Acceptance criteria:**

- User can toggle: replies, mentions, mod actions, community posts
- User can mute specific communities
- Muted notifications are not delivered via WebSocket or stored
- Preferences are checked before sending (not after)

---

# 9. Media

### US-35: Upload images to a post

**As a** user, **I want** to attach an image to my post **so that** I can share visual content.

**Priority:** P2 | **Estimate:** 2 days

Images are uploaded to the Media Service, which validates the file type and size, scans it with ClamAV, generates a thumbnail, and stores the original and thumbnail in S3.

**Acceptance criteria:**

- Accepted formats: JPEG, PNG, GIF, WebP
- Max file size: 20MB
- ClamAV scan runs before S3 upload; infected files are rejected
- Thumbnail is generated (max 320px wide)
- S3 URL and thumbnail URL are returned to the client
- Media is served through CloudFront CDN

---

### US-36: Upload community banner and icon

**As a** community moderator, **I want** to set a banner and icon for my community **so that** it has a recognizable identity.

**Priority:** P2 | **Estimate:** 1 day

Banner and icon uploads go through the same Media Service pipeline. The resulting URLs are saved in the community settings.

**Acceptance criteria:**

- Banner and icon upload use the same validation and scanning pipeline
- Banner recommended size: 1920x384; icon: 256x256
- URLs are stored in pg-community
- Old banner/icon files are not deleted immediately (kept for rollback)

---

# 10. Moderation

### US-37: Remove a post or comment

**As a** moderator, **I want** to remove a post or comment from my community **so that** I can enforce community rules.

**Priority:** P2 | **Estimate:** 1 day

Removed content is hidden from regular users but still exists in the database. The author is notified. The action is logged in the mod log.

**Acceptance criteria:**

- Moderators of the community can remove any post or comment within it
- Removed content is hidden from regular users (still visible to mods)
- The author receives a notification about the removal
- A ModAction record is created in the mod log
- A PostRemoved/CommentRemoved event is published to Kafka

---

### US-38: Ban a user from a community

**As a** moderator, **I want** to ban a user from my community **so that** they can no longer post or comment there.

**Priority:** P2 | **Estimate:** 1 day

Bans can be temporary or permanent. The banned user cannot post, comment, or vote in that community.

**Acceptance criteria:**

- Moderator specifies: target user, reason, duration (or permanent)
- Banned user cannot create posts or comments in the community
- Ban is checked on every write operation to the community
- Expired temporary bans are cleaned up by a background job
- Ban is logged in the mod log and the user is notified

---

### US-39: Pin posts

**As a** moderator, **I want** to pin up to 2 posts in my community **so that** important announcements stay at the top.

**Priority:** P2 | **Estimate:** 0.5 days

Pinned posts appear at the top of the community feed regardless of the sort order. A community can have at most 2 pinned posts.

**Acceptance criteria:**

- Moderator can pin a post; it appears first in the community feed
- Maximum 2 pinned posts per community
- Pinning a third post fails with an error
- Moderator can unpin a post at any time
- Pinned posts are visually marked in the UI

---

### US-40: View mod queue and mod log

**As a** moderator, **I want** to see reported content and a log of all mod actions **so that** I can review flagged content and audit past decisions.

**Priority:** P2 | **Estimate:** 2 days

The mod queue lists content that has been reported or auto-flagged. The mod log shows every moderation action with who did it and when.

**Acceptance criteria:**

- Mod queue lists reported items, sorted by report count (most reported first)
- Each item shows: content snippet, author, report count, report reasons
- Mod log is paginated and shows: action type, moderator, target, reason, timestamp
- Only moderators and owners of the community can access these pages

---

# 11. Rate limiting

### US-41: General API rate limiting

**As the** system, **I want** to limit the number of API requests per user **so that** no single user can overload the platform.

**Priority:** P1 | **Estimate:** 2 days

Rate limits run at the Envoy gateway using a token bucket algorithm. Limits are tiered by user type.

**Acceptance criteria:**

- Anonymous users: 10 requests/min
- Authenticated users: 100 requests/min
- Trusted users (high karma): 300 requests/min
- Exceeding the limit returns HTTP 429 with a Retry-After header
- Counters are stored in Redis db1 with TTL-based expiry

---

### US-42: Action-specific rate limits

**As the** system, **I want** to enforce per-action limits **so that** users can't spam posts, comments, or votes.

**Priority:** P1 | **Estimate:** 1 day

Sliding window counters track specific actions separately from general API limits.

**Acceptance criteria:**

- Posts: 5 per hour
- Comments: 30 per hour
- Votes: 60 per minute
- Community creation: 1 per day
- Registration: 3 per hour per IP
- Each action limit is enforced independently of the general rate limit

---

# 12. Spam and abuse detection

### US-43: Pre-publish content filtering

**As the** system, **I want** to check content against blocklists and spam patterns before it goes live **so that** obvious spam never reaches users.

**Priority:** P1 | **Estimate:** 2 days

Every post and comment is synchronously checked before being saved. The checks include keyword blocklists, URL reputation, and duplicate detection.

**Acceptance criteria:**

- Content matching the keyword blocklist is rejected with a reason
- URLs are checked against a known-bad domain list
- Identical content from the same user (by hash) is rejected
- New accounts (< 24h) cannot post; accounts < 1h cannot comment
- Rejected content returns a specific error message to the author

---

### US-44: Post-publish behavior analysis

**As the** system, **I want** to analyze user behavior asynchronously **so that** I can catch spam patterns that slip past pre-publish checks.

**Priority:** P2 | **Estimate:** 3 days

Kafka consumers analyze events for patterns like rapid posting, link spam across communities, and coordinated vote manipulation.

**Acceptance criteria:**

- Rapid posting from one user is flagged (e.g., same link posted in 5+ communities in 10 min)
- Vote manipulation detection runs on VoteCreated events (timing cluster analysis)
- Behavior scores are maintained in Redis db5 per user
- Suspicious content is flagged for moderator review
- All automated actions are logged in the audit log

---

### US-45: Shadow-ban

**As a** platform admin, **I want** to shadow-ban a spammer **so that** their content is invisible to everyone else but they don't know they've been banned.

**Priority:** P3 | **Estimate:** 1 day

A shadow-banned user's posts and comments are visible only to themselves. To everyone else, the content doesn't appear. The user receives no notification of the ban.

**Acceptance criteria:**

- Shadow-banned user can still post and comment normally from their perspective
- Their content is filtered out of feeds, search results, and community pages for all other users
- No notification or error is shown to the shadow-banned user
- Shadow-ban can be applied and revoked by platform admins
- Ban record is stored in pg-platform with an audit log entry

---

# Summary

| Area           | Stories       | Priority range | Total estimate |
|----------------|---------------|----------------|----------------|
| Authentication | US-01 to US-05 | P0            | 8 days         |
| User profiles  | US-06 to US-09 | P1-P2         | 6 days         |
| Communities    | US-10 to US-13 | P0-P1         | 6 days         |
| Posts          | US-14 to US-21 | P0-P2         | 12 days        |
| Comments       | US-22 to US-25 | P0-P1         | 4.5 days       |
| Voting         | US-26 to US-28 | P0            | 4 days         |
| Search         | US-29 to US-30 | P2            | 3 days         |
| Notifications  | US-31 to US-34 | P1-P2         | 5.5 days       |
| Media          | US-35 to US-36 | P2            | 3 days         |
| Moderation     | US-37 to US-40 | P2            | 4.5 days       |
| Rate limiting  | US-41 to US-42 | P1            | 3 days         |
| Spam detection | US-43 to US-45 | P1-P3         | 6 days         |
| **Total**      | **45 stories** |               | **65.5 days**  |
