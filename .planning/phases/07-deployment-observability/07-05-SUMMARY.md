---
phase: 07-deployment-observability
plan: 05
subsystem: infra
tags: [kubernetes, helm, prometheus, servicemonitor, envoy, observability]

# Dependency graph
requires:
  - phase: 07-02
    provides: Data store K8s deployments
  - phase: 07-03
    provides: Monitoring stack (Prometheus, Grafana, Jaeger)
  - phase: 07-04
    provides: Service instrumentation (metrics, tracing)
provides:
  - Complete Helm values for all 12 services with metrics ports
  - ServiceMonitor template for Prometheus scraping
  - K8s-adapted Envoy config with internal DNS
  - Envoy deployment with proper ConfigMap mounts
affects: [07-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - ServiceMonitor CRD for Prometheus operator integration
    - Helm Files.Get for loading config files into ConfigMaps
    - K8s DNS service discovery ({service}.{namespace}.svc.cluster.local)

key-files:
  created:
    - deploy/k8s/charts/redyx-services/templates/servicemonitor.yaml
    - deploy/envoy/envoy-k8s.yaml
    - deploy/k8s/envoy-configmap.yaml
    - deploy/k8s/charts/redyx-services/files/envoy-k8s.yaml
    - deploy/k8s/charts/redyx-services/files/proto.pb
  modified:
    - deploy/k8s/charts/redyx-services/values.yaml
    - deploy/k8s/charts/redyx-services/values-dev.yaml
    - deploy/k8s/charts/redyx-services/templates/deployment.yaml
    - deploy/k8s/charts/redyx-services/templates/service.yaml
    - deploy/k8s/charts/redyx-services/templates/configmap.yaml
    - Makefile

key-decisions:
  - "metricsPort: 9090 standardized across all services for Prometheus scraping"
  - "ServiceMonitor template iterates over enabled services with metricsPort"
  - "Envoy ConfigMap split into envoy-config (yaml) and envoy-proto (binary) for proper mount handling"

patterns-established:
  - "Helm Files.Get pattern for loading external config files into ConfigMaps"
  - "Separate ConfigMap for binary data (proto.pb) using binaryData field"

requirements-completed: [INFRA-02, INFRA-03]

# Metrics
duration: 6min
completed: 2026-04-01
---

# Phase 07 Plan 05: Helm Values, ServiceMonitor & Envoy K8s Config Summary

**Complete Helm chart configuration with ServiceMonitor template for Prometheus and K8s-adapted Envoy gateway config**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-01T08:01:44Z
- **Completed:** 2026-04-01T08:08:40Z
- **Tasks:** 3
- **Files modified:** 11

## Accomplishments

- Updated Helm values with metricsPort configuration for all 12 services
- Created ServiceMonitor template that generates 12 ServiceMonitors for Prometheus
- Created K8s-adapted Envoy config with internal DNS names
- Updated Envoy deployment with proper ConfigMap volume mounts

## Task Commits

Each task was committed atomically:

1. **Task 1: Update Helm values with complete service configuration** - `d612a38` (feat)
2. **Task 2: Create ServiceMonitor template and Envoy K8s config** - `2e7e302` (feat)
3. **Task 3: Add Envoy deployment template and finalize Helm chart** - `2b192da` (feat)

## Files Created/Modified

- `deploy/k8s/charts/redyx-services/values.yaml` - Added metricsPort, METRICS_PORT env, global otel config
- `deploy/k8s/charts/redyx-services/values-dev.yaml` - Added METRICS_PORT to all services
- `deploy/k8s/charts/redyx-services/templates/servicemonitor.yaml` - New ServiceMonitor template
- `deploy/k8s/charts/redyx-services/templates/deployment.yaml` - Added metrics port, updated Envoy mounts
- `deploy/k8s/charts/redyx-services/templates/service.yaml` - Added metrics port to services
- `deploy/k8s/charts/redyx-services/templates/configmap.yaml` - Load Envoy config from files/
- `deploy/envoy/envoy-k8s.yaml` - K8s-adapted Envoy config with DNS names
- `deploy/k8s/envoy-configmap.yaml` - Reference ConfigMap for manual operations
- `deploy/k8s/charts/redyx-services/files/` - Directory with envoy-k8s.yaml and proto.pb
- `Makefile` - Updated k8s-app target to copy Envoy files before deploy

## Decisions Made

- Standardized metricsPort at 9090 for all services (consistent with Phase 07-04 instrumentation)
- Split Envoy ConfigMap into two: envoy-config (yaml) and envoy-proto (binaryData for proto.pb)
- Used Helm Files.Get pattern to load external files into ConfigMaps

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Helm chart fully configured with all 12 services
- ServiceMonitors ready for Prometheus scraping
- Envoy gateway ready for K8s deployment with proper DNS names
- Ready for Plan 07-06: E2E Deployment Testing

---
*Phase: 07-deployment-observability*
*Completed: 2026-04-01*
