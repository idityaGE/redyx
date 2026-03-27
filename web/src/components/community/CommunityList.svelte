<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { isAuthenticated, isLoading, subscribe } from '../../lib/auth';

  type Community = {
    communityId: string;
    name: string;
    description: string;
    visibility: string;
    memberCount: number;
    createdAt: string;
  };

  type CommunitiesResponse = {
    communities: Community[];
    pagination: {
      nextCursor?: string;
      prevCursor?: string;
    };
  };

  let sortBy = $state<'members' | 'created'>('members');
  let communities = $state<Community[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let nextCursor = $state<string | null>(null);
  let prevCursor = $state<string | null>(null);
  let authed = $state(isAuthenticated());
  let authLoading = $state(isLoading());
  let searchQuery = $state('');
  let debounceTimer: ReturnType<typeof setTimeout>;

  const visibilityLabel = (v: string): string => {
    switch (v) {
      case 'VISIBILITY_PUBLIC': return 'public';
      case 'VISIBILITY_RESTRICTED': return 'restricted';
      case 'VISIBILITY_PRIVATE': return 'private';
      default: return 'public';
    }
  };

  const formatDate = (iso: string): string => {
    try {
      const d = new Date(iso);
      return d.toISOString().slice(0, 10);
    } catch {
      return '—';
    }
  };

  async function fetchCommunities(cursor?: string | null, direction?: 'next' | 'prev') {
    loading = true;
    error = null;

    try {
      // Note: sort is client-side only — the proto ListCommunitiesRequest has only pagination + query
      let path = `/communities?pagination.limit=25`;
      if (searchQuery.trim()) {
        path += `&query=${encodeURIComponent(searchQuery.trim())}`;
      }
      if (cursor && direction === 'next') {
        path += `&pagination.cursor=${encodeURIComponent(cursor)}`;
      } else if (cursor && direction === 'prev') {
        path += `&pagination.prevCursor=${encodeURIComponent(cursor)}`;
      }

      const data = await api<CommunitiesResponse>(path);
      communities = data.communities ?? [];
      nextCursor = data.pagination?.nextCursor ?? null;
      prevCursor = data.pagination?.prevCursor ?? null;
    } catch (e) {
      if (e instanceof ApiError) {
        error = e.message;
      } else {
        error = 'Failed to load communities';
      }
      communities = [];
    } finally {
      loading = false;
    }
  }

  function changeSort(newSort: 'members' | 'created') {
    sortBy = newSort;
    nextCursor = null;
    prevCursor = null;
    fetchCommunities();
  }

  function handleSearchInput() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
      nextCursor = null;
      prevCursor = null;
      fetchCommunities();
    }, 300);
  }

  onMount(() => {
    const unsub = subscribe(() => {
      authed = isAuthenticated();
      authLoading = isLoading();
    });

    fetchCommunities();

    return unsub;
  });
</script>

<div class="max-w-3xl">
  <!-- Header -->
  <div class="box-terminal mb-4">
    <div class="flex items-center justify-between">
      <div>
        <div class="text-accent-500 text-sm">~ /communities</div>
        <div class="text-terminal-dim text-xs mt-0.5">
          browse all communities
          {#if authed}
            &middot; <a href="/my-communities" class="text-accent-600 hover:text-accent-500 transition-colors">my communities</a>
          {/if}
        </div>
      </div>
      {#if authed}
        <a href="/communities/create" class="text-xs text-accent-500 hover:text-accent-400 transition-colors">
          [+ create community]
        </a>
      {:else if !authLoading}
        <span class="text-xs text-terminal-dim">&gt; login required to create communities</span>
      {/if}
    </div>
  </div>

  <!-- Search bar -->
  <div class="flex items-center gap-1 text-xs font-mono mb-3 px-1 border border-terminal-border bg-terminal-surface py-1.5 px-2">
    <span class="text-terminal-dim">&gt;</span>
    <input
      type="text"
      placeholder="search communities..."
      bind:value={searchQuery}
      oninput={handleSearchInput}
      class="bg-transparent text-terminal-fg placeholder:text-terminal-dim outline-none flex-1 font-mono text-xs"
    />
  </div>

  <!-- Sort controls -->
  <div class="flex items-center gap-1 text-xs font-mono mb-3 px-1">
    <span class="text-terminal-dim">sort:</span>
    {#each ['members', 'created'] as s}
      <button
        class="px-2 py-0.5 border transition-colors {sortBy === s
          ? 'border-accent-500 text-accent-500 bg-terminal-surface'
          : 'border-terminal-border text-terminal-dim hover:text-terminal-fg hover:border-terminal-fg'}"
        onclick={() => changeSort(s as 'members' | 'created')}
      >
        [{s}]
      </button>
    {/each}
  </div>

  <!-- Loading state -->
  {#if loading}
    <div class="px-2 py-4 text-xs text-terminal-dim font-mono">
      <span class="animate-pulse">[loading communities...]</span>
    </div>
  {:else if error}
    <div class="px-2 py-4 text-xs font-mono">
      <span class="text-red-500">&gt; error:</span>
      <span class="text-red-400"> {error}</span>
    </div>
  {:else if communities.length === 0}
    <div class="px-2 py-4 text-xs text-terminal-dim font-mono">
      no communities found. be the first to create one.
    </div>
  {:else}
    <!-- Community table -->
    <div class="border border-terminal-border bg-terminal-surface">
      <!-- Table header -->
      <div class="flex items-center text-xs text-terminal-dim px-3 py-1.5 border-b border-terminal-border font-mono">
        <span class="flex-1">name</span>
        <span class="w-28 text-right">members</span>
        <span class="w-24 text-right hidden sm:block">visibility</span>
        <span class="w-28 text-right hidden md:block">created</span>
      </div>

      <!-- Rows -->
      {#each communities as community, i}
        <a
          href="/community/{community.name}"
          class="flex items-center text-xs px-3 py-1.5 font-mono hover:bg-terminal-bg transition-colors group {i < communities.length - 1 ? 'border-b border-terminal-border' : ''}"
        >
          <span class="flex-1 min-w-0">
            <span class="text-terminal-dim mr-1">{i < communities.length - 1 ? '├──' : '└──'}</span>
            <span class="text-terminal-fg group-hover:text-accent-500 transition-colors">r/{community.name}</span>
          </span>
          <span class="w-28 text-right text-terminal-dim">
            {community.memberCount.toLocaleString()} members
          </span>
          <span class="w-24 text-right text-terminal-dim hidden sm:block">
            {visibilityLabel(community.visibility)}
          </span>
          <span class="w-28 text-right text-terminal-dim hidden md:block">
            {formatDate(community.createdAt)}
          </span>
        </a>
      {/each}
    </div>

    <!-- Pagination -->
    <div class="flex items-center justify-between text-xs font-mono px-1 mt-2">
      {#if prevCursor}
        <button
          class="text-terminal-dim hover:text-accent-500 transition-colors"
          onclick={() => fetchCommunities(prevCursor, 'prev')}
        >
          &lt; prev
        </button>
      {:else}
        <span></span>
      {/if}
      {#if nextCursor}
        <button
          class="text-terminal-dim hover:text-accent-500 transition-colors"
          onclick={() => fetchCommunities(nextCursor, 'next')}
        >
          next &gt;
        </button>
      {:else}
        <span></span>
      {/if}
    </div>
  {/if}
</div>
