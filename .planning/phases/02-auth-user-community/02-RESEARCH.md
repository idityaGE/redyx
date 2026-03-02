# Phase 2: Auth + User + Community (Full Stack) - Research

**Researched:** 2026-03-03
**Domain:** Authentication, user management, community management, rate limiting — Go gRPC services + Astro/Svelte frontend
**Confidence:** HIGH

## Summary

Phase 2 builds three backend gRPC services (auth, user, community) plus a rate-limiting module, with corresponding frontend pages for all auth flows, profile views, and community management. All proto definitions already exist with HTTP annotations for Envoy transcoding. The platform libraries (grpcserver, middleware, database, redis, config, errors, pagination) are ready to use. The frontend has an established Astro SSR + Svelte 5 runes pattern with a terminal aesthetic design system.

The Go ecosystem provides mature, well-maintained libraries for every auth concern: `golang.org/x/crypto/argon2` for password hashing, `github.com/golang-jwt/jwt/v5` for JWT tokens, `golang.org/x/oauth2` for Google OAuth, and the existing `go-redis/v9` for OTP storage, refresh token blacklisting, rate limiting, and community metadata caching. No exotic dependencies are needed. The primary complexity is in wiring: auth interceptor placement in the gRPC middleware chain, Envoy route configuration for multiple backend services, database-per-service with separate PostgreSQL databases, and establishing the frontend API client + auth state management pattern that all future phases will build on.

**Primary recommendation:** Three separate Go services (auth-service, user-service, community-service) each with their own PostgreSQL database, sharing a single Redis instance with logical DB separation. Application-level rate limiting via a Redis token bucket called from a gRPC unary interceptor (not a separate service). JWT access tokens in memory + refresh tokens in httpOnly cookies. Frontend API client as a thin fetch wrapper in `web/src/lib/api.ts` with Svelte 5 runes-based auth store.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Terminal prompt style for registration and login pages — centered minimal form with monospace labels, ASCII box border, blinking cursor feel. Matches the established terminal aesthetic from Phase 1
- Separate `/verify` page for OTP verification after registration — distinct URL so users can return if they close the browser. Not an in-place transition
- Inline terminal-style error messages below each field — red/orange monospace text styled like stderr output (e.g. `> error: invalid credentials`). No toast notifications or banner errors on auth pages
- Dedicated `/choose-username` page after Google OAuth callback for new users — clean separation from the OAuth flow. Redirect to home after username is set
- Password reset follows the two-step proto flow (email to initiate, token + new password to complete) — presented as two separate terminal-style pages
- Tabbed sections for profile content: Posts | Comments | About — each tab shows a paginated list. Tab navigation styled like terminal multiplexer tabs
- Compact status line for profile header: `u/username | 1,234 karma | joined 2026-01-15` — single line, dense, no large hero section
- Avatar displayed as small square thumbnail (32-48px) with ASCII box-drawing border — falls back to first letter of username if no avatar uploaded
- Inline editing on own profile page — click-to-edit for display name, bio, and avatar. No separate /settings route for profile editing
- Bio limited to 500 chars per USER-04. Markdown rendering in the About tab
- Right sidebar info panel on community detail page — description, rules list, member count, mod list, join/leave button. Main content area reserved for post feed (empty until Phase 3). Sidebar uses box-drawing borders matching terminal aesthetic
- Single-page form for community creation — all fields on one page: name, description, visibility (public/restricted/private), rules. No multi-step wizard. Fast and simple
- Browse page at `/communities` with sorting by member count, creation date, or activity — communities displayed in a list/table format fitting the information-dense terminal style
- Dedicated settings page per community at `/community/{name}/settings` — sections for description editing, rules management (add/remove/reorder), visibility toggle, moderator assignment/revocation. Only accessible to community owner and moderators
- Header: `u/username` replaces `[anonymous]` when authenticated — clicking username opens a dropdown menu with Profile, Settings, Logout options. Notification diamond gets an unread count badge (placeholder until Phase 5)
- Silent JWT refresh — automatically use refresh token to get new access token when access token expires. User never sees interruption. Only redirect to `/login` when refresh token is also expired (7 days)
- Full read access for anonymous users — can browse communities, read posts, view profiles without authentication. Voting, posting, commenting, joining communities, and community creation require login. Gentle terminal-style prompt when attempting restricted actions (e.g. `> login required: /login`)
- Sidebar keeps static shortcuts (Home, Popular, All, Saved) at top + adds a "My Communities" section below with the user's joined communities when authenticated. Tree-drawing characters (├── , └──) maintained. Anonymous users see shortcuts only

