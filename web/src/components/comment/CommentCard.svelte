<script lang="ts">
  import { api, ApiError } from '../../lib/api';
  import { isAuthenticated, getUser } from '../../lib/auth';
  import VoteButtons from '../post/VoteButtons.svelte';
  import PostBody from '../post/PostBody.svelte';
  import CommentForm from './CommentForm.svelte';
  import ReportDialog from '../moderation/ReportDialog.svelte';
  import { relativeTime } from '../../lib/time';

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
    comment: Comment;
    postId: string;
    depth: number;
    communityName?: string;
    isModerator?: boolean;
    onReplySubmitted?: (comment: Comment, parentPath: string) => void;
    onDeleted?: (commentId: string) => void;
  }

  let {
    comment,
    postId,
    depth,
    communityName = '',
    isModerator = false,
    onReplySubmitted,
    onDeleted,
  }: Props = $props();

  // Component-local state (NOT global store)
  let collapsed = $state(comment.voteScore < -5 && !comment.isDeleted);
  let replying = $state(false);
  let editing = $state(false);
  let editBody = $state(comment.body);
  let editError = $state('');
  let editSaving = $state(false);
  let confirmingDelete = $state(false);
  let localComment = $state<Comment>({ ...comment });

  // Overflow menu state
  let showMenu = $state(false);
  let showReportDialog = $state(false);
  let confirmingModRemove = $state(false);

  let isOwnComment = $derived((() => {
    const user = getUser();
    return user?.userId === localComment.authorId;
  })());

  // Visual nesting: max 3 levels (0, 1, 2 indentation steps)
  let indentPx = $derived(Math.min(depth - 1, 2) * 1.5);

  function toggleCollapse() {
    collapsed = !collapsed;
  }

  async function submitEdit() {
    if (!editBody.trim() || editSaving) return;
    editSaving = true;
    editError = '';

    try {
      const res = await api<{ comment: Comment }>(`/comments/${localComment.commentId}`, {
        method: 'PATCH',
        body: JSON.stringify({ body: editBody.trim() }),
      });

      localComment = { ...localComment, body: res.comment.body, isEdited: true, editedAt: res.comment.editedAt };
      editing = false;
    } catch (e) {
      if (e instanceof ApiError) {
        editError = e.message;
      } else {
        editError = 'failed to save edit';
      }
    } finally {
      editSaving = false;
    }
  }

  async function handleDelete() {
    const prev = { ...localComment };

    // Optimistic: mark as deleted locally
    localComment = { ...localComment, body: '[deleted]', authorUsername: '[deleted]', isDeleted: true };
    confirmingDelete = false;

    try {
      await api(`/comments/${comment.commentId}`, { method: 'DELETE' });
      if (onDeleted) onDeleted(comment.commentId);
    } catch {
      // Revert on failure
      localComment = prev;
    }
  }

  function toggleCommentMenu() {
    showMenu = !showMenu;
    confirmingModRemove = false;
  }

  function closeCommentMenu() {
    showMenu = false;
    confirmingModRemove = false;
  }

  function openCommentReport() {
    showMenu = false;
    showReportDialog = true;
  }

  async function handleModRemove() {
    if (!confirmingModRemove) {
      confirmingModRemove = true;
      return;
    }
    try {
      await api(`/communities/${encodeURIComponent(communityName)}/moderation/remove`, {
        method: 'POST',
        body: JSON.stringify({ contentId: localComment.commentId, contentType: 2 }),
      });
      localComment = { ...localComment, body: '[removed]', isDeleted: true };
      closeCommentMenu();
    } catch {
      // Silently fail
    }
  }

  function handleReplySubmitted(newComment: Comment) {
    if (onReplySubmitted) {
      onReplySubmitted(newComment, localComment.path);
    }
    localComment = { ...localComment, replyCount: localComment.replyCount + 1 };
    replying = false;
  }
</script>

<svelte:window onclick={() => { if (showMenu) closeCommentMenu(); }} />

