<script lang="ts">
  let activeTab = $state('home');

  const tabs = [
    { id: 'home', icon: '\u2302', label: 'Home', href: '/' },
    { id: 'communities', icon: '\u25C8', label: 'Communities', href: '/communities' },
    { id: 'search', icon: '\u2315', label: 'Search', href: '/search' },
    { id: 'notifications', icon: '\u2666', label: 'Alerts', href: '/notifications' },
    { id: 'profile', icon: '\u25C9', label: 'Profile', href: '/profile' },
  ];
</script>

<nav class="h-14 flex items-stretch bg-terminal-surface border-t border-terminal-border font-mono">
  {#each tabs as tab}
    <a
      href={tab.href}
      class="flex-1 flex flex-col items-center justify-center gap-0.5 transition-colors {activeTab === tab.id ? 'text-accent-500' : 'text-terminal-dim hover:text-terminal-fg'}"
      onclick={(e) => { activeTab = tab.id; }}
    >
      <span class="text-base leading-none">{tab.icon}</span>
      <span class="text-[10px] leading-none">{tab.label}</span>
    </a>
  {/each}
</nav>
