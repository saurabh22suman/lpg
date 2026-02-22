# LLM Privacy Gateway (LPG) v2.1 - Product Requirements Document

## 1. Product Vision

LPG is a lightweight, open-source Meaning Firewall for LLM traffic.

It intercepts LLM API calls, evaluates semantic risk, sanitizes sensitive information, optionally compresses structured data using token-efficient formats (e.g., TOON), routes requests intelligently, and safely rehydrates responses while remaining transparent and auditable.

### Why This Exists

Modern AI usage has three tensions:

- Privacy vs Cloud Intelligence
- Cost vs Context Size
- Speed vs Local Sovereignty

LPG balances these tensions without forcing extreme tradeoffs.

------------------------------------------------------------------------

## 2. Target Users

### Primary

- Developers using OpenAI-compatible APIs
- Automation builders (Home Assistant, Node-RED)
- Privacy-conscious hybrid AI users

### Secondary

- Small teams experimenting with multi-model routing
- Cost-sensitive AI builders optimizing token usage

------------------------------------------------------------------------

## 3. Core Value Proposition

LPG provides:

1. Privacy-aware prompt interception
2. Risk-based routing logic
3. Semantic abstraction for sensitive content
4. Token-efficient serialization (TOON support)
5. Structured, safe rehydration
6. Transparent preview and audit

------------------------------------------------------------------------

## 4. Product Principles

### 4.1 Fail Closed by Default

If sanitization fails or confidence is low, block the external call.

### 4.2 Deterministic Before Probabilistic

Apply rule-based masking before invoking LLM abstraction.

### 4.3 Mask → Abstract → Compress

Sensitive masking first, abstraction second, compression third.

### 4.4 Store Policies, Not Secrets

Persist configuration, not raw prompt data by default.

### 4.5 Transparency is Mandatory

Expose masking, abstraction, compression, and token savings clearly.

------------------------------------------------------------------------

## 5. System Architecture

Pipeline:

Input
→ Deterministic Sanitization
→ Sensitivity Scoring
→ Routing Decision
→ Optional Local LLM Abstraction
→ Structured Data Detection
→ Optional TOON Conversion
→ External API Call
→ Response
→ Safe Rehydration
→ Output

### 5.1 Trust Boundaries & Data States

#### Trust boundaries

- **TB-1 Client Ingress Boundary**: external caller to LPG (untrusted input).
- **TB-2 Internal Processing Boundary**: deterministic sanitizer, scorer, router, TOON modules.
- **TB-3 Local Model Boundary**: local LLM runtime used for abstraction.
- **TB-4 External Provider Boundary**: any outbound call to configured upstream LLM API.
- **TB-5 Persistence Boundary**: config, optional encrypted state, and audit artifacts.
- **TB-6 Operator Boundary**: CLI/admin actions that can change policy or reveal previews.

#### Data states

| State | Description | Persistence default |
|---|---|---|
| S0 Raw Input | Original request payload before transformations | MUST NOT persist |
| S1 Sanitized | Deterministically masked payload | In-memory only |
| S2 Risk-Tagged | Sanitized payload plus score/category/routing metadata | In-memory only |
| S3 Abstracted | Local-LLM abstracted payload (if used) | In-memory only |
| S4 TOON Payload | TOON-encoded payload (if used) | In-memory only |
| S5 External Payload | Final payload sent across TB-4 | Transit only |
| S6 External Response | Upstream response prior to rehydration | In-memory only |
| S7 Rehydrated Output | Safe user-facing output | In-memory only by default |

#### Normative routing invariants

- **Critical** risk requests MUST NOT cross TB-4.
- If risk confidence is below threshold, routing MUST escalate to stricter handling.
- S0 MUST NOT be written to disk unless an explicit exception is configured and audited.
- Any transition error between states MUST fail closed and return a policy-safe error.

------------------------------------------------------------------------

## 6. Feature Requirements

### 6.1 Deterministic Sanitization Layer

#### Inputs

