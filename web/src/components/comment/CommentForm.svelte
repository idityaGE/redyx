<script lang="ts">
  import { api, ApiError } from '../../lib/api';
  import { isAuthenticated } from '../../lib/auth';

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

  interface Props {
    postId: string;
    parentId?: string;
    onSubmit: (comment: Comment) => void;
    onCancel?: () => void;
    isTopLevel?: boolean;
    isBanned?: boolean;
  }

  let {
    postId,
    parentId = '',
    onSubmit,
    onCancel,
    isTopLevel = false,
    isBanned = false,
  }: Props = $props();

  let body = $state('');
  let expanded = $state(!isTopLevel);
  let submitting = $state(false);
  let error = $state('');

  const MAX_CHARS = 10000;

  async function handleSubmit() {
    if (!body.trim() || submitting) return;

    submitting = true;
    error = '';

    try {
      const res = await api<{ comment: Comment }>(`/posts/${postId}/comments`, {
        method: 'POST',
        body: JSON.stringify({
          postId,
          parentId: parentId || undefined,
          body: body.trim(),
        }),
      });

      onSubmit(res.comment);
      body = '';

      if (isTopLevel) {
        expanded = false;
      }
    } catch (e) {
      if (e instanceof ApiError) {
        error = e.message;
      } else {
        error = 'failed to submit comment';
      }
    } finally {
      submitting = false;
    }
  }

  function handleCancel() {
    body = '';
    error = '';
    if (isTopLevel) {
      expanded = false;
    } else if (onCancel) {
      onCancel();
    }
  }
</script>

{#if !isAuthenticated()}
  <div class="text-xs font-mono text-terminal-dim py-2">
    &gt; <a href="/login" class="text-accent-500 hover:text-accent-400">login</a> to comment
  </div>
{:else if isBanned}
  <div class="text-xs font-mono text-terminal-dim py-2">
    <span class="text-red-400">&gt; you are banned from this community</span>
  </div>
{:else if isTopLevel && !expanded}
  <button
    onclick={() => (expanded = true)}
    class="text-xs font-mono text-terminal-dim hover:text-accent-500 transition-colors cursor-pointer py-2"
  >
    [write comment]
  </button>
{:else}
  <div class="mt-2 mb-3">
    <textarea
      bind:value={body}
      maxlength={MAX_CHARS}
      placeholder={parentId ? 'write a reply...' : 'write a comment...'}
      rows={4}
      class="w-full font-mono text-sm bg-terminal-bg border border-terminal-border p-2 text-terminal-fg focus:outline-none focus:border-accent-500 resize-y"
    ></textarea>

    <div class="flex items-center gap-2 mt-1">
      <button
        onclick={handleSubmit}
        disabled={!body.trim() || submitting}
        class="text-xs font-mono px-2 py-1 border border-terminal-border bg-terminal-bg transition-colors cursor-pointer
          {!body.trim() || submitting
            ? 'text-terminal-dim cursor-not-allowed'
            : 'text-accent-500 hover:text-accent-400'}"
      >
        {submitting ? '[submitting...]' : '[submit]'}
      </button>

      <button
        onclick={handleCancel}
        class="text-xs font-mono px-2 py-1 text-terminal-dim hover:text-terminal-fg transition-colors cursor-pointer"
      >
        [cancel]
      </button>

      {#if body.length > 9000}
        <span class="text-xs font-mono {body.length >= MAX_CHARS ? 'text-red-500' : 'text-terminal-dim'}">
          {body.length}/{MAX_CHARS}
        </span>
      {/if}
    </div>

    {#if error}
      <div class="text-xs font-mono mt-1">
        <span class="text-red-500">&gt; error:</span>
        <span class="text-red-400"> {error}</span>
      </div>
    {/if}
  </div>
{/if}
