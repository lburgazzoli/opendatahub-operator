---
task: "05-2 - TestDSCIDriven_ServiceModuleCreated"
file: tests/integration/platform/platform_dsci_test.go
depends_on: task-05-1-dsci-platform-reflects.md
started:
completed:
---

# Task 05-2: TestDSCIDriven_ServiceModuleCreated

Add `TestDSCIDriven_ServiceModuleCreated` to
`tests/integration/platform/platform_dsci_test.go`.

Assume monitoring is already module-backed for this test. Use injected module
handlers and a test CRD; do not depend on the real monitoring controller.

## Test: `TestDSCIDriven_ServiceModuleCreated`

DSCI should create the monitoring operand CR it owns, and Platform should create
the matching `PlatformModule`.

**Setup**:
- Same helpers and registries as task 05-1.
- Register the monitoring module test CRD.

**Action**:
- Create DSCI with `Monitoring.ManagementState = Managed`.

**Assertions**:
- The monitoring operand CR exists.
- The monitoring operand CR has an owner reference to the DSCI instance.
- `PlatformModule/monitoring` exists.

## Verification

```bash
go test -failfast -timeout 2m -count=1 -run 'TestDSCIDriven_ServiceModuleCreated' ./tests/integration/platform/...
```
