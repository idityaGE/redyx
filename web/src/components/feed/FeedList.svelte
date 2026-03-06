<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { api } from '../../lib/api';
  import { whenReady } from '../../lib/auth';
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

  type FeedResponse = {
    posts: Post[];
    pagination: { nextCursor: string; hasMore: boolean };
  };

  interface Props {
    endpoint: string;
    sort: string;
    timeRange?: string;
    isModerator?: boolean;
  }

  let { endpoint, sort, timeRange, isModerator = false }: Props = $props();

  let posts = $state<Post[]>([]);

  // Sort pinned posts to the top of the feed (community feeds only)
  let sortedPosts = $derived((() => {
    if (!posts.length) return posts;
    const pinned = posts.filter(p => p.isPinned);
    const unpinned = posts.filter(p => !p.isPinned);
    // Only sort if there are pinned posts (avoids unnecessary array allocation)
    return pinned.length > 0 ? [...pinned, ...unpinned] : posts;
  })());

  let nextCursor = $state<string | null>(null);
  let hasMore = $state(true);
  let loading = $state(false);
  let initialLoading = $state(true);
  // Track whether the first load has completed (not reactive — avoids $effect re-fire)
  let hasLoaded = false;
  let sentinel: HTMLElement;

  async function loadPage() {
    if (loading || !hasMore) return;
    loading = true;

    try {
      const params = new URLSearchParams({ 'pagination.limit': '25' });
      if (nextCursor) params.set('pagination.cursor', nextCursor);
      if (sort) params.set('sort', sort);
      if (timeRange) params.set('timeRange', timeRange);

      const res = await api<FeedResponse>(`${endpoint}?${params}`);

      posts = [...posts, ...(res.posts ?? [])];
      nextCursor = res.pagination?.nextCursor ?? null;
      hasMore = res.pagination?.hasMore ?? false;
    } catch {
      hasMore = false;
    } finally {
      loading = false;
      initialLoading = false;
      hasLoaded = true;
    }
  }

  onMount(() => {
    // Wait for auth initialization before making API calls
    // This prevents 401 races when accessToken hasn't been set yet
    whenReady().then(() => loadPage());

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && hasMore && !loading) {
          loadPage();
        }
      },
      { rootMargin: '200px' }
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  });

  // Reset feed when sort or timeRange changes
  $effect(() => {
    // Access sort and timeRange to establish dependency tracking
    const _sort = sort;
    const _timeRange = timeRange;

    // Skip until first load completes (onMount handles initial load)
    // Using plain boolean (not $state) so this check doesn't create a dependency
    if (!hasLoaded) return;

    // Untrack the reset+reload so writes to $state (posts, nextCursor, hasMore)
    // and reads inside loadPage() (loading, hasMore) don't become dependencies
    // of this effect — otherwise loadPage toggling `loading` re-triggers the
    // effect in an infinite loop.
    untrack(() => {
      posts = [];
      nextCursor = null;
      hasMore = true;
      loadPage();
    });
  });
</script>

{#if initialLoading}
  <div class="text-xs text-terminal-dim animate-pulse p-4 font-mono">[loading feed...]</div>
{:else if posts.length === 0}
  <div class="text-xs text-terminal-dim p-4 font-mono">no posts yet</div>
{:else}
  {#each sortedPosts as post (post.postId)}
    <FeedRow {post} {isModerator} />
  {/each}
{/if}

{#if loading && !initialLoading}
  <div class="text-xs text-terminal-dim animate-pulse p-2 font-mono">[loading more...]</div>
{/if}

<div bind:this={sentinel} class="h-1"></div>
