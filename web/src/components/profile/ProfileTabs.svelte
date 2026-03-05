<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { getUser, subscribe } from '../../lib/auth';
  import { relativeTime } from '../../lib/time';
  import ProfileEditor from './ProfileEditor.svelte';
  import VoteButtons from '../post/VoteButtons.svelte';

  interface Props {
    username: string;
  }

  let { username }: Props = $props();

  type Tab = 'posts' | 'comments' | 'saved' | 'about';

  let activeTab = $state<Tab>('posts');
  let isOwnProfile = $state(false);

  // Posts state
  let posts = $state<any[]>([]);
  let postsLoading = $state(false);
  let postsError = $state<string | null>(null);

  // Comments state
  let comments = $state<any[]>([]);
  let commentsLoading = $state(false);
  let commentsError = $state<string | null>(null);

  // Saved state
  let savedPosts = $state<any[]>([]);
  let savedLoading = $state(false);
  let savedError = $state<string | null>(null);

  // About state
  interface AboutData {
    displayName: string;
    bio: string;
    avatarUrl: string;
  }
  let aboutData = $state<AboutData | null>(null);
  let aboutLoading = $state(false);

  const tabs = $derived<{ id: Tab; label: string }[]>([
    { id: 'posts', label: 'Posts' },
    { id: 'comments', label: 'Comments' },
    ...(isOwnProfile ? [{ id: 'saved' as Tab, label: 'Saved' }] : []),
    { id: 'about', label: 'About' },
  ]);

  onMount(() => {
    checkOwnProfile();
    fetchTabData();

    const unsub = subscribe(() => {
      checkOwnProfile();
    });

    return unsub;
  });

  function checkOwnProfile() {
    const currentUser = getUser();
    isOwnProfile = currentUser?.username === username;
  }

  function switchTab(tab: Tab) {
    activeTab = tab;
    fetchTabData();
  }

  async function fetchTabData() {
    if (activeTab === 'posts') {
      await fetchPosts();
    } else if (activeTab === 'comments') {
      await fetchComments();
    } else if (activeTab === 'saved') {
      await fetchSaved();
    } else if (activeTab === 'about') {
      await fetchAbout();
    }
  }

  async function fetchPosts() {
    if (posts.length > 0) return;
    postsLoading = true;
    postsError = null;
    try {
      const data = await api<{ posts: any[] }>(`/users/${username}/posts`);
      posts = data.posts ?? [];
    } catch (e) {
      if (e instanceof ApiError) {
        postsError = e.message;
      } else {
        postsError = 'failed to load posts';
      }
    } finally {
      postsLoading = false;
    }
  }

  async function fetchComments() {
    if (comments.length > 0) return;
    commentsLoading = true;
    commentsError = null;
    try {
      const data = await api<{ comments: any[] }>(`/users/${username}/comments`);
      comments = data.comments ?? [];
    } catch (e) {
      if (e instanceof ApiError) {
        commentsError = e.message;
      } else {
        commentsError = 'failed to load comments';
      }
    } finally {
      commentsLoading = false;
    }
  }

  async function fetchSaved() {
    if (savedPosts.length > 0) return;
    savedLoading = true;
    savedError = null;
    try {
      const data = await api<{ posts: any[] }>('/saved?pagination.limit=25');
      savedPosts = data.posts ?? [];
    } catch (e) {
      if (e instanceof ApiError) {
        savedError = e.message;
      } else {
        savedError = 'failed to load saved posts';
      }
    } finally {
      savedLoading = false;
    }
  }

  async function fetchAbout() {
    if (aboutData) return;
    aboutLoading = true;
    try {
      const data = await api<{ user: { displayName: string; bio: string; avatarUrl: string } }>(`/users/${username}`);
      aboutData = {
        displayName: data.user.displayName ?? '',
        bio: data.user.bio ?? '',
        avatarUrl: data.user.avatarUrl ?? '',
      };
    } catch {
      aboutData = { displayName: '', bio: '', avatarUrl: '' };
    } finally {
      aboutLoading = false;
    }
  }

  function handleAboutUpdate(updated: Partial<AboutData>) {
    if (aboutData) {
      aboutData = { ...aboutData, ...updated };
    }
  }
</script>

