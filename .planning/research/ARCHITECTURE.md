# Architecture Research

**Domain:** Go microservices monorepo with gRPC, Envoy gateway, and multi-database backend
**Researched:** 2026-03-02
**Confidence:** HIGH (Go monorepo patterns, Envoy transcoding, Kafka topic design are well-established; verified against official Go module docs, Envoy docs, and buf.build docs)

## System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                     EXTERNAL / EDGE LAYER                           │
│  ┌────────────┐  ┌─────────────────────────────────────────────┐    │
│  │ Cloudflare │  │  Astro SSR Frontend (Svelte islands)        │    │
│  │ CDN        │──│  REST/JSON → Envoy Gateway                  │    │
│  └────────────┘  └──────────────────┬──────────────────────────┘    │
│                                     │ REST/JSON                     │
│  ┌──────────────────────────────────▼──────────────────────────┐    │
│  │                    ENVOY API GATEWAY                         │    │
│  │  • JWT validation (jwt_authn filter)                        │    │
│  │  • Rate limiting (ext rate limit + Redis)                   │    │
│  │  • gRPC-JSON transcoding (grpc_json_transcoder filter)      │    │
│  │  • TLS termination, CORS, routing                           │    │
│  └──────────┬──────────────────────────────────────────────────┘    │
│             │ gRPC (HTTP/2, proto binary)                           │
├─────────────┴──────────────────────────────────────────────────────┤
│                     SERVICE LAYER (12 Go services)                  │
│                                                                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐              │
│  │  Auth    │ │  User    │ │Community │ │  Post    │              │
│  │ Service  │ │ Service  │ │ Service  │ │ Service  │              │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘              │
│       │            │            │         (sharded)                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐              │
│  │ Comment  │ │  Vote    │ │  Search  │ │  Media   │              │
│  │ Service  │ │ Service  │ │ Service  │ │ Service  │              │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘              │
│       │            │            │            │                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐              │
│  │Notif.   │ │Rate Limit│ │  Spam    │ │Moderation│              │
│  │ Service  │ │ Service  │ │ Service  │ │ Service  │              │
│  └────┬─────┘ └──────────┘ └────┬─────┘ └────┬─────┘              │
│       │                         │            │                     │
├───────┴─────────────────────────┴────────────┴─────────────────────┤
│                     EVENT / MESSAGING LAYER                         │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    Apache Kafka                              │    │
│  │  Topics: votes, posts, comments, moderation, notifications  │    │
│  └─────────────────────────────────────────────────────────────┘    │
├─────────────────────────────────────────────────────────────────────┤
│                     DATA LAYER                                      │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐ ┌────────────┐       │
│  │pg-auth │ │pg-user │ │pg-comm │ │pg-post │ │pg-platform │       │
│  │        │ │        │ │        │ │(shard) │ │(mod/spam/  │       │
│  │        │ │        │ │        │ │        │ │notif/vote) │       │
│  └────────┘ └────────┘ └────────┘ └────────┘ └────────────┘       │
│  ┌─────────────┐ ┌───────────┐ ┌─────────────┐ ┌───────────┐      │
│  │  ScyllaDB   │ │   Redis   │ │ Meilisearch │ │  AWS S3   │      │
│  │ (comments)  │ │ (6 DBs)   │ │  (search)   │ │ (media)   │      │
│  └─────────────┘ └───────────┘ └─────────────┘ └───────────┘      │
└─────────────────────────────────────────────────────────────────────┘
```

## Recommended Monorepo Structure

This is a **single Go module** monorepo. All 12 services share one `go.mod` at the root, which simplifies dependency management, allows shared code via `internal/`, and enables atomic cross-service changes. This follows Go's official guidance for server projects with multiple commands.

```
redyx/
├── go.mod                          # Single module: github.com/[org]/redyx
├── go.sum
├── buf.yaml                        # Buf workspace config (v2 format)
├── buf.gen.yaml                    # Buf code generation config
├── buf.lock                        # Buf dependency lock
├── Makefile                        # Build, generate, lint, test targets
├── docker-compose.yml              # Local dev (all infra + services)
├── docker-compose.infra.yml        # Infrastructure only (DBs, Kafka, Redis)
│
├── proto/                          # ALL protobuf definitions
│   ├── redyx/                      # Top-level package namespace
│   │   ├── auth/v1/
│   │   │   └── auth.proto          # AuthService RPCs + messages
│   │   ├── user/v1/
│   │   │   └── user.proto          # UserService RPCs + messages
│   │   ├── community/v1/
│   │   │   └── community.proto
│   │   ├── post/v1/
│   │   │   └── post.proto
│   │   ├── comment/v1/
│   │   │   └── comment.proto
│   │   ├── vote/v1/
│   │   │   └── vote.proto
│   │   ├── search/v1/
│   │   │   └── search.proto
│   │   ├── media/v1/
│   │   │   └── media.proto
│   │   ├── notification/v1/
│   │   │   └── notification.proto
│   │   ├── moderation/v1/
│   │   │   └── moderation.proto
│   │   ├── ratelimit/v1/
│   │   │   └── ratelimit.proto
│   │   ├── spam/v1/
│   │   │   └── spam.proto
│   │   └── common/v1/
│   │       └── common.proto        # Shared types: Pagination, Timestamp, UserRef
│   └── google/api/                 # google.api.http annotations (for Envoy transcoding)
│       ├── annotations.proto
│       └── http.proto
│
├── gen/                            # GENERATED Go code (from buf generate)
│   └── redyx/                      # Mirrors proto/ structure
│       ├── auth/v1/
│       │   ├── auth.pb.go
│       │   └── auth_grpc.pb.go
│       ├── user/v1/
│       ├── community/v1/
│       ├── common/v1/
│       └── ...
│
├── cmd/                            # Service entry points (one per service)
│   ├── auth-service/
│   │   └── main.go                 # Wires up auth server, starts gRPC listener
│   ├── user-service/
│   │   └── main.go
│   ├── community-service/
│   │   └── main.go
│   ├── post-service/
│   │   └── main.go
│   ├── comment-service/
│   │   └── main.go
│   ├── vote-service/
│   │   └── main.go
│   ├── search-service/
│   │   └── main.go
│   ├── media-service/
│   │   └── main.go
│   ├── notification-service/
│   │   └── main.go
│   ├── moderation-service/
│   │   └── main.go
│   ├── ratelimit-service/
│   │   └── main.go
│   ├── spam-service/
│   │   └── main.go
│   └── migrate/
│       └── main.go                 # DB migration runner tool
│
├── internal/                       # Private packages (Go compiler enforced)
│   ├── auth/                       # Auth service business logic
│   │   ├── server.go               # gRPC server implementation
│   │   ├── service.go              # Business logic
│   │   ├── repository.go           # Data access interface
│   │   ├── repository_pg.go        # PostgreSQL implementation
│   │   └── jwt.go                  # JWT token handling
│   ├── user/                       # User service logic
│   │   ├── server.go
│   │   ├── service.go
│   │   └── repository.go
│   ├── community/                  # Community service logic
│   ├── post/                       # Post service logic (includes shard router)
│   │   ├── server.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── shard.go                # Consistent hash ring + routing
│   ├── comment/                    # Comment service logic
│   │   ├── server.go
│   │   ├── service.go
│   │   └── repository_scylla.go    # ScyllaDB implementation
│   ├── vote/
│   ├── search/
│   ├── media/
│   ├── notification/
│   │   ├── server.go
│   │   └── ws.go                   # WebSocket handler
│   ├── moderation/
│   ├── ratelimit/
│   ├── spam/
│   │
│   └── platform/                   # SHARED internal libraries
│       ├── grpcserver/             # Common gRPC server bootstrap
│       │   └── server.go           # NewServer(), health check, reflection, interceptors
│       ├── config/                 # Config loading (env vars, defaults)
│       │   └── config.go
│       ├── database/              # DB connection helpers
│       │   ├── postgres.go         # pgxpool setup
│       │   └── scylla.go           # gocql session setup
│       ├── redis/                 # Redis client setup
│       │   └── client.go
│       ├── kafka/                 # Kafka producer/consumer wrappers
│       │   ├── producer.go
│       │   └── consumer.go
│       ├── middleware/            # gRPC interceptors
│       │   ├── logging.go         # Structured logging interceptor
│       │   ├── tracing.go         # OpenTelemetry interceptor
│       │   ├── recovery.go        # Panic recovery
│       │   └── auth.go            # JWT claim extraction from metadata
│       ├── observability/         # Metrics, tracing setup
│       │   ├── metrics.go         # Prometheus registry
│       │   └── tracing.go         # OpenTelemetry provider
│       ├── errors/                # Standardized gRPC error responses
│       │   └── errors.go
│       └── pagination/            # Cursor-based pagination helpers
│           └── pagination.go
│
├── migrations/                     # SQL migration files
│   ├── auth/
│   │   ├── 001_create_credentials.up.sql
│   │   └── 001_create_credentials.down.sql
│   ├── user/
│   ├── community/
│   ├── post/
│   ├── platform/                   # Shared pg-platform schemas
│   │   ├── moderation/
│   │   ├── spam/
│   │   ├── notification/
│   │   └── vote/
│   └── scylla/
│       └── 001_create_comments.cql
│
├── deploy/                         # Deployment configs
│   ├── docker/
│   │   └── Dockerfile              # Multi-stage build (shared for all services)
│   ├── envoy/
│   │   ├── envoy.yaml              # Gateway config
│   │   └── proto.pb                # Compiled proto descriptor set (for transcoding)
│   ├── k8s/
│   │   ├── base/                   # Kustomize base
│   │   │   ├── auth-service/
│   │   │   ├── user-service/
│   │   │   └── ...
│   │   ├── overlays/
│   │   │   ├── dev/
│   │   │   ├── staging/
│   │   │   └── prod/
│   │   └── monitoring/
│   │       ├── prometheus/
│   │       ├── grafana/
│   │       ├── loki/
│   │       └── jaeger/
│   └── scripts/
│       ├── setup-local.sh          # Bootstrap local dev environment
│       └── generate-proto.sh       # Proto compilation wrapper
│
├── web/                            # Astro + Svelte frontend
│   ├── package.json
│   ├── astro.config.mjs
│   ├── src/
│   │   ├── pages/
│   │   ├── layouts/
│   │   ├── components/             # Svelte islands
│   │   └── lib/                    # API client, types
│   └── public/
│
├── scripts/                        # Dev tooling scripts
│   ├── seed.go                     # Seed test data
│   └── wait-for-it.sh              # Docker Compose service waiter
│
└── docs/                           # Existing SRS, architecture plan, diagrams
    ├── Architecture Plan.md
    ├── Software Requirement Specification Document.md
    └── Core Concepts.md
