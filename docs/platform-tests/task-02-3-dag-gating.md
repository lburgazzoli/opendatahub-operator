---
task: "02-3 - TestPlatformOnly_DAG_Gating_ComponentBlocksModule"
file: tests/integration/platform/platform_only_test.go
depends_on: task-02-1-two-modules-created.md
started:
completed:
---

# Task 02-3: TestPlatformOnly_DAG_Gating_ComponentBlocksModule

Add `TestPlatformOnly_DAG_Gating_ComponentBlocksModule` to
`tests/integration/platform/platform_only_test.go`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**: `suite_test.go` provides `startAllControllers`, `suiteOpts`,
`createPlatform`, `createGatewayConfig`, `registerModuleCRD`,
`newTestModuleHandler`, `setPlatformModuleReady`, `setUnstructuredReady`,
`testModuleAGVK`.

## Test: `TestPlatformOnly_DAG_Gating_ComponentBlocksModule`

A test component at RL10 blocks a test module at RL20. Exercises the composite
readiness checker that spans both component and module readiness.

**Setup**:
- One test module handler: `"monitoring"` with `testModuleAGVK`.
- `moduleReg := modules.NewRegistry()`; add handler.
- One test component handler using `cr.BaseComponentHandler`:
  ```go
  componentReg := &cr.Registry{}
  componentReg.Add(&cr.BaseComponentHandler{
      Name: "dashboard",
      GVK:  gvk.Dashboard,
  })
  ```
  No `IsEnabledFn` or `NewCRObjectFn` needed -- the Platform controller only
  uses `GroupVersionKind()` from component handlers (for the readiness checker).
- `provisionReg := provision.NewRegistry()`:
  - `provisionReg.Add("dashboard", provision.KindComponent, dag.RL(10))`
  - `provisionReg.Add("monitoring", provision.KindModule, dag.RL(20))`
  - `provisionReg.Enable("dashboard")`
  - `provisionReg.Enable("monitoring")`
- Call `startAllControllers(t, suiteOpts{...})`.
- Register module CRD: `registerModuleCRD(t, et, testModuleAGVK)`.
- Call `createGatewayConfig(t, tc)`.

**Action**:
- `createPlatform(t, tc, ...)` with `Monitoring=Managed`.

**Assertions** (step by step):

1. PlatformModule CR `"monitoring"` created.

2. No Dashboard CR exists. The component readiness checker returns `false` for
   `"dashboard"`. The DAG is blocked at RL10. Assert Platform has
   `ProvisioningProgress=False` with message containing `"dashboard"`:
   ```go
   wt.Get(gvk.Platform, nn).Eventually().Should(And(
       jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "False"`),
       jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .message | contains("dashboard")`),
   ))
   ```

3. Create a Dashboard CR (unstructured), mark it NOT ready:
   ```go
   dashboard := &unstructured.Unstructured{}
   dashboard.SetGroupVersionKind(gvk.Dashboard)
   dashboard.SetName("default-dashboard")
   NewWithT(t).Expect(cli.Create(ctx, dashboard)).Should(Succeed())
   t.Cleanup(func() { _ = cli.Delete(context.Background(), dashboard) })
   ```
   Still blocked -- `ProvisioningProgress=False`.

4. Mark Dashboard `Ready=True` via `setUnstructuredReady(t, cli, dashboard, true)`.
   RL10 clears. Assert `ProvisioningProgress=True`:
   ```go
   wt.Get(gvk.Platform, nn).Eventually().Should(
       jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "True"`),
   )
   ```

5. Mark monitoring PlatformModule `Ready=True` via `setPlatformModuleReady`.
   Assert `ModulesReady=True` and `Ready=True`.

## Verification

```bash
go test -v -count=1 -run 'TestPlatformOnly_DAG_Gating_ComponentBlocksModule' ./tests/integration/platform/...
```