### Claude's Discretion
- Loading skeleton/spinner design within the terminal aesthetic for auth operations
- Exact form field validation timing (on-blur vs on-submit)
- Password strength indicator design (if any)
- Exact spacing and padding values within auth forms
- Mobile responsiveness details for auth pages (bottom tab bar hides on auth pages or stays)
- Account deletion confirmation flow (how many steps, what warnings)
- Community rules editor UX details (inline text fields vs textarea)
- Rate limit error display (429 response presentation to user)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| AUTH-01 | Register with email, username, password (argon2id) | argon2.IDKey from golang.org/x/crypto/argon2 with RFC 9106 params; password hash storage pattern in PostgreSQL |
| AUTH-02 | 6-digit OTP via email, 5min TTL, Redis stored | Redis SETEX with `otp:{email}` key pattern; crypto/rand for OTP generation |
| AUTH-03 | Google OAuth registration with username choice | golang.org/x/oauth2 + google provider; `/choose-username` page for new users |
| AUTH-04 | Login with email/password or Google OAuth | Password verification via argon2.IDKey comparison; OAuth code exchange flow |
| AUTH-05 | JWT access token (15min) + refresh token (7 days) | golang-jwt/jwt/v5 with HS256 signing; refresh tokens as opaque UUIDs stored in PostgreSQL |
| AUTH-06 | Logout invalidates refresh token | Delete refresh token from PostgreSQL + add to Redis blacklist for remaining TTL |
| AUTH-07 | Password reset via email link with token | Crypto/rand token stored in Redis with 1hr TTL; two-step proto flow |
| AUTH-08 | Email/auth method never exposed via API | User proto message excludes email field; auth_method stored in auth DB only, not user DB |
| USER-01 | Public profile: username, karma, cake day | User table with karma (integer) and created_at; GetProfile RPC returns User message |
| USER-02 | Paginated post/comment history | GetUserPosts/GetUserComments RPCs with cursor-based pagination; cross-service calls to post/comment services (stub until Phase 3/4) |
| USER-03 | Karma from total upvotes on posts/comments | Karma column in users table; updated asynchronously (Kafka in Phase 3); starts at 0 |
| USER-04 | Update display name, bio (500 chars), avatar | UpdateProfile RPC with field validation; bio length check in service layer |
| USER-05 | Delete account, wipe PII, replace with [deleted] | DeleteAccount RPC; mark account as deleted, clear PII, broadcast deletion event for future phases |
| COMM-01 | Create community with unique immutable name | Name validation regex `^[a-zA-Z0-9_]{3,21}$`; UNIQUE constraint in PostgreSQL |
| COMM-02 | Community description (markdown), rules, banner, icon | Community table with description, jsonb rules array, banner_url, icon_url |
| COMM-03 | Visibility: public/restricted/private | Enum column in communities table; access control in service layer |
| COMM-04 | Join/leave communities | community_members table with user_id, community_id; member_count denormalized |
| COMM-05 | Creator auto-assigned owner + moderator | On CreateCommunity: insert member with role='owner' + role='moderator' |
| COMM-06 | Owner assigns/revokes moderator roles | AssignModerator/RevokeModerator RPCs; ownership check before operation |
| COMM-07 | Community metadata cached in Redis (1hr TTL) | Redis GET/SET with `community:{name}` key; invalidate on UpdateCommunity |
| RATE-01 | Per-user rate limits via Redis token bucket | Token bucket algorithm in Redis using Lua script for atomicity |
| RATE-02 | Tiered limits: anonymous (10/min), auth (100/min), trusted (300/min) | Tier determined from JWT claims or absence thereof; config-driven limits |
| RATE-03 | Action-specific limits: 5 posts/hr, 30 comments/hr, 60 votes/min, 1 community/day | Separate token buckets per action with different window sizes |
| RATE-04 | 429 with Retry-After header | gRPC interceptor returns codes.ResourceExhausted; Envoy transcodes to HTTP 429; Retry-After via gRPC trailer metadata |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| golang.org/x/crypto | v0.48.0 | argon2id password hashing + crypto/rand | Go's official extended crypto library; argon2 subpackage implements RFC 9106 |
| github.com/golang-jwt/jwt/v5 | v5.3.1 | JWT access token creation and validation | De facto Go JWT library; 13K+ importers; supports HS256/RS256/ES256 with RegisteredClaims |
| golang.org/x/oauth2 | v0.35.0 | Google OAuth2 code exchange | Go's official OAuth2 library; built-in Google endpoint configuration |
| github.com/jackc/pgx/v5 | v5.8.0 (existing) | PostgreSQL raw SQL queries | Already in go.mod; pgxpool for connection pooling |
| github.com/redis/go-redis/v9 | v9.18.0 (existing) | OTP storage, refresh token blacklist, rate limiting, community cache | Already in go.mod; Lua script support for atomic rate limiting |
| google.golang.org/grpc | v1.79.1 (existing) | gRPC server + interceptors | Already in go.mod; ChainUnaryInterceptor for auth middleware |
| github.com/google/uuid | latest | UUID generation for user IDs, refresh tokens | Standard Go UUID library; v4 random UUIDs |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| crypto/rand (stdlib) | Go 1.25 | Cryptographic random for OTP codes, salt generation, reset tokens | All random value generation — never use math/rand |
| encoding/base64 (stdlib) | Go 1.25 | Encoding reset tokens for URL-safe transmission | Password reset email links |
| net/smtp or stub | Go 1.25 | Email sending for OTP and password reset | Development: log to stdout. Production: real SMTP |
| regexp (stdlib) | Go 1.25 | Community name validation, input sanitization | `^[a-zA-Z0-9_]{3,21}$` for community names |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| golang-jwt/jwt (HS256) | PASETO v4 | PASETO is more opinionated/secure by design but less ecosystem support in Go; JWT is sufficient with HS256 + proper validation. HS256 with a strong secret is fine for single-issuer systems |
| Application-level rate limiting | Envoy external rate limit service | Envoy ratelimit service (lyft/ratelimit) adds another container + gRPC sidecar. Application-level with Redis is simpler, already have Redis, and gives finer control over action-specific limits |
| Separate rate-limit gRPC service | Rate limit as interceptor | The proto defines RateLimitService with CheckRateLimit RPC, but this is better implemented as a gRPC interceptor that directly checks Redis rather than a separate network hop. The proto can still exist for admin/debugging queries |
| httpOnly cookies for access token | localStorage for access token | httpOnly cookies prevent XSS token theft but complicate the Envoy/gRPC flow since Envoy transcoder reads Authorization header. Better: access token in memory (JS variable), refresh token in httpOnly cookie |

**Installation:**
```bash
# Go dependencies (from project root)
go get golang.org/x/crypto@v0.48.0
go get github.com/golang-jwt/jwt/v5@v5.3.1
go get golang.org/x/oauth2@v0.35.0
go get github.com/google/uuid
```

## Architecture Patterns

