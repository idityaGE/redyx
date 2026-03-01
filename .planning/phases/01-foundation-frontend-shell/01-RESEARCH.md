# Phase 1: Foundation + Frontend Shell - Research

**Researched:** 2026-03-02
**Domain:** Proto definitions, shared Go platform libraries, Envoy gRPC-JSON transcoding, Docker Compose, Astro+Svelte frontend shell
**Confidence:** HIGH

## Summary

Phase 1 establishes the scaffolding every subsequent phase builds on: protobuf definitions with HTTP annotations compiled via buf, shared Go platform libraries for gRPC server bootstrap and middleware, Envoy API gateway with REST-to-gRPC transcoding, Docker Compose for local development infrastructure, and an Astro+Svelte frontend project with a responsive terminal-aesthetic layout shell.

The primary technical challenge is getting the Envoy gRPC-JSON transcoder configured correctly from day one — field naming conventions (camelCase vs snake_case), route matching behavior (`match_incoming_request_route`), and the proto descriptor build pipeline must all be decided before any service code is written. These decisions propagate to every future service and the entire frontend API layer. The Astro+Svelte frontend is straightforward — Astro 5 with SSR via `@astrojs/node`, Svelte 5 for interactive islands, and TailwindCSS for the terminal-like monospace aesthetic.

**Primary recommendation:** Build buf config → proto definitions → Go platform libraries → skeleton gRPC service → Envoy transcoding → Docker Compose → Astro frontend in strict dependency order. Verify the full chain (REST JSON → Envoy → gRPC → Go → response) works end-to-end before moving to Phase 2.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **Content density:** Information-rich, terminal-like aesthetic. Dense data per screen, not spacious cards. Feed items show title + metadata only. Media as small thumbnails. Vote arrows on left. Density closer to old.reddit.com / Hacker News
- **Visual identity:** TailwindCSS with dark/light mode. Orange/amber primary accent. Full monospace typography (JetBrains Mono, Fira Code, or similar). ASCII-style box-drawing borders. "TUI app in the browser" feel
- **Layout & navigation:** Sidebar + top bar (Reddit desktop). SPA-style panel navigation. On mobile: bottom tab bar. Sidebar collapses on tablet, bottom tabs on mobile
- **API naming conventions:** Flat resource paths `/api/v1/...`. gRPC status codes mapped to HTTP. Cursor-based pagination. API versioning via URL path

### Claude's Discretion
- JSON field naming convention (snake_case vs camelCase) — pick based on Envoy transcoding defaults and proto conventions
- Loading skeleton/spinner design within the terminal aesthetic
- Error state UI patterns
- Exact spacing and padding values within the monospace grid
- Sidebar collapse/expand animation style
- Specific monospace font choice (JetBrains Mono vs Fira Code vs others)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| INFRA-01 | Docker Compose configuration for local development with all services and data stores | Docker Compose patterns documented in ARCHITECTURE.md; Phase 1 needs PostgreSQL (1 instance for skeleton), Redis, Envoy — not all 9 data stores yet |
| FEND-01 | Astro SSR frontend with Svelte interactive islands for dynamic components | Astro 5 + @astrojs/svelte + @astrojs/node adapter verified; SSR with `output: 'server'` config |
| FEND-02 | Envoy API gateway with REST-to-gRPC transcoding via proto descriptor set | Envoy v1.37 grpc_json_transcoder filter verified; requires proto descriptor from `buf build -o proto.pb`; `match_incoming_request_route: true` recommended |
| FEND-03 | Responsive layout for desktop, tablet, and mobile | TailwindCSS responsive breakpoints; sidebar + top bar on desktop, collapsed sidebar on tablet, bottom tabs on mobile |
</phase_requirements>

## Standard Stack

### Core

| Library / Tool | Version | Purpose | Why Standard |
|----------------|---------|---------|--------------|
| Go | 1.26.0 | Skeleton service, shared platform libraries | Latest stable. Native gRPC, static binaries. |
| grpc-go | v1.79.1 | gRPC server/client framework | Official Go gRPC. Interceptors, health checking, reflection. |
| protoc-gen-go | latest | Go protobuf code generation | `google.golang.org/protobuf` — the current module (NOT deprecated `github.com/golang/protobuf`). |
| protoc-gen-go-grpc | v1.6.1 | gRPC service stub generation | Paired with grpc-go. |
| buf | v1.66.0 | Proto management, linting, code generation, descriptor build | Replaces raw protoc entirely. Single tool for lint, generate, breaking change detection, descriptor output. |
| pgx | v5.8.0 | PostgreSQL driver | Pure Go, fastest, built-in connection pooling via pgxpool. Used by skeleton service to verify DB connectivity. |
| go-redis | v9.18.0 | Redis client | Official Redis Go client. Used by skeleton service to verify Redis connectivity. |
| Envoy | v1.37.0 | API gateway, REST-to-gRPC transcoding | Native grpc_json_transcoder filter. Docker image: `envoyproxy/envoy:v1.37.0`. |
| Astro | 5.x | SSR frontend framework | Ships zero JS by default, hydrates Svelte islands on demand. |
| Svelte | 5.x | Interactive UI components | Svelte 5 with runes API. Minimal bundle for interactive elements. |
| @astrojs/svelte | latest | Astro-Svelte integration | Enables `client:*` directives for Svelte components. |
| @astrojs/node | latest | Astro Node.js SSR adapter | Required for server-side rendering deployment. |
| TailwindCSS | v4.x | Utility-first CSS framework | Built-in dark mode, responsive utilities, custom theme for terminal aesthetic. |
| zap | v1.27.1 | Structured logging | JSON output for future Loki ingestion. Fastest Go logger. |

