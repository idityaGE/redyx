<script lang="ts">
  import AuthForm from './AuthForm.svelte';
  import { api, ApiError } from '../lib/api';
  import { loginWithTokens } from '../lib/auth';

  let username = $state('');
  let error = $state<string | null>(null);
  let loading = $state(false);

  // Read OAuth code from URL query param
  let code = $state('');

  if (typeof window !== 'undefined') {
    const params = new URLSearchParams(window.location.search);
    code = params.get('code') ?? '';
  }

  function validateUsername(): boolean {
    if (!username || !/^[a-zA-Z0-9_]{3,20}$/.test(username)) {
      error = '3-20 chars, alphanumeric and underscore only';
      return false;
    }
    return true;
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    error = null;

    if (!validateUsername()) return;

    if (!code) {
      error = 'missing oauth code — try signing in again';
      return;
    }

    loading = true;
    try {
      const data = await api<{
        accessToken: string;
        refreshToken: string;
        expiresAt: string;
        userId: string;
        isNewUser: boolean;
      }>('/auth/google', {
        method: 'POST',
        body: JSON.stringify({ code, username }),
      });

      if (data.accessToken) {
        await loginWithTokens(data.accessToken, data.refreshToken, data.userId);
        window.location.href = '/';
      }
    } catch (err) {
      if (err instanceof ApiError) {
        error = err.message;
      } else {
        error = 'unexpected error occurred';
      }
    } finally {
      loading = false;
    }
  }
</script>

<AuthForm title="choose username" submitLabel="claim username" {error} {loading} onsubmit={handleSubmit}>
  <!-- Welcome message -->
  <div class="text-xs font-mono text-terminal-dim">
    welcome to <span class="text-accent-500">redyx</span> — choose your username
  </div>

  <!-- Username field -->
  <div class="space-y-1">
    <label class="block text-xs text-terminal-dim" for="choose-username">username:</label>
    <input
      id="choose-username"
      type="text"
      bind:value={username}
      autocomplete="username"
      class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs font-mono text-terminal-fg outline-none focus:border-accent-500 transition-colors"
      placeholder="cool_user"
    />
    <div class="text-xs text-terminal-dim">3-20 chars, letters, numbers, underscores</div>
  </div>
</AuthForm>
