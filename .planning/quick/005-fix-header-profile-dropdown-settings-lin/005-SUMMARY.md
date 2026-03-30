---
phase: quick-005
plan: 01
subsystem: frontend/settings
tags: [svelte, astro, settings, navigation, account]
dependency_graph:
  requires: [auth, user-api]
  provides: [settings-page, account-settings]
  affects: [user-dropdown, notifications-page]
tech_stack:
  added: []
  patterns: [settings-layout, inline-edit, type-to-confirm]
key_files:
  created:
    - web/src/components/settings/SettingsLayout.svelte
    - web/src/components/settings/AccountSettings.svelte
    - web/src/pages/settings/index.astro
  modified:
    - web/src/components/layout/UserDropdown.svelte
    - web/src/pages/settings/notifications.astro
decisions:
  - SettingsLayout as Svelte component with sidebar and slot for content
  - AccountSettings fetches user data directly (self-contained component)
  - Both account and notifications pages share SettingsLayout for consistent nav
metrics:
  duration: ~5 min
  completed: 2026-03-30
---

# Quick Task 005: Fix Header Profile Dropdown Settings Link

Settings page hub with sidebar navigation and account settings editing.

## One-liner

Fixed settings dropdown to navigate to /settings with sidebar nav and inline account editing (displayName, bio, avatar, deletion).

## Changes Made

### Task 1-2: Settings Page Structure + Account Settings

**Files created:**
- `web/src/components/settings/SettingsLayout.svelte` - Settings page layout with sidebar navigation
- `web/src/components/settings/AccountSettings.svelte` - Account/profile settings with inline editing
- `web/src/pages/settings/index.astro` - Main settings page entry point

**Files modified:**
- `web/src/components/layout/UserDropdown.svelte` - Changed settings href from `/user/{username}` to `/settings`

**Key implementation details:**
- SettingsLayout provides sidebar with "account" and "notifications" links
- Active section indicated with `*` marker and accent color
- Auth guard redirects to `/login?redirect=/settings` if not authenticated
- AccountSettings fetches current user via `/users/me` endpoint
- Inline edit pattern for displayName, bio (with 500 char limit), and avatarUrl
- Account deletion uses type-to-confirm "delete" pattern
- Terminal styling consistent with app design (box borders, `┌─` headers, `└───` footers)

**Commit:** c9850bd

### Task 3: Update Notifications Page

**File modified:**
- `web/src/pages/settings/notifications.astro` - Wrapped NotificationPreferences in SettingsLayout

**Key implementation details:**
- Added SettingsLayout wrapper with `activeSection="notifications"`
- Enables sidebar navigation between account and notification settings
- Consistent layout across all settings pages

**Commit:** bd5e921

## Deviations from Plan

None - plan executed exactly as written.

## Verification

- [x] `grep -n "href=\"/settings\"" web/src/components/layout/UserDropdown.svelte` returns the settings link (line 45)
- [x] `web/src/pages/settings/index.astro` exists
- [x] `web/src/components/settings/SettingsLayout.svelte` exists
- [x] `web/src/components/settings/AccountSettings.svelte` exists with PATCH and DELETE functionality
- [x] `web/src/pages/settings/notifications.astro` imports and uses SettingsLayout

## Self-Check: PASSED

All created files verified to exist. All commits verified in git log.
