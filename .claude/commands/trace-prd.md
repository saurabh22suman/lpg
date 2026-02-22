# /trace-prd

Require explicit requirement traceability before implementation or merge.

## Input
- task description
- changed files (or planned files)

## Output template
- PRD references: `<section-ids>`
- TV references: `<TV-groups/ids>`
- Risk category touched: `<Low|Medium|High|Critical|N/A>`
- Trust boundaries affected: `<TB-*>`
- Leak-safety impact: `<statement>`
- Non-goals reaffirmed: `<list>`

## Rules
- Reject tasks/PRs missing PRD + TV references.
- Reject tasks that expand beyond declared non-goals.
