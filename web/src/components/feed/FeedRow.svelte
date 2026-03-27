<script lang="ts">
  import { onMount } from 'svelte';
  import VoteButtons from '../post/VoteButtons.svelte';
  import ReportDialog from '../moderation/ReportDialog.svelte';
  import { api } from '../../lib/api';
  import { isAuthenticated } from '../../lib/auth';
  import { relativeTime } from '../../lib/time';

  type Post = {
    postId: string;
    title: string;
    body: string;
    url: string;
    postType: 'POST_TYPE_TEXT' | 'POST_TYPE_LINK' | 'POST_TYPE_MEDIA';
    authorId: string;
    authorUsername: string;
    communityId: string;
    communityName: string;
    voteScore: number;
    commentCount: number;
    isEdited: boolean;
    isDeleted: boolean;
    isPinned: boolean;
    isAnonymous: boolean;
    mediaUrls: string[];
    thumbnailUrl: string;
    createdAt: string;
    editedAt: string;
  };

  interface Props {
    post: Post;
    userVote?: number;
    isModerator?: boolean;
    showPinned?: boolean;
  }

  let { post, userVote = 0, isModerator = false, showPinned = true }: Props = $props();

  // Overflow menu state
  let showMenu = $state(false);
  let showReportDialog = $state(false);
  let confirmingRemove = $state(false);
  let pinning = $state(false);

  /**
   * Extract hostname from a URL string safely.
   * Returns empty string on parse failure.
   */
  function extractDomain(url: string): string {
    try {
      return new URL(url).hostname;
    } catch {
      return '';
    }
  }

  let domain = $derived(
    post.postType === 'POST_TYPE_LINK' && post.url ? extractDomain(post.url) : ''
  );

  function toggleMenu(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    showMenu = !showMenu;
    confirmingRemove = false;
  }

  function closeMenu() {
    showMenu = false;
    confirmingRemove = false;
  }

  function openReport(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    showMenu = false;
    showReportDialog = true;
  }

  async function handleRemove(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (!confirmingRemove) {
      confirmingRemove = true;
      return;
    }
    try {
      await api(`/communities/${encodeURIComponent(post.communityName)}/moderation/remove`, {
        method: 'POST',
        body: JSON.stringify({ contentId: post.postId, contentType: 1 }),
      });
      closeMenu();
      // Visually mark as removed
      post = { ...post, isDeleted: true };
    } catch {
      // Silently fail
    }
  }

  async function handlePin(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (pinning) return;
    pinning = true;
    const action = post.isPinned ? 'unpin' : 'pin';
    try {
      await api(`/communities/${encodeURIComponent(post.communityName)}/moderation/${action}`, {
        method: 'POST',
        body: JSON.stringify({ postId: post.postId }),
      });
      post = { ...post, isPinned: !post.isPinned };
    } catch {
      // Silently fail
    } finally {
      pinning = false;
      closeMenu();
    }
  }

  // Close menu on outside click
  function handleWindowClick() {
    if (showMenu) closeMenu();
  }
</script>

<svelte:window onclick={handleWindowClick} />

<div class="flex items-center gap-3 px-2 py-1.5 border-b border-terminal-border hover:bg-terminal-surface transition-colors text-xs font-mono group {showPinned && post.isPinned ? 'border-l-2 border-l-accent-500 bg-terminal-surface/50' : ''}">
  <!-- Vote column -->
  <VoteButtons postId={post.postId} initialScore={post.voteScore} initialVote={userVote} authorId={post.authorId} />

  <!-- Content -->
  <div class="flex-1 min-w-0">
    <a
      href="/post/{post.postId}"
      class="text-terminal-fg group-hover:text-accent-500 transition-colors truncate block text-sm"
    >
      {#if showPinned && post.isPinned}
        <span class="text-accent-500 mr-1">[pinned]</span>
      {/if}
      {post.title}
      {#if domain}
        <span class="text-terminal-dim text-xs ml-1">({domain})</span>
      {/if}
    </a>
    <div class="text-terminal-dim mt-0.5">
      <a href="/community/{post.communityName}" class="text-accent-600 hover:text-accent-500">
        r/{post.communityName}
      </a>
      <span class="mx-1">&middot;</span>
      <span>{post.isAnonymous ? '[anonymous]' : `u/${post.authorUsername}`}</span>
      <span class="mx-1">&middot;</span>
      <span>{relativeTime(post.createdAt)}</span>
      <span class="mx-1">&middot;</span>
      <span>{post.commentCount} comments</span>
    </div>
  </div>

  <!-- Thumbnail for media posts -->
  {#if post.postType === 'POST_TYPE_MEDIA' && post.thumbnailUrl}
    <div class="shrink-0">
      <img
        src={post.thumbnailUrl}
        alt=""
        class="w-8 h-8 object-cover border border-terminal-border rounded-sm"
        loading="lazy"
      />
    </div>
  {/if}

  <!-- Three-dot overflow menu -->
  {#if isAuthenticated()}
    <div class="relative shrink-0">
      <button
        onclick={toggleMenu}
        class="text-terminal-dim hover:text-terminal-fg transition-colors cursor-pointer px-1 opacity-0 group-hover:opacity-100"
        title="More actions"
      >
        [...]
      </button>

      {#if showMenu}
        <div
          class="absolute right-0 top-full mt-1 bg-terminal-bg border border-terminal-border text-xs font-mono z-20 min-w-35"
          onclick={(e) => e.stopPropagation()}
          onkeydown={(e) => {
            if (e.key === 'Escape') closeMenu();
            e.stopPropagation();
          }}
          role="menu"
          tabindex="0"
        >
          <!-- Report (all authenticated users) -->
          <button
            onclick={openReport}
            class="w-full text-left px-3 py-1.5 text-terminal-fg hover:text-accent-500 hover:bg-terminal-surface transition-colors cursor-pointer"
          >
            [report]
          </button>

          {#if isModerator}
            <div class="border-t border-terminal-border"></div>
            {#if showPinned}
              <!-- Pin/Unpin -->
              <button
                onclick={handlePin}
                disabled={pinning}
                class="w-full text-left px-3 py-1.5 text-terminal-fg hover:text-accent-500 hover:bg-terminal-surface transition-colors cursor-pointer disabled:text-terminal-dim"
              >
                {post.isPinned ? '[unpin]' : '[pin]'}
              </button>
            {/if}
            <!-- Remove -->
            <button
              onclick={handleRemove}
              class="w-full text-left px-3 py-1.5 transition-colors cursor-pointer {confirmingRemove ? 'text-red-500 bg-red-500/10' : 'text-terminal-fg hover:text-red-500 hover:bg-terminal-surface'}"
            >
              {confirmingRemove ? '[confirm remove?]' : '[remove]'}
            </button>
          {/if}
        </div>
      {/if}
    </div>
  {/if}
</div>

{#if showReportDialog}
  <ReportDialog
    communityName={post.communityName}
    contentId={post.postId}
    contentType="post"
    onClose={() => (showReportDialog = false)}
  />
{/if}