- Raw prompt text and supported textual attachments
- Built-in PII/entity patterns
- YAML-configurable custom masking rules
- Entity policy (hard-block types, deterministic surrogate format)

#### Expected behavior

- Detect sensitive spans using deterministic rules first.
- Replace detected spans with stable surrogate values.
- Emit mapping records using format: `{ placeholder, original_value, entity_type, confidence_score }` where `placeholder` stores the surrogate value.

#### Acceptance criteria

1. Built-in entity corpus achieves **recall ≥ 0.99** and **precision ≥ 0.98** for deterministic classes.
2. Surrogate format is deterministic and stable per request (for example `personN@example.net`, `555-010-XXXX`, `900-00-XXXX`) for 100% of replacements.
3. 100% of detections produce mapping entries with all required fields.
4. Sanitized payload contains zero exact matches for all detected original values.

#### Failure / edge-case behavior

- Invalid regex in config MUST fail startup with `ERR_CONFIG_VALIDATION`.
- Runtime sanitizer failure MUST block outbound call with `ERR_SANITIZATION_FAILURE`.
- Overlapping matches MUST resolve deterministically (longest-span-first, then earliest index).

#### Verification method

- Unit tests for built-in patterns and overlap resolution
- Config validation tests for malformed custom rules
- Golden-file tests validating mapping and surrogate integrity

---

### 6.2 Sensitivity Scoring Engine

#### Inputs

- S1 sanitized payload
- Deterministic detection metadata
- Scoring policy weights and category thresholds

#### Expected behavior

- Produce `risk_score` (0-100 integer), `risk_category`, and `risk_confidence`.
- Apply deterministic routing using fixed risk bands.

Risk bands:

| Category | Score range | Default route |
|---|---|---|
| Low | 0-24 | Raw forwarding only if explicitly allowed |
| Medium | 25-49 | Deterministic sanitization only |
| High | 50-74 | Deterministic + local abstraction |
| Critical | 75-100 | Local-only fallback or block |

Routing triggers:

- **Low**: External raw forwarding allowed only when `allow_raw_forwarding=true` and no hard-block entities are present.
- **Medium**: External call uses S1 sanitized payload; no raw payload allowed.
- **High**: Local abstraction is mandatory before external call.
- **Critical**: External call prohibited; return local-only response or policy block.
- If `risk_confidence < 0.70`, escalate one category (capped at Critical).

#### Acceptance criteria

1. 100% of requests receive integer score, category, confidence, and route decision.
2. Category assignment strictly follows fixed score ranges with no overlap.
3. Route decision is deterministic for identical input + config.
4. Labeled evaluation set achieves **macro-F1 ≥ 0.90** for category prediction.

#### Failure / edge-case behavior

- Missing scoring config MUST fail startup.
- Scoring runtime error MUST escalate to Critical route and fail closed.
- Unknown category values MUST be rejected and treated as Critical.

#### Verification method

- Unit tests for threshold boundaries and escalation rules
- Regression tests with labeled risk fixtures
- Determinism tests over repeated identical inputs

---

### 6.3 Optional Local LLM Abstraction

#### Inputs

- High-risk S2 payload selected for abstraction
- Local model endpoint/runtime configuration
- Strict JSON output schema

#### Expected behavior

- Rewrite sensitive context while preserving user intent.
- Reduce token volume before external call.
- Emit schema-valid JSON only; reject invalid model outputs.

#### Acceptance criteria

1. 100% abstraction outputs validate against required JSON schema.
2. On high-risk benchmark set, median token reduction is **≥ 15%** vs pre-abstraction input.
3. Zero mapped `original_value` strings from 6.1 appear in abstraction output.
4. Invalid abstraction outputs never cross TB-4.

#### Failure / edge-case behavior

- Local model unavailable or timeout on High route MUST fail closed (`ERR_ABSTRACTION_UNAVAILABLE`).
- Schema-invalid abstraction output MUST be rejected and not retried with raw content.
- If abstraction quality checks fail, request MUST block unless policy explicitly allows local-only response.

