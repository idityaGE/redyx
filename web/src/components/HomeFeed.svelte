<script lang="ts">
  import { onMount } from 'svelte';
  import { isAuthenticated, subscribe } from '../lib/auth';
  import SortBar from './SortBar.svelte';
  import FeedList from './FeedList.svelte';

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

<!-- Welcome header -->
<div class="box-terminal mb-4">
  <div class="text-accent-500 text-sm mb-2">~ welcome to redyx</div>
  <div class="text-terminal-dim text-xs">
    a terminal-aesthetic community platform &middot; information-dense &middot; privacy-first
  </div>
  {#if !authed}
    <div class="text-terminal-dim text-xs mt-2">
      <a href="/login" class="text-accent-600 hover:text-accent-500">log in</a> or
      <a href="/register" class="text-accent-600 hover:text-accent-500">register</a>
      to join communities and customize your feed
    </div>
  {/if}
</div>

<!-- Sort controls -->
<div class="mb-3">
  <SortBar {sort} {timeRange} onSortChange={handleSortChange} />
</div>

<!-- Feed with infinite scroll -->
<FeedList endpoint="/feed" {sort} {timeRange} />
