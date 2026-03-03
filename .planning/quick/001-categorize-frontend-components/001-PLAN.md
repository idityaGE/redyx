---
phase: quick
plan: 001
type: quick
wave: 1
depends_on: []
files_modified:
  - web/src/components/**
  - web/src/pages/**
  - web/src/layouts/BaseLayout.astro
autonomous: true
---

<objective>
Reorganize 33 flat frontend components into domain-specific subfolders under web/src/components/ and update all import paths.

Categories:
- auth/ (7): AuthForm, LoginForm, RegisterForm, VerifyForm, ChooseUsernameForm, ResetPasswordForm, ResetCompleteForm
- community/ (6): CommunityDetail, CommunityFeed, CommunitySidebar, CommunityList, CommunityCreateForm, CommunitySettings
- post/ (4): PostDetail, PostBody, PostSubmitForm, VoteButtons
- feed/ (5): FeedList, FeedRow, SortBar, HomeFeed, SavedPosts
- profile/ (3): ProfileHeader, ProfileTabs, ProfileEditor
- layout/ (5): Header.svelte, Sidebar.svelte, MobileNav.svelte, ThemeToggle.svelte, UserDropdown.svelte
- root (3 .astro): Footer.astro, Header.astro, Sidebar.astro (keep in root — unused legacy)
</objective>

<tasks>

<task type="auto">
  <name>Task 1: Move component files into subfolders</name>
  <action>
    Create subdirectories: auth/, community/, post/, feed/, profile/, layout/
    Move each .svelte file to its category folder using git mv.
  </action>
  <verify>All files exist in new locations, none left in root (except .astro files)</verify>
  <done>33 components moved to 6 subfolders</done>
</task>

<task type="auto">
  <name>Task 2: Update all import paths</name>
  <action>
    Update import paths in:
    - Svelte component-to-component imports (./X.svelte → ./X.svelte or ../category/X.svelte)
    - Astro page imports (../components/X → ../components/category/X)
    - Astro layout imports (../components/X → ../components/layout/X)
  </action>
  <verify>astro build succeeds with no import errors</verify>
  <done>All import paths updated, build passes</done>
</task>

</tasks>
