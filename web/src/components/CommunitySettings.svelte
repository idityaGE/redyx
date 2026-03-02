<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { getUser, isAuthenticated, isLoading, initialize, subscribe } from '../lib/auth';

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

  type Member = {
    userId: string;
    username: string;
    role: string;
    joinedAt: string;
  };

  let { name }: Props = $props();

  // Community data
  let community = $state<Community | null>(null);
  let isModerator = $state(false);
  let loading = $state(true);

  // Auth state
  let authed = $state(isAuthenticated());
  let authLoading = $state(isLoading());
  let currentUser = $state(getUser());

  // Description section
  let editDescription = $state('');
  let descSaving = $state(false);
  let descMessage = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  // Rules section
  let editRules = $state<CommunityRule[]>([]);
  let rulesSaving = $state(false);
  let rulesMessage = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  // Visibility section
  let editVisibility = $state(1);
  let visSaving = $state(false);
  let visMessage = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  // Moderators section
  let moderators = $state<Member[]>([]);
  let newModUsername = $state('');
  let modAdding = $state(false);
  let modMessage = $state<{ type: 'success' | 'error'; text: string } | null>(null);

  let isOwner = $derived(currentUser?.userId === community?.ownerId);

  const visibilityOptions = [
    { value: 1, label: 'public', desc: 'anyone can view and join' },
    { value: 2, label: 'restricted', desc: 'anyone can view, approval to join' },
    { value: 3, label: 'private', desc: 'invitation only, hidden from browse' },
  ];

  async function fetchCommunity() {
    loading = true;
    try {
      const data = await api<CommunityResponse>(`/communities/${encodeURIComponent(name)}`);
      community = data.community;
      isModerator = data.isModerator ?? false;

      // Populate edit fields
      editDescription = data.community.description ?? '';
      editRules = (data.community.rules ?? []).map(r => ({ ...r }));
      editVisibility = data.community.visibility ?? 1;

      if (!data.isModerator) {
        window.location.href = `/community/${name}`;
        return;
      }

      // Fetch moderators
      await fetchModerators();
    } catch (e) {
      if (e instanceof ApiError) {
        // Not authorized or not found — redirect
        window.location.href = `/community/${name}`;
      }
    } finally {
      loading = false;
    }
  }

  async function fetchModerators() {
    try {
      const data = await api<{ members: Member[] }>(
        `/communities/${encodeURIComponent(name)}/members?role=moderator`
      );
      moderators = data.members ?? [];
    } catch {
      moderators = [];
    }
  }

  // ── Description ──────────────────────────────────────

  async function saveDescription() {
    descSaving = true;
    descMessage = null;
    try {
      await api(`/communities/${encodeURIComponent(name)}`, {
        method: 'PATCH',
        body: JSON.stringify({ description: editDescription }),
      });
      if (community) {
        community = { ...community, description: editDescription };
      }
      descMessage = { type: 'success', text: 'description updated' };
    } catch (e) {
      descMessage = { type: 'error', text: e instanceof ApiError ? e.message : 'failed to save' };
    } finally {
      descSaving = false;
    }
  }

  // ── Rules ────────────────────────────────────────────

  function addRule() {
    editRules = [...editRules, { title: '', description: '' }];
  }

  function removeRule(index: number) {
    editRules = editRules.filter((_, i) => i !== index);
  }

  function updateRuleTitle(index: number, value: string) {
    editRules = editRules.map((r, i) => i === index ? { ...r, title: value } : r);
  }

  function updateRuleDescription(index: number, value: string) {
    editRules = editRules.map((r, i) => i === index ? { ...r, description: value } : r);
  }

  function moveRuleUp(index: number) {
    if (index <= 0) return;
    const arr = [...editRules];
    [arr[index - 1], arr[index]] = [arr[index], arr[index - 1]];
    editRules = arr;
  }

  function moveRuleDown(index: number) {
    if (index >= editRules.length - 1) return;
    const arr = [...editRules];
    [arr[index], arr[index + 1]] = [arr[index + 1], arr[index]];
    editRules = arr;
  }

  async function saveRules() {
    rulesSaving = true;
    rulesMessage = null;
    try {
      const validRules = editRules
        .filter(r => r.title.trim() !== '')
        .map(r => ({ title: r.title.trim(), description: r.description.trim() }));

      await api(`/communities/${encodeURIComponent(name)}`, {
        method: 'PATCH',
        body: JSON.stringify({ rules: validRules }),
      });
      if (community) {
        community = { ...community, rules: validRules };
      }
      editRules = validRules.map(r => ({ ...r }));
      rulesMessage = { type: 'success', text: 'rules updated' };
    } catch (e) {
      rulesMessage = { type: 'error', text: e instanceof ApiError ? e.message : 'failed to save' };
    } finally {
      rulesSaving = false;
    }
  }

  // ── Visibility ───────────────────────────────────────

  async function saveVisibility() {
    visSaving = true;
    visMessage = null;
    try {
      await api(`/communities/${encodeURIComponent(name)}`, {
        method: 'PATCH',
        body: JSON.stringify({ visibility: editVisibility }),
      });
      if (community) {
        community = { ...community, visibility: editVisibility };
      }
      visMessage = { type: 'success', text: 'visibility updated' };
    } catch (e) {
      visMessage = { type: 'error', text: e instanceof ApiError ? e.message : 'failed to save' };
    } finally {
      visSaving = false;
    }
  }

  // ── Moderators ───────────────────────────────────────

  async function addModerator() {
    if (!newModUsername.trim()) return;

    modAdding = true;
    modMessage = null;

    try {
      // Look up user by username to get userId
      const userProfile = await api<{ userId: string; username: string }>(
        `/users/${encodeURIComponent(newModUsername.trim())}`
      );

      // Assign moderator role
      await api(`/communities/${encodeURIComponent(name)}/moderators`, {
        method: 'POST',
        body: JSON.stringify({ userId: userProfile.userId }),
      });

      newModUsername = '';
      modMessage = { type: 'success', text: `u/${userProfile.username} added as moderator` };
      await fetchModerators();
    } catch (e) {
      modMessage = { type: 'error', text: e instanceof ApiError ? e.message : 'failed to add moderator' };
    } finally {
      modAdding = false;
    }
  }

  async function revokeModerator(userId: string, username: string) {
    modMessage = null;
    try {
      await api(`/communities/${encodeURIComponent(name)}/moderators/${userId}`, {
        method: 'DELETE',
      });
      modMessage = { type: 'success', text: `u/${username} removed as moderator` };
      await fetchModerators();
    } catch (e) {
      modMessage = { type: 'error', text: e instanceof ApiError ? e.message : 'failed to revoke' };
    }
  }

  onMount(() => {
    // Ensure auth is initialized before checking
    initialize();

    const unsub = subscribe(() => {
      authed = isAuthenticated();
      authLoading = isLoading();
      currentUser = getUser();
    });

    fetchCommunity();

    return unsub;
  });

  // Redirect to login only AFTER auth loading completes
  $effect(() => {
    if (!authLoading && !authed) {
      window.location.href = '/login';
    }
  });
