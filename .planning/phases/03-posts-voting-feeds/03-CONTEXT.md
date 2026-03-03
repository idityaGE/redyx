# Phase 3: Posts + Voting + Feeds (Full Stack) - Context

**Gathered:** 2026-03-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can create posts (text, link, media types), vote on content (upvote/downvote/remove), and browse community feeds and a home feed aggregating joined communities — with frontend pages for post creation, feed browsing with sort controls, optimistic voting interactions, and post detail views. Protos for PostService and VoteService are already defined. Media upload/storage infrastructure is Phase 5; media post type is stubbed in Phase 3.

</domain>

<decisions>
## Implementation Decisions

### Feed layout & density
- Compact terminal-style list rows (1-2 lines per post), matching current index.astro mockup aesthetic
- Each row: vote arrows | title | metadata (community, author, time, comment count)
- Link posts show a small domain tag next to the title, e.g. `(github.com)`
- Media posts show a tiny 32-40px thumbnail at the end of the row
- Post detail page shows full content inline — title, full body (rendered markdown for text), link embed, or full media — with vote controls and metadata

### Post creation experience
- Dedicated page at `/community/{name}/submit` (full page, not modal)
- Tab bar at top: Text | Link | Media — switching tabs changes the form fields
- Text post editor: plain textarea with a separate Preview tab for rendered markdown
- Visible "Post anonymously" checkbox on the form (posts as [anonymous])
- Media tab is stubbed with "coming soon" message — gets wired up in Phase 5

### Sort controls & feed navigation
- Inline horizontal tab bar above the feed: Hot | New | Top | Rising
- When Top is selected, a secondary time filter row appears: Hour | Day | Week | Month | Year | All
- Home feed at `/` (index page), community feed at `/community/{name}` — sidebar navigation
- Infinite scroll for pagination (auto-load as user approaches bottom)
- Saved posts accessible via a "Saved" tab on user profile page + "Saved" link in sidebar

### Voting interaction feel
- Optimistic updates — score changes instantly on click, reverts on API failure
- Color change for vote state: upvote arrow turns accent color, downvote arrow turns contrasting color (red/orange), score text tints to match
- Score format: exact numbers up to 999, then compact (1.4k, 15.8k, 1.2m)
- Unauthenticated users see vote buttons but clicking triggers a "Log in to vote" prompt

### Claude's Discretion
- Loading skeleton design for feed and post detail
- Exact spacing, typography, and terminal styling details
- Error state handling (network failures, 404 posts, etc.)
- Infinite scroll threshold distance and loading indicator
- Markdown rendering library choice
- Post edit/delete confirmation UX

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/src/lib/api.ts`: API client with auth injection and 401 silent refresh — all API calls go through `api<T>(path, options)`
- `web/src/lib/auth.ts`: Auth store with pub/sub pattern (Svelte 5 compatible) — provides `getUser()`, `isAuthenticated()` for checking login state
- `web/src/layouts/BaseLayout.astro`: Layout shell with Header, Sidebar, main content area, footer, mobile nav — all new pages use this
- `web/src/components/Sidebar.svelte`: Sidebar navigation listing communities — integration point for feed switching and saved posts link
- `web/src/components/CommunityDetail.svelte`: Pattern for data-fetching Svelte components (onMount fetch, loading/error states with `$state` runes)
- `web/src/styles/terminal.css`: Terminal styling utilities (box-terminal, border-terminal, text-terminal-dim)
- `web/src/pages/index.astro`: Hardcoded feed mockup — will be replaced with real home feed component

### Established Patterns
- **Page structure**: Astro page → `BaseLayout` → Svelte component with `client:load`
- **Svelte 5 runes**: Components use `$state`, `$props`, `$derived` — not Svelte 4 stores
- **API calls**: Through `api<T>()` wrapper, not raw `fetch()` — handles auth, refresh, error extraction
- **Go services**: Each service in `internal/{name}/` + `cmd/{name}/` with gRPC server bootstrap from `internal/platform/`
- **Database**: PostgreSQL per-service with separate databases, migrations in `migrations/{service}/`
- **Cache**: Redis per-service (each on its own DB number)
- **Pagination**: Cursor-based pagination helper in `internal/platform/pagination/cursor.go`
- **Proto → REST**: Envoy gRPC-JSON transcoding via google.api.http annotations

### Integration Points
- `docker-compose.yml`: Needs new post-service and vote-service entries (following auth/user/community pattern)
- `deploy/envoy/envoy.yaml`: Needs route config for `/api/v1/posts/*`, `/api/v1/feed`, `/api/v1/votes/*`, `/api/v1/saved`
- `web/src/pages/`: New pages: post detail, submit, saved — plus updating index.astro
- `web/src/pages/community/[name].astro`: Community page needs to show the community feed with sort controls
- `web/src/components/Sidebar.svelte`: Add "Saved" link
- `web/src/components/ProfileTabs.svelte`: Add "Saved" tab
- Kafka: Not yet in the stack — needs to be added for async karma updates on vote events

</code_context>

<specifics>
## Specific Ideas

- Feed rows should match the terminal aesthetic already established in index.astro — monospace font, compact density, `text-terminal-dim` for metadata
- Domain tags on link posts styled like `(github.com)` in the terminal dim color
- Vote color states: accent color (the existing teal/green) for upvote, red/orange for downvote — matching the terminal color palette
- Markdown preview for text posts should render inside the terminal aesthetic (monospace code blocks, clean headings)

</specifics>

<deferred>
## Deferred Ideas

- Media upload/storage/CDN infrastructure — Phase 5 (media tab is stubbed in Phase 3)
- Media post creation wiring — Phase 5 (when Media service exists)

</deferred>

---

*Phase: 03-posts-voting-feeds*
*Context gathered: 2026-03-03*
