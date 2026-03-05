---
status: awaiting_human_verify
trigger: "Profile page shows no posts/comments, notification badge count wrong, and wrong post URL pattern in notifications dropdown and search results."
created: 2026-03-05T00:00:00Z
updated: 2026-03-05T00:15:00Z
---

## Current Focus

hypothesis: All four round-2 issues fixed
test: go build ./... passed, go vet passed, frontend logic verified
expecting: All issues resolved after deploy
next_action: Await human verification

## Symptoms

expected: 
1. Profile page `/user/idityage` shows user's posts and comments with correct counts
2. Notification badge shows correct unread count (1 for 1 notification)
3. Post links in notifications and search go to `/post/{postId}`

actual:
1. Profile API returns empty arrays for posts and comments
2. Notification badge shows "2" for a single comment notification
3. Notification dropdown links go to `/community/{communityName}/post/{postId}` instead of `/post/{postId}`; search results same wrong pattern

## Round 2 Issues (from human verification)

A. Ghost notification "u/" with NaN timestamp
B. Mark-as-read not clearing badge  
C. User comments returns empty (comments_by_author not backfilled)
D. Architecture — post shard pools passed to karma consumer

## Eliminated

(No false hypotheses — all root causes confirmed on first investigation)

## Evidence

- timestamp: 2026-03-05T00:10:00Z
  checked: notification/store.go Create method
  found: Create() returns only ID via RETURNING id. Notification.CreatedAt stays zero. WS sends struct with zero CreatedAt → frontend gets "0001-01-01T00:00:00Z" → NaN in relativeTime.
  implication: Issue A root cause confirmed

- timestamp: 2026-03-05T00:10:05Z
  checked: NotificationBell.svelte, NotificationDropdown.svelte
  found: markAllRead() in Dropdown has no callback to Bell. Bell's unreadCount never zeroed.
  implication: Issue B root cause confirmed

- timestamp: 2026-03-05T00:10:10Z
  checked: comments_by_author table, GetUserComments
  found: Table is empty — only new CreateComment populates it. No backfill for existing data.
  implication: Issue C root cause confirmed

- timestamp: 2026-03-05T00:10:15Z
  checked: VoteRequest proto, VoteEvent proto, vote/server.go, vote/consumer.go
  found: VoteRequest had no author_id field. VoteEvent.author_id always empty. KarmaConsumer did postShard DB lookups to find author.
  implication: Issue D root cause confirmed

- timestamp: 2026-03-05T00:15:00Z
  checked: go build ./... and go vet ./...
  found: All code compiles clean after all fixes
  implication: Fixes are structurally correct

## Resolution

root_cause:
A. notification.Store.Create returned only ID from INSERT; in-memory Notification sent via WebSocket had zero CreatedAt and potentially empty ActorUsername
B. NotificationDropdown.markAllRead had no callback to NotificationBell to reset unreadCount
C. comments_by_author ScyllaDB table had no historical data — only new comments populated it
D. VoteRequest proto lacked author_id; vote-service left VoteEvent.author_id empty; KarmaConsumer needed post shard DB pools to look up author

fix:
A. Changed store.Create to RETURNING id, created_at and populate n.CreatedAt. Updated relativeTime() to handle invalid/zero timestamps gracefully. Updated NotificationItem to conditionally render actorUsername and timestamp.
B. Added onmarkallread and onmarkread callback props to NotificationDropdown. Wired callbacks in NotificationBell to reset/decrement unreadCount.
C. Added BackfillCommentsByAuthor() method to comment.Store (scans comments_by_post, inserts into comments_by_author with IF NOT EXISTS for idempotency). Called from comment-service startup.
D. Added author_id field (field 4) to VoteRequest proto. Updated vote-service to forward req.GetAuthorId() into VoteEvent. Updated frontend VoteButtons to accept and send authorId. Updated all VoteButtons callers (PostDetail, FeedRow, CommentCard) to pass authorId. Removed postShards from KarmaConsumer — it now expects author_id in VoteEvents. Updated cmd/user/main.go to not pass postShards to NewKarmaConsumer. Added SCYLLADB_HOSTS and SCYLLADB_KEYSPACE to user-service docker-compose.

verification: go build ./... ✓, go vet ./... ✓, frontend logic verified manually

files_changed:
- internal/notification/store.go (RETURNING created_at, populate n.CreatedAt)
- web/src/lib/time.ts (handle invalid/zero timestamps in relativeTime)
- web/src/components/notification/NotificationItem.svelte (conditional actorUsername + timestamp)
- web/src/components/notification/NotificationDropdown.svelte (onmarkallread + onmarkread callbacks)
- web/src/components/notification/NotificationBell.svelte (wire callbacks to reset/decrement unreadCount)
- internal/comment/scylla.go (BackfillCommentsByAuthor method)
- cmd/comment/main.go (call backfill on startup)
- proto/redyx/vote/v1/vote.proto (added author_id field 4 to VoteRequest)
- gen/redyx/vote/v1/vote.pb.go (regenerated)
- gen/redyx/vote/v1/vote_grpc.pb.go (regenerated)
- deploy/envoy/proto.pb (regenerated)
- internal/vote/server.go (forward req.GetAuthorId() to VoteEvent)
- internal/vote/consumer.go (removed postShards, simplified to use event author_id)
- cmd/user/main.go (removed postShards from NewKarmaConsumer call, updated comments)
- web/src/components/post/VoteButtons.svelte (added authorId prop, send in API call)
- web/src/components/post/PostDetail.svelte (pass authorId to VoteButtons)
- web/src/components/feed/FeedRow.svelte (pass authorId to VoteButtons)
- web/src/components/comment/CommentCard.svelte (pass authorId to VoteButtons)
- docker-compose.yml (added SCYLLADB_HOSTS/SCYLLADB_KEYSPACE to user-service, added scylladb dependency)
