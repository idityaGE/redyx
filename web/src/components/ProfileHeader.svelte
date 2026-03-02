<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { getUser, isAuthenticated, subscribe } from '../lib/auth';
  import ProfileEditor from './ProfileEditor.svelte';

  interface Props {
    username: string;
  }

  let { username }: Props = $props();

  interface ProfileData {
    userId: string;
    username: string;
    displayName: string;
    bio: string;
    avatarUrl: string;
    karma: number;
    createdAt: string;
  }

  let profile = $state<ProfileData | null>(null);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let isOwnProfile = $state(false);
  let showEditor = $state(false);

  let formattedDate = $derived(
    profile?.createdAt
      ? new Date(profile.createdAt).toISOString().slice(0, 10)
      : ''
  );

  let formattedKarma = $derived(
    profile?.karma !== undefined
      ? profile.karma.toLocaleString()
      : '0'
  );

  let avatarLetter = $derived(
    username ? username.charAt(0).toUpperCase() : '?'
  );

  onMount(() => {
    fetchProfile();

    const unsub = subscribe(() => {
      checkOwnProfile();
    });

    return unsub;
  });

  function checkOwnProfile() {
    const currentUser = getUser();
    isOwnProfile = currentUser?.username === username;
  }

  async function fetchProfile() {
    loading = true;
    error = null;
    try {
      const data = await api<{ user: ProfileData }>(`/users/${username}`);
      profile = data.user;
      checkOwnProfile();
    } catch (e) {
      if (e instanceof ApiError) {
        error = e.status === 404 ? 'user not found' : e.message;
      } else {
        error = 'failed to load profile';
      }
    } finally {
      loading = false;
    }
  }

  function handleProfileUpdate(updated: Partial<ProfileData>) {
    if (profile) {
      profile = { ...profile, ...updated };
    }
  }
</script>

{#if loading}
  <div class="box-terminal mb-4">
    <span class="text-terminal-dim text-xs animate-pulse">[loading profile...]</span>
  </div>
{:else if error}
  <div class="box-terminal mb-4">
    <span class="text-red-500 text-xs">&gt; error:</span>
    <span class="text-red-400 text-xs"> {error}</span>
  </div>
{:else if profile}
  <div class="box-terminal mb-4">
    <div class="flex items-start gap-3">
      <!-- Avatar with ASCII box-drawing border -->
      <div class="shrink-0 font-mono text-terminal-dim text-xs leading-none select-none">
        <div>&#x250C;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2510;</div>
        <div>&#x2502;{#if profile.avatarUrl}<img src={profile.avatarUrl} alt={username} class="inline-block w-10 h-10 object-cover" />{:else}<span class="inline-flex items-center justify-center w-10 h-10 text-lg text-accent-500 bg-terminal-bg font-bold">{avatarLetter}</span>{/if}&#x2502;</div>
        <div>&#x2514;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2500;&#x2518;</div>
      </div>

      <!-- Status line -->
      <div class="flex-1 min-w-0">
        <div class="text-sm text-terminal-fg font-mono">
          <span class="text-accent-500">u/{profile.username}</span>
          <span class="text-terminal-dim mx-1">|</span>
          <span>{formattedKarma} karma</span>
          <span class="text-terminal-dim mx-1">|</span>
          <span class="text-terminal-dim">joined {formattedDate}</span>
        </div>

        {#if profile.displayName && profile.displayName !== profile.username}
          <div class="text-xs text-terminal-dim mt-0.5">
            aka {profile.displayName}
          </div>
        {/if}

        {#if isOwnProfile}
          <button
            class="text-xs text-terminal-dim hover:text-accent-500 mt-1 font-mono transition-colors cursor-pointer"
            onclick={() => { showEditor = !showEditor; }}
          >
            <span class="text-accent-500">&gt;</span> {showEditor ? 'close editor' : 'edit profile'}
          </button>
        {/if}
      </div>
    </div>

    {#if isOwnProfile && showEditor}
      <div class="mt-3 pt-3 border-t border-terminal-border">
        <ProfileEditor
          displayName={profile.displayName ?? ''}
          bio={profile.bio ?? ''}
          avatarUrl={profile.avatarUrl ?? ''}
          onupdate={handleProfileUpdate}
        />
      </div>
    {/if}
  </div>
{/if}
