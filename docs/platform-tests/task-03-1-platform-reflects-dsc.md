---
task: "03-1 - TestDSCDriven_PlatformReflectsDSC"
file: tests/integration/platform/dsc_driven_test.go
depends_on: task-01-1-core.md, task-01-2-helpers.md
started:
completed:
---

# Task 03-1: TestDSCDriven_PlatformReflectsDSC

Create `tests/integration/platform/dsc_driven_test.go` (package `platform_test`)
with the first DSC-driven test function.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**: `suite_test.go` provides `startAllControllers`, `suiteOpts`,
`createDSCI`, `createDSC`, `createGatewayConfig`.

## File to Create

`tests/integration/platform/dsc_driven_test.go`

## Scenario

DSC is the driver. The DSC controller creates component CRs, module operand CRs,
and SSA-patches the Platform CR. The Platform controller reacts and creates
PlatformModule CRs. The PlatformModule controller deploys module operators.

**DSC's `checkPreConditions` requires both a DSCI and a DSC to exist.** Every test
must call `createDSCI(t, tc)` before `createDSC(t, tc, spec)`.

## Test: `TestDSCDriven_PlatformReflectsDSC`

Verifies `syncPlatformModules` in the DSC controller: when DSC has
`AIGateway=Managed`, the DSC controller SSA-patches Platform CR with
`spec.modules.aigateway.managementState=Managed`.

**Setup**:
- `moduleReg := modules.NewRegistry()` -- empty.
- `provisionReg := provision.NewRegistry()` -- empty.
- `componentReg := &cr.Registry{}` -- empty.
- Call `startAllControllers(t, suiteOpts{...})`.
- Call `createGatewayConfig(t, tc)`.
- Call `createDSCI(t, tc)`.

**Action**:
- `createDSC(t, tc, dscv2.DataScienceClusterSpec{...})` with:
  ```go
  Components: dscv2.Components{
      AIGateway: componentApi.DSCAIGateway{
          ManagementSpec: common.ManagementSpec{
              ManagementState: operatorv1.Managed,
          },
      },
  }
  ```

**Assertions**:
```go
wt.Get(gvk.Platform, types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}).
    Eventually().Should(
        jq.Match(`.spec.modules.aigateway.managementState == "Managed"`),
    )
```

This mirrors `TestDSCReconciler_PlatformCRSyncedWithEnabledModules` in
`internal/controller/datasciencecluster/datasciencecluster_controller_test.go:291-314`
but with all controllers running together.

## Imports

In addition to suite imports, this file needs:

```go
import (
    componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
    dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
    operatorv1 "github.com/openshift/api/operator/v1"
)
```

## Verification

```bash
go test -v -count=1 -run 'TestDSCDriven_PlatformReflectsDSC' ./tests/integration/platform/...
```
