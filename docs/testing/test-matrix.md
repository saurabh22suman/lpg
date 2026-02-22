# LPG Test Matrix (Phase 1)

This matrix maps PRD metrics (M1–M8) and TV taxonomy to concrete tests/jobs in this repository.

## TV taxonomy to tests

| TV Group | Focus | Current files |
|---|---|---|
| TV-DET | Deterministic masking and mapping correctness | `internal/sanitizer/sanitizer_test.go` (`TV-DET-001`) |
| TV-ROUTE | Score banding and route enforcement | `internal/risk/risk_test.go` (`TV-ROUTE-001`, `TV-ROUTE-002`), `test/integration/tv_route_001_boundary_test.go`, `test/integration/tv_route_002_confidence_escalation_test.go`, `test/integration/tv_route_003_raw_forward_payload_test.go`, `test/integration/tv_route_critical_no_egress_test.go` |
| TV-REL | Provider fault and safe handling | `test/reliability/tv_rel_001_timeout_test.go` (`TV-REL-001`, `TV-REL-002`, `TV-REL-003`, `TV-REL-004`, `TV-REL-005`), `test/reliability/tv_rel_006_retry_test.go` (`TV-REL-006`) |
| TV-LEAK | End-to-end leakage prevention | `test/leakage/tv_leak_001_no_raw_entity_egress_test.go` (`TV-LEAK-001`), `test/leakage/tv_leak_002_error_audit_no_raw_test.go` (`TV-LEAK-002`, `TV-LEAK-003`) |
| TV-INT | OpenAI-compatible interface checks | `test/integration/chat_completions_integration_test.go`, `test/integration/prd_6_6_contract_gaps_integration_test.go` |
| TV-REDTEAM | Adversarial scenarios | `test/redteam/` (reserved for next phase) |
| TV-ABS | Local abstraction behavior | `internal/proxy/high_abstractor_http_test.go` (`RouteHighAbstraction` instruction behavior + provider path), plus handler route-path tests |
| TV-TOON | TOON eligibility/conversion | deferred in phase 1 |
| TV-DX | CLI/onboarding workflow checks | README provider setup + manual smoke commands for `vllm_local` and `mimo_online` |
| TV-COST | Budget guardrails | deferred in phase 1 |

## M1–M8 metric mapping

| Metric | Target (PRD) | Repository mapping |
|---|---|---|
| M1 Deterministic overhead | p95 ≤ 300ms | CI coverage for deterministic pipeline; benchmark suite deferred (`TV-PERF-*` not yet implemented) |
| M2 Zero leakage | 0 critical leak events | `test/leakage/tv_leak_001_no_raw_entity_egress_test.go`, `test/leakage/tv_leak_002_error_audit_no_raw_test.go`, plus route fail-closed tests |
| M3 Token optimization | median ≥ 15% | deferred with TOON/abstraction depth (`TV-TOON-03`, `TV-ABS-02` pending) |
| M4 Routing correctness | ≥99.5% conformance | `internal/risk/risk_test.go`, `test/integration/tv_route_001_boundary_test.go`, `test/integration/tv_route_002_confidence_escalation_test.go`, `test/integration/tv_route_003_raw_forward_payload_test.go` |
| M5 Fallback reliability | Critical no-egress + safe outcomes | `test/reliability/tv_rel_001_timeout_test.go` (timeout + strict/non-strict audit behavior + idempotency forwarding), `test/reliability/tv_rel_006_retry_test.go`, `test/integration/tv_route_critical_no_egress_test.go` |
| M6 Integration success | required compatibility scenarios | `test/integration/chat_completions_integration_test.go`, `test/integration/prd_6_6_contract_gaps_integration_test.go` for `/v1/chat/completions` thin slice |
| M7 Onboarding success | docs-only setup success | README provider setup and smoke commands for local `vllm_local` and online `mimo_online`; formal DX suite still pending |
| M8 Cost governance | guardrail actions enforced | deferred in phase 1 |

## CI job mapping

| Job | Enforced by | Purpose |
|---|---|---|
| format/lint/test/race/coverage | `.github/workflows/ci.yml` (`make ci`) | Main quality gate |
| vulnerability + secret scans | `.github/workflows/security.yml` and `make security` | Security gate, must be green |

## First-slice mandatory IDs status

| ID | Status | File |
|---|---|---|
| TV-DET-001 | implemented | `internal/sanitizer/sanitizer_test.go` |
| TV-ROUTE-001 | implemented | `internal/risk/risk_test.go`, `test/integration/tv_route_001_boundary_test.go` |
| TV-ROUTE-002 | implemented | `internal/risk/risk_test.go`, `test/integration/tv_route_002_confidence_escalation_test.go` |
| TV-ROUTE-003 | implemented | `test/integration/tv_route_003_raw_forward_payload_test.go` |
| TV-REL-001 | implemented | `test/reliability/tv_rel_001_timeout_test.go` |
| TV-REL-002 | implemented | `test/reliability/tv_rel_001_timeout_test.go` |
| TV-REL-003 | implemented | `test/reliability/tv_rel_001_timeout_test.go` |
| TV-REL-004 | implemented | `test/reliability/tv_rel_001_timeout_test.go` |
| TV-REL-005 | implemented | `test/reliability/tv_rel_001_timeout_test.go` |
| TV-REL-006 | implemented | `test/reliability/tv_rel_006_retry_test.go` |
| TV-LEAK-001 | implemented | `test/leakage/tv_leak_001_no_raw_entity_egress_test.go` |
| TV-LEAK-002 | implemented | `test/leakage/tv_leak_002_error_audit_no_raw_test.go` |
| TV-LEAK-003 | implemented | `test/leakage/tv_leak_002_error_audit_no_raw_test.go` |
