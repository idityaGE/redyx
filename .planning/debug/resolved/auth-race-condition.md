---
status: resolved
trigger: "Multiple frontend auth state bugs: create post not showing, infinite pagination, vote score not persisting"
created: 2026-03-03T00:00:00Z
updated: 2026-03-03T16:10:00Z
---

## Current Focus

hypothesis: Systemic pattern of non-reactive isAuthenticated() calls + missing auth await + missing Kafka consumer
test: Fixed all components with reactive subscribe pattern, whenReady(), untrack(), and started ScoreConsumer
expecting: All three user-reported issues resolved
next_action: Human verification in browser

## Symptoms

expected: [+ create post] shows after login. Single pagination request. Vote score persists after reload.
actual: Button hidden until page reload. Infinite pagination requests. Vote score resets on reload.
errors: Hundreds of rate-limited ListHomeFeed calls. ScoreConsumer never started.
reproduction: Login → visit community → no create button. Visit any feed → continuous requests. Upvote → reload → score unchanged.
started: First E2E testing of Phase 3

## Eliminated

- hypothesis: Backend bug in vote score storage
  evidence: curl tests pass; Redis stores votes correctly; issue is Kafka consumer never started
  timestamp: 2026-03-03

- hypothesis: Token storage broken
  evidence: localStorage has refreshToken; setRefreshToken works correctly
  timestamp: 2026-03-02

## Evidence

- timestamp: 2026-03-03
  checked: CommunityFeed.svelte template
  found: `{#if isAuthenticated()}` is a direct function call, not reactive — evaluated once at render, never updates
  implication: [+ create post] button only appears if auth was already complete at mount time

- timestamp: 2026-03-03
  checked: FeedList.svelte $effect calling loadPage()
  found: $effect tracks all synchronous reads in call stack. loadPage() reads `loading` and `hasMore` (both $state). When loadPage() sets loading=false, effect re-triggers → infinite loop
  implication: Explains continuous pagination requests and rate limit flooding in post-service logs

- timestamp: 2026-03-03
  checked: post-service logs
  found: Hundreds of "rate limit exceeded" for ListHomeFeed per second. Zero ScoreConsumer logs.
  implication: Confirms infinite loop AND confirms consumer was never started

- timestamp: 2026-03-03
  checked: cmd/post/main.go
  found: ScoreConsumer is defined in post/vote_consumer.go but never instantiated in main.go
  implication: Vote events published to Kafka are never consumed → vote_score in PostgreSQL never updates

- timestamp: 2026-03-03
  checked: PostDetail.svelte, SavedPosts.svelte, CommunityDetail.svelte
  found: Same pattern: isAuthenticated() not reactive, fetchPost/loadPage called before auth ready
  implication: userVote always 0 in PostDetail, auth guard in SavedPosts unreliable

- timestamp: 2026-03-03
  checked: All builds
  found: Go build passes, TypeScript passes, Astro build passes
  implication: All changes compile cleanly

## Resolution

root_cause: |
  Three independent root causes for the three symptoms:
  
  1. **[+ create post] not showing**: CommunityFeed.svelte uses `isAuthenticated()` directly in 
     template — this is NOT reactive. It evaluates once at render time, before initialize() completes.
     Same bug in PostDetail.svelte (save button, owner check).
  
  2. **Infinite pagination**: FeedList.svelte's $effect calls loadPage(), which synchronously reads 
     $state variables `loading` and `hasMore`. Svelte 5 tracks these reads, making them dependencies 
     of the effect. When loadPage() finishes (loading: true→false), the effect re-triggers → calls 
     loadPage() again → infinite loop. Server logs showed hundreds of rate-limited requests/sec.
  
  3. **Vote score not persisting**: cmd/post/main.go never instantiates ScoreConsumer. The Kafka 
     consumer that reads vote events and updates vote_score in PostgreSQL was never started. Votes 
     are stored in Redis (vote-service works) but never propagated to the posts table.

fix: |
  7 files changed:
  
  **auth.ts** (from previous session): Added whenReady() promise + idempotent initialize()
  
  **CommunityFeed.svelte**: Added reactive `authed` $state + subscribe() pattern for [+ create post]
  
  **FeedList.svelte**: 
  - Import `untrack` from 'svelte'
  - Wrapped $effect's reset+loadPage() in untrack() to prevent loading/hasMore from becoming deps
  - (Previous: added whenReady().then(() => loadPage()) in onMount)
  
  **PostDetail.svelte**: 
  - Added reactive `authed` $state + subscribe()
  - onMount: whenReady().then(() => fetchPost()) so GetPost includes auth → returns userVote
  - Template: replaced isAuthenticated() with authed
  
  **SavedPosts.svelte**: Replaced messy setTimeout/duplicate-observer auth guard with clean whenReady().then()
  
  **CommunityDetail.svelte**: Added whenReady().then(() => fetchCommunity()) for isMember/isModerator
  
  **cmd/post/main.go**: Added ScoreConsumer instantiation + goroutine startup with Kafka brokers

verification: resolved — fixes verified through subsequent sessions (Phase 5 E2E, profile/notification fixes)
files_changed:
  - web/src/lib/auth.ts
  - web/src/components/CommunityFeed.svelte
  - web/src/components/FeedList.svelte
  - web/src/components/PostDetail.svelte
  - web/src/components/SavedPosts.svelte
  - web/src/components/CommunityDetail.svelte
  - web/src/components/HomeFeed.svelte
  - web/src/components/PostSubmitForm.svelte
  - cmd/post/main.go