### Supporting

| Library / Tool | Version | Purpose | When to Use |
|----------------|---------|---------|-------------|
| golang-migrate | v4.19.1 | Database migrations | Run schema setup for skeleton service PostgreSQL tables |
| grpc-gateway v2 | v2.28.0 | protoc-gen-openapiv2 plugin only | Generate OpenAPI specs from proto files (NOT for actual gateway — Envoy handles that) |
| google.golang.org/grpc/health | (included in grpc-go) | gRPC health checking protocol | Every service registers health checks; Envoy and Docker Compose use them |
| google.golang.org/grpc/reflection | (included in grpc-go) | gRPC server reflection | Enable on all services for debugging with grpcurl/grpcui |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| buf (proto management) | raw protoc | Never — buf is strictly superior for linting, generation, dependency management |
| Envoy (gateway) | Custom Go HTTP gateway | More control but massive effort to implement REST-to-gRPC transcoding, JWT validation, rate limiting |
| TailwindCSS v4 | TailwindCSS v3 | v4 is current; uses CSS-first config instead of JS config file. If any compatibility issues arise, v3 is a safe fallback |
| JetBrains Mono (font) | Fira Code, Source Code Pro, IBM Plex Mono | All are excellent monospace fonts. JetBrains Mono has best ligature support and readability at small sizes. **Recommendation: JetBrains Mono** |

