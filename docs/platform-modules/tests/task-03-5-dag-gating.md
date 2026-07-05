---
task: "03-5 - TestDSCDriven_DAG_Gating_ModuleBlocksModule"
file: tests/integration/platform/dsc_driven_test.go
depends_on: task-03-1-platform-reflects-dsc.md
started:
completed:
---

# Task 03-5: TestDSCDriven_DAG_Gating_ModuleBlocksModule

Add `TestDSCDriven_DAG_Gating_ModuleBlocksModule` to
`tests/integration/platform/dsc_driven_test.go`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**: `suite_test.go` provides `startAllControllers`, `suiteOpts`,
`createPlatform`, `createGatewayConfig`, `registerModuleCRD`,
`newTestModuleHandler`, `testModuleAGVK`, `testModuleBGVK`.

## Test: `TestDSCDriven_DAG_Gating_ModuleBlocksModule`

Two modules at different runlevels. Lower module must become ready before higher
module deploys.

**Setup**:
- Module `"monitoring"` with `testModuleAGVK` at RL(10).
- Module `"aigateway"` with `testModuleBGVK` at RL(20).
- Both handlers added to `moduleReg`.
- `provisionReg` with both at their respective runlevels, both enabled.
- Call `startAllControllers`, `createGatewayConfig`.
- Register both module CRDs dynamically.

Note: This test uses `createPlatform` directly instead of `createDSC`. The DSC
controller is registered but won't reconcile anything until a DSC CR is created.
No DSC CR = no interference. No need for `createDSCI` or `createDSC`.

**Action**:
- `createPlatform(t, tc, ...)` with `Monitoring=Managed`, `AIGateway=Managed`.

**Assertions**:

Phase 1 -- both PlatformModule CRs created:
```go
wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
    Eventually().Should(Succeed())
wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
    Eventually().Should(Succeed())
```

Phase 2 -- monitoring's PlatformModule reconciler eventually becomes `Ready=True`
(no manifests -> no deploys -> Info conditions -> Ready=True). The Platform
controller's `walkModuleDAG` should then clear RL10 after seeing monitoring
PlatformModule Ready=True.

Wait for monitoring PlatformModule `Ready=True`:
```go
wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
    Eventually().Should(
        jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
    )
```

Phase 3 -- aigateway PlatformModule should eventually also become `Ready=True`
after RL20 is unblocked:
```go
wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
    Eventually().Should(
        jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
    )
```

Phase 4 -- Platform `ModulesReady=True` and `Ready=True`:
```go
wt.Get(gvk.Platform, nn).Eventually().Should(And(
    jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
    jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
))
```

## Verification

```bash
go test -v -count=1 -run 'TestDSCDriven_DAG_Gating_ModuleBlocksModule' ./tests/integration/platform/...
```
