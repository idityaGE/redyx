<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { getUser, isAuthenticated, isLoading, subscribe } from '../../lib/auth';

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

  type Member = {
    userId: string;
    username: string;
    role: string;
    joinedAt: string;
  };

  interface Props {
    community: Community;
    isMember: boolean;
    isModerator: boolean;
    onMembershipChange: (isMember: boolean, newCount: number) => void;
  }

  let { community, isMember, isModerator, onMembershipChange }: Props = $props();

  let authed = $state(isAuthenticated());
  let authLoading = $state(isLoading());
  let currentUser = $state(getUser());
  let moderators = $state<Member[]>([]);
  let joining = $state(false);
  let leaving = $state(false);

  let isOwner = $derived(currentUser?.userId === community.ownerId);

  const visibilityLabel = (v: number): string => {
    switch (v) {
      case 1: return 'public';
      case 2: return 'restricted';
      case 3: return 'private';
      default: return 'unknown';
    }
  };

  const formatDate = (iso: string): string => {
    try {
      return new Date(iso).toISOString().slice(0, 10);
    } catch {
      return '—';
    }
  };

  async function fetchModerators() {
    try {
      const data = await api<{ members: Member[] }>(
        `/communities/${encodeURIComponent(community.name)}/members?role=moderator`
      );
      moderators = data.members ?? [];
    } catch {
      // Best effort — moderator list is informational
      moderators = [];
    }
  }

  async function handleJoin() {
    joining = true;
    try {
      await api(`/communities/${encodeURIComponent(community.name)}/members`, {
        method: 'POST',
        body: JSON.stringify({}),
      });
      onMembershipChange(true, community.memberCount + 1);
    } catch (e) {
      // silently fail — user can retry
    } finally {
      joining = false;
    }
  }

  async function handleLeave() {
    leaving = true;
    try {
      await api(`/communities/${encodeURIComponent(community.name)}/members`, {
        method: 'DELETE',
      });
      onMembershipChange(false, Math.max(0, community.memberCount - 1));
    } catch (e) {
      // silently fail — user can retry
    } finally {
      leaving = false;
    }
  }

  onMount(() => {
    const unsub = subscribe(() => {
      authed = isAuthenticated();
      authLoading = isLoading();
      currentUser = getUser();
    });

    fetchModerators();

    return unsub;
  });
</script>

<div class="border border-terminal-border bg-terminal-surface font-mono text-xs">
  <!-- Sidebar title -->
  <div class="px-3 py-1.5 border-b border-terminal-border text-terminal-dim">
    ┌─ community info
  </div>

  <div class="p-3 space-y-3">
    <!-- Community name -->
    <div>
      <span class="text-accent-500 font-semibold text-sm">r/{community.name}</span>
      <span class="text-terminal-dim ml-1">{visibilityLabel(community.visibility)}</span>
    </div>

    <!-- Description -->
    {#if community.description}
      <div class="text-terminal-fg leading-relaxed">
        {community.description}
      </div>
    {/if}

    <!-- Divider -->
    <div class="text-terminal-border select-none">────────────────</div>

    <!-- Stats -->
    <div class="space-y-1">
      <div class="flex justify-between">
        <span class="text-terminal-dim">members:</span>
        <span class="text-terminal-fg">{community.memberCount.toLocaleString()}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-terminal-dim">created:</span>
        <span class="text-terminal-fg">{formatDate(community.createdAt)}</span>
      </div>
    </div>

    <!-- Rules -->
    {#if community.rules && community.rules.length > 0}
      <div class="text-terminal-border select-none">────────────────</div>
      <div>
        <div class="text-terminal-dim mb-1">rules:</div>
        {#each community.rules as rule, i}
          <div class="mb-1">
            <span class="text-terminal-dim">{i + 1}.</span>
            <span class="text-terminal-fg font-medium">{rule.title}</span>
            {#if rule.description}
              <div class="text-terminal-dim pl-3">— {rule.description}</div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    <!-- Moderators -->
    {#if moderators.length > 0}
      <div class="text-terminal-border select-none">────────────────</div>
      <div>
        <div class="text-terminal-dim mb-1">moderators:</div>
        {#each moderators as mod, i}
          <div class="flex items-center">
            <span class="text-terminal-dim mr-1">
              {i < moderators.length - 1 ? '├──' : '└──'}
            </span>
            <span class="text-terminal-fg">u/{mod.username}</span>
            {#if mod.role === 'owner'}
              <span class="text-accent-500 ml-1">(owner)</span>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    <!-- Divider before action -->
    <div class="text-terminal-border select-none">────────────────</div>

    <!-- Join/Leave/Login -->
    {#if authLoading}
      <div class="text-terminal-dim animate-pulse">[...]</div>
    {:else if !authed}
      <a href="/login" class="text-accent-500 hover:text-accent-400 transition-colors block">
        &gt; login to join
      </a>
    {:else if isOwner}
      <div class="text-terminal-dim">(owner)</div>
    {:else if isMember}
      <button
        onclick={handleLeave}
        disabled={leaving}
        class="text-terminal-fg hover:text-red-400 transition-colors disabled:opacity-50"
      >
        {#if leaving}
          <span class="text-terminal-dim">[leaving...]</span>
        {:else}
          [leave]
        {/if}
      </button>
    {:else}
      <button
        onclick={handleJoin}
        disabled={joining}
        class="text-accent-500 hover:text-accent-400 transition-colors disabled:opacity-50"
      >
        {#if joining}
          <span class="text-terminal-dim">[joining...]</span>
        {:else}
          [join]
        {/if}
      </button>
    {/if}

    <!-- Settings link for moderators -->
    {#if isModerator}
      <a
        href="/community/{community.name}/settings"
        class="text-accent-500 hover:text-accent-400 transition-colors block"
      >
        [settings]
      </a>
    {/if}
  </div>

  <!-- Bottom border -->
  <div class="px-3 py-1 border-t border-terminal-border text-terminal-dim">
    └─────────────────
  </div>
</div>