**Installation — Backend:**
```bash
# Go module init
go mod init github.com/redyx/redyx

# Core gRPC + proto
go get google.golang.org/grpc@v1.79.1
go get google.golang.org/protobuf@latest

# Database + cache
go get github.com/jackc/pgx/v5@v5.8.0
go get github.com/redis/go-redis/v9@v9.18.0

# Migrations
go get github.com/golang-migrate/migrate/v4@v4.19.1

# Logging
go get go.uber.org/zap@v1.27.1

# Dev tools (install separately)
# buf: https://buf.build/docs/cli/installation
# golangci-lint: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

**Installation — Frontend:**
```bash
# In web/ directory
npm create astro@latest -- --template minimal
npx astro add svelte
npx astro add node
npm install -D tailwindcss @tailwindcss/vite
```

## Architecture Patterns

### Recommended Project Structure (Phase 1 scope)

```
redyx/
├── go.mod                          # Single module: github.com/redyx/redyx
├── go.sum
├── buf.yaml                        # Buf v2 workspace config
├── buf.gen.yaml                    # Buf code generation config
├── buf.lock                        # Buf dependency lock
├── Makefile                        # proto, build, lint, test targets
├── docker-compose.yml              # Local dev infrastructure + services
│
├── proto/                          # ALL protobuf definitions
│   └── redyx/
│       ├── common/v1/
│       │   └── common.proto        # Shared types: Pagination, Timestamp, etc.
│       ├── health/v1/
│       │   └── health.proto        # Health check RPC (skeleton service)
│       ├── auth/v1/
│       │   └── auth.proto          # Auth service protos (stubs for Phase 2)
│       ├── user/v1/
│       │   └── user.proto
│       ├── community/v1/
│       │   └── community.proto
│       ├── post/v1/
│       │   └── post.proto
│       ├── comment/v1/
│       │   └── comment.proto
│       ├── vote/v1/
│       │   └── vote.proto
│       ├── search/v1/
│       │   └── search.proto
│       ├── media/v1/
│       │   └── media.proto
│       ├── notification/v1/
│       │   └── notification.proto
│       ├── moderation/v1/
│       │   └── moderation.proto
│       ├── ratelimit/v1/
│       │   └── ratelimit.proto
│       └── spam/v1/
│           └── spam.proto
│
├── gen/                            # GENERATED Go code (from buf generate)
│   └── redyx/
│       ├── common/v1/
│       ├── health/v1/
│       └── ...                     # Mirrors proto/ structure
│
├── cmd/                            # Service entry points
│   └── skeleton/                   # Phase 1 skeleton service
│       └── main.go
│
├── internal/
│   ├── skeleton/                   # Skeleton service implementation
│   │   └── server.go
│   └── platform/                   # SHARED internal libraries
│       ├── grpcserver/
│       │   └── server.go           # NewServer(), health check, reflection, interceptors
│       ├── config/
│       │   └── config.go           # Env-based config loading
│       ├── database/
│       │   └── postgres.go         # pgxpool setup helper
│       ├── redis/
│       │   └── client.go           # Redis client setup helper
│       ├── middleware/
│       │   ├── logging.go          # Structured logging interceptor
│       │   ├── recovery.go         # Panic recovery interceptor
│       │   └── errors.go           # gRPC error code mapping interceptor
│       ├── errors/
│       │   └── errors.go           # Standardized domain error types
│       └── pagination/
│           └── cursor.go           # Cursor-based pagination helpers
│
├── migrations/
│   └── skeleton/
│       └── 001_initial.up.sql      # Test table for connectivity verification
│
├── deploy/
│   ├── envoy/
│   │   ├── envoy.yaml              # Gateway config with transcoding
│   │   └── proto.pb                # Compiled proto descriptor set
│   └── docker/
│       └── Dockerfile              # Multi-stage Go build (shared for all services)
│
└── web/                            # Astro + Svelte frontend
    ├── package.json
    ├── astro.config.mjs
    ├── tailwind.css                # TailwindCSS entry with custom theme
    ├── src/
    │   ├── layouts/
    │   │   └── BaseLayout.astro    # Shell: header + sidebar + content + footer
    │   ├── pages/
    │   │   └── index.astro         # Home page placeholder
    │   ├── components/
    │   │   ├── Header.astro        # Top bar (search, notifications, user menu)
    │   │   ├── Sidebar.astro       # Left sidebar (communities, shortcuts)
    │   │   ├── Footer.astro        # Footer bar
    │   │   ├── MobileNav.svelte    # Bottom tab bar (mobile, client:visible)
    │   │   └── ThemeToggle.svelte  # Dark/light mode switch (client:load)
    │   ├── styles/
    │   │   └── terminal.css        # Box-drawing borders, ASCII-style utilities
    │   └── lib/
    │       └── api.ts              # API client for Envoy gateway
    └── public/
        └── fonts/                  # JetBrains Mono self-hosted
