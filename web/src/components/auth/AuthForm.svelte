<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    title: string;
    submitLabel: string;
    error?: string | null;
    loading?: boolean;
    onsubmit: (e: SubmitEvent) => void;
    children: Snippet;
  }

  let { title, submitLabel, error = null, loading = false, onsubmit, children }: Props = $props();
</script>

<div class="flex items-center justify-center min-h-[60vh] px-4">
  <div class="w-full max-w-sm">
    <!-- Terminal-style ASCII box -->
    <div class="border border-terminal-border bg-terminal-surface font-mono">
      <!-- Title bar -->
      <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim">
        ┌─ {title}
      </div>

      <!-- Form content -->
      <form class="p-4 space-y-3" novalidate onsubmit={onsubmit}>
        {@render children()}

        <!-- Error display -->
        {#if error}
          <div class="text-xs font-mono">
            <span class="text-red-500">&gt; error:</span>
            <span class="text-red-400"> {error}</span>
          </div>
        {/if}

        <!-- Submit button -->
        <button
          type="submit"
          disabled={loading}
          class="w-full text-left px-3 py-1.5 text-xs font-mono border border-terminal-border bg-terminal-bg text-terminal-fg hover:text-accent-500 hover:border-accent-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {#if loading}
            <span class="text-terminal-dim">[processing...]</span>
          {:else}
            <span class="text-accent-500">&gt;</span> {submitLabel}<span class="animate-pulse">_</span>
          {/if}
        </button>
      </form>

      <!-- Bottom border -->
      <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
        └─────────────────
      </div>
    </div>
  </div>
</div>