### Recommended Project Structure
```
cmd/
├── auth/main.go          # Auth service entry point
├── user/main.go          # User service entry point
├── community/main.go     # Community service entry point
internal/
├── auth/
│   ├── server.go         # AuthServiceServer implementation
│   ├── hasher.go         # argon2id password hashing helpers
│   ├── jwt.go            # JWT token creation/validation
│   ├── otp.go            # OTP generation and Redis storage
│   └── oauth.go          # Google OAuth code exchange
├── user/
│   ├── server.go         # UserServiceServer implementation
│   └── repo.go           # PostgreSQL queries (optional split)
├── community/
│   ├── server.go         # CommunityServiceServer implementation
│   ├── cache.go          # Redis community metadata caching
│   └── repo.go           # PostgreSQL queries (optional split)
├── platform/
│   ├── auth/             # NEW: shared auth interceptor + context helpers
│   │   ├── interceptor.go   # JWT validation interceptor
│   │   └── context.go       # UserID/claims from context
│   ├── ratelimit/        # NEW: rate limiting interceptor
│   │   ├── interceptor.go   # gRPC interceptor
│   │   └── limiter.go       # Redis token bucket implementation
│   ├── config/config.go  # Extend with JWT/OAuth config fields
│   ├── ... (existing)
migrations/
├── auth/                 # Auth service database migrations
│   ├── 001_users.up.sql
│   └── 001_users.down.sql
├── user/                 # User service database migrations
│   ├── 001_profiles.up.sql
│   └── 001_profiles.down.sql
├── community/            # Community service database migrations
│   ├── 001_communities.up.sql
│   └── 001_communities.down.sql
web/src/
├── lib/
│   ├── api.ts            # NEW: fetch wrapper with auth token injection
│   └── auth.ts           # NEW: Svelte 5 auth store (runes-based)
├── pages/
│   ├── register.astro    # Registration page
│   ├── login.astro       # Login page
│   ├── verify.astro      # OTP verification page
│   ├── reset-password.astro    # Password reset initiation
│   ├── reset-complete.astro    # Password reset completion
│   ├── choose-username.astro   # Post-OAuth username selection
│   ├── user/[username].astro   # User profile page
│   ├── communities/
│   │   ├── index.astro         # Browse communities
│   │   └── create.astro        # Create community form
│   ├── community/
│   │   ├── [name].astro        # Community detail page
│   │   └── [name]/settings.astro  # Community settings (mods only)
├── components/
│   ├── AuthForm.svelte         # Reusable terminal-style form wrapper
│   ├── Header.svelte           # NEW: Replace Astro Header with Svelte for auth-aware dynamic content
│   ├── Sidebar.svelte          # NEW: Replace Astro Sidebar with Svelte for dynamic communities
│   ├── ProfileTabs.svelte      # Tabbed profile content
│   ├── CommunityCard.svelte    # Community list item
│   ├── CommunitySettings.svelte # Settings form component
│   └── UserDropdown.svelte     # Header user dropdown menu
```

### Pattern 1: Auth Interceptor Placement in Middleware Chain

**What:** A unary interceptor that extracts JWT from gRPC metadata (`authorization` header), validates it, and injects user claims into the request context. Must handle both authenticated and anonymous requests.

**When to use:** Every gRPC service (auth, user, community) includes this interceptor in its chain.

**Example:**
```go
// internal/platform/auth/interceptor.go
package auth

import (
    "context"
    "strings"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
)

// Skip list — RPCs that don't require auth (anonymous allowed)
var publicMethods = map[string]bool{
    "/redyx.auth.v1.AuthService/Register":      true,
    "/redyx.auth.v1.AuthService/Login":          true,
    "/redyx.auth.v1.AuthService/VerifyOTP":      true,
    "/redyx.auth.v1.AuthService/ResetPassword":  true,
    "/redyx.auth.v1.AuthService/GoogleOAuth":    true,
    "/redyx.auth.v1.AuthService/RefreshToken":   true,
    "/redyx.user.v1.UserService/GetProfile":     true,
    "/redyx.user.v1.UserService/GetUserPosts":   true,
    "/redyx.user.v1.UserService/GetUserComments": true,
    "/redyx.community.v1.CommunityService/GetCommunity":    true,
    "/redyx.community.v1.CommunityService/ListCommunities": true,
    "/redyx.community.v1.CommunityService/ListMembers":     true,
}

// UnaryInterceptor validates JWT and injects user claims into context.
// For public methods: if no token, proceeds with anonymous context.
// For protected methods: if no valid token, returns Unauthenticated.
func UnaryInterceptor(validator *JWTValidator) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        token := extractToken(ctx)

        if token != "" {
            claims, err := validator.Validate(token)
            if err == nil {
                ctx = WithClaims(ctx, claims)
            }
            // Invalid token on public method: proceed anonymously
            // Invalid token on protected method: fall through to check below
        }

        // Protected method without valid auth → reject
        if !publicMethods[info.FullMethod] && ClaimsFromContext(ctx) == nil {
            return nil, status.Error(codes.Unauthenticated, "authentication required")
        }

        return handler(ctx, req)
    }
}

func extractToken(ctx context.Context) string {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return ""
    }
    vals := md.Get("authorization")
    if len(vals) == 0 {
        return ""
    }
    // "Bearer <token>"
    parts := strings.SplitN(vals[0], " ", 2)
    if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
        return ""
    }
    return parts[1]
}
```

**Middleware chain order (updated):**
```
Recovery → Logging → RateLimit → AuthInterceptor → ErrorMapping → handler
```
Note: Recovery stays outermost. RateLimit before auth so even unauthenticated requests are rate-limited (by IP). Auth before ErrorMapping so auth errors get proper mapping.

### Pattern 2: Argon2id Password Hashing

**What:** Hash passwords with argon2id using RFC 9106 recommended parameters, storing hash + salt + params in a single self-describing string.

