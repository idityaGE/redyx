# Redyx

## What This Is

Redyx is an anonymous, community-driven discussion platform modeled after Reddit. Users create communities, post content (text, links, media), comment in nested threads, and vote on everything. The system runs on a microservice architecture built with Go, communicating over gRPC internally and REST/JSON externally, deployed behind an Envoy API gateway. The frontend uses Astro (SSR) with Svelte islands for interactive components.

The distinguishing goal is anonymity. Users interact through pseudonymous usernames. The platform collects minimal personal information and never exposes private data to other users.

## Core Value

Users can anonymously create communities, post content, and have threaded discussions — with minimal personal data collected and maximum privacy preserved.

## Current Milestone: v1.0 Redyx

**Goal:** Build the full Redyx platform — all 12 microservices, the Astro+Svelte frontend, and Kubernetes deployment — as a working end-to-end demo.

**Target features:**
- Authentication (email/password, Google OAuth, OTP verification, JWT tokens)
- User profiles with karma tracking and account deletion
- Community creation, membership, roles, and settings
- Text, link, and media posts with sharded storage
- Nested threaded comments (ScyllaDB)
- Upvote/downvote system with async karma updates via Kafka
- Full-text search (Meilisearch)
- Real-time WebSocket notifications
- Image/video upload and CDN delivery (S3 + CloudFront)
- Community moderation tools (remove, ban, pin, mod queue/log)
- Rate limiting (tiered, action-specific)
- Spam/abuse detection (pre-publish filtering, post-publish analysis)
- Kubernetes deployment with monitoring (Prometheus, Grafana, Loki, Jaeger)

## Requirements

### Validated

<!-- Shipped and confirmed valuable. -->

(None yet — ship to validate)

### Active

<!-- Current scope. Building toward these. -->

- [ ] 12 Go microservices communicating over gRPC
- [ ] Envoy API gateway with REST-to-gRPC translation
- [ ] Astro SSR frontend with Svelte interactive islands
- [ ] Email/password + Google OAuth authentication with OTP verification
- [ ] Community CRUD with membership, roles, visibility settings
- [ ] Text, link, and media posts with sharded PostgreSQL storage
- [ ] Nested comment threads in ScyllaDB
- [ ] Upvote/downvote with Redis real-time counts and Kafka event processing
- [ ] Full-text search via Meilisearch
- [ ] Real-time WebSocket notifications
- [ ] Media upload pipeline (validation, scanning, S3, CDN)
- [ ] Moderation tools (remove, ban, pin, mod queue, mod log)
- [ ] Rate limiting (token bucket + sliding window, tiered)
- [ ] Spam/abuse detection (keyword filter, URL check, duplicate detection, behavior scoring)
- [ ] Kubernetes deployment with HPA, health checks, namespace isolation
- [ ] Observability: Prometheus metrics, Grafana dashboards, Loki logs, Jaeger traces

### Out of Scope

<!-- Explicit boundaries. Includes reasoning to prevent re-adding. -->

- Shadow-banning (P3) — Complex feature, defer to v1.1
- Vote manipulation detection via cluster analysis (P3) — Requires significant data before useful
- Real-time chat / direct messaging — Not in SRS, high complexity, not core to community value
- Mobile native app — Web-first approach, mobile later
- OAuth providers beyond Google — Google sufficient for v1
- Flairs/tags system — P3, low priority

## Context

- **Existing artifacts:** Complete SRS (IEEE 830), architecture plan, 45 user stories with acceptance criteria, UML diagrams (activity, class, data flow, sequence, use case)
- **Tech ecosystem:** Go microservices, gRPC + Protocol Buffers, Envoy gateway, Astro + Svelte frontend
- **Database strategy:** 5 PostgreSQL instances (auth, user, community, post-sharded, platform-shared), 1 ScyllaDB cluster (comments), 1 Redis cluster (6 logical DBs), 1 Meilisearch, 1 S3 bucket
- **Development approach:** Docker Compose for local dev, services built first, K8s deployment as final phase. Start with PostgreSQL + Redis, add ScyllaDB/Kafka/Meilisearch as services need them
- **Priority scope:** P0 + P1 + P2 features included. P3 deferred.
- **Estimated effort:** ~65 dev days across 45 user stories

## Constraints

- **Tech stack**: Go (backend), Astro + Svelte (frontend), gRPC (internal), REST/JSON (external) — per SRS specification
- **Anonymity**: User privacy is a hard constraint — no PII exposure through any API, emails encrypted at rest, IPs never stored in app DBs
- **Database per service**: Each service owns its data — no shared databases except pg-platform (low-volume services)
- **Sharding**: Post service uses application-level consistent hashing on community_id — required by design
- **Infrastructure**: Docker Compose for development, Kubernetes for deployment
- **Security**: argon2id passwords, AES-256 email encryption, mTLS between services, JWT with 15min/7day tokens

## Key Decisions

<!-- Decisions that constrain future work. Add throughout project lifecycle. -->

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go for all backend services | Performance, concurrency, strong gRPC support | — Pending |
| ScyllaDB for comments (not PostgreSQL) | High write throughput, wide-column model suits comment trees | — Pending |
| Application-level sharding (not database-level) | Control over routing, consistent hashing on community_id | — Pending |
| Envoy gateway (not custom Go gateway) | REST-to-gRPC transcoding, rate limiting, TLS termination built-in | — Pending |
| Astro + Svelte (not React/Next.js) | Content-driven SSR, minimal JS shipped, Svelte islands for interactivity | — Pending |
| Kafka for async events (not RabbitMQ) | Durable event log, multi-consumer fan-out, replay capability | — Pending |
| Docker Compose for dev, K8s for prod | Simpler local dev, production-grade orchestration | — Pending |
| Start minimal DBs, add as needed | Avoid running 9 data stores from day one | — Pending |

---
*Last updated: 2026-03-02 after milestone v1.0 initialization*
