<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';
  import SearchResultRow from './SearchResultRow.svelte';

  type SearchResult = {
    postId: string;
    title: string;
    snippet: string;
    authorUsername: string;
    communityName: string;
    voteScore: number;
    commentCount: number;
    createdAt: string;
  };

  type SearchResponse = {
    results: SearchResult[];
    pagination: {
      nextCursor: string;
      totalCount: number;
      hasMore: boolean;
    };
  };

  type CommunitySuggestion = {
    name: string;
    iconUrl: string;
    memberCount: number;
  };

  let results = $state<SearchResult[]>([]);
  let loading = $state(true);
  let totalHits = $state(0);
  let sort = $state<'relevance' | 'recency' | 'score'>('relevance');
  let query = $state('');
  let community = $state<string | null>(null);
  let nextCursor = $state('');
  let hasMore = $state(false);
  let loadingMore = $state(false);

  // Community sidebar state
  let matchedCommunities = $state<CommunitySuggestion[]>([]);
  let communitiesLoading = $state(false);

  const LIMIT = 25;

  const sortOptions = [
    { value: 'relevance' as const, label: 'relevance' },
    { value: 'recency' as const, label: 'new' },
    { value: 'score' as const, label: 'top' },
  ];

  async function fetchResults(append = false) {
    if (!query) {
      results = [];
      loading = false;
      return;
    }

    if (append) {
      loadingMore = true;
    } else {
      loading = true;
      nextCursor = '';
    }

    try {
      let url = `/search/posts?query=${encodeURIComponent(query)}&pagination.limit=${LIMIT}`;
      if (nextCursor) {
        url += `&pagination.cursor=${encodeURIComponent(nextCursor)}`;
      }
      if (community) {
        url += `&community_name=${encodeURIComponent(community)}`;
      }

      const data = await api<SearchResponse>(url);

      if (append) {
        results = [...results, ...(data.results ?? [])];
      } else {
        results = data.results ?? [];
      }

      nextCursor = data.pagination?.nextCursor ?? '';
      totalHits = data.pagination?.totalCount ?? results.length;
      hasMore = data.pagination?.hasMore ?? false;
    } catch {
      if (!append) results = [];
    } finally {
      loading = false;
      loadingMore = false;
    }
  }

  async function fetchCommunities() {
    if (!query || query.length < 2) {
      matchedCommunities = [];
      return;
    }

    communitiesLoading = true;
    try {
      const data = await api<{ suggestions: CommunitySuggestion[] }>(
        `/search/communities?query=${encodeURIComponent(query)}&limit=10`
      );
      matchedCommunities = data.suggestions ?? [];
    } catch {
      matchedCommunities = [];
    } finally {
      communitiesLoading = false;
    }
  }

  function pinCommunity(name: string) {
    community = name;
    nextCursor = '';
    // Update URL without reload
    const url = new URL(window.location.href);
    url.searchParams.set('community', name);
    window.history.replaceState({}, '', url.toString());
    fetchResults();
  }

  function clearCommunity() {
    community = null;
    nextCursor = '';
    const url = new URL(window.location.href);
    url.searchParams.delete('community');
    window.history.replaceState({}, '', url.toString());
    fetchResults();
  }

  function formatMemberCount(count: number): string {
    if (count >= 1000) return `${(count / 1000).toFixed(1)}k`;
    return String(count);
  }

  // Re-fetch when sort changes
  $effect(() => {
    const _s = sort;
    if (query) {
      nextCursor = '';
      fetchResults();
    }
  });

  function loadMore() {
    fetchResults(true);
  }

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    query = params.get('q') ?? '';
    community = params.get('community') ?? null;
    fetchResults();
    fetchCommunities();
  });
</script>

