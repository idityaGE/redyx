<script lang="ts">
  import { onMount } from 'svelte';
  import { isAuthenticated, subscribe } from '../lib/auth';
  import SortBar from './SortBar.svelte';
  import FeedList from './FeedList.svelte';

  interface Props {
    communityName: string;
  }

  let { communityName }: Props = $props();

  let sort = $state('SORT_ORDER_HOT');
  let timeRange = $state<string | undefined>(undefined);
  let authed = $state(isAuthenticated());

  onMount(() => {
    const unsub = subscribe(() => {
      authed = isAuthenticated();
    });
    return unsub;
  });

  function handleSortChange(newSort: string, newTimeRange?: string) {
    sort = newSort;
    timeRange = newTimeRange;
  }
</script>

{#if authed}
  <div class="mb-3">
    <a
      href="/community/{communityName}/submit"
      class="inline-block text-xs font-mono px-3 py-1.5 border border-terminal-border bg-terminal-surface text-accent-500 hover:bg-terminal-bg hover:text-accent-400 transition-colors"
    >
      [+ create post]
    </a>
  </div>
{/if}

<div class="mb-3">
  <SortBar {sort} {timeRange} onSortChange={handleSortChange} />
</div>

<FeedList endpoint={`/communities/${communityName}/posts`} {sort} {timeRange} />
