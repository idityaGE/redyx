<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { whenReady } from '../lib/auth';
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
  let loading = $state(true);
  let errorStatus = $state<number | null>(null);
  let errorMessage = $state<string | null>(null);

  async function fetchCommunity() {
    loading = true;
    errorStatus = null;
    errorMessage = null;

    try {
      const data = await api<CommunityResponse>(`/communities/${encodeURIComponent(name)}`);
      community = data.community;
      isMember = data.isMember ?? false;
      isModerator = data.isModerator ?? false;
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

      <!-- Posts feed -->
      <CommunityFeed communityName={community.name} />
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
