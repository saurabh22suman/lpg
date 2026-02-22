# LPG (LLM Privacy Gateway)

LPG is a local-first meaning firewall for OpenAI-compatible traffic.

This repository currently implements a **Phase 1 thin vertical slice** focused on:
- deterministic sanitization,
- risk scoring and route decisions,
- fail-closed behavior,
- request ID traceability,
- append-only redacted audit chaining.

---

## Setup Guide

### 1) Prerequisites

- Go `1.24.13+`
- `golangci-lint`
- `govulncheck`
- `gitleaks`

Example installs:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

Install `gitleaks` from the project releases:
- https://github.com/gitleaks/gitleaks

### 2) Install dependencies

```bash
go mod tidy
```

### 3) Ensure Go bin is on PATH

```bash
export PATH="$PATH:$HOME/go/bin"
```

---

## Run & Verification Commands

```bash
make fmt
make fmt-check
make lint
make test
make test-race
make coverage
make security
make ci
```

If your local Go version is older than `go.mod`, run with toolchain override:

```bash
GOTOOLCHAIN=go1.24.13 make ci
```

---

## Usage Guidelines

## 1) Start the proxy

Use the helper script (installs missing deps + starts LPG):

```bash
./start.sh
```

Skip installation on repeated runs:

```bash
./start.sh --skip-install
```

Or run directly:

```bash
go run ./cmd/lpg
```

Default bind address:
- `127.0.0.1:8080`

Optional audit log location:

```bash
LPG_AUDIT_PATH=/tmp/lpg-audit.log go run ./cmd/lpg
```

If `LPG_AUDIT_PATH` is not set, LPG writes to `./audit.log`.

## Provider setup

LPG supports hybrid deployment patterns:

1. **Local lightweight CPU model (llama.cpp, 1B–3B)** for deterministic masking-adjacent tasks (structured abstraction, entity-aware shaping), not heavy reasoning.
2. **Remote reasoning model** (for example Mimo v2 Flash / ClawedBot backend) for complex generation, always after LPG masking/routing.
3. **Critical local-only mode** to prevent any remote egress for critical sensitivity requests.

LPG selects the upstream provider at startup from environment variables.

### Provider env vars

- `LPG_PROVIDER`: `stub` (default), `vllm_local`, `mimo_online`, `openai_compatible`, or alias `llamacpp_local`
- `LPG_PROVIDER_TIMEOUT`: optional Go duration (for example `2s`, `1500ms`)
- `LPG_ALLOW_RAW_FORWARDING`: optional bool (`true|false`, default `false`) for low-risk minimal-mask forwarding
- `LPG_CRITICAL_LOCAL_ONLY`: optional bool (`true|false`, default `false`) to force local-only critical handling with no remote calls
- `LPG_AUDIT_PATH`: optional path for append-only audit log (unchanged behavior)

#### `stub` (default)

No additional provider variables are required.

#### `openai_compatible` + local llama.cpp CPU (recommended local option)

For CPU-only environments, prefer a local OpenAI-compatible server backed by llama.cpp with a 1B–3B model.
This local model should be used for lightweight deterministic/structured transformations, not heavy reasoning.

Required:
- `LPG_UPSTREAM_BASE_URL` (for example local server URL)

Optional:
- `LPG_UPSTREAM_MODEL`
- `LPG_UPSTREAM_API_KEY` (if your local endpoint requires it)

Example:

```bash
export LPG_PROVIDER=openai_compatible
export LPG_UPSTREAM_BASE_URL=http://127.0.0.1:8081
export LPG_UPSTREAM_MODEL=llama-3.2-3b-instruct-q4
./start.sh --skip-install
```

#### `vllm_local` (GPU-backed optional path)

Use this only when you actually have a GPU-backed deployment. It is not required for CPU-only environments.

Required:
- `LPG_VLLM_BASE_URL` (for example `http://127.0.0.1:8000`)

Optional:
- `LPG_VLLM_MODEL` (fallback model if request model is empty)

Example:

```bash
export LPG_PROVIDER=vllm_local
export LPG_VLLM_BASE_URL=http://127.0.0.1:8000
export LPG_VLLM_MODEL=meta-llama/Llama-3.1-8B-Instruct
./start.sh --skip-install
```

#### `mimo_online`

Required:
- `LPG_MIMO_BASE_URL` (set to the provider endpoint you use)
- `LPG_MIMO_API_KEY`

Optional:
- `LPG_MIMO_MODEL` (default: `mimo-v2-flash`)

Example:

```bash
export LPG_PROVIDER=mimo_online
export LPG_MIMO_BASE_URL=https://your-mimo-endpoint.example
export LPG_MIMO_API_KEY=your-secret-key
export LPG_MIMO_MODEL=mimo-v2-flash
./start.sh --skip-install
```

