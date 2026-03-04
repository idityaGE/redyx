# Phase 4: Comments (Full Stack) - Context

**Gathered:** 2026-03-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can have threaded discussions on posts. Backend comment service with ScyllaDB storage using materialized path ordering. Frontend comment tree component supporting nested replies, sorting, and lazy-loading of deep threads. Comment voting integrates with the existing vote service.

This phase does NOT include: search within comments, @mentions, notifications on replies (Phase 5), or moderation actions on comments (Phase 6).

</domain>

<decisions>
## Implementation Decisions

### Thread Depth & Nesting
- Visual nesting limited to **3 levels** deep (top-level → reply → reply-to-reply)
- Deeper threads display a `[load N more replies]` link — continuation UX is Claude's discretion (inline expansion with reset indentation, or separate thread view)
- Depth indicated by **left padding + a thin `border-left` line** per nesting level (same dim terminal color)
- Each comment has a **[-]/[+] toggle button** in the comment header to collapse/expand its subtree
- Collapsed comment shows: `[+] u/username · N replies`

### Reply Flow
- Reply form appears **inline directly below the comment** being replied to (matches existing inline-edit pattern from PostDetail)
- After submitting: reply form **closes and the new reply appears inline** via optimistic insert at correct thread position
- Top-level comment box is a **click-to-reveal button**: `[write comment]` expands into a textarea
- **No markdown preview** for comments — submit directly, markdown renders after posting (comments are typically short)

### Deleted & Collapsed State
- Deleted comments show **`[deleted]` as body** but preserve: timestamp, depth position, reply count, and thread structure; author becomes `[deleted]`
- Children of deleted comments **remain visible** — thread structure is preserved
- If a deleted comment is a leaf node with no children, it still shows as `[deleted]` stub (consistent behavior)
- `[load N more replies]` shows the **actual reply count** so users know how many are hidden
- Score-based auto-collapse behavior is **Claude's discretion** (e.g., whether to collapse comments below -5 threshold)

### Comment Sort
- Default sort order: **Best (Wilson score)** — surfaces quality comments by upvote ratio
- Sort selector: **inline SortBar** component above the comment list (matching existing feed SortBar pattern)
- 3 sort options for v1: **Best, Top, New** — Controversial deferred to later (requires more nuanced scoring and vote data)
- Sort preference **persists in localStorage** across posts

### Claude's Discretion
- Deep thread continuation UX (inline expansion vs separate view)
- Score-based auto-collapse threshold and behavior
- Loading skeleton design for comment tree
- Exact spacing, indentation width per nesting level
- Error state handling for failed comment operations
- Comment character count indicator approach (10K max from proto)
- Controversial sort implementation timing

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `VoteButtons.svelte`: Already accepts `targetType` prop (defaults to `TARGET_TYPE_POST`). Reuse with `targetType='TARGET_TYPE_COMMENT'` and pass `commentId` as `postId` prop for comment voting
- `PostBody.svelte`: Uses `marked` + `DOMPurify` for markdown rendering. Directly reusable for comment body display
- `SortBar.svelte`: Sort selector component from feed pages. Pattern can be replicated for comment sort (Best/Top/New)
- `api.ts`: API client with auth injection, 401 retry, `ApiError` class — all comment API calls use this
- `auth.ts`: `whenReady()` promise, `isAuthenticated()`, `getUser()`, `subscribe()` — needed for reply forms and vote state
- `time.ts`: `relativeTime()` utility — reuse for comment timestamps

### Established Patterns
- **Component-local `$state`**: Each comment card should own its own vote/collapse/reply state (not global store), matching VoteButtons pattern
- **Optimistic UI**: Vote and save use optimistic update with rollback — apply same pattern to comment voting and reply submission
- **`whenReady().then()`**: Auth initialization before API calls — comment section must wait for auth before fetching
- **`untrack()` pattern**: For `$effect` + `$state` mutation to prevent infinite loops in Svelte 5
- **Inline actions**: Terminal-style `[edit]`, `[delete]`, `[reply]` buttons matching PostDetail action bar
- **Inline confirmation**: `delete? yes / no` pattern from PostDetail — reuse for comment deletion
- **Terminal aesthetic**: `box-terminal`, `border-terminal-border`, `text-terminal-dim`, `font-mono`, `> error:` prefix for errors

### Integration Points
- `PostDetail.svelte` line 330: `{post.commentCount} comments` — comment tree component mounts below this in the post detail page
- `web/src/pages/post/[id].astro`: Post detail page — comment section added here
- `comment.proto`: Fully defined with CreateComment, GetComment, UpdateComment, DeleteComment, ListComments (top-level with sort + pagination), ListReplies (per-comment with pagination)
- Vote service `/api/v1/votes` endpoint: Already supports `targetType` field — comment voting works by passing `TARGET_TYPE_COMMENT`
- `CommentSortOrder` enum in proto: `BEST`, `TOP`, `NEW`, `CONTROVERSIAL` — frontend exposes first 3

</code_context>

<specifics>
## Specific Ideas

- Collapse summary follows terminal aesthetic: `[+] u/username · 12 replies` (monospace, dim, one line)
- Reply button as `[reply]` action in comment action bar, consistent with existing `[edit]`, `[delete]`, `[save]` patterns
- Top-level comment entry as `[write comment]` button, matching the terminal command style
- Comment sort bar follows the same visual pattern as feed SortBar but with comment-specific options

</specifics>

<deferred>
## Deferred Ideas

- **Controversial sort** — Requires more vote data and nuanced scoring. Add when comment engagement is measurable.
- **Comment @mentions** — Notify users when mentioned. Belongs in Phase 5 (Notifications).
- **Comment search** — Search within comments on a post. Belongs in Phase 5 (Search).
- **Moderator comment actions** — Remove, lock threads. Belongs in Phase 6 (Moderation).

</deferred>

---

*Phase: 04-comments*
*Context gathered: 2026-03-04*
