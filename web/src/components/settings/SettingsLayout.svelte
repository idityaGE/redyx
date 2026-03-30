<script lang="ts">
  import { onMount } from 'svelte';
  import { isAuthenticated, isLoading, initialize, subscribe } from '../../lib/auth';

  interface Props {
    activeSection: 'account' | 'notifications';
  }

  let { activeSection }: Props = $props();

  let authed = $state(isAuthenticated());
  let authLoading = $state(isLoading());

  onMount(() => {
    initialize();

    const unsub = subscribe(() => {
      authed = isAuthenticated();
      authLoading = isLoading();
    });

    return unsub;
  });

  // Redirect to login if not authenticated
  $effect(() => {
    if (!authLoading && !authed) {
      window.location.href = '/login?redirect=/settings';
    }
  });
</script>

{#if authLoading}
  <div class="flex items-center justify-center min-h-[40vh]">
    <span class="text-xs text-terminal-dim font-mono animate-pulse">[loading...]</span>
  </div>
{:else if authed}
  <div class="flex gap-6">
    <!-- Sidebar navigation -->
    <aside class="w-48 shrink-0">
      <div class="border border-terminal-border bg-terminal-surface font-mono text-xs">
        <div class="px-3 py-1.5 border-b border-terminal-border text-terminal-dim">
          ~/settings
        </div>
        <nav class="p-2 space-y-1">
          <a
            href="/settings"
            class="flex items-center gap-2 px-2 py-1 transition-colors {activeSection === 'account' ? 'text-accent-500' : 'text-terminal-fg hover:text-accent-500'}"
          >
            <span class="text-accent-500">&gt;</span>
            <span>account</span>
            {#if activeSection === 'account'}
              <span class="text-terminal-dim ml-auto">*</span>
            {/if}
          </a>
          <a
            href="/settings/notifications"
            class="flex items-center gap-2 px-2 py-1 transition-colors {activeSection === 'notifications' ? 'text-accent-500' : 'text-terminal-fg hover:text-accent-500'}"
          >
            <span class="text-accent-500">&gt;</span>
            <span>notifications</span>
            {#if activeSection === 'notifications'}
              <span class="text-terminal-dim ml-auto">*</span>
            {/if}
          </a>
        </nav>
      </div>
    </aside>

    <!-- Main content area -->
    <main class="flex-1 min-w-0">
      <slot />
    </main>
  </div>
{/if}