**Example:**
```go
// internal/auth/hasher.go
package auth

import (
    "crypto/rand"
    "crypto/subtle"
    "encoding/base64"
    "fmt"
    "strings"

    "golang.org/x/crypto/argon2"
)

// Params holds argon2id configuration per RFC 9106 §7.3
type Params struct {
    Memory      uint32 // KiB
    Iterations  uint32 // time parameter
    Parallelism uint8
    SaltLength  uint32
    KeyLength   uint32
}

// DefaultParams uses RFC 9106 recommended: time=1, memory=64MB, threads=4
var DefaultParams = &Params{
    Memory:      64 * 1024, // 64 MB
    Iterations:  1,
    Parallelism: 4,
    SaltLength:  16,
    KeyLength:   32,
}

// HashPassword generates an argon2id hash string.
// Format: $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
func HashPassword(password string, p *Params) (string, error) {
    salt := make([]byte, p.SaltLength)
    if _, err := rand.Read(salt); err != nil {
        return "", fmt.Errorf("generate salt: %w", err)
    }

    hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

    return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
        argon2.Version, p.Memory, p.Iterations, p.Parallelism,
        base64.RawStdEncoding.EncodeToString(salt),
        base64.RawStdEncoding.EncodeToString(hash),
    ), nil
}

// VerifyPassword checks a password against a stored hash string.
func VerifyPassword(password, encodedHash string) (bool, error) {
    p, salt, hash, err := decodeHash(encodedHash)
    if err != nil {
        return false, err
    }

    otherHash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)

    // Constant-time comparison prevents timing attacks
    return subtle.ConstantTimeCompare(hash, otherHash) == 1, nil
}

func decodeHash(encodedHash string) (*Params, []byte, []byte, error) {
    // Parse $argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
    parts := strings.Split(encodedHash, "$")
    if len(parts) != 6 {
        return nil, nil, nil, fmt.Errorf("invalid hash format")
    }
    // ... parse params, decode salt and hash
    // (implementation straightforward from format)
    return nil, nil, nil, nil // placeholder
}
```

### Pattern 3: Redis Token Bucket Rate Limiting

**What:** Atomic token bucket via Redis Lua script. Single round-trip per request.

**Example:**
```go
// internal/platform/ratelimit/limiter.go
package ratelimit

import (
    "context"
    "fmt"
    "time"

    goredis "github.com/redis/go-redis/v9"
)

// Lua script for atomic token bucket
// KEYS[1] = bucket key
// ARGV[1] = max tokens (capacity)
// ARGV[2] = refill rate (tokens per second)
// ARGV[3] = current time (Unix microseconds)
// Returns: {allowed (0/1), remaining, retry_after_ms}
var tokenBucketScript = goredis.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
    tokens = capacity
    last_refill = now
end

-- Refill tokens based on elapsed time
local elapsed = (now - last_refill) / 1000000  -- convert to seconds
local refill = math.floor(elapsed * rate)
if refill > 0 then
    tokens = math.min(capacity, tokens + refill)
    last_refill = now
end

local allowed = 0
local retry_after = 0

if tokens > 0 then
    tokens = tokens - 1
    allowed = 1
else
    -- Calculate time until next token
    retry_after = math.ceil((1 / rate) * 1000)  -- ms
end

redis.call('HMSET', key, 'tokens', tokens, 'last_refill', last_refill)
redis.call('EXPIRE', key, math.ceil(capacity / rate) + 1)

return {allowed, tokens, retry_after}
`)

type Limiter struct {
    rdb *goredis.Client
}

func New(rdb *goredis.Client) *Limiter {
    return &Limiter{rdb: rdb}
}

type Result struct {
    Allowed    bool
    Remaining  int
    Limit      int
    RetryAfter time.Duration
}

func (l *Limiter) Check(ctx context.Context, key string, limit int, windowSeconds int) (*Result, error) {
    rate := float64(limit) / float64(windowSeconds)
    now := time.Now().UnixMicro()

    res, err := tokenBucketScript.Run(ctx, l.rdb, []string{fmt.Sprintf("rl:%s", key)}, limit, rate, now).Int64Slice()
    if err != nil {
        return nil, fmt.Errorf("rate limit check: %w", err)
    }

    return &Result{
        Allowed:    res[0] == 1,
        Remaining:  int(res[1]),
        Limit:      limit,
        RetryAfter: time.Duration(res[2]) * time.Millisecond,
    }, nil
}
```

### Pattern 4: Frontend API Client with Auth Token Management

**What:** A thin fetch wrapper that injects the access token, handles 401 responses by attempting a silent refresh, and retries the original request.

**Example:**
```typescript
// web/src/lib/api.ts

const API_BASE = '/api/v1';

let accessToken: string | null = null;
let refreshPromise: Promise<boolean> | null = null;

export function setAccessToken(token: string | null) {
  accessToken = token;
}

export function getAccessToken(): string | null {
  return accessToken;
}

async function refreshAccessToken(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: 'POST',
      credentials: 'include', // send httpOnly cookie
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    });
    if (!res.ok) return false;
    const data = await res.json();
    accessToken = data.accessToken;
    return true;
  } catch {
    return false;
  }
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers);
  if (accessToken) {
    headers.set('Authorization', `Bearer ${accessToken}`);
  }
  if (!headers.has('Content-Type') && options.body) {
    headers.set('Content-Type', 'application/json');
  }

  let res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
    credentials: 'include',
  });

  // On 401, attempt silent refresh (deduplicated)
  if (res.status === 401 && accessToken) {
    if (!refreshPromise) {
      refreshPromise = refreshAccessToken();
    }
    const refreshed = await refreshPromise;
    refreshPromise = null;

    if (refreshed) {
      headers.set('Authorization', `Bearer ${accessToken}`);
      res = await fetch(`${API_BASE}${path}`, {
        ...options,
        headers,
        credentials: 'include',
      });
    }
  }

  if (!res.ok) {
    const error = await res.json().catch(() => ({ message: res.statusText }));
    throw new ApiError(res.status, error.message || res.statusText);
  }

  return res.json();
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}
```

### Pattern 5: Svelte 5 Auth Store (Runes-based)

**What:** A reactive auth store using Svelte 5's `$state` rune. Persists minimal state, manages token lifecycle.

**Example:**
```typescript
// web/src/lib/auth.ts
import { setAccessToken, api } from './api';

