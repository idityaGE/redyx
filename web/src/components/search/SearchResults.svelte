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
      totalCount: number;
      limit: number;
      offset: number;
      hasMore: boolean;
    };
  };

  let results = $state<SearchResult[]>([]);
  let loading = $state(true);
  let totalHits = $state(0);
  let sort = $state<'relevance' | 'recency' | 'score'>('relevance');
  let query = $state('');
  let community = $state<string | null>(null);
  let offset = $state(0);
  let hasMore = $state(false);
  let loadingMore = $state(false);

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
      offset = 0;
    }

    try {
      let url = `/search/posts?query=${encodeURIComponent(query)}&pagination.limit=${LIMIT}&pagination.offset=${offset}`;
      if (community) {
        url += `&communityName=${encodeURIComponent(community)}`;
      }
      if (sort === 'recency') {
        url += '&sort=createdAt:desc';
      } else if (sort === 'score') {
        url += '&sort=voteScore:desc';
      }

      const data = await api<SearchResponse>(url);

      if (append) {
        results = [...results, ...(data.results ?? [])];
      } else {
        results = data.results ?? [];
      }

      totalHits = data.pagination?.totalCount ?? results.length;
      hasMore = data.pagination?.hasMore ?? false;
    } catch {
      if (!append) results = [];
    } finally {
      loading = false;
      loadingMore = false;
    }
  }

  // Re-fetch when sort changes
  $effect(() => {
    // Subscribe to sort value
    const _s = sort;
    if (query) {
      offset = 0;
      fetchResults();
    }
  });

  function loadMore() {
    offset += LIMIT;
    fetchResults(true);
  }

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    query = params.get('q') ?? '';
    community = params.get('community') ?? null;
    fetchResults();
  });
</script>

<div class="max-w-3xl mx-auto font-mono">
  <!-- Search header -->
  <div class="flex items-center justify-between mb-3 border-b border-terminal-border pb-2">
    <div class="text-xs text-terminal-dim">
      {#if query}
        search: <span class="text-terminal-fg">"{query}"</span>
        {#if community}
          <span class="text-terminal-dim ml-1">in r/{community}</span>
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

  <!-- Results -->
  {#if loading}
    <div class="text-xs text-terminal-dim py-8 text-center">
      searching...
    </div>
  {:else if results.length === 0}
    <div class="text-xs text-terminal-dim py-8 text-center">
      No results found for '{query}'
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
