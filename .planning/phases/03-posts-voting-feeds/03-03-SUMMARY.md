---
phase: 03-posts-voting-feeds
plan: 03
subsystem: infra
tags: [kafka, docker-compose, envoy, franz-go, redis, sharding, grpc-transcoding]

# Dependency graph
requires:
  - phase: 03-posts-voting-feeds
    provides: "Post service with ShardRouter, vote service with Redis store and Kafka producer, KarmaConsumer module"
provides:
  - "Docker Compose stack with Kafka broker, post-service, and vote-service containers"
  - "Envoy routing for all post/vote/feed/saved REST endpoints"
  - "Post-service ScoreConsumer (Kafka → Redis SCARD → shard DB update)"
  - "User-service karma consumer goroutine integration"
affects: [03-posts-voting-feeds, 04-comments-threading]

# Tech tracking
tech-stack:
  added: [bitnami/kafka:3.7]
  patterns: [kafka-kraft-mode, set-based-idempotent-score-update, cross-service-redis-read]

key-files:
  created:
    - internal/post/vote_consumer.go
  modified:
    - cmd/user/main.go
    - docker-compose.yml
    - deploy/envoy/envoy.yaml

key-decisions:
  - "ScoreConsumer reads authoritative vote counts from Redis SCARD (set-based, naturally idempotent)"
  - "Kafka in KRaft mode — no Zookeeper dependency"
  - "Community posts regex route before community catch-all in Envoy"
  - "Karma consumer runs as goroutine in user-service with graceful shutdown"

patterns-established:
  - "Cross-service Redis read: post-service reads vote-service Redis DB 5 for authoritative vote counts"
  - "Kafka consumer goroutine: embed consumer in existing service process, cancel context on shutdown"

requirements-completed: [VOTE-04, POST-10]

# Metrics
duration: 5min
completed: 2026-03-03
---

# Phase 3 Plan 03: Infrastructure Wiring Summary

**Kafka broker (KRaft), post-service and vote-service Docker Compose containers, Envoy routes for all post/vote/feed/saved endpoints, set-based idempotent score consumer, and user-service karma consumer integration**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-03T14:32:41Z
- **Completed:** 2026-03-03T14:37:58Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- ScoreConsumer in post-service reads authoritative vote counts from Redis SCARD (naturally idempotent), updates vote_score/upvotes/downvotes/hot_score on the correct shard
- Docker Compose stack includes Kafka broker (bitnami/kafka:3.7 KRaft mode), post-service (dual shard DSNs, Redis DB 4), and vote-service (Redis-only DB 5)
- Envoy routes all post/vote/feed/saved endpoints to correct backends, with regex route for community posts before community catch-all
- User-service starts karma consumer goroutine alongside gRPC server with graceful shutdown

## Task Commits

Each task was committed atomically:

1. **Task 1: Post-service score consumer + user-service karma consumer integration** - `fa903d6` (feat)
2. **Task 2: Docker Compose additions + Envoy route config** - `34e1adb` (feat)

## Files Created/Modified
- `internal/post/vote_consumer.go` - ScoreConsumer that reads Redis SCARD for idempotent score updates, routes to correct shard
- `cmd/user/main.go` - KarmaConsumer goroutine integration with graceful shutdown
- `docker-compose.yml` - Kafka broker, post-service, vote-service containers, user-service Kafka env
- `deploy/envoy/envoy.yaml` - Routes for posts/feed/saved/votes/community-posts, clusters for post-service and vote-service, transcoder registration

## Decisions Made
- **Set-based idempotent score update:** ScoreConsumer uses Redis SCARD (set cardinality) for upvotes/downvotes rather than delta-based updates — naturally idempotent per RESEARCH.md recommendation
- **Kafka KRaft mode:** bitnami/kafka:3.7 with KRaft (no Zookeeper) — simpler infrastructure, fewer containers
- **Community posts regex route:** `/api/v1/communities/[^/]+/posts.*` regex route placed before `/api/v1/communities` catch-all to correctly route ListPosts to post-service
- **Karma consumer as goroutine:** Embedded in user-service process (not a separate container) — uses context cancellation for clean shutdown

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Full async vote→score→karma pipeline is wired: vote-service produces → post-service consumes (score) → user-service consumes (karma)
- All services accessible via REST through Envoy gateway
- Ready for Phase 3 frontend plans (post creation, feed browsing, voting UI)

## Self-Check: PASSED

All 1 created file verified on disk. All 2 task commits found in git history. All 3 modified files verified.

---
*Phase: 03-posts-voting-feeds*
*Completed: 2026-03-03*
