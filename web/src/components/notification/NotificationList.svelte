<script lang="ts">
  import { onMount } from 'svelte';
  import { isAuthenticated, isLoading, initialize, subscribe } from '../../lib/auth';
  import NotificationDropdown from './NotificationDropdown.svelte';

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

  // Redirect to login if not authenticated (after auth check completes)
  $effect(() => {
    if (!authLoading && !authed) {
      window.location.href = '/login?redirect=/notifications';
    }
  });
</script>

{#if authLoading}
  <div class="flex items-center justify-center min-h-[40vh]">
    <span class="text-xs text-terminal-dim font-mono animate-pulse">[loading...]</span>
  </div>
{:else if authed}
  <div class="max-w-2xl">
    <div class="box-terminal mb-4">
      <div class="text-accent-500 text-sm">~ /notifications</div>
      <div class="text-xs text-terminal-dim mt-1">all notifications</div>
    </div>

    <NotificationDropdown fullPage={true} />
  </div>
{:else}
  <div class="flex items-center justify-center min-h-[40vh]">
    <span class="text-xs text-terminal-dim font-mono">&gt; redirecting to login...</span>
  </div>
{/if}
