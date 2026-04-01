---
phase: 07-deployment-observability
plan: 04
subsystem: infra
tags: [prometheus, opentelemetry, tracing, metrics, observability, grpc-prometheus, otelgrpc]

# Dependency graph
requires:
  - phase: 07-01
    provides: K8s deployment manifests for all services
  - phase: 07-03
    provides: Prometheus, Grafana, Jaeger monitoring stack
provides:
  - Observability package with Prometheus metrics and OTEL tracing initialization
  - grpcserver WithServerOptions for stats handler injection
  - Trace context (trace_id, span_id) in logging middleware
  - All 12 services instrumented with metrics and tracing
affects: [07-05, 07-06]

# Tech tracking
tech-stack:
  added: [grpc-prometheus, opentelemetry-go, otelgrpc, otlptracegrpc, promhttp]
  patterns: [observability package initialization pattern, trace context logging, stats handler injection]

key-files:
  created:
    - internal/platform/observability/metrics.go
    - internal/platform/observability/tracing.go
  modified:
    - internal/platform/grpcserver/server.go
    - internal/platform/middleware/logging.go
    - cmd/auth/main.go
    - cmd/user/main.go
    - cmd/community/main.go
    - cmd/post/main.go
    - cmd/vote/main.go
    - cmd/comment/main.go
    - cmd/search/main.go
    - cmd/notification/main.go
    - cmd/media/main.go
    - cmd/moderation/main.go
    - cmd/spam/main.go
    - cmd/skeleton/main.go

key-decisions:
  - "Metrics HTTP server on METRICS_PORT (default 9090) for Prometheus scraping"
  - "Tracing optional via OTEL_EXPORTER_OTLP_ENDPOINT env var (nil if not set)"
  - "grpc.StatsHandler used for OTEL tracing injection"
  - "Trace context extracted via trace.SpanContextFromContext in logging middleware"
  - "Stream interceptors added alongside unary interceptors for completeness"

patterns-established:
  - "Observability init pattern: InitMetrics(logger) + InitTracing(ctx, logger) in main.go before gRPC server"
  - "WithServerOptions() for injecting grpc.ServerOption (like stats handler) into grpcserver.New()"
  - "Trace context logging: spanCtx.TraceID().String() and spanCtx.SpanID().String() as zap fields"

requirements-completed: [INFRA-03, INFRA-06]

# Metrics
duration: 8min
completed: 2026-04-01
---

# Phase 07-04: Service Instrumentation Summary

**Prometheus metrics and OpenTelemetry tracing instrumentation for all 12 Go services with trace context in logs**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-01T04:00:00Z
- **Completed:** 2026-04-01T04:08:00Z
- **Tasks:** 3
- **Files modified:** 14

## Accomplishments
- Created observability package with InitMetrics (Prometheus) and InitTracing (OTEL) functions
- Extended grpcserver with WithServerOptions() for stats handler injection
- Added trace_id and span_id to all log entries via logging middleware
- Instrumented all 12 services: auth, user, community, post, vote, comment, search, notification, media, moderation, spam, skeleton

## Task Commits

Each task was committed atomically:

1. **Task 1: Create observability package** - `74da3b3` (feat)
2. **Task 2: Update grpcserver and logging middleware** - `20bad54` (feat)
3. **Task 3: Update all 12 service main.go files** - `182bc60` (feat)

**Plan metadata:** TBD (docs: complete plan)

## Files Created/Modified

### Created
- `internal/platform/observability/metrics.go` - Prometheus metrics initialization with grpc-prometheus interceptors
- `internal/platform/observability/tracing.go` - OTEL tracer initialization with OTLP gRPC exporter

### Modified
- `internal/platform/grpcserver/server.go` - Added WithServerOptions() for stats handler injection
- `internal/platform/middleware/logging.go` - Added trace context extraction (trace_id, span_id)
- `cmd/auth/main.go` - Observability initialization + interceptors
- `cmd/user/main.go` - Observability initialization + interceptors
- `cmd/community/main.go` - Observability initialization + interceptors
- `cmd/post/main.go` - Observability initialization + interceptors
- `cmd/vote/main.go` - Observability initialization + interceptors
- `cmd/comment/main.go` - Observability initialization + interceptors
- `cmd/search/main.go` - Observability initialization + interceptors
- `cmd/notification/main.go` - Observability initialization + interceptors
- `cmd/media/main.go` - Observability initialization + interceptors
- `cmd/moderation/main.go` - Observability initialization + interceptors
- `cmd/spam/main.go` - Observability initialization + interceptors
- `cmd/skeleton/main.go` - Observability initialization + interceptors

## Decisions Made

1. **Metrics port from env (METRICS_PORT)** - Default 9090, allows per-service override in K8s
2. **Tracing optional** - Returns nil if OTEL_EXPORTER_OTLP_ENDPOINT not set (graceful degradation)
3. **Stats handler via WithServerOptions** - Added new grpcserver option rather than modifying New() signature
4. **Stream interceptors included** - Added StreamInterceptor for completeness even though no streaming RPCs currently
5. **Metrics interceptor first in chain** - Runs before other interceptors to capture full request lifecycle

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - all services followed the same pattern and compiled without issues.

## User Setup Required

None - observability is configured via environment variables already in K8s manifests:
- `METRICS_PORT` - Prometheus scrape port (default 9090)
- `OTEL_EXPORTER_OTLP_ENDPOINT` - Jaeger OTLP collector endpoint
- `OTEL_SERVICE_NAME` - Service name for traces

## Next Phase Readiness

- All services export Prometheus metrics on /metrics endpoint
- All services emit OpenTelemetry traces when OTEL_EXPORTER_OTLP_ENDPOINT is set
- All logs include trace_id and span_id when in trace context
- Ready for Plan 05 (Service Mesh / Linkerd) or Plan 06 (Chaos Testing)

## Self-Check: PASSED

All files and commits verified:
- FOUND: internal/platform/observability/metrics.go
- FOUND: internal/platform/observability/tracing.go
- FOUND: 74da3b3 (Task 1)
- FOUND: 20bad54 (Task 2)
- FOUND: 182bc60 (Task 3)

---
*Phase: 07-deployment-observability*
*Completed: 2026-04-01*
