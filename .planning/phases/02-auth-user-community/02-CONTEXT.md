# Phase 2: Auth + User + Community (Full Stack) - Context

**Gathered:** 2026-03-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Users can create accounts, log in, manage profiles, and create/join communities — with working frontend pages for all auth flows, profile views, and community management. Covers requirements AUTH-01 through AUTH-08, USER-01 through USER-05, COMM-01 through COMM-07, and RATE-01 through RATE-04. Proto definitions for AuthService (7 RPCs), UserService (5 RPCs), CommunityService (9 RPCs), and RateLimitService (1 RPC) are already defined. This phase builds the backend implementations and all corresponding frontend pages.

</domain>

<decisions>
## Implementation Decisions

### Auth flow experience
- Terminal prompt style for registration and login pages — centered minimal form with monospace labels, ASCII box border, blinking cursor feel. Matches the established terminal aesthetic from Phase 1
- Separate `/verify` page for OTP verification after registration — distinct URL so users can return if they close the browser. Not an in-place transition
- Inline terminal-style error messages below each field — red/orange monospace text styled like stderr output (e.g. `> error: invalid credentials`). No toast notifications or banner errors on auth pages
- Dedicated `/choose-username` page after Google OAuth callback for new users — clean separation from the OAuth flow. Redirect to home after username is set
- Password reset follows the two-step proto flow (email to initiate, token + new password to complete) — presented as two separate terminal-style pages

### Profile page presentation
- Tabbed sections for profile content: Posts | Comments | About — each tab shows a paginated list. Tab navigation styled like terminal multiplexer tabs
- Compact status line for profile header: `u/username | 1,234 karma | joined 2026-01-15` — single line, dense, no large hero section
- Avatar displayed as small square thumbnail (32-48px) with ASCII box-drawing border — falls back to first letter of username if no avatar uploaded
- Inline editing on own profile page — click-to-edit for display name, bio, and avatar. No separate /settings route for profile editing
- Bio limited to 500 chars per USER-04. Markdown rendering in the About tab

### Community pages & discovery
- Right sidebar info panel on community detail page — description, rules list, member count, mod list, join/leave button. Main content area reserved for post feed (empty until Phase 3). Sidebar uses box-drawing borders matching terminal aesthetic
- Single-page form for community creation — all fields on one page: name, description, visibility (public/restricted/private), rules. No multi-step wizard. Fast and simple
- Browse page at `/communities` with sorting by member count, creation date, or activity — communities displayed in a list/table format fitting the information-dense terminal style
- Dedicated settings page per community at `/community/{name}/settings` — sections for description editing, rules management (add/remove/reorder), visibility toggle, moderator assignment/revocation. Only accessible to community owner and moderators

### Logged-in state & session edges
- Header: `u/username` replaces `[anonymous]` when authenticated — clicking username opens a dropdown menu with Profile, Settings, Logout options. Notification diamond gets an unread count badge (placeholder until Phase 5)
- Silent JWT refresh — automatically use refresh token to get new access token when access token expires. User never sees interruption. Only redirect to `/login` when refresh token is also expired (7 days)
- Full read access for anonymous users — can browse communities, read posts, view profiles without authentication. Voting, posting, commenting, joining communities, and community creation require login. Gentle terminal-style prompt when attempting restricted actions (e.g. `> login required: /login`)
- Sidebar keeps static shortcuts (Home, Popular, All, Saved) at top + adds a "My Communities" section below with the user's joined communities when authenticated. Tree-drawing characters (├── , └──) maintained. Anonymous users see shortcuts only