#### Verification method

- Schema conformance tests with malformed model outputs
- Leakage tests against mapping values
- Token delta benchmarks on fixed corpora

---

### 6.4 Structured Data Detection

#### Inputs

- Candidate payload after sanitization/abstraction
- TOON eligibility policy thresholds

#### Expected behavior

- Detect TOON eligibility only for structured JSON data.
- Mark request as TOON-eligible only if all numeric gates pass.

TOON eligibility gates (all required):

1. Valid JSON parse succeeds.
2. Top-level value is an array of objects.
3. Row count **≥ 10** objects.
4. Key-set uniformity across rows **≥ 0.95**.
5. Serialized JSON size **≥ 1500 bytes**.
6. Maximum nesting depth **≤ 4**.

#### Acceptance criteria

1. Eligibility flag is deterministic and reproducible for identical payloads.
2. Negative corpus false-positive rate is **≤ 0.01**.
3. Positive corpus false-negative rate is **≤ 0.03**.
4. Non-JSON payloads are always marked ineligible.

#### Failure / edge-case behavior

- JSON parse errors MUST mark ineligible and continue without TOON.
- Mixed-type arrays MUST mark ineligible.
- Excessive nesting (>4) MUST mark ineligible.

#### Verification method

- Property-based tests for JSON shape detection
- Fixture tests covering boundary values for each threshold
- Precision/recall evaluation against labeled eligibility corpus

---

### 6.5 TOON Compression Module

#### Inputs

- TOON-eligible payload from 6.4
- Compression policy thresholds

#### Expected behavior

- Convert eligible JSON to TOON representation.
- Measure estimated and actual token savings.
- Add a concise TOON interpretation note to downstream prompt context.

TOON conversion gates:

- Estimated savings must be **≥ 12%** and **≥ 100 tokens**.
- Keep TOON output only if measured savings after conversion is **≥ 10%**.

#### Acceptance criteria

1. Round-trip equivalence (`decode(encode(JSON))`) is exact for 100% of eligible regression fixtures.
2. TOON interpretation note length is **≤ 120 tokens**.
3. Token delta is logged for 100% of TOON-attempted requests.
4. Requests failing conversion gates fall back to non-TOON payload deterministically.

#### Failure / edge-case behavior

- Conversion or parse failure MUST skip TOON and continue safely without external leak.
- If TOON output fails validation, system MUST revert to sanitized/abstracted JSON.
- Inbound TOON parsing remains out of scope for v2.1 (Phase 2).

#### Verification method

- Round-trip correctness tests with canonical JSON fixtures
- Token accounting regression tests
- Failure injection tests for malformed TOON artifacts

---

### 6.6 Proxy Mode (Primary Interface)

#### Inputs

- OpenAI-compatible HTTP requests (e.g., `/v1/chat/completions`, `/v1/responses`)
- Proxy policy config (timeouts, retries, fallback providers)

#### Expected behavior

- Preserve OpenAI-compatible request/response semantics.
- Apply LPG pipeline before any upstream call.
- Emit deterministic policy-safe error responses when blocked.

Reliability semantics:

- Add `x-lpg-request-id` to every response.
- Support optional `Idempotency-Key` for safe client retries.
- Upstream retry count: max 1 retry for idempotent Low/Medium requests.
- No raw-content retry path for High/Critical requests.

#### Acceptance criteria

1. Contract tests for required OpenAI-compatible fields pass at **100%**.
2. Deterministic-only mode adds **p95 overhead ≤ 300ms**.
3. Under injected provider failures, behavior matches route policy matrix at **100%**.
4. Every processed request yields a unique request ID and route decision log.

#### Failure / edge-case behavior

- Malformed API payloads return deterministic 4xx validation errors.
- If no fallback provider is available, return explicit safe 5xx without leaking payload.
- Critical requests MUST never be forwarded externally during provider failover.

#### Verification method

- API contract tests against OpenAI-compatible fixtures
- Reliability tests with upstream timeout/5xx fault injection
- Latency benchmarks under deterministic and full pipeline modes