```

### Structure Rationale

- **Single `go.mod`:** One module for all services. Services import shared code as `github.com/[org]/redyx/internal/platform/...`. No multi-module complexity. Atomic refactors across services. Go's official docs explicitly recommend this pattern for server projects.
- **`proto/` at root:** All protobuf definitions in one place. Buf workspace (`buf.yaml`) points here. Services that need to call other services import the generated client stubs from `gen/`. This is the standard buf.build pattern.
- **`gen/` for generated code:** Keep generated Go code in a separate `gen/` directory, committed to git. This means consumers don't need `buf` installed to build. The `gen/` directory is regenerated via `make proto`.
- **`cmd/` per service:** Each service has a tiny `main.go` that wires dependencies and starts the gRPC server. Business logic lives in `internal/[service]/`.
- **`internal/platform/`:** Shared libraries that every service uses (gRPC bootstrap, DB connections, Kafka wrappers, middleware). The `internal/` boundary prevents external import per Go compiler rules.
- **`internal/[service]/`:** Each service's implementation isolated in its own package. Clean separation of server (gRPC handler), service (business logic), and repository (data access).
- **`migrations/` separated by service:** Each service's DB has its own migration directory. `pg-platform` has subdirectories per schema. Use `golang-migrate/migrate` as the migration tool.
- **`deploy/` not `deployments/`:** Shorter, matches common Go ecosystem convention. Contains Docker, Envoy, K8s, and monitoring configs.
- **`web/` at root:** The Astro frontend is a sibling to the Go backend. It has its own `package.json` and build toolchain. No Go code here.

## Service Integration Map

### Synchronous gRPC Dependencies (Service → Service)

These are direct gRPC calls where one service is a client of another.

```
Auth Service ← (no dependencies on other services)
    ↑ (JWT verification)
    │
