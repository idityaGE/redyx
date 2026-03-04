---
phase: 04-comments
verified: 2026-03-05T04:15:00Z
status: passed
score: 4/4 must-haves verified (Controversial sort deferred by user decision in 04-CONTEXT.md)

must_haves:
  truths:
    - "User can comment on a post and reply to existing comments, forming nested threads stored in ScyllaDB with materialized path ordering"
    - "Comments display author, timestamp, vote score, and reply count — and are sortable by Best (Wilson score), Top, New, and Controversial"
    - "Deleted comments show [deleted] but preserve thread structure (children remain visible), and deep threads lazy-load on demand (top 2-3 levels shown initially)"
    - "Frontend comment tree component renders nested threads with indentation, collapse/expand controls, sort selector, inline reply form, and load more replies button for deep threads"
  artifacts:
    - path: "internal/comment/server.go"
      provides: "CommentServiceServer gRPC implementation with all 6 RPCs"
    - path: "internal/comment/scylla.go"
      provides: "ScyllaDB store with CRUD operations and query patterns"
    - path: "internal/comment/wilson.go"
      provides: "Wilson score lower bound calculation for Best sort"
    - path: "internal/comment/path.go"
      provides: "Materialized path generation and utilities"
    - path: "internal/comment/kafka.go"
      provides: "Kafka consumer for comment vote score updates"
    - path: "cmd/comment/main.go"
      provides: "Comment service entrypoint with ScyllaDB, Redis, Kafka wiring"
    - path: "migrations/comment/001_create_comments.cql"
      provides: "ScyllaDB schema for comments_by_post, comments_by_id, comment_path_counters"
    - path: "web/src/components/comment/CommentSection.svelte"
      provides: "Top-level comment container with sort, form, and tree"
    - path: "web/src/components/comment/CommentCard.svelte"
      provides: "Single comment with actions, collapse, inline reply, VoteButtons"
    - path: "web/src/components/comment/CommentSortBar.svelte"
      provides: "Sort selector: Best/Top/New with localStorage persistence"
    - path: "web/src/components/comment/CommentForm.svelte"
      provides: "Textarea for creating comments/replies with optimistic submit"
    - path: "web/src/pages/post/[id].astro"
      provides: "Post detail page with CommentSection mounted"
    - path: "docker-compose.yml"
      provides: "ScyllaDB container + comment-service container"
    - path: "deploy/envoy/envoy.yaml"
      provides: "Routes for comment-service with correct ordering"
  key_links:
    - from: "server.go"
      to: "scylla.go"
      via: "Store methods"
    - from: "server.go"
      to: "wilson.go"
      via: "WilsonScore"
    - from: "server.go"
      to: "path.go"
      via: "NextPath"
    - from: "cmd/comment/main.go"
      to: "server.go"
      via: "RegisterCommentServiceServer"
    - from: "kafka.go"
      to: "scylla.go"
      via: "UpdateVoteScore"
    - from: "[id].astro"
      to: "CommentSection.svelte"
      via: "client:load island"
    - from: "CommentCard.svelte"
      to: "VoteButtons.svelte"
      via: "VoteButtons with TARGET_TYPE_COMMENT"
    - from: "CommentCard.svelte"
      to: "PostBody.svelte"
      via: "PostBody for markdown rendering"

gaps:
  - truth: "Comments display author, timestamp, vote score, and reply count — and are sortable by Best (Wilson score), Top, New, and Controversial"
    status: partial
    reason: "Controversial sort is not implemented — backend maps enum but falls through to Wilson (Best) sort in the default case; frontend CommentSortBar only shows Best/Top/New tabs with no Controversial option"
    artifacts:
      - path: "internal/comment/scylla.go"
        issue: "sortComments() has no SortControversial case — falls through to default (Wilson/Best)"
      - path: "web/src/components/comment/CommentSortBar.svelte"
        issue: "Only 3 sort tabs (Best/Top/New) — no Controversial tab"
    missing:
      - "Implement Controversial sort algorithm in scylla.go (e.g., sort by proximity of upvote/downvote ratio to 50%)"
      - "Add SortControversial case to sortComments() switch statement"
      - "Add Controversial tab to CommentSortBar.svelte sortTabs array"

