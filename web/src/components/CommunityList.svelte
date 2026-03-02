<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { isAuthenticated, isLoading, subscribe } from '../lib/auth';

  type Community = {
    communityId: string;
    name: string;
    description: string;
    visibility: number;
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

  let sortBy = $state<'members' | 'created' | 'activity'>('members');
  let communities = $state<Community[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let nextCursor = $state<string | null>(null);
  let prevCursor = $state<string | null>(null);
  let authed = $state(isAuthenticated());
  let authLoading = $state(isLoading());

  const visibilityLabel = (v: number): string => {
    switch (v) {
      case 1: return 'public';
      case 2: return 'restricted';
      case 3: return 'private';
      default: return 'unknown';
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
      let path = `/communities?pagination.limit=25&sort=${sortBy}`;
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

  function changeSort(newSort: 'members' | 'created' | 'activity') {
    sortBy = newSort;
    nextCursor = null;
    prevCursor = null;
    fetchCommunities();
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
      <div class="text-accent-500 text-sm">~ /communities</div>
      {#if authed}
        <a href="/communities/create" class="text-xs text-accent-500 hover:text-accent-400 transition-colors">
          [+ create community]
        </a>
      {:else if !authLoading}
        <span class="text-xs text-terminal-dim">&gt; login required to create communities</span>
      {/if}
    </div>
  </div>

  <!-- Sort controls -->
  <div class="flex items-center gap-1 text-xs font-mono mb-3 px-1">
    <span class="text-terminal-dim">sort:</span>
    {#each ['members', 'created', 'activity'] as s}
      <button
        class="px-2 py-0.5 border transition-colors {sortBy === s
          ? 'border-accent-500 text-accent-500 bg-terminal-surface'
          : 'border-terminal-border text-terminal-dim hover:text-terminal-fg hover:border-terminal-fg'}"
        onclick={() => changeSort(s as 'members' | 'created' | 'activity')}
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