<div class="flex gap-4 max-w-5xl font-mono">
  <!-- Left sidebar: matching communities -->
  <div class="w-56 shrink-0 hidden lg:block">
    <div class="border border-terminal-border bg-terminal-surface sticky top-4">
      <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim uppercase tracking-wide">
        communities
      </div>

      {#if communitiesLoading}
        <div class="px-3 py-2 text-xs text-terminal-dim animate-pulse">searching...</div>
      {:else if matchedCommunities.length === 0}
        <div class="px-3 py-2 text-xs text-terminal-dim">no matches</div>
      {:else}
        {#each matchedCommunities as comm}
          <button
            class="w-full text-left px-3 py-1.5 text-xs hover:bg-terminal-bg transition-colors flex items-center justify-between group cursor-pointer
              {community === comm.name ? 'bg-terminal-bg border-l-2 border-accent-500' : ''}"
            onclick={() => community === comm.name ? clearCommunity() : pinCommunity(comm.name)}
          >
            <span class="text-terminal-fg group-hover:text-accent-500 transition-colors truncate">
              r/{comm.name}
            </span>
            {#if comm.memberCount > 0}
              <span class="text-terminal-dim text-[10px] shrink-0 ml-1">
                {formatMemberCount(comm.memberCount)}
              </span>
            {/if}
          </button>
        {/each}
      {/if}

      {#if community}
        <div class="px-3 py-1.5 border-t border-terminal-border">
          <button
            class="text-xs text-terminal-dim hover:text-accent-500 transition-colors cursor-pointer"
            onclick={clearCommunity}
          >
            &times; clear filter
          </button>
        </div>
      {/if}
    </div>
  </div>

  <!-- Main content: search results -->
  <div class="flex-1 min-w-0">
    <!-- Search header -->
    <div class="flex items-center justify-between mb-3 border-b border-terminal-border pb-2">
      <div class="text-xs text-terminal-dim">
        {#if query}
          search: <span class="text-terminal-fg">"{query}"</span>
          {#if community}
            <span class="text-terminal-dim ml-1">
              in <span class="text-accent-500">r/{community}</span>
              <button
                class="text-terminal-dim hover:text-accent-500 ml-0.5 cursor-pointer"
                onclick={clearCommunity}
                title="Clear community filter"
              >&times;</button>
            </span>
          {/if}
          {#if !loading}
            <span class="ml-2">({totalHits} results)</span>
          {/if}
        {/if}
      </div>

      <!-- Sort bar -->
      <div class="flex items-center gap-1 text-xs">
        {#each sortOptions as opt}
          <button
            class="px-2 py-0.5 rounded transition-colors cursor-pointer {sort === opt.value
              ? 'bg-terminal-surface text-accent-500 border border-terminal-border'
              : 'text-terminal-dim hover:text-terminal-fg'}"
            onclick={() => { sort = opt.value; }}
          >
            {opt.label}
          </button>
        {/each}
      </div>
    </div>

    <!-- Mobile community filter (visible on small screens) -->
    {#if matchedCommunities.length > 0}
      <div class="lg:hidden mb-3 flex flex-wrap gap-1">
        {#each matchedCommunities.slice(0, 5) as comm}
          <button
            class="text-xs px-2 py-0.5 border transition-colors cursor-pointer
              {community === comm.name
                ? 'border-accent-500 text-accent-500 bg-terminal-surface'
                : 'border-terminal-border text-terminal-dim hover:text-terminal-fg'}"
            onclick={() => community === comm.name ? clearCommunity() : pinCommunity(comm.name)}
          >
            r/{comm.name}
          </button>
        {/each}
      </div>
    {/if}

    <!-- Results -->
    {#if loading}
      <div class="text-xs text-terminal-dim py-8 text-center">
        searching...
      </div>
    {:else if results.length === 0}
      <div class="text-xs text-terminal-dim py-8 text-center">
        No results found for '{query}'
        {#if community}
          <span>in r/{community}</span>
        {/if}
      </div>
    {:else}
      <div class="border border-terminal-border rounded">
        {#each results as result (result.postId)}
          <SearchResultRow {result} />
        {/each}
      </div>

      {#if hasMore}
        <div class="text-center py-3">
          <button
            class="text-xs text-accent-500 hover:text-accent-400 transition-colors cursor-pointer font-mono"
            onclick={loadMore}
            disabled={loadingMore}
          >
            {loadingMore ? 'loading...' : '[ load more ]'}
          </button>
        </div>
      {/if}
    {/if}
  </div>
</div>
