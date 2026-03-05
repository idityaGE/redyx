<script lang="ts">
  import { relativeTime } from '../../lib/time';
  import { api } from '../../lib/api';

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
    notification: Notification;
    onread?: (id: string) => void;
  }

  let { notification, onread }: Props = $props();

  /** Type prefix label and color class based on notification type */
  function getTypeLabel(type: string): { label: string; colorClass: string } {
    switch (type) {
      case 'NOTIFICATION_TYPE_MENTION':
        return { label: '[@]', colorClass: 'text-yellow-400' };
      case 'NOTIFICATION_TYPE_POST_REPLY':
      case 'NOTIFICATION_TYPE_COMMENT_REPLY':
      default:
        return { label: '[reply]', colorClass: 'text-accent-500' };
    }
  }

  const typeInfo = $derived(getTypeLabel(notification.type));

  function buildLink(n: Notification): string {
    if (n.postId) {
      return `/post/${encodeURIComponent(n.postId)}`;
    }
    return '#';
  }

  async function handleClick() {
    if (!notification.isRead) {
      try {
        await api(`/notifications/${notification.notificationId}/read`, {
          method: 'POST',
        });
        onread?.(notification.notificationId);
      } catch {
        // Best-effort mark read
      }
    }
  }
</script>

<a
  href={buildLink(notification)}
  onclick={handleClick}
  class="flex items-start gap-2 px-3 py-2 text-xs font-mono transition-colors hover:bg-terminal-bg/60 {notification.isRead ? '' : 'bg-terminal-surface/50 border-l-2 border-l-accent-500'}"
>
  <span class="{typeInfo.colorClass} shrink-0 mt-0.5">{typeInfo.label}</span>
  <div class="flex-1 min-w-0">
    <span class="text-terminal-fg">
      {notification.message}
    </span>
    {#if notification.communityName}
      <span class="text-terminal-dim"> in r/{notification.communityName}</span>
    {/if}
  </div>
  {#if relativeTime(notification.createdAt)}
    <span class="text-terminal-dim shrink-0 ml-1">{relativeTime(notification.createdAt)}</span>
  {/if}
</a>
