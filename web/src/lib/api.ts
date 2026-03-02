/**
 * API client with auth token injection and silent JWT refresh.
 *
 * Pattern: module-level token storage (not localStorage, not cookies)
 * with automatic 401 retry via refresh token.
 */

const API_BASE = '/api/v1';

// Access token: in-memory only (short-lived, 15 min)
// Refresh token: persisted in localStorage (long-lived, 7 days) to survive page reloads
let accessToken: string | null = null;
let refreshToken: string | null = (typeof window !== 'undefined' ? localStorage.getItem('refreshToken') : null);

// Singleton refresh promise for deduplication
let refreshPromise: Promise<boolean> | null = null;

/** Custom error with HTTP status code */
export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

/** Store access token in memory */
export function setAccessToken(token: string | null): void {
  accessToken = token;
}

/** Retrieve current access token */
export function getAccessToken(): string | null {
  return accessToken;
}

/** Store refresh token in memory AND localStorage (survives page reloads) */
export function setRefreshToken(token: string | null): void {
  refreshToken = token;
  if (typeof window !== 'undefined') {
    if (token) {
      localStorage.setItem('refreshToken', token);
    } else {
      localStorage.removeItem('refreshToken');
    }
  }
}

/** Retrieve current refresh token */
export function getRefreshToken(): string | null {
  return refreshToken;
}

/**
 * Attempt to refresh the access token using the stored refresh token.
 * Returns true on success, false on failure.
 */
async function refreshAccessToken(): Promise<boolean> {
  if (!refreshToken) return false;

  try {
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refreshToken }),
    });

    if (!res.ok) {
      accessToken = null;
      setRefreshToken(null);
      return false;
    }

    const data = await res.json();
    accessToken = data.accessToken ?? null;
    if (data.refreshToken) {
      setRefreshToken(data.refreshToken);
    }
    return true;
  } catch {
    accessToken = null;
    setRefreshToken(null);
    return false;
  }
}

/**
 * Fetch wrapper with auth token injection and silent 401 refresh.
 *
 * - Injects `Authorization: Bearer <token>` if token exists
 * - Sets `Content-Type: application/json` if body present
 * - On 401: attempts token refresh (deduplicated), retries once
 * - On error: throws ApiError with status and server message
 */
export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const url = `${API_BASE}${path}`;

  const headers = new Headers(options?.headers);

  if (accessToken) {
    headers.set('Authorization', `Bearer ${accessToken}`);
  }

  if (options?.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }

  const res = await fetch(url, { ...options, headers });

  // Handle 401 — attempt silent refresh
  if (res.status === 401 && refreshToken) {
    // Deduplicate concurrent refresh attempts
    if (!refreshPromise) {
      refreshPromise = refreshAccessToken().finally(() => {
        refreshPromise = null;
      });
    }

    const refreshed = await refreshPromise;

    if (refreshed) {
      // Retry the original request with new token
      const retryHeaders = new Headers(options?.headers);
      if (accessToken) {
        retryHeaders.set('Authorization', `Bearer ${accessToken}`);
      }
      if (options?.body && !retryHeaders.has('Content-Type')) {
        retryHeaders.set('Content-Type', 'application/json');
      }

      const retryRes = await fetch(url, { ...options, headers: retryHeaders });

      if (!retryRes.ok) {
        const body = await retryRes.json().catch(() => ({ message: retryRes.statusText }));
        throw new ApiError(retryRes.status, body.message ?? retryRes.statusText);
      }

      return retryRes.json() as Promise<T>;
    }

    // Refresh failed — throw the original 401
    throw new ApiError(401, 'Session expired. Please log in again.');
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({ message: res.statusText }));
    throw new ApiError(res.status, body.message ?? res.statusText);
  }

  return res.json() as Promise<T>;
}
