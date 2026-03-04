<script lang="ts">
  import { api } from '../../lib/api';

  /** Community suggestion from autocomplete */
  type CommunitySuggestion = {
    name: string;
    iconUrl: string;
    memberCount: number;
  };

  let query = $state('');
  let suggestions = $state<CommunitySuggestion[]>([]);
  let showDropdown = $state(false);

  // Auto-detect community scope from URL path
  let detectedScope = $derived(
    typeof window !== 'undefined'
      ? window.location.pathname.match(/^\/community\/([^/]+)/)?.[1] ?? null
      : null
  );

  // Writable scope that user can clear
  let communityScope = $state<string | null>(null);

  // Sync detected scope on mount
  $effect(() => {
    if (detectedScope !== null) {
      communityScope = detectedScope;
    }
  });

  // Debounced autocomplete fetch
  let debounceTimer: ReturnType<typeof setTimeout> | null = null;

  $effect(() => {
    const q = query;
    if (debounceTimer) clearTimeout(debounceTimer);

    if (q.length < 2) {
      suggestions = [];
      showDropdown = false;
      return;
    }

    debounceTimer = setTimeout(async () => {
      try {
        const data = await api<{ suggestions: CommunitySuggestion[] }>(
          `/search/communities?query=${encodeURIComponent(q)}&limit=8`
        );
        suggestions = data.suggestions ?? [];
        showDropdown = suggestions.length > 0;
      } catch {
        suggestions = [];
        showDropdown = false;
      }
    }, 300);

    return () => {
      if (debounceTimer) clearTimeout(debounceTimer);
    };
  });

  // Click outside to close dropdown
  $effect(() => {
    if (!showDropdown) return;

    function handleClickOutside(e: MouseEvent) {
      const target = e.target as HTMLElement;
      if (!target.closest('.search-bar-container')) {
        showDropdown = false;
      }
    }

    document.addEventListener('click', handleClickOutside);
    return () => document.removeEventListener('click', handleClickOutside);
  });

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && query.trim()) {
      let url = `/search?q=${encodeURIComponent(query.trim())}`;
      if (communityScope) {
        url += `&community=${encodeURIComponent(communityScope)}`;
      }
      showDropdown = false;
      window.location.href = url;
    } else if (e.key === 'Escape') {
      showDropdown = false;
      suggestions = [];
    }
  }

  function navigateToCommunity(name: string) {
    showDropdown = false;
    query = '';
    window.location.href = `/community/${name}`;
  }

  function clearScope() {
    communityScope = null;
  }

  function formatMemberCount(count: number): string {
    if (count >= 1000) return `${(count / 1000).toFixed(1)}k`;
    return String(count);
  }
</script>

<div class="flex-1 max-w-md mx-4 relative search-bar-container">
  <div class="flex items-center bg-terminal-bg border border-terminal-border rounded px-2 h-7">
    <span class="text-accent-500 text-xs mr-1">&gt;</span>

    {#if communityScope}
      <span class="inline-flex items-center text-xs bg-terminal-surface border border-terminal-border rounded px-1 mr-1 text-accent-500 whitespace-nowrap">
        r/{communityScope}
        <button
          class="ml-0.5 text-terminal-dim hover:text-accent-500 transition-colors"
          onclick={clearScope}
          title="Remove community scope"
        >&times;</button>
      </span>
    {/if}

    <input
      type="text"
      placeholder="search..."
      bind:value={query}
      onkeydown={handleKeydown}
      class="bg-transparent text-xs text-terminal-fg placeholder:text-terminal-dim outline-none w-full font-mono"
    />
  </div>

  {#if showDropdown && suggestions.length > 0}
    <div class="absolute top-full left-0 right-0 mt-1 bg-terminal-bg border border-terminal-border rounded shadow-lg z-50 max-h-64 overflow-y-auto">
      {#each suggestions as suggestion}
        <button
          class="w-full text-left px-3 py-1.5 text-xs font-mono flex items-center justify-between hover:bg-terminal-surface transition-colors cursor-pointer"
          onclick={() => navigateToCommunity(suggestion.name)}
        >
          <span class="text-terminal-fg">r/{suggestion.name}</span>
          <span class="text-terminal-dim">{formatMemberCount(suggestion.memberCount)} members</span>
        </button>
      {/each}
    </div>
  {/if}
</div>
