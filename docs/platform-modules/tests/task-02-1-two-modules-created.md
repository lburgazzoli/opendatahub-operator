---
task: "02-1 - TestPlatformOnly_TwoModules_Created"
file: tests/integration/platform/platform_only_test.go
depends_on: task-01-1-core.md, task-01-2-helpers.md
started:
completed:
---

# Task 02-1: TestPlatformOnly_TwoModules_Created

Create `tests/integration/platform/platform_only_test.go` (package `platform_test`)
with the first test function.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**: `tests/integration/platform/suite_test.go` from Task 01 must
exist and provide `startAllControllers`, `suiteOpts`, `createPlatform`,
`createGatewayConfig`, `testModuleAGVK`, `testModuleBGVK`.

## File to Create

`tests/integration/platform/platform_only_test.go`

## Scenario

No DSC involved. The Platform CR is the direct driver. Tests exercise the
Platform + PlatformModule controllers together.

## Test: `TestPlatformOnly_TwoModules_Created`

Verifies that creating a Platform CR with two modules Managed causes two
PlatformModule CRs to be created.

**Setup**:
- `moduleReg := modules.NewRegistry()` -- empty, no handlers needed for this test.
- `provisionReg := provision.NewRegistry()` -- empty.
- `componentReg := &cr.Registry{}` -- empty.
- Call `startAllControllers(t, suiteOpts{...})`.
- Call `createGatewayConfig(t, tc)`.

**Action**:
- `createPlatform(t, tc, ...)` with both `Monitoring` and `AIGateway` set to
  `operatorv1.Managed`.

**Assertions** (use `wt := tc.NewWithT(t)` then `wt.Get(...).Eventually().Should(...)`):
1. PlatformModule CR `"monitoring"` exists.
2. PlatformModule CR `"aigateway"` exists.
3. Platform `status.modules` contains both `"monitoring"` and `"aigateway"`.
   Use `jq.Match` on `gvk.Platform`:
   ```go
   jq.Match(`.status.modules | sort == ["aigateway","monitoring"]`)
   ```

## Status Condition Constants

Use these from `internal/controller/status/status.go`:
- `status.ConditionTypeReady` = `"Ready"`
- `status.ConditionTypeModulesReady` = `"ModulesReady"`
- `status.ConditionTypeProvisioningProgress` = `"ProvisioningProgress"`

## Verification

```bash
go test -v -count=1 -run 'TestPlatformOnly_TwoModules_Created' ./tests/integration/platform/...
```
