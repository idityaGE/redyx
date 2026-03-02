/**
 * Auth store — manages user authentication state with pub/sub reactivity.
 *
 * Uses plain TypeScript with subscriber pattern (not Svelte runes)
 * so it can be imported from both .svelte and .ts files safely.
 *
 * Svelte components use subscribe() for reactivity.
 */

import { api, setAccessToken, setRefreshToken, ApiError } from './api';

export type AuthUser = {
  userId: string;
  username: string;
  email?: string;
  avatarUrl?: string;
};

// Internal state
let user: AuthUser | null = null;
let loading = true;

// Pub/sub for reactivity
type Subscriber = () => void;
const subscribers = new Set<Subscriber>();

function notify(): void {
  subscribers.forEach((fn) => fn());
}

/** Subscribe to auth state changes. Returns unsubscribe function. */
export function subscribe(fn: Subscriber): () => void {
  subscribers.add(fn);
  return () => {
    subscribers.delete(fn);
  };
}

/** Get the current authenticated user, or null. */
export function getUser(): AuthUser | null {
  return user;
}

/** Check if user is currently authenticated. */
export function isAuthenticated(): boolean {
  return user !== null;
}

/** Check if initial auth check is still in progress. */
export function isLoading(): boolean {
  return loading;
}

/**
 * Initialize auth state on app start.
 *
 * Attempts to refresh tokens (if stored). On success, fetches user profile.
 * On failure, sets user to null (anonymous). Always sets loading=false.
 */
export async function initialize(): Promise<void> {
  if (!loading) return; // already initialized

  try {
    // Try refreshing the session — server may have a valid refresh cookie
    const data = await api<{ accessToken: string; refreshToken: string; userId: string }>('/auth/refresh', {
      method: 'POST',
      body: JSON.stringify({}),
    });

    setAccessToken(data.accessToken);
    setRefreshToken(data.refreshToken);

    // Fetch user profile
    const profile = await api<{ userId: string; username: string; email?: string; avatarUrl?: string }>(
      `/users/${data.userId}`
    );

    user = {
      userId: profile.userId,
      username: profile.username,
      email: profile.email,
      avatarUrl: profile.avatarUrl,
    };
  } catch {
    // Not authenticated — that's fine, anonymous access is allowed
    user = null;
  } finally {
    loading = false;
    notify();
  }
}

/**
 * Log in with email and password.
 */
export async function login(email: string, password: string): Promise<void> {
  const data = await api<{
    accessToken: string;
    refreshToken: string;
    userId: string;
  }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });

  setAccessToken(data.accessToken);
  setRefreshToken(data.refreshToken);

  // Fetch user profile
  const profile = await api<{ userId: string; username: string; email?: string; avatarUrl?: string }>(
    `/users/${data.userId}`
  );

  user = {
    userId: profile.userId,
    username: profile.username,
    email: profile.email,
    avatarUrl: profile.avatarUrl,
  };

  loading = false;
  notify();
}

/**
 * Set auth state from external token source (OAuth callback, OTP verification).
 */
export async function loginWithTokens(
  accessTokenValue: string,
  refreshTokenValue: string,
  userId: string
): Promise<void> {
  setAccessToken(accessTokenValue);
  setRefreshToken(refreshTokenValue);

  // Fetch user profile
  const profile = await api<{ userId: string; username: string; email?: string; avatarUrl?: string }>(
    `/users/${userId}`
  );

  user = {
    userId: profile.userId,
    username: profile.username,
    email: profile.email,
    avatarUrl: profile.avatarUrl,
  };

  loading = false;
  notify();
}

/**
 * Log out — clears tokens and user state.
 */
export async function logout(): Promise<void> {
  try {
    await api('/auth/logout', { method: 'POST' });
  } catch {
    // Best-effort server logout — clear local state regardless
  }

  setAccessToken(null);
  setRefreshToken(null);
  user = null;
  loading = false;
  notify();
}