---

### 6.7 CLI Mode (Secondary Interface)

#### Inputs

- Commands: `lpg proxy`, `lpg send`, `lpg preview`, `lpg config init`
- Optional flags: `--config`, `--output`, `--profile`, `--strict`

#### Expected behavior

- Provide deterministic command behavior for local operation and automation.
- Return machine-readable outputs for CI and scripting.

#### Acceptance criteria

1. All commands support non-interactive execution suitable for scripts.
2. `lpg config init` generates schema-valid config with secure defaults.
3. Exit codes are stable and documented:
   - `0` success
   - `2` policy block
   - `3` upstream/provider failure
   - `4` config/validation failure
4. `lpg preview` output includes risk band, route, transformations, and token delta when applicable.

#### Failure / edge-case behavior

- Unknown commands/flags produce deterministic validation error output.
- Missing config path returns `4` with actionable error.
- CLI MUST NOT print raw secrets unless explicit raw preview is enabled.

#### Verification method

- CLI integration tests for command matrix and exit codes
- Golden output tests for JSON/text modes
- Shell automation tests in CI

---

### 6.8 Preview & Audit Mode

#### Inputs

- Processed request/response artifacts
- Audit and preview policy config

#### Expected behavior

- Display a clear transformation trace:
  - Original prompt (redacted by default)
  - Sanitized prompt
  - Abstraction result (if applied)
  - TOON output (if applied)
  - Mapping table summary
  - Risk score/category
  - Token reduction percentage
- Persist local audit events without outbound telemetry.

#### Acceptance criteria

1. Preview output includes all required trace elements for 100% of previewed requests.
2. Audit records are generated for 100% of processed requests when audit mode is enabled.
3. Raw values are hidden by default in preview and audit outputs.
4. Audit chain verification succeeds for non-tampered logs at 100%.

#### Failure / edge-case behavior

- If audit write fails and `strict_audit=true`, request MUST fail closed.
- If audit write fails and `strict_audit=false`, request may continue but MUST emit local warning event.
- `--show-raw` requires explicit config permission and operator intent.

#### Verification method

- Preview rendering tests with all route paths
- Audit append and verification tests
- Tamper simulation tests for chain integrity checks

---

### 6.9 Rehydration Guard

#### Inputs

- External response payload
- Mapping table from sanitization phase

#### Expected behavior

- Replace only known surrogate tokens from trusted mapping.
- Ignore unknown or malformed surrogate tokens.
- Validate entity type before substitution.

#### Acceptance criteria

1. 100% of substitutions reference existing mapping keys.
2. Unknown surrogate tokens are never substituted.
3. Type validation passes for 100% of applied substitutions.
4. No unmapped secret values are introduced during rehydration.

#### Failure / edge-case behavior

- Mapping corruption or mismatch MUST block substitution and return safe error.
- Surrogate-token collisions MUST resolve deterministically and be logged.
- If response contains malicious surrogate-token injection attempts, replacements are skipped for injected tokens.

#### Verification method

- Rehydration unit tests with valid/invalid mappings
- Injection-focused adversarial tests for surrogate-token spoofing
- End-to-end leak checks from input to output

---

### 6.10 Configuration & Memory

#### Inputs

- YAML config files
- Optional encrypted persistence backend
- Runtime policy/profile selection

#### Expected behavior

- Store rules, thresholds, and preferences under versioned schema.
- Keep sensitive runtime artifacts in memory by default.
- Provide explicit wipe command for local sensitive state.

#### Acceptance criteria

1. Config schema validation runs at startup and blocks invalid config.
2. Secure defaults are enforced when omitted:
   - `fail_closed=true`
   - `telemetry=false`
   - `audit_redacted=true`
3. Raw prompts are not persisted by default under any mode.
4. Wipe command removes configured encrypted state and invalidates local keys used by LPG state.

#### Failure / edge-case behavior

