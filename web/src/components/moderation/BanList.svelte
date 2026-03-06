<script lang="ts">
  import { api } from '../../lib/api';
  import { relativeTime } from '../../lib/time';

  interface Props {
    communityName: string;
  }

  let { communityName }: Props = $props();

  type Ban = {
    userId: string;
    username: string;
    reason: string;
    durationSeconds: number;
    bannedAt: string;
    expiresAt: string;
  };

  type BanListResponse = {
    bans: Ban[];
  };

  let bans = $state<Ban[]>([]);
  let loading = $state(true);

  // Inline confirmation state
  let confirmingUnban = $state<string | null>(null);
  let actionInProgress = $state<string | null>(null);

  async function fetchBans() {
    loading = true;
    try {
      const data = await api<BanListResponse>(
        `/communities/${encodeURIComponent(communityName)}/moderation/bans`
      );
      bans = data.bans ?? [];
    } catch {
      bans = [];
    } finally {
      loading = false;
    }
  }

  function formatDuration(seconds: number): string {
    if (seconds === 0) return 'Permanent';
    if (seconds === 86400) return '1 day';
    if (seconds === 259200) return '3 days';
    if (seconds === 604800) return '7 days';
    if (seconds === 2592000) return '30 days';
    const days = Math.floor(seconds / 86400);
    return `${days} day${days !== 1 ? 's' : ''}`;
  }

  function formatExpiry(expiresAt: string, durationSeconds: number): string {
    if (durationSeconds === 0) return 'Permanent';
    if (!expiresAt) return 'Unknown';
    const d = new Date(expiresAt);
    return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  async function unbanUser(ban: Ban) {
    actionInProgress = ban.userId;
    try {
      await api(`/communities/${encodeURIComponent(communityName)}/moderation/unban`, {
        method: 'POST',
        body: JSON.stringify({ userId: ban.userId }),
      });
      confirmingUnban = null;
      await fetchBans();
    } catch {
      // Silently fail
    } finally {
      actionInProgress = null;
    }
  }

  // Fetch on mount
  $effect(() => {
    fetchBans();
  });
</script>

<div class="font-mono">
  {#if loading}
    <div class="text-xs text-terminal-dim animate-pulse">[loading ban list...]</div>
  {:else if bans.length === 0}
    <div class="text-xs text-terminal-dim italic">no active bans</div>
  {:else}
    <div class="space-y-2">
      {#each bans as ban (ban.userId)}
        <div class="border border-terminal-border bg-terminal-surface px-3 py-2 text-xs">
          <div class="flex items-center justify-between mb-1">
            <div class="flex items-center gap-2">
              <span class="text-terminal-fg">u/{ban.username}</span>
              <span class="text-terminal-dim">&middot;</span>
              <span class="text-terminal-dim">{formatDuration(ban.durationSeconds)}</span>
            </div>

            <!-- Unban action -->
            {#if confirmingUnban === ban.userId}
              <span class="text-yellow-400">
                unban?
                <button
                  onclick={() => unbanUser(ban)}
                  disabled={actionInProgress === ban.userId}
                  class="text-yellow-500 hover:text-yellow-400 underline cursor-pointer disabled:opacity-50"
                >
                  [confirm]
                </button>
                <button
                  onclick={() => { confirmingUnban = null; }}
                  class="text-terminal-dim hover:text-terminal-fg underline cursor-pointer"
                >
                  [cancel]
                </button>
              </span>
            {:else}
              <button
                onclick={() => { confirmingUnban = ban.userId; }}
                class="text-terminal-dim hover:text-accent-500 transition-colors border border-terminal-border px-1.5 py-0.5"
              >
                [unban]
              </button>
            {/if}
          </div>

          <div class="text-terminal-dim space-y-0.5">
            <div>reason: {ban.reason || '(none)'}</div>
            <div>banned: {relativeTime(ban.bannedAt)}</div>
            <div>expires: {formatExpiry(ban.expiresAt, ban.durationSeconds)}</div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>
