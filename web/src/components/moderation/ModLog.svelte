<script lang="ts">
  import { api } from '../../lib/api';
  import { relativeTime } from '../../lib/time';

  interface Props {
    communityName: string;
  }

  let { communityName }: Props = $props();

  type ModLogEntry = {
    entryId: string;
    moderatorId: string;
    moderatorUsername: string;
    action: string;
    targetType: string;
    targetId: string;
    reason: string;
    createdAt: string;
  };

  type ModLogResponse = {
    entries: ModLogEntry[];
    pagination: { nextCursor: string; hasMore: boolean; totalCount: number };
  };

  const actionTypes = [
    { value: '', label: 'All' },
    { value: 'MOD_ACTION_REMOVE_POST', label: 'Remove Post' },
    { value: 'MOD_ACTION_REMOVE_COMMENT', label: 'Remove Comment' },
    { value: 'MOD_ACTION_BAN_USER', label: 'Ban User' },
    { value: 'MOD_ACTION_UNBAN_USER', label: 'Unban User' },
    { value: 'MOD_ACTION_PIN_POST', label: 'Pin Post' },
    { value: 'MOD_ACTION_UNPIN_POST', label: 'Unpin Post' },
    { value: 'MOD_ACTION_DISMISS_REPORT', label: 'Dismiss Report' },
    { value: 'MOD_ACTION_RESTORE_CONTENT', label: 'Restore Content' },
  ];

  let selectedFilter = $state('');
  let entries = $state<ModLogEntry[]>([]);
  let loading = $state(true);
  let nextCursor = $state('');
  let loadingMore = $state(false);

  async function fetchLog(append = false) {
    if (!append) {
      loading = true;
      nextCursor = '';
    } else {
      loadingMore = true;
    }

    try {
      const params = new URLSearchParams();
      if (selectedFilter) params.set('action_filter', selectedFilter);
      if (append && nextCursor) params.set('cursor', nextCursor);

      const data = await api<ModLogResponse>(
        `/communities/${encodeURIComponent(communityName)}/moderation/log?${params.toString()}`
      );
      if (append) {
        entries = [...entries, ...(data.entries ?? [])];
      } else {
        entries = data.entries ?? [];
      }
      nextCursor = data.pagination?.nextCursor ?? '';
    } catch {
      if (!append) entries = [];
    } finally {
      loading = false;
      loadingMore = false;
    }
  }

  $effect(() => {
    const _filter = selectedFilter;
    fetchLog();
  });

  function formatAction(action: string): string {
    // Strip MOD_ACTION_ prefix and format nicely
    const clean = action.replace(/^MOD_ACTION_/, '');
    return clean.toLowerCase().replace(/_/g, ' ');
  }
</script>

<div class="font-mono">
  <!-- Action type filter -->
  <div class="mb-4">
    <div class="text-xs text-terminal-dim mb-1">filter by action:</div>
    <select
      bind:value={selectedFilter}
      class="bg-terminal-bg border border-terminal-border text-xs text-terminal-fg px-2 py-1 font-mono outline-none focus:border-accent-500 transition-colors"
    >
      {#each actionTypes as at}
        <option value={at.value}>{at.label}</option>
      {/each}
    </select>
  </div>

  <!-- Loading -->
  {#if loading}
    <div class="text-xs text-terminal-dim animate-pulse">[loading mod log...]</div>
  {:else if entries.length === 0}
    <div class="text-xs text-terminal-dim italic">no log entries</div>
  {:else}
    <!-- Log entries -->
    <div class="space-y-1">
      {#each entries as entry (entry.entryId)}
        <div class="border border-terminal-border bg-terminal-surface px-3 py-2 text-xs">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="text-terminal-dim">{relativeTime(entry.createdAt)}</span>
            <span class="text-accent-500">@{entry.moderatorUsername}</span>
            <span class="text-terminal-fg">{formatAction(entry.action)}</span>
            <span class="text-terminal-dim">{entry.targetType}: {entry.targetId.slice(0, 8)}...</span>
          </div>
          {#if entry.reason}
            <div class="text-terminal-dim mt-0.5 pl-2">
              reason: {entry.reason}
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Load more -->
    {#if nextCursor}
      <div class="mt-3">
        <button
          onclick={() => fetchLog(true)}
          disabled={loadingMore}
          class="text-xs text-terminal-dim hover:text-accent-500 transition-colors"
        >
          {loadingMore ? '[loading...]' : '[load more]'}
        </button>
      </div>
    {/if}
  {/if}
</div>
