---
phase: 07
slug: deployment-observability
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-31
---

# Phase 07 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | kubectl + bash scripts (infrastructure validation) |
| **Config file** | scripts/k8s-validate.sh (Wave 0 creates) |
| **Quick run command** | `kubectl get pods -n redyx-app -o wide` |
| **Full suite command** | `./scripts/k8s-validate.sh` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick health check via kubectl
- **After every plan wave:** Run full validation script
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds (K8s operations are slower than unit tests)

---

## Per-task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 01 | 1 | INFRA-02 | infra | `kind get clusters \| grep redyx` | ❌ W0 | ⬜ pending |
| 07-01-02 | 01 | 1 | INFRA-02 | infra | `kubectl get ns redyx-app` | ❌ W0 | ⬜ pending |
| 07-02-01 | 02 | 2 | INFRA-02 | infra | `kubectl get pods -n redyx-data --no-headers \| wc -l` | ❌ W0 | ⬜ pending |
| 07-03-01 | 03 | 2 | INFRA-03 | infra | `curl -s localhost:9090/api/v1/targets \| jq '.data.activeTargets \| length'` | ❌ W0 | ⬜ pending |
| 07-03-02 | 03 | 2 | INFRA-04 | infra | `curl -s localhost:3000/api/search \| jq '.[] \| select(.type=="dash-db")' \| wc -l` | ❌ W0 | ⬜ pending |
| 07-03-03 | 03 | 2 | INFRA-05 | infra | `kubectl logs -n redyx-monitoring -l app=promtail --tail=1 \| jq .` | ❌ W0 | ⬜ pending |
| 07-03-04 | 03 | 2 | INFRA-06 | infra | `curl -s localhost:16686/api/services \| jq '.data \| length'` | ❌ W0 | ⬜ pending |
| 07-04-01 | 04 | 3 | INFRA-03 | unit | `go test ./internal/platform/observability/...` | ❌ W0 | ⬜ pending |
| 07-05-01 | 05 | 4 | INFRA-02 | infra | `kubectl get pods -n redyx-app --no-headers \| grep -c Running` | ❌ W0 | ⬜ pending |
| 07-05-02 | 05 | 4 | INFRA-02 | infra | `kubectl get hpa -n redyx-app --no-headers \| wc -l` | ❌ W0 | ⬜ pending |
| 07-06-01 | 06 | 5 | ALL | e2e | `./scripts/k8s-validate.sh` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `scripts/k8s-validate.sh` — main validation script
- [ ] `deploy/k8s/kind-config.yaml` — kind cluster configuration
- [ ] `kubectl`, `kind`, `helm` — CLI tools (documented in README)

*Note: Wave 0 creates validation infrastructure alongside cluster setup in Plan 01.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Grafana dashboards visually correct | INFRA-04 | Layout/aesthetics | Open localhost:3000, verify panels render data |
| Jaeger trace waterfall | INFRA-06 | Visual trace structure | Click a trace in Jaeger, verify spans are nested correctly |

*Most behaviors have automated verification — visual checks are supplementary.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