User Service → Auth Service (validate tokens for account operations)
    ↑
    │
Community Service → User Service (resolve usernames, check existence)
    ↑
    │
Post Service → Community Service (verify community exists, check membership)
             → User Service (resolve author info for denormalization)
    ↑
    │
Comment Service → Post Service (verify post exists)
               → User Service (resolve author username for denormalization)
    ↑
    │
Vote Service → (none — writes to Redis/Kafka, reads validate via Redis state)
    │
Search Service → (none — reads from Meilisearch, writes from Kafka consumers)
    │
Media Service → (none — standalone S3 upload/serve)
    │
Notification Service → User Service (fetch notification preferences)
    │
Rate Limit Service → (none — Redis only, called from Envoy ext_authz)
    │
Spam Service → User Service (check account age, karma)
    │
Moderation Service → Post Service (get post details)
                   → Comment Service (get comment details)
                   → Community Service (verify mod role)
```

### Asynchronous Kafka Event Flow (Producer → Topic → Consumer)

```
┌─────────────────┐     ┌──────────────────┐     ┌──────────────────────┐
│   PRODUCERS      │     │     TOPICS        │     │      CONSUMERS       │
├─────────────────┤     ├──────────────────┤     ├──────────────────────┤
│                  │     │                  │     │                      │
│ Vote Service ───────→ │ redyx.votes.v1   │───→ │ Post Service         │
│                  │     │                  │     │ (update post score)  │
│                  │     │                  │───→ │ Comment Service      │
│                  │     │                  │     │ (update comment score│
│                  │     │                  │───→ │ User Service         │
│                  │     │                  │     │ (update karma)       │
│                  │     │                  │───→ │ Spam Service         │
│                  │     │                  │     │ (vote manipulation)  │
│                  │     │                  │     │                      │
│ Post Service ───────→ │ redyx.posts.v1   │───→ │ Search Service       │
│                  │     │                  │     │ (index/deindex)      │
│                  │     │                  │───→ │ Notification Service │
│                  │     │                  │     │ (notify followers)   │
│                  │     │                  │───→ │ Spam Service         │
│                  │     │                  │     │ (analyze content)    │
│                  │     │                  │     │                      │
│ Comment Service ────→ │ redyx.comments.v1│───→ │ Search Service       │
│                  │     │                  │───→ │ Notification Service │
│                  │     │                  │───→ │ Spam Service         │
│                  │     │                  │     │                      │
│ Community Svc ──────→ │ redyx.communities│───→ │ Search Service       │
│                  │     │  .v1             │     │ (index community)    │
│                  │     │                  │     │                      │
│ Moderation Svc ─────→ │ redyx.moderation │───→ │ Post Service         │
│                  │     │  .v1             │     │ (mark removed)       │
│                  │     │                  │───→ │ Comment Service      │
│                  │     │                  │     │ (mark removed)       │
│                  │     │                  │───→ │ Notification Service │
│                  │     │                  │     │ (notify author)      │
│                  │     │                  │───→ │ Search Service       │
│                  │     │                  │     │ (deindex content)    │
│                  │     │                  │     │                      │
│ Auth Service ───────→ │ redyx.auth.v1    │───→ │ User Service         │
│                  │     │                  │     │ (create profile on   │
│                  │     │                  │     │  registration)       │
└─────────────────┘     └──────────────────┘     └──────────────────────┘
```

### Integration Summary Table

| Service | gRPC Server | gRPC Client Of | Kafka Producer | Kafka Consumer | Databases |
|---------|-------------|----------------|----------------|----------------|-----------|
| Auth | Yes | — | `auth.v1` | — | pg-auth, Redis db2 |
| User | Yes | Auth | — | `auth.v1`, `votes.v1` | pg-user, Redis db4 |
| Community | Yes | User | `communities.v1` | — | pg-community, Redis db4 |
| Post | Yes | Community, User | `posts.v1` | `votes.v1`, `moderation.v1` | pg-post (sharded), Redis db4 |
| Comment | Yes | Post, User | `comments.v1` | `votes.v1`, `moderation.v1` | ScyllaDB, Redis db4 |
| Vote | Yes | — | `votes.v1` | — | Redis db0, pg-platform |
| Search | Yes | — | — | `posts.v1`, `comments.v1`, `communities.v1`, `moderation.v1` | Meilisearch |
| Media | Yes | — | — | — | S3 |
| Notification | Yes | User | — | `posts.v1`, `comments.v1`, `moderation.v1` | Redis db3, pg-platform |
| Rate Limit | Yes | — | — | — | Redis db1 |
| Spam | Yes | User | — | `posts.v1`, `comments.v1`, `votes.v1` | Redis db5, pg-platform |
| Moderation | Yes | Post, Comment, Community | `moderation.v1` | — | pg-platform |

## Architectural Patterns

### Pattern 1: Per-Service gRPC Server Bootstrap

**What:** Every service follows the same initialization pattern — load config, connect to databases, create service layer, register gRPC server, start health checks, wait for shutdown signal.

**When to use:** Every service's `cmd/[service]/main.go`.

**Trade-offs:** Slight code repetition in `main.go` files, but each service controls its own wiring. Shared bootstrap in `internal/platform/grpcserver/` reduces boilerplate.

**Example:**
```go
// cmd/auth-service/main.go
func main() {
    cfg := config.Load("auth")

    // Connect to databases
    db, err := database.NewPostgres(cfg.DatabaseURL)
    rdb := redis.NewClient(cfg.RedisURL, 2) // db2 for auth

    // Build service layer
    repo := auth.NewPostgresRepository(db)
    svc := auth.NewService(repo, rdb, cfg.JWTSecret)

    // Create and start gRPC server
    srv := grpcserver.New(cfg.GRPCPort,
        grpcserver.WithUnaryInterceptors(
            middleware.Logging(),
            middleware.Tracing(),
            middleware.Recovery(),
        ),
    )
    authv1.RegisterAuthServiceServer(srv.Server(), svc)

    srv.Run() // blocks until SIGTERM, handles graceful shutdown
}
```

### Pattern 2: Repository Interface Pattern for Data Access

**What:** Each service defines a Go interface for data access, with concrete implementations for PostgreSQL, ScyllaDB, or Redis. The service layer depends on the interface, not the concrete store.

**When to use:** Every service that touches a database.

**Trade-offs:** More files per service, but enables unit testing with mock repositories and makes future DB swaps possible (e.g., moving comment store).

**Example:**
```go
// internal/post/repository.go
type Repository interface {
    Create(ctx context.Context, post *Post) error
    GetByID(ctx context.Context, id string) (*Post, error)
    ListByCommunity(ctx context.Context, communityID string, cursor string, limit int) ([]*Post, string, error)
    UpdateScore(ctx context.Context, id string, delta int) error
}

// internal/post/repository_pg.go
type postgresRepository struct {
    pool *pgxpool.Pool
    shardRouter *ShardRouter  // consistent hash ring
}

func (r *postgresRepository) Create(ctx context.Context, post *Post) error {
    shard := r.shardRouter.GetShard(post.CommunityID)
    conn := shard.Pool()
    // ... insert into shard
}
```

### Pattern 3: Kafka Event Envelope Pattern

**What:** All Kafka messages use a consistent envelope format with metadata (event type, timestamp, trace ID, producer service) wrapping the domain payload. Define events in protobuf for type safety.

**When to use:** Every Kafka producer and consumer.

**Trade-offs:** Slight overhead per message, but enables consistent deserialization, tracing across async boundaries, and dead-letter-queue handling.

**Example:**
```protobuf
// proto/redyx/common/v1/event.proto
message EventEnvelope {
  string event_id = 1;          // UUID
  string event_type = 2;        // "vote.created", "post.created"
  string source_service = 3;    // "vote-service"
  google.protobuf.Timestamp occurred_at = 4;
  string trace_id = 5;          // OpenTelemetry trace propagation
  bytes payload = 6;            // Serialized domain event proto
}
```

```go
// internal/platform/kafka/producer.go
func (p *Producer) Publish(ctx context.Context, topic string, key string, event proto.Message) error {
    payload, _ := proto.Marshal(event)
    envelope := &commonv1.EventEnvelope{
        EventId:       uuid.New().String(),
        EventType:     string(event.ProtoReflect().Descriptor().FullName()),
        SourceService: p.serviceName,
        OccurredAt:    timestamppb.Now(),
        TraceId:       trace.SpanFromContext(ctx).SpanContext().TraceID().String(),
        Payload:       payload,
    }
    data, _ := proto.Marshal(envelope)
    return p.writer.WriteMessages(ctx, kafka.Message{
        Key:   []byte(key),
        Value: data,
    })
}
```

### Pattern 4: Envoy gRPC-JSON Transcoding via google.api.http Annotations

**What:** Each proto service RPC is annotated with `google.api.http` options that define the REST endpoint mapping. Envoy's `grpc_json_transcoder` filter reads a compiled proto descriptor set (`proto.pb`) and automatically converts REST/JSON requests to gRPC calls. No custom gateway code needed.

**When to use:** Every public-facing RPC that the frontend calls.

**Trade-offs:** Requires compiling and deploying the proto descriptor set alongside Envoy. Adding new endpoints requires regenerating `proto.pb`. But eliminates an entire REST gateway service.

**Example:**
```protobuf
// proto/redyx/post/v1/post.proto
import "google/api/annotations.proto";

service PostService {
  rpc CreatePost(CreatePostRequest) returns (CreatePostResponse) {
    option (google.api.http) = {
      post: "/api/v1/communities/{community_id}/posts"
      body: "*"
    };
  }

  rpc GetPost(GetPostRequest) returns (GetPostResponse) {
    option (google.api.http) = {
      get: "/api/v1/posts/{post_id}"
    };
  }

  rpc ListPosts(ListPostsRequest) returns (ListPostsResponse) {
    option (google.api.http) = {
      get: "/api/v1/communities/{community_id}/posts"
    };
  }
}
```

**Envoy configuration (key excerpt):**
```yaml
http_filters:
  - name: envoy.filters.http.jwt_authn
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.filters.http.jwt_authn.v3.JwtAuthentication
      providers:
        redyx_jwt:
          issuer: "redyx"
          local_jwks:
            filename: "/etc/envoy/jwks.json"
          forward: true
          forward_payload_header: x-jwt-payload
          claim_to_headers:
            - header_name: x-user-id
              claim_name: sub
      rules:
        - match: { prefix: "/api/v1/auth" }  # No JWT required
        - match: { prefix: "/api/v1" }
          requires: { provider_name: redyx_jwt }

  - name: envoy.filters.http.grpc_json_transcoder
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.filters.http.grpc_json_transcoder.v3.GrpcJsonTranscoder
      proto_descriptor: "/etc/envoy/proto.pb"
      services:
        - redyx.auth.v1.AuthService
        - redyx.user.v1.UserService
        - redyx.post.v1.PostService
        - redyx.community.v1.CommunityService
        - redyx.comment.v1.CommentService
        - redyx.vote.v1.VoteService
        - redyx.search.v1.SearchService
        - redyx.media.v1.MediaService
        - redyx.notification.v1.NotificationService
        - redyx.moderation.v1.ModerationService
      print_options:
        add_whitespace: true
        always_print_primitive_fields: true
        preserve_proto_field_names: false   # Use camelCase in JSON

  - name: envoy.filters.http.router
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
```

**Proto descriptor compilation (add to Makefile):**
```bash
# Compile proto descriptor set for Envoy transcoding
proto-descriptor:
	buf build -o deploy/envoy/proto.pb
```

### Pattern 5: Service Discovery via Docker Compose DNS / Kubernetes DNS

**What:** Services discover each other by hostname. In Docker Compose, service names are DNS-resolvable (`auth-service:50051`). In Kubernetes, each service gets a ClusterIP Service (`auth-service.redyx.svc.cluster.local:50051`). No service registry (Consul/etcd) needed.

**When to use:** All inter-service gRPC connections.

**Trade-offs:** Simpler than Consul-based discovery. In Docker Compose, requires all services on the same network. In K8s, just uses native DNS. For local dev, environment variables override addresses.

**Example:**
```go
// internal/platform/config/config.go
type ServiceAddresses struct {
    AuthService      string `env:"AUTH_SERVICE_ADDR"      envDefault:"auth-service:50051"`
    UserService      string `env:"USER_SERVICE_ADDR"      envDefault:"user-service:50051"`
    CommunityService string `env:"COMMUNITY_SERVICE_ADDR" envDefault:"community-service:50051"`
    PostService      string `env:"POST_SERVICE_ADDR"      envDefault:"post-service:50051"`
    CommentService   string `env:"COMMENT_SERVICE_ADDR"   envDefault:"comment-service:50051"`
}
```

## Kafka Topic Design

### Topic Naming Convention

`redyx.<domain>.v1` — versioned topics allow schema evolution.

### Topic Configuration

| Topic | Key | Partitions | Retention | Consumers |
|-------|-----|------------|-----------|-----------|
| `redyx.votes.v1` | `target_id` (post/comment ID) | 12 | 7 days | Post, Comment, User, Spam |
| `redyx.posts.v1` | `community_id` | 6 | 7 days | Search, Notification, Spam |
| `redyx.comments.v1` | `post_id` | 6 | 7 days | Search, Notification, Spam |
| `redyx.communities.v1` | `community_id` | 3 | 7 days | Search |
| `redyx.moderation.v1` | `target_id` | 3 | 30 days | Post, Comment, Notification, Search |
| `redyx.auth.v1` | `user_id` | 3 | 7 days | User |

### Consumer Group Naming

`<service-name>.<topic-name>` — e.g., `post-service.redyx.votes.v1`. This ensures each consuming service has its own offset and processes all messages independently.

### Key Design Decisions

- **Key by target entity:** Vote events keyed by `target_id` ensures all votes for one post land on the same partition, enabling ordered processing per post.
- **Proto-serialized payloads:** Use protobuf (not JSON) for Kafka message payloads. Type-safe, smaller on wire, consistent with gRPC layer.
- **Idempotent consumers:** Every consumer must handle duplicate messages (at-least-once delivery). Use `event_id` for deduplication where needed.
- **12 partitions for votes:** Highest throughput topic. Allows scaling to 12 consumer instances per consumer group.

## Database Migration Strategy

### Tool: golang-migrate/migrate

Use `golang-migrate/migrate` — the standard Go migration tool. Supports PostgreSQL (via `pgx`) and has a CLI + Go library API.

### Migration File Convention

```
migrations/<service>/<version>_<description>.<direction>.sql

Example:
migrations/auth/001_create_credentials.up.sql
migrations/auth/001_create_credentials.down.sql
migrations/auth/002_add_oauth_tokens.up.sql
migrations/auth/002_add_oauth_tokens.down.sql
```

### Per-Service Migration Execution

Each service runs its own migrations on startup (or via a separate `cmd/migrate` tool). The migration tool connects to the correct database based on the service configuration.

```go
// Run from cmd/migrate/main.go or from service startup
migrate.New("file://migrations/auth", cfg.AuthDatabaseURL)
migrate.Up()
```

### pg-platform Schema Isolation

Services sharing `pg-platform` (Moderation, Spam, Notification, Vote persistence) each use their own PostgreSQL schema:

```sql
-- migrations/platform/moderation/001_create_schema.up.sql
CREATE SCHEMA IF NOT EXISTS moderation;

CREATE TABLE moderation.mod_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    community_id UUID NOT NULL,
    moderator_id UUID NOT NULL,
    ...
);
```

### ScyllaDB Migrations

ScyllaDB doesn't have a standard migration tool. Use plain CQL scripts executed in order:

```
migrations/scylla/001_create_comments.cql
```

Execute via a script that tracks applied migrations in a `schema_version` table in ScyllaDB itself.

## Docker Compose Structure (Local Dev)

### Two-file strategy

1. **`docker-compose.infra.yml`** — Infrastructure only. Run this first, let it stabilize.
2. **`docker-compose.yml`** — Extends infra, adds all 12 services.

This supports the project's stated approach: "Start with PostgreSQL + Redis, add ScyllaDB/Kafka/Meilisearch as services need them."

### Infrastructure Compose (docker-compose.infra.yml)

```yaml
services:
  # === PostgreSQL Instances ===
  pg-auth:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: auth
      POSTGRES_USER: redyx
      POSTGRES_PASSWORD: dev
    ports: ["5432:5432"]
    volumes: [pg-auth-data:/var/lib/postgresql/data]

  pg-user:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: userdb
      POSTGRES_USER: redyx
      POSTGRES_PASSWORD: dev
    ports: ["5433:5432"]
    volumes: [pg-user-data:/var/lib/postgresql/data]

  pg-community:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: community
      POSTGRES_USER: redyx
      POSTGRES_PASSWORD: dev
    ports: ["5434:5432"]
    volumes: [pg-community-data:/var/lib/postgresql/data]

  pg-post:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: post
      POSTGRES_USER: redyx
      POSTGRES_PASSWORD: dev
    ports: ["5435:5432"]
    volumes: [pg-post-data:/var/lib/postgresql/data]
    # For dev, single instance. Sharding is app-level logic.

  pg-platform:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: platform
      POSTGRES_USER: redyx
      POSTGRES_PASSWORD: dev
    ports: ["5436:5432"]
    volumes: [pg-platform-data:/var/lib/postgresql/data]

  # === Redis ===
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]
    command: redis-server --save 60 1

  # === Kafka ===
  kafka:
    image: bitnami/kafka:3.7
    ports: ["9092:9092"]
    environment:
      KAFKA_CFG_NODE_ID: 1
      KAFKA_CFG_PROCESS_ROLES: controller,broker
      KAFKA_CFG_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
      KAFKA_CFG_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
      KAFKA_CFG_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
      KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT
      KAFKA_CFG_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_CFG_AUTO_CREATE_TOPICS_ENABLE: "true"
    # KRaft mode — no Zookeeper needed with Kafka 3.x+

  # === ScyllaDB ===
  scylladb:
    image: scylladb/scylla:5.4
    ports: ["9042:9042"]
    command: --smp 1 --memory 512M
    # Light config for dev

  # === Meilisearch ===
  meilisearch:
    image: getmeili/meilisearch:v1.7
    ports: ["7700:7700"]
    environment:
      MEILI_MASTER_KEY: dev-master-key
    volumes: [meili-data:/meili_data]

  # === MinIO (S3 compatible, local dev) ===
  minio:
    image: minio/minio:latest
    ports: ["9000:9000", "9001:9001"]
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    volumes: [minio-data:/data]

