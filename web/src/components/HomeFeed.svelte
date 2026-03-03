<script lang="ts">
  import SortBar from './SortBar.svelte';
  import FeedList from './FeedList.svelte';

  let sort = $state('SORT_ORDER_HOT');
  let timeRange = $state<string | undefined>(undefined);

  function handleSortChange(newSort: string, newTimeRange?: string) {
    sort = newSort;
    timeRange = newTimeRange;
  }
</script>

<div class="box-terminal mb-4">
  <div class="text-accent-500 text-sm mb-2">~ welcome to redyx</div>
  <div class="text-terminal-dim text-xs">
    a terminal-aesthetic community platform &middot; information-dense &middot; privacy-first
  </div>
</div>

<div class="mb-3">
  <SortBar {sort} {timeRange} onSortChange={handleSortChange} />
</div>

<FeedList endpoint="/feed" {sort} {timeRange} />