</script>

{#if loading || authLoading}
  <div class="flex items-center justify-center min-h-[40vh]">
    <span class="text-xs text-terminal-dim font-mono animate-pulse">[loading settings...]</span>
  </div>
{:else if !community || !isModerator}
  <div class="flex items-center justify-center min-h-[40vh]">
    <span class="text-xs text-terminal-dim font-mono">&gt; redirecting...</span>
  </div>
{:else}
  <div class="max-w-2xl">
    <!-- Page header -->
    <div class="box-terminal mb-4">
      <div class="text-accent-500 text-sm">~ /community/{community.name}/settings</div>
      <div class="text-xs text-terminal-dim mt-1">moderator controls</div>
    </div>

    <div class="space-y-4">

      <!-- ── Description section ─────────────────── -->
      <div class="border border-terminal-border bg-terminal-surface font-mono">
        <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim">
          ┌─ description
        </div>
        <div class="p-3 space-y-2">
          <textarea
            bind:value={editDescription}
            rows="4"
            class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs text-terminal-fg placeholder:text-terminal-dim outline-none font-mono resize-y focus:border-accent-500 transition-colors"
            placeholder="community description (markdown supported)"
          ></textarea>
          <div class="flex items-center justify-between">
            {#if descMessage}
              <span class="text-xs {descMessage.type === 'success' ? 'text-green-500' : 'text-red-400'}">
                &gt; {descMessage.text}
              </span>
            {:else}
              <span></span>
            {/if}
            <button
              onclick={saveDescription}
              disabled={descSaving}
              class="text-xs border border-terminal-border px-2 py-0.5 text-terminal-fg hover:text-accent-500 hover:border-accent-500 transition-colors disabled:opacity-50"
            >
              {descSaving ? '[saving...]' : '[save]'}
            </button>
          </div>
        </div>
        <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
          └─────────────────
        </div>
      </div>

      <!-- ── Rules section ───────────────────────── -->
      <div class="border border-terminal-border bg-terminal-surface font-mono">
        <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim flex items-center justify-between">
          <span>┌─ rules</span>
          <button
            onclick={addRule}
            class="text-accent-500 hover:text-accent-400 transition-colors"
          >
            [+ add]
          </button>
        </div>
        <div class="p-3 space-y-2">
          {#if editRules.length === 0}
            <div class="text-xs text-terminal-dim italic">no rules defined</div>
          {/if}

          {#each editRules as rule, i}
            <div class="border border-terminal-border bg-terminal-bg p-2 space-y-1">
              <div class="flex items-center justify-between text-xs">
                <span class="text-terminal-dim">rule {i + 1}:</span>
                <div class="flex items-center gap-2">
                  {#if i > 0}
                    <button
                      onclick={() => moveRuleUp(i)}
                      class="text-terminal-dim hover:text-accent-500 transition-colors"
                      title="Move up"
                    >
                      [▲]
                    </button>
                  {/if}
                  {#if i < editRules.length - 1}
                    <button
                      onclick={() => moveRuleDown(i)}
                      class="text-terminal-dim hover:text-accent-500 transition-colors"
                      title="Move down"
                    >
                      [▼]
                    </button>
                  {/if}
                  <button
                    onclick={() => removeRule(i)}
                    class="text-red-500 hover:text-red-400 transition-colors"
                  >
                    [remove]
                  </button>
                </div>
              </div>
              <input
                type="text"
                value={rule.title}
                oninput={(e) => updateRuleTitle(i, (e.target as HTMLInputElement).value)}
                placeholder="rule title"
                class="w-full bg-terminal-surface border border-terminal-border px-2 py-0.5 text-xs text-terminal-fg placeholder:text-terminal-dim outline-none font-mono focus:border-accent-500 transition-colors"
              />
              <input
                type="text"
                value={rule.description}
                oninput={(e) => updateRuleDescription(i, (e.target as HTMLInputElement).value)}
                placeholder="description (optional)"
                class="w-full bg-terminal-surface border border-terminal-border px-2 py-0.5 text-xs text-terminal-fg placeholder:text-terminal-dim outline-none font-mono focus:border-accent-500 transition-colors"
              />
            </div>
          {/each}

          <div class="flex items-center justify-between">
            {#if rulesMessage}
              <span class="text-xs {rulesMessage.type === 'success' ? 'text-green-500' : 'text-red-400'}">
                &gt; {rulesMessage.text}
              </span>
            {:else}
              <span></span>
            {/if}
            <button
              onclick={saveRules}
              disabled={rulesSaving}
              class="text-xs border border-terminal-border px-2 py-0.5 text-terminal-fg hover:text-accent-500 hover:border-accent-500 transition-colors disabled:opacity-50"
            >
              {rulesSaving ? '[saving...]' : '[save rules]'}
            </button>
          </div>
        </div>
        <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
          └─────────────────
        </div>
      </div>

      <!-- ── Visibility section ──────────────────── -->
      <div class="border border-terminal-border bg-terminal-surface font-mono">
        <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim">
          ┌─ visibility
        </div>
        <div class="p-3 space-y-2">
          {#each visibilityOptions as opt}
            <label class="flex items-start gap-2 cursor-pointer group">
              <input
                type="radio"
                name="settings-visibility"
                value={opt.value}
                checked={editVisibility === opt.value}
                onchange={() => { editVisibility = opt.value; }}
                class="mt-0.5 accent-accent-500"
              />
              <div class="text-xs">
                <span class="text-terminal-fg group-hover:text-accent-500 transition-colors">{opt.label}</span>
                <span class="text-terminal-dim ml-1">— {opt.desc}</span>
              </div>
            </label>
          {/each}

          <div class="flex items-center justify-between mt-2">
            {#if visMessage}
              <span class="text-xs {visMessage.type === 'success' ? 'text-green-500' : 'text-red-400'}">
                &gt; {visMessage.text}
              </span>
            {:else}
              <span></span>
            {/if}
            <button
              onclick={saveVisibility}
              disabled={visSaving}
              class="text-xs border border-terminal-border px-2 py-0.5 text-terminal-fg hover:text-accent-500 hover:border-accent-500 transition-colors disabled:opacity-50"
            >
              {visSaving ? '[saving...]' : '[save]'}
            </button>
          </div>
        </div>
        <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
          └─────────────────
        </div>
      </div>

      <!-- ── Moderators section (owner only) ─────── -->
      {#if isOwner}
        <div class="border border-terminal-border bg-terminal-surface font-mono">
          <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim">
            ┌─ moderators (owner only)
          </div>
          <div class="p-3 space-y-3">
            <!-- Current moderators -->
            {#if moderators.length === 0}
              <div class="text-xs text-terminal-dim italic">no moderators yet</div>
            {:else}
              {#each moderators as mod, i}
                <div class="flex items-center justify-between text-xs">
                  <div class="flex items-center">
                    <span class="text-terminal-dim mr-1">
                      {i < moderators.length - 1 ? '├──' : '└──'}
                    </span>
                    <span class="text-terminal-fg">u/{mod.username}</span>
                    {#if mod.role === 'owner'}
                      <span class="text-accent-500 ml-1">(owner)</span>
                    {/if}
                  </div>
                  {#if mod.role !== 'owner' && mod.userId !== currentUser?.userId}
                    <button
                      onclick={() => revokeModerator(mod.userId, mod.username)}
                      class="text-red-500 hover:text-red-400 transition-colors"
                    >
                      [revoke]
                    </button>
                  {/if}
                </div>
              {/each}
            {/if}

            <!-- Add moderator -->
            <div class="border-t border-terminal-border pt-2 mt-2">
              <div class="text-xs text-terminal-dim mb-1">add moderator:</div>
              <div class="flex items-center gap-2">
                <div class="flex items-center flex-1">
                  <span class="text-xs text-terminal-dim mr-1">u/</span>
                  <input
                    type="text"
                    bind:value={newModUsername}
                    placeholder="username"
                    class="flex-1 bg-terminal-bg border border-terminal-border px-2 py-0.5 text-xs text-terminal-fg placeholder:text-terminal-dim outline-none font-mono focus:border-accent-500 transition-colors"
                  />
                </div>
                <button
                  onclick={addModerator}
                  disabled={modAdding || !newModUsername.trim()}
                  class="text-xs border border-terminal-border px-2 py-0.5 text-accent-500 hover:border-accent-500 transition-colors disabled:opacity-50"
                >
                  {modAdding ? '[adding...]' : '[add]'}
                </button>
              </div>
            </div>

            {#if modMessage}
              <div class="text-xs {modMessage.type === 'success' ? 'text-green-500' : 'text-red-400'}">
                &gt; {modMessage.text}
              </div>
            {/if}
          </div>
          <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
            └─────────────────
          </div>
        </div>
      {/if}

    </div>
  </div>
{/if}