volumes:
  pg-auth-data:
  pg-user-data:
  pg-community-data:
  pg-post-data:
  pg-platform-data:
  meili-data:
  minio-data:
```

### Service Compose (docker-compose.yml)

```yaml
include:
  - docker-compose.infra.yml

services:
  envoy:
    image: envoyproxy/envoy:v1.31-latest
    ports: ["8080:8080", "9901:9901"]
    volumes:
      - ./deploy/envoy/envoy.yaml:/etc/envoy/envoy.yaml
      - ./deploy/envoy/proto.pb:/etc/envoy/proto.pb
    depends_on: [auth-service]

  auth-service:
    build:
      context: .
      dockerfile: deploy/docker/Dockerfile
      args: [SERVICE=auth-service]
    environment:
      DATABASE_URL: postgres://redyx:dev@pg-auth:5432/auth?sslmode=disable
      REDIS_URL: redis://redis:6379/2
      GRPC_PORT: "50051"
    depends_on: [pg-auth, redis]

  user-service:
    build:
      context: .
      dockerfile: deploy/docker/Dockerfile
      args: [SERVICE=user-service]
    environment:
      DATABASE_URL: postgres://redyx:dev@pg-user:5432/userdb?sslmode=disable
      REDIS_URL: redis://redis:6379/4
      KAFKA_BROKERS: kafka:9092
      AUTH_SERVICE_ADDR: auth-service:50051
      GRPC_PORT: "50051"
    depends_on: [pg-user, redis, kafka, auth-service]

  # ... similar for all 12 services
