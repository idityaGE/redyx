# Phase 5: Search + Notifications + Media (Full Stack) - Context

**Gathered:** 2026-03-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can search posts by title/body (globally or within a community), receive real-time WebSocket notifications when someone replies to their post/comment or mentions them, and upload images/videos when creating a post. Frontend includes search bar with autocomplete, search results page, notification bell with dropdown, notification preferences page, and media upload component with drag-and-drop.

Proto definitions already exist for all three services (search, notification, media). No backend service implementations exist yet. No WebSocket infrastructure exists yet.

</domain>

<decisions>
## Implementation Decisions

### Search experience
- **Inline dropdown + results page:** Autocomplete community suggestions appear in a dropdown below the existing Header search bar as the user types (2+ chars). Pressing Enter navigates to a dedicated `/search?q=...` results page.
- **Context-aware scoping:** When on a community page, search auto-scopes to that community with a visible removable pill/tag. From global pages (home, etc.), search is global. Users can remove the community scope pill to search globally.
- **Feed-row style results:** Search results reuse the FeedRow layout pattern — compact single-line title + metadata row (author, community, timestamp, score, comment count). Highlighted snippet shown as secondary text below the title.
- **Minimal empty state:** Simple "No results found" message, terminal-style. No elaborate suggestions.

### Notification panel & real-time behavior
- **Dropdown panel:** Clicking the bell icon opens a dropdown below it showing recent notifications (10-20 most recent). "View all" link at the bottom navigates to a full notifications page.
- **Chronological list:** All notification types (post_reply, comment_reply, mention) in a single reverse-chronological list. Type indicated by icon or prefix text.
- **Silent badge updates:** When a new notification arrives via WebSocket, the unread count badge on the bell updates silently. If the dropdown is open, the new notification appears at the top. No toasts, no sounds.
- **Click to navigate + auto-mark:** Clicking a notification navigates to the relevant post/comment and marks it as read. A "mark all read" button at the top of the dropdown.

### Media upload flow
- **Multiple files (up to 4-5 images OR 1 video):** Users can attach multiple images, or one video — no mixing video with images in a single post. Proto's `mediaUrls` array on posts supports this.
- **Drag-and-drop zone:** Dashed border area in the media tab with terminal-style text like `[ drop files here or click to browse ]`. Click fallback opens native file picker.
- **Per-file progress bar:** Each uploading file shows a terminal-style progress bar like `[=========>   ] 78%` with file name and size.
- **Thumbnail previews:** After upload, small thumbnails shown below the upload area. Each with an `[x]` to remove. Video shows first frame.
- **Submit during processing:** Post can be submitted while media is still processing (PENDING/PROCESSING status). Post displays a processing indicator that resolves to final media once READY. Like Twitter.
- **One video, no mixing:** If user selects a video, it takes the full media slot. Cannot combine video + images.
- **Inline per-file errors:** Failed uploads show red text next to the file entry: `> error: file too large (23MB / 20MB max)`. User can remove and retry.
- **Stacked images with lightbox:** In post detail view, images display stacked vertically. Clicking opens a fullscreen lightbox with prev/next navigation.
- **No AWS dependency in dev:** Local development must work without real S3. Use MinIO (S3-compatible, Docker) or equivalent for presigned-URL flow. Researcher should confirm best option.

### Claude's Discretion
- Notification preferences page layout and interaction patterns (proto defines the toggles)
- Search results page sort controls implementation (relevance, recency, score — proto supports all)
- Autocomplete dropdown styling and animation
- Lightbox component implementation details
- WebSocket reconnection strategy and offline notification delivery mechanism
- Loading skeletons and transition animations for all three features
- Exact terminal-aesthetic styling for new components

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Header.astro` (`web/src/components/Header.astro`): Already has a non-functional search input and notification bell placeholder — both need to be replaced with interactive Svelte components
- `FeedRow.svelte` (`web/src/components/feed/FeedRow.svelte`): Already handles `POST_TYPE_MEDIA` with thumbnail rendering — search results should reuse this layout pattern
- `PostSubmitForm.svelte` (`web/src/components/post/PostSubmitForm.svelte`): Has a media tab stub ("coming in phase 5") — replace with actual upload component
- `api()` wrapper (`web/src/lib/api.ts`): Fetch wrapper with auth token injection and 401 refresh — use for all new API calls
- `relativeTime()` (`web/src/lib/time.ts`): Time formatting utility — use for notification timestamps

### Established Patterns
- **Backend:** Go microservices with gRPC, Envoy as API gateway (gRPC-JSON transcoding), Kafka for async events, PostgreSQL + Redis per service, ScyllaDB for comments
- **Frontend:** Astro SSR + Svelte 5 interactive components, Tailwind CSS v4 with terminal/hacker aesthetic (monospace font, green accent, `border-terminal-border`, `bg-terminal-surface`, `text-terminal-dim`)
- **Proto-first API design:** All service contracts defined in proto files, generated Go code in `gen/`, HTTP routes via `google.api.http` annotations
- **Service pattern:** Each service has `cmd/{service}/main.go` + `internal/{service}/server.go`

### Integration Points
- **Envoy config** (`deploy/envoy/envoy.yaml`): Needs new clusters for search, notification, media services + WebSocket upgrade support for notifications
- **Docker Compose** (`docker-compose.yml`): Needs new service entries for search, notification, media services + MinIO for object storage
- **Kafka topics:** Notification events need to be published from comment-service (on reply) and from a mention-detection pipeline
- **Proto descriptor** (`deploy/envoy/proto.pb`): Needs regeneration to include search, notification, media service routes
- **Post proto/service:** Post creation needs to accept `media_ids` to associate uploaded media with a post

</code_context>

<specifics>
## Specific Ideas

- Search should feel like a terminal command: the `>` prefix in the search bar reinforces this (already in Header.astro)
- Media upload progress bars should use terminal-style ASCII: `[=========>   ] 78%`
- The notification bell already uses a diamond character `&#9830;` — keep this as the icon, add a count badge next to it
- User specifically asked about no-AWS-in-dev: confirm MinIO works for presigned URL flow and document Docker setup

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-search-notifications-media-full-stack*
*Context gathered: 2026-03-05*
