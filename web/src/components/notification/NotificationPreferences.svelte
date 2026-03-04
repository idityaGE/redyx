<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { isAuthenticated, isLoading, initialize, subscribe } from '../../lib/auth';

  let authed = $state(isAuthenticated());
  let authLoading = $state(isLoading());

  // Preferences state
  let prefs = $state({
    postReplies: true,
    commentReplies: true,
    mentions: true,
    mutedCommunities: [] as string[],
  });
  let loading = $state(true);
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  // Muted community input
  let newMutedCommunity = $state('');

  async function fetchPreferences() {
    loading = true;
    try {
      const data = await api<{
        preferences: {
          postReplies: boolean;
          commentReplies: boolean;
          mentions: boolean;
          mutedCommunities: string[];
        };
      }>('/notifications/preferences');

      prefs = {
        postReplies: data.preferences?.postReplies ?? true,
        commentReplies: data.preferences?.commentReplies ?? true,
        mentions: data.preferences?.mentions ?? true,
        mutedCommunities: data.preferences?.mutedCommunities ?? [],
      };
    } catch {
      // Use defaults if fetch fails
    } finally {
      loading = false;
    }
  }

  async function savePreferences() {
    saving = true;
    saved = false;
    error = '';
    try {
      await api('/notifications/preferences', {
        method: 'PATCH',
        body: JSON.stringify({
          postReplies: prefs.postReplies,
          commentReplies: prefs.commentReplies,
          mentions: prefs.mentions,
          mutedCommunities: prefs.mutedCommunities,
        }),
      });
      saved = true;
      // Auto-clear success message after 2 seconds
      setTimeout(() => { saved = false; }, 2000);
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'failed to save preferences';
    } finally {
      saving = false;
    }
  }

  function togglePostReplies() {
    prefs = { ...prefs, postReplies: !prefs.postReplies };
  }

  function toggleCommentReplies() {
    prefs = { ...prefs, commentReplies: !prefs.commentReplies };
  }

  function toggleMentions() {
    prefs = { ...prefs, mentions: !prefs.mentions };
  }

  function addMutedCommunity() {
    const name = newMutedCommunity.trim();
    if (!name) return;
    if (prefs.mutedCommunities.includes(name)) {
      newMutedCommunity = '';
      return;
    }
    prefs = {
      ...prefs,
      mutedCommunities: [...prefs.mutedCommunities, name],
    };
    newMutedCommunity = '';
  }

  function removeMutedCommunity(name: string) {
    prefs = {
      ...prefs,
      mutedCommunities: prefs.mutedCommunities.filter((c) => c !== name),
    };
  }

  onMount(() => {
    initialize();

    const unsub = subscribe(() => {
      authed = isAuthenticated();
      authLoading = isLoading();
    });

    fetchPreferences();

    return unsub;
  });

  // Redirect to login if not authenticated
  $effect(() => {
    if (!authLoading && !authed) {
      window.location.href = '/login?redirect=/settings/notifications';
    }
  });
</script>

