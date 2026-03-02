<script lang="ts">
  import AuthForm from './AuthForm.svelte';
  import { api, ApiError } from '../lib/api';
  import { loginWithTokens } from '../lib/auth';

  let code = $state('');
  let error = $state<string | null>(null);
  let loading = $state(false);

  // Read email from URL query param
  let email = $state('');

  if (typeof window !== 'undefined') {
    const params = new URLSearchParams(window.location.search);
    email = params.get('email') ?? '';
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    error = null;

    if (!code || code.length !== 6) {
      error = 'enter a 6-digit code';
      return;
    }

    if (!email) {
      error = 'missing email — go back to /register';
      return;
    }

    loading = true;
    try {
      const data = await api<{
        verified: boolean;
        accessToken: string;
        refreshToken: string;
        expiresAt: string;
        userId: string;
      }>('/auth/verify-otp', {
        method: 'POST',
        body: JSON.stringify({ email, code }),
      });

      if (data.verified && data.accessToken) {
        await loginWithTokens(data.accessToken, data.refreshToken, data.userId);
        window.location.href = '/';
      }
    } catch (err) {
      if (err instanceof ApiError) {
        error = err.message;
      } else {
        error = 'invalid or expired code';
      }
    } finally {
      loading = false;
    }
  }
</script>

<AuthForm title="verify" submitLabel="verify code" {error} {loading} onsubmit={handleSubmit}>
  <!-- Email display -->
  {#if email}
    <div class="text-xs font-mono text-terminal-dim">
      verifying: <span class="text-terminal-fg">{email}</span>
    </div>
  {/if}

  <!-- OTP code field -->
  <div class="space-y-1">
    <label class="block text-xs text-terminal-dim" for="verify-code">6-digit code:</label>
    <input
      id="verify-code"
      type="text"
      bind:value={code}
      maxlength="6"
      autocomplete="one-time-code"
      inputmode="numeric"
      pattern="[0-9]{6}"
      class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs font-mono text-terminal-fg outline-none focus:border-accent-500 transition-colors tracking-[0.3em] text-center"
      placeholder="000000"
    />
  </div>

  <!-- Hint -->
  <div class="text-xs text-terminal-dim pt-1">
    didn't receive a code? <a href="/register" class="text-accent-500 hover:text-accent-400 transition-colors">register again</a>
  </div>
</AuthForm>
