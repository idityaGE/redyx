# Phase 7: Deployment + Observability - Context

**Gathered:** 2026-03-31
**Status:** Ready for planning

<domain>
## Phase Boundary

Deploy all 12 services and supporting data stores to Kubernetes with full observability. The platform runs in a kind cluster locally with HPA, readiness/liveness probes, and namespace isolation. Prometheus collects metrics from every service, Grafana displays per-service and global dashboards, Loki aggregates structured JSON logs, and Jaeger visualizes distributed traces. Frontend performance optimization (FEND-04) is skipped per user decision to focus on infrastructure.

Requirements: INFRA-02, INFRA-03, INFRA-04, INFRA-05, INFRA-06

</domain>

<decisions>
## Implementation Decisions

### Kubernetes deployment approach
- **Local environment:** kind (Kubernetes IN Docker) for development and testing
- **Namespace strategy:** Logical grouping with 3 namespaces:
  - `redyx-app` for the 12 microservices
  - `redyx-data` for stateful workloads (PostgreSQL, Redis, ScyllaDB, Kafka, Meilisearch, MinIO)
  - `redyx-monitoring` for observability stack (Prometheus, Grafana, Loki, Jaeger)
- **HPA configuration:** Scale on both CPU and memory thresholds
- **Secret management:** Native Kubernetes Secrets (base64-encoded in YAML, gitignored) for v1
- **Data stores:** Deploy using Helm charts (Bitnami/official) for PostgreSQL, Redis, ScyllaDB, Kafka, Meilisearch, MinIO
- **Service manifests:** Helm chart for the 12 microservices with values.yaml per environment
- **Ingress:** NGINX Ingress Controller for external traffic
- **Gateway architecture:** NGINX ingress external, Envoy remains internal API gateway for REST-to-gRPC transcoding

### Metrics & dashboards
- **Metrics export:** promgrpc library for automatic gRPC metrics (call count, latency histograms, error rates)
- **Dashboard organization:** 1 global overview dashboard + 1 dashboard per service (12 service dashboards)
- **Global dashboard metrics:**
  - Request rate & latency (gRPC calls/sec, p50/p95/p99 latency, error rate by method)
  - Resource utilization (CPU, memory, goroutines, GC pause per pod)
  - Data store health (PostgreSQL connections, Redis hit rate, Kafka consumer lag)
  - Client-facing metrics (HTTP requests, WebSocket connections, error codes)
- **Alerting:** No alerting for v1 — focus on dashboards only

### Logging & tracing
- **Log format:** JSON via zap production encoder (already in use) — Loki-ready
- **Log retention:** 7 days in Loki
- **Trace coverage:** Full instrumentation (gRPC calls + Kafka + database operations)
- **Trace delivery:** Direct to Jaeger (no OTel Collector for v1)

### Dev-to-K8s workflow
- **Image loading:** `kind load docker-image` — build locally, load into kind cluster, no registry needed
- **Deploy commands:** Makefile targets (`make k8s-up`, `make k8s-down`) consistent with existing docker-compose pattern
- **Rebuild flow:** Per-service incremental rebuild (build single image, reload into kind, restart pod)

### Observability UI access
- **UI exposure:** `kubectl port-forward` on demand — no persistent ingress, most secure
- **Grafana auth:** Default admin/admin for local dev
- **Dashboard management:** GitOps — dashboards as JSON files in repo, imported via Grafana provisioning

### OpenCode's Discretion
- Exact Helm chart versions and value overrides
- HPA min/max replica counts and threshold percentages
- Prometheus scrape intervals and retention period
- Per-service dashboard panel layout and queries
- Jaeger trace sampling rate (if needed for performance)
- Loki chunk and ingestion configuration
- NGINX Ingress configuration details

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `docker-compose.yml`: Complete 12-service orchestration — all environment variables, ports, dependencies defined. K8s manifests can mirror this structure.
- `deploy/docker/Dockerfile`: Multi-stage Go build already works — can be reused for K8s with minor tweaks.
- `deploy/envoy/envoy.yaml`: Envoy config with REST-to-gRPC transcoding — needs adaptation for K8s internal service discovery (DNS names change).
- `internal/platform/middleware/recovery.go`: Uses zap for structured logging — add trace context propagation here.
- `internal/platform/grpcserver/`: gRPC server bootstrap — add Prometheus metrics interceptor here.

### Established Patterns
- **Service bootstrap:** Each service in `cmd/{service}/main.go` uses shared platform libraries
- **Health checks:** All services have gRPC health service (`grpc_health_v1`)
- **Configuration:** Environment variables with defaults — needs K8s ConfigMaps
- **Logging:** zap logger initialized in each service — add OTEL trace context fields
- **Port assignment:** gRPC ports 50051-50062 + WebSocket 8081 — already defined

### Integration Points
- `internal/platform/grpcserver/server.go`: Add promgrpc metrics interceptor to middleware chain
- `internal/platform/middleware/logging.go`: Add trace ID to log fields
- Each `cmd/{service}/main.go`: Add OTEL tracer initialization
- `deploy/k8s/` (new): Helm chart for services + values files

</code_context>

<specifics>
## Specific Ideas

- Keep Envoy inside the cluster for REST-to-gRPC transcoding — don't try to replace it with NGINX
- Makefile targets should feel like the docker-compose workflow: `make k8s-up` should be as simple as `docker-compose up`
- Dashboards should be version-controlled — no manual dashboard creation that gets lost
- Port-forward for UIs is acceptable for v1 — production ingress for observability tools is future work

</specifics>

<deferred>
## Deferred Ideas

- **FEND-04 (Frontend performance):** User chose not to discuss — skipped for Phase 7. Can be addressed separately or added to backlog.
- **Alerting:** Rules and Alertmanager setup deferred to post-v1.
- **OTel Collector:** Direct Jaeger export for v1 — collector can be added later for multi-backend support.
- **Storage configuration:** PVC sizing and storage classes not discussed — OpenCode's discretion for v1.
- **Distributed context propagation:** Trace context across Kafka not explicitly discussed — covered under "full instrumentation" decision.

</deferred>

---

*Phase: 07-deployment-observability*
*Context gathered: 2026-03-31*