{#if loading || authLoading}
  <div class="flex items-center justify-center min-h-[40vh]">
    <span class="text-xs text-terminal-dim font-mono animate-pulse">[loading preferences...]</span>
  </div>
{:else}
  <div class="max-w-2xl">
    <!-- Page header -->
    <div class="box-terminal mb-4">
      <div class="text-accent-500 text-sm">~ /settings/notifications</div>
      <div class="text-xs text-terminal-dim mt-1">notification preferences</div>
    </div>

    <div class="space-y-4">

      <!-- ── Notification Types ──────────────────── -->
      <div class="border border-terminal-border bg-terminal-surface font-mono">
        <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim">
          ┌─ notification types
        </div>
        <div class="p-3 space-y-3">
          <!-- Post replies toggle -->
          <div class="flex items-center justify-between">
            <div class="text-xs">
              <span class="text-terminal-fg">post replies</span>
              <span class="text-terminal-dim ml-1">— replies to your posts</span>
            </div>
            <button
              onclick={togglePostReplies}
              class="text-xs font-mono cursor-pointer {prefs.postReplies ? 'text-green-500' : 'text-terminal-dim'}"
            >
              {prefs.postReplies ? '[ ON ]' : '[OFF]'}
            </button>
          </div>

          <!-- Comment replies toggle -->
          <div class="flex items-center justify-between">
            <div class="text-xs">
              <span class="text-terminal-fg">comment replies</span>
              <span class="text-terminal-dim ml-1">— replies to your comments</span>
            </div>
            <button
              onclick={toggleCommentReplies}
              class="text-xs font-mono cursor-pointer {prefs.commentReplies ? 'text-green-500' : 'text-terminal-dim'}"
            >
              {prefs.commentReplies ? '[ ON ]' : '[OFF]'}
            </button>
          </div>

          <!-- Mentions toggle -->
          <div class="flex items-center justify-between">
            <div class="text-xs">
              <span class="text-terminal-fg">mentions</span>
              <span class="text-terminal-dim ml-1">— when someone @mentions you</span>
            </div>
            <button
              onclick={toggleMentions}
              class="text-xs font-mono cursor-pointer {prefs.mentions ? 'text-green-500' : 'text-terminal-dim'}"
            >
              {prefs.mentions ? '[ ON ]' : '[OFF]'}
            </button>
          </div>
        </div>
        <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
          └─────────────────
        </div>
      </div>

      <!-- ── Muted Communities ───────────────────── -->
      <div class="border border-terminal-border bg-terminal-surface font-mono">
        <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim">
          ┌─ muted communities
        </div>
        <div class="p-3 space-y-2">
          {#if prefs.mutedCommunities.length === 0}
            <div class="text-xs text-terminal-dim italic">no muted communities</div>
          {:else}
            {#each prefs.mutedCommunities as community, i}
              <div class="flex items-center justify-between text-xs">
                <div class="flex items-center">
                  <span class="text-terminal-dim mr-1">
                    {i < prefs.mutedCommunities.length - 1 ? '├──' : '└──'}
                  </span>
                  <span class="text-terminal-fg">r/{community}</span>
                </div>
                <button
                  onclick={() => removeMutedCommunity(community)}
                  class="text-red-500 hover:text-red-400 transition-colors cursor-pointer"
                >
                  [x]
                </button>
              </div>
            {/each}
          {/if}

          <!-- Add muted community -->
          <div class="border-t border-terminal-border pt-2 mt-2">
            <div class="text-xs text-terminal-dim mb-1">mute community:</div>
            <div class="flex items-center gap-2">
              <div class="flex items-center flex-1">
                <span class="text-xs text-terminal-dim mr-1">r/</span>
                <input
                  type="text"
                  bind:value={newMutedCommunity}
                  placeholder="community name"
                  onkeydown={(e) => { if (e.key === 'Enter') addMutedCommunity(); }}
                  class="flex-1 bg-terminal-bg border border-terminal-border px-2 py-0.5 text-xs text-terminal-fg placeholder:text-terminal-dim outline-none font-mono focus:border-accent-500 transition-colors"
                />
              </div>
              <button
                onclick={addMutedCommunity}
                disabled={!newMutedCommunity.trim()}
                class="text-xs border border-terminal-border px-2 py-0.5 text-accent-500 hover:border-accent-500 transition-colors disabled:opacity-50 cursor-pointer"
              >
                [add]
              </button>
            </div>
          </div>
        </div>
        <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
          └─────────────────
        </div>
      </div>

      <!-- ── Save button ────────────────────────── -->
      <div class="flex items-center justify-between">
        <div>
          {#if saved}
            <span class="text-xs text-green-500 font-mono">&gt; preferences saved</span>
          {/if}
          {#if error}
            <span class="text-xs text-red-400 font-mono">&gt; error: {error}</span>
          {/if}
        </div>
        <button
          onclick={savePreferences}
          disabled={saving}
          class="text-xs border border-terminal-border px-3 py-1 text-terminal-fg hover:text-accent-500 hover:border-accent-500 transition-colors disabled:opacity-50 font-mono cursor-pointer"
        >
          {saving ? '[saving...]' : '[save preferences]'}
        </button>
      </div>

    </div>
  </div>
{/if}
