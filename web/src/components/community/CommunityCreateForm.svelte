<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '../../lib/api';
  import { isAuthenticated, isLoading, initialize, subscribe } from '../../lib/auth';

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
  let visibility = $state('VISIBILITY_PUBLIC');
  let rules = $state<CommunityRule[]>([]);

  // Banner/Icon state
  let bannerUrl = $state('');
  let iconUrl = $state('');
  let bannerFile = $state<File | null>(null);
  let iconFile = $state<File | null>(null);
  let bannerPreview = $state('');
  let iconPreview = $state('');
  let bannerUploading = $state(false);
  let iconUploading = $state(false);
  let bannerError = $state<string | null>(null);
  let iconError = $state<string | null>(null);

  // UI state
  let submitting = $state(false);
  let error = $state<string | null>(null);
  let nameError = $state<string | null>(null);
  let authed = $state(isAuthenticated());
  let authLoading = $state(isLoading());

  const NAME_REGEX = /^[a-zA-Z0-9_]{3,21}$/;

  const visibilityOptions = [
    { value: 'VISIBILITY_PUBLIC', label: 'public', desc: 'anyone can view and join' },
    { value: 'VISIBILITY_RESTRICTED', label: 'restricted', desc: 'anyone can view, approval to join' },
    { value: 'VISIBILITY_PRIVATE', label: 'private', desc: 'invitation only, hidden from browse' },
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

  // ── Banner/Icon Upload ───────────────────────────────

  async function uploadImage(file: File): Promise<string | null> {
    try {
      // Step 1: Init upload
      const initRes = await api<{ mediaId: string; uploadUrl: string }>('/media/upload', {
        method: 'POST',
        body: JSON.stringify({
          filename: file.name,
          contentType: file.type,
          sizeBytes: String(file.size),
          mediaType: 'MEDIA_TYPE_IMAGE',
        }),
      });

      // Step 2: PUT file to presigned URL
      const uploadRes = await fetch(initRes.uploadUrl, {
        method: 'PUT',
        headers: { 'Content-Type': file.type },
        body: file,
      });

      if (!uploadRes.ok) {
        throw new Error(`Upload failed with status ${uploadRes.status}`);
      }

      // Step 3: Complete upload
      const completeRes = await api<{ url: string }>(`/media/${initRes.mediaId}/complete`, {
        method: 'POST',
      });

      return completeRes.url;
    } catch (e) {
      throw e;
    }
  }

  function handleBannerSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    if (!input.files?.[0]) return;

    const file = input.files[0];
    input.value = '';

    // Validate file type
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      bannerError = 'only jpg, png, webp allowed';
      return;
    }

    // Validate file size (max 5MB for banner)
    if (file.size > 5 * 1024 * 1024) {
      bannerError = 'max file size is 5MB';
      return;
    }

    bannerFile = file;
    bannerPreview = URL.createObjectURL(file);
    bannerError = null;
  }

  function handleIconSelect(e: Event) {
    const input = e.target as HTMLInputElement;
    if (!input.files?.[0]) return;

    const file = input.files[0];
    input.value = '';

    // Validate file type
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      iconError = 'only jpg, png, webp allowed';
      return;
    }

    // Validate file size (max 2MB for icon)
    if (file.size > 2 * 1024 * 1024) {
      iconError = 'max file size is 2MB';
      return;
    }

    iconFile = file;
    iconPreview = URL.createObjectURL(file);
    iconError = null;
  }

  function removeBanner() {
    bannerFile = null;
    bannerPreview = '';
    bannerUrl = '';
    bannerError = null;
  }

  function removeIcon() {
    iconFile = null;
    iconPreview = '';
    iconUrl = '';
    iconError = null;
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
      // Step 1: Upload banner if selected
      let uploadedBannerUrl = '';
      if (bannerFile) {
        bannerUploading = true;
        try {
          const url = await uploadImage(bannerFile);
          if (url) uploadedBannerUrl = url;
        } catch (e) {
          bannerError = e instanceof Error ? e.message : 'banner upload failed';
          submitting = false;
          bannerUploading = false;
          return;
        }
        bannerUploading = false;
      }

      // Step 2: Upload icon if selected
      let uploadedIconUrl = '';
      if (iconFile) {
        iconUploading = true;
        try {
          const url = await uploadImage(iconFile);
          if (url) uploadedIconUrl = url;
        } catch (e) {
          iconError = e instanceof Error ? e.message : 'icon upload failed';
          submitting = false;
          iconUploading = false;
          return;
        }
        iconUploading = false;
      }

      // Step 3: Create the community
      const data = await api<CreateResponse>('/communities', {
        method: 'POST',
        body: JSON.stringify({
          name,
          description,
          visibility,
        }),
      });

      // Step 4: PATCH to add rules, banner, icon if any
      const validRules = rules.filter(r => r.title.trim() !== '');
      const patchBody: Record<string, unknown> = {};
      
      if (validRules.length > 0) {
        patchBody.rules = validRules.map(r => ({
          title: r.title.trim(),
          description: r.description.trim(),
        }));
      }
      if (uploadedBannerUrl) {
        patchBody.bannerUrl = uploadedBannerUrl;
      }
      if (uploadedIconUrl) {
        patchBody.iconUrl = uploadedIconUrl;
      }

      if (Object.keys(patchBody).length > 0) {
        await api(`/communities/${data.community.name}`, {
          method: 'PATCH',
          body: JSON.stringify(patchBody),
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
    // Ensure auth is initialized before checking
    initialize();

    const unsub = subscribe(() => {
      authed = isAuthenticated();
      authLoading = isLoading();
    });

    return unsub;
  });

  // Watch for auth changes — only redirect AFTER loading completes
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
  <div class="flex items-center justify-center min-h-[60vh] px-4 py-8">
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

          <!-- Banner -->
          <div class="space-y-1">
            <span class="block text-xs text-terminal-dim">banner image:</span>
            {#if bannerPreview}
              <div class="relative">
                <img
                  src={bannerPreview}
                  alt="Banner preview"
                  class="w-full h-20 object-cover border border-terminal-border"
                />
                <button
                  type="button"
                  onclick={removeBanner}
                  class="absolute top-1 right-1 text-xs bg-terminal-bg border border-terminal-border px-1 text-red-500 hover:text-red-400 transition-colors"
                >
                  [x]
                </button>
              </div>
            {:else}
              <label class="block border border-dashed border-terminal-border p-3 text-center cursor-pointer hover:border-accent-500 transition-colors">
                <span class="text-xs text-terminal-dim">click to select banner (optional)</span>
                <input
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  onchange={handleBannerSelect}
                  class="hidden"
                />
              </label>
            {/if}
            <div class="text-xs text-terminal-dim">jpg, png, webp (max 5MB)</div>
            {#if bannerError}
              <div class="text-xs">
                <span class="text-red-500">&gt; error:</span>
                <span class="text-red-400"> {bannerError}</span>
              </div>
            {/if}
          </div>

          <!-- Icon -->
          <div class="space-y-1">
            <span class="block text-xs text-terminal-dim">community icon:</span>
            <div class="flex items-center gap-3">
              {#if iconPreview}
                <div class="relative">
                  <img
                    src={iconPreview}
                    alt="Icon preview"
                    class="w-14 h-14 object-cover border border-terminal-border rounded"
                  />
                  <button
                    type="button"
                    onclick={removeIcon}
                    class="absolute -top-1 -right-1 text-xs bg-terminal-bg border border-terminal-border px-1 text-red-500 hover:text-red-400 transition-colors"
                  >
                    [x]
                  </button>
                </div>
              {:else}
                <label class="w-14 h-14 border border-dashed border-terminal-border rounded flex items-center justify-center cursor-pointer hover:border-accent-500 transition-colors">
                  <span class="text-xs text-terminal-dim">+</span>
                  <input
                    type="file"
                    accept="image/jpeg,image/png,image/webp"
                    onchange={handleIconSelect}
                    class="hidden"
                  />
                </label>
              {/if}
              <div class="text-xs text-terminal-dim">
                <div>square image recommended</div>
                <div>jpg, png, webp (max 2MB)</div>
              </div>
            </div>
            {#if iconError}
              <div class="text-xs">
                <span class="text-red-500">&gt; error:</span>
                <span class="text-red-400"> {iconError}</span>
              </div>
            {/if}
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
              <span class="text-terminal-dim">[creating{bannerUploading ? ' (uploading banner...)' : iconUploading ? ' (uploading icon...)' : '...'}]</span>
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
