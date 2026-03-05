---
phase: quick
plan: 004
type: execute
wave: 1
depends_on: []
files_modified:
  - web/src/components/community/CommunityList.svelte
  - web/src/components/community/MyCommunities.svelte
  - web/src/components/layout/UserDropdown.svelte
  - web/src/components/layout/Sidebar.svelte
  - web/src/pages/communities/index.astro
  - web/src/pages/my-communities/index.astro
autonomous: true
requirements: [QUICK-004]
must_haves:
  truths:
    - "/communities page shows ALL communities with a search bar"
    - "/my-communities page shows only communities the user has joined"
    - "Activity sort option does not exist anywhere"
    - "User dropdown says 'my communities' and links to /my-communities"
    - "Sidebar has a link to all communities page"
  artifacts:
    - path: "web/src/components/community/MyCommunities.svelte"
      provides: "My communities list using ListUserCommunities API"
    - path: "web/src/pages/my-communities/index.astro"
      provides: "My communities page route"
  key_links:
    - from: "web/src/components/community/MyCommunities.svelte"
      to: "/api/v1/users/{user_id}/communities"
      via: "api() call with userId from auth store"
      pattern: "api.*users.*communities"
    - from: "web/src/components/community/CommunityList.svelte"
      to: "/api/v1/communities?query="
      via: "api() call with search query param"
      pattern: "api.*communities.*query"
---

