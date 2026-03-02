<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { getUser, subscribe } from '../lib/auth';
  import ProfileEditor from './ProfileEditor.svelte';

  interface Props {
    username: string;
  }

  let { username }: Props = $props();

  type Tab = 'posts' | 'comments' | 'about';

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

  // About state
  interface AboutData {
    displayName: string;
    bio: string;
    avatarUrl: string;
  }
  let aboutData = $state<AboutData | null>(null);
  let aboutLoading = $state(false);

  const tabs: { id: Tab; label: string }[] = [
    { id: 'posts', label: 'Posts' },
    { id: 'comments', label: 'Comments' },
    { id: 'about', label: 'About' },
  ];

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
          <div class="py-1 text-xs border-b border-terminal-border last:border-0">
            <a href="/post/{post.id}" class="text-terminal-fg hover:text-accent-500 transition-colors">
              {post.title}
            </a>
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
          <div class="py-1 text-xs border-b border-terminal-border last:border-0">
            <div class="text-terminal-fg">{comment.body}</div>
            <div class="text-terminal-dim mt-0.5">
              on <a href="/post/{comment.postId}" class="text-accent-600 hover:text-accent-500">{comment.postTitle ?? 'post'}</a>
            </div>
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
