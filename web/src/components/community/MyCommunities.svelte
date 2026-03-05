<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { getUser, isAuthenticated, isLoading, whenReady, subscribe } from '../../lib/auth';

  type UserCommunity = {
    communityId: string;
    name: string;
  };

  type UserCommunitiesResponse = {
    communities: UserCommunity[];
  };

  let userCommunities = $state<UserCommunity[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let authed = $state(isAuthenticated());
  let authLoading = $state(isLoading());

  async function fetchMyCommunities() {
    const user = getUser();
    if (!user) {
      userCommunities = [];
      loading = false;
      return;
    }

    loading = true;
    error = null;

    try {
      const data = await api<UserCommunitiesResponse>(
        `/users/${user.userId}/communities?pagination.limit=100`
      );
      userCommunities = data.communities ?? [];
    } catch (e) {
      if (e instanceof ApiError) {
        error = e.message;
      } else {
        error = 'Failed to load your communities';
      }
      userCommunities = [];
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    whenReady().then(() => {
      authed = isAuthenticated();
      authLoading = isLoading();
      if (authed) {
        fetchMyCommunities();
      } else {
        loading = false;
      }
    });

    const unsub = subscribe(() => {
      const wasAuthed = authed;
      authed = isAuthenticated();
      authLoading = isLoading();

      if (authed && !wasAuthed) {
        fetchMyCommunities();
      }
      if (!authed) {
        userCommunities = [];
        loading = false;
      }
    });

    return unsub;
  });
</script>

<div class="max-w-3xl">
  <!-- Header -->
  <div class="box-terminal mb-4">
    <div class="flex items-center justify-between">
      <div>
        <div class="text-accent-500 text-sm">~ /my-communities</div>
        <div class="text-terminal-dim text-xs">communities you've joined</div>
      </div>
      {#if authed}
        <a href="/communities/create" class="text-xs text-accent-500 hover:text-accent-400 transition-colors">
          [+ create community]
        </a>
      {/if}
    </div>
  </div>

  <!-- Not authenticated -->
  {#if !authed && !authLoading}
    <div class="border border-terminal-border bg-terminal-surface px-3 py-4 text-xs font-mono">
      <div class="text-terminal-dim mb-2">you must be logged in to see your communities.</div>
      <a href="/login" class="text-accent-500 hover:text-accent-400 transition-colors">
        &gt; log in
      </a>
    </div>

  <!-- Loading -->
  {:else if loading}
    <div class="px-2 py-4 text-xs text-terminal-dim font-mono">
      <span class="animate-pulse">[loading your communities...]</span>
    </div>

  <!-- Error -->
  {:else if error}
    <div class="px-2 py-4 text-xs font-mono">
      <span class="text-red-500">&gt; error:</span>
      <span class="text-red-400"> {error}</span>
    </div>

  <!-- Empty state -->
  {:else if userCommunities.length === 0}
    <div class="border border-terminal-border bg-terminal-surface px-3 py-4 text-xs font-mono">
      <div class="text-terminal-dim mb-2">you haven't joined any communities yet.</div>
      <a href="/communities" class="text-accent-500 hover:text-accent-400 transition-colors">
        browse communities &rarr;
      </a>
    </div>

  <!-- Community list -->
  {:else}
    <div class="border border-terminal-border bg-terminal-surface">
      {#each userCommunities as community, i}
        <a
          href="/community/{community.name}"
          class="flex items-center text-xs px-3 py-1.5 font-mono hover:bg-terminal-bg transition-colors group {i < userCommunities.length - 1 ? 'border-b border-terminal-border' : ''}"
        >
          <span class="text-terminal-dim mr-1">
            {i < userCommunities.length - 1 ? '├──' : '└──'}
          </span>
          <span class="text-terminal-fg group-hover:text-accent-500 transition-colors">
            r/{community.name}
          </span>
        </a>
      {/each}
    </div>
  {/if}
</div>
