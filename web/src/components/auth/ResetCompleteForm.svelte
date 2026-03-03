<script lang="ts">
  import AuthForm from './AuthForm.svelte';
  import { api, ApiError } from '../../lib/api';

  let token = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let error = $state<string | null>(null);
  let fieldErrors = $state<{ password?: string; confirm?: string }>({});
  let success = $state(false);
  let loading = $state(false);

  // Read email and token from URL query params
  let email = $state('');

  if (typeof window !== 'undefined') {
    const params = new URLSearchParams(window.location.search);
    email = params.get('email') ?? '';
    const urlToken = params.get('token');
    if (urlToken) {
      token = urlToken;
    }
  }

  function validate(): boolean {
    fieldErrors = {};
    let valid = true;

    if (!newPassword || newPassword.length < 8) {
      fieldErrors.password = 'minimum 8 characters';
      valid = false;
    }

    if (newPassword !== confirmPassword) {
      fieldErrors.confirm = 'passwords do not match';
      valid = false;
    }

    return valid;
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    error = null;
    fieldErrors = {};

    if (!token) {
      error = 'reset token required';
      return;
    }

    if (!email) {
      error = 'missing email — go back to /reset-password';
      return;
    }

    if (!validate()) return;

    loading = true;
    try {
      const data = await api<{ completed: boolean }>('/auth/reset-password', {
        method: 'POST',
        body: JSON.stringify({ email, token, newPassword }),
      });

      if (data.completed) {
        success = true;
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

{#if success}
  <div class="flex items-center justify-center min-h-[60vh] px-4">
    <div class="w-full max-w-sm">
      <div class="border border-terminal-border bg-terminal-surface font-mono">
        <div class="px-3 py-1.5 border-b border-terminal-border text-xs text-terminal-dim">
          ┌─ password reset
        </div>
        <div class="p-4 space-y-3">
          <div class="text-xs font-mono">
            <span class="text-accent-500">&gt;</span> password reset successful
          </div>
          <a
            href="/login"
            class="block w-full text-left px-3 py-1.5 text-xs font-mono border border-terminal-border bg-terminal-bg text-terminal-fg hover:text-accent-500 hover:border-accent-500 transition-colors"
          >
            <span class="text-accent-500">&gt;</span> continue to /login<span class="animate-pulse">_</span>
          </a>
        </div>
        <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
          └─────────────────
        </div>
      </div>
    </div>
  </div>
{:else}
  <AuthForm title="complete reset" submitLabel="reset password" {error} {loading} onsubmit={handleSubmit}>
    <!-- Email display -->
    {#if email}
      <div class="text-xs font-mono text-terminal-dim">
        resetting password for: <span class="text-terminal-fg">{email}</span>
      </div>
    {/if}

    <!-- Token field (shown only if not in URL) -->
    {#if !token}
      <div class="space-y-1">
        <label class="block text-xs text-terminal-dim" for="reset-token">reset token:</label>
        <input
          id="reset-token"
          type="text"
          bind:value={token}
          class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs font-mono text-terminal-fg outline-none focus:border-accent-500 transition-colors"
          placeholder="paste token from email"
        />
      </div>
    {/if}

    <!-- New password field -->
    <div class="space-y-1">
      <label class="block text-xs text-terminal-dim" for="reset-new-pw">new password:</label>
      <input
        id="reset-new-pw"
        type="password"
        bind:value={newPassword}
        autocomplete="new-password"
        class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs font-mono text-terminal-fg outline-none focus:border-accent-500 transition-colors"
        placeholder="••••••••"
      />
      {#if fieldErrors.password}
        <div class="text-xs font-mono">
          <span class="text-red-500">&gt; error:</span>
          <span class="text-red-400"> {fieldErrors.password}</span>
        </div>
      {/if}
    </div>

    <!-- Confirm password field -->
    <div class="space-y-1">
      <label class="block text-xs text-terminal-dim" for="reset-confirm-pw">confirm password:</label>
      <input
        id="reset-confirm-pw"
        type="password"
        bind:value={confirmPassword}
        autocomplete="new-password"
        class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs font-mono text-terminal-fg outline-none focus:border-accent-500 transition-colors"
        placeholder="••••••••"
      />
      {#if fieldErrors.confirm}
        <div class="text-xs font-mono">
          <span class="text-red-500">&gt; error:</span>
          <span class="text-red-400"> {fieldErrors.confirm}</span>
        </div>
      {/if}
    </div>
  </AuthForm>
{/if}