```

### Multi-stage Dockerfile (shared)

```dockerfile
# deploy/docker/Dockerfile
FROM golang:1.22-alpine AS builder
ARG SERVICE
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd/${SERVICE}

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/service /bin/service
EXPOSE 50051
ENTRYPOINT ["/bin/service"]
```

## Protobuf Management with Buf

### buf.yaml (v2 format, workspace root)

```yaml
version: v2
modules:
  - path: proto
    name: buf.build/redyx/platform  # or local-only
lint:
  use:
    - STANDARD
  except:
    - PACKAGE_VERSION_SUFFIX  # Allow google.api without version suffix
breaking:
  use:
    - WIRE_JSON  # Catch wire-breaking and JSON-breaking changes
deps:
  - buf.build/googleapis/googleapis  # For google.api.http annotations
```

### buf.gen.yaml (code generation)

```yaml
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/[org]/redyx/gen
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

### Makefile Targets

```makefile
.PHONY: proto proto-lint proto-breaking proto-descriptor

proto: proto-lint          ## Generate Go code from protos
	buf generate
	buf build -o deploy/envoy/proto.pb

proto-lint:                ## Lint proto files
	buf lint

proto-breaking:            ## Check for breaking changes
	buf breaking --against '.git#subdir=proto'

proto-descriptor:          ## Build Envoy descriptor set only
	buf build -o deploy/envoy/proto.pb
```