#### `openai_compatible` (generic user-configurable mode)

Use this mode for any OpenAI-compatible provider (hosted or local) by setting env vars.

Required:
- `LPG_UPSTREAM_BASE_URL` (for example `http://127.0.0.1:8000` or provider URL)

Optional:
- `LPG_UPSTREAM_API_KEY`
- `LPG_UPSTREAM_MODEL` (fallback model if request model is empty)
- `LPG_UPSTREAM_API_KEY_HEADER` (default: `Authorization`)
- `LPG_UPSTREAM_API_KEY_PREFIX` (default: `Bearer`; set empty for raw key values)
- `LPG_UPSTREAM_CHAT_PATH` (default: `/v1/chat/completions`)

Optional local abstraction provider (enables true two-model process in one LPG instance):
- `LPG_LOCAL_ABSTRACTION_BASE_URL` (OpenAI-compatible local endpoint)
- `LPG_LOCAL_ABSTRACTION_API_KEY`
- `LPG_LOCAL_ABSTRACTION_MODEL` (required when local abstraction base URL is set)
- `LPG_LOCAL_ABSTRACTION_API_KEY_HEADER` (default: `Authorization`)
- `LPG_LOCAL_ABSTRACTION_API_KEY_PREFIX` (default: `Bearer`; set empty for raw key values)
- `LPG_LOCAL_ABSTRACTION_CHAT_PATH` (default: `/v1/chat/completions`)

Aliases for `LPG_PROVIDER`: `generic`, `openai`, `custom`, `llamacpp_local`.

Example:

```bash
export LPG_PROVIDER=openai_compatible
export LPG_UPSTREAM_BASE_URL=https://your-provider.example
export LPG_UPSTREAM_API_KEY=your-secret-key
export LPG_UPSTREAM_MODEL=your-model
export LPG_UPSTREAM_API_KEY_HEADER=Authorization
export LPG_UPSTREAM_API_KEY_PREFIX=Bearer
export LPG_UPSTREAM_CHAT_PATH=/v1/chat/completions
./start.sh --skip-install
```

#### True multiprovider example (local abstraction + remote generation)

This runs both models in one process:
- local OpenAI-compatible endpoint for abstraction (`LPG_LOCAL_ABSTRACTION_*`),
- remote provider for final generation (`LPG_UPSTREAM_*` or `LPG_MIMO_*`).

```bash
export LPG_PROVIDER=mimo_online
export LPG_MIMO_BASE_URL=https://your-mimo-endpoint.example
export LPG_MIMO_API_KEY=your-secret-key
export LPG_MIMO_MODEL=mimo-v2-flash

export LPG_LOCAL_ABSTRACTION_BASE_URL=http://127.0.0.1:8081
export LPG_LOCAL_ABSTRACTION_MODEL=qwen2.5-3b-instruct

# Optional local auth/path overrides
# export LPG_LOCAL_ABSTRACTION_API_KEY=...
# export LPG_LOCAL_ABSTRACTION_API_KEY_HEADER=Authorization
# export LPG_LOCAL_ABSTRACTION_API_KEY_PREFIX=Bearer
# export LPG_LOCAL_ABSTRACTION_CHAT_PATH=/v1/chat/completions

./start.sh --skip-install
```

Behavior with this setup:
- `low` / `medium`: forward to configured remote provider per route policy.
- `high`: LPG calls the local abstraction model with a constrained jumble instruction over sanitized text, then forwards the rewritten text to the remote provider.
- `critical` + `LPG_CRITICAL_LOCAL_ONLY=true`: LPG calls local abstraction model only, no remote egress.

### Routing mode toggles (local-only / hybrid / minimal-mask)

LPG route selection is risk-driven:
- Low: raw-forward (if enabled) or sanitized-forward
- Medium: sanitized-forward
- High: local abstraction then remote forwarding
- Critical: blocked by default, or local-only when enabled

Example toggles:

```bash
# Enable minimal-mask forwarding for low risk only
export LPG_ALLOW_RAW_FORWARDING=true

# Enable critical local-only handling (no remote egress on critical)
export LPG_CRITICAL_LOCAL_ONLY=true
```

### Secret handling

- Keep API keys in environment variables only.
- Do not commit secrets to source control.
- LPG does not persist provider secrets in audit logs.

## 2) Supported endpoint (Phase 1)

- `POST /v1/chat/completions`

> Note: `/v1/responses` is not implemented in this phase.

## 3) Request format

Required fields:
- `model` (string)
- `messages` (non-empty array)
- each message must include non-empty `role` and `content`

Example request:

```bash
curl -sS http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: req-001" \
  -d '{
    "model": "gpt-test",
    "messages": [
      {"role": "user", "content": "Hello"}
    ]
  }'
```

