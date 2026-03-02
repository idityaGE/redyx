<script lang="ts">
  import { logout } from '../lib/auth';

  interface Props {
    username: string;
    onclose: () => void;
  }

  let { username, onclose }: Props = $props();

  function handleClickOutside(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.user-dropdown')) {
      onclose();
    }
  }

  async function handleLogout() {
    await logout();
    onclose();
    window.location.href = '/';
  }
</script>

<svelte:window onclick={handleClickOutside} />

<div class="user-dropdown absolute right-0 top-full mt-1 z-50 min-w-[160px] border border-terminal-border bg-terminal-surface font-mono text-xs shadow-lg">
  <!-- Terminal-style menu items -->
  <a
    href="/user/{username}"
    class="flex items-center gap-2 px-3 py-1.5 text-terminal-fg hover:text-accent-500 hover:bg-terminal-bg transition-colors"
    onclick={onclose}
  >
    <span class="text-accent-500">&gt;</span> profile
  </a>
  <a
    href="/user/{username}"
    class="flex items-center gap-2 px-3 py-1.5 text-terminal-fg hover:text-accent-500 hover:bg-terminal-bg transition-colors"
    onclick={onclose}
  >
    <span class="text-accent-500">&gt;</span> settings
  </a>
  <div class="border-t border-terminal-border"></div>
  <button
    class="flex items-center gap-2 px-3 py-1.5 w-full text-left text-terminal-fg hover:text-accent-500 hover:bg-terminal-bg transition-colors cursor-pointer"
    onclick={handleLogout}
  >
    <span class="text-accent-500">&gt;</span> logout
  </button>
</div>