## Service Build Order

Based on the dependency graph, services should be built and brought online in this order:

### Phase 1: Foundation (no service dependencies)

| Order | Service | Why First | Dependencies |
|-------|---------|-----------|--------------|
| 1 | **Shared platform libraries** | Every service imports `internal/platform/*` | None |
| 2 | **Proto definitions + codegen** | Every service needs generated stubs | None |
| 3 | **Auth Service** | Issues JWTs; every authenticated request depends on tokens existing | pg-auth, Redis |

### Phase 2: Core Identity + Structure

| Order | Service | Why Now | Dependencies |
|-------|---------|--------|--------------|
| 4 | **User Service** | Profiles needed by Community, Post, Comment for denormalization | pg-user, Redis, Kafka (consumer: auth events) |
| 5 | **Community Service** | Posts belong to communities; can't create posts without communities | pg-community, Redis, User Service (gRPC) |

### Phase 3: Content Creation

| Order | Service | Why Now | Dependencies |
|-------|---------|--------|--------------|
| 6 | **Post Service** | Core content. Needs communities to exist. | pg-post (sharded), Redis, Kafka, Community + User (gRPC) |
| 7 | **Comment Service** | Comments attach to posts. | ScyllaDB, Kafka, Post + User (gRPC) |
| 8 | **Vote Service** | Votes attach to posts and comments. | Redis, Kafka, pg-platform |

