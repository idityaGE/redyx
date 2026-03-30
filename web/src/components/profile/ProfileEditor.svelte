<script lang="ts">
  import { api, ApiError } from '../../lib/api';
  import { logout, getUser } from '../../lib/auth';

  interface Props {
    displayName: string;
    bio: string;
    avatarUrl: string;
    onupdate: (updated: Record<string, string>) => void;
  }

  let { displayName, bio, avatarUrl, onupdate }: Props = $props();
  
  // Get username from auth store
  let username = getUser()?.username || '';

  // Edit states
  let editingDisplayName = $state(false);
  let editingBio = $state(false);
  let editingAvatar = $state(false);

  // Field values — initialized via $state, synced from props via $effect
  let displayNameValue = $state('');
  let bioValue = $state('');
  let avatarUrlValue = $state('');

  // Sync from props when they change (e.g., after save updates parent)
  $effect(() => {
    if (!editingDisplayName) displayNameValue = displayName;
  });
  $effect(() => {
    if (!editingBio) bioValue = bio;
  });
  $effect(() => {
    if (!editingAvatar) avatarUrlValue = avatarUrl;
  });

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
      // Map camelCase field names to snake_case for proto
      const fieldMap: Record<string, string> = {
        displayName: 'display_name',
        bio: 'bio',
        avatarUrl: 'avatar_url',
      };
      const protoField = fieldMap[field] || field;
      
      const body: Record<string, string> = {};
      body[protoField] = value;
      await api('/users/me', {
        method: 'PATCH',
        body: JSON.stringify(body),
      });
      onupdate({ [field]: value });
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

  function cancelEdit(field: string) {
    if (field === 'displayName') {
      editingDisplayName = false;
      displayNameValue = displayName;
    } else if (field === 'bio') {
      editingBio = false;
      bioValue = bio;
    } else if (field === 'avatarUrl') {
      editingAvatar = false;
      avatarUrlValue = avatarUrl;
    }
  }
</script>

<div class="text-xs font-mono space-y-3">
  <!-- Status messages -->
  {#if error}
    <div>
      <span class="text-red-500">&gt; error:</span>
      <span class="text-red-400"> {error}</span>
    </div>
  {/if}
  {#if success}
    <div class="text-green-500">&gt; {success}</div>
  {/if}

  <!-- Display name -->
  <div>
    <div class="flex items-center gap-2">
      <span class="text-terminal-dim">display_name:</span>
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
        <span class="text-terminal-fg">{displayName || '(not set)'}</span>
        <button
          class="text-terminal-dim hover:text-accent-500 transition-colors cursor-pointer"
          onclick={() => { editingDisplayName = true; }}
        >
          [edit]
        </button>
      {/if}
    </div>
  </div>

  <!-- Bio -->
  <div>
    <div class="flex items-center gap-2 mb-1">
      <span class="text-terminal-dim">bio:</span>
      {#if !editingBio}
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
        class="w-full bg-terminal-bg border border-terminal-border text-terminal-fg px-2 py-1.5 text-xs font-mono outline-none focus:border-accent-500 resize-y min-h-[60px]"
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
    {:else}
      <pre class="text-terminal-fg whitespace-pre-wrap text-xs">{bio || '(no bio set)'}</pre>
    {/if}
  </div>

  <!-- Avatar URL -->
  <div>
    <div class="flex items-center gap-2">
      <span class="text-terminal-dim">avatar_url:</span>
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
        <span class="text-terminal-fg truncate max-w-[200px]">{avatarUrl || '(none)'}</span>
        <button
          class="text-terminal-dim hover:text-accent-500 transition-colors cursor-pointer"
          onclick={() => { editingAvatar = true; }}
        >
          [edit]
        </button>
      {/if}
    </div>
  </div>

  <!-- Divider -->
  <div class="text-terminal-border select-none">
    ────────────────
  </div>

  <!-- Account deletion -->
  <div>
    {#if !showDeleteConfirm}
      <button
        class="text-red-500 hover:text-red-400 transition-colors cursor-pointer"
        onclick={() => { showDeleteConfirm = true; }}
      >
        <span class="text-red-600">&gt;</span> rm -rf account
      </button>
    {:else}
      <div class="border border-red-900 bg-terminal-bg p-2 space-y-2">
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
</div>
