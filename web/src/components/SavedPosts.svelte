<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { isAuthenticated, initialize, subscribe } from '../lib/auth';
  import FeedRow from './FeedRow.svelte';

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

  type SavedResponse = {
    posts: Post[];
    pagination: { nextCursor: string; hasMore: boolean };
  };

  let posts = $state<Post[]>([]);
  let nextCursor = $state<string | null>(null);
  let hasMore = $state(true);
  let loading = $state(false);
  let initialLoading = $state(true);
  let sentinel: HTMLElement;

  async function loadPage() {
    if (loading || !hasMore) return;
    loading = true;

    try {
      const params = new URLSearchParams({ 'pagination.limit': '25' });
      if (nextCursor) params.set('pagination.cursor', nextCursor);

      const res = await api<SavedResponse>(`/saved?${params}`);

      posts = [...posts, ...(res.posts ?? [])];
      nextCursor = res.pagination?.nextCursor ?? null;
      hasMore = res.pagination?.hasMore ?? false;
    } catch {
      hasMore = false;
    } finally {
      loading = false;
      initialLoading = false;
    }
  }

  async function unsave(postId: string) {
    // Optimistic removal
    const removed = posts.find(p => p.postId === postId);
    const removedIndex = posts.findIndex(p => p.postId === postId);
    posts = posts.filter(p => p.postId !== postId);

    try {
      await api(`/posts/${postId}/save`, {
        method: 'POST',
        body: JSON.stringify({ save: false }),
      });
    } catch {
      // Re-add post on failure
      if (removed && removedIndex >= 0) {
        const restored = [...posts];
        restored.splice(removedIndex, 0, removed);
        posts = restored;
      }
    }
  }

  onMount(() => {
    initialize();

    const unsub = subscribe(() => {
      if (!isAuthenticated()) {
        window.location.href = '/login?redirect=/saved';
      }
    });

    // Auth guard: redirect if not authenticated
    // Wait a tick for auth to initialize before checking
    const checkAuth = setTimeout(() => {
      if (!isAuthenticated()) {
        window.location.href = '/login?redirect=/saved';
        return;
      }
      loadPage();

      const observer = new IntersectionObserver(
        (entries) => {
          if (entries[0]?.isIntersecting && hasMore && !loading) {
            loadPage();
          }
        },
        { rootMargin: '200px' }
      );

      if (sentinel) observer.observe(sentinel);
      return () => observer.disconnect();
    }, 100);

    // If already authenticated, load immediately
    if (isAuthenticated()) {
      clearTimeout(checkAuth);
      loadPage();

      const observer = new IntersectionObserver(
        (entries) => {
          if (entries[0]?.isIntersecting && hasMore && !loading) {
            loadPage();
          }
        },
        { rootMargin: '200px' }
      );

      if (sentinel) {
        observer.observe(sentinel);
      }
    }

    return () => {
      clearTimeout(checkAuth);
      unsub();
    };
  });
</script>

<!-- Header -->
<div class="border border-terminal-border bg-terminal-surface px-3 py-2 mb-2 font-mono">
  <span class="text-terminal-dim text-xs">~</span>
  <span class="text-accent-500 text-sm ml-1">/saved</span>
</div>

{#if initialLoading}
  <div class="text-xs text-terminal-dim animate-pulse p-4 font-mono">[loading saved posts...]</div>
{:else if posts.length === 0}
  <div class="text-xs text-terminal-dim p-4 font-mono">
    <span class="text-terminal-dim">&gt;</span> no saved posts yet
  </div>
{:else}
  {#each posts as post (post.postId)}
    <div class="relative group/saved">
      <FeedRow {post} />
      <button
        onclick={() => unsave(post.postId)}
        class="absolute top-1 right-2 text-terminal-dim hover:text-red-500 text-xs font-mono opacity-0 group-hover/saved:opacity-100 transition-opacity cursor-pointer"
        title="Unsave"
      >
        [unsave]
      </button>
    </div>
  {/each}
{/if}

{#if loading && !initialLoading}
  <div class="text-xs text-terminal-dim animate-pulse p-2 font-mono">[loading more...]</div>
{/if}

<div bind:this={sentinel} class="h-1"></div>