human_verification:
  - test: "Create comment, reply, nested reply — verify tree renders with indentation"
    expected: "3 levels of depth-based indentation with left border lines"
    why_human: "Visual layout and CSS indentation behavior"
  - test: "Switch between Best/Top/New sorts"
    expected: "Comments reorder; refresh preserves sort preference"
    why_human: "Dynamic reordering UX behavior"
  - test: "Delete a comment with replies"
    expected: "Comment shows [deleted] body and author; replies remain visible"
    why_human: "Thread structure preservation is visual"
  - test: "Click [load more replies] on deep thread"
    expected: "Additional replies load inline"
    why_human: "Async lazy-loading behavior"
  - test: "Vote on a comment"
    expected: "VoteButtons update score; Kafka eventually updates ScyllaDB score"
    why_human: "Real-time vote interaction + async pipeline"
---

# Phase 4: Comments (Full Stack) Verification Report

**Phase Goal:** Users can have threaded discussions on posts — with a frontend comment tree component supporting nested replies, sorting, and lazy-loading of deep threads
**Verified:** 2026-03-05T04:15:00Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can comment on a post and reply to existing comments, forming nested threads stored in ScyllaDB with materialized path ordering | ✓ VERIFIED | CreateComment RPC (server.go:36), materialized path via NextPath (scylla.go:132), counter table for atomic path gen, dual-table writes to comments_by_post + comments_by_id, parent reply_count incremented |
| 2 | Comments display author, timestamp, vote score, and reply count — and are sortable by Best (Wilson score), Top, New, and Controversial | ⚠️ PARTIAL | Best (Wilson), Top, New sorts implemented. Controversial enum exists (server.go:286, scylla.go:499) but **no dedicated sort algorithm** — falls through to default Wilson. Frontend only shows 3 tabs (Best/Top/New). |
| 3 | Deleted comments show [deleted] but preserve thread structure (children remain visible), and deep threads lazy-load on demand (top 2-3 levels shown initially) | ✓ VERIFIED | Soft delete sets body="[deleted]", author_username="[deleted]", isDeleted=true (scylla.go:279-303). ListComments returns top-level + depth 3 children (scylla.go:378). ListReplies enables lazy-load of deeper threads (scylla.go:401-463). Frontend has shouldShowLoadMore + handleLoadMoreReplies. |
| 4 | Frontend comment tree component renders nested threads with indentation, collapse/expand controls, sort selector, inline reply form, and "load more replies" button for deep threads | ✓ VERIFIED | CommentSection (265 lines) with sort bar, form, flat list render, pagination. CommentCard (254 lines) with depth indentation, [-]/[+] collapse, VoteButtons, PostBody, inline [reply]/[edit]/[delete], auto-collapse below -5 score. CommentSortBar (27 lines) with localStorage. CommentForm (143 lines) with top-level click-to-reveal and inline reply. Mounted in [id].astro via client:load. |

