<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { isAuthenticated, whenReady } from '../../lib/auth';
  import CommunitySidebar from './CommunitySidebar.svelte';
  import CommunityFeed from './CommunityFeed.svelte';

  interface Props {
    name: string;
  }

  type CommunityRule = {
    title: string;
    description: string;
  };

  type Community = {
    communityId: string;
    name: string;
    description: string;
    rules: CommunityRule[];
    visibility: number;
    memberCount: number;
    ownerId: string;
    createdAt: string;
  };

  type CommunityResponse = {
    community: Community;
    isMember: boolean;
    isModerator: boolean;
  };

  let { name }: Props = $props();

  let community = $state<Community | null>(null);
  let isMember = $state(false);
  let isModerator = $state(false);
  let isBanned = $state(false);
  let banReason = $state('');
  let banExpiresAt = $state<string | null>(null);
  let loading = $state(true);
  let errorStatus = $state<number | null>(null);
  let errorMessage = $state<string | null>(null);

  function formatBanExpiry(expiresAt: string | null): string {
    if (!expiresAt) return 'Permanent';
    try {
      const date = new Date(expiresAt);
      if (isNaN(date.getTime())) return 'Permanent';
      return date.toLocaleDateString('en-US', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return 'Permanent';
    }
  }

  async function fetchCommunity() {
    loading = true;
    errorStatus = null;
    errorMessage = null;

    try {
      const data = await api<CommunityResponse>(`/communities/${encodeURIComponent(name)}`);
      community = data.community;
      isMember = data.isMember ?? false;
      isModerator = data.isModerator ?? false;

      // Check ban status for authenticated users (fail-open)
      if (isAuthenticated()) {
        try {
          const banCheck = await api<{ isBanned: boolean; reason?: string; expiresAt?: string }>(
            `/communities/${encodeURIComponent(name)}/moderation/check-ban`,
            { method: 'POST', body: JSON.stringify({ communityName: name }) }
          );
          if (banCheck.isBanned) {
            isBanned = true;
            banReason = banCheck.reason ?? '';
            banExpiresAt = banCheck.expiresAt ?? null;
          }
        } catch {
          // Fail-open: if ban check fails, allow access
        }
      }
    } catch (e) {
      if (e instanceof ApiError) {
        errorStatus = e.status;
        errorMessage = e.message;
      } else {
        errorStatus = 500;
        errorMessage = 'Failed to load community';
      }
    } finally {
      loading = false;
    }
  }

  function handleMembershipChange(newIsMember: boolean, newCount: number) {
    isMember = newIsMember;
    if (community) {
      community = { ...community, memberCount: newCount };
    }
  }

  onMount(() => {
    // Wait for auth so isMember/isModerator are populated correctly
    whenReady().then(() => fetchCommunity());
  });
</script>

{#if loading}
  <div class="flex items-center justify-center min-h-[40vh]">
    <span class="text-xs text-terminal-dim font-mono animate-pulse">[loading r/{name}...]</span>
  </div>
{:else if errorStatus === 404}
  <div class="flex items-center justify-center min-h-[40vh]">
    <div class="box-terminal text-center p-6">
      <div class="text-accent-500 text-sm mb-2">404 — community not found</div>
      <div class="text-xs text-terminal-dim">r/{name} does not exist</div>
      <a href="/communities" class="text-xs text-accent-500 hover:text-accent-400 mt-3 inline-block">
        &gt; browse communities
      </a>
    </div>
  </div>
{:else if errorStatus === 403}
  <div class="flex items-center justify-center min-h-[40vh]">
    <div class="box-terminal text-center p-6">
      <div class="text-accent-500 text-sm mb-2">private community</div>
      <div class="text-xs text-terminal-dim">this community is private — invitation required</div>
      <a href="/communities" class="text-xs text-accent-500 hover:text-accent-400 mt-3 inline-block">
        &gt; browse communities
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
{:else if community}
  <div class="flex flex-col lg:flex-row gap-4 max-w-5xl">
    <!-- Main content area (left) -->
    <div class="flex-1 min-w-0">
      <!-- Community header with breadcrumb -->
      <div class="box-terminal mb-4">
        <div class="text-accent-500 text-sm">
          ~ <a href="/communities" class="hover:text-accent-400 transition-colors">/communities</a>/{community.name}
        </div>
      </div>

      <!-- Ban banner -->
      {#if isBanned}
        <div class="border border-red-500/50 bg-red-500/10 text-red-400 font-mono text-sm p-3 mb-4">
          <div class="text-xs text-red-500 mb-1">&#9484;&#9472; banned &#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9488;</div>
          <div class="text-xs">You are banned from this community.</div>
          {#if banReason}
            <div class="text-xs mt-1">Reason: {banReason}</div>
          {/if}
          <div class="text-xs mt-1">Expires: {formatBanExpiry(banExpiresAt)}</div>
          <div class="text-xs text-red-500 mt-1">&#9492;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9472;&#9496;</div>
        </div>
      {/if}

      <!-- Posts feed -->
      <CommunityFeed communityName={community.name} isMember={isMember && !isBanned} {isModerator} {isBanned} />
    </div>

    <!-- Right sidebar -->
    <div class="w-full lg:w-72 shrink-0">
      <CommunitySidebar
        {community}
        {isMember}
        {isModerator}
        onMembershipChange={handleMembershipChange}
      />
    </div>
  </div>
{/if}
