---
task: "02-2 - TestPlatformOnly_DAG_Advancement"
file: tests/integration/platform/platform_only_test.go
depends_on: task-02-1-two-modules-created.md
started:
completed:
---

# Task 02-2: TestPlatformOnly_DAG_Advancement

Add `TestPlatformOnly_DAG_Advancement` to
`tests/integration/platform/platform_only_test.go`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**: `suite_test.go` provides `startAllControllers`, `suiteOpts`,
`createPlatform`, `createGatewayConfig`, `registerModuleCRD`,
`newTestModuleHandler`, `testModuleAGVK`, `testModuleBGVK`.

## Test: `TestPlatformOnly_DAG_Advancement`

Verifies that the Platform controller's `walkModuleDAG` clears runlevels in
order and the PlatformModule controller's `RunlevelGateAction` unblocks deploy
accordingly.

**Setup**:
- Two test module handlers: `"monitoring"` (with `testModuleAGVK`) and
  `"aigateway"` (with `testModuleBGVK`). Handler names **must** match the
  `module:` struct tags in `PlatformModules` because `EnabledModules()` returns
  tag values and PlatformModule CRs are named accordingly.
- `moduleReg := modules.NewRegistry()`; add both handlers.
- `provisionReg := provision.NewRegistry()`:
  - `provisionReg.Add("monitoring", provision.KindModule, dag.RL(10))`
  - `provisionReg.Add("aigateway", provision.KindModule, dag.RL(20))`
  - `provisionReg.Enable("monitoring")`
  - `provisionReg.Enable("aigateway")`
- `componentReg := &cr.Registry{}` -- empty.
- Call `startAllControllers(t, suiteOpts{...})`.
- Register both module CRDs dynamically:
  - `registerModuleCRD(t, et, testModuleAGVK)`
  - `registerModuleCRD(t, et, testModuleBGVK)`
- Call `createGatewayConfig(t, tc)`.

**Action**:
- `createPlatform(t, tc, ...)` with `Monitoring=Managed`, `AIGateway=Managed`.

**Assertions** (step by step):

1. Both PlatformModule CRs created:
   ```go
   wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).Eventually().Should(Succeed())
   wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).Eventually().Should(Succeed())
   ```

2. The PlatformModule controller sees the handlers, finds no manifests (the
   handlers return no `OperatorManifests`), and sets conditions accordingly.
   The key is the DAG ordering: monitoring (RL10) should become ready before
   aigateway (RL20) is unblocked.

3. Since the module handlers have no manifests and the module CRDs are registered
   with `WithPermissiveSchema()`, the PlatformModule reconciler will:
   - Not deploy anything (no manifests, no tracked resources).
   - Set `DeploymentsAvailable=True` (default path when `pm.Status.Resources`
     is empty and no deployments exist).
   - Check the module operand CR -- initially absent, so `OperandAvailable=False`
     with `OperandAbsent` reason and `Info` severity.
   - Info-severity conditions don't block `Ready`, so `Ready=True`.

4. With `Ready=True` on the monitoring PlatformModule, the Platform controller's
   `walkModuleDAG` should clear RL10. Then the aigateway PlatformModule should
   also become `Ready=True`.

5. Final assertion: Platform `ModulesReady=True` and `Ready=True`:
   ```go
   wt.Get(gvk.Platform, nn).Eventually().Should(And(
       jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
       jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
   ))
   ```

## Verification

```bash
go test -v -count=1 -run 'TestPlatformOnly_DAG_Advancement' ./tests/integration/platform/...
```
