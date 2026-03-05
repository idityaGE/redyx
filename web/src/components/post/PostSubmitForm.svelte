<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { isAuthenticated, whenReady } from '../../lib/auth';
  import PostBody from './PostBody.svelte';
  import MediaUpload from '../media/MediaUpload.svelte';

  interface Props {
    communityName: string;
  }

  let { communityName }: Props = $props();

  // Tab state
  type PostTab = 'text' | 'link' | 'media';
  let activeTab = $state<PostTab>('text');

  // Write/Preview sub-tab for text body
  type BodyMode = 'write' | 'preview';
  let bodyMode = $state<BodyMode>('write');

  // Form fields
  let title = $state('');
  let body = $state('');
  let url = $state('');
  let isAnonymous = $state(false);

  // Submission state
  let submitting = $state(false);
  let error = $state<string | null>(null);

  // Media state
  let mediaIds = $state<string[]>([]);
  let mediaUploading = $derived(false); // Will be true if any uploads in progress

  function handleMediaChange(ids: string[]) {
    mediaIds = ids;
  }

  // URL validation
  let urlValid = $derived((() => {
    if (!url) return false;
    try {
      new URL(url);
      return true;
    } catch {
      return false;
    }
  })());

  let urlDomain = $derived((() => {
    if (!url) return '';
    try {
      return new URL(url).hostname;
    } catch {
      return '';
    }
  })());

  // Submit button disabled state
  let submitDisabled = $derived(
    submitting ||
    !title.trim() ||
    (activeTab === 'text' && !body.trim()) ||
    (activeTab === 'link' && !urlValid) ||
    (activeTab === 'media' && mediaIds.length === 0)
  );

  // Auth guard — wait for auth initialization before checking
  onMount(() => {
    whenReady().then(() => {
      if (!isAuthenticated()) {
        window.location.href = `/login?redirect=/community/${encodeURIComponent(communityName)}/submit`;
      }
    });
  });

  const postTabs: { id: PostTab; label: string }[] = [
    { id: 'text', label: 'Text' },
    { id: 'link', label: 'Link' },
    { id: 'media', label: 'Media' },
  ];

  function switchTab(tab: PostTab) {
    activeTab = tab;
    error = null;
  }

  async function handleSubmit() {
    if (submitDisabled) return;

    submitting = true;
    error = null;

    try {
      const postType =
        activeTab === 'text'
          ? 'POST_TYPE_TEXT'
          : activeTab === 'link'
            ? 'POST_TYPE_LINK'
            : 'POST_TYPE_MEDIA';

      const res = await api<{ post: { postId: string } }>('/posts', {
        method: 'POST',
        body: JSON.stringify({
          communityName,
          title: title.trim(),
          body: activeTab === 'text' ? body : '',
          url: activeTab === 'link' ? url : '',
          postType,
          isAnonymous,
          mediaIds: activeTab === 'media' ? mediaIds : [],
        }),
      });

      // Redirect to new post detail page
      window.location.href = `/post/${res.post.postId}`;
    } catch (e) {
      if (e instanceof ApiError) {
        error = e.message;
      } else {
        error = 'Failed to create post';
      }
      submitting = false;
    }
  }
</script>

<!-- Header with breadcrumb -->
<div class="box-terminal mb-4">
  <div class="text-accent-500 text-sm">
    ~ <a href="/communities" class="hover:text-accent-400 transition-colors">/communities</a>/<a href="/community/{communityName}" class="hover:text-accent-400 transition-colors">{communityName}</a>/submit
  </div>
</div>