<div class="relative py-1.5" style="padding-left: {indentPx}rem">
  {#if depth > 1}
    <div
      class="absolute top-0 bottom-0 border-l border-terminal-border"
      style="left: {(Math.min(depth - 2, 1) * 1.5) + 0.25}rem"
    ></div>
  {/if}

  <!-- Comment header -->
  <div class="flex items-center gap-2 text-xs font-mono text-terminal-dim">
    <button
      onclick={toggleCollapse}
      class="hover:text-terminal-fg transition-colors cursor-pointer"
    >
      {collapsed ? '[+]' : '[-]'}
    </button>

    {#if localComment.isDeleted}
      <span>[deleted]</span>
    {:else}
      <span class="text-terminal-fg">u/{localComment.authorUsername}</span>
    {/if}

    <span>&middot;</span>
    <span>{relativeTime(localComment.createdAt)}</span>

    {#if localComment.isEdited && !localComment.isDeleted}
      <span class="italic">(edited)</span>
    {/if}

    {#if collapsed}
      <span>&middot; {localComment.replyCount} {localComment.replyCount === 1 ? 'reply' : 'replies'}</span>
    {/if}
  </div>

  {#if !collapsed}
    <!-- Vote + body -->
    <div class="flex gap-2 mt-1">
      {#if !localComment.isDeleted}
        <VoteButtons
          postId={localComment.commentId}
          targetType="TARGET_TYPE_COMMENT"
          initialScore={localComment.voteScore}
          initialVote={0}
          authorId={localComment.authorId}
        />
      {:else}
        <div class="flex flex-col items-center w-10 shrink-0">
          <span class="text-xs text-terminal-dim">{localComment.voteScore}</span>
        </div>
      {/if}

      <div class="flex-1 min-w-0">
        {#if editing}
          <!-- Inline edit textarea -->
          <textarea
            bind:value={editBody}
            rows={4}
            class="w-full font-mono text-sm bg-terminal-bg border border-terminal-border p-2 text-terminal-fg focus:outline-none focus:border-accent-500 resize-y"
          ></textarea>
          <div class="flex gap-2 mt-1">
            <button
              onclick={submitEdit}
              disabled={!editBody.trim() || editSaving}
              class="text-xs font-mono text-accent-500 hover:text-accent-400 transition-colors cursor-pointer disabled:text-terminal-dim disabled:cursor-not-allowed"
            >
              {editSaving ? '[saving...]' : '[save]'}
            </button>
            <button
              onclick={() => { editing = false; editError = ''; editBody = localComment.body; }}
              class="text-xs font-mono text-terminal-dim hover:text-terminal-fg transition-colors cursor-pointer"
            >
              [cancel]
            </button>
          </div>
          {#if editError}
            <div class="text-xs font-mono mt-1">
              <span class="text-red-500">&gt; error:</span>
              <span class="text-red-400"> {editError}</span>
            </div>
          {/if}
        {:else if localComment.isDeleted}
          <p class="text-sm font-mono text-terminal-dim italic">[deleted]</p>
        {:else}
          <PostBody body={localComment.body} />
        {/if}

        <!-- Action bar (only for non-deleted, non-editing) -->
        {#if !localComment.isDeleted && !editing}
          <div class="flex items-center gap-3 text-xs font-mono text-terminal-dim mt-1">
            {#if isAuthenticated()}
              <button
                onclick={() => (replying = !replying)}
                class="hover:text-accent-500 transition-colors cursor-pointer"
              >
                [reply]
              </button>

              {#if isOwnComment}
                <button
                  onclick={() => { editing = true; editBody = localComment.body; }}
                  class="hover:text-accent-500 transition-colors cursor-pointer"
                >
                  [edit]
                </button>

                {#if !confirmingDelete}
                  <button
                    onclick={() => (confirmingDelete = true)}
                    class="hover:text-red-500 transition-colors cursor-pointer"
                  >
                    [delete]
                  </button>
                {:else}
                  <span class="text-red-400">
                    delete?
                    <button onclick={handleDelete} class="text-red-500 hover:text-red-400 underline cursor-pointer">yes</button>
                    <span class="mx-0.5">/</span>
                    <button onclick={() => (confirmingDelete = false)} class="text-terminal-dim hover:text-terminal-fg underline cursor-pointer">no</button>
                  </span>
                {/if}
              {/if}

              <!-- Three-dot overflow menu -->
              <div class="relative">
                <button
                  onclick={(e) => { e.stopPropagation(); toggleCommentMenu(); }}
                  class="hover:text-terminal-fg transition-colors cursor-pointer"
                  title="More actions"
                >
                  [...]
                </button>

                {#if showMenu}
                  <!-- svelte-ignore a11y_no_static_element_interactions -->
                  <div
                    class="absolute left-0 bottom-full mb-1 bg-terminal-bg border border-terminal-border text-xs font-mono z-20 min-w-[140px]"
                    onclick={(e) => e.stopPropagation()}
                  >
                    <button
                      onclick={openCommentReport}
                      class="w-full text-left px-3 py-1.5 text-terminal-fg hover:text-accent-500 hover:bg-terminal-surface transition-colors cursor-pointer"
                    >
                      [report]
                    </button>

                    {#if isModerator}
                      <div class="border-t border-terminal-border"></div>
                      <button
                        onclick={handleModRemove}
                        class="w-full text-left px-3 py-1.5 transition-colors cursor-pointer {confirmingModRemove ? 'text-red-500 bg-red-500/10' : 'text-terminal-fg hover:text-red-500 hover:bg-terminal-surface'}"
                      >
                        {confirmingModRemove ? '[confirm remove?]' : '[remove]'}
                      </button>
                    {/if}
                  </div>
                {/if}
              </div>
            {/if}

            <span>{localComment.replyCount} {localComment.replyCount === 1 ? 'reply' : 'replies'}</span>
          </div>
        {/if}

        {#if showReportDialog && communityName}
          <ReportDialog
            {communityName}
            contentId={localComment.commentId}
            contentType="comment"
            onClose={() => (showReportDialog = false)}
          />
        {/if}

        <!-- Inline reply form -->
        {#if replying}
          <div class="mt-2">
            <CommentForm
              {postId}
              parentId={localComment.commentId}
              onSubmit={handleReplySubmitted}
              onCancel={() => (replying = false)}
            />
          </div>
        {/if}
      </div>
    </div>
  {/if}
</div>
