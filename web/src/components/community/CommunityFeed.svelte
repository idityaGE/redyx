<script lang="ts">
  import SortBar from '../feed/SortBar.svelte';
  import FeedList from '../feed/FeedList.svelte';

  interface Props {
    communityName: string;
    isMember?: boolean;
  }

  let { communityName, isMember = false }: Props = $props();

  let sort = $state('SORT_ORDER_HOT');
  let timeRange = $state<string | undefined>(undefined);

  function handleSortChange(newSort: string, newTimeRange?: string) {
    sort = newSort;
    timeRange = newTimeRange;
  }
</script>

{#if isMember}
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
