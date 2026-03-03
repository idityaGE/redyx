<script lang="ts">
  import AuthForm from './AuthForm.svelte';
  import { login } from '../../lib/auth';
  import { ApiError } from '../../lib/api';

  let email = $state('');
  let password = $state('');
  let error = $state<string | null>(null);
  let needsVerify = $state(false);
  let loading = $state(false);

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    error = null;
    needsVerify = false;

    if (!email || !password) {
      error = 'email and password required';
      return;
    }

    loading = true;
    try {
      await login(email, password);
      window.location.href = '/';
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.message.toLowerCase().includes('not verified') || err.message.toLowerCase().includes('verify')) {
          needsVerify = true;
          error = 'account not verified';
        } else {
          error = err.message;
        }
      } else {
        error = 'invalid credentials';
      }
    } finally {
      loading = false;
    }
  }
</script>

<AuthForm title="login" submitLabel="log in" {error} {loading} onsubmit={handleSubmit}>
  <!-- Email field -->
  <div class="space-y-1">
    <label class="block text-xs text-terminal-dim" for="login-email">email:</label>
    <input
      id="login-email"
      type="text"
      bind:value={email}
      autocomplete="email"
      class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs font-mono text-terminal-fg outline-none focus:border-accent-500 transition-colors"
      placeholder="user@example.com"
    />
  </div>

  <!-- Password field -->
  <div class="space-y-1">
    <label class="block text-xs text-terminal-dim" for="login-password">password:</label>
    <input
      id="login-password"
      type="password"
      bind:value={password}
      autocomplete="current-password"
      class="w-full bg-terminal-bg border border-terminal-border px-2 py-1 text-xs font-mono text-terminal-fg outline-none focus:border-accent-500 transition-colors"
      placeholder="••••••••"
    />
  </div>

  <!-- Verify link if account not verified -->
  {#if needsVerify}
    <div class="text-xs font-mono">
      <a href="/verify?email={encodeURIComponent(email)}" class="text-accent-500 hover:text-accent-400 transition-colors">
        &gt; verify your account: /verify
      </a>
    </div>
  {/if}

  <!-- Links -->
  <div class="text-xs text-terminal-dim pt-1 space-y-0.5">
    <div>new user? <a href="/register" class="text-accent-500 hover:text-accent-400 transition-colors">/register</a></div>
    <div>forgot password? <a href="/reset-password" class="text-accent-500 hover:text-accent-400 transition-colors">/reset-password</a></div>
  </div>

  <!-- Google OAuth button -->
  <div class="pt-1 border-t border-terminal-border">
    <a
      href="/api/v1/auth/google/consent"
      class="block w-full text-left px-3 py-1.5 text-xs font-mono border border-terminal-border bg-terminal-bg text-terminal-dim hover:text-accent-500 hover:border-accent-500 transition-colors"
    >
      <span class="text-accent-500">&gt;</span> login with google<span class="animate-pulse">_</span>
    </a>
  </div>
</AuthForm>
