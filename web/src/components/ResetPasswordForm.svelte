<script lang="ts">
  import AuthForm from './AuthForm.svelte';
  import { api, ApiError } from '../lib/api';

  let email = $state('');
  let error = $state<string | null>(null);
  let success = $state(false);
  let loading = $state(false);

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    error = null;
    success = false;

    if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      error = 'valid email required';
      return;
    }

    loading = true;
    try {
      await api<{ completed: boolean }>('/auth/reset-password', {
        method: 'POST',
        body: JSON.stringify({ email }),
      });

      success = true;
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
          ┌─ reset password
        </div>
        <div class="p-4 space-y-3">
          <div class="text-xs font-mono">
            <span class="text-accent-500">&gt;</span> reset link sent to <span class="text-terminal-fg">{email}</span>
          </div>
          <div class="text-xs text-terminal-dim">
            check your email and follow the link, or enter the token below:
          </div>
          <a
            href="/reset-complete?email={encodeURIComponent(email)}"
            class="block w-full text-left px-3 py-1.5 text-xs font-mono border border-terminal-border bg-terminal-bg text-terminal-fg hover:text-accent-500 hover:border-accent-500 transition-colors"
          >
            <span class="text-accent-500">&gt;</span> continue to /reset-complete<span class="animate-pulse">_</span>
          </a>
        </div>
        <div class="px-3 py-1 border-t border-terminal-border text-xs text-terminal-dim">
          └─────────────────
        </div>
      </div>
    </div>
  </div>
{:else}
  <AuthForm title="reset password" submitLabel="send reset link" {error} {loading} onsubmit={handleSubmit}>
    <!-- Email field -->
    <div class="space-y-1">
      <label class="block text-xs text-terminal-dim" for="reset-email">email:</label>
      <input
        id="reset-email"
        type="text"
        bind:value={email}
        autocomplete="email"
        class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs font-mono text-terminal-fg outline-none focus:border-accent-500 transition-colors"
        placeholder="user@example.com"
      />
    </div>

    <!-- Links -->
    <div class="text-xs text-terminal-dim pt-1">
      remember your password? <a href="/login" class="text-accent-500 hover:text-accent-400 transition-colors">/login</a>
    </div>
  </AuthForm>
{/if}