<div class="border border-terminal-border bg-terminal-surface font-mono max-w-2xl">
  <!-- Tab bar -->
  <div class="flex border-b border-terminal-border text-xs">
    {#each postTabs as tab}
      <button
        class="px-4 py-1.5 transition-colors cursor-pointer {activeTab === tab.id
          ? 'bg-terminal-bg text-accent-500 border-b-2 border-accent-500'
          : 'text-terminal-dim hover:text-terminal-fg hover:bg-terminal-bg'}"
        onclick={() => switchTab(tab.id)}
      >
        [{tab.label}]
      </button>
    {/each}
  </div>

  <div class="p-4 space-y-4">
    <!-- Title input (common to all tabs) -->
    <div>
      <input
        type="text"
        placeholder="Title"
        maxlength={300}
        bind:value={title}
        class="w-full text-sm font-mono bg-terminal-bg border border-terminal-border px-3 py-2 text-terminal-fg placeholder:text-terminal-dim focus:outline-none focus:border-accent-500"
      />
      <div class="text-xs text-terminal-dim mt-1 text-right">{title.length}/300</div>
    </div>

    <!-- Text tab content -->
    {#if activeTab === 'text'}
      <div>
        <!-- Write/Preview sub-tabs -->
        <div class="flex border border-terminal-border border-b-0 text-xs">
          <button
            class="px-3 py-1 transition-colors cursor-pointer {bodyMode === 'write'
              ? 'bg-terminal-bg text-accent-500'
              : 'text-terminal-dim hover:text-terminal-fg'}"
            onclick={() => (bodyMode = 'write')}
          >
            [Write]
          </button>
          <button
            class="px-3 py-1 transition-colors cursor-pointer {bodyMode === 'preview'
              ? 'bg-terminal-bg text-accent-500'
              : 'text-terminal-dim hover:text-terminal-fg'}"
            onclick={() => (bodyMode = 'preview')}
          >
            [Preview]
          </button>
        </div>

        {#if bodyMode === 'write'}
          <textarea
            placeholder="Body (markdown supported)"
            maxlength={40000}
            rows={12}
            bind:value={body}
            class="w-full font-mono text-sm bg-terminal-bg border border-terminal-border px-3 py-2 text-terminal-fg placeholder:text-terminal-dim focus:outline-none focus:border-accent-500 resize-y"
          ></textarea>
        {:else}
          <div class="border border-terminal-border bg-terminal-bg px-3 py-2 min-h-[192px]">
            {#if body.trim()}
              <PostBody {body} />
            {:else}
              <div class="text-xs text-terminal-dim">nothing to preview</div>
            {/if}
          </div>
        {/if}

        <div class="text-xs text-terminal-dim mt-1 text-right">{body.length}/40000</div>
      </div>

    <!-- Link tab content -->
    {:else if activeTab === 'link'}
      <div>
        <input
          type="url"
          placeholder="https://example.com"
          bind:value={url}
          class="w-full text-sm font-mono bg-terminal-bg border border-terminal-border px-3 py-2 text-terminal-fg placeholder:text-terminal-dim focus:outline-none focus:border-accent-500"
        />
        {#if urlDomain}
          <div class="text-xs text-terminal-dim mt-1">({urlDomain})</div>
        {/if}
      </div>

    <!-- Media tab content -->
    {:else if activeTab === 'media'}
      <MediaUpload onMediaChange={handleMediaChange} />
      {#if mediaIds.length > 0}
        <div class="text-xs text-terminal-dim font-mono mt-2">
          &gt; {mediaIds.length} file{mediaIds.length !== 1 ? 's' : ''} attached
        </div>
      {/if}
    {/if}

    <!-- Anonymous checkbox -->
    <label class="flex items-center gap-2 text-xs text-terminal-dim cursor-pointer select-none">
      <input
        type="checkbox"
        bind:checked={isAnonymous}
        class="accent-accent-500"
      />
      Post anonymously
    </label>

    <!-- Error message -->
    {#if error}
      <div class="text-xs font-mono">
        <span class="text-red-500">&gt; error:</span>
        <span class="text-red-400"> {error}</span>
      </div>
    {/if}

    <!-- Submit button -->
    <button
      onclick={handleSubmit}
      disabled={submitDisabled}
      class="text-xs font-mono px-4 py-1.5 border border-terminal-border transition-colors cursor-pointer
        {submitDisabled
          ? 'bg-terminal-surface text-terminal-dim cursor-not-allowed'
          : 'bg-terminal-bg text-accent-500 hover:bg-terminal-surface hover:text-accent-400'}"
    >
      {submitting ? '[submitting...]' : '[submit post]'}
    </button>
  </div>
</div>