- Corrupt config MUST fail startup with explicit validation errors.
- Unknown config keys MUST be rejected in strict mode.
- Encryption backend errors MUST block persistence and raise local alert.

#### Verification method

- Schema validation tests across config versions
- Startup tests for secure-default behavior
- Persistence/wipe tests for encrypted and non-encrypted modes

------------------------------------------------------------------------

## 7. Technical Stack

- Language: Go
- Local LLM: llama.cpp (GGUF quantized)
- Config: YAML
- Optional DB: BoltDB or SQLite
- Distribution: Static binary + minimal Docker image
- Dependency hygiene: pinned versions and integrity checks for release builds

------------------------------------------------------------------------

## 8. Security & Governance Requirements

### 8.1 Threat Model and Abuse Cases

Primary abuse cases in scope:

- Direct prompt injection attempting policy bypass
- Indirect injection via retrieved/tool-provided text
- Sensitive data exfiltration attempts
- Upstream provider outage/failure under strict policy
- Local artifact tampering (config/audit/mapping)
- Unauthorized operator access to admin or raw preview capabilities

OWASP LLM Top 10 mapping:

| OWASP LLM risk | LPG control focus |
|---|---|
| LLM01 Prompt Injection | Input/output injection filters, risk escalation, fail-closed routing |
| LLM02 Insecure Output Handling | Rehydration guard, strict surrogate-token validation |
| LLM03 Training Data Poisoning | Signed model/config artifacts and supply-chain controls |
| LLM04 Model DoS | Payload limits, timeout/retry limits, bounded local model calls |
| LLM05 Supply Chain Vulnerabilities | Dependency pinning, artifact verification, controlled updates |
| LLM06 Sensitive Information Disclosure | Deterministic sanitization + zero leakage gates |
| LLM07 Insecure Plugin Design | Strict egress allowlist, no implicit tool/plugin execution |
| LLM08 Excessive Agency | No autonomous external side effects from model output |
| LLM09 Overreliance | Deterministic precedence, confidence escalation, preview transparency |
| LLM10 Model Theft | Local model file permissions and optional encryption at rest |

NIST AI RMF profile alignment:

| NIST AI RMF function | LPG implementation expectation |
|---|---|
| Govern | Documented policy ownership, exception approvals, release gates |
| Map | Explicit trust boundaries/data states and abuse-case catalog |
| Measure | Quantitative metrics, adversarial tests, FP/FN thresholds |
| Manage | Rollout controls, incident response hooks, rollback policy |

### 8.2 AuthN/AuthZ for Proxy and Admin Interfaces

- Proxy binds to localhost by default.
- Non-local bind requires explicit auth mode (`api_key` or `mTLS`).
- Admin-mutating actions require authenticated operator context.
- Authorization scopes are minimum:
  - `proxy:invoke`
  - `config:read`
  - `config:write`
  - `audit:read`
  - `state:wipe`
- Auth failures MUST return safe 401/403 responses without policy leakage.

### 8.3 Key and Secret Lifecycle

- **Create**: keys/secrets generated or imported through approved interfaces only.
- **Store**: encrypted at rest when persisted; never hardcoded in config defaults.
- **Rotate**: operational secrets rotate at policy interval (default max age: 90 days).
- **Revoke**: compromised keys disabled immediately.
- **Wipe**: explicit wipe invalidates stored keys used for LPG-managed encrypted state.

### 8.4 Data Retention and Encryption Defaults

- No telemetry outbound by default.
- No raw prompt/response persistence by default.
- Audit logs store redacted metadata and hashes, not raw secret values.
- Default audit retention: 30 days (configurable, explicit override required).
- Persisted sensitive local artifacts MUST use encryption at rest.

### 8.5 Log Integrity and Audit Chain-of-Custody

- Audit log is append-only.
- Every audit record includes: timestamp, request_id, policy_version, action summary, `prev_hash`, `entry_hash`.
- `entry_hash = H(record_without_hash + prev_hash)`.
- Chain verification command MUST detect tampering deterministically.
- Verification failures are security incidents and block compliance-ready exports.

