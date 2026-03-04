<script lang="ts">
  import { onMount } from 'svelte';
  import { getUser, isAuthenticated, isLoading, initialize, subscribe, whenReady } from '../../lib/auth';
  import { getAccessToken } from '../../lib/api';
  import { api } from '../../lib/api';
  import { createNotificationSocket, type NotificationSocketHandle } from '../../lib/websocket';
  import NotificationDropdown from './NotificationDropdown.svelte';

  let unreadCount = $state(0);
  let dropdownOpen = $state(false);
  let incoming = $state<any[]>([]);
  let authed = $state(isAuthenticated());
  let authLoading = $state(isLoading());

  let socketHandle: NotificationSocketHandle | null = null;

  function handleNewNotification(data: any) {
    unreadCount += 1;
    incoming = [data, ...incoming];
  }

  async function startSocket() {
    const token = getAccessToken();
    if (!token) return;

    // Fetch initial unread count
    try {
      const data = await api<{
        notifications: any[];
        pagination: { nextCursor: string };
        unreadCount: number;
      }>('/notifications?unreadOnly=true&pagination.limit=1');
      unreadCount = data.unreadCount ?? 0;
    } catch {
      // Fallback: 0 unread
    }

    // Connect WebSocket
    socketHandle = createNotificationSocket(token, handleNewNotification);
  }

  function toggleDropdown(e: MouseEvent) {
    e.stopPropagation();
    dropdownOpen = !dropdownOpen;
  }

  function handleClickOutside(event: MouseEvent) {
    const target = event.target as HTMLElement;
    if (!target.closest('.notification-bell-container')) {
      dropdownOpen = false;
    }
  }

  onMount(() => {
    initialize();

    const unsub = subscribe(() => {
      const wasAuthed = authed;
      authed = isAuthenticated();
      authLoading = isLoading();

      // If user just logged in, start socket
      if (authed && !wasAuthed) {
        startSocket();
      }

      // If user logged out, close socket
      if (!authed && wasAuthed) {
        socketHandle?.close();
        socketHandle = null;
        unreadCount = 0;
        incoming = [];
        dropdownOpen = false;
      }
    });

    // Start socket after auth is ready (if already authenticated)
    whenReady().then(() => {
      if (isAuthenticated()) {
        startSocket();
      }
    });

    return () => {
      unsub();
      socketHandle?.close();
    };
  });

  let badgeText = $derived(unreadCount > 9 ? '9+' : String(unreadCount));
</script>

<svelte:window onclick={handleClickOutside} />

<div class="relative notification-bell-container">
  <button
    onclick={toggleDropdown}
    class="text-terminal-dim hover:text-accent-500 transition-colors relative cursor-pointer"
    title="Notifications"
  >
    <span>&#9830;</span>
    {#if authed && unreadCount > 0}
      <span class="absolute -top-1.5 -right-2 min-w-[14px] h-[14px] flex items-center justify-center bg-red-500 text-white text-[9px] font-bold rounded-full leading-none px-0.5">
        {badgeText}
      </span>
    {/if}
  </button>

  {#if dropdownOpen && authed}
    <div class="absolute right-0 top-full mt-1 z-50 w-80">
      <NotificationDropdown {incoming} />
    </div>
  {/if}
</div>
