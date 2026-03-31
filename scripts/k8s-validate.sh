#!/usr/bin/env bash
# k8s-validate.sh - Validate Redyx Kubernetes deployment
# Checks cluster, namespaces, and service health

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

CLUSTER_NAME="redyx"
NAMESPACE_APP="redyx-app"
NAMESPACE_DATA="redyx-data"
NAMESPACE_MON="redyx-monitoring"

# Services to validate (12 services from docker-compose)
SERVICES=(
  "skeleton-service"
  "auth-service"
  "user-service"
  "community-service"
  "post-service"
  "vote-service"
  "comment-service"
  "search-service"
  "notification-service"
  "media-service"
  "moderation-service"
  "spam-service"
)

# Data stores to validate
DATA_STORES=(
  "postgresql"
  "redis"
  "scylladb"
  "kafka"
  "meilisearch"
  "minio"
)

# Monitoring components
MONITORING=(
  "prometheus"
  "grafana"
  "loki"
  "jaeger"
)

pass_count=0
fail_count=0

check_pass() {
  echo -e "${GREEN}[PASS]${NC} $1"
  ((pass_count++))
}

check_fail() {
  echo -e "${RED}[FAIL]${NC} $1"
  ((fail_count++))
}

check_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

echo "============================================"
echo "  Redyx Kubernetes Validation"
echo "============================================"
echo ""

# Check 1: Kind cluster exists
echo ">>> Checking kind cluster..."
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  check_pass "Kind cluster '${CLUSTER_NAME}' exists"
else
  check_fail "Kind cluster '${CLUSTER_NAME}' not found"
  echo "  Run: make k8s-create"
  exit 1
fi

# Check 2: kubectl context
echo ""
echo ">>> Checking kubectl context..."
CURRENT_CONTEXT=$(kubectl config current-context 2>/dev/null || echo "none")
if [[ "$CURRENT_CONTEXT" == *"${CLUSTER_NAME}"* ]]; then
  check_pass "kubectl context is set to kind-${CLUSTER_NAME}"
else
  check_warn "kubectl context is '${CURRENT_CONTEXT}', expected 'kind-${CLUSTER_NAME}'"
fi

# Check 3: Namespaces exist
echo ""
echo ">>> Checking namespaces..."
for ns in "$NAMESPACE_APP" "$NAMESPACE_DATA" "$NAMESPACE_MON"; do
  if kubectl get namespace "$ns" &>/dev/null; then
    check_pass "Namespace '$ns' exists"
  else
    check_fail "Namespace '$ns' not found"
  fi
done

# Check 4: App pods ready
echo ""
echo ">>> Checking app pods in ${NAMESPACE_APP}..."
if kubectl get namespace "$NAMESPACE_APP" &>/dev/null; then
  for svc in "${SERVICES[@]}"; do
    pod_status=$(kubectl get pods -n "$NAMESPACE_APP" -l "app.kubernetes.io/name=${svc}" -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "NotFound")
    ready=$(kubectl get pods -n "$NAMESPACE_APP" -l "app.kubernetes.io/name=${svc}" -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "False")
    
    if [[ "$pod_status" == "Running" && "$ready" == "True" ]]; then
      check_pass "Pod ${svc} is Running and Ready"
    elif [[ "$pod_status" == "Running" ]]; then
      check_warn "Pod ${svc} is Running but not Ready"
    elif [[ "$pod_status" == "NotFound" ]]; then
      check_fail "Pod ${svc} not found"
    else
      check_fail "Pod ${svc} status: ${pod_status}"
    fi
  done
else
  check_warn "Namespace ${NAMESPACE_APP} not found, skipping app pod checks"
fi

# Check 5: Data store pods ready
echo ""
echo ">>> Checking data store pods in ${NAMESPACE_DATA}..."
if kubectl get namespace "$NAMESPACE_DATA" &>/dev/null; then
  for store in "${DATA_STORES[@]}"; do
    pod_count=$(kubectl get pods -n "$NAMESPACE_DATA" -l "app.kubernetes.io/name=${store}" --no-headers 2>/dev/null | wc -l)
    if [[ "$pod_count" -gt 0 ]]; then
      ready_count=$(kubectl get pods -n "$NAMESPACE_DATA" -l "app.kubernetes.io/name=${store}" -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -c "True" || echo "0")
      if [[ "$ready_count" -gt 0 ]]; then
        check_pass "Data store ${store} has ${ready_count} ready pod(s)"
      else
        check_fail "Data store ${store} pods not ready"
      fi
    else
      check_fail "Data store ${store} not found"
    fi
  done
else
  check_warn "Namespace ${NAMESPACE_DATA} not found, skipping data store checks"
fi

# Check 6: Monitoring pods ready
echo ""
echo ">>> Checking monitoring pods in ${NAMESPACE_MON}..."
if kubectl get namespace "$NAMESPACE_MON" &>/dev/null; then
  for mon in "${MONITORING[@]}"; do
    pod_count=$(kubectl get pods -n "$NAMESPACE_MON" -l "app.kubernetes.io/name=${mon}" --no-headers 2>/dev/null | wc -l)
    if [[ "$pod_count" -gt 0 ]]; then
      ready_count=$(kubectl get pods -n "$NAMESPACE_MON" -l "app.kubernetes.io/name=${mon}" -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -c "True" || echo "0")
      if [[ "$ready_count" -gt 0 ]]; then
        check_pass "Monitoring ${mon} has ${ready_count} ready pod(s)"
      else
        check_fail "Monitoring ${mon} pods not ready"
      fi
    else
      check_fail "Monitoring ${mon} not found"
    fi
  done
else
  check_warn "Namespace ${NAMESPACE_MON} not found, skipping monitoring checks"
fi

# Check 7: Service endpoints
echo ""
echo ">>> Checking service endpoints..."
if kubectl get namespace "$NAMESPACE_APP" &>/dev/null; then
  for svc in "${SERVICES[@]}"; do
    endpoints=$(kubectl get endpoints -n "$NAMESPACE_APP" "$svc" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null | wc -w)
    if [[ "$endpoints" -gt 0 ]]; then
      check_pass "Service ${svc} has ${endpoints} endpoint(s)"
    else
      check_warn "Service ${svc} has no endpoints"
    fi
  done
fi

# Summary
echo ""
echo "============================================"
echo "  Validation Summary"
echo "============================================"
echo -e "  ${GREEN}Passed:${NC} ${pass_count}"
echo -e "  ${RED}Failed:${NC} ${fail_count}"
echo ""

if [[ "$fail_count" -gt 0 ]]; then
  echo -e "${RED}Validation FAILED${NC}"
  exit 1
else
  echo -e "${GREEN}Validation PASSED${NC}"
  exit 0
fi
