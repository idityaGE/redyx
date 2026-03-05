<script lang="ts">
  import { onMount } from 'svelte';
  import { getUser, isAuthenticated, isLoading, whenReady, subscribe } from '../../lib/auth';
  import { api } from '../../lib/api';

  let authed = $state(isAuthenticated());
  let loading = $state(isLoading());
  let currentUser = $state(getUser());

  type Community = {
    communityId: string;
    name: string;
    ownerId: string;
  };

  let myCommunities = $state<string[]>([]);

  const shortcuts = [
    { icon: '~', label: 'Home', href: '/' },
    { icon: '\u2605', label: 'Popular', href: '/popular' },
    { icon: '\u25C9', label: 'All', href: '/all' },
    { icon: '\u25C7', label: 'Communities', href: '/communities' },
    { icon: '\u22A1', label: 'Saved', href: '/saved' },
  ];

  async function fetchMyCommunities() {
    const user = getUser();
    if (!user) {
      myCommunities = [];
      return;
    }
    try {
      const data = await api<{ communities: { communityId: string; name: string }[] }>(
        `/users/${user.userId}/communities?pagination.limit=100`
      );
      myCommunities = (data.communities ?? []).map(c => c.name);
    } catch {
      myCommunities = [];
    }
  }

  onMount(() => {
    // Wait for auth to resolve, then fetch if logged in
    whenReady().then(() => {
      authed = isAuthenticated();
      loading = isLoading();
      currentUser = getUser();
      if (authed) fetchMyCommunities();
    });

    const unsub = subscribe(() => {
      const wasAuthed = authed;
      authed = isAuthenticated();
      loading = isLoading();
      currentUser = getUser();

      // Fetch communities when auth state changes to logged-in
      if (authed && !wasAuthed) {
        fetchMyCommunities();
      }
      if (!authed) {
        myCommunities = [];
      }
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

      {#if myCommunities.length === 0}
        <div class="px-2 py-1 text-terminal-dim text-xs italic">
          join communities to see them here
        </div>
      {:else}
        {#each myCommunities as name, i}
          <a
            href="/community/{name}"
            class="flex items-center px-2 py-0.5 text-terminal-fg hover:text-accent-500 hover:bg-terminal-bg rounded transition-colors"
          >
            <span class="text-terminal-dim text-xs mr-1 w-5 shrink-0">
              {i < myCommunities.length - 1 ? '\u251C\u2500\u2500' : '\u2514\u2500\u2500'}
            </span>
            <span class="truncate">r/{name}</span>
          </a>
        {/each}
      {/if}
    </div>

  {/if}
</nav>
