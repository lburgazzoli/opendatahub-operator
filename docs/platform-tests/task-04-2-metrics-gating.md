---
task: "04-2 - Add metric assertions to DAG gating tests"
file: tests/integration/platform/platform_only_test.go, tests/integration/platform/dsc_driven_test.go
depends_on: task-02-3-dag-gating.md, task-03-5-dag-gating.md
started:
completed:
---

# Task 04-2: Add Metric Assertions to DAG Gating Tests

Enhance `TestPlatformOnly_DAG_Gating_ComponentBlocksModule` and
`TestDSCDriven_DAG_Gating_ModuleBlocksModule` to assert on Prometheus metrics
emitted by `WalkBatches`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**:
- DAG metrics implemented in `pkg/controller/provision/gating_metrics.go`
- `runlevelStatusValue` helper from task 04-1 available in `suite_test.go`
- Existing tests from tasks 02-3 and 03-5 pass

## Changes to `TestPlatformOnly_DAG_Gating_ComponentBlocksModule`

This test has dashboard (component) at RL(10), monitoring (module) at RL(20).
Dashboard CR is initially absent, blocking RL20.

**Reset metrics at test start** (before `createPlatform`):

```go
provision.RunlevelStatus.Reset()
provision.RunlevelDurationSeconds.Reset()
provision.RunlevelCleared.Set(0)
provision.RunlevelBlocked.Set(0)
```

**After Step 1** (DAG blocked — no Dashboard CR):

```go
g := NewWithT(t)
g.Eventually(func() float64 {
    return runlevelStatusValue(10, "processed")
}).Should(Equal(float64(1)))
g.Eventually(func() float64 {
    return runlevelStatusValue(20, "blocked")
}).Should(Equal(float64(1)))
g.Expect(testutil.ToFloat64(provision.RunlevelBlocked)).Should(Equal(float64(20)))
g.Expect(testutil.ToFloat64(provision.RunlevelDurationSeconds.WithLabelValues("20"))).
    Should(BeNumerically(">", 0))
```

**After Step 3** (Dashboard Ready, RL10 clears, monitoring unblocked):

```go
g.Eventually(func() float64 {
    return runlevelStatusValue(20, "processed")
}).Should(Equal(float64(1)))
g.Expect(testutil.ToFloat64(provision.RunlevelCleared)).Should(Equal(float64(20)))
g.Expect(testutil.ToFloat64(provision.RunlevelBlocked)).Should(Equal(float64(0)))
```

## Changes to `TestDSCDriven_DAG_Gating_ModuleBlocksModule`

This test has monitoring (module) at RL(10), aigateway (module) at RL(20).
Both have no manifests and absent operand CRs, so they become Ready quickly
(OperandAbsent + Info severity does not block Ready).

Because both modules go Ready quickly in this test, the DAG advances without
blocking. The metrics should reflect that all runlevels are processed:

**Reset metrics at test start** (before `createPlatform`):

```go
provision.RunlevelStatus.Reset()
provision.RunlevelDurationSeconds.Reset()
provision.RunlevelCleared.Set(0)
provision.RunlevelBlocked.Set(0)
```

**After Phase 4** (all ready, DAG fully advanced):

```go
g := NewWithT(t)
g.Eventually(func() float64 {
    return runlevelStatusValue(10, "processed")
}).Should(Equal(float64(1)))
g.Eventually(func() float64 {
    return runlevelStatusValue(20, "processed")
}).Should(Equal(float64(1)))
g.Expect(testutil.ToFloat64(provision.RunlevelCleared)).Should(Equal(float64(20)))
g.Expect(testutil.ToFloat64(provision.RunlevelBlocked)).Should(Equal(float64(0)))
g.Expect(testutil.ToFloat64(provision.BatchesProcessedTotal)).Should(BeNumerically(">=", 2))
```

## Verification

```bash
go test -v -count=1 -run 'TestPlatformOnly_DAG_Gating|TestDSCDriven_DAG_Gating' ./tests/integration/platform/...
```
