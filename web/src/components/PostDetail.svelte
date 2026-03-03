<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { getUser, isAuthenticated, whenReady, subscribe } from '../lib/auth';
  import VoteButtons from './VoteButtons.svelte';
  import PostBody from './PostBody.svelte';
  import { relativeTime } from '../lib/time';

  interface Props {
    postId: string;
  }

  let { postId }: Props = $props();

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

  type PostResponse = {
    post: Post;
    userVote: number;
    isSaved: boolean;
  };

  // Data state
  let post = $state<Post | null>(null);
  let userVote = $state(0);
  let isSaved = $state(false);
  let loading = $state(true);
  let errorStatus = $state<number | null>(null);
  let errorMessage = $state<string | null>(null);

  // Edit state
  let editing = $state(false);
  let editTitle = $state('');
  let editBody = $state('');
  let editUrl = $state('');
  let editSaving = $state(false);
  let editError = $state<string | null>(null);

  // Delete state
  let confirmingDelete = $state(false);

  // Reactive auth state
  let authed = $state(isAuthenticated());

  // Check if current user owns this post
  let isOwner = $derived((() => {
    if (!post || !authed) return false;
    const user = getUser();
    return user?.userId === post.authorId;
  })());

  function extractDomain(url: string): string {
    try {
      return new URL(url).hostname;
    } catch {
      return '';
    }
  }

  async function fetchPost() {
    loading = true;
    errorStatus = null;
    errorMessage = null;

    try {
      const data = await api<PostResponse>(`/posts/${postId}`);
      post = data.post;
      userVote = data.userVote ?? 0;
      isSaved = data.isSaved ?? false;
    } catch (e) {
      if (e instanceof ApiError) {
        errorStatus = e.status;
        errorMessage = e.message;
      } else {
        errorStatus = 500;
        errorMessage = 'Failed to load post';
      }
    } finally {
      loading = false;
    }
  }

  async function toggleSave() {
    if (!post || !authed) return;

    const prevSaved = isSaved;
    isSaved = !isSaved; // optimistic

    try {
      await api(`/posts/${postId}/save`, {
        method: 'POST',
        body: JSON.stringify({ save: isSaved }),
      });
    } catch {
      isSaved = prevSaved; // rollback
    }
  }

  function startEdit() {
    if (!post) return;
    editTitle = post.title;
    editBody = post.body;
    editUrl = post.url;
    editError = null;
    editing = true;
  }

  function cancelEdit() {
    editing = false;
    editError = null;
  }

  async function saveEdit() {
    if (!post) return;
    editSaving = true;
    editError = null;

    try {
      const res = await api<{ post: Post }>(`/posts/${postId}`, {
        method: 'PATCH',
        body: JSON.stringify({
          title: editTitle.trim(),
          body: post.postType === 'POST_TYPE_TEXT' ? editBody : undefined,
          url: post.postType === 'POST_TYPE_LINK' ? editUrl : undefined,
        }),
      });

      post = res.post;
      editing = false;
    } catch (e) {
      if (e instanceof ApiError) {
        editError = e.message;
      } else {
        editError = 'Failed to save changes';
      }
    } finally {
      editSaving = false;
    }
  }

  async function deletePost() {
    if (!post) return;

    try {
      await api(`/posts/${postId}`, { method: 'DELETE' });
      window.location.href = `/community/${post.communityName}`;
    } catch (e) {
      if (e instanceof ApiError) {
        editError = e.message;
      } else {
        editError = 'Failed to delete post';
      }
      confirmingDelete = false;
    }
  }

  onMount(() => {
    // Wait for auth initialization so the request includes the access token
    // (needed for user_vote and is_saved in GetPostResponse)
    whenReady().then(() => fetchPost());

    const unsub = subscribe(() => {
      authed = isAuthenticated();
    });
    return unsub;
  });
</script>

