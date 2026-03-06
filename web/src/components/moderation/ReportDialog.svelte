<script lang="ts">
  import { api, ApiError } from '../../lib/api';

  interface Props {
    communityName: string;
    contentId: string;
    contentType: 'post' | 'comment';
    onClose: () => void;
  }

  let { communityName, contentId, contentType, onClose }: Props = $props();

  const reasons = [
    'Spam',
    'Harassment',
    'Misinformation',
    'Breaks community rules',
    'Other',
  ];

  let selectedReason = $state('');
  let submitting = $state(false);
  let statusMessage = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  async function submitReport() {
    if (!selectedReason || submitting) return;
    submitting = true;
    statusMessage = null;

    try {
      await api(`/communities/${encodeURIComponent(communityName)}/moderation/reports`, {
        method: 'POST',
        body: JSON.stringify({
          contentId,
          contentType: contentType === 'post' ? 1 : 2,
          reason: selectedReason,
        }),
      });
      statusMessage = { type: 'success', text: 'report submitted' };
      // Auto-dismiss after 2s
      setTimeout(() => {
        onClose();
      }, 2000);
    } catch (e) {
      statusMessage = {
        type: 'error',
        text: e instanceof ApiError ? e.message : 'failed to submit report',
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
      ┌─ report content
    </div>

    <!-- Reason picker -->
    <div class="space-y-1 mb-4">
      {#each reasons as reason}
        <button
          onclick={() => { selectedReason = reason; }}
          class="w-full text-left text-xs px-2 py-1 transition-colors {selectedReason === reason ? 'text-accent-500 bg-terminal-surface' : 'text-terminal-fg hover:text-accent-500 hover:bg-terminal-surface'}"
        >
          &gt; [{selectedReason === reason ? '●' : '○'}] {reason}
        </button>
      {/each}
    </div>

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
        onclick={submitReport}
        disabled={!selectedReason || submitting}
        class="text-xs border border-terminal-border px-2 py-0.5 text-accent-500 hover:border-accent-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {submitting ? '[submitting...]' : '[submit report]'}
      </button>
    </div>

    <!-- Footer -->
    <div class="text-xs text-terminal-dim mt-3">
      └─────────────
    </div>
  </div>
</div>
