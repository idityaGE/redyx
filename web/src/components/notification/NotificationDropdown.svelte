<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../../lib/api';
  import NotificationItem from './NotificationItem.svelte';

  interface Notification {
    notificationId: string;
    type: string;
    actorId: string;
    actorUsername: string;
    targetId: string;
    targetType: string;
    postId: string;
    communityName: string;
    message: string;
    isRead: boolean;
    createdAt: string;
  }

  interface Props {
    /** Notifications pushed in real-time from parent (WebSocket) */
    incoming?: Notification[];
    /** Full page mode with pagination */
    fullPage?: boolean;
  }

  let { incoming = [], fullPage = false }: Props = $props();

  let notifications = $state<Notification[]>([]);
  let loading = $state(true);
  let markingAll = $state(false);
  let nextCursor = $state('');
  let hasMore = $state(false);
  let loadingMore = $state(false);

  const limit = fullPage ? 50 : 20;

  async function fetchNotifications(cursor?: string) {
    try {
      let url = `/notifications?pagination.limit=${limit}`;
      if (cursor) {
        url += `&pagination.cursor=${encodeURIComponent(cursor)}`;
      }

      const data = await api<{
        notifications: Notification[];
        pagination: { nextCursor: string };
        unreadCount: number;
      }>(url);

      if (cursor) {
        notifications = [...notifications, ...(data.notifications ?? [])];
      } else {
        notifications = data.notifications ?? [];
      }
      nextCursor = data.pagination?.nextCursor ?? '';
      hasMore = nextCursor !== '';
    } catch {
      // Silently fail — empty list is acceptable
    } finally {
      loading = false;
      loadingMore = false;
    }
  }

  async function markAllRead() {
    markingAll = true;
    try {
      await api<{ markedCount: number }>('/notifications/read-all', {
        method: 'POST',
      });
      notifications = notifications.map((n) => ({ ...n, isRead: true }));
    } catch {
      // Best-effort
    } finally {
      markingAll = false;
    }
  }

  function handleRead(id: string) {
    notifications = notifications.map((n) =>
      n.notificationId === id ? { ...n, isRead: true } : n
    );
  }

  async function loadMore() {
    if (!hasMore || loadingMore) return;
    loadingMore = true;
    await fetchNotifications(nextCursor);
  }

  // Merge incoming real-time notifications
  $effect(() => {
    if (incoming.length > 0) {
      // Prepend new ones, deduplicate by notificationId
      const existingIds = new Set(notifications.map((n) => n.notificationId));
      const newOnes = incoming.filter((n) => !existingIds.has(n.notificationId));
      if (newOnes.length > 0) {
        notifications = [...newOnes, ...notifications];
      }
    }
  });

  onMount(() => {
    fetchNotifications();
  });
</script>

<div class="{fullPage ? '' : 'bg-terminal-bg border border-terminal-border rounded shadow-lg max-h-96 overflow-y-auto'} font-mono">
  <!-- Header -->
  <div class="flex items-center justify-between px-3 py-2 border-b border-terminal-border">
    <span class="text-xs text-terminal-dim">
      {fullPage ? '~ /notifications' : 'notifications'}
    </span>
    <button
      onclick={markAllRead}
      disabled={markingAll}
      class="text-xs text-terminal-dim hover:text-accent-500 transition-colors disabled:opacity-50 cursor-pointer"
    >
      {markingAll ? '[marking...]' : '[mark all read]'}
    </button>
  </div>

  <!-- List -->
  {#if loading}
    <div class="px-3 py-4 text-center">
      <span class="text-xs text-terminal-dim animate-pulse">[loading...]</span>
    </div>
  {:else if notifications.length === 0}
    <div class="px-3 py-4 text-center">
      <span class="text-xs text-terminal-dim">no notifications</span>
    </div>
  {:else}
    <div class="divide-y divide-terminal-border/50">
      {#each notifications as notification (notification.notificationId)}
        <NotificationItem {notification} onread={handleRead} />
      {/each}
    </div>
  {/if}

  <!-- Footer -->
  {#if !fullPage && !loading && notifications.length > 0}
    <div class="border-t border-terminal-border px-3 py-1.5 text-center">
      <a href="/notifications" class="text-xs text-accent-500 hover:text-accent-400 transition-colors">
        view all &rarr;
      </a>
    </div>
  {/if}

  {#if fullPage && hasMore}
    <div class="border-t border-terminal-border px-3 py-2 text-center">
      <button
        onclick={loadMore}
        disabled={loadingMore}
        class="text-xs text-accent-500 hover:text-accent-400 transition-colors disabled:opacity-50 cursor-pointer"
      >
        {loadingMore ? '[loading...]' : '[load more]'}
      </button>
    </div>
  {/if}
</div>
