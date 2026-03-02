<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../lib/api';
  import { isAuthenticated, isLoading, subscribe } from '../lib/auth';

  type CommunityRule = {
    title: string;
    description: string;
  };

  type CreateResponse = {
    community: {
      communityId: string;
      name: string;
    };
  };

  // Form state
  let name = $state('');
  let description = $state('');
  let visibility = $state(1); // 1=PUBLIC, 2=RESTRICTED, 3=PRIVATE
  let rules = $state<CommunityRule[]>([]);

  // UI state
  let submitting = $state(false);
  let error = $state<string | null>(null);
  let nameError = $state<string | null>(null);
  let authed = $state(isAuthenticated());
  let authLoading = $state(isLoading());

  const NAME_REGEX = /^[a-zA-Z0-9_]{3,21}$/;

  const visibilityOptions = [
    { value: 1, label: 'public', desc: 'anyone can view and join' },
    { value: 2, label: 'restricted', desc: 'anyone can view, approval to join' },
    { value: 3, label: 'private', desc: 'invitation only, hidden from browse' },
  ];

  function validateName(value: string): string | null {
    if (value.length === 0) return null;
    if (value.length < 3) return 'min 3 characters';
    if (value.length > 21) return 'max 21 characters';
    if (!NAME_REGEX.test(value)) return 'alphanumeric and underscores only';
    return null;
  }

  function onNameInput() {
    nameError = validateName(name);
  }

  function addRule() {
    rules = [...rules, { title: '', description: '' }];
  }

  function removeRule(index: number) {
    rules = rules.filter((_, i) => i !== index);
  }

  function updateRuleTitle(index: number, value: string) {
    rules = rules.map((r, i) => i === index ? { ...r, title: value } : r);
  }

  function updateRuleDescription(index: number, value: string) {
    rules = rules.map((r, i) => i === index ? { ...r, description: value } : r);
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();

    // Validate
    const nameErr = validateName(name);
    if (nameErr) {
      nameError = nameErr;
      return;
    }
    if (!name) {
      nameError = 'name is required';
      return;
    }

    submitting = true;
    error = null;

    try {
      // Step 1: Create the community
      const data = await api<CreateResponse>('/communities', {
        method: 'POST',
        body: JSON.stringify({
          name,
          description,
          visibility,
        }),
      });

      // Step 2: If rules are entered, PATCH to add them
      const validRules = rules.filter(r => r.title.trim() !== '');
      if (validRules.length > 0) {
        await api(`/communities/${data.community.name}`, {
          method: 'PATCH',
          body: JSON.stringify({
            rules: validRules.map(r => ({
              title: r.title.trim(),
              description: r.description.trim(),
            })),
          }),
        });
      }

      // Redirect to the new community
      window.location.href = `/community/${data.community.name}`;
    } catch (e) {
      if (e instanceof ApiError) {
        error = e.message;
      } else {
        error = 'Failed to create community';
      }
    } finally {
      submitting = false;
    }
  }

  onMount(() => {
    const unsub = subscribe(() => {
      authed = isAuthenticated();
      authLoading = isLoading();
    });

    // Redirect if not authenticated
    if (!isLoading() && !isAuthenticated()) {
      window.location.href = '/login';
    }

    return unsub;
  });

  // Watch for auth changes — redirect if logged out
  $effect(() => {
    if (!authLoading && !authed) {
      window.location.href = '/login';
    }
  });
</script>

