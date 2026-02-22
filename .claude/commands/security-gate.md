# /security-gate

Run security verification before merge.

## Required checks
- leakage tests for changed paths
- adversarial tests relevant to scope
- vulnerability scan
- secret scan

## Decision output
- status: PASS|FAIL
- failing controls: `<PRD section 8 refs>`
- blocked reason: `<if fail>`
- required remediation: `<actions>`

If any critical control fails, merge is blocked.
