<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { logout, getUser, subscribe as authSubscribe } from '../../lib/auth';

  // Auth state
  let username = $state(getUser()?.username || '');

  // User data
  let user = $state<{
    displayName: string;
    bio: string;
    avatarUrl: string;
  } | null>(null);
  let loading = $state(true);
  let fetchError = $state('');

  // Edit mode - single edit mode for better UX
  let editMode = $state(false);
  
  // Field values (for editing)
  let displayNameValue = $state('');
  let bioValue = $state('');
  let avatarUrlValue = $state('');

  // UI state
  let saving = $state(false);
  let error = $state<string | null>(null);
  let success = $state<string | null>(null);

  // Account deletion
  let showDeleteConfirm = $state(false);
  let deleteInput = $state('');
  let deleting = $state(false);
  let deleteError = $state<string | null>(null);

  // Derived
  let bioCharCount = $derived(bioValue.length);
  let bioOverLimit = $derived(bioValue.length > 500);
  let avatarLetter = $derived(username ? username.charAt(0).toUpperCase() : '?');
  let hasChanges = $derived(
    displayNameValue !== (user?.displayName || '') ||
    bioValue !== (user?.bio || '') ||
    avatarUrlValue !== (user?.avatarUrl || '')
  );

  // Sync from user when loaded or edit mode changes
  $effect(() => {
    if (user && !editMode) {
      displayNameValue = user.displayName || '';
      bioValue = user.bio || '';
      avatarUrlValue = user.avatarUrl || '';
    }
  });

  async function fetchUser() {
    loading = true;
    fetchError = '';
    
    if (!username) {
      fetchError = 'not authenticated';
      loading = false;
      return;
    }
    
    try {
      const data = await api<{
        user: {
          displayName: string;
          bio: string;
          avatarUrl: string;
        };
      }>(`/users/${username}`);
      user = {
        displayName: data.user?.displayName || '',
        bio: data.user?.bio || '',
        avatarUrl: data.user?.avatarUrl || '',
      };
    } catch (e) {
      fetchError = e instanceof ApiError ? e.message : 'failed to load user data';
    } finally {
      loading = false;
    }
  }

  async function saveProfile() {
    saving = true;
    error = null;
    success = null;
    
    if (!username) {
      error = 'not authenticated';
      saving = false;
      return;
    }
    
    try {
      const body: Record<string, string> = {};
      
      // Only include changed fields (use snake_case for proto)
      if (displayNameValue !== (user?.displayName || '')) {
        body.display_name = displayNameValue;
      }
      if (bioValue !== (user?.bio || '')) {
        body.bio = bioValue;
      }
      if (avatarUrlValue !== (user?.avatarUrl || '')) {
        body.avatar_url = avatarUrlValue;
      }
      
      if (Object.keys(body).length === 0) {
        editMode = false;
        return;
      }
      
      await api('/users/me', {
        method: 'PATCH',
        body: JSON.stringify(body),
      });

      // Update local user state
      if (user) {
        user = {
          displayName: displayNameValue,
          bio: bioValue,
          avatarUrl: avatarUrlValue,
        };
      }
      
      success = 'profile updated';
      editMode = false;
      setTimeout(() => { success = null; }, 3000);
    } catch (e) {
      if (e instanceof ApiError) {
        error = e.message;
      } else {
        error = 'save failed';
      }
    } finally {
      saving = false;
    }
  }

  function cancelEdit() {
    editMode = false;
    error = null;
    // Reset values from user
    if (user) {
      displayNameValue = user.displayName || '';
      bioValue = user.bio || '';
      avatarUrlValue = user.avatarUrl || '';
    }
  }

  async function deleteAccount() {
    if (deleteInput !== 'delete') return;
    if (!username) {
      deleteError = 'not authenticated';
      return;
    }
    deleting = true;
    deleteError = null;
    try {
      await api('/users/me', { method: 'DELETE' });
      await logout();
      window.location.href = '/';
    } catch (e) {
      if (e instanceof ApiError) {
        deleteError = e.message;
      } else {
        deleteError = 'deletion failed';
      }
      deleting = false;
    }
  }

  onMount(() => {
    const unsub = authSubscribe(() => {
      const currentUser = getUser();
      if (currentUser?.username && currentUser.username !== username) {
        username = currentUser.username;
        fetchUser();
      }
    });
    
    fetchUser();
    
    return unsub;
  });
</script>

{#if loading}
  <div class="flex items-center justify-center min-h-[20vh]">
    <span class="text-xs text-terminal-dim font-mono animate-pulse">[loading account...]</span>
  </div>
{:else if fetchError}
  <div class="text-xs font-mono">
    <span class="text-red-500">&gt; error:</span>
    <span class="text-red-400"> {fetchError}</span>
  </div>
{:else}
  <div class="max-w-2xl font-mono">
    <!-- Status messages -->
    {#if error}
      <div class="mb-4 p-2 border border-red-900 bg-red-950/30 text-xs">
        <span class="text-red-500">&gt; error:</span>
        <span class="text-red-400"> {error}</span>
      </div>
    {/if}
    {#if success}
      <div class="mb-4 p-2 border border-green-900 bg-green-950/30 text-xs text-green-500">
        &gt; {success}
      </div>
    {/if}

    <!-- Profile Card -->
    <div class="border border-terminal-border bg-terminal-surface mb-6">
      <div class="px-4 py-2 border-b border-terminal-border flex items-center justify-between">
        <span class="text-xs text-terminal-dim">┌─ profile</span>
        {#if !editMode}
          <button
            class="text-xs text-terminal-dim hover:text-accent-500 transition-colors cursor-pointer"
            onclick={() => { editMode = true; }}
          >
            [edit]
          </button>
        {/if}
      </div>
      
      <div class="p-4">
        <!-- Avatar + Username row -->
        <div class="flex items-start gap-4 mb-6">
          <!-- Avatar preview with ASCII border -->
          <div class="shrink-0">
            <div class="text-terminal-dim text-xs leading-none select-none">
              <div>&#x250C;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2510;</div>
              <div>&#x2502;{#if avatarUrlValue}<img src={avatarUrlValue} alt={username} class="inline-block w-14 h-14 object-cover" onerror={(e) => { e.currentTarget.style.display = 'none'; }} />{:else}<span class="inline-flex items-center justify-center w-14 h-14 text-2xl text-accent-500 bg-terminal-bg font-bold">{avatarLetter}</span>{/if}&#x2502;</div>
              <div>&#x2514;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2518;</div>
            </div>
          </div>
          
          <!-- User info -->
          <div class="flex-1 min-w-0">
            <div class="text-sm text-accent-500 mb-1">u/{username}</div>
            {#if !editMode}
              <div class="text-xs text-terminal-fg">
                {user?.displayName || username}
              </div>
              {#if user?.bio}
                <div class="text-xs text-terminal-dim mt-2 whitespace-pre-wrap break-words">
                  {user.bio}
                </div>
              {/if}
            {/if}
          </div>
        </div>

        {#if editMode}
          <!-- Edit Form -->
          <div class="space-y-4">
            <!-- Display Name -->
            <div>
              <label class="block text-xs text-terminal-dim mb-1">
                display_name
              </label>
              <input
                type="text"
                bind:value={displayNameValue}
                class="w-full bg-terminal-bg border border-terminal-border text-terminal-fg px-3 py-2 text-xs outline-none focus:border-accent-500 transition-colors"
                maxlength="50"
                placeholder="your display name"
              />
              <div class="text-xs text-terminal-dim mt-1">
                shown instead of username
              </div>
            </div>
            
            <!-- Bio -->
            <div>
              <label class="block text-xs text-terminal-dim mb-1">
                bio
              </label>
              <textarea
                bind:value={bioValue}
                class="w-full bg-terminal-bg border border-terminal-border text-terminal-fg px-3 py-2 text-xs outline-none focus:border-accent-500 resize-y min-h-[100px] transition-colors"
                maxlength="500"
                rows="4"
                placeholder="tell us about yourself..."
              ></textarea>
              <div class="flex justify-between text-xs mt-1">
                <span class="text-terminal-dim">markdown supported</span>
                <span class="{bioOverLimit ? 'text-red-500' : 'text-terminal-dim'}">{bioCharCount}/500</span>
              </div>
            </div>
            
            <!-- Avatar URL -->
            <div>
              <label class="block text-xs text-terminal-dim mb-1">
                avatar_url
              </label>
              <input
                type="url"
                bind:value={avatarUrlValue}
                class="w-full bg-terminal-bg border border-terminal-border text-terminal-fg px-3 py-2 text-xs outline-none focus:border-accent-500 transition-colors"
                placeholder="https://example.com/avatar.png"
              />
              <div class="text-xs text-terminal-dim mt-1">
                direct link to image (updates preview above)
              </div>
            </div>

            <!-- Actions -->
            <div class="flex items-center gap-3 pt-2">
              <button
                class="px-4 py-1.5 bg-accent-500 text-terminal-bg text-xs hover:bg-accent-400 transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                disabled={saving || bioOverLimit || !hasChanges}
                onclick={saveProfile}
              >
                {#if saving}
                  [saving...]
                {:else}
                  [save changes]
                {/if}
              </button>
              <button
                class="text-xs text-terminal-dim hover:text-terminal-fg transition-colors cursor-pointer"
                onclick={cancelEdit}
              >
                [cancel]
              </button>
              {#if hasChanges}
                <span class="text-xs text-yellow-500 ml-auto">unsaved changes</span>
              {/if}
            </div>
          </div>
        {/if}
      </div>
      
      <div class="px-4 py-1 border-t border-terminal-border text-xs text-terminal-dim">
        └────────────────────────
      </div>
    </div>

    <!-- Danger Zone -->
    <div class="border border-red-900/50 bg-terminal-surface">
      <div class="px-4 py-2 border-b border-red-900/50 text-xs text-red-500">
        ┌─ danger zone
      </div>
      
      <div class="p-4">
        {#if !showDeleteConfirm}
          <div class="flex items-center justify-between">
            <div>
              <div class="text-xs text-terminal-fg">Delete account</div>
              <div class="text-xs text-terminal-dim mt-0.5">
                permanently remove your account and all associated data
              </div>
            </div>
            <button
              class="px-3 py-1 border border-red-900 text-red-500 text-xs hover:bg-red-950/50 transition-colors cursor-pointer"
              onclick={() => { showDeleteConfirm = true; }}
            >
              [delete]
            </button>
          </div>
        {:else}
          <div class="space-y-3">
            <div class="text-xs text-red-400">
              This action cannot be undone. All your posts, comments, and data will be permanently deleted.
            </div>
            
            <div>
              <label class="block text-xs text-terminal-dim mb-1">
                type '<span class="text-red-400">delete</span>' to confirm
              </label>
              <div class="flex items-center gap-2">
                <span class="text-red-500">&gt;</span>
                <input
                  type="text"
                  bind:value={deleteInput}
                  class="flex-1 bg-terminal-bg border border-red-900 text-red-400 px-3 py-1.5 text-xs outline-none focus:border-red-500 transition-colors"
                  placeholder="delete"
                />
              </div>
            </div>
            
            {#if deleteError}
              <div class="text-xs">
                <span class="text-red-500">&gt; error:</span>
                <span class="text-red-400"> {deleteError}</span>
              </div>
            {/if}
            
            <div class="flex items-center gap-3">
              <button
                class="px-4 py-1.5 bg-red-600 text-white text-xs hover:bg-red-500 transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                disabled={deleteInput !== 'delete' || deleting}
                onclick={deleteAccount}
              >
                {#if deleting}
                  [deleting...]
                {:else}
                  [permanently delete]
                {/if}
              </button>
              <button
                class="text-xs text-terminal-dim hover:text-terminal-fg transition-colors cursor-pointer"
                onclick={() => { showDeleteConfirm = false; deleteInput = ''; deleteError = null; }}
              >
                [cancel]
              </button>
            </div>
          </div>
        {/if}
      </div>
      
      <div class="px-4 py-1 border-t border-red-900/50 text-xs text-red-500/50">
        └────────────────────────
      </div>
    </div>
  </div>
{/if}
