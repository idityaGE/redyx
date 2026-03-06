<script lang="ts">
  import { onMount } from 'svelte';
  import { untrack } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { isAuthenticated, whenReady } from '../../lib/auth';
  import CommentSortBar from './CommentSortBar.svelte';
  import CommentForm from './CommentForm.svelte';
  import CommentCard from './CommentCard.svelte';

  type Comment = {
    commentId: string;
    postId: string;
    parentId: string;
    authorId: string;
    authorUsername: string;
    body: string;
    voteScore: number;
    replyCount: number;
    path: string;
    depth: number;
    isEdited: boolean;
    isDeleted: boolean;
    createdAt: string;
    editedAt: string;
  };

  type ListCommentsResponse = {
    comments: Comment[];
    pagination: { nextCursor: string; hasMore: boolean; totalCount: number };
  };

  type ListRepliesResponse = {
    replies: Comment[];
    pagination: { nextCursor: string; hasMore: boolean };
  };

  interface Props {
    postId: string;
    communityName?: string;
    isModerator?: boolean;
  }

  let { postId, communityName: propCommunityName = '', isModerator: propIsModerator = false }: Props = $props();

  let communityName = $state(propCommunityName);
  let isModerator = $state(propIsModerator);
  let isBanned = $state(false);

  // Read stored sort preference from localStorage
  const storedSort = typeof window !== 'undefined' ? localStorage.getItem('commentSort') : null;

  let comments = $state<Comment[]>([]);
  let sort = $state(storedSort || 'COMMENT_SORT_ORDER_BEST');
  let loading = $state(true);
  let error = $state('');
  let hasMore = $state(false);
  let nextCursor = $state('');
  let totalCount = $state(0);
  let loadingMore = $state(false);

  // Track which comments have had their deep replies loaded
  let loadedReplyIds = $state<Set<string>>(new Set());

  async function fetchComments(cursor?: string) {
    if (cursor) {
      loadingMore = true;
    } else {
      loading = true;
    }
    error = '';

    try {
      const params = new URLSearchParams();
      params.set('sort', sort);
      params.set('pagination.limit', '20');
      if (cursor) {
        params.set('pagination.cursor', cursor);
      }

      const res = await api<ListCommentsResponse>(`/posts/${postId}/comments?${params.toString()}`);

      if (cursor) {
        comments = [...comments, ...res.comments];
      } else {
        comments = res.comments;
      }

      hasMore = res.pagination?.hasMore ?? false;
      nextCursor = res.pagination?.nextCursor ?? '';
      totalCount = res.pagination?.totalCount ?? 0;
    } catch (e) {
      if (e instanceof ApiError) {
        error = e.message;
      } else {
        error = 'failed to load comments';
      }
    } finally {
      loading = false;
      loadingMore = false;
    }
  }

  function handleSortChange(newSort: string) {
    sort = newSort;
    if (typeof window !== 'undefined') {
      localStorage.setItem('commentSort', newSort);
    }
    // Reset and refetch
    comments = [];
    nextCursor = '';
    hasMore = false;
    loadedReplyIds = new Set();
    fetchComments();
  }

  function handleNewTopLevelComment(comment: Comment) {
    // Prepend new comment at top (optimistic — appears immediately regardless of sort)
    comments = [comment, ...comments];
    totalCount = totalCount + 1;
  }

  function handleReplySubmitted(newComment: Comment, parentPath: string) {
    // Find insertion index: after the parent's entire subtree in the flat list
    const parentIndex = comments.findIndex((c) => c.path === parentPath);
    if (parentIndex === -1) {
      // Fallback: just append
      comments = [...comments, newComment];
      return;
    }

    // Walk forward from parent to find the end of its subtree
    let insertAt = parentIndex + 1;
    while (insertAt < comments.length && comments[insertAt].path.startsWith(parentPath + '.')) {
      insertAt++;
    }

    // Insert the new reply at the correct position
    const updated = [...comments];
    updated.splice(insertAt, 0, newComment);

    // Update parent's reply count
    const parentComment = updated[parentIndex];
    updated[parentIndex] = { ...parentComment, replyCount: parentComment.replyCount + 1 };

    comments = updated;
    totalCount = totalCount + 1;
  }

  function handleCommentDeleted(commentId: string) {
    // The CommentCard already handles optimistic deletion visually.
    // We just update the totalCount.
    // Don't remove from array — children must stay visible.
  }

  async function handleLoadMoreReplies(commentId: string) {
    try {
      const res = await api<ListRepliesResponse>(`/comments/${commentId}/replies?pagination.limit=20`);

      if (!res.replies || res.replies.length === 0) {
        loadedReplyIds = new Set([...loadedReplyIds, commentId]);
        return;
      }

      // Find the parent in the flat list
      const parentIndex = comments.findIndex((c) => c.commentId === commentId);
      if (parentIndex === -1) return;

      const parentPath = comments[parentIndex].path;

      // Find where to insert: after existing subtree
      let insertAt = parentIndex + 1;
      while (insertAt < comments.length && comments[insertAt].path.startsWith(parentPath + '.')) {
        insertAt++;
      }

      // Filter out any duplicates
      const existingIds = new Set(comments.map((c) => c.commentId));
      const newReplies = res.replies.filter((r) => !existingIds.has(r.commentId));

      if (newReplies.length > 0) {
        const updated = [...comments];
        updated.splice(insertAt, 0, ...newReplies);
        comments = updated;
      }

      loadedReplyIds = new Set([...loadedReplyIds, commentId]);
    } catch {
      // Silently fail — user can retry
    }
  }

  /**
   * Determine if a comment should show a [load more replies] trigger.
   * Shows when:
   * - Comment has replies (replyCount > 0)
   * - Those replies aren't already in the flat list
   * - We haven't already loaded them
   */
  function shouldShowLoadMore(comment: Comment, index: number): boolean {
    if (comment.replyCount === 0) return false;
    if (loadedReplyIds.has(comment.commentId)) return false;

    // Check if any children are already in the list
    const nextIndex = index + 1;
    if (nextIndex < comments.length && comments[nextIndex].path.startsWith(comment.path + '.')) {
      return false; // Children already present
    }

    return true;
  }

  onMount(() => {
    whenReady().then(async () => {
      // Fetch community context for moderation features
      if (!communityName) {
        try {
          const postData = await api<{ post: { communityName: string } }>(`/posts/${postId}`);
          communityName = postData.post.communityName;
        } catch {
          // Non-critical — moderation features won't be available
        }
      }
      if (communityName) {
        try {
          const communityData = await api<{ isModerator?: boolean }>(`/communities/${encodeURIComponent(communityName)}`);
          isModerator = communityData.isModerator ?? false;
        } catch {
          // Non-critical
        }

        // Check ban status (fail-open)
        if (isAuthenticated()) {
          try {
            const banCheck = await api<{ isBanned: boolean }>(
              `/communities/${encodeURIComponent(communityName)}/moderation/check-ban`,
              { method: 'POST', body: JSON.stringify({ communityName }) }
            );
            isBanned = banCheck.isBanned ?? false;
          } catch {
            // Fail-open
          }
        }
      }
      fetchComments();
    });
  });
