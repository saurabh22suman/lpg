# /e2e-gate

Run full LPG regression and live multiprovider end-to-end verification.

## Preconditions
- `.env` exists and includes provider + local abstraction settings.
- Required tools installed: `golangci-lint`, `govulncheck`, `gitleaks`.

## Execution
1. Spawn `e2e-tester` agent.
2. Agent runs:
   - `make fmt-check`
   - `make lint`
   - `go test ./...`
   - `go test -race ./...`
   - coverage run
   - `govulncheck ./...`
   - `gitleaks detect --source . --redact --no-banner --no-git` (with `.env` / `.env.example` excluded via `.gitleaks.toml`)
   - live edge-case smoke against:
     - `POST /v1/debug/explain`
     - `POST /v1/chat/completions`
3. Agent verifies request-id, route/egress behavior, and redacted audit output.

## Required output
- `status: PASS|FAIL`
- gate-by-gate results
- edge-case result matrix (low/medium/high/critical)
- explicit blocker list for failures

If any mandatory gate fails, final status is FAIL.
