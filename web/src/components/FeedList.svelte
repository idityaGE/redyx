<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
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
  }

  let { endpoint, sort, timeRange }: Props = $props();

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
    }
  }

  onMount(() => {
    loadPage();

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

    // Skip the initial run (onMount handles first load)
    if (initialLoading) return;

    // Reset and reload
    posts = [];
    nextCursor = null;
    hasMore = true;
    loadPage();
  });
</script>

{#if initialLoading}
  <div class="text-xs text-terminal-dim animate-pulse p-4 font-mono">[loading feed...]</div>
{:else if posts.length === 0}
  <div class="text-xs text-terminal-dim p-4 font-mono">no posts yet</div>
{:else}
  {#each posts as post (post.postId)}
    <FeedRow {post} />
  {/each}
{/if}

{#if loading && !initialLoading}
  <div class="text-xs text-terminal-dim animate-pulse p-2 font-mono">[loading more...]</div>
{/if}

<div bind:this={sentinel} class="h-1"></div>