### 8.6 Prompt-Injection and Exfiltration Controls

Input controls:

- Detect role override attempts (e.g., “ignore previous instructions”).
- Detect exfiltration patterns targeting surrogate tokens/mappings/secrets.
- Escalate risk for encoded/obfuscated extraction attempts.

Output controls:

- Scan outbound payloads and inbound model responses for forbidden raw entities.
- Block responses attempting to reveal mappings or hidden values.
- Unknown surrogate tokens are never dereferenced.

### 8.7 Security Exceptions and Governance Path

- Any policy exception must be explicit, scoped, and auditable.
- Exception records require: reason, owner, scope, expiry, compensating control.
- Expired exceptions are invalid and MUST fail closed.

------------------------------------------------------------------------

## 9. Non-Goals (v2.1)

- Enterprise dashboards
- Multi-tenant RBAC
- Telemetry analytics
- Automatic prompt learning
- Deep nested TOON support
- Inbound TOON parsing (planned Phase 2)

------------------------------------------------------------------------

## 10. Success Metrics & Release Quality Gates

### 10.1 Must-pass metrics

| ID | Metric | Target | Verification method | Owner |
|---|---|---|---|---|
| M1 | Deterministic overhead | p95 ≤ 300ms | Performance benchmark suite | Engineering |
| M2 | Zero leakage | 0 critical leak events in release suite | End-to-end leakage suite + adversarial checks | Security |
| M3 | Token optimization | Median reduction ≥ 15% on eligible workloads | Token delta benchmark corpus | Engineering |
| M4 | Routing correctness | ≥ 99.5% route-policy conformance | Labeled routing test set + policy replay | Engineering |
| M5 | Fallback reliability | 100% Critical no-egress; ≥ 99.5% safe outcomes for Low/Medium under faults | Fault-injection reliability suite | Engineering + Security |
| M6 | Integration success | 100% pass on required compatibility scenarios | Integration matrix test run | Integrations |
| M7 | Onboarding success | ≥ 90% of evaluators complete setup checklist from docs alone | Structured onboarding trial | Product + DX |
| M8 | Cost governance | Budget guardrails enforce warn/soft-limit/hard-limit actions at configured thresholds | Budget-policy simulation tests | Product + Engineering |

### 10.2 Metric-to-test traceability matrix

| Metric ID | Primary test groups |
|---|---|
| M1 | TV-PERF-01, TV-PERF-02 |
| M2 | TV-LEAK-01..05, TV-REDTEAM-ALL |
| M3 | TV-TOON-03, TV-ABS-02 |
| M4 | TV-ROUTE-01..04 |
| M5 | TV-REL-01..06 |
| M6 | TV-INT-HA, TV-INT-NR, TV-INT-OAI |
| M7 | TV-DX-01 onboarding checklist |
| M8 | TV-COST-01..03 |

------------------------------------------------------------------------

## 11. Test & Verification Strategy

### 11.1 Formal definition of zero leakage

A leakage event occurs if any value classified as sensitive by policy appears in clear form in any forbidden surface:

1. Payload sent across TB-4
2. Local audit logs when redaction is enabled
3. CLI/preview output when raw preview is disabled
4. Error messages returned to clients

Release criterion for M2: **zero critical leakage events** in full verification suite.

### 11.2 Test suite taxonomy

- **TV-DET**: deterministic masking and mapping correctness
- **TV-ROUTE**: score banding and route enforcement
- **TV-ABS**: abstraction schema/leakage/token reduction
- **TV-TOON**: eligibility, conversion, round-trip correctness
- **TV-REL**: provider failure/fallback/retry behavior
- **TV-LEAK**: end-to-end leakage detection
- **TV-INT**: compatibility integrations (OpenAI API clients, Home Assistant, Node-RED)
- **TV-DX**: CLI and onboarding workflow validation
- **TV-COST**: budget policy and guardrail behavior

### 11.3 Adversarial / red-team categories

Minimum release set includes cases for:

