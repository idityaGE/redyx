<script lang="ts">
  import { onMount } from 'svelte';

  let isDark = $state(true);

  onMount(() => {
    const saved = localStorage.getItem('theme');
    isDark = saved !== 'light';
    applyTheme();
  });

  function applyTheme() {
    if (isDark) {
      document.documentElement.classList.add('dark');
      document.documentElement.classList.remove('light');
    } else {
      document.documentElement.classList.remove('dark');
      document.documentElement.classList.add('light');
    }
  }

  function toggle() {
    isDark = !isDark;
    localStorage.setItem('theme', isDark ? 'dark' : 'light');
    applyTheme();
  }

  let label = $derived(isDark ? '[dark]' : '[light]');
</script>

<button
  onclick={toggle}
  class="px-2 py-1 text-xs font-mono border border-terminal-border bg-terminal-surface text-terminal-dim hover:text-accent-500 hover:border-accent-500 transition-colors rounded"
  title="Toggle theme"
>
  {label}
</button>
