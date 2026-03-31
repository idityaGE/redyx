---
phase: 07-deployment-observability
plan: 02
subsystem: infrastructure
tags: [kubernetes, helm, postgresql, redis, scylladb, kafka, meilisearch, minio]

# Dependency graph
requires:
  - phase: 07-01
    provides: K8s infrastructure foundation (namespaces, kind cluster, service Helm chart)
provides:
  - Helm values for all 6 data stores (PostgreSQL, Redis, ScyllaDB, Kafka, Meilisearch, MinIO)
  - Makefile k8s-data target for one-command data layer deployment
  - PostgreSQL init script ConfigMap pattern for database creation
affects: [07-03, 07-04, 07-05]

# Tech tracking
tech-stack:
  added: [bitnami/postgresql, bitnami/redis, bitnami/cassandra, bitnami/kafka, meilisearch/meilisearch, bitnami/minio]
  patterns: [Helm values files, ConfigMap init scripts, topic provisioning, KRaft mode Kafka]

key-files:
  created:
    - deploy/k8s/data/postgresql.yaml
    - deploy/k8s/data/redis.yaml
    - deploy/k8s/data/scylladb.yaml
    - deploy/k8s/data/kafka.yaml
    - deploy/k8s/data/meilisearch.yaml
    - deploy/k8s/data/minio.yaml
  modified:
    - Makefile

key-decisions:
  - "ScyllaDB via Cassandra chart with image override (operator too complex for local dev)"
  - "Kafka KRaft mode matches docker-compose (no Zookeeper)"
  - "Topic provisioning for votes, posts, comments on Kafka startup"

patterns-established:
  - "ConfigMap from file pattern for PostgreSQL init scripts"
  - "Bitnami charts for stateful workloads with ServiceMonitor integration"

requirements-completed: [INFRA-02]

# Metrics
duration: 51min
completed: 2026-03-31
---

# Phase 7 Plan 02: K8s Data Stores Summary

**Bitnami Helm values for PostgreSQL, Redis, ScyllaDB, Kafka, Meilisearch, and MinIO with Makefile automation and ConfigMap init pattern**

## Performance

- **Duration:** 51 min
- **Started:** 2026-03-31T22:06:55Z
- **Completed:** 2026-03-31T22:58:23Z
- **Tasks:** 3
- **Files created:** 6
- **Files modified:** 1

## Accomplishments

- Created Helm values for all 6 data stores matching docker-compose configurations
- Implemented k8s-data Makefile target for one-command deployment
- Established ConfigMap pattern for PostgreSQL database initialization
- Configured topic provisioning for Kafka (votes, posts, comments topics)
- Enabled Prometheus ServiceMonitor for all stateful workloads

## Task Commits

Each task was committed atomically:

1. **Task 1: Create PostgreSQL and Redis Helm values** - `b2beb2b` (feat)
2. **Task 2: Create ScyllaDB, Kafka, Meilisearch, MinIO Helm values** - `80b7d82` (feat)
3. **Task 3: Update Makefile k8s-data target with Helm commands** - `90b8b13` (feat)

## Files Created/Modified

- `deploy/k8s/data/postgresql.yaml` - Bitnami PostgreSQL values with initdb ConfigMap
- `deploy/k8s/data/redis.yaml` - Bitnami Redis standalone mode values
- `deploy/k8s/data/scylladb.yaml` - ScyllaDB via Cassandra chart with image override
- `deploy/k8s/data/kafka.yaml` - Bitnami Kafka KRaft mode with topic provisioning
- `deploy/k8s/data/meilisearch.yaml` - Meilisearch values with dev master key
- `deploy/k8s/data/minio.yaml` - Bitnami MinIO with redyx-media bucket
- `Makefile` - k8s-data and k8s-data-down targets

## K8s DNS Names Reference

Services should connect to data stores via these K8s DNS names:
- PostgreSQL: `postgresql.redyx-data.svc.cluster.local:5432`
- Redis: `redis-master.redyx-data.svc.cluster.local:6379`
- ScyllaDB: `scylladb.redyx-data.svc.cluster.local:9042`
- Kafka: `kafka.redyx-data.svc.cluster.local:9092`
- Meilisearch: `meilisearch.redyx-data.svc.cluster.local:7700`
- MinIO: `minio.redyx-data.svc.cluster.local:9000`

## Decisions Made

1. **ScyllaDB via Cassandra chart** - Used bitnami/cassandra with ScyllaDB image override rather than ScyllaDB operator (too complex for local dev)
2. **Kafka KRaft mode** - Matches docker-compose configuration, no Zookeeper dependency
3. **Topic provisioning** - Pre-create redyx.votes.v1 (6 partitions), redyx.posts.v1 (3), redyx.comments.v1 (3) on startup

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Data store Helm values ready for `make k8s-data` deployment
- Ready for Plan 07-03: Observability Stack (Prometheus, Grafana, Loki, Jaeger)
- Services can use K8s DNS names documented above for data store connections

---
*Phase: 07-deployment-observability*
*Completed: 2026-03-31*
