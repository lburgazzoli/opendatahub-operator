---
task: "05-4 - TestCombined_DSCAndDSCI_ModulesCombined"
file: tests/integration/platform/platform_combined_test.go
depends_on: task-05-3-dsci-disable-cleanup.md
started:
completed:
---

# Task 05-4: TestCombined_DSCAndDSCI_ModulesCombined

Add `TestCombined_DSCAndDSCI_ModulesCombined` to
`tests/integration/platform/platform_combined_test.go`.

## Test: `TestCombined_DSCAndDSCI_ModulesCombined`

DSCI and DSC should both be able to project modules into the same Platform CR.

**Setup**:
- One module handler `"monitoring"` backed by a test GVK.
- One module handler `"aigateway"` backed by a test GVK.
- Shared `moduleReg`, isolated `provisionReg`.
- Register both test CRDs.
- Create `GatewayConfig`.

**Action**:
- Create DSCI with `Monitoring.ManagementState = Managed`.
- Create DSC with `AIGateway.ManagementState = Managed`.

**Assertions**:
- `Platform.Spec.Modules.Monitoring.ManagementState == "Managed"`.
- `Platform.Spec.Modules.AIGateway.ManagementState == "Managed"`.
- Both `PlatformModule/monitoring` and `PlatformModule/aigateway` exist.

## Verification

```bash
go test -failfast -timeout 2m -count=1 -run 'TestCombined_DSCAndDSCI_ModulesCombined' ./tests/integration/platform/...
```
