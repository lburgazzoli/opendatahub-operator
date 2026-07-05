---
task: "05-3 - TestDSCIDriven_DisableModule_Cleanup"
file: tests/integration/platform/platform_dsci_test.go
depends_on: task-05-2-dsci-module-created.md
started:
completed:
---

# Task 05-3: TestDSCIDriven_DisableModule_Cleanup

Add `TestDSCIDriven_DisableModule_Cleanup` to
`tests/integration/platform/platform_dsci_test.go`.

## Test: `TestDSCIDriven_DisableModule_Cleanup`

When DSCI disables monitoring, the DSCI-owned monitoring operand CR should be
deleted and Platform should remove `PlatformModule/monitoring`.

**Setup**:
- Same as task 05-2.
- Create DSCI with `Monitoring.ManagementState = Managed`.
- Wait for the monitoring operand CR and `PlatformModule/monitoring` to exist.

**Action**:
- Update the DSCI instance to `Monitoring.ManagementState = Removed`.

**Assertions**:
- The monitoring operand CR is deleted.
- `PlatformModule/monitoring` is deleted.
- `Platform.Spec.Modules.Monitoring.ManagementState == "Removed"` or the field is
  omitted consistently with the chosen implementation.

## Verification

```bash
go test -failfast -timeout 2m -count=1 -run 'TestDSCIDriven_DisableModule_Cleanup' ./tests/integration/platform/...
```
