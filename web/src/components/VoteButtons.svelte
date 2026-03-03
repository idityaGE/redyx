<script lang="ts">
  import { api, ApiError } from '../lib/api';
  import { isAuthenticated } from '../lib/auth';

  interface Props {
    postId: string;
    targetType?: string;
    initialScore: number;
    initialVote?: number;
  }

  let {
    postId,
    targetType = 'TARGET_TYPE_POST',
    initialScore,
    initialVote = 0,
  }: Props = $props();

  let score = $state(initialScore);
  let userVote = $state(initialVote); // -1=down, 0=none, 1=up

  /**
   * Format score for compact display.
   * Exact up to 999, then 1.4k, 15.8k, 1.2m.
   * Removes trailing .0 (e.g., 1.0k → 1k).
   */
  function formatScore(n: number): string {
    if (n >= 1_000_000) {
      const val = (n / 1_000_000).toFixed(1);
      return val.endsWith('.0') ? val.slice(0, -2) + 'm' : val + 'm';
    }
    if (n >= 1_000) {
      const val = (n / 1_000).toFixed(1);
      return val.endsWith('.0') ? val.slice(0, -2) + 'k' : val + 'k';
    }
    return n.toString();
  }

  async function vote(direction: 'up' | 'down') {
    if (!isAuthenticated()) {
      window.location.href = '/login';
      return;
    }

    const prevScore = score;
    const prevVote = userVote;

    // Compute new vote state — toggle off if clicking same direction
    const newDirection =
      (direction === 'up' && userVote === 1) || (direction === 'down' && userVote === -1)
        ? 0
        : direction === 'up'
          ? 1
          : -1;

    // Apply optimistic update immediately
    score = prevScore + (newDirection - prevVote);
    userVote = newDirection;

    try {
      const directionEnum =
        newDirection === 1
          ? 'VOTE_DIRECTION_UP'
          : newDirection === -1
            ? 'VOTE_DIRECTION_DOWN'
            : 'VOTE_DIRECTION_NONE';

      const res = await api<{ newScore: number }>('/votes', {
        method: 'POST',
        body: JSON.stringify({
          targetId: postId,
          targetType,
          direction: directionEnum,
        }),
      });

      // Reconcile with server score (handles concurrent votes)
      score = res.newScore;
    } catch (e) {
      // Rollback on failure
      score = prevScore;
      userVote = prevVote;

      if (e instanceof ApiError && e.status === 401) {
        window.location.href = '/login';
      }
    }
  }
</script>

<div class="flex flex-col items-center w-10 shrink-0">
  <button
    onclick={() => vote('up')}
    class="transition-colors leading-none cursor-pointer {userVote === 1
      ? 'text-accent-500'
      : 'text-terminal-dim hover:text-accent-500'}"
    aria-label="Upvote"
  >&#9650;</button>
  <span
    class="text-xs font-medium {userVote === 1
      ? 'text-accent-500'
      : userVote === -1
        ? 'text-red-500'
        : 'text-terminal-fg'}"
  >
    {formatScore(score)}
  </span>
  <button
    onclick={() => vote('down')}
    class="transition-colors leading-none cursor-pointer {userVote === -1
      ? 'text-red-500'
      : 'text-terminal-dim hover:text-red-500'}"
    aria-label="Downvote"
  >&#9660;</button>
</div>
