<script lang="ts">
  import AuthForm from './AuthForm.svelte';
  import { api, ApiError } from '../../lib/api';

  let email = $state('');
  let username = $state('');
  let password = $state('');
  let error = $state<string | null>(null);
  let fieldErrors = $state<{ email?: string; username?: string; password?: string }>({});
  let loading = $state(false);

  function validate(): boolean {
    fieldErrors = {};
    let valid = true;

    if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      fieldErrors.email = 'valid email required';
      valid = false;
    }

    if (!username || !/^[a-zA-Z0-9_]{3,20}$/.test(username)) {
      fieldErrors.username = '3-20 chars, alphanumeric and underscore only';
      valid = false;
    }

    if (!password || password.length < 8) {
      fieldErrors.password = 'minimum 8 characters';
      valid = false;
    }

    return valid;
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    error = null;
    fieldErrors = {};

    if (!validate()) return;

    loading = true;
    try {
      const data = await api<{ userId: string; requiresVerification: boolean }>('/auth/register', {
        method: 'POST',
        body: JSON.stringify({ email, username, password }),
      });

      if (data.requiresVerification) {
        window.location.href = `/verify?email=${encodeURIComponent(email)}`;
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

<AuthForm title="register" submitLabel="create account" {error} {loading} onsubmit={handleSubmit}>
  <!-- Email field -->
  <div class="space-y-1">
    <label class="block text-xs text-terminal-dim" for="reg-email">email:</label>
    <input
      id="reg-email"
      type="text"
      bind:value={email}
      autocomplete="email"
      class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs font-mono text-terminal-fg outline-none focus:border-accent-500 transition-colors"
      placeholder="user@example.com"
    />
    {#if fieldErrors.email}
      <div class="text-xs font-mono">
        <span class="text-red-500">&gt; error:</span>
        <span class="text-red-400"> {fieldErrors.email}</span>
      </div>
    {/if}
  </div>

  <!-- Username field -->
  <div class="space-y-1">
    <label class="block text-xs text-terminal-dim" for="reg-username">username:</label>
    <input
      id="reg-username"
      type="text"
      bind:value={username}
      autocomplete="username"
      class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs font-mono text-terminal-fg outline-none focus:border-accent-500 transition-colors"
      placeholder="cool_user"
    />
    {#if fieldErrors.username}
      <div class="text-xs font-mono">
        <span class="text-red-500">&gt; error:</span>
        <span class="text-red-400"> {fieldErrors.username}</span>
      </div>
    {/if}
  </div>

  <!-- Password field -->
  <div class="space-y-1">
    <label class="block text-xs text-terminal-dim" for="reg-password">password:</label>
    <input
      id="reg-password"
      type="password"
      bind:value={password}
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

  <!-- Links -->
  <div class="text-xs text-terminal-dim pt-1">
    already have an account? <a href="/login" class="text-accent-500 hover:text-accent-400 transition-colors">/login</a>
  </div>
</AuthForm>
