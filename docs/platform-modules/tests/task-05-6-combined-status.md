---
task: "05-6 - TestCombined_StatusAggregation"
file: tests/integration/platform/platform_combined_test.go
depends_on: task-05-4-combined-modules.md
started:
completed:
---

# Task 05-6: TestCombined_StatusAggregation

Add `TestCombined_StatusAggregation` to
`tests/integration/platform/platform_combined_test.go`.

## Test: `TestCombined_StatusAggregation`

DSCI and DSC should each report status from the module CRs they own, while the
Platform controller reports aggregate readiness from `PlatformModule` objects.

**Setup**:
- Same combined setup as task 05-4.
- Create DSCI with monitoring managed and DSC with aigateway managed.

**Action**:
- Patch the monitoring operand CR status and trigger DSCI reconciliation if needed.
- Patch the aigateway operand CR status and trigger DSC reconciliation if needed.

**Assertions**:
- DSCI conditions reflect monitoring readiness only.
- DSC conditions reflect aigateway readiness only.
- Platform conditions aggregate the corresponding `PlatformModule` readiness.

## Verification

```bash
go test -failfast -timeout 2m -count=1 -run 'TestCombined_StatusAggregation' ./tests/integration/platform/...
```
