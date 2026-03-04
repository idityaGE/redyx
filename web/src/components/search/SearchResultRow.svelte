<script lang="ts">
  import { relativeTime } from '../../lib/time';

  type SearchResult = {
    postId: string;
    title: string;
    snippet: string;
    authorUsername: string;
    communityName: string;
    voteScore: number;
    commentCount: number;
    createdAt: string;
  };

  interface Props {
    result: SearchResult;
  }

  let { result }: Props = $props();
</script>

<div class="flex items-start gap-3 px-2 py-2 border-b border-terminal-border hover:bg-terminal-surface transition-colors text-xs font-mono group">
  <div class="flex-1 min-w-0">
    <a
      href="/community/{result.communityName}/post/{result.postId}"
      class="text-terminal-fg group-hover:text-accent-500 transition-colors text-sm block"
    >
      <!-- eslint-disable-next-line svelte/no-at-html-tags -->
      {@html result.title}
    </a>

    {#if result.snippet}
      <p class="text-terminal-dim mt-0.5 line-clamp-2 text-xs">
        <!-- eslint-disable-next-line svelte/no-at-html-tags -->
        {@html result.snippet}
      </p>
    {/if}

    <div class="text-terminal-dim mt-0.5">
      <span>{result.authorUsername}</span>
      <span class="mx-1">in</span>
      <a href="/community/{result.communityName}" class="text-accent-600 hover:text-accent-500">
        r/{result.communityName}
      </a>
      <span class="mx-1">&middot;</span>
      <span>{relativeTime(result.createdAt)}</span>
      <span class="mx-1">&middot;</span>
      <span>{result.voteScore} pts</span>
      <span class="mx-1">&middot;</span>
      <span>{result.commentCount} comments</span>
    </div>
  </div>
</div>