{#if authLoading}
  <div class="flex items-center justify-center min-h-[60vh] px-4">
    <span class="text-xs text-terminal-dim font-mono animate-pulse">[checking auth...]</span>
  </div>
{:else if !authed}
  <div class="flex items-center justify-center min-h-[60vh] px-4">
    <span class="text-xs text-terminal-dim font-mono">&gt; redirecting to login...</span>
  </div>
{:else}
  <div class="flex items-center justify-center min-h-[60vh] px-4">
    <div class="w-full max-w-lg">
      <!-- Terminal-style form box -->
      <div class="border border-terminal-border bg-terminal-surface font-mono">
        <!-- Title bar -->
        <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim">
          ┌─ create community
        </div>

        <!-- Form -->
        <form class="p-4 space-y-4" onsubmit={handleSubmit}>
          <!-- Name -->
          <div class="space-y-1">
            <label for="name" class="block text-xs text-terminal-dim">name:</label>
            <div class="flex items-center">
              <span class="text-xs text-terminal-dim mr-1">r/</span>
              <input
                id="name"
                type="text"
                bind:value={name}
                oninput={onNameInput}
                placeholder="community_name"
                maxlength="21"
                class="flex-1 bg-terminal-bg border border-terminal-border px-2 py-1 text-xs text-terminal-fg placeholder:text-terminal-dim outline-none font-mono focus:border-accent-500 transition-colors"
              />
            </div>
            <div class="text-xs text-terminal-dim">/^[a-zA-Z0-9_]&#123;3,21&#125;$/</div>
            {#if nameError}
              <div class="text-xs">
                <span class="text-red-500">&gt; error:</span>
                <span class="text-red-400"> {nameError}</span>
              </div>
            {/if}
          </div>

          <!-- Description -->
          <div class="space-y-1">
            <label for="description" class="block text-xs text-terminal-dim">description:</label>
            <textarea
              id="description"
              bind:value={description}
              placeholder="what is this community about?"
              rows="3"
              class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs text-terminal-fg placeholder:text-terminal-dim outline-none font-mono resize-y focus:border-accent-500 transition-colors"
            ></textarea>
            <div class="text-xs text-terminal-dim">markdown supported</div>
          </div>

          <!-- Visibility -->
          <div class="space-y-1">
            <span class="block text-xs text-terminal-dim">visibility:</span>
            <div class="space-y-1">
              {#each visibilityOptions as opt}
                <label class="flex items-start gap-2 cursor-pointer group">
                  <input
                    type="radio"
                    name="visibility"
                    value={opt.value}
                    checked={visibility === opt.value}
                    onchange={() => { visibility = opt.value; }}
                    class="mt-0.5 accent-accent-500"
                  />
                  <div>
                    <span class="text-xs text-terminal-fg group-hover:text-accent-500 transition-colors">{opt.label}</span>
                    <span class="text-xs text-terminal-dim ml-1">— {opt.desc}</span>
                  </div>
                </label>
              {/each}
            </div>
          </div>

          <!-- Rules -->
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs text-terminal-dim">rules:</span>
              <button
                type="button"
                onclick={addRule}
                class="text-xs text-accent-500 hover:text-accent-400 transition-colors"
              >
                [+ add rule]
              </button>
            </div>

            {#if rules.length === 0}
              <div class="text-xs text-terminal-dim italic px-1">no rules defined (optional)</div>
            {/if}

            {#each rules as rule, i}
              <div class="border border-terminal-border bg-terminal-bg p-2 space-y-1">
                <div class="flex items-center justify-between">
                  <span class="text-xs text-terminal-dim">rule {i + 1}:</span>
                  <button
                    type="button"
                    onclick={() => removeRule(i)}
                    class="text-xs text-red-500 hover:text-red-400 transition-colors"
                  >
                    [remove]
                  </button>
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
          </div>

          <!-- Error display -->
          {#if error}
            <div class="text-xs font-mono">
              <span class="text-red-500">&gt; error:</span>
              <span class="text-red-400"> {error}</span>
            </div>
          {/if}

          <!-- Submit button -->
          <button
            type="submit"
            disabled={submitting || !!nameError}
            class="w-full text-left px-3 py-1.5 text-xs font-mono border border-terminal-border bg-terminal-bg text-terminal-fg hover:text-accent-500 hover:border-accent-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {#if submitting}
              <span class="text-terminal-dim">[creating...]</span>
            {:else}
              <span class="text-accent-500">&gt;</span> create community<span class="animate-pulse">_</span>
            {/if}
          </button>
        </form>

        <!-- Bottom border -->
        <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
          └─────────────────
        </div>
      </div>
    </div>
  </div>
{/if}