interface User {
  userId: string;
  username: string;
}

// Module-level state (Svelte 5 runes)
let user = $state<User | null>(null);
let loading = $state(true);

export function getUser() { return user; }
export function isAuthenticated() { return user !== null; }
export function isLoading() { return loading; }

export async function initialize() {
  // On app start, try to refresh token (cookie-based)
  try {
    const res = await api<{ accessToken: string; userId: string }>('/auth/refresh', {
      method: 'POST',
    });
    setAccessToken(res.accessToken);
    // Fetch minimal user info
    // ...set user state
  } catch {
    // Not authenticated — that's fine
    user = null;
  } finally {
    loading = false;
  }
}

export async function login(email: string, password: string) {
  const res = await api<{ accessToken: string; refreshToken: string; userId: string }>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
  setAccessToken(res.accessToken);
  // Store refresh token handled by httpOnly cookie set by server
  // Fetch user profile...
}

export function logout() {
  api('/auth/logout', { method: 'POST' }).catch(() => {});
  setAccessToken(null);
  user = null;
}
```

### Pattern 6: Database Schema Design

**Auth Database (`auth`):**
```sql
-- migrations/auth/001_users.up.sql
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email       TEXT NOT NULL UNIQUE,
    username    TEXT NOT NULL UNIQUE,
    password_hash TEXT,  -- NULL for OAuth-only users
    auth_method TEXT NOT NULL DEFAULT 'email',  -- 'email' | 'google'
    google_id   TEXT UNIQUE,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_google_id ON users(google_id) WHERE google_id IS NOT NULL;

CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,  -- SHA-256 hash of the token
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens(expires_at);
```

**User Database (`user_profiles`):**
```sql
-- migrations/user/001_profiles.up.sql
CREATE TABLE profiles (
    user_id      UUID PRIMARY KEY,  -- matches auth.users.id (no FK — separate DB)
    username     TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    bio          TEXT NOT NULL DEFAULT '' CHECK (length(bio) <= 500),
    avatar_url   TEXT NOT NULL DEFAULT '',
    karma        INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ  -- soft delete for USER-05
);

