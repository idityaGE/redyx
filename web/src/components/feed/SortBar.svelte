<script lang="ts">
  interface Props {
    sort: string;
    timeRange?: string;
    onSortChange: (sort: string, timeRange?: string) => void;
  }

  let { sort, timeRange, onSortChange }: Props = $props();

  const sortTabs = [
    { id: 'SORT_ORDER_HOT', label: 'Hot' },
    { id: 'SORT_ORDER_NEW', label: 'New' },
    { id: 'SORT_ORDER_TOP', label: 'Top' },
    { id: 'SORT_ORDER_RISING', label: 'Rising' },
  ];

  const timeRanges = [
    { id: 'TIME_RANGE_HOUR', label: 'Hour' },
    { id: 'TIME_RANGE_DAY', label: 'Day' },
    { id: 'TIME_RANGE_WEEK', label: 'Week' },
    { id: 'TIME_RANGE_MONTH', label: 'Month' },
    { id: 'TIME_RANGE_YEAR', label: 'Year' },
    { id: 'TIME_RANGE_ALL', label: 'All' },
  ];

  function handleSortClick(newSort: string) {
    if (newSort === 'SORT_ORDER_TOP') {
      // Default to Day when switching to Top
      onSortChange(newSort, timeRange || 'TIME_RANGE_DAY');
    } else {
      onSortChange(newSort, undefined);
    }
  }

  function handleTimeRangeClick(newTimeRange: string) {
    onSortChange(sort, newTimeRange);
  }
</script>

<div class="border border-terminal-border bg-terminal-surface font-mono text-xs">
  <!-- Sort tabs -->
  <div class="flex border-b border-terminal-border">
    {#each sortTabs as tab}
      <button
        class="px-4 py-1.5 transition-colors cursor-pointer {sort === tab.id
          ? 'bg-terminal-bg text-accent-500 border-b-2 border-accent-500'
          : 'text-terminal-dim hover:text-terminal-fg hover:bg-terminal-bg'}"
        onclick={() => handleSortClick(tab.id)}
      >
        [{tab.label}]
      </button>
    {/each}
  </div>

  <!-- Time range filter (only visible when Top is selected) -->
  {#if sort === 'SORT_ORDER_TOP'}
    <div class="flex border-t border-terminal-border px-2 py-1 gap-1">
      {#each timeRanges as range}
        <button
          class="px-2 py-0.5 transition-colors cursor-pointer rounded-sm {timeRange === range.id
            ? 'bg-terminal-bg text-accent-500'
            : 'text-terminal-dim hover:text-terminal-fg'}"
          onclick={() => handleTimeRangeClick(range.id)}
        >
          {range.label}
        </button>
      {/each}
    </div>
  {/if}
</div>
