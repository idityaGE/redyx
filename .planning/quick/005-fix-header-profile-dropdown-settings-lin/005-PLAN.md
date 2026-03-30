---
phase: quick-005
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/src/components/layout/UserDropdown.svelte
  - web/src/pages/settings/index.astro
  - web/src/components/settings/SettingsLayout.svelte
  - web/src/components/settings/AccountSettings.svelte
  - web/src/pages/settings/notifications.astro
autonomous: true
requirements: [QUICK-005]

must_haves:
  truths:
    - "Settings dropdown link navigates to /settings (not profile page)"
    - "Main settings page shows sidebar with Account and Notifications sections"
    - "Account settings allows editing displayName, bio, avatarUrl, and account deletion"
    - "Notifications link in settings sidebar navigates to existing /settings/notifications"
  artifacts:
    - path: "web/src/pages/settings/index.astro"
      provides: "Main settings page entry point"
    - path: "web/src/components/settings/SettingsLayout.svelte"
      provides: "Settings navigation sidebar + content area"
    - path: "web/src/components/settings/AccountSettings.svelte"
      provides: "Account/profile settings form (reusing ProfileEditor patterns)"
  key_links:
    - from: "UserDropdown.svelte"
      to: "/settings"
      via: "href attribute on settings link"
    - from: "SettingsLayout.svelte"
      to: "/settings/notifications"
      via: "sidebar navigation link"
---

<objective>
Fix header profile dropdown settings link to redirect to a proper /settings page with all available settings instead of the profile page.

Purpose: Currently both "Profile" and "Settings" in the dropdown go to the same profile page. Create proper settings hub with account and notifications sections.
Output: Working /settings page with sidebar navigation, account settings, and link to notifications.
</objective>

<execution_context>
@./.opencode/get-shit-done/workflows/execute-plan.md
@./.opencode/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@web/src/components/layout/UserDropdown.svelte
@web/src/components/profile/ProfileEditor.svelte
@web/src/pages/settings/notifications.astro
@web/src/components/notification/NotificationPreferences.svelte
</context>

<interfaces>
<!-- Key patterns from existing codebase -->

From web/src/components/layout/UserDropdown.svelte (lines 44-50):
```svelte
<!-- Current broken settings link - needs to change href -->
<a
  href="/user/{username}"  <!-- BUG: Should be /settings -->
  class="flex items-center gap-2 px-3 py-1.5 text-terminal-fg hover:text-accent-500 hover:bg-terminal-bg transition-colors"
  onclick={onclose}
>
  <span class="text-accent-500">&gt;</span> settings
</a>
```

From web/src/components/profile/ProfileEditor.svelte:
```typescript
// Pattern for account settings - reuse this approach
async function saveField(field: string, value: string) { ... }
async function deleteAccount() { ... }
```

From web/src/components/notification/NotificationPreferences.svelte:
```svelte
<!-- Terminal-style page header pattern -->
<div class="box-terminal mb-4">
  <div class="text-accent-500 text-sm">~ /settings/notifications</div>
  <div class="text-xs text-terminal-dim mt-1">notification preferences</div>
</div>
```
</interfaces>

<tasks>

<task type="auto">
  <name>Task 1: Create settings page structure and fix dropdown link</name>
  <files>
    web/src/pages/settings/index.astro
    web/src/components/settings/SettingsLayout.svelte
    web/src/components/layout/UserDropdown.svelte
  </files>
  <action>
1. Create `web/src/components/settings/` directory

2. Create `SettingsLayout.svelte` - settings page with sidebar navigation:
   - Left sidebar (w-48) with terminal-style navigation links:
     - `> account` → active state when on /settings (or /settings/account)
     - `> notifications` → links to /settings/notifications
   - Main content area receives slot for page content
   - Use terminal styling consistent with rest of app (border-terminal-border, text-terminal-fg, etc.)
   - Auth guard: redirect to /login?redirect=/settings if not authenticated
   - Use same auth pattern as NotificationPreferences.svelte (initialize, subscribe, $effect redirect)

3. Create `web/src/pages/settings/index.astro`:
   - Import BaseLayout and SettingsLayout
   - Import AccountSettings component (created in Task 2)
   - Pass 'account' as activeSection prop