- Direct prompt injection
- Indirect injection via retrieved text
- Exfiltration requests against mappings/surrogate tokens
- Encoding obfuscation (base64, unicode confusables)
- Role-play jailbreak attempts
- Provider error manipulation attempts
- Placeholder spoofing in model outputs
- Data-shape attacks against TOON conversion

### 11.4 Detector FP/FN targets

| Detector class | False positive target | False negative target |
|---|---:|---:|
| Deterministic sensitive entity detection | ≤ 2% | ≤ 1% |
| Injection/exfiltration risk detection | ≤ 5% | ≤ 5% |
| TOON eligibility detection | ≤ 1% | ≤ 3% |

### 11.5 Routing and fallback reliability tests

Required checks:

- Upstream timeout, 5xx, connection reset, DNS failure, and TLS failure scenarios
- Route-constrained fallback behavior per risk category
- Critical route never egresses externally under any failure mode
- Safe deterministic error response when no valid fallback exists

### 11.6 TOON correctness and regression checks

- Canonical round-trip equality tests on fixed corpora
- Regression guard for token savings drift (release fails if median savings drops below M3 target)
- Validation that TOON is never applied to ineligible payloads

### 11.7 Scenario walk-through verification

The following scenarios are mandatory release walkthroughs:

1. Sensitive prompt routed under strict policy
2. Prompt injection attempt with exfiltration intent
3. Provider failure + fallback behavior under fail-closed policy
4. Structured JSON request with and without TOON eligibility

Each walkthrough must document: input, route decision, transformation outputs, security outcome, and pass/fail result.

------------------------------------------------------------------------

## 12. Rollout, Versioning & Change Management

### 12.1 Versioning model

- LPG binary uses semantic versioning.
- PRD version increments on normative requirement changes.
- Config schema has explicit version and migration path.
- Policy packs are versioned independently and tied to release notes.

### 12.2 Rollout stages

1. **Local validation**: full test suite + scenario walkthroughs
2. **Canary rollout**: limited traffic/prompt classes with strict audit review
3. **General availability**: default profile after gate pass and exception review

### 12.3 Breaking changes and compatibility

- Breaking API/config behavior requires major version bump.
- Deprecated fields require explicit warning before removal.
- Migration docs and tooling are required for schema-breaking updates.

### 12.4 Change control requirements

- Any threshold/policy default change requires changelog entry with rationale.
- Security-related default changes require Security owner sign-off.
- Exception grants must be re-reviewed at expiry.

### 12.5 Rollback requirements

- Last known-good policy/config versions must be restorable.
- Rollback procedure must preserve audit chain integrity.
- Failed rollout gates automatically halt promotion.

------------------------------------------------------------------------

## 13. Compatibility & Dependency Matrix

| Area | Baseline support | Requirement level | Notes |
|---|---|---|---|
| OpenAI-compatible API | `/v1/chat/completions` | Required | Primary proxy interface |
| OpenAI-compatible API | `/v1/responses` | Recommended | Supported where client expects it |
| Local LLM runtime | llama.cpp (GGUF) | Required for High-risk abstraction | If unavailable, High route fails closed |
| Config format | YAML | Required | Schema-validated at startup |
| Local storage | Filesystem | Required | Redacted/auditable artifacts only |
| Optional storage | BoltDB / SQLite | Optional | For encrypted local state and indexes |
| Automation integration | Home Assistant | Targeted | Must pass integration scenario suite |
| Automation integration | Node-RED | Targeted | Must pass integration scenario suite |
| Deployment | Static binary / minimal Docker image | Required | Local-first operational model |

Compatibility rules:

- If a dependency version is outside validated matrix, LPG starts in strict mode and warns operator.
- Unsupported provider/client features MUST return explicit compatibility errors (not silent fallback).
- TOON compatibility is limited to outbound JSON-array conversion for v2.1.

------------------------------------------------------------------------

## 14. Strategic Identity

LPG is a Meaning Firewall with Economic Optimization.

It controls semantic exposure and token gravity simultaneously.