{#if loading}
  <div class="flex items-center justify-center min-h-[40vh]">
    <span class="text-xs text-terminal-dim font-mono animate-pulse">[loading post...]</span>
  </div>
{:else if errorStatus === 404}
  <div class="flex items-center justify-center min-h-[40vh]">
    <div class="box-terminal text-center p-6">
      <div class="text-accent-500 text-sm mb-2">404 — post not found</div>
      <div class="text-xs text-terminal-dim">this post does not exist or has been deleted</div>
      <a href="/" class="text-xs text-accent-500 hover:text-accent-400 mt-3 inline-block">
        &gt; back to home
      </a>
    </div>
  </div>
{:else if errorMessage}
  <div class="flex items-center justify-center min-h-[40vh]">
    <div class="box-terminal text-center p-6">
      <div class="text-xs font-mono">
        <span class="text-red-500">&gt; error:</span>
        <span class="text-red-400"> {errorMessage}</span>
      </div>
    </div>
  </div>
{:else if post}
  <!-- Post header with path -->
  <div class="box-terminal mb-4">
    <div class="text-accent-500 text-sm">
      ~ /community/<a href="/community/{post.communityName}" class="hover:text-accent-400">{post.communityName}</a>/post/{post.postId.slice(0, 8)}
    </div>
  </div>

  <div class="max-w-3xl">
    <div class="border border-terminal-border bg-terminal-surface">
      <div class="flex gap-3 p-4">
        <!-- Vote column -->
        <VoteButtons {postId} initialScore={post.voteScore} initialVote={userVote} />

        <!-- Content area -->
        <div class="flex-1 min-w-0">
          <!-- Title -->
          {#if editing}
            <input
              type="text"
              maxlength={300}
              bind:value={editTitle}
              class="w-full text-sm font-mono bg-terminal-bg border border-terminal-border px-2 py-1 text-terminal-fg focus:outline-none focus:border-accent-500 mb-2"
            />
          {:else}
            <h1 class="text-lg font-mono text-terminal-fg mb-1">{post.title}</h1>
          {/if}

          <!-- Author + time metadata -->
          <div class="text-xs text-terminal-dim font-mono mb-3">
            <a href="/community/{post.communityName}" class="text-accent-600 hover:text-accent-500">
              r/{post.communityName}
            </a>
            <span class="mx-1">&middot;</span>
            <span>{post.isAnonymous ? '[anonymous]' : `u/${post.authorUsername}`}</span>
            <span class="mx-1">&middot;</span>
            <span>{relativeTime(post.createdAt)}</span>
            {#if post.isEdited && post.editedAt}
              <span class="mx-1">&middot;</span>
              <span class="italic">(edited {relativeTime(post.editedAt)})</span>
            {/if}
          </div>

          <!-- Full content -->
          <div class="mb-4">
            {#if editing}
              <!-- Edit mode -->
              {#if post.postType === 'POST_TYPE_TEXT'}
                <textarea
                  rows={10}
                  maxlength={40000}
                  bind:value={editBody}
                  class="w-full font-mono text-sm bg-terminal-bg border border-terminal-border px-2 py-1 text-terminal-fg focus:outline-none focus:border-accent-500 resize-y"
                ></textarea>
              {:else if post.postType === 'POST_TYPE_LINK'}
                <input
                  type="url"
                  bind:value={editUrl}
                  class="w-full text-sm font-mono bg-terminal-bg border border-terminal-border px-2 py-1 text-terminal-fg focus:outline-none focus:border-accent-500"
                />
              {/if}
            {:else}
              <!-- View mode -->
              {#if post.postType === 'POST_TYPE_TEXT' && post.body}
                <PostBody body={post.body} />
              {:else if post.postType === 'POST_TYPE_LINK' && post.url}
                <div class="text-sm font-mono">
                  <span class="text-terminal-dim">&gt; </span>
                  <a
                    href={post.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-accent-500 hover:text-accent-400 underline break-all"
                  >
                    {post.url}
                  </a>
                  {#if extractDomain(post.url)}
                    <span class="text-terminal-dim text-xs ml-1">({extractDomain(post.url)})</span>
                  {/if}
                </div>
              {:else if post.postType === 'POST_TYPE_MEDIA'}
                <div class="text-xs text-terminal-dim font-mono text-center py-4">
                  <div>┌─────────────────────────────────────┐</div>
                  <div>│  media display — coming in phase 5   │</div>
                  <div>└─────────────────────────────────────┘</div>
                </div>
              {/if}
            {/if}
          </div>

          <!-- Edit mode buttons -->
          {#if editing}
            <div class="flex gap-2 mb-3">
              <button
                onclick={saveEdit}
                disabled={editSaving}
                class="text-xs font-mono px-3 py-1 border border-terminal-border bg-terminal-bg text-accent-500 hover:text-accent-400 transition-colors cursor-pointer disabled:text-terminal-dim disabled:cursor-not-allowed"
              >
                {editSaving ? '[saving...]' : '[save changes]'}
              </button>
              <button
                onclick={cancelEdit}
                class="text-xs font-mono px-3 py-1 border border-terminal-border bg-terminal-bg text-terminal-dim hover:text-terminal-fg transition-colors cursor-pointer"
              >
                [cancel]
              </button>
            </div>
            {#if editError}
              <div class="text-xs font-mono mb-3">
                <span class="text-red-500">&gt; error:</span>
                <span class="text-red-400"> {editError}</span>
              </div>
            {/if}
          {/if}

          <!-- Action bar -->
          <div class="flex items-center gap-3 text-xs font-mono border-t border-terminal-border pt-2">
            <!-- Comment count -->
            <span class="text-terminal-dim">{post.commentCount} comments</span>

            <!-- Save/unsave -->
            {#if authed}
              <button
                onclick={toggleSave}
                class="text-terminal-dim hover:text-accent-500 transition-colors cursor-pointer"
              >
                {isSaved ? '[unsave]' : '[save]'}
              </button>
            {/if}

            <!-- Owner actions -->
            {#if isOwner && !editing}
              <button
                onclick={startEdit}
                class="text-terminal-dim hover:text-accent-500 transition-colors cursor-pointer"
              >
                [edit]
              </button>

              {#if confirmingDelete}
                <span class="text-red-400">
                  delete? 
                  <button onclick={deletePost} class="text-red-500 hover:text-red-400 underline cursor-pointer">yes</button>
                  <span class="mx-0.5">/</span>
                  <button onclick={() => (confirmingDelete = false)} class="text-terminal-dim hover:text-terminal-fg underline cursor-pointer">no</button>
                </span>
              {:else}
                <button
                  onclick={() => (confirmingDelete = true)}
                  class="text-terminal-dim hover:text-red-500 transition-colors cursor-pointer"
                >
                  [delete]
                </button>
              {/if}
            {/if}
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}
