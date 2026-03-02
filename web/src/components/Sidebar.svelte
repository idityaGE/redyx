<script lang="ts">
  import { onMount } from 'svelte';
  import { getUser, isAuthenticated, isLoading, subscribe } from '../lib/auth';

  let authed = $state(isAuthenticated());
  let loading = $state(isLoading());

  // Placeholder for joined communities — will be populated from API or local state
  let joinedCommunities = $state<string[]>([]);

  const shortcuts = [
    { icon: '~', label: 'Home', href: '/' },
    { icon: '\u2605', label: 'Popular', href: '/popular' },
    { icon: '\u25C9', label: 'All', href: '/all' },
    { icon: '\u22A1', label: 'Saved', href: '/saved' },
  ];

  onMount(() => {
    const unsub = subscribe(() => {
      authed = isAuthenticated();
      loading = isLoading();
    });

    return unsub;
  });
</script>

<nav class="py-2 px-2 text-sm font-mono h-full">
  <!-- Shortcuts -->
  <div class="mb-2">
    {#each shortcuts as { icon, label, href }}
      <a
        {href}
        class="flex items-center gap-2 px-2 py-0.5 text-terminal-fg hover:text-accent-500 hover:bg-terminal-bg rounded transition-colors"
      >
        <span class="w-4 text-center text-terminal-dim">{icon}</span>
        <span>{label}</span>
      </a>
    {/each}
  </div>

  <!-- Divider -->
  <div class="px-2 text-terminal-border text-xs select-none mb-2">
    ────────────────
  </div>

  <!-- My Communities: visible when authenticated -->
  {#if authed}
    <div>
      <div class="px-2 text-terminal-dim text-xs mb-1 uppercase tracking-wide">
        My Communities
      </div>

      {#if joinedCommunities.length === 0}
        <div class="px-2 py-1 text-terminal-dim text-xs italic">
          join communities to see them here
        </div>
      {:else}
        {#each joinedCommunities as name, i}
          <a
            href="/community/{name}"
            class="flex items-center px-2 py-0.5 text-terminal-fg hover:text-accent-500 hover:bg-terminal-bg rounded transition-colors"
          >
            <span class="text-terminal-dim text-xs mr-1 w-5 shrink-0">
              {i < joinedCommunities.length - 1 ? '\u251C\u2500\u2500' : '\u2514\u2500\u2500'}
            </span>
            <span class="truncate">r/{name}</span>
          </a>
        {/each}
      {/if}
    </div>
  {:else if !loading}
    <!-- Anonymous: show static community list -->
    <div>
      <div class="px-2 text-terminal-dim text-xs mb-1 uppercase tracking-wide">
        Communities
      </div>
      {#each ['programming', 'typescript', 'svelte', 'linux', 'privacy', 'selfhosted', 'terminal', 'rust'] as name, i}
        <a
          href="/community/{name}"
          class="flex items-center px-2 py-0.5 text-terminal-fg hover:text-accent-500 hover:bg-terminal-bg rounded transition-colors"
        >
          <span class="text-terminal-dim text-xs mr-1 w-5 shrink-0">
            {i < 7 ? '\u251C\u2500\u2500' : '\u2514\u2500\u2500'}
          </span>
          <span class="truncate">r/{name}</span>
        </a>
      {/each}
    </div>
  {/if}
</nav>