```

### Pattern 1: gRPC Server Bootstrap (internal/platform/grpcserver)

**What:** A shared bootstrap function that creates a gRPC server with standard interceptors (logging, recovery, error mapping), registers health checking, enables reflection, and handles graceful shutdown on SIGTERM.

**When to use:** Every service's `cmd/*/main.go` calls this.

**Example:**
```go
// Source: based on grpc-go official patterns + project architecture research
// internal/platform/grpcserver/server.go
package grpcserver

import (
    "context"
    "fmt"
    "net"
    "os"
    "os/signal"
    "syscall"

    "go.uber.org/zap"
    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/reflection"
)

type Server struct {
    srv    *grpc.Server
    health *health.Server
    port   int
    logger *zap.Logger
}

type Option func(*serverConfig)

func WithUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) Option {
    return func(c *serverConfig) {
        c.unaryInterceptors = append(c.unaryInterceptors, interceptors...)
    }
}

func New(port int, logger *zap.Logger, opts ...Option) *Server {
    cfg := &serverConfig{}
    for _, opt := range opts {
        opt(cfg)
    }

    srv := grpc.NewServer(
        grpc.ChainUnaryInterceptor(cfg.unaryInterceptors...),
    )

    hs := health.NewServer()
    healthpb.RegisterHealthServer(srv, hs)
    reflection.Register(srv)

    return &Server{srv: srv, health: hs, port: port, logger: logger}
}

func (s *Server) Server() *grpc.Server { return s.srv }

func (s *Server) Run() error {
    lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
    if err != nil {
        return fmt.Errorf("failed to listen: %w", err)
    }

    // Graceful shutdown
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    go func() {
        <-ctx.Done()
        s.logger.Info("shutting down gRPC server")
        s.health.Shutdown()
        s.srv.GracefulStop()
    }()

    s.logger.Info("starting gRPC server", zap.Int("port", s.port))
    return s.srv.Serve(lis)
}
```

### Pattern 2: Error Mapping Interceptor

**What:** A gRPC unary interceptor that catches domain errors and maps them to proper gRPC status codes so Envoy transcodes them to correct HTTP status codes.

**When to use:** Registered in every service via grpcserver bootstrap.

**Example:**
```go
// Source: grpc.io/docs/guides/error + project PITFALLS.md Pitfall 11
// internal/platform/middleware/errors.go
package middleware

import (
    "context"
    "errors"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    perrors "github.com/redyx/redyx/internal/platform/errors"
)

func ErrorMapping() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        resp, err := handler(ctx, req)
        if err == nil {
            return resp, nil
        }
        // Already a gRPC status — pass through
        if _, ok := status.FromError(err); ok {
            return resp, err
        }
        // Map domain errors to gRPC codes
        switch {
        case errors.Is(err, perrors.ErrNotFound):
            return nil, status.Error(codes.NotFound, err.Error())
        case errors.Is(err, perrors.ErrAlreadyExists):
            return nil, status.Error(codes.AlreadyExists, err.Error())
        case errors.Is(err, perrors.ErrForbidden):
            return nil, status.Error(codes.PermissionDenied, err.Error())
        case errors.Is(err, perrors.ErrInvalidInput):
            return nil, status.Error(codes.InvalidArgument, err.Error())
        case errors.Is(err, perrors.ErrUnauthenticated):
            return nil, status.Error(codes.Unauthenticated, err.Error())
        default:
            return nil, status.Error(codes.Internal, "internal error")
        }
    }
}
```

### Pattern 3: Envoy gRPC-JSON Transcoder with match_incoming_request_route

**What:** Configure Envoy to match routes on the incoming REST path (not the rewritten gRPC path) using `match_incoming_request_route: true`. This is critical for intuitive route configuration.

**When to use:** Envoy gateway config (deploy/envoy/envoy.yaml).

**Example:**
```yaml
# Source: envoyproxy.io/docs — grpc_json_transcoder_filter
# deploy/envoy/envoy.yaml (key excerpts)
static_resources:
  listeners:
  - name: main
    address:
      socket_address: { address: 0.0.0.0, port_value: 8080 }
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress
          codec_type: AUTO
          route_config:
            name: local_route
            virtual_hosts:
            - name: backend
              domains: ["*"]
              routes:
              # Routes match on incoming REST paths (not gRPC paths)
              - match: { prefix: "/api/v1/" }
                route:
                  cluster: skeleton-service
                  timeout: 30s
          http_filters:
          - name: envoy.filters.http.grpc_json_transcoder
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.grpc_json_transcoder.v3.GrpcJsonTranscoder
              proto_descriptor: "/etc/envoy/proto.pb"
              services:
                - redyx.health.v1.HealthService
              match_incoming_request_route: true
              print_options:
                add_whitespace: true
                always_print_primitive_fields: true
                always_print_enums_as_ints: false
                preserve_proto_field_names: false  # camelCase JSON output
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router

  clusters:
  - name: skeleton-service
    type: STRICT_DNS
    lb_policy: ROUND_ROBIN
    typed_extension_protocol_options:
      envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
        "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
        explicit_http_config:
          http2_protocol_options: {}  # CRITICAL: gRPC requires HTTP/2
    load_assignment:
      cluster_name: skeleton-service
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address: { address: skeleton-service, port_value: 50051 }
```

### Pattern 4: Buf v2 Configuration for Proto Management

**What:** Use buf v2 format for workspace-level proto management. A single `buf.yaml` at the workspace root defines modules, dependencies, lint rules, and breaking change detection. `buf.gen.yaml` configures code generation with managed mode for go_package.

**When to use:** Project root — configured once, used by `make proto`.

**Example:**
```yaml
# buf.yaml (v2 format) — verified from buf.build/docs/configuration/v2/buf-yaml
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - WIRE_JSON
deps:
  - buf.build/googleapis/googleapis
```

```yaml
# buf.gen.yaml (v2 format) — verified from buf.build/docs/configuration/v2/buf-gen-yaml
version: v2
clean: true
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/redyx/redyx/gen
plugins:
  - remote: buf.build/protocolbuffers/go
    out: gen
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: gen
    opt: paths=source_relative
inputs:
  - directory: proto
```

### Pattern 5: Astro SSR Layout Shell

**What:** Astro base layout that server-renders the full page shell (header, sidebar, content area, footer) with TailwindCSS responsive classes. Interactive elements (mobile nav, theme toggle) hydrate as Svelte islands.

**When to use:** `web/src/layouts/BaseLayout.astro` — every page uses this layout.

**Example:**
```astro
---
// Source: Astro docs — layouts, on-demand rendering
// web/src/layouts/BaseLayout.astro
import Header from '../components/Header.astro';
import Sidebar from '../components/Sidebar.astro';
import Footer from '../components/Footer.astro';
import MobileNav from '../components/MobileNav.svelte';
import ThemeToggle from '../components/ThemeToggle.svelte';

interface Props {
  title: string;
}
const { title } = Astro.props;
---
<html lang="en" class="dark">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>{title} | Redyx</title>
  <link rel="preconnect" href="https://fonts.bunny.net" />
  <link href="https://fonts.bunny.net/css?family=jetbrains-mono:400,500,600,700" rel="stylesheet" />
</head>
<body class="bg-neutral-950 text-neutral-200 font-mono min-h-screen flex flex-col">
  <Header />
  <div class="flex flex-1 overflow-hidden">
    <!-- Sidebar: visible on desktop, hidden on mobile/tablet -->
    <aside class="hidden lg:block w-64 shrink-0 overflow-y-auto border-r border-neutral-800">
      <Sidebar />
    </aside>
    <!-- Main content area -->
    <main class="flex-1 overflow-y-auto p-4">
      <slot />
    </main>
  </div>
  <Footer />
  <!-- Mobile bottom nav: visible on small screens only -->
  <div class="lg:hidden fixed bottom-0 left-0 right-0">
    <MobileNav client:visible />
  </div>
  <ThemeToggle client:load />
</body>
</html>
```

### Anti-Patterns to Avoid

- **Testing gRPC directly but not through Envoy:** Every RPC must be tested through Envoy transcoding. Field name mismatches and route matching issues only surface through the transcoder.
- **Separate proto generation and descriptor build steps:** Use a single `make proto` target that generates Go code AND the Envoy descriptor set. If they can drift, they will.
- **Using `preserve_proto_field_names: true` in Envoy:** This sends snake_case JSON which conflicts with proto3's canonical camelCase mapping. Frontend expectations and Envoy defaults must match. Pick camelCase (the default) and commit to it.
- **Running all 5 PostgreSQL instances in Phase 1:** Start with 1 PostgreSQL instance for the skeleton service. Add per-service instances as services are built. This matches the project's "start minimal" constraint.
- **Using HTTP frameworks (gin/echo/fiber) for the skeleton service:** Services expose gRPC only. Envoy handles REST. No HTTP framework needed.
- **Hard-coding config values:** Use environment variables with sensible defaults from day one. Docker Compose and Kubernetes both inject config via env vars.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Proto compilation & linting | Shell scripts calling protoc | `buf generate` + `buf lint` | Dependency resolution, include path management, and cross-module references are handled automatically |
| REST-to-gRPC translation | Custom Go HTTP handlers that call gRPC | Envoy grpc_json_transcoder filter | Envoy handles path mapping, field name conversion, HTTP method mapping, streaming, and error code translation automatically from proto annotations |
| gRPC health checking | Custom health endpoint | `google.golang.org/grpc/health` | Standard health checking protocol understood by Envoy, Docker, Kubernetes natively |
| Config loading | Manual os.Getenv calls scattered everywhere | Structured config package with defaults | One config struct per service, loaded once, passed via dependency injection |
| CSS reset & utility classes | Custom CSS framework | TailwindCSS | Battle-tested responsive utilities, dark mode, purging unused CSS |
| Responsive breakpoints | Custom media query system | TailwindCSS responsive prefixes (`sm:`, `md:`, `lg:`) | Standard breakpoints, mobile-first approach, consistent across the frontend |
| Monospace font loading | @font-face declarations | Google Fonts / Bunny Fonts CDN or self-hosted | Reliable font delivery, WOFF2 format, subsetting handled by CDN |

**Key insight:** Phase 1 is about establishing conventions, not building features. Every shortcut here multiplies across 12 services and dozens of frontend pages. Invest in correctness now.

## Common Pitfalls

### Pitfall 1: Envoy gRPC-JSON Transcoding Field Name Mismatch

**What goes wrong:** Proto field names use `snake_case` (e.g., `community_id`) but Envoy's transcoder converts to `camelCase` by default (`communityId`). Frontend sends `community_id`, Envoy silently ignores the field (sets to zero-value). The Go handler receives an empty string.

**Why it happens:** `preserve_proto_field_names` defaults to `false` in Envoy, following proto3's canonical JSON mapping (camelCase). Developers test gRPC directly (works) then add Envoy (breaks silently).

**How to avoid:** Set `preserve_proto_field_names: false` (keep default camelCase). Document that the REST API uses camelCase. Frontend must use camelCase. Add an integration test through Envoy that verifies non-zero field values arrive at the Go handler.

**Warning signs:** Fields arrive as zero-values in Go handlers when sent from REST. Works fine with grpcurl but breaks through Envoy.

### Pitfall 2: Envoy Route Matching on gRPC Paths Instead of REST Paths

**What goes wrong:** Without `match_incoming_request_route: true`, Envoy rewrites the path to `/<package>.<service>/<method>` before route matching. Your route prefix `/api/v1/` doesn't match — 404.

**Why it happens:** The transcoder transforms the path before routing by default. Per official Envoy docs: "The requests processed by the transcoder filter will have `/<package>.<service>/<method>` path and `POST` method."

**How to avoid:** Set `match_incoming_request_route: true` in the transcoder config. Then route prefixes match the incoming REST URL as expected.

**Warning signs:** 404s on all REST endpoints even though gRPC works directly. Routes only work with `/<package>.<service>/` prefix format.

### Pitfall 3: Proto Descriptor File Out of Sync

**What goes wrong:** Update .proto files, regenerate Go code, but forget to regenerate the descriptor file for Envoy. New RPCs return 404 via REST. New fields silently dropped.

**Why it happens:** Go code generation and descriptor set generation are separate build artifacts. Without a single build step, they drift.

**How to avoid:** Single `make proto` target that runs `buf generate` AND `buf build -o deploy/envoy/proto.pb` atomically. Never run one without the other.

**Warning signs:** New RPCs work via gRPC but 404 via REST. Added proto fields not appearing in JSON responses.

### Pitfall 4: gRPC Error Code Misuse (Returning Go Errors)

**What goes wrong:** Return `fmt.Errorf("not found")` from gRPC handler. Client receives `UNKNOWN` status. Envoy translates to HTTP 500. Every "not found" looks like a server crash.

**Why it happens:** Go idiom is `return nil, err`. gRPC expects `status.Error(codes.NotFound, ...)`. Plain Go errors become `codes.Unknown`.

**How to avoid:** Build the error-mapping interceptor (middleware/errors.go) before any service handler is written. Register it in grpcserver bootstrap.

**Warning signs:** Frontend gets HTTP 500 for not-found resources. All gRPC errors show `UNKNOWN` code in traces.

### Pitfall 5: Envoy Upstream Not Using HTTP/2

**What goes wrong:** Envoy's cluster config doesn't specify `http2_protocol_options` for the gRPC upstream. Envoy sends HTTP/1.1 to the gRPC backend. Connection fails silently or returns garbled data.

**Why it happens:** Envoy defaults to HTTP/1.1. gRPC requires HTTP/2. This is easy to miss in the cluster config.

**How to avoid:** Every gRPC cluster MUST include:
```yaml
typed_extension_protocol_options:
  envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
    "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
    explicit_http_config:
      http2_protocol_options: {}
```

**Warning signs:** Envoy connects but gRPC calls fail with protocol errors. Works with direct gRPC but not through Envoy.

### Pitfall 6: Astro SSR Not Configured for On-Demand Rendering

**What goes wrong:** Astro defaults to static site generation. Pages with dynamic data (API calls) fail at build time or serve stale content.

**Why it happens:** Astro 5 defaults to `output: 'static'`. SSR requires `output: 'server'` in astro.config.mjs and a server adapter (@astrojs/node).

**How to avoid:** Configure `output: 'server'` in astro.config.mjs from the start. Install @astrojs/node adapter.

**Warning signs:** Build errors when accessing Astro.request or dynamic routes. All pages pre-rendered at build time.

## Code Examples

### Proto File with google.api.http Annotations

```protobuf
// Source: Envoy docs + google.api.http spec
// proto/redyx/health/v1/health.proto
syntax = "proto3";

package redyx.health.v1;

import "google/api/annotations.proto";

service HealthService {
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse) {
    option (google.api.http) = {
      get: "/api/v1/health"
    };
  }
}

message HealthCheckRequest {
  string service = 1;
}

message HealthCheckResponse {
  enum ServingStatus {
    SERVING_STATUS_UNSPECIFIED = 0;
    SERVING_STATUS_SERVING = 1;
    SERVING_STATUS_NOT_SERVING = 2;
  }
  ServingStatus status = 1;
}
```

### Makefile Proto Target

```makefile
# Source: buf.build docs + project architecture research
.PHONY: proto proto-lint proto-breaking

proto: proto-lint  ## Generate Go code + Envoy descriptor from protos
	buf generate
	buf build -o deploy/envoy/proto.pb

proto-lint:  ## Lint proto files
	buf lint

proto-breaking:  ## Check for breaking changes vs git
	buf breaking --against '.git#branch=main'

proto-descriptor:  ## Build Envoy descriptor set only
	buf build -o deploy/envoy/proto.pb
```

### Docker Compose (Phase 1 — Minimal)

```yaml
# docker-compose.yml — Phase 1: minimal infrastructure
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: skeleton
      POSTGRES_USER: redyx
      POSTGRES_PASSWORD: dev
    ports: ["5432:5432"]
    volumes: [pg-data:/var/lib/postgresql/data]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U redyx -d skeleton"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    command: redis-server --save 60 1
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

  skeleton-service:
    build:
      context: .
      dockerfile: deploy/docker/Dockerfile
      args: [SERVICE=skeleton]
    environment:
      DATABASE_URL: postgres://redyx:dev@postgres:5432/skeleton?sslmode=disable
      REDIS_URL: redis://redis:6379/0
      GRPC_PORT: "50051"
    ports: ["50051:50051"]
    depends_on:
      postgres: { condition: service_healthy }
      redis: { condition: service_healthy }

  envoy:
    image: envoyproxy/envoy:v1.37.0
    ports:
      - "8080:8080"   # API gateway
      - "9901:9901"   # Admin interface
    volumes:
      - ./deploy/envoy/envoy.yaml:/etc/envoy/envoy.yaml:ro
      - ./deploy/envoy/proto.pb:/etc/envoy/proto.pb:ro
    depends_on:
      - skeleton-service

volumes:
  pg-data:
```

### Astro Config with SSR + Svelte + Tailwind

```javascript
// Source: Astro docs — on-demand rendering + integrations
// web/astro.config.mjs
import { defineConfig } from 'astro/config';
import svelte from '@astrojs/svelte';
import node from '@astrojs/node';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
  output: 'server',
  adapter: node({
    mode: 'standalone',
  }),
  integrations: [svelte()],
  vite: {
    plugins: [tailwindcss()],
  },
});
```

### TailwindCSS Terminal Theme Configuration

```css
/* Source: TailwindCSS v4 docs — CSS-first config
   web/tailwind.css */