CREATE INDEX idx_profiles_username ON profiles(username);
```

**Community Database (`community`):**
```sql
-- migrations/community/001_communities.up.sql
CREATE TABLE communities (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         TEXT NOT NULL UNIQUE CHECK (name ~ '^[a-zA-Z0-9_]{3,21}$'),
    description  TEXT NOT NULL DEFAULT '',
    rules        JSONB NOT NULL DEFAULT '[]',
    banner_url   TEXT NOT NULL DEFAULT '',
    icon_url     TEXT NOT NULL DEFAULT '',
    visibility   SMALLINT NOT NULL DEFAULT 1,  -- 1=public, 2=restricted, 3=private
    member_count INTEGER NOT NULL DEFAULT 0,
    owner_id     UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_communities_name ON communities(name);

CREATE TABLE community_members (
    community_id UUID NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL,
    role         TEXT NOT NULL DEFAULT 'member',  -- 'member' | 'moderator' | 'owner'
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (community_id, user_id)
);

CREATE INDEX idx_community_members_user ON community_members(user_id);
```

### Anti-Patterns to Avoid
- **Shared database between services:** Auth, user, and community services MUST have separate PostgreSQL databases. Cross-service data is fetched via gRPC calls or eventual consistency, never direct DB queries.
- **Storing JWT access tokens server-side:** Access tokens are stateless and validated by signature alone. Only refresh tokens are stored. Don't create an "active sessions" table for access tokens.
- **Using math/rand for any security value:** Always use `crypto/rand` for OTP codes, salt, reset tokens, refresh tokens. `math/rand` is predictable.
- **Blocking email sends in the request path:** OTP and password reset emails must not block the gRPC response. Log/stub in dev, use async queue in production.
- **Storing refresh token as plaintext in DB:** Store SHA-256 hash of the refresh token. The token itself is sent to the client. This way a DB leak doesn't compromise active sessions.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Password hashing | Custom hash function | `argon2.IDKey` from `golang.org/x/crypto/argon2` | Timing attacks, parameter tuning, salt management — decades of research behind argon2id |
| JWT creation/validation | Manual base64 + HMAC | `github.com/golang-jwt/jwt/v5` | Token format, claim validation, expiry checking, algorithm verification — all handled correctly |
| OAuth2 code exchange | Manual HTTP to Google token endpoint | `golang.org/x/oauth2` with `google.Endpoint` | Token refresh, error handling, PKCE, endpoint discovery |
| Rate limit token bucket | Manual Redis GET/SET (non-atomic) | Redis Lua script (see Pattern 3) | Race conditions between check and decrement — MUST be atomic |
| UUID generation | Custom random string IDs | `github.com/google/uuid` (v4) | Proper entropy, correct formatting, uniqueness guarantees |
| Cursor-based pagination | Custom offset/limit | Existing `internal/platform/pagination` | Already built in Phase 1, handles encode/decode correctly |

**Key insight:** Every auth/security component has subtle edge cases (timing attacks, race conditions, token format exploits) that libraries handle after years of security review. Custom implementations for these are the #1 source of security vulnerabilities.

## Common Pitfalls

### Pitfall 1: Envoy Authorization Header Forwarding
**What goes wrong:** Envoy's gRPC-JSON transcoder reads the `Authorization` header from the HTTP request and forwards it as gRPC metadata. But if you also set a JWT in a cookie, the cookie won't automatically become gRPC metadata.
**Why it happens:** Envoy transcoder only forwards standard HTTP headers as gRPC metadata. Cookies are forwarded but need explicit extraction on the gRPC side.
**How to avoid:** Use `Authorization: Bearer <token>` header for access tokens (frontend sends this via fetch). Use httpOnly cookie ONLY for the refresh token, and the refresh endpoint specifically reads the cookie from the `cookie` gRPC metadata.
**Warning signs:** Auth works in direct gRPC calls but fails through Envoy; refresh endpoint returns unauthenticated.

### Pitfall 2: Race Condition on Concurrent Refresh
**What goes wrong:** Multiple browser tabs or parallel requests all detect expired access token and simultaneously call `/auth/refresh`. If refresh tokens are single-use (rotated), the second request fails because the first already consumed the token.
**Why it happens:** Standard refresh token rotation invalidates the old token, but concurrent requests share the same old token.
**How to avoid:** Deduplicate refresh calls in the frontend API client (see Pattern 4 — `refreshPromise` singleton). On the server side, add a small grace period (~5s) where the old refresh token is still valid after rotation, OR don't rotate refresh tokens (just use long-lived with periodic cleanup).
**Warning signs:** Users randomly get logged out; "invalid refresh token" errors in production logs.

### Pitfall 3: Missing Error Mapping for codes.ResourceExhausted
**What goes wrong:** Rate limiter returns `codes.ResourceExhausted` but the existing error mapping interceptor doesn't handle it — falls through to `codes.Internal` with sanitized "internal error" message. Client gets 500 instead of 429.
**Why it happens:** The current `ErrorMapping()` interceptor only handles domain errors (`ErrNotFound`, etc.), not gRPC status codes returned by other interceptors.
**How to avoid:** The rate limit interceptor should return a proper `status.Error(codes.ResourceExhausted, ...)` with gRPC trailing metadata for `retry-after`. Since ErrorMapping passes through errors that are already gRPC statuses (`status.FromError` check), this should work correctly. But verify that Envoy maps `ResourceExhausted` to HTTP 429 (it does per gRPC-HTTP status mapping spec).
**Warning signs:** Rate-limited requests return 500 instead of 429.

### Pitfall 4: Database-Per-Service PostgreSQL Setup in Docker Compose
**What goes wrong:** Currently Docker Compose has a single `postgres` service with `POSTGRES_DB: skeleton`. Adding 3 more databases requires either (a) multiple PostgreSQL containers, or (b) an init script that creates multiple databases in one container.
**Why it happens:** `POSTGRES_DB` only creates one database on first run.
**How to avoid:** Use a PostgreSQL init script mounted at `/docker-entrypoint-initdb.d/init.sql` that creates `auth`, `user_profiles`, and `community` databases. Single PostgreSQL container, multiple databases. This is standard for local dev (production would be separate instances).
**Warning signs:** Services fail to start with "database does not exist" errors.

### Pitfall 5: Envoy Route Configuration for Multiple Services
**What goes wrong:** Current Envoy config routes ALL `/api/v1/` to `skeleton-service`. Phase 2 needs auth, user, and community services to each receive their own routes.
**Why it happens:** Single catch-all route prefix.
**How to avoid:** Add specific route prefixes BEFORE the catch-all:
```yaml
routes:
  - match: { prefix: "/api/v1/auth/" }
    route: { cluster: auth-service }
  - match: { prefix: "/api/v1/users/" }
    route: { cluster: user-service }
  - match: { prefix: "/api/v1/communities/" }
    route: { cluster: community-service }
  - match: { prefix: "/api/v1/ratelimit/" }
    route: { cluster: auth-service }  # rate limit debugging endpoint
  - match: { prefix: "/api/v1/" }
    route: { cluster: skeleton-service }  # fallback
```
Also register all proto services in the `grpc_json_transcoder` services list.
**Warning signs:** 404 on auth endpoints; requests routed to wrong service.

### Pitfall 6: Header/Sidebar Must Become Svelte Components
**What goes wrong:** Header.astro and Sidebar.astro are static Astro components. They can't react to auth state changes (login/logout) without a full page reload.
**Why it happens:** Astro components render once at build/SSR time. Dynamic state requires Svelte islands.
**How to avoid:** Replace `Header.astro` with `Header.svelte` (mounted with `client:load`) and `Sidebar.astro` with `Sidebar.svelte` (mounted with `client:load`). They import the auth store and reactively show user info / joined communities.
**Warning signs:** User logs in but header still shows `[anonymous]` until page refresh.

### Pitfall 7: OTP Timing Attack
**What goes wrong:** Comparing OTP codes with `==` string comparison leaks timing information about which characters match.
**Why it happens:** Standard string equality is not constant-time.
**How to avoid:** Use `crypto/subtle.ConstantTimeCompare` for OTP code comparison. Also enforce rate limiting on the verify endpoint (e.g., 5 attempts per email per 5 minutes) to prevent brute-force.
**Warning signs:** Theoretical vulnerability; hard to detect in testing but real in production.

### Pitfall 8: Refresh Token in Response Body vs Cookie
**What goes wrong:** Proto `LoginResponse` includes `refresh_token` as a field. If returned in JSON body, JavaScript can read it (XSS risk). If we want httpOnly cookie, Envoy transcoder doesn't have a mechanism to set cookies from proto response fields.
**Why it happens:** gRPC/Envoy transcoder maps proto fields to JSON. No built-in cookie-setting mechanism.
**How to avoid:** Two approaches:
1. **Simpler (recommended for this phase):** Return refresh token in response body. Store in a Svelte module-level variable (not localStorage). Accept the XSS risk is mitigated by the terminal-style UI having no user-generated HTML rendering in auth contexts.
2. **Alternative:** Use gRPC server-side trailing metadata to set a `set-cookie` header, and configure Envoy to forward response headers. More complex but more secure.
For v1, approach 1 is acceptable. The refresh token is also hashed in the DB, so a stolen token can be revoked.
**Warning signs:** Refresh token accessible in browser DevTools network tab (by design with approach 1).

## Code Examples

### JWT Token Creation and Validation
```go
// internal/auth/jwt.go
package auth

import (
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
    secret     []byte
    accessTTL  time.Duration
}

type Claims struct {
    jwt.RegisteredClaims
    UserID   string `json:"uid"`
    Username string `json:"username"`
}

func NewJWTManager(secret string, accessTTL time.Duration) *JWTManager {
    return &JWTManager{
        secret:    []byte(secret),
        accessTTL: accessTTL,
    }
}

func (m *JWTManager) Generate(userID, username string) (string, time.Time, error) {
    expiresAt := time.Now().Add(m.accessTTL)
    claims := &Claims{
        RegisteredClaims: jwt.RegisteredClaims{
            Subject:   userID,
            ExpiresAt: jwt.NewNumericDate(expiresAt),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "redyx",
        },
        UserID:   userID,
        Username: username,
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    signed, err := token.SignedString(m.secret)
    if err != nil {
        return "", time.Time{}, fmt.Errorf("sign token: %w", err)
    }
    return signed, expiresAt, nil
}

type JWTValidator struct {
    parser *jwt.Parser
    secret []byte
}

func NewJWTValidator(secret string) *JWTValidator {
    return &JWTValidator{
        parser: jwt.NewParser(
            jwt.WithValidMethods([]string{"HS256"}),
            jwt.WithIssuer("redyx"),
            jwt.WithExpirationRequired(),
        ),
        secret: []byte(secret),
    }
}

func (v *JWTValidator) Validate(tokenString string) (*Claims, error) {
    token, err := v.parser.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
        return v.secret, nil
    })
    if err != nil {
        return nil, fmt.Errorf("parse token: %w", err)
    }

    claims, ok := token.Claims.(*Claims)
    if !ok {
        return nil, fmt.Errorf("invalid claims type")
    }
    return claims, nil
}
```

### OTP Generation and Verification
```go
// internal/auth/otp.go
package auth

