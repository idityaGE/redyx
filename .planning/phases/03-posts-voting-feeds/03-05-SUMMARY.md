---
phase: 03-posts-voting-feeds
plan: 05
subsystem: ui
tags: [svelte, astro, feed-pages, post-detail, post-submit, markdown-preview, voting]

# Dependency graph
requires:
  - phase: 01-foundation-frontend-shell
    provides: Astro+Svelte shell, BaseLayout, terminal CSS
  - phase: 02-auth-user-community
    provides: api.ts fetch wrapper, auth.ts state management
  - phase: 03-posts-voting-feeds
    provides: VoteButtons, FeedRow, FeedList, SortBar, PostBody, relativeTime from Plan 04
provides:
  - Home feed page replacing hardcoded mockup with real API-driven feed
  - Community feed integration into existing community detail page
  - Post submit page with text/link/media tabs and markdown preview
  - Post detail page with full content, voting, save/unsave, edit/delete
affects: [04-comments, 05-media]

# Tech tracking
tech-stack:
  added: []
  patterns: [page-composition-with-svelte-islands, auth-guard-redirect, inline-edit-mode, delete-confirmation-pattern]

key-files:
  created:
    - web/src/components/HomeFeed.svelte
    - web/src/components/CommunityFeed.svelte
    - web/src/components/PostSubmitForm.svelte
    - web/src/components/PostDetail.svelte
    - web/src/pages/community/[name]/submit.astro
    - web/src/pages/post/[id].astro
  modified:
    - web/src/pages/index.astro
    - web/src/components/CommunityDetail.svelte

key-decisions:
  - "HomeFeed and CommunityFeed compose SortBar+FeedList with local $state sort management"
  - "Post submit uses tab bar (Text/Link/Media) with Write/Preview sub-tabs for markdown body"
  - "PostDetail inline edit mode toggles form fields in-place (no separate page)"
  - "Delete uses inline yes/no confirmation pattern (not browser confirm dialog)"

patterns-established:
  - "Page-level Svelte island pattern: Astro page → BaseLayout → Svelte component with client:load"
  - "Auth guard on mount: check isAuthenticated() → redirect to /login?redirect= if not"
  - "Optimistic save/unsave toggle with rollback on API failure"
  - "Inline edit mode with $state toggle for own-content editing"

requirements-completed: [POST-01, POST-02, POST-03, POST-07, POST-08]

# Metrics
duration: 6min
completed: 2026-03-03
---

# Phase 3 Plan 5: Frontend Pages for Posts and Feeds Summary

**Home feed, community feed, post submit form (text/link/media tabs with markdown preview), and post detail page (voting, save, edit/delete) — replacing all hardcoded mockups with real API-driven Svelte components**

## Performance

- **Duration:** 6 min
- **Started:** 2026-03-03T14:42:33Z
- **Completed:** 2026-03-03T14:48:42Z
- **Tasks:** 3
- **Files modified:** 8

## Accomplishments
- Home page (index.astro) replaced hardcoded mockup data with real HomeFeed component using SortBar + FeedList for /api/v1/feed with infinite scroll
- Community detail page now shows CommunityFeed with sort controls and create-post button instead of "coming in phase 3" placeholder
- Post submit page at /community/{name}/submit with text/link/media tab bar, markdown Write/Preview sub-tabs, URL domain preview, anonymous checkbox, and media stub
- Post detail page at /post/{id} with full content display (rendered markdown for text, clickable URL for links), VoteButtons, save/unsave toggle, inline edit/delete for own posts, and proper error/loading states

## Task Commits

Each task was committed atomically:

1. **Task 1: Home feed page and community feed integration** - `916b738` (feat)
2. **Task 2: Post submit page with text/link/media tabs** - `726b869` (feat)
3. **Task 3: Post detail page with voting, edit/delete, save** - `6be7960` (feat)
4. **Fix: HomeFeed auth prompt and min_lines** - `87c5133` (fix)

## Files Created/Modified
- `web/src/components/HomeFeed.svelte` - Home feed composing SortBar + FeedList for /feed endpoint, welcome header, auth prompt for anon users
- `web/src/components/CommunityFeed.svelte` - Community feed composing SortBar + FeedList for /communities/{name}/posts, create post button for authenticated users
- `web/src/components/PostSubmitForm.svelte` - Post creation form with Text/Link/Media tabs, markdown Write/Preview, title counter, URL domain preview, anonymous checkbox, auth guard
- `web/src/components/PostDetail.svelte` - Full post detail with VoteButtons, PostBody markdown rendering, save/unsave toggle, inline edit mode, delete confirmation, error/loading states
- `web/src/pages/index.astro` - Replaced hardcoded feedItems array with HomeFeed Svelte island
- `web/src/pages/community/[name]/submit.astro` - Post submit page routing to PostSubmitForm
- `web/src/pages/post/[id].astro` - Post detail page routing to PostDetail
- `web/src/components/CommunityDetail.svelte` - Replaced placeholder with CommunityFeed component

## Decisions Made
- HomeFeed and CommunityFeed both manage sort state locally with $state (consistent with Plan 04 component-local state pattern)
- Post submit form uses tab bar matching terminal multiplexer style from ProfileTabs/SortBar (bg-terminal-bg, accent border for active)
- PostDetail uses inline edit mode (not separate page) for consistency with terminal workflow feel
- Delete confirmation uses inline yes/no text buttons instead of browser confirm() dialog for terminal aesthetic

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added auth prompt for anonymous users on home feed**
- **Found during:** Task 1 (HomeFeed creation)
- **Issue:** Plan mentioned handling unauthenticated users gracefully, component initially had no guidance for anonymous users
- **Fix:** Added login/register links in welcome header for unauthenticated visitors
- **Files modified:** web/src/components/HomeFeed.svelte
- **Verification:** Build passes, component meets min_lines requirement
- **Committed in:** 87c5133

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Minor addition for better UX with anonymous users. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All frontend pages for posts and feeds are complete
- Ready for Plan 06 (saved posts page) and Plan 07 (integration testing/polish)
- Components integrate with Plan 04 reusable components (VoteButtons, FeedList, SortBar, PostBody)
- All pages use Svelte 5 runes ($state, $props, $derived) and existing api.ts/auth.ts wrappers

## Self-Check: PASSED

- All 8 key files exist on disk
- All 4 commits verified in git log
- All min_lines requirements met (36, 36, 242, 362)
- Build passes with no errors

---
*Phase: 03-posts-voting-feeds*
*Completed: 2026-03-03*
