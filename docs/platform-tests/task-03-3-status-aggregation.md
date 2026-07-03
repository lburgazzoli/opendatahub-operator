---
task: "03-3 - TestDSCDriven_StatusAggregation"
file: tests/integration/platform/dsc_driven_test.go
depends_on: task-03-1-platform-reflects-dsc.md
started:
completed:
---

# Task 03-3: TestDSCDriven_StatusAggregation

Add `TestDSCDriven_StatusAggregation` to
`tests/integration/platform/dsc_driven_test.go`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**: `suite_test.go` provides `startAllControllers`, `suiteOpts`,
`createDSCI`, `createDSC`, `createGatewayConfig`, `registerModuleCRD`,
`newTestModuleHandler`, `setUnstructuredReady`, `testModuleAGVK`.

## Test: `TestDSCDriven_StatusAggregation`

Full status reporting chain across DSC and Platform controllers.

**Setup**:
- One component handler `"dashboard"` with `gvk.Dashboard`:
  - `IsEnabledFn` returns `true`.
  - `UpdateDSCStatusFn` returns `metav1.ConditionTrue, nil`.
- One module handler `"aigateway"` with `testModuleAGVK`.
- `moduleReg`, `componentReg`, `provisionReg` as in task-03-2.
- Call `startAllControllers`, `createGatewayConfig`, `createDSCI`.
- Register module CRD: `registerModuleCRD(t, et, testModuleAGVK)`.

**Action**:
- `createDSC(t, tc, ...)` with AIGateway=Managed.

**Assertions**:

Phase 1 -- component ready, module not ready:
```go
dscKey := types.NamespacedName{Name: "default-dsc"}

// ComponentsReady=True (dashboard returns ConditionTrue).
wt.Get(gvk.DataScienceCluster, dscKey).Eventually().Should(
    jq.Match(`.status.conditions[] | select(.type == "ComponentsReady") | .status == "True"`),
)

// ModulesReady=False (module CR has no Ready condition yet).
wt.Get(gvk.DataScienceCluster, dscKey).Eventually().Should(
    jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "False"`),
)
```

Phase 2 -- patch module operand CR with Ready=True:
```go
moduleCR := &unstructured.Unstructured{}
moduleCR.SetGroupVersionKind(testModuleAGVK)
NewWithT(t).Eventually(func() error {
    return tc.Client().Get(context.Background(), types.NamespacedName{Name: "default-aigateway"}, moduleCR)
}).Should(Succeed())

setUnstructuredReady(t, tc.Client(), moduleCR, true)
```

Then trigger DSC re-reconcile (annotation bump -- same pattern as
`internal/controller/datasciencecluster/datasciencecluster_controller_test.go:277-283`):
```go
latestDSC := &dscv2.DataScienceCluster{}
NewWithT(t).Expect(tc.Client().Get(context.Background(), dscKey, latestDSC)).Should(Succeed())
if latestDSC.Annotations == nil {
    latestDSC.Annotations = map[string]string{}
}
latestDSC.Annotations["test/trigger"] = "ready"
NewWithT(t).Expect(tc.Client().Update(context.Background(), latestDSC)).Should(Succeed())
```

Assert `ModulesReady=True`:
```go
wt.Get(gvk.DataScienceCluster, dscKey).Eventually().Should(
    jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
)
```

## Verification

```bash
go test -v -count=1 -run 'TestDSCDriven_StatusAggregation' ./tests/integration/platform/...
```