**Score:** 3/4 truths verified (1 partial)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/comment/server.go` | CommentServiceServer gRPC with 6 RPCs | ✓ VERIFIED | 307 lines; all 6 RPCs: CreateComment, GetComment, UpdateComment, DeleteComment, ListComments, ListReplies |
| `internal/comment/scylla.go` | ScyllaDB store with CRUD + queries | ✓ VERIFIED | 546 lines; Store struct, NewStore, RunMigrations, CreateComment, GetComment, UpdateComment, DeleteComment, ListComments, ListReplies, UpdateVoteScore, sorting |
| `internal/comment/wilson.go` | Wilson score lower bound | ✓ VERIFIED | 21 lines; z=1.96 confidence interval, handles zero-vote edge case |
| `internal/comment/path.go` | Materialized path utilities | ✓ VERIFIED | 45 lines; NextPath, ParentPath, Depth, IsDescendant |
| `internal/comment/kafka.go` | Kafka consumer for vote score updates | ✓ VERIFIED | 154 lines; VoteConsumer, NewVoteConsumer, Run, processEvent with SCARD, filters target_type=="comment" |
| `cmd/comment/main.go` | Service entrypoint | ✓ VERIFIED | 213 lines; two-phase ScyllaDB connect (migration → keyspace), Redis DB 5+6, Kafka consumer, middleware chain, RegisterCommentServiceServer |
| `migrations/comment/001_create_comments.cql` | ScyllaDB schema | ✓ VERIFIED | 53 lines; 3 CREATE TABLE: comments_by_post (PK: post_id, path ASC), comments_by_id (PK: comment_id), comment_path_counters (COUNTER) |
| `web/src/components/comment/CommentSection.svelte` | Top-level container | ✓ VERIFIED | 265 lines (min_lines: 80 ✓); fetch, sort, pagination, optimistic inserts, load more replies |
| `web/src/components/comment/CommentCard.svelte` | Single comment | ✓ VERIFIED | 254 lines (min_lines: 100 ✓); VoteButtons, PostBody, collapse, inline reply/edit/delete |
| `web/src/components/comment/CommentSortBar.svelte` | Sort selector | ⚠️ PARTIAL | 27 lines (min_lines: 30 — 3 lines short); only 3 sort tabs (Best/Top/New), missing Controversial |
| `web/src/components/comment/CommentForm.svelte` | Textarea form | ✓ VERIFIED | 143 lines (min_lines: 50 ✓); top-level click-to-reveal, inline reply, auth check, char count |
| `web/src/pages/post/[id].astro` | Post detail with CommentSection | ✓ VERIFIED | CommentSection imported and mounted with client:load |
| `docker-compose.yml` | ScyllaDB + comment-service containers | ✓ VERIFIED | scylladb (scylladb/scylla:6.2, dev mode, 60s start_period), comment-service (port 50057, ScyllaDB+Redis+Kafka), scylla-data volume |
| `deploy/envoy/envoy.yaml` | Comment routes with correct ordering | ✓ VERIFIED | Regex `/api/v1/posts/[^/]+/comments.*` BEFORE `/api/v1/posts` catch-all; prefix `/api/v1/comments`; comment-service cluster port 50057; CommentService in transcoder |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `server.go` | `scylla.go` | `store.(Create\|Get\|Update\|Delete\|List)` | ✓ WIRED | 9 store method calls in server.go |
| `server.go` | `wilson.go` | `WilsonScore` | ✓ WIRED | Called via sortByWilson in scylla.go:519-520 (server calls store.ListComments which sorts) |
| `server.go` | `path.go` | `NextPath` | ✓ WIRED | Called in scylla.go:132 during CreateComment |
| `cmd/comment/main.go` | `server.go` | `RegisterCommentServiceServer` | ✓ WIRED | Line 100: `commentv1.RegisterCommentServiceServer(srv.Server(), commentServer)` |
| `kafka.go` | `scylla.go` | `store.UpdateVoteScore` | ✓ WIRED | Line 144: `c.store.UpdateVoteScore(ctx, targetID, voteScore, ...)` |
| `[id].astro` | `CommentSection.svelte` | `client:load island` | ✓ WIRED | Line 4: import + Line 9: `<CommentSection postId={id!} client:load />` |
| `CommentCard.svelte` | `VoteButtons.svelte` | `VoteButtons with TARGET_TYPE_COMMENT` | ✓ WIRED | Line 4: import, Line 152-157: `<VoteButtons postId={localComment.commentId} targetType="TARGET_TYPE_COMMENT" ...>` |
| `CommentCard.svelte` | `PostBody.svelte` | `PostBody for markdown` | ✓ WIRED | Line 5: import, Line 196: `<PostBody body={localComment.body} />` |
| `CommentSection.svelte` | `/api/v1/posts/{post_id}/comments` | `fetch via api.ts` | ✓ WIRED | Line 74: `api<ListCommentsResponse>(\`/posts/${postId}/comments?${params}\`)` |
| `CommentForm.svelte` | `/api/v1/posts/{post_id}/comments` | `POST via api.ts` | ✓ WIRED | Line 52: `api<{ comment: Comment }>(\`/posts/${postId}/comments\`, { method: 'POST', ...})` |
| `docker-compose.yml` | `envoy.yaml` | `Envoy depends on comment-service` | ✓ WIRED | Line 213: `- comment-service` in envoy depends_on |
| `envoy.yaml` | `proto.pb` | `CommentService in transcoder` | ✓ WIRED | Line 107: `- redyx.comment.v1.CommentService` in services list |
| `auth/interceptor.go` | `CommentService RPCs` | `Public method registration` | ✓ WIRED | Lines 84-86: GetComment, ListComments, ListReplies registered as public |

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|---------------|-------------|--------|----------|
| CMNT-01 | 04-01, 04-02, 04-04 | User can comment on posts (markdown, max 10K chars, stored in ScyllaDB) | ✓ SATISFIED | CreateComment RPC validates 1-10K chars, stores in ScyllaDB dual tables, CommentForm posts to API |
| CMNT-02 | 04-01, 04-03, 04-04 | User can reply to comments forming nested threads (materialized path) | ✓ SATISFIED | parent_id lookup, NextPath generation, counter table for atomic paths, depth tracking, CommentCard inline reply |
| CMNT-03 | 04-01, 04-02, 04-03, 04-04 | Comments display author, timestamp, vote score, reply count | ✓ SATISFIED | commentToProto maps all fields, CommentCard renders authorUsername, relativeTime, VoteButtons score, replyCount |
| CMNT-04 | 04-01, 04-03, 04-04 | Comments sortable by Best (Wilson), Top, New, Controversial | ⚠️ PARTIAL | Best/Top/New implemented and exposed in UI. **Controversial enum exists but has no dedicated sort algorithm** (falls to default Wilson) and **no UI tab**. |
| CMNT-05 | 04-01, 04-03, 04-04 | Deleted comments show [deleted] but thread structure preserved | ✓ SATISFIED | Soft delete: body="[deleted]", author="[deleted]", isDeleted=true. No row deletion. CommentCard shows "[deleted]" for isDeleted, children remain in flat list. |
| CMNT-06 | 04-01, 04-03, 04-04 | Deep threads lazy-loaded (top 2-3 levels initially, rest on demand) | ✓ SATISFIED | ListComments returns depth ≤ 3 (scylla.go:378). ListReplies for deeper threads (scylla.go:401). Frontend shouldShowLoadMore + handleLoadMoreReplies + [load N more replies] button. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `scylla.go` | 511 | `default: sortByWilson(comments)` — SortControversial silently falls through | ⚠️ Warning | Controversial sort returns Wilson-sorted results instead of controversial-sorted |
| `CommentSortBar.svelte` | 9-13 | Only 3 sort tabs, missing Controversial | ⚠️ Warning | Users cannot access Controversial sort from UI |

No TODO/FIXME/placeholder comments found. No empty implementations. No console.log-only handlers.

### Human Verification Required

### 1. Comment Tree Visual Rendering
**Test:** Create a post, add a top-level comment, reply to it, reply to the reply
**Expected:** 3 levels of depth-based indentation with left border lines; collapse/expand toggles work
**Why human:** Visual layout and CSS indentation behavior cannot be verified programmatically

### 2. Sort Switching Persistence
**Test:** Switch between Best/Top/New in the sort bar; refresh the page
**Expected:** Comments reorder on sort change; sort preference persists after refresh (localStorage)
**Why human:** Dynamic reordering UX and localStorage persistence across page loads

### 3. Delete Preserves Thread Structure
**Test:** Delete a comment that has replies
**Expected:** Comment shows [deleted] body and [deleted] author; child replies remain visible in tree
**Why human:** Thread structure preservation is visual

### 4. Lazy-Load Deep Replies
**Test:** Create a deep thread (4+ levels); navigate to the post
**Expected:** Only top 3 levels shown initially; [load N more replies] button appears; clicking loads deeper replies inline
**Why human:** Async lazy-loading behavior across API boundary

### 5. Vote on Comment
**Test:** Click upvote/downvote on a comment
**Expected:** VoteButtons update score instantly; Kafka consumer eventually updates ScyllaDB score
**Why human:** Real-time vote interaction + async pipeline verification

### Gaps Summary

**1 gap found — Controversial sort not implemented:**

The CMNT-04 requirement and Success Criterion 2 specify that comments should be sortable by "Best (Wilson score), Top, New, **and Controversial**". The backend has the `SortControversial` enum constant defined and the proto mapping in `mapSortOrder()`, but the `sortComments()` function in `scylla.go` has no `case SortControversial:` — it falls through to the `default` case which applies Wilson (Best) sort. The frontend `CommentSortBar.svelte` only renders 3 tabs (Best, Top, New) with no Controversial option.

**Impact:** Low — Controversial sort is a secondary sort option. All primary sorts (Best, Top, New) work correctly. The infrastructure to add it is in place (enum, mapping, sort function switch). Only needs: (1) a Controversial sort algorithm implementation, (2) a case in the switch, (3) a tab in the UI.

**All other aspects of Phase 4 are fully verified:**
- Complete backend: 6 RPCs, ScyllaDB dual-table store, materialized path ordering, Wilson score, Kafka vote consumer
- Complete infrastructure: ScyllaDB container, comment-service container, Envoy routes with correct ordering
- Complete frontend: 4 Svelte components with all interactions, mounted in post detail page
- Go binary builds, go vet passes, no anti-patterns or stubs detected
- All key links wired end-to-end

---

_Verified: 2026-03-05T04:15:00Z_
_Verifier: Claude (gsd-verifier)_
