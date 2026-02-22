# Security Policy

## Reporting vulnerabilities

Please report vulnerabilities privately to project maintainers before public disclosure. Include:

- affected component/file
- reproduction steps
- impact and boundary crossed (TB-1..TB-6)
- suggested remediation (if available)

Do not include raw secrets in reports.

## Secure defaults (PRD section 8 + 6.10)

LPG defaults are:

- fail closed by default
- no outbound telemetry by default
- no raw prompt/response persistence by default
- redacted audit logging by default
- deterministic sanitization before any optional probabilistic stage

## Security requirements enforced in this repo

- Critical-risk requests must not egress externally.
- Unknown or malformed surrogate tokens in mapping-aware stages are never dereferenced.
- Errors must be policy-safe and must not leak sensitive values.
- Security exceptions must be explicit, scoped, auditable, and expiring.

## Verification gates

Security checks are required locally and in CI:

- `govulncheck ./...`
- secret scanning (`gitleaks`)
- leakage tests under `test/leakage/`
- adversarial tests under `test/redteam/`

If a strict security gate fails, release promotion is blocked.
