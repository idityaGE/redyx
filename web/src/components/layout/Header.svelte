<script lang="ts">
  import { onMount } from 'svelte';
  import { getUser, isAuthenticated, isLoading, initialize, subscribe } from '../../lib/auth';
  import UserDropdown from './UserDropdown.svelte';

  let user = $state(getUser());
  let authed = $state(isAuthenticated());
  let loading = $state(isLoading());
  let showDropdown = $state(false);

  onMount(() => {
    initialize();

    const unsub = subscribe(() => {
      user = getUser();
      authed = isAuthenticated();
      loading = isLoading();
    });

    return unsub;
  });
</script>

<header class="h-10 flex items-center justify-between px-3 border-b border-terminal-border bg-terminal-surface text-terminal-fg font-mono shrink-0">
  <!-- Left: Brand -->
  <div class="flex items-center gap-2">
    <a href="/" class="text-accent-500 font-semibold text-sm tracking-tight hover:text-accent-400 transition-colors">
      redyx
    </a>
    <span class="text-terminal-dim text-xs hidden sm:inline">v0.1.0</span>
  </div>

  <!-- Center: Search -->
  <div class="flex-1 max-w-md mx-4">
    <div class="flex items-center bg-terminal-bg border border-terminal-border rounded px-2 h-7">
      <span class="text-accent-500 text-xs mr-1">&gt;</span>
      <input
        type="text"
        placeholder="search..."
        class="bg-transparent text-xs text-terminal-fg placeholder:text-terminal-dim outline-none w-full font-mono"
      />
    </div>
  </div>

  <!-- Right: Actions -->
  <div class="flex items-center gap-3 text-xs">
    <!-- Notification diamond -->
    <button class="text-terminal-dim hover:text-accent-500 transition-colors" title="Notifications">
      <span>&#9830;</span>
    </button>

    {#if loading}
      <span class="text-terminal-dim">[...]</span>
    {:else if authed && user}
      <!-- Authenticated: show u/username, click opens dropdown -->
      <div class="relative user-dropdown">
        <button
          class="text-terminal-fg hover:text-accent-500 transition-colors cursor-pointer"
          onclick={(e) => { e.stopPropagation(); showDropdown = !showDropdown; }}
        >
          u/{user.username}
        </button>
        {#if showDropdown}
          <UserDropdown
            username={user.username}
            onclose={() => { showDropdown = false; }}
          />
        {/if}
      </div>
    {:else}
      <!-- Anonymous: click navigates to login -->
      <a href="/login" class="text-terminal-dim hover:text-accent-500 transition-colors">
        [anonymous]
      </a>
    {/if}
  </div>
</header>
