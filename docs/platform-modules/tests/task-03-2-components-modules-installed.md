---
task: "03-2 - TestDSCDriven_ComponentsAndModules_Installed"
file: tests/integration/platform/dsc_driven_test.go
depends_on: task-03-1-platform-reflects-dsc.md
started:
completed:
---

# Task 03-2: TestDSCDriven_ComponentsAndModules_Installed

Add `TestDSCDriven_ComponentsAndModules_Installed` to
`tests/integration/platform/dsc_driven_test.go`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**: `suite_test.go` provides `startAllControllers`, `suiteOpts`,
`createDSCI`, `createDSC`, `createGatewayConfig`, `registerModuleCRD`,
`newTestModuleHandler`, `testModuleAGVK`.

## Test: `TestDSCDriven_ComponentsAndModules_Installed`

Verifies the full chain: DSC creates component CRs + module CRs, Platform CR
gets synced, PlatformModule CRs get created.

**Setup**:
- One test component using `cr.BaseComponentHandler`:
  ```go
  componentReg := &cr.Registry{}
  componentReg.Add(&cr.BaseComponentHandler{
      Name: "dashboard",
      GVK:  gvk.Dashboard,
      IsEnabledFn: func(_ *dscv2.DataScienceCluster) bool { return true },
      NewCRObjectFn: func(_ context.Context, _ client.Client, _ *dscv2.DataScienceCluster) (common.PlatformObject, error) {
          return &componentApi.Dashboard{
              ObjectMeta: metav1.ObjectMeta{Name: "default-dashboard"},
          }, nil
      },
      UpdateDSCStatusFn: func(_ context.Context, _ *rrtypes.ReconciliationRequest) (metav1.ConditionStatus, error) {
          return metav1.ConditionTrue, nil
      },
  })
  ```
- One test module handler: `"aigateway"` with `testModuleAGVK`.
  Use the base `testModuleHandler` which always returns `true` from `IsEnabled`.
- `moduleReg := modules.NewRegistry()`; add `"aigateway"` handler.
- `provisionReg := provision.NewRegistry()` -- empty (no DAG ordering needed).
- Call `startAllControllers(t, suiteOpts{...})`.
- Register module CRD: `registerModuleCRD(t, et, testModuleAGVK)`.
- Call `createGatewayConfig(t, tc)`.
- Call `createDSCI(t, tc)`.

**Action**:
- `createDSC(t, tc, dscv2.DataScienceClusterSpec{...})` with AIGateway=Managed.

**Assertions**:
1. Component CR created by DSC's `provisionComponents` + deploy:
   ```go
   wt.Get(gvk.Dashboard, types.NamespacedName{Name: "default-dashboard"}).
       Eventually().Should(Succeed())
   ```

2. Module operand CR created by DSC's `provisionModuleCRs` + deploy
   (the `testModuleHandler.BuildModuleCR` returns an unstructured CR with the
   test GVK):
   ```go
   wt.Get(testModuleAGVK, types.NamespacedName{Name: "default-aigateway"}).
       Eventually().Should(Succeed())
   ```

3. Platform CR synced with aigateway=Managed:
   ```go
   wt.Get(gvk.Platform, types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}).
       Eventually().Should(
           jq.Match(`.spec.modules.aigateway.managementState == "Managed"`),
       )
   ```

4. PlatformModule CR created by Platform controller:
   ```go
   wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
       Eventually().Should(Succeed())
   ```

## Additional Imports

```go
import (
    rrtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)
```

## Verification

```bash
go test -v -count=1 -run 'TestDSCDriven_ComponentsAndModules_Installed' ./tests/integration/platform/...
```