import (
    "context"
    "crypto/rand"
    "crypto/subtle"
    "fmt"
    "math/big"
    "time"

    goredis "github.com/redis/go-redis/v9"
)

const otpTTL = 5 * time.Minute

type OTPManager struct {
    rdb *goredis.Client
}

func NewOTPManager(rdb *goredis.Client) *OTPManager {
    return &OTPManager{rdb: rdb}
}

func (m *OTPManager) Generate(ctx context.Context, email string) (string, error) {
    code, err := generateCode(6)
    if err != nil {
        return "", err
    }

    key := fmt.Sprintf("otp:%s", email)
    if err := m.rdb.Set(ctx, key, code, otpTTL).Err(); err != nil {
        return "", fmt.Errorf("store OTP: %w", err)
    }
    return code, nil
}

func (m *OTPManager) Verify(ctx context.Context, email, code string) (bool, error) {
    key := fmt.Sprintf("otp:%s", email)
    stored, err := m.rdb.Get(ctx, key).Result()
    if err == goredis.Nil {
        return false, nil // expired or never set
    }
    if err != nil {
        return false, fmt.Errorf("get OTP: %w", err)
    }

    // Constant-time comparison
    if subtle.ConstantTimeCompare([]byte(stored), []byte(code)) != 1 {
        return false, nil
    }

    // Delete after successful verification (single-use)
    m.rdb.Del(ctx, key)
    return true, nil
}

func generateCode(length int) (string, error) {
    code := ""
    for i := 0; i < length; i++ {
        n, err := rand.Int(rand.Reader, big.NewInt(10))
        if err != nil {
            return "", fmt.Errorf("generate random digit: %w", err)
        }
        code += fmt.Sprintf("%d", n.Int64())
    }
    return code, nil
}
```

### Google OAuth Code Exchange
```go
// internal/auth/oauth.go
package auth

import (
    "context"
    "encoding/json"
    "fmt"
    "io"

    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
)

type GoogleUser struct {
    ID    string `json:"sub"`
    Email string `json:"email"`
    Name  string `json:"name"`
}

type OAuthManager struct {
    config *oauth2.Config
}

func NewOAuthManager(clientID, clientSecret, redirectURL string) *OAuthManager {
    return &OAuthManager{
        config: &oauth2.Config{
            ClientID:     clientID,
            ClientSecret: clientSecret,
            RedirectURL:  redirectURL,
            Scopes:       []string{"openid", "email", "profile"},
            Endpoint:     google.Endpoint,
        },
    }
}

