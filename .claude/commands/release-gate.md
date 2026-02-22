# /release-gate

Validate release-readiness against M1–M8 and CI gates.

## Inputs
- CI run results
- test matrix (`docs/testing/test-matrix.md`)
- security gate result

## Required output
- Gate status per metric (M1..M8)
- Failing metric IDs and blocking tests
- Statement that critical route demonstrates no egress
- Statement that no raw sensitive values appear on forbidden surfaces

Release recommendation is FAIL unless all mandatory gates are green.
