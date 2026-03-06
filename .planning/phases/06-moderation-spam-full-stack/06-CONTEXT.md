# Phase 6: Moderation + Spam (Full Stack) - Context

**Gathered:** 2026-03-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Communities get moderation tools and the platform detects/prevents spam. Moderators can remove posts/comments, ban users (temporary or permanent), and pin posts — all logged. Users can report content, and mods triage via a report queue. Content is checked against keyword/URL blocklists before publishing, and async behavior analysis via Kafka detects spam patterns. Frontend includes report dialog, mod dashboard (report queue, mod log, ban management), and pinned post controls — all gated to moderator role.

</domain>

<decisions>
## Implementation Decisions

### Report flow
- Report action triggered via three-dot overflow menu on posts and comments
- Report dialog shows a predefined reason picker — no free text field
- Reasons: Spam, Harassment, Misinformation, Breaks community rules, Other
- After submitting: confirmation toast ("Report submitted"), no status tracking or "My Reports" page
- Reports are anonymous to the reported user

### Mod dashboard & queue
- Report queue is the primary/landing view when mod opens mod tools
- Mod tools accessed as a tab within existing CommunitySettings (already gated to moderators)
- Inline action buttons on each queue item: [remove] [dismiss] [ban user] — no detail view expansion
- Inline confirmation pattern before destructive actions (button changes to [confirm?] / [cancel]) — matches existing CommentCard delete pattern
- No pending report count badge or indicator — mods check manually
- Resolved items move to a separate "Resolved" tab — mods can undo ALL actions (remove, dismiss, ban) from there
- Undoing an action moves the item back to the active queue
- Mod log shows full entries: mod name, action taken, target content/user, timestamp, reason
- Mod log filterable by action type (remove, ban, pin, dismiss) using dropdown — reuse SortBar pattern

### Ban communication
- Banned user sees a banner at top of community page: "You are banned from this community. Reason: [reason]. Expires: [date or Permanent]."
- Post/comment forms disabled for banned users — can still view/read all content
- Preset ban durations only: 1 day, 3 days, 7 days, 30 days, Permanent
- Ban reason is required — mod must provide a reason
- Ban reason is visible to the banned user in the ban banner
- Ban list tab in mod tools showing all active bans: username, reason, duration, date banned, [unban] button
- Expired bans auto-removed from active list
- No warning system or appeal mechanism — direct bans only (v1)
- Ban dialog includes checkbox: "Also remove all posts/comments by this user" — optional, not default
- Mods can ban directly from report queue inline actions (not just from ban management section)

### Spam filter visibility
- Keyword/URL blocklist check happens at publish time — content is rejected, not silently held
- User sees vague rejection message: "Your post couldn't be published — it may contain restricted content." Does not reveal exact blocklist match
- Async behavior analysis (Kafka: rapid posting, link spam patterns) flags to mod queue — no auto-punishment or auto-restriction
- Spam-detected items show a distinct [spam-detection] tag in the report queue, separate from [user-report] tags
- Same queue, same actions — mods can filter by source type
- Blocklists are platform-level only (global) — no per-community customization in v1
- Duplicate content from same user is rejected at publish time (same mechanism as blocklist)

### Claude's Discretion
- Loading states and skeleton design for mod dashboard
- Exact spacing, typography, and terminal-aesthetic styling details
- Error state handling across mod tools
- How pinned posts are visually distinguished in the feed (pin icon, position, styling)
- Pin/unpin UI interaction details (inline on post vs from mod tools)
- Exact behavior analysis thresholds (what constitutes "rapid posting" or "link spam")
- Blocklist seed data (initial keywords and URLs)
- Database schema details (mod_log, reports, bans tables)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `CommunitySettings.svelte`: Already has mod-only gating — extend with mod tools tabs (queue, log, bans)
- `CommentCard.svelte`: Has inline confirmation delete pattern (`confirmingDelete` state) — reuse for mod action confirmations
- `FeedList.svelte` + `FeedRow.svelte`: Paginated list components — reuse for report queue and mod log
- `SortBar.svelte`: Filter/sort bar component — reuse for mod log action type filtering
- `VoteButtons.svelte`: Action button pattern — reference for report/remove/pin action buttons
- `CommunitySidebar.svelte`: Potential integration point for "Mod Tools" quick link (if needed later)
- `NotificationItem.svelte`: Entry/row rendering pattern — reference for mod log entries

### Established Patterns
- Terminal/hacker aesthetic: `border-terminal-border`, `bg-terminal-surface`, `font-mono`, `[bracket]` buttons
- Box sections with `┌─ title` / `└─────` borders
- Status messages: `{ type: 'success' | 'error'; text: string }` — reuse for report/ban confirmations
- Loading states: `[loading...]` / `[saving...]` text patterns
- Inline confirmation: `confirmingX` boolean state pattern (not modal dialogs)
- Auth context: `auth.ClaimsFromContext(ctx)` for user identity in backend
- Kafka producer/consumer pattern: `NewXProducer` / `NewXConsumer` with `EnsureTopic` and `PollFetches` loop

### Integration Points
- **Moderation proto**: `proto/redyx/moderation/v1/moderation.proto` — already defined with `RemoveContent`, `BanUser`, `UnbanUser`, `PinPost`, `UnpinPost`, `GetModLog`, `ListReportQueue`
- **Spam proto**: `proto/redyx/spam/v1/spam.proto` — already defined with `CheckContent`, `ReportSpam`, `SpamCheckResult` enum
- **Generated Go code**: `gen/redyx/moderation/v1/` and `gen/redyx/spam/v1/` — compiled and ready
- **Post `is_pinned`**: Column exists in posts table, field exists in proto, already scanned in queries
- **Post `is_deleted`** / **Comment `is_deleted`**: Both exist and are filtered in list queries
- **Role system**: `community_members.role` with `member/moderator/owner` — `getMemberRole()` helper at `internal/community/server.go:617`
- **Post server TODO**: `internal/post/server.go:407` — needs moderator permission check (deferred to this phase)
- **Comment server TODO**: `internal/comment/server.go:184` — moderator check deferred to Phase 6
- **Envoy gateway**: Needs new routes for moderation and spam services in `deploy/envoy/envoy.yaml`
- **Docker Compose**: Needs `moderation-service` container and `moderation` database in `deploy/docker/init-databases.sql`
- **Auth interceptor**: Needs mod service methods added to `publicMethods` map in `internal/platform/auth/interceptor.go`

</code_context>

<specifics>
## Specific Ideas

- Resolved tab with undo capability — user wants mods to be able to reverse any action, not just bans. Full audit trail with reversibility.
- Ban dialog "remove all content" checkbox — gives mods the nuclear option but doesn't force it. Default unchecked.
- Vague rejection message for blocklist hits — intentionally doesn't reveal what triggered the block to prevent gaming the filter.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 06-moderation-spam-full-stack*
*Context gathered: 2026-03-06*
