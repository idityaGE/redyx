/**
 * Auth store — manages user authentication state with pub/sub reactivity.
 *
 * Uses plain TypeScript with subscriber pattern (not Svelte runes)
 * so it can be imported from both .svelte and .ts files safely.
 *
 * Svelte components use subscribe() for reactivity.
 */

import { api, setAccessToken, setRefreshToken, getRefreshToken } from './api';

export type AuthUser = {
  userId: string;
  username: string;
  email?: string;
  avatarUrl?: string;
};

/**
 * Decode the payload of a JWT without verification (client-side only).
 * Used to extract userId/username from access tokens when the API response
 * doesn't include them (e.g., VerifyOTPResponse has no user_id field).
 */
function decodeJwtPayload(token: string): { uid?: string; username?: string } {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return {};
    const payload = JSON.parse(atob(parts[1].replace(/-/g, '+').replace(/_/g, '/')));
    return payload;
  } catch {
    return {};
  }
}

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
    const storedRefresh = getRefreshToken();
    if (!storedRefresh) {
      // No refresh token stored — user is anonymous, skip API call
      user = null;
      loading = false;
      notify();
      return;
    }

    // Try refreshing the session using the stored refresh token
    const data = await api<{ accessToken: string; refreshToken: string }>('/auth/refresh', {
      method: 'POST',
      body: JSON.stringify({ refreshToken: storedRefresh }),
    });

    setAccessToken(data.accessToken);
    setRefreshToken(data.refreshToken);

    // Decode username from the new access token (route is /users/{username}, not /users/{uuid})
    const claims = decodeJwtPayload(data.accessToken);
    if (claims.username) {
      // Fetch user profile (response is { user: { ... } } per GetProfileResponse proto)
      const res = await api<{ user: { userId: string; username: string; email?: string; avatarUrl?: string } }>(
        `/users/${claims.username}`
      );

      user = {
        userId: res.user.userId,
        username: res.user.username,
        email: res.user.email,
        avatarUrl: res.user.avatarUrl,
      };
    }
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

  // Decode JWT for username (route is /users/{username}, not /users/{uuid})
  const claims = decodeJwtPayload(data.accessToken);
  const username = claims.username;

  if (username) {
    // Fetch user profile (response is { user: { ... } } per GetProfileResponse proto)
    const res = await api<{ user: { userId: string; username: string; email?: string; avatarUrl?: string } }>(
      `/users/${username}`
    );

    user = {
      userId: res.user.userId,
      username: res.user.username,
      email: res.user.email,
      avatarUrl: res.user.avatarUrl,
    };
  } else {
    // Fallback: use data from login response
    user = {
      userId: data.userId,
      username: 'user',
    };
  }

  loading = false;
  notify();
}

/**
 * Set auth state from external token source (OAuth callback, OTP verification).
 */
export async function loginWithTokens(
  accessTokenValue: string,
  refreshTokenValue: string,
  userId?: string
): Promise<void> {
  setAccessToken(accessTokenValue);
  setRefreshToken(refreshTokenValue);

  // Decode JWT for username (route is /users/{username}, not /users/{uuid})
  const claims = decodeJwtPayload(accessTokenValue);
  const resolvedUsername = claims.username;
  const resolvedUserId = userId ?? claims.uid;

  if (resolvedUsername) {
    try {
      // Fetch user profile by username (response is { user: { ... } } per GetProfileResponse proto)
      const res = await api<{ user: { userId: string; username: string; email?: string; avatarUrl?: string } }>(
        `/users/${resolvedUsername}`
      );

      user = {
        userId: res.user.userId,
        username: res.user.username,
        email: res.user.email,
        avatarUrl: res.user.avatarUrl,
      };
    } catch {
      // Profile fetch failed — still logged in with tokens, use JWT claims
      user = {
        userId: resolvedUserId ?? '',
        username: resolvedUsername,
      };
    }
  } else if (resolvedUserId) {
    // No username in JWT (shouldn't happen) — fallback
    user = {
      userId: resolvedUserId,
      username: 'user',
    };
  }

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
