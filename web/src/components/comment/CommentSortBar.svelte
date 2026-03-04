<script lang="ts">
  interface Props {
    sort: string;
    onSortChange: (sort: string) => void;
  }

  let { sort, onSortChange }: Props = $props();

  const sortTabs = [
    { id: 'COMMENT_SORT_ORDER_BEST', label: 'Best' },
    { id: 'COMMENT_SORT_ORDER_TOP', label: 'Top' },
    { id: 'COMMENT_SORT_ORDER_NEW', label: 'New' },
  ];
</script>

<div class="flex items-center gap-1 font-mono text-xs">
  {#each sortTabs as tab}
    <button
      class="px-2 py-1 transition-colors cursor-pointer {sort === tab.id
        ? 'text-accent-500 border-b border-accent-500'
        : 'text-terminal-dim hover:text-terminal-fg'}"
      onclick={() => onSortChange(tab.id)}
    >
      [{tab.label}]
    </button>
  {/each}
</div>
