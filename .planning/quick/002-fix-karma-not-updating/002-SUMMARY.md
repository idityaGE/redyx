---
phase: quick
plan: 002
subsystem: vote/karma
tags: [bugfix, karma, vote-consumer, postgres]
dependency_graph:
  requires: [internal/vote/kafka.go, internal/vote/server.go, internal/platform/config/config.go]
  provides: [working-karma-pipeline]
  affects: [user-profiles, user-service]
tech_stack:
  added: []
  patterns: [cross-shard-lookup, fallback-on-empty-field]
key_files:
  created: []
  modified:
    - internal/vote/consumer.go
    - cmd/user/main.go
    - docker-compose.yml
decisions:
  - "Keep vote RPC fast (fire-and-forget) — consumer does the author lookup instead"
  - "Query all post shards sequentially for author lookup (2 shards, negligible latency)"
  - "Graceful degradation: warn on shard connection failure, continue with available shards"
metrics:
  duration: 98s
  completed: 2026-03-05
---

# Quick Task 002: Fix Karma Not Updating Summary

Fixed 3-link bug chain preventing karma from ever updating: vote-service publishes empty author_id, consumer skipped all such events, and SQL targeted wrong table/column.

## What Changed

### Bug Chain (Root Cause)

1. **Vote-service** publishes `VoteEvent` with `AuthorId: ""` (by design — keeps RPC fast)
2. **Karma consumer** skipped all events with empty `author_id` (line 91-95) — every event was dropped
3. **SQL was wrong**: `UPDATE users SET karma = karma + $1 WHERE id = $2` — table is `profiles`, column is `user_id`

### Fixes Applied

**Fix 1 — Post shard author lookup (consumer.go):**
- Added `postShards []*pgxpool.Pool` field to `KarmaConsumer` struct
- Added `lookupAuthorID()` method that queries post shard DBs: `SELECT author_id FROM posts WHERE id = $1`
- Replaced early-return skip logic with fallback lookup in `processEvent`

**Fix 2 — Correct SQL target (consumer.go):**
- Changed `UPDATE users SET karma = karma + $1 WHERE id = $2` to `UPDATE profiles SET karma = karma + $1 WHERE user_id = $2`

**Fix 3 — Wiring in user-service (main.go):**
- Connect to post shard databases from `cfg.PostShardDSNs` with proper error handling
- Pass `postShards` slice to `vote.NewKarmaConsumer()`
- Defer cleanup of all shard pool connections

**Fix 4 — Docker environment (docker-compose.yml):**
- Added `POST_SHARD_DSNS` env var to `user-service` container

## Tasks

| # | Task | Status | Commit | Key Files |
|---|------|--------|--------|-----------|
| 1 | Fix karma consumer — add post-shard author lookup and correct SQL | Done | e38e3f5 | internal/vote/consumer.go, cmd/user/main.go, docker-compose.yml |
| 2 | Verify karma updates end-to-end | Pending human verification | — | — |

## Verification

- `go build ./cmd/user/...` — passes
- `go vet ./internal/vote/... ./cmd/user/...` — passes
- SQL pattern `UPDATE profiles.*karma.*WHERE user_id` — confirmed in consumer.go:175
- `SELECT author_id FROM posts` pattern — confirmed in consumer.go:128
- `postShards []*pgxpool.Pool` in struct — confirmed in consumer.go:30
- `POST_SHARD_DSNS` in docker-compose.yml user-service — confirmed
- ScoreConsumer untouched — only 3 files modified (consumer.go, main.go, docker-compose.yml)

## Deviations from Plan

None — plan executed exactly as written.

## Human Verification Needed (Task 2)

End-to-end verification of the karma pipeline requires running services:

1. Run `docker compose up -d --build user-service vote-service post-service kafka redis postgres`
2. Wait for services: `docker compose logs user-service --tail 20` — look for "karma consumer started"
3. Log in as user A, create a post in any community
4. Log in as user B, upvote user A's post
5. Check user A's profile — karma should show 1 (not 0)
6. User B downvotes then re-upvotes — karma should still be 1 (dedup works)
7. Check logs: `docker compose logs user-service --tail 50 | grep karma` — should show processing, not skipping

## Self-Check: PASSED
