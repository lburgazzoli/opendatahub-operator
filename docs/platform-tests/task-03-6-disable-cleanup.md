---
task: "03-6 - TestDSCDriven_DisableComponent_Cleanup"
file: tests/integration/platform/dsc_driven_test.go
depends_on: task-03-1-platform-reflects-dsc.md
started:
completed:
---

# Task 03-6: TestDSCDriven_DisableComponent_Cleanup

Add `TestDSCDriven_DisableComponent_Cleanup` to
`tests/integration/platform/dsc_driven_test.go`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**: `suite_test.go` provides `startAllControllers`, `suiteOpts`,
`createDSCI`, `createDSC`, `createGatewayConfig`.

## Test: `TestDSCDriven_DisableComponent_Cleanup`

Verifies that disabling a component on DSC causes its component CR to be deleted.

**Setup**:
- One component handler `"dashboard"` with:
  ```go
  IsEnabledFn: func(dsc *dscv2.DataScienceCluster) bool {
      return dsc.Spec.Components.Dashboard.ManagementState == operatorv1.Managed
  },
  NewCRObjectFn: func(_ context.Context, _ client.Client, _ *dscv2.DataScienceCluster) (common.PlatformObject, error) {
      return &componentApi.Dashboard{
          ObjectMeta: metav1.ObjectMeta{Name: "default-dashboard"},
      }, nil
  },
  ```
- Empty module registry.
- Call `startAllControllers`, `createGatewayConfig`, `createDSCI`.

**Action**:
1. Create DSC with Dashboard=Managed:
   ```go
   createDSC(t, tc, dscv2.DataScienceClusterSpec{
       Components: dscv2.Components{
           Dashboard: componentApi.DSCDashboard{
               ManagementSpec: common.ManagementSpec{ManagementState: operatorv1.Managed},
           },
       },
   })
   ```

2. Wait for Dashboard CR to exist:
   ```go
   wt.Get(gvk.Dashboard, types.NamespacedName{Name: "default-dashboard"}).
       Eventually().Should(Succeed())
   ```

3. Update DSC to disable Dashboard:
   ```go
   dsc := &dscv2.DataScienceCluster{}
   NewWithT(t).Expect(tc.Client().Get(context.Background(),
       types.NamespacedName{Name: "default-dsc"}, dsc)).Should(Succeed())
   dsc.Spec.Components.Dashboard.ManagementState = operatorv1.Removed
   NewWithT(t).Expect(tc.Client().Update(context.Background(), dsc)).Should(Succeed())
   ```

**Assertions**:
- Dashboard CR deleted (get returns not found):
  ```go
  g.Eventually(func() error {
      return tc.Client().Get(context.Background(),
          types.NamespacedName{Name: "default-dashboard"},
          &componentApi.Dashboard{})
  }).Should(MatchError(ContainSubstring("not found")))
  ```

## Verification

```bash
go test -v -count=1 -run 'TestDSCDriven_DisableComponent_Cleanup' ./tests/integration/platform/...
```
