---
phase: 07-deployment-observability
plan: 01
subsystem: infra
tags: [kubernetes, kind, helm, k8s]

# Dependency graph
requires: []
provides:
  - Kind cluster configuration for local K8s development
  - Helm chart rendering 12 microservice deployments
  - Makefile targets mirroring docker-compose workflow
affects: [07-02, 07-03, 07-04, 07-05]

# Tech tracking
tech-stack:
  added: [kind, helm]
  patterns: [helm-chart-templates, k8s-namespace-isolation, hpa-stabilization]

key-files:
  created:
    - deploy/k8s/kind-config.yaml
    - deploy/k8s/charts/redyx-services/Chart.yaml
    - deploy/k8s/charts/redyx-services/values.yaml
    - deploy/k8s/charts/redyx-services/values-dev.yaml
    - deploy/k8s/charts/redyx-services/templates/_helpers.tpl
    - deploy/k8s/charts/redyx-services/templates/deployment.yaml
    - deploy/k8s/charts/redyx-services/templates/service.yaml
    - deploy/k8s/charts/redyx-services/templates/hpa.yaml
    - deploy/k8s/charts/redyx-services/templates/configmap.yaml
    - scripts/k8s-validate.sh
  modified:
    - Makefile

key-decisions:
  - "K8s DNS names use {service}.{namespace}.svc.cluster.local pattern"
  - "HPA stabilizationWindowSeconds: 300 for scaleDown to prevent flapping"
  - "Envoy deployed as NodePort service for external API gateway access"
  - "gRPC native probes for readiness/liveness (not HTTP)"
  - "Security context with readOnlyRootFilesystem and runAsNonRoot"

patterns-established:
  - "Helm values.yaml with direct K8s DNS names (not templates within values)"
  - "ConfigMap per service for environment variables"
  - "Service selector using app.kubernetes.io/name convention"
  - "Makefile k8s-* targets parallel to docker-* targets"

requirements-completed: [INFRA-02]

# Metrics
duration: 15 min
completed: 2026-03-31
---

# Phase 7 Plan 01: K8s Infrastructure Foundation Summary

**Kind cluster config with NodePort mappings, Helm chart for 12 microservices with HPA/probes/configmaps, and complete Makefile K8s lifecycle targets**

## Performance

- **Duration:** 15 min
- **Started:** 2026-03-31T19:49:53Z
- **Completed:** 2026-03-31T21:24:39Z
- **Tasks:** 3
- **Files modified:** 12

## Accomplishments
- Kind cluster configuration with ingress-ready node and Docker socket mount
- Complete Helm chart structure rendering 12 Deployments + 12 Services + 12 HPAs + 13 ConfigMaps
- Validation script checking cluster, namespaces, pods, and service endpoints
- Full Makefile k8s-* target set matching docker-compose workflow

## Task Commits

Each task was committed atomically:

1. **Task 1: Create kind cluster config and validation script** - `3432623` (feat)
2. **Task 2: Create Helm chart structure for 12 services** - `36a3adc` (feat)
3. **Task 3: Add Kubernetes Makefile targets** - `30c992f` (feat)

**Plan metadata:** (will be committed with SUMMARY.md)

## Files Created/Modified

### Created
- `deploy/k8s/kind-config.yaml` - Kind cluster config with NodePort extraPortMappings
- `deploy/k8s/charts/redyx-services/Chart.yaml` - Helm chart metadata
- `deploy/k8s/charts/redyx-services/values.yaml` - Default values with K8s DNS service addresses
- `deploy/k8s/charts/redyx-services/values-dev.yaml` - Dev overrides with lower resources + OTEL
- `deploy/k8s/charts/redyx-services/templates/_helpers.tpl` - Chart helpers and label templates
- `deploy/k8s/charts/redyx-services/templates/deployment.yaml` - Deployment template with gRPC probes
- `deploy/k8s/charts/redyx-services/templates/service.yaml` - ClusterIP service template
- `deploy/k8s/charts/redyx-services/templates/hpa.yaml` - HPA template with stabilization
- `deploy/k8s/charts/redyx-services/templates/configmap.yaml` - ConfigMap template for env vars
- `scripts/k8s-validate.sh` - Cluster validation script

### Modified
- `Makefile` - Added k8s-* targets, fixed help regex for numeric target names

## Decisions Made

1. **K8s DNS pattern:** All services use `{service}.redyx-app.svc.cluster.local` for internal DNS resolution. Data stores use `{store}.redyx-data.svc.cluster.local`.

2. **Direct values in values.yaml:** Environment variables are literal strings with K8s DNS names, not Helm template references (e.g., `DATABASE_URL: "postgres://redyx:dev@postgresql.redyx-data.svc.cluster.local:5432/auth?sslmode=disable"`).

3. **HPA stabilization:** Added `stabilizationWindowSeconds: 300` for scaleDown and `60` for scaleUp to prevent rapid scaling flapping.

4. **Security context:** All service pods run with `readOnlyRootFilesystem: true`, `runAsNonRoot: true`, `runAsUser: 1000`, and drop all capabilities.

5. **Help regex fix:** Updated Makefile help regex from `[a-zA-Z_-]` to `[a-zA-Z0-9_-]` to include numeric characters in target names (e.g., `k8s-*`).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- Helm not installed in environment - resolved by installing to `~/.local/bin`
- Help regex didn't match `k8s-*` targets due to missing `0-9` in character class - fixed inline

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for Plan 02: Data Store Helm Charts
- Kind cluster config ready for `kind create cluster --config`
- Namespace strategy defined (redyx-app, redyx-data, redyx-monitoring)
- Helm chart structure established as pattern for data store charts
- Makefile targets provide `k8s-data` placeholder for Plan 02 implementation

---
*Phase: 07-deployment-observability*
*Completed: 2026-03-31*

## Self-Check: PASSED

All key files verified:
- deploy/k8s/kind-config.yaml
- deploy/k8s/charts/redyx-services/Chart.yaml
- deploy/k8s/charts/redyx-services/templates/deployment.yaml
- scripts/k8s-validate.sh
- Commit 3432623 (Task 1)
- Commit 36a3adc (Task 2)
- Commit 30c992f (Task 3)
