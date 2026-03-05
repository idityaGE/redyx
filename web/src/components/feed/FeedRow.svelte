<script lang="ts">
  import VoteButtons from '../post/VoteButtons.svelte';
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
  }

  let { post, userVote = 0 }: Props = $props();

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
</script>

<div class="flex items-center gap-3 px-2 py-1.5 border-b border-terminal-border hover:bg-terminal-surface transition-colors text-xs font-mono group">
  <!-- Vote column -->
  <VoteButtons postId={post.postId} initialScore={post.voteScore} initialVote={userVote} authorId={post.authorId} />

  <!-- Content -->
  <div class="flex-1 min-w-0">
    <a
      href="/post/{post.postId}"
      class="text-terminal-fg group-hover:text-accent-500 transition-colors truncate block text-sm"
    >
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
</div>
