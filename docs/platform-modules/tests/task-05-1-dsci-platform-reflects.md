---
task: "05-1 - TestDSCIDriven_PlatformReflectsDSCI"
file: tests/integration/platform/platform_dsci_test.go
depends_on: task-01-2-helpers.md
started:
completed:
---

# Task 05-1: TestDSCIDriven_PlatformReflectsDSCI

Add `TestDSCIDriven_PlatformReflectsDSCI` to
`tests/integration/platform/platform_dsci_test.go`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

Assume monitoring is already module-backed for this test. Use injected module
handlers and a test CRD; do not depend on the real monitoring controller.

## Test: `TestDSCIDriven_PlatformReflectsDSCI`

DSCI should project monitoring enablement into the Platform CR.

**Setup**:
- One module handler `"monitoring"` backed by a test module GVK.
- `moduleReg`, `componentReg := &cr.Registry{}`, `provisionReg := provision.NewRegistry()`.
- Call `startAllControllers`.
- Register the monitoring test CRD with `registerModuleCRD(...)`.
- Create `GatewayConfig`.

**Action**:
- Create DSCI with `Monitoring.ManagementState = Managed`.

**Assertions**:
- Platform exists.
- `Platform.Spec.Modules.Monitoring.ManagementState == "Managed"`.

## Verification

```bash
go test -failfast -timeout 2m -count=1 -run 'TestDSCIDriven_PlatformReflectsDSCI' ./tests/integration/platform/...
```