### Claude's Discretion
- Loading skeleton/spinner design within the terminal aesthetic for auth operations
- Exact form field validation timing (on-blur vs on-submit)
- Password strength indicator design (if any)
- Exact spacing and padding values within auth forms
- Mobile responsiveness details for auth pages (bottom tab bar hides on auth pages or stays)
- Account deletion confirmation flow (how many steps, what warnings)
- Community rules editor UX details (inline text fields vs textarea)
- Rate limit error display (429 response presentation to user)

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- **BaseLayout** (`web/src/layouts/BaseLayout.astro`): Full application shell with header, sidebar, mobile nav, theme toggle. Auth pages and new pages slot into this layout
- **Header** (`web/src/components/Header.astro`): Currently shows `[anonymous]` — needs to become auth-aware Svelte component for dynamic username/dropdown
- **Sidebar** (`web/src/components/Sidebar.astro`): Has tree-drawing char pattern for community links — needs to become dynamic with joined communities section
- **MobileNav** (`web/src/components/MobileNav.svelte`): Svelte 5 component with 5 tabs — Profile tab needs auth-aware routing
- **ThemeToggle** (`web/src/components/ThemeToggle.svelte`): Svelte 5 runes pattern — reference for building other interactive components
- **Terminal CSS utilities** (`web/src/styles/terminal.css`): `border-terminal`, `box-terminal`, `text-terminal-dim`, `bg-terminal`, `bg-terminal-surface` — use for all new pages
- **Tailwind theme tokens** (`web/tailwind.css`): Full accent color palette (orange), terminal semantic colors, JetBrains Mono font stack — all auth/profile/community pages use these
- **Platform errors** (`internal/platform/errors/errors.go`): `ErrUnauthenticated`, `ErrForbidden`, `ErrNotFound`, `ErrAlreadyExists`, `ErrInvalidInput` — maps to gRPC status codes via middleware
- **Pagination helpers** (`internal/platform/pagination/cursor.go`): Cursor encode/decode for profile post/comment history lists
- **gRPC server bootstrap** (`internal/platform/grpcserver/server.go`): Functional options pattern — auth interceptor slots into the middleware chain
- **Config loader** (`internal/platform/config/config.go`): Environment-based config with local-dev defaults — extend for auth service config (JWT secret, OTP TTL, OAuth credentials)

### Established Patterns
- **Service structure**: `cmd/{service}/main.go` entry point + `internal/{service}/server.go` implementation
- **Middleware chain order**: Recovery -> Logging -> ErrorMapping -> (auth interceptor goes here)
- **Frontend components**: Astro for static, Svelte 5 runes (`$state`, `$derived`) for interactive (`client:load` or `client:visible`)
- **Styling**: TailwindCSS v4 with `@theme` directive, custom `@utility` rules, dark mode default
- **Database**: pgx/v5 raw SQL with `pgxpool.Pool` — no ORM. Migrations in `migrations/{service}/NNN_{name}.{up,down}.sql`
- **Redis**: go-redis/v9 — will be used for OTP storage, refresh token blacklist, rate limiting token bucket
- **Proto-first API**: All RPCs defined in proto with `google.api.http` annotations, Envoy transcodes REST to gRPC
- **Envoy config**: Needs new service(s) registered in transcoder services list at `deploy/envoy/envoy.yaml`
- **Docker Compose**: New service containers added to `docker-compose.yml` with health checks and dependency ordering
- **JSON field naming**: camelCase via `preserve_proto_field_names: false` in Envoy transcoder config

### Integration Points
- **Envoy transcoder**: Must register `AuthService`, `UserService`, `CommunityService`, `RateLimitService` in the proto descriptor services list
- **Envoy routes**: Currently single `/api/v1/` prefix route to skeleton-service — needs routing to auth/user/community services
- **Docker Compose**: Add auth-service (and potentially user-service, community-service) containers with PostgreSQL database(s)
- **Frontend API calls**: No API client exists yet — first fetch/API layer needs to be established (auth token injection, error handling, response parsing)
- **Proto descriptor**: `make proto` rebuilds `deploy/envoy/proto.pb` — already includes auth/user/community service definitions from Phase 1

</code_context>

<specifics>
## Specific Ideas

- Auth pages should feel like a CLI login prompt — user types credentials into a terminal-like form, not a polished SaaS signup page
- The `[anonymous]` -> `u/username` transition in the header should be the primary visual indicator of auth state — keep it subtle, not a dramatic UI change
- Community sidebar section should use the same tree-drawing character pattern (├── , └──) already established for navigation items
- Profile status line (`u/username | karma | joined`) echoes the information-dense, single-line status bar pattern from the footer (`redyx v0.1.0 | [connected]`)
- Browse communities page should feel like listing directories — information-dense table/list, not cards or tiles

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 02-auth-user-community*
*Context gathered: 2026-03-03*
