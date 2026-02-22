# Agent workflow

This document defines the project-local multiagent operating model for LPG phase 1.

## Mandatory traceability for every task/PR

Each task and PR must include:

- PRD clause references (for example `5.1`, `6.1`, `6.2`, `6.6`, `8.5`)
- TV references (for example `TV-DET`, `TV-ROUTE`, `TV-REL`, `TV-LEAK`)
- Risk category touched (`Low|Medium|High|Critical|N/A`)
- Tests added or changed
- Leak-safety impact statement

## Handoff protocol

Handoff order is mandatory:

1. **architect → backend**
2. **backend → qa**
3. **qa → security**

No handoff may skip a role.

### Architect acceptance criteria

- scope tied to PRD clauses and non-goals
- trust boundary impact called out (TB-1..TB-6)
- explicit backend acceptance criteria written

### Backend acceptance criteria

- failing tests added first (red)
- minimal implementation only (green)
- changed files and known limitations documented

### QA acceptance criteria

- required TV groups covered
- deterministic outcomes asserted
- integration/reliability checks executed for touched route paths

### Security acceptance criteria

- leakage surfaces evaluated for changed scope
- fail-closed behavior verified for critical/error paths
- security gates (vuln + secret scan + relevant tests) passing

## Task cadence

1. Create task with PRD + TV IDs.
2. QA/security define or confirm failing tests.
3. Backend implements minimal passing code.
4. Architect validates boundaries and non-goals.
5. Security gate and CI must be green before merge.

## Review payload template

- PRD refs:
- TV refs:
- Risk category:
- Trust boundaries:
- Tests:
- Leak-safety impact:
- Non-goals preserved:
- Handoff acceptance met by:
