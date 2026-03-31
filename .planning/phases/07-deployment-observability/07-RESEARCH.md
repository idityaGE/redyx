# Phase 7: Deployment + Observability - Research

**Completed:** 2026-03-31
**Duration:** Level 2 Standard Research

## Standard Stack

| Concern | Solution | Why |
|---------|----------|-----|
| Local K8s | kind v0.24+ | Kubernetes IN Docker — zero cloud dependencies, fast cluster creation, already Docker-based |
| Ingress | NGINX Ingress Controller | Industry standard, simple config, LoadBalancer for kind with metallb not needed |
| Metrics | Prometheus + promgrpc | Native Go gRPC metrics library, automatic histogram/counter generation |
| Dashboards | Grafana 11 | JSON-as-code provisioning, native Prometheus support |
| Logs | Loki + Promtail | Log aggregation designed for Prometheus ecosystem, native Grafana integration |
| Tracing | Jaeger + OTEL SDK | Direct OTLP export, no collector needed for v1, trace ID propagation |
| Helm charts | Bitnami where available | Production-tested charts with sane defaults for PostgreSQL, Redis, Kafka, Meilisearch |

## Validation Architecture

### Contract Anchors

| Observable Behavior | Verification Method |
|---------------------|---------------------|
| All 12 services running in kind | `kubectl get pods -n redyx-app` shows 12/12 Ready |
| HPA scales on load | `kubectl get hpa -n redyx-app` shows replicas > min |
| Prometheus scrapes all targets | `curl localhost:9090/api/v1/targets` shows all UP |
| Grafana dashboards load | `curl localhost:3000/api/search` returns 13 dashboards |
| Loki ingests logs | `logcli query '{namespace="redyx-app"}'` returns logs |
| Jaeger receives traces | `curl localhost:16686/api/traces?service=auth-service` returns traces |

### Test Strategy

| Category | Approach | Tools |
|----------|----------|-------|
| K8s manifests | Schema validation + dry-run | `kubectl apply --dry-run=server` |
| Service health | Readiness probe verification | `kubectl wait --for=condition=ready` |
| Metrics export | Scrape endpoint check | `curl /metrics` on each service |
| Log format | JSON structure validation | `kubectl logs | jq .` |
| Trace propagation | End-to-end request with trace context | `curl -H 'traceparent:...'` |

### Verification Gates

1. **Pre-deploy**: All manifests pass `--dry-run=server`, images exist in kind
2. **Post-deploy**: All pods Ready, all probes passing, services reachable via ClusterIP
3. **Observability**: Prometheus targets UP, Grafana dashboards imported, Loki receiving logs, Jaeger receiving traces

## Architecture Patterns

### Service Mesh (NOT NEEDED)
Per user decision, Envoy remains internal API gateway. No istio/linkerd required — direct gRPC between services.

### Namespace Strategy
```
redyx-app        # 12 microservices + Envoy
redyx-data       # PostgreSQL, Redis, ScyllaDB, Kafka, Meilisearch, MinIO
redyx-monitoring # Prometheus, Grafana, Loki, Promtail, Jaeger
```

### Image Loading Workflow
```bash
# Build locally (no registry needed)
docker build -t redyx/auth-service:dev -f deploy/docker/Dockerfile --build-arg SERVICE=auth .
# Load into kind cluster
kind load docker-image redyx/auth-service:dev --name redyx
# Apply/restart deployment
kubectl rollout restart deployment/auth-service -n redyx-app
```

### Helm Chart Structure
```
deploy/k8s/
├── charts/
│   └── redyx-services/          # Umbrella chart for all 12 services
│       ├── Chart.yaml
│       ├── values.yaml           # Default values
│       ├── values-dev.yaml       # Local kind overrides
│       └── templates/
│           ├── deployment.yaml
│           ├── service.yaml
│           ├── hpa.yaml
│           └── configmap.yaml
├── data/                         # Data store Helm values
│   ├── postgresql.yaml
│   ├── redis.yaml
│   ├── scylladb.yaml
│   ├── kafka.yaml
│   ├── meilisearch.yaml
│   └── minio.yaml
└── monitoring/                   # Observability Helm values
    ├── prometheus.yaml
    ├── grafana.yaml
    ├── loki.yaml
    └── jaeger.yaml
```

## Integration Points

### Go Service Changes Required

**1. Prometheus Metrics (promgrpc library)**
```go
import "github.com/grpc-ecosystem/go-grpc-prometheus"

// In grpcserver.New()
grpcMetrics := grpc_prometheus.NewServerMetrics()
grpcMetrics.EnableHandlingTimeHistogram()
serverOpts = append(serverOpts, grpc.ChainUnaryInterceptor(grpcMetrics.UnaryServerInterceptor()))

// HTTP endpoint for /metrics
http.Handle("/metrics", promhttp.Handler())
```

**2. OpenTelemetry Tracing**
```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

// In main() - init tracer
exporter, _ := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")))
tp := trace.NewTracerProvider(trace.WithBatcher(exporter))
otel.SetTracerProvider(tp)

// gRPC interceptor
serverOpts = append(serverOpts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
```

**3. Structured Logging with Trace Context**
```go
import "go.opentelemetry.io/otel/trace"

// In logging middleware
spanCtx := trace.SpanContextFromContext(ctx)
if spanCtx.IsValid() {
    logger = logger.With(
        zap.String("trace_id", spanCtx.TraceID().String()),
        zap.String("span_id", spanCtx.SpanID().String()),
    )
}
```