### Phase 4: Supporting Services

| Order | Service | Why Now | Dependencies |
|-------|---------|--------|--------------|
| 9 | **Search Service** | Needs content to exist to index. Kafka consumer only. | Meilisearch, Kafka |
| 10 | **Notification Service** | Needs content events to notify about. | Redis, Kafka, pg-platform, User (gRPC) |
| 11 | **Media Service** | Standalone upload pipeline. Can be added whenever media posts are needed. | S3/MinIO |

### Phase 5: Safety + Moderation

| Order | Service | Why Now | Dependencies |
|-------|---------|--------|--------------|
| 12 | **Rate Limit Service** | Protects all endpoints. Add after core services work. | Redis |
| 13 | **Spam Service** | Analyzes content. Needs content flow to exist. | Redis, Kafka, pg-platform, User (gRPC) |
| 14 | **Moderation Service** | Mod tools for existing content. | pg-platform, Post + Comment + Community (gRPC), Kafka |

### Phase 6: Gateway + Frontend

| Order | Component | Why Now | Dependencies |
|-------|-----------|--------|--------------|
| 15 | **Envoy Gateway** | Configure transcoding, JWT validation, rate limiting after services stabilize. | All services, proto.pb descriptor |
| 16 | **Astro Frontend** | Build against the REST API that Envoy exposes. | Envoy gateway |

### Phase 7: Infrastructure + Observability

| Order | Component | Why Now | Dependencies |
|-------|-----------|--------|--------------|
| 17 | **Kubernetes manifests** | Containerize and orchestrate after services work locally. | Docker images for all services |
| 18 | **Monitoring stack** | Prometheus, Grafana, Loki, Jaeger. Add after K8s deployment works. | K8s cluster |

## Data Flow Examples

### User Creates a Post

```
Browser → POST /api/v1/communities/{id}/posts (JSON)
    ↓
Envoy Gateway
    │ 1. JWT validation (jwt_authn filter) → extracts user_id to x-user-id header
    │ 2. Rate limit check (ext_authz → rate-limit-service → Redis db1)
    │ 3. gRPC-JSON transcoding → PostService.CreatePost (gRPC)
    ↓
Post Service (gRPC)
    │ 4. Extract user_id from gRPC metadata (set by Envoy)
    │ 5. gRPC call → Community Service: verify community exists + user is member
    │ 6. gRPC call → Spam Service: pre-publish content check
    │ 7. Determine shard: hash(community_id) → shard ring → target PG
    │ 8. INSERT into pg-post-shardN
    │ 9. Publish PostCreated event → Kafka topic: redyx.posts.v1
    ↓
Kafka consumers (async, parallel):
    │ 10. Search Service → index post in Meilisearch
    │ 11. Notification Service → notify community followers (if opted in)
    │ 12. Spam Service → async content analysis + behavior scoring
    ↓
Response ← 201 Created (JSON via Envoy transcoding)
```