</script>

<div class="mt-6 border-t border-terminal-border pt-4">
  <div class="flex items-center gap-4 mb-4">
    <h3 class="text-terminal-fg font-mono text-sm">{totalCount} comments</h3>
    <CommentSortBar {sort} onSortChange={handleSortChange} />
  </div>

  <CommentForm
    {postId}
    isTopLevel={true}
    {isBanned}
    onSubmit={handleNewTopLevelComment}
  />

  {#if loading}
    <div class="text-terminal-dim font-mono text-xs py-4 animate-pulse">loading comments...</div>
  {:else if error}
    <div class="text-xs font-mono py-4">
      <span class="text-red-500">&gt; error:</span>
      <span class="text-red-400"> {error}</span>
    </div>
  {:else if comments.length === 0}
    <div class="text-terminal-dim font-mono text-xs py-8 text-center">no comments yet. be the first.</div>
  {:else}
    {#each comments as comment, index (comment.commentId)}
      <CommentCard
        {comment}
        {postId}
        depth={comment.depth}
        {communityName}
        {isModerator}
        onReplySubmitted={handleReplySubmitted}
        onDeleted={handleCommentDeleted}
      />

      <!-- [load more replies] for comments with hidden children -->
      {#if shouldShowLoadMore(comment, index)}
        <div style="padding-left: {Math.min(comment.depth, 2) * 1.5}rem">
          <button
            onclick={() => handleLoadMoreReplies(comment.commentId)}
            class="text-xs font-mono text-accent-500 hover:text-accent-400 transition-colors cursor-pointer py-1"
          >
            [load {comment.replyCount} more {comment.replyCount === 1 ? 'reply' : 'replies'}]
          </button>
        </div>
      {/if}
    {/each}

    {#if hasMore}
      <button
        onclick={() => fetchComments(nextCursor)}
        disabled={loadingMore}
        class="text-xs font-mono text-accent-500 hover:text-accent-400 transition-colors cursor-pointer py-2 disabled:text-terminal-dim"
      >
        {loadingMore ? 'loading...' : '[load more comments]'}
      </button>
    {/if}
  {/if}
</div>