func (m *OAuthManager) Exchange(ctx context.Context, code string) (*GoogleUser, error) {
    token, err := m.config.Exchange(ctx, code)
    if err != nil {
        return nil, fmt.Errorf("exchange code: %w", err)
    }

    client := m.config.Client(ctx, token)
    resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
    if err != nil {
        return nil, fmt.Errorf("get user info: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    var user GoogleUser
    if err := json.Unmarshal(body, &user); err != nil {
        return nil, fmt.Errorf("parse user info: %w", err)
    }
    return &user, nil
}
```

### Docker Compose Init Script (Multiple Databases)
```sql
-- deploy/docker/init-databases.sql
-- Creates all databases needed for Phase 2 services
-- Mounted at /docker-entrypoint-initdb.d/ in PostgreSQL container

CREATE DATABASE auth;
CREATE DATABASE user_profiles;
CREATE DATABASE community;

-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE auth TO redyx;
GRANT ALL PRIVILEGES ON DATABASE user_profiles TO redyx;
GRANT ALL PRIVILEGES ON DATABASE community TO redyx;
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| bcrypt for passwords | argon2id (RFC 9106) | 2019 (PHC winner), RFC 2022 | argon2id is memory-hard, resistant to GPU/ASIC attacks. bcrypt is still acceptable but argon2id is recommended for new projects |
| dgrijalva/jwt-go | golang-jwt/jwt/v5 | 2021 (maintainer change), v5 2023 | Original library abandoned; golang-jwt is the maintained fork with improved validation |
| Session cookies (server-side sessions) | JWT access + refresh token pattern | Ongoing standard | Stateless access tokens reduce DB load; refresh tokens maintain session control |
| Envoy ext_authz for auth | Application-level JWT validation | Both are valid | ext_authz adds complexity (separate auth service HTTP call per request). Application-level is simpler when services already share platform libraries |
| Separate rate limit service (lyft/ratelimit) | Application-level Redis token bucket | Both valid | lyft/ratelimit is designed for Envoy but adds operational complexity. Redis Lua script in application is simpler for this project's scale |

**Deprecated/outdated:**
- `dgrijalva/jwt-go`: Abandoned, don't use. Use `github.com/golang-jwt/jwt/v5` instead
- bcrypt with cost 10: Still works but argon2id is the current recommendation for new projects (RFC 9106)
- `oauth2.NoContext`: Deprecated in golang.org/x/oauth2; always pass a real context

## Open Questions

1. **Email sending in development**
   - What we know: OTP and password reset require sending emails. The proto flow expects codes/tokens to be sent to the user's email.
   - What's unclear: No email service or provider chosen. For development, emails shouldn't actually be sent.
   - Recommendation: Log OTP codes and reset tokens to stdout in development. Define an `EmailSender` interface so a real implementation (SendGrid, SES, SMTP) can be swapped in later. Not a blocker for Phase 2.

2. **Avatar upload mechanism**
   - What we know: USER-04 requires avatar update. The `UpdateProfileRequest` has `avatar_url` field. Media uploads (MDIA-*) are deferred to Phase 5.
   - What's unclear: How does the user get an `avatar_url` before Phase 5?
   - Recommendation: For Phase 2, accept `avatar_url` as a direct URL string (user provides a URL to an externally hosted image). Full upload flow comes in Phase 5. The UI can show a text input for avatar URL.

3. **Cross-service user data synchronization**
   - What we know: Auth service has the canonical user record (email, password). User service has the profile (display_name, bio, karma). Community service references user_id. Username must be consistent across all three.
   - What's unclear: When a user registers in auth-service, how does user-service get the initial profile record?
   - Recommendation: Auth service calls user-service (via gRPC) during registration to create the initial profile. Or, use a synchronous "create profile" call from auth after user creation. Event-based (Kafka) is overkill for this — save Kafka for Phase 3's vote/karma updates.

4. **GetUserPosts / GetUserComments in Phase 2**
   - What we know: Proto defines these RPCs on UserService. Post and comment services don't exist until Phase 3 and 4.
   - What's unclear: What should these return in Phase 2?
   - Recommendation: Implement the RPCs but return empty lists. The proto responses already support empty `repeated` fields. Frontend profile tabs show "No posts yet" / "No comments yet". Full implementation comes when post/comment services exist.

5. **Trusted user tier for rate limiting**
   - What we know: RATE-02 defines three tiers. Anonymous and authenticated are clear. "Trusted" is undefined.
   - What's unclear: What makes a user "trusted"? Account age? Karma threshold? Manual assignment?
   - Recommendation: Add a `trust_level` column to the auth users table (default: 0=normal, 1=trusted). For Phase 2, all authenticated users are "authenticated" tier. Trusted tier logic (karma-based promotion) can be added in a later phase. The rate limit system supports it from day one.

## Sources

### Primary (HIGH confidence)
- golang.org/x/crypto/argon2 — pkg.go.dev v0.48.0 docs; RFC 9106 §7.3 parameter recommendations verified
- github.com/golang-jwt/jwt/v5 — pkg.go.dev v5.3.1; RegisteredClaims, NewParser with WithValidMethods/WithIssuer/WithExpirationRequired
- golang.org/x/oauth2 — pkg.go.dev v0.35.0; google.Endpoint built-in
- Existing codebase: proto definitions (proto/redyx/auth/v1/auth.proto, user/v1/user.proto, community/v1/community.proto, ratelimit/v1/ratelimit.proto)
- Existing codebase: platform libraries (internal/platform/*), skeleton service pattern (cmd/skeleton/main.go, internal/skeleton/server.go)
- Existing codebase: Docker Compose, Envoy config, Makefile, frontend structure (web/src/*)

### Secondary (MEDIUM confidence)
- Redis Lua script pattern for token bucket rate limiting — well-established pattern, multiple production references
- Argon2id encoded hash string format ($argon2id$v=19$...) — de facto standard from the reference implementation
- Envoy gRPC status code to HTTP status code mapping — documented in gRPC spec (ResourceExhausted → 429)

### Tertiary (LOW confidence)
- Svelte 5 runes module-level `$state` for auth stores — Svelte 5 is relatively new; module-level runes may have SSR edge cases with Astro. Need to validate that `$state` works correctly when imported across multiple Svelte components in an Astro SSR context. Fallback: use a simple pub/sub pattern with vanilla JS if runes don't work cross-component.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries are well-established, widely used in Go ecosystem, versions verified
- Architecture: HIGH — follows existing codebase patterns exactly (service structure, middleware chain, proto-first)
- Pitfalls: HIGH — derived from direct codebase analysis (Envoy config, middleware chain, Docker Compose) and well-known auth security patterns
- Frontend auth: MEDIUM — Svelte 5 runes for cross-component state is newer territory; may need validation

**Research date:** 2026-03-03
**Valid until:** 2026-04-03 (stable libraries, 30-day validity)
