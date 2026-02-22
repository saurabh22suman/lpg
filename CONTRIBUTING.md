# Contributing

## Development policy

This repository is TDD-first and PRD-traceable by default.

Every implementation task and PR must include:

1. PRD clause references (for example `6.1`, `6.2`, `8.5`).
2. TV group references (for example `TV-DET`, `TV-ROUTE`, `TV-LEAK`).
3. Risk category touched (`Low`, `Medium`, `High`, `Critical`, or `N/A`).
4. Tests added or updated.
5. Leak-safety impact statement.

## Branch and PR standards

- Use short-lived topic branches from main.
- Keep PRs focused on one vertical slice or one gate-hardening step.
- Include a PR summary with:
  - requirement references (PRD + TV)
  - implementation scope
  - verification output
  - explicit non-goals for the PR

## Required checks

All PRs must pass:

- formatting (`make fmt-check`)
- lint (`make lint`)
- unit/integration tests (`make test`)
- race tests (`make test-race`)
- coverage gate (`make coverage`)
- security gate (`make security`)

Merges require green CI and no skipped security gates.

## TDD workflow (mandatory)

1. Add or update failing tests first (`red`).
2. Implement the minimal code to pass tests (`green`).
3. Refactor only when tests stay green and scope remains unchanged.

For this phase, write tests for first-slice IDs before broadening feature scope:

- `TV-DET-001`
- `TV-ROUTE-001`
- `TV-ROUTE-002`
- `TV-REL-001`
- `TV-LEAK-001`

## Multiagent handoff protocol

Work should follow:

1. **architect**: define scope and acceptance criteria with PRD + TV references.
2. **backend**: implement smallest passing increment.
3. **qa**: validate test additions and integration/reliability behavior.
4. **security**: verify leakage/injection/fail-closed outcomes.

Handoffs must include explicit acceptance criteria and unresolved risks.
