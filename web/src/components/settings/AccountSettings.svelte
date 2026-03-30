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

  // Edit states
  let editingDisplayName = $state(false);
  let editingBio = $state(false);
  let editingAvatar = $state(false);

  // Field values
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

  let bioCharCount = $derived(bioValue.length);
  let bioOverLimit = $derived(bioValue.length > 500);

  // Sync from user when loaded
  $effect(() => {
    if (user && !editingDisplayName) displayNameValue = user.displayName || '';
  });
  $effect(() => {
    if (user && !editingBio) bioValue = user.bio || '';
  });
  $effect(() => {
    if (user && !editingAvatar) avatarUrlValue = user.avatarUrl || '';
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

  async function saveField(field: string, value: string) {
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
      body[field] = value;
      await api(`/users/${username}`, {
        method: 'PATCH',
        body: JSON.stringify(body),
      });

      // Update local user state
      if (user) {
        if (field === 'displayName') user.displayName = value;
        if (field === 'bio') user.bio = value;
        if (field === 'avatarUrl') user.avatarUrl = value;
      }
      success = `${field} updated`;

      // Close the editor for this field
      if (field === 'displayName') editingDisplayName = false;
      if (field === 'bio') editingBio = false;
      if (field === 'avatarUrl') editingAvatar = false;

      setTimeout(() => { success = null; }, 2000);
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

  async function deleteAccount() {
    if (deleteInput !== 'delete') return;
    if (!username) {
      deleteError = 'not authenticated';
      return;
    }
    deleting = true;
    deleteError = null;
    try {
      await api(`/users/${username}`, { method: 'DELETE' });
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

  function cancelEdit(field: string) {
    if (field === 'displayName') {
      editingDisplayName = false;
      displayNameValue = user?.displayName || '';
    } else if (field === 'bio') {
      editingBio = false;
      bioValue = user?.bio || '';
    } else if (field === 'avatarUrl') {
      editingAvatar = false;
      avatarUrlValue = user?.avatarUrl || '';
    }
  }

  onMount(() => {
    // Subscribe to auth changes
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
  <div class="max-w-2xl">
    <!-- Page header -->
    <div class="box-terminal mb-4">
      <div class="text-accent-500 text-sm">~ /settings/account</div>
      <div class="text-xs text-terminal-dim mt-1">account settings</div>
    </div>

    <div class="space-y-4">
      <!-- Status messages -->
      {#if error}
        <div class="text-xs font-mono">
          <span class="text-red-500">&gt; error:</span>
          <span class="text-red-400"> {error}</span>
        </div>
      {/if}
      {#if success}
        <div class="text-xs font-mono text-green-500">&gt; {success}</div>
      {/if}

      <!-- Profile section -->
      <div class="border border-terminal-border bg-terminal-surface font-mono">
        <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim">
          ┌─ profile
        </div>
        <div class="p-3 space-y-3 text-xs">
          <!-- Display name -->
          <div class="flex items-center gap-2">
            <span class="text-terminal-dim w-24 shrink-0">display_name:</span>
            {#if editingDisplayName}
              <input
                type="text"
                bind:value={displayNameValue}
                class="bg-terminal-bg border border-terminal-border text-terminal-fg px-1.5 py-0.5 text-xs font-mono outline-none focus:border-accent-500 flex-1"
                maxlength="50"
              />
              <button
                class="text-accent-500 hover:text-accent-400 transition-colors cursor-pointer"
                disabled={saving}
                onclick={() => saveField('displayName', displayNameValue)}
              >
                [save]
              </button>
              <button
                class="text-terminal-dim hover:text-terminal-fg transition-colors cursor-pointer"
                onclick={() => cancelEdit('displayName')}
              >
                [cancel]
              </button>
            {:else}
              <span class="text-terminal-fg">{user?.displayName || '(not set)'}</span>
              <button
                class="text-terminal-dim hover:text-accent-500 transition-colors cursor-pointer"
                onclick={() => { editingDisplayName = true; }}
              >
                [edit]
              </button>
            {/if}
          </div>

          <!-- Bio -->
          <div>
            <div class="flex items-center gap-2 mb-1">
              <span class="text-terminal-dim w-24 shrink-0">bio:</span>
              {#if !editingBio}
                <span class="text-terminal-fg truncate">{user?.bio || '(not set)'}</span>
                <button
                  class="text-terminal-dim hover:text-accent-500 transition-colors cursor-pointer"
                  onclick={() => { editingBio = true; }}
                >
                  [edit]
                </button>
              {/if}
            </div>
            {#if editingBio}
              <textarea
                bind:value={bioValue}
                class="w-full bg-terminal-bg border border-terminal-border text-terminal-fg px-2 py-1.5 text-xs font-mono outline-none focus:border-accent-500 resize-y min-h-[60px] ml-0"
                maxlength="500"
                rows="4"
                placeholder="tell us about yourself..."
              ></textarea>
              <div class="flex items-center justify-between mt-1">
                <span class="text-terminal-dim {bioOverLimit ? 'text-red-500' : ''}">{bioCharCount}/500</span>
                <div class="flex gap-2">
                  <button
                    class="text-accent-500 hover:text-accent-400 transition-colors cursor-pointer"
                    disabled={saving || bioOverLimit}
                    onclick={() => saveField('bio', bioValue)}
                  >
                    [save]
                  </button>
                  <button
                    class="text-terminal-dim hover:text-terminal-fg transition-colors cursor-pointer"
                    onclick={() => cancelEdit('bio')}
                  >
                    [cancel]
                  </button>
                </div>
              </div>
            {/if}
          </div>

          <!-- Avatar URL -->
          <div class="flex items-center gap-2">
            <span class="text-terminal-dim w-24 shrink-0">avatar_url:</span>
            {#if editingAvatar}
              <input
                type="url"
                bind:value={avatarUrlValue}
                class="bg-terminal-bg border border-terminal-border text-terminal-fg px-1.5 py-0.5 text-xs font-mono outline-none focus:border-accent-500 flex-1"
                placeholder="https://..."
              />
              <button
                class="text-accent-500 hover:text-accent-400 transition-colors cursor-pointer"
                disabled={saving}
                onclick={() => saveField('avatarUrl', avatarUrlValue)}
              >
                [save]
              </button>
              <button
                class="text-terminal-dim hover:text-terminal-fg transition-colors cursor-pointer"
                onclick={() => cancelEdit('avatarUrl')}
              >
                [cancel]
              </button>
            {:else}
              <span class="text-terminal-fg truncate max-w-[200px]">{user?.avatarUrl || '(none)'}</span>
              <button
                class="text-terminal-dim hover:text-accent-500 transition-colors cursor-pointer"
                onclick={() => { editingAvatar = true; }}
              >
                [edit]
              </button>
            {/if}
          </div>
        </div>
        <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
          └─────────────────
        </div>
      </div>

      <!-- Danger zone section -->
      <div class="border border-red-900 bg-terminal-surface font-mono">
        <div class="px-3 py-1.5 border-b border-red-900 text-xs text-red-500">
          ┌─ danger zone
        </div>
        <div class="p-3 text-xs">
          {#if !showDeleteConfirm}
            <button
              class="text-red-500 hover:text-red-400 transition-colors cursor-pointer"
              onclick={() => { showDeleteConfirm = true; }}
            >
              <span class="text-red-600">&gt;</span> rm -rf account
            </button>
          {:else}
            <div class="space-y-2">
              <div class="text-red-500">
                <span class="font-bold">WARNING:</span> this will permanently delete your account
              </div>
              <div class="text-terminal-dim">
                type '<span class="text-red-400">delete</span>' to confirm:
              </div>
              <div class="flex items-center gap-2">
                <span class="text-red-500">&gt;</span>
                <input
                  type="text"
                  bind:value={deleteInput}
                  class="bg-terminal-bg border border-red-900 text-red-400 px-1.5 py-0.5 text-xs font-mono outline-none focus:border-red-500 flex-1"
                  placeholder="type 'delete'"
                />
              </div>
              {#if deleteError}
                <div>
                  <span class="text-red-500">&gt; error:</span>
                  <span class="text-red-400"> {deleteError}</span>
                </div>
              {/if}
              <div class="flex gap-2">
                <button
                  class="text-red-500 hover:text-red-400 transition-colors cursor-pointer disabled:opacity-50"
                  disabled={deleteInput !== 'delete' || deleting}
                  onclick={deleteAccount}
                >
                  {#if deleting}
                    [deleting...]
                  {:else}
                    [confirm delete]
                  {/if}
                </button>
                <button
                  class="text-terminal-dim hover:text-terminal-fg transition-colors cursor-pointer"
                  onclick={() => { showDeleteConfirm = false; deleteInput = ''; deleteError = null; }}
                >
                  [cancel]
                </button>
              </div>
            </div>
          {/if}
        </div>
        <div class="px-3 py-1 border-t border-red-900 text-xs text-red-500">
          └─────────────────
        </div>
      </div>
    </div>
  </div>
{/if}