## 4) Response behavior

- LPG always returns `x-lpg-request-id` header.
- Successful responses use an OpenAI-style `chat.completion` envelope.
- Validation and policy/provider failures return deterministic JSON error payloads.

Error shape:

```json
{
  "error": {
    "code": "ERR_*",
    "message": "..."
  },
  "request_id": "req-..."
}
```

Common error codes:
- `ERR_VALIDATION`
- `ERR_POLICY_BLOCK`
- `ERR_PROVIDER_TIMEOUT`
- `ERR_PROVIDER_FAILURE`
- `ERR_AUDIT_FAILURE`

## 5) Operational guidelines

- Treat LPG as a policy boundary; send all LLM traffic through it.
- Use `x-lpg-request-id` for traceability in logs/tests.
- Keep `make ci` and `make security` green before merge.
- For higher-assurance deployments, enable strict-audit behavior in handler configuration when embedding LPG as a library.

## 6) Smoke tests through LPG

Use the Phase 1 endpoint:
- `POST /v1/chat/completions`

> Note: `/v1/responses` is not implemented in this phase.

Example smoke request:

```bash
curl -sS http://127.0.0.1:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: smoke-001" \
  -d '{
    "model": "smoke-model",
    "messages": [
      {"role": "user", "content": "Say hello in one sentence."}
    ]
  }'
```

Expected checks:
- HTTP `200` on success
- `x-lpg-request-id` header present
- deterministic error envelope on provider timeout/failure

### `/v1/debug/explain` examples (explainability)

Use `POST /v1/debug/explain` with the same request body shape as `/v1/chat/completions` to inspect:
- sanitized input,
- detection count and confidences,
- risk score/category,
- selected route,
- egress decision.

> The debug explain endpoint analyzes and explains routing decisions. It does not call the upstream model and is best used alongside `/v1/chat/completions` + audit logs for end-to-end testing.

#### Low-risk example (likely `sanitized_forward`, or `raw_forward` if raw forwarding enabled)

```bash
curl -sS http://127.0.0.1:8080/v1/debug/explain \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5:3b",
    "messages": [
      {"role": "user", "content": "Summarize this generic paragraph about weather."}
    ]
  }'
```

#### Medium-risk example (email only → `sanitized_forward`)

```bash
curl -sS http://127.0.0.1:8080/v1/debug/explain \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5:3b",
    "messages": [
      {"role": "user", "content": "Contact me at alice@example.com about the invoice."}
    ]
  }'
```

#### High-risk example (email + phone → `high_abstraction`, `egress: true`)

```bash
curl -sS http://127.0.0.1:8080/v1/debug/explain \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5:3b",
    "messages": [
      {"role": "user", "content": "alice@example.com and 555-123-4567 need follow-up."}
    ]
  }'
```

#### Critical-risk example (4 detections → `critical_blocked` or `critical_local_only`)

```bash
curl -sS http://127.0.0.1:8080/v1/debug/explain \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen2.5:3b",
    "messages": [
      {"role": "user", "content": "a@example.com b@example.com 555-123-4567 123-45-6789"}
    ]
  }'
```

If `LPG_CRITICAL_LOCAL_ONLY=true`, the explain response should show `route: "critical_local_only"` and `egress: false`.
Otherwise it should show `route: "critical_blocked"` and `egress: false`.

Example fields to inspect in the JSON response:
- `sanitized_input`
- `detections`
- `risk_score`
- `risk_category`
- `route`
- `egress`
- `hard_block`
- `mappings[]`

---

## Architecture Map (PRD Traceability)

- Pipeline and trust boundaries: PRD section 5 (`docs/lpg_prd_v2.md`)
- Deterministic sanitization + mappings: PRD 6.1
- Sensitivity scoring + category escalation: PRD 6.2
- Proxy interface contract and reliability semantics: PRD 6.6
- Security and governance controls: PRD section 8
- Audit chain-of-custody fields: PRD 8.5
- Success metrics M1–M8 and TV taxonomy: PRD sections 10 and 11.2

---

## Repository Layout

- `cmd/lpg/`: binary entrypoint
- `internal/sanitizer/`: deterministic masking and surrogate mapping records
- `internal/risk/`: risk scoring
- `internal/router/`: category and route decision engine
- `internal/proxy/`: `/v1/chat/completions` handler and upstream adapter interfaces
- `internal/audit/`: append-only redacted audit chain records + chain verification
- `test/integration/`, `test/reliability/`, `test/leakage/`, `test/redteam/`: test suites aligned to TV taxonomy
- `docs/testing/test-matrix.md`: M1–M8 and TV mapping to tests/jobs
- `.claude/agents/` and `.claude/commands/`: project-local multiagent workflows