### User Upvotes a Post

```
Browser → POST /api/v1/posts/{id}/vote (JSON body: {direction: "up"})
    ↓
Envoy → JWT validation → Rate limit → gRPC-JSON transcode
    ↓
Vote Service (gRPC)
    │ 1. Check Redis db0: has user already voted on this target?
    │    - If same direction → return (idempotent, no-op)
    │    - If opposite direction → flip vote
    │    - If new → record vote
    │ 2. SETEX vote state in Redis db0: "vote:{user_id}:{target_id}" = "up"
    │ 3. INCR/DECR score in Redis db0: "score:{target_id}"
    │ 4. Publish VoteCreated event → Kafka: redyx.votes.v1 (key: post_id)
    ↓
Kafka consumers:
    │ 5. Post Service → UPDATE posts SET score = score + 1 WHERE id = ?
    │ 6. User Service → UPDATE users SET karma = karma + 1 WHERE id = author_id
    │ 7. Spam Service → record vote timing, check for coordinated patterns
    ↓
Response ← 200 OK {score: 42, user_vote: "up"}
```

## Anti-Patterns

### Anti-Pattern 1: Shared Database Across Services

**What people do:** Multiple services directly query each other's PostgreSQL tables.
**Why it's wrong:** Couples services at the data layer. Schema changes in one service break others. Violates the "each service owns its data" constraint.
**Do this instead:** Services communicate via gRPC (sync) or Kafka events (async). If service B needs data from service A's DB, service A exposes a gRPC endpoint.

### Anti-Pattern 2: Synchronous Chains for Event Processing

**What people do:** Post creation synchronously calls Search → Notification → Spam via gRPC, waiting for each to complete.
**Why it's wrong:** Creates a fragile chain. If Search is down, post creation fails. Adds latency proportional to the number of downstream consumers.
**Do this instead:** Publish a Kafka event. Consumers process independently. Post creation succeeds even if Search is temporarily down.

### Anti-Pattern 3: Fat Proto Files (God Proto)

**What people do:** Put all message types and all service RPCs in one massive `.proto` file.
**Why it's wrong:** Every change to any type regenerates all code. Makes it impossible to identify which service owns which type. Merge conflicts constantly.
**Do this instead:** One `.proto` file per service (in `proto/redyx/<service>/v1/`). Shared types in `proto/redyx/common/v1/`. Import what you need.

### Anti-Pattern 4: Blocking Kafka Consumers in the gRPC Request Path

**What people do:** A service that's a Kafka consumer blocks its gRPC handler waiting for Kafka events.
**Why it's wrong:** Kafka consumption is asynchronous. gRPC handlers should respond quickly. Mixing them creates deadlocks and timeouts.
**Do this instead:** Run Kafka consumers in separate goroutines from the gRPC server. They process events independently and update the service's own data store.

### Anti-Pattern 5: N+1 Cross-Service Calls

**What people do:** To render a post feed, call User Service once per post to get author info (10 posts = 10 gRPC calls).
**Why it's wrong:** Latency multiplied by N. Cascading load on User Service.
**Do this instead:** Denormalize author username into the post record at write time. Or use a batch gRPC endpoint (`GetUsers(ids: [])`) to fetch all needed users in one call.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| Dev / 0-100 users | Docker Compose, single PG per service, single-node Kafka, single Redis. Sharding logic exists but routes to 1 shard. |
| 100-10K users | K8s with 2-3 replicas per service. Add Redis read replicas for caching. Enable HPA on Vote and Post services. |
| 10K-100K users | Add post shards (2-4 PG instances). Enable Kafka partition scaling. ScyllaDB multi-node cluster. Meilisearch replicas. |
| 100K+ users | Full sharding (8+ shards). Dedicated Redis clusters per concern. Kafka partitions scaled to match consumer count. CDN for media. Consider splitting pg-platform into dedicated instances. |

### Scaling Priorities (What Breaks First)

1. **Vote Service / Redis:** First bottleneck. Thousands of votes per second. Mitigate with Redis Cluster and more Kafka partitions.
2. **Post Service (feed queries):** Feed generation across subscriptions is expensive. Mitigate with aggressive Redis caching (hot feeds, 5 min TTL) and read replicas.
3. **Comment Service / ScyllaDB writes:** High-traffic posts generate massive comment volumes. ScyllaDB handles this well but needs multi-node cluster past 10K users.
4. **Kafka consumer lag:** If consumers fall behind, search/notifications delay. Mitigate by adding consumer instances (up to partition count).

## Sources

- Go official module layout guidance: https://go.dev/doc/modules/layout (HIGH confidence — official Go docs)
- golang-standards/project-layout: https://github.com/golang-standards/project-layout (MEDIUM confidence — community convention, not official standard)
- Envoy gRPC-JSON transcoder filter: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/grpc_json_transcoder_filter (HIGH confidence — official Envoy docs)
- Envoy JWT authentication filter: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/jwt_authn_filter (HIGH confidence — official Envoy docs)
- Buf CLI quickstart and workspace configuration: https://buf.build/docs/cli/quickstart (HIGH confidence — official buf.build docs)
- google.api.http annotations for REST mapping: https://cloud.google.com/service-management/reference/rpc/google.api#http (HIGH confidence — Google API design guide)
- Existing project architecture plan: `docs/Architecture Plan.md` (HIGH confidence — project-specific)

---
*Architecture research for: Redyx — Go microservices monorepo with gRPC, Envoy, Kafka*
*Researched: 2026-03-02*