@import "tailwindcss";

@theme {
  --font-mono: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', ui-monospace, monospace;

  --color-accent-50: #fff7ed;
  --color-accent-100: #ffedd5;
  --color-accent-200: #fed7aa;
  --color-accent-300: #fdba74;
  --color-accent-400: #fb923c;
  --color-accent-500: #f97316;
  --color-accent-600: #ea580c;
  --color-accent-700: #c2410c;
  --color-accent-800: #9a3412;
  --color-accent-900: #7c2d12;

  --color-terminal-bg: #0a0a0a;
  --color-terminal-fg: #d4d4d4;
  --color-terminal-border: #262626;
  --color-terminal-dim: #737373;
}

/* Box-drawing border utilities */
@utility border-terminal {
  border: 1px solid var(--color-terminal-border);
}

@utility box-terminal {
  border: 1px solid var(--color-terminal-border);
  background-color: var(--color-terminal-bg);
  padding: 0.5rem;
  font-family: var(--font-mono);
}
```

## Discretion Decisions

These are areas marked as "Claude's Discretion" in CONTEXT.md. Here are my recommendations:

### JSON Field Naming: camelCase (proto3 canonical default)

**Recommendation:** Use camelCase in JSON responses. Set `preserve_proto_field_names: false` (the Envoy default).

**Rationale:** Proto3's canonical JSON mapping uses camelCase. Envoy defaults to camelCase. Fighting the defaults creates maintenance burden. The frontend should send and receive camelCase (`communityId`, `createdAt`, `commentCount`). Proto field names remain snake_case (`community_id`, `created_at`). This is the standard proto3 convention.

### Font Choice: JetBrains Mono

**Recommendation:** JetBrains Mono as primary, Fira Code as fallback.

**Rationale:** JetBrains Mono has the best readability at small sizes (important for information-dense layouts), excellent ligature support, and is free/open source. It's designed specifically for code reading, which aligns with the TUI aesthetic. Load via Bunny Fonts (privacy-friendly CDN) or self-host.

### Loading Skeletons: ASCII-Style Shimmer

**Recommendation:** Use pulsing `░` / `▒` / `▓` block characters or dashed outlines instead of smooth gradient shimmer skeletons. This matches the terminal aesthetic.

**Rationale:** Standard loading skeletons (smooth gray rectangles with gradient animation) clash with the monospace/terminal look. ASCII block characters pulsing between shades maintain the TUI feel while clearly communicating loading state.

### Error State UI: Terminal-Style Error Boxes

**Recommendation:** Error messages displayed in `[ERROR]` prefixed blocks with red/orange accent borders. Similar to CLI error output.

**Rationale:** Matches the terminal aesthetic. Format: `[ERR] 404 — Community not found` inside a border-terminal box with accent-colored left border.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| protoc + shell scripts | buf v2 workspace | buf v2 released 2024 | Single config file replaces complex protoc invocations |
| buf.yaml v1 + buf.work.yaml | buf.yaml v2 (workspace config built in) | 2024 | No separate workspace file needed; modules defined inline |
| Astro `output: 'server'` with hybrid | Astro 5 defaults to `static`, set `output: 'server'` for full SSR | Astro 5.0 (Dec 2024) | Must explicitly opt into SSR |
| TailwindCSS v3 (JS config) | TailwindCSS v4 (CSS-first config with `@theme`) | TailwindCSS v4 (2025) | Config lives in CSS file, not tailwind.config.js. Uses `@theme` directive |
| Svelte 4 (stores, reactive declarations) | Svelte 5 (runes: `$state`, `$derived`, `$effect`) | Svelte 5 (Oct 2024) | New reactivity API; `$:` syntax deprecated |
| `@astrojs/tailwind` integration | TailwindCSS v4 Vite plugin directly | 2025 | Use `@tailwindcss/vite` plugin in astro.config.mjs instead of deprecated `@astrojs/tailwind` |

**Deprecated/outdated:**
- `@astrojs/tailwind` integration: Use `@tailwindcss/vite` plugin directly with TailwindCSS v4
- `tailwind.config.js`: TailwindCSS v4 uses CSS-first config via `@theme` in your CSS file
- Svelte stores (`writable`, `readable`): Replaced by Svelte 5 runes (`$state`, `$derived`)
- buf.yaml v1 format: v2 is current; combines workspace and module config in one file

## Open Questions

1. **TailwindCSS v4 vs v3 stability**
   - What we know: TailwindCSS v4 uses CSS-first config and is the current release. The `@tailwindcss/vite` plugin replaces `@astrojs/tailwind`.
   - What's unclear: If any Astro-specific edge cases exist with TailwindCSS v4 + Svelte components
   - Recommendation: Start with v4. If any compatibility issues arise during implementation, TailwindCSS v3 is a safe fallback with minimal config changes.

2. **Proto definitions scope — stub all 12 services or just skeleton?**
   - What we know: Phase description says "proto definitions for all 12 services." Success criteria mentions "at least one test RPC."
   - What's unclear: Whether all 12 service protos need full message definitions or just service/RPC stubs
   - Recommendation: Define complete proto files for all 12 services with `google.api.http` annotations, but only the health/skeleton service needs full implementation. This front-loads the API design work and generates the complete Envoy descriptor set.

3. **Astro development server port vs Envoy**
   - What we know: Envoy serves on 8080, Astro dev server runs on 4321. In production, Envoy proxies both API calls and serves the frontend.
   - What's unclear: Whether to proxy Astro through Envoy in dev or let Astro call Envoy directly
   - Recommendation: In development, Astro dev server runs independently on port 4321 and calls Envoy at http://localhost:8080/api/v1/. In production, Envoy serves the Astro SSR app as an upstream cluster. Keep dev simple.

## Sources

### Primary (HIGH confidence)
- Buf CLI v2 buf.yaml configuration: https://buf.build/docs/configuration/v2/buf-yaml (verified 2026-03-02)
- Buf CLI v2 buf.gen.yaml configuration: https://buf.build/docs/configuration/v2/buf-gen-yaml (verified 2026-03-02)
- Envoy gRPC-JSON transcoder filter docs: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/grpc_json_transcoder_filter (verified 2026-03-02)
- Astro on-demand rendering (SSR): https://docs.astro.build/en/guides/on-demand-rendering/ (verified 2026-03-02)
- @astrojs/svelte integration: https://docs.astro.build/en/guides/integrations-guide/svelte/ (verified 2026-03-02)
- Project STACK.md: verified versions for grpc-go v1.79.1, pgx v5.8.0, go-redis v9.18.0, Envoy v1.37.0, Astro 5.x, Svelte 5.x
- Project ARCHITECTURE.md: monorepo structure, service patterns, Envoy config
- Project PITFALLS.md: Envoy transcoding pitfalls (1-3), gRPC error handling (11), proto field safety (12)

### Secondary (MEDIUM confidence)
- TailwindCSS v4 CSS-first config: based on training knowledge of TailwindCSS v4 release (2025), cross-referenced with `@tailwindcss/vite` plugin pattern
- JetBrains Mono font characteristics: well-established, widely used in developer tools

### Tertiary (LOW confidence)
- Exact Astro + TailwindCSS v4 + Svelte 5 interaction edge cases: no specific issues found, but the combination is relatively new. Validate during implementation.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all versions verified from official releases, confirmed in project STACK.md
- Architecture: HIGH — monorepo structure, Envoy config, buf config all verified against official documentation
- Pitfalls: HIGH — Envoy transcoding pitfalls verified against official Envoy docs, confirmed in project PITFALLS.md
- Frontend (Astro+Svelte+Tailwind): MEDIUM-HIGH — individual technologies verified, but TailwindCSS v4 + Astro 5 combination is newer

**Research date:** 2026-03-02
**Valid until:** 2026-04-01 (stable technologies, 30-day validity)

---
*Phase: 01-foundation-frontend-shell*
*Research completed: 2026-03-02*