4. Update `UserDropdown.svelte` line 44-50:
   - Change href from `/user/{username}` to `/settings`
   - Keep all other attributes unchanged
  </action>
  <verify>
    - `grep -n "href=\"/settings\"" web/src/components/layout/UserDropdown.svelte` returns the settings link
    - `ls web/src/pages/settings/index.astro` exists
    - `ls web/src/components/settings/SettingsLayout.svelte` exists
  </verify>
  <done>
    - UserDropdown settings link points to /settings
    - Settings page structure exists with sidebar navigation
    - Sidebar shows account (active) and notifications links
  </done>
</task>

<task type="auto">
  <name>Task 2: Create AccountSettings component with profile editing</name>
  <files>
    web/src/components/settings/AccountSettings.svelte
  </files>
  <action>
Create `AccountSettings.svelte` that provides account/profile settings:

1. Fetch current user data on mount using `/users/me` endpoint
2. Display and allow editing of:
   - displayName (inline edit pattern from ProfileEditor)
   - bio (textarea with character count, 500 char limit)
   - avatarUrl (inline edit)
3. Account deletion section at bottom (type-to-confirm "delete" pattern)
4. Use terminal styling:
   - Section headers with `┌─ section name` pattern
   - `└─────────` footers
   - `[save]` / `[cancel]` / `[edit]` button styling
   - Green success messages, red error messages

Key patterns to reuse from ProfileEditor.svelte:
- `saveField(field, value)` function for PATCH /users/me
- `$effect()` for prop-to-state sync
- Type-to-confirm for account deletion
- Saving/success/error state management

Structure:
```
┌─ profile
├ display_name: [value] [edit]
├ bio: [value] [edit]  
└ avatar_url: [value] [edit]

┌─ danger zone
└ > rm -rf account
```
  </action>
  <verify>
    - `ls web/src/components/settings/AccountSettings.svelte` exists
    - `grep -n "PATCH" web/src/components/settings/AccountSettings.svelte` shows save functionality
    - `grep -n "DELETE" web/src/components/settings/AccountSettings.svelte` shows account deletion
  </verify>
  <done>
    - AccountSettings component displays user profile fields
    - Inline editing works for displayName, bio, avatarUrl
    - Account deletion with type-to-confirm pattern works
    - Terminal styling consistent with app design
  </done>
</task>

<task type="auto">
  <name>Task 3: Update notifications page to use settings layout</name>
  <files>
    web/src/pages/settings/notifications.astro
  </files>
  <action>
Update `/settings/notifications.astro` to use the new SettingsLayout for consistent navigation:

1. Import SettingsLayout from components/settings/
2. Wrap NotificationPreferences in SettingsLayout with activeSection="notifications"
3. This provides sidebar navigation when viewing notification settings

The page should render as:
- Left sidebar: account link, notifications link (active)
- Main content: existing NotificationPreferences component
  </action>
  <verify>
    - `grep -n "SettingsLayout" web/src/pages/settings/notifications.astro` shows layout import and usage
    - Page renders with sidebar navigation
  </verify>
  <done>
    - Notification settings page has sidebar navigation
    - Can navigate between account and notification settings using sidebar
    - Consistent layout across all settings pages
  </done>
</task>

</tasks>

<verification>
1. Click user dropdown in header → settings link exists
2. Click settings → navigates to /settings (not /user/{username})
3. Settings page shows sidebar with account and notifications
4. Account settings allow editing profile fields
5. Clicking notifications in sidebar navigates to /settings/notifications
6. Notification settings page also shows sidebar navigation
</verification>

<success_criteria>
- Header dropdown "settings" links to /settings
- /settings page renders with sidebar + account settings
- /settings/notifications renders with sidebar + notification preferences
- Account settings allow inline editing of displayName, bio, avatarUrl
- Account deletion with type-to-confirm works
- All pages require authentication (redirect to login if anonymous)
</success_criteria>

<output>
After completion, create `.planning/quick/005-fix-header-profile-dropdown-settings-lin/005-SUMMARY.md`
</output>