### New Environment Variables
```yaml
# For all services
OTEL_EXPORTER_OTLP_ENDPOINT: "jaeger.redyx-monitoring.svc:4317"
OTEL_SERVICE_NAME: "${SERVICE_NAME}"
METRICS_PORT: "9090"
```

### Kubernetes Manifest Patterns

**Deployment with probes:**
```yaml
readinessProbe:
  grpc:
    port: 50052
  initialDelaySeconds: 5
  periodSeconds: 10
livenessProbe:
  grpc:
    port: 50052
  initialDelaySeconds: 15
  periodSeconds: 20
```

**HPA:**
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
spec:
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

**ServiceMonitor (for Prometheus Operator):**
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
spec:
  selector:
    matchLabels:
      app: auth-service
  endpoints:
  - port: metrics
    interval: 15s
```

## Don't Hand-Roll

| Concern | Use Instead | Why |
|---------|-------------|-----|
| Container orchestration | Kind/K8s APIs | Docker Compose works but K8s abstractions are native |
| Metrics format | promgrpc histograms | Custom metrics lack conventions, histograms auto-calculate percentiles |
| Log shipping | Promtail DaemonSet | Container stdout → Loki native path |
| Trace context propagation | OTEL SDK | W3C traceparent header handling, automatic instrumentation |
| Dashboard provisioning | Grafana JSON + ConfigMap | Manual dashboards get lost |

## Common Pitfalls

### 1. Kind Load Balancer
**Problem:** `kind` clusters don't have cloud load balancers.
**Solution:** Use NodePort or `kubectl port-forward` for external access. No LoadBalancer type services.

### 2. Image Pull Failures
**Problem:** K8s tries to pull from registry but images are local.
**Solution:** `imagePullPolicy: Never` or `imagePullPolicy: IfNotPresent` + `kind load docker-image`.

### 3. Prometheus Scrape Misses
**Problem:** ServiceMonitor selects wrong pods.
**Solution:** Label consistency — all services use `app.kubernetes.io/name` label matching ServiceMonitor selector.

### 4. ScyllaDB Init Race
**Problem:** ScyllaDB needs 60s+ to become ready, services crash on connect.
**Solution:** initContainer that waits for ScyllaDB + longer startupProbe in ScyllaDB StatefulSet.

### 5. Envoy Upstream Discovery
**Problem:** Envoy config has hardcoded `host:port`, doesn't work in K8s.
**Solution:** Change Envoy clusters to use K8s DNS names (`auth-service.redyx-app.svc:50052`).

### 6. Trace ID Not in Logs
**Problem:** Logs exist but no correlation to traces.
**Solution:** Extract trace context in logging middleware, add trace_id field to all log entries.

### 7. HPA Flapping
**Problem:** HPA scales up/down rapidly on bursty traffic.
**Solution:** Add `stabilizationWindowSeconds: 300` to HPA behavior.

## Decision: Helm vs Raw Manifests

**Recommendation:** Umbrella Helm chart for services, Bitnami Helm charts for data stores, Prometheus community charts for monitoring.

**Why Helm:**
- Values files per environment (dev/prod)
- Single command install: `helm upgrade --install redyx-app ./deploy/k8s/charts/redyx-services`
- Version pinning for data store charts
- Easy uninstall: `helm uninstall redyx-app`

**Why NOT kustomize:**
- Adds complexity without significant benefit for this project
- Helm chart templating is simpler for same-structure deployments across 12 services

## Decision: Prometheus Operator vs Standalone

**Recommendation:** Prometheus Operator via kube-prometheus-stack chart.

**Why:**
- ServiceMonitor CRD makes scrape config declarative
- Pre-built Grafana dashboards for K8s metrics
- AlertManager included (even if alerting is deferred)
- Single Helm release: `helm install prometheus prometheus-community/kube-prometheus-stack`

## Decision: Loki Mode

**Recommendation:** Loki in single-binary mode with Promtail DaemonSet.

**Why:**
- Simplest deployment (one pod)
- DaemonSet auto-collects from all nodes
- 7-day retention fits local dev
- Can upgrade to distributed mode later if needed

## Makefile Targets

```makefile
# Cluster lifecycle
k8s-create:     # Create kind cluster + install ingress
k8s-delete:     # Delete kind cluster

# Image management
k8s-build:      # Build all 12 service images
k8s-load:       # Load all images into kind

# Deployments
k8s-data:       # Deploy data stores via Helm
k8s-monitoring: # Deploy observability stack via Helm
k8s-app:        # Deploy all services via Helm
k8s-up:         # k8s-data + k8s-monitoring + k8s-app

# Operations
k8s-down:       # Uninstall all Helm releases
k8s-logs:       # kubectl logs with label selector
k8s-port:       # Port-forward Grafana (3000), Prometheus (9090), Jaeger (16686)
```

## References

- [kind Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [promgrpc](https://github.com/grpc-ecosystem/go-grpc-prometheus)
- [OTEL Go SDK](https://opentelemetry.io/docs/languages/go/)
- [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)
- [Loki Helm Chart](https://grafana.com/docs/loki/latest/setup/install/helm/)
- [Bitnami Charts](https://github.com/bitnami/charts)

---
*Research completed: 2026-03-31*
*Phase: 07-deployment-observability*
