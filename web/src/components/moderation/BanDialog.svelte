<script lang="ts">
  import { api, ApiError } from '../../lib/api';

  interface Props {
    communityName: string;
    userId: string;
    username: string;
    onClose: () => void;
    onBanned: () => void;
  }

  let { communityName, userId, username, onClose, onBanned }: Props = $props();

  const durations = [
    { label: '1 day', seconds: 86400 },
    { label: '3 days', seconds: 259200 },
    { label: '7 days', seconds: 604800 },
    { label: '30 days', seconds: 2592000 },
    { label: 'Permanent', seconds: 0 },
  ];

  let selectedDuration = $state<number | null>(null);
  let reason = $state('');
  let removeContent = $state(false);
  let submitting = $state(false);
  let statusMessage = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  let canSubmit = $derived(selectedDuration !== null && reason.trim().length > 0 && !submitting);

  async function banUser() {
    if (!canSubmit || selectedDuration === null) return;
    submitting = true;
    statusMessage = null;

    try {
      await api(`/communities/${encodeURIComponent(communityName)}/moderation/ban`, {
        method: 'POST',
        body: JSON.stringify({
          userId,
          username,
          reason: reason.trim(),
          durationSeconds: selectedDuration,
          removeContent,
        }),
      });
      onBanned();
      onClose();
    } catch (e) {
      statusMessage = {
        type: 'error',
        text: e instanceof ApiError ? e.message : 'failed to ban user',
      };
    } finally {
      submitting = false;
    }
  }

  function handleOverlayClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      onClose();
    }
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/70"
  onclick={handleOverlayClick}
>
  <div class="bg-terminal-bg border border-terminal-border p-4 max-w-md w-full font-mono">
    <!-- Title -->
    <div class="text-xs text-terminal-dim mb-3">
      ┌─ ban user: {username}
    </div>

    <!-- Duration picker -->
    <div class="mb-3">
      <div class="text-xs text-terminal-dim mb-1">duration:</div>
      <div class="space-y-1">
        {#each durations as dur}
          <button
            onclick={() => { selectedDuration = dur.seconds; }}
            class="w-full text-left text-xs px-2 py-1 transition-colors {selectedDuration === dur.seconds ? 'text-accent-500 bg-terminal-surface' : 'text-terminal-fg hover:text-accent-500 hover:bg-terminal-surface'}"
          >
            &gt; [{selectedDuration === dur.seconds ? '●' : '○'}] {dur.label}
          </button>
        {/each}
      </div>
    </div>

    <!-- Reason input -->
    <div class="mb-3">
      <div class="text-xs text-terminal-dim mb-1">reason (required):</div>
      <input
        type="text"
        bind:value={reason}
        placeholder="Ban reason (required)"
        class="w-full bg-terminal-surface border border-terminal-border px-2 py-1 text-xs text-terminal-fg placeholder:text-terminal-dim outline-none font-mono focus:border-accent-500 transition-colors"
      />
    </div>

    <!-- Remove content checkbox -->
    <label class="flex items-center gap-2 mb-4 cursor-pointer text-xs text-terminal-fg">
      <input
        type="checkbox"
        bind:checked={removeContent}
        class="accent-accent-500"
      />
      Also remove all posts/comments by this user
    </label>

    <!-- Status message -->
    {#if statusMessage}
      <div class="text-xs mb-3 {statusMessage.type === 'success' ? 'text-green-500' : 'text-red-400'}">
        &gt; {statusMessage.text}
      </div>
    {/if}

    <!-- Action buttons -->
    <div class="flex items-center justify-between">
      <button
        onclick={onClose}
        class="text-xs border border-terminal-border px-2 py-0.5 text-terminal-dim hover:text-terminal-fg hover:border-terminal-fg transition-colors"
      >
        [cancel]
      </button>
      <button
        onclick={banUser}
        disabled={!canSubmit}
        class="text-xs border border-terminal-border px-2 py-0.5 text-red-500 hover:border-red-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {submitting ? '[banning...]' : '[ban user]'}
      </button>
    </div>

    <!-- Footer -->
    <div class="text-xs text-terminal-dim mt-3">
      └─────────────
    </div>
  </div>
</div>
