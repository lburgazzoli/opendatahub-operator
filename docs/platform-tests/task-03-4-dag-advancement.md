---
task: "03-4 - TestDSCDriven_DAG_Advancement"
file: tests/integration/platform/dsc_driven_test.go
depends_on: task-03-1-platform-reflects-dsc.md
started:
completed:
---

# Task 03-4: TestDSCDriven_DAG_Advancement

Add `TestDSCDriven_DAG_Advancement` to
`tests/integration/platform/dsc_driven_test.go`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**: `suite_test.go` provides `startAllControllers`, `suiteOpts`,
`createDSCI`, `createDSC`, `createGatewayConfig`, `registerModuleCRD`,
`newTestModuleHandler`, `setUnstructuredReady`, `testModuleAGVK`.

## Test: `TestDSCDriven_DAG_Advancement`

Component at RL10, module at RL20. The Platform controller walks the DAG: the
component must be ready before the module can deploy.

**Setup**:
- One component handler `"dashboard"` with `gvk.Dashboard` (minimal -- only
  `Name` and `GVK` needed for the readiness checker).
- One module handler `"aigateway"` with `testModuleAGVK`.
- `provisionReg := provision.NewRegistry()`:
  - `provisionReg.Add("dashboard", provision.KindComponent, dag.RL(10))`
  - `provisionReg.Add("aigateway", provision.KindModule, dag.RL(20))`
  - `provisionReg.Enable("dashboard")`
  - `provisionReg.Enable("aigateway")`
- Call `startAllControllers`, `createGatewayConfig`, `createDSCI`.
- Register module CRD: `registerModuleCRD(t, et, testModuleAGVK)`.

**Action**:
- `createDSC(t, tc, ...)` with AIGateway=Managed.

**Assertions**:

1. PlatformModule `"aigateway"` created. Platform `ProvisioningProgress=False`
   with message containing `"dashboard"` (blocked at RL10).

2. Create Dashboard CR (unstructured), mark `Ready=True`:
   ```go
   dashboard := &unstructured.Unstructured{}
   dashboard.SetGroupVersionKind(gvk.Dashboard)
   dashboard.SetName("default-dashboard")
   NewWithT(t).Expect(tc.Client().Create(ctx, dashboard)).Should(Succeed())
   t.Cleanup(func() { _ = tc.Client().Delete(context.Background(), dashboard) })
   setUnstructuredReady(t, tc.Client(), dashboard, true)
   ```

3. RL10 cleared. `ProvisioningProgress=True`:
   ```go
   wt.Get(gvk.Platform, nn).Eventually().Should(
       jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "True"`),
   )
   ```

4. aigateway PlatformModule becomes ready (no manifests, module CR absent ->
   OperandAbsent+Info -> Ready=True). Platform `ModulesReady=True`.

## Verification

```bash
go test -v -count=1 -run 'TestDSCDriven_DAG_Advancement' ./tests/integration/platform/...
```