<objective>
Fix the frontend communities experience: create separate "all communities" (with search) and "my communities" (user's joined communities) views, remove the broken "activity" sort, update navigation links.

Purpose: The current /communities page conflates "browse all" with "my communities" — sort tabs imply user-specific filtering but actually show all communities. This separates concerns into two clear pages.
Output: Two distinct pages (/communities for browsing all, /my-communities for user's joined), updated nav links.
</objective>

<execution_context>
@./.opencode/get-shit-done/workflows/execute-plan.md
@./.opencode/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md

Key API contracts:
- `GET /api/v1/communities?pagination.limit=25&query={search}` — lists all communities, supports search by name
- `GET /api/v1/users/{user_id}/communities` — lists communities user has joined (returns `UserCommunity[]` with `communityId` and `name`)
- `GET /api/v1/communities/{name}` — returns community detail with `isMember` and `isModerator` flags

Auth store (`web/src/lib/auth.ts`):
- `getUser()` returns `{ userId, username, email?, avatarUrl? } | null`
- `isAuthenticated()`, `isLoading()`, `whenReady()`, `subscribe()`

@web/src/components/community/CommunityList.svelte
@web/src/components/layout/UserDropdown.svelte
@web/src/components/layout/Sidebar.svelte
@web/src/pages/communities/index.astro
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix CommunityList (all communities page) — add search, remove activity sort</name>
  <files>web/src/components/community/CommunityList.svelte</files>
  <action>
Modify CommunityList.svelte to serve as the "all communities" browse page:

1. **Remove "activity" sort option**: Change the sort type from `'members' | 'created' | 'activity'` to `'members' | 'created'`. Remove 'activity' from the `{#each ['members', 'created', 'activity']}` array, leaving just `['members', 'created']`. Update the `changeSort` function signature accordingly.

2. **Add search bar**: Add a search input above the sort controls. Style it in terminal aesthetic matching the project:
   - Monospace font, terminal border, small text (text-xs)
   - Prefix with `>` prompt character in terminal-dim color
   - Placeholder: "search communities..."
   - Bind to a `searchQuery` $state variable
   - On input (debounced ~300ms), call `fetchCommunities()` which passes the query to the API

3. **Update `fetchCommunities` to use search**: When `searchQuery` is non-empty, append `&query=${encodeURIComponent(searchQuery)}` to the API path. Reset pagination cursors when search query changes.

4. **Update page header**: Change the header from `~ /communities` to `~ /all-communities` or keep as `~ /communities` but add subtitle text "browse all communities" in terminal-dim.

Implementation details for debounce: Use a simple `setTimeout`/`clearTimeout` pattern with a module-level `let debounceTimer: ReturnType<typeof setTimeout>` variable. On search input change, clear existing timer and set new 300ms timer that calls `fetchCommunities()`.
  </action>
  <verify>
    <automated>cd web && npx astro check 2>&1 | tail -20</automated>
  </verify>
  <done>
    - /communities page shows all communities with a working search bar
    - Only "members" and "created" sort options exist (no "activity")
    - Typing in search filters communities via API query param
    - Search resets pagination
  </done>
</task>

<task type="auto">
  <name>Task 2: Create MyCommunities component and /my-communities page</name>
  <files>web/src/components/community/MyCommunities.svelte, web/src/pages/my-communities/index.astro</files>
  <action>
**Create `web/src/pages/my-communities/index.astro`:**
```astro
---
import BaseLayout from '../../layouts/BaseLayout.astro';
import MyCommunities from '../../components/community/MyCommunities.svelte';
---
<BaseLayout title="My Communities">
  <MyCommunities client:load />
</BaseLayout>
```

**Create `web/src/components/community/MyCommunities.svelte`:**

This component shows only communities the authenticated user has joined, using the `ListUserCommunities` API.

Structure (follow CommunityList.svelte patterns for terminal aesthetic):

1. **Script section:**
   - Import `onMount` from svelte, `api`/`ApiError` from `../../lib/api`, auth functions from `../../lib/auth`
   - Types: reuse same `Community` type shape. The ListUserCommunities endpoint returns `{ communities: UserCommunity[] }` where UserCommunity has `communityId` and `name` only — so for each, fetch full detail via `GET /communities/{name}` to get memberCount, description, etc. OR keep it lightweight with just names.
   - Better approach for performance: fetch user communities list (`/users/{userId}/communities?pagination.limit=100`), then for display use just name (the UserCommunity response has communityId + name). For richer display, batch-fetch details only if needed. Start with lightweight list showing just community names with links.
   - State: `let userCommunities = $state<{communityId: string; name: string}[]>([])`, `let loading`, `let error`, `let authed`, `let authLoading`
   - `fetchMyCommunities()`: calls `api<{communities: {communityId: string; name: string}[]}>(`/users/${getUser()!.userId}/communities?pagination.limit=100`)`
   - `onMount`: use `whenReady()` pattern, then if authenticated fetch communities. Subscribe to auth changes.

2. **Template section:**
   - Header box: `~ /my-communities` with `[+ create community]` link (same as CommunityList)
   - If not authenticated: show message "log in to see your communities" with link to `/login`
   - If authenticated but no communities: "you haven't joined any communities yet." with link to `/communities` ("browse communities →")
   - Community list: terminal-style rows with tree characters (├── / └──), each linking to `/community/{name}`. Show `r/{name}` for each. Since UserCommunity is lightweight (no memberCount), keep rows simple — just name with link.
   - No sort controls needed (simple list of user's communities)
   - No pagination needed for v1 (limit=100 is plenty)

3. **Style matching:** Use same classes as CommunityList — `box-terminal`, `border-terminal-border`, `bg-terminal-surface`, `text-xs font-mono`, tree chars for list items.
  </action>
  <verify>
    <automated>cd web && npx astro check 2>&1 | tail -20</automated>
  </verify>
  <done>
    - /my-communities page exists and renders
    - Authenticated users see their joined communities list via ListUserCommunities API
    - Unauthenticated users see login prompt
    - Empty state links to /communities for browsing
  </done>
</task>

<task type="auto">
  <name>Task 3: Update UserDropdown and Sidebar navigation links</name>
  <files>web/src/components/layout/UserDropdown.svelte, web/src/components/layout/Sidebar.svelte</files>
  <action>
**UserDropdown.svelte changes:**
1. Change the "communities" menu item (line 37-43):
   - Text: change `communities` to `my communities`
   - href: change `/communities` to `/my-communities`
   - Keep same styling and onclick={onclose}

**Sidebar.svelte changes:**
1. In the authenticated section (after the "My Communities" list, around line 126-127, before the closing `{/if}` for authed):
   Add a divider and an "all communities" link with a small inline search shortcut:
   ```
   <!-- Divider -->
   <div class="px-2 text-terminal-border text-xs select-none my-2">────────────────</div>
   <!-- All communities link -->
   <a
     href="/communities"
     class="flex items-center gap-2 px-2 py-0.5 text-accent-600 hover:text-accent-500 transition-colors text-xs"
   >
     <span class="w-4 text-center text-terminal-dim">◈</span>
     <span>all communities</span>
   </a>
   ```

2. In the anonymous section (around line 129-143), the "browse all communities →" link already goes to `/communities` — keep it. But also ensure it's clearly labeled. The existing `browse all communities →` link is fine.

3. Keep the existing "My Communities" section header and community list unchanged in the authenticated block — that fetches via the sidebar's own mechanism and is working correctly.
  </action>
  <verify>
    <automated>cd web && npx astro check 2>&1 | tail -20</automated>
  </verify>
  <done>
    - User dropdown shows "my communities" linking to /my-communities
    - Sidebar has "all communities" link going to /communities (for authenticated users)
    - Anonymous sidebar still shows "browse all communities" link
    - No broken links
  </done>
</task>

</tasks>

<verification>
1. `cd web && npx astro check` — no type errors
2. Navigate to /communities — see all communities with search bar, only "members" and "created" sort options
3. Navigate to /my-communities while logged in — see only joined communities
4. Navigate to /my-communities while logged out — see login prompt
5. Click user dropdown → "my communities" links to /my-communities
6. Sidebar shows "all communities" link going to /communities
</verification>

<success_criteria>
- /communities is the "all communities" browse page with search and no "activity" sort
- /my-communities shows only user's joined communities (via ListUserCommunities API)
- User dropdown says "my communities" → /my-communities
- Sidebar has "all communities" link → /communities
- All type checks pass
</success_criteria>

<output>
After completion, create `.planning/quick/004-fix-frontend-communities/004-SUMMARY.md`
</output>