<!-- Tab navigation — terminal multiplexer style -->
<div class="border border-terminal-border bg-terminal-surface font-mono">
  <!-- Tab bar -->
  <div class="flex border-b border-terminal-border text-xs">
    {#each tabs as tab}
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

  <!-- Tab content -->
  <div class="p-4">
    {#if activeTab === 'posts'}
      {#if postsLoading}
        <div class="text-terminal-dim text-xs animate-pulse">[loading posts...]</div>
      {:else if postsError}
        <div class="text-xs">
          <span class="text-red-500">&gt; error:</span>
          <span class="text-red-400"> {postsError}</span>
        </div>
      {:else if posts.length === 0}
        <div class="text-terminal-dim text-xs">
          <span class="text-terminal-dim">&gt;</span> no posts yet
        </div>
      {:else}
        {#each posts as post}
          <div class="flex items-center gap-3 px-2 py-1.5 border-b border-terminal-border hover:bg-terminal-surface transition-colors text-xs font-mono group">
            <VoteButtons postId={post.postId ?? post.id} initialScore={post.voteScore ?? 0} initialVote={0} authorId={post.authorId ?? ''} />
            <div class="flex-1 min-w-0">
              <a
                href="/post/{post.postId ?? post.id}"
                class="text-terminal-fg group-hover:text-accent-500 transition-colors truncate block text-sm"
              >
                {post.title}
                {#if post.postType === 'POST_TYPE_LINK' && post.url}
                  <span class="text-terminal-dim text-xs ml-1">({new URL(post.url).hostname})</span>
                {/if}
              </a>
              <div class="text-terminal-dim mt-0.5">
                {#if post.communityName}
                  <a href="/community/{post.communityName}" class="text-accent-600 hover:text-accent-500">r/{post.communityName}</a>
                  <span class="mx-1">&middot;</span>
                {/if}
                <span>{relativeTime(post.createdAt)}</span>
                <span class="mx-1">&middot;</span>
                <span>{post.commentCount ?? 0} comments</span>
              </div>
            </div>
            {#if post.postType === 'POST_TYPE_MEDIA' && post.thumbnailUrl}
              <div class="shrink-0">
                <img src={post.thumbnailUrl} alt="" class="w-8 h-8 object-cover border border-terminal-border rounded-sm" loading="lazy" />
              </div>
            {/if}
          </div>
        {/each}
      {/if}

    {:else if activeTab === 'comments'}
      {#if commentsLoading}
        <div class="text-terminal-dim text-xs animate-pulse">[loading comments...]</div>
      {:else if commentsError}
        <div class="text-xs">
          <span class="text-red-500">&gt; error:</span>
          <span class="text-red-400"> {commentsError}</span>
        </div>
      {:else if comments.length === 0}
        <div class="text-terminal-dim text-xs">
          <span class="text-terminal-dim">&gt;</span> no comments yet
        </div>
      {:else}
        {#each comments as comment}
          <a
            href="/post/{comment.postId}"
            class="block px-2 py-2 border-b border-terminal-border hover:bg-terminal-surface transition-colors text-xs font-mono group"
          >
            <!-- Comment context line -->
            <div class="text-terminal-dim mb-1 flex items-center gap-1">
              <span class="text-accent-600">&#9654;</span>
              <span>commented on</span>
              <span class="text-terminal-fg group-hover:text-accent-500">{comment.postTitle ?? 'a post'}</span>
              {#if comment.communityName}
                <span>in</span>
                <span class="text-accent-600">r/{comment.communityName}</span>
              {/if}
            </div>
            <!-- Comment body -->
            <div class="text-terminal-fg pl-4 border-l-2 border-terminal-border/50 line-clamp-3">
              {comment.body}
            </div>
            <!-- Comment metadata -->
            <div class="text-terminal-dim mt-1 pl-4 flex items-center gap-2">
              {#if comment.voteScore !== undefined}
                <span>{comment.voteScore} pts</span>
                <span>&middot;</span>
              {/if}
              <span>{relativeTime(comment.createdAt)}</span>
            </div>
          </a>
        {/each}
      {/if}

    {:else if activeTab === 'saved'}
      {#if savedLoading}
        <div class="text-terminal-dim text-xs animate-pulse">[loading saved posts...]</div>
      {:else if savedError}
        <div class="text-xs">
          <span class="text-red-500">&gt; error:</span>
          <span class="text-red-400"> {savedError}</span>
        </div>
      {:else if savedPosts.length === 0}
        <div class="text-terminal-dim text-xs">
          <span class="text-terminal-dim">&gt;</span> no saved posts
        </div>
      {:else}
        {#each savedPosts as post}
          <div class="flex items-center gap-3 px-2 py-1.5 border-b border-terminal-border hover:bg-terminal-surface transition-colors text-xs font-mono group">
            <VoteButtons postId={post.postId ?? post.id} initialScore={post.voteScore ?? 0} initialVote={0} authorId={post.authorId ?? ''} />
            <div class="flex-1 min-w-0">
              <a
                href="/post/{post.postId ?? post.id}"
                class="text-terminal-fg group-hover:text-accent-500 transition-colors truncate block text-sm"
              >
                {post.title}
              </a>
              <div class="text-terminal-dim mt-0.5">
                {#if post.communityName}
                  <a href="/community/{post.communityName}" class="text-accent-600 hover:text-accent-500">r/{post.communityName}</a>
                  <span class="mx-1">&middot;</span>
                {/if}
                <span>{post.isAnonymous ? '[anonymous]' : `u/${post.authorUsername}`}</span>
                <span class="mx-1">&middot;</span>
                <span>{relativeTime(post.createdAt)}</span>
                <span class="mx-1">&middot;</span>
                <span>{post.commentCount ?? 0} comments</span>
              </div>
            </div>
            {#if post.postType === 'POST_TYPE_MEDIA' && post.thumbnailUrl}
              <div class="shrink-0">
                <img src={post.thumbnailUrl} alt="" class="w-8 h-8 object-cover border border-terminal-border rounded-sm" loading="lazy" />
              </div>
            {/if}
          </div>
        {/each}
      {/if}

    {:else if activeTab === 'about'}
      {#if aboutLoading}
        <div class="text-terminal-dim text-xs animate-pulse">[loading about...]</div>
      {:else if aboutData}
        <div class="text-xs font-mono space-y-2">
          {#if aboutData.displayName}
            <div>
              <span class="text-terminal-dim">display_name:</span>
              <span class="text-terminal-fg ml-1">{aboutData.displayName}</span>
            </div>
          {/if}

          {#if aboutData.bio}
            <div>
              <span class="text-terminal-dim">bio:</span>
              <pre class="text-terminal-fg mt-1 whitespace-pre-wrap text-xs leading-relaxed">{aboutData.bio}</pre>
            </div>
          {:else}
            <div class="text-terminal-dim">
              <span>&gt;</span> no bio set
            </div>
          {/if}

          {#if isOwnProfile}
            <div class="pt-2 border-t border-terminal-border">
              <ProfileEditor
                displayName={aboutData.displayName}
                bio={aboutData.bio}
                avatarUrl={aboutData.avatarUrl}
                onupdate={handleAboutUpdate}
              />
            </div>
          {/if}
        </div>
      {/if}
    {/if}
  </div>
</div>
