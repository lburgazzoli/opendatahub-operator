---
task: "04-1 - Add metric assertions to DAG advancement tests"
file: tests/integration/platform/platform_only_test.go, tests/integration/platform/dsc_driven_test.go
depends_on: task-02-2-dag-advancement.md, task-03-4-dag-advancement.md
started:
completed:
---

# Task 04-1: Add Metric Assertions to DAG Advancement Tests

Enhance `TestPlatformOnly_DAG_Advancement` and `TestDSCDriven_DAG_Advancement`
to assert on Prometheus metrics emitted by `WalkBatches`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

**Prerequisites**:
- DAG metrics implemented in `pkg/controller/provision/gating_metrics.go`
- Existing tests from tasks 02-2 and 03-4 pass

## Imports to Add

Both test files need:

```go
"strconv"

"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
"github.com/prometheus/client_golang/prometheus/testutil"
```

## Helper

Add a helper to `suite_test.go` (or inline) that reads the info-style
`odh_dag_runlevel_status` metric for a given runlevel and status label:

```go
func runlevelStatusValue(runlevel int, status string) float64 {
    return testutil.ToFloat64(
        provision.RunlevelStatus.WithLabelValues(strconv.Itoa(runlevel), status),
    )
}
```

## Changes to `TestPlatformOnly_DAG_Advancement`

This test has two modules: monitoring at RL(10), aigateway at RL(20).

**Reset metrics at test start** (before `createPlatform`):

```go
provision.RunlevelStatus.Reset()
provision.RunlevelDurationSeconds.Reset()
provision.RunlevelCleared.Set(0)
provision.RunlevelBlocked.Set(0)
```

**After Step 1** (both PlatformModule CRs exist, neither Ready):

Assert the DAG is blocked at RL20 because RL10 (monitoring) is not ready:

```go
// RL10 was processed (first batch, never gated), RL20 is blocked.
g := NewWithT(t)
g.Eventually(func() float64 {
    return runlevelStatusValue(10, "processed")
}).Should(Equal(float64(1)))
g.Eventually(func() float64 {
    return runlevelStatusValue(20, "blocked")
}).Should(Equal(float64(1)))
g.Expect(testutil.ToFloat64(provision.RunlevelBlocked)).Should(Equal(float64(20)))
```

**After Step 3** (both modules Ready, DAG fully advanced):

```go
g.Eventually(func() float64 {
    return runlevelStatusValue(20, "processed")
}).Should(Equal(float64(1)))
g.Expect(testutil.ToFloat64(provision.RunlevelCleared)).Should(Equal(float64(20)))
g.Expect(testutil.ToFloat64(provision.RunlevelBlocked)).Should(Equal(float64(0)))
g.Expect(testutil.ToFloat64(provision.BatchesProcessedTotal)).Should(BeNumerically(">=", 2))
```

## Changes to `TestDSCDriven_DAG_Advancement`

This test has dashboard (component) at RL(10), aigateway (module) at RL(20).

**Reset metrics at test start** (before `createDSCI`):

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
```

**After Step 5** (all ready, DAG fully advanced):

```go
g.Eventually(func() float64 {
    return runlevelStatusValue(20, "processed")
}).Should(Equal(float64(1)))
g.Expect(testutil.ToFloat64(provision.RunlevelCleared)).Should(Equal(float64(20)))
g.Expect(testutil.ToFloat64(provision.RunlevelBlocked)).Should(Equal(float64(0)))
g.Expect(testutil.ToFloat64(provision.BatchesProcessedTotal)).Should(BeNumerically(">=", 2))
```

## Verification

```bash
go test -v -count=1 -run 'TestPlatformOnly_DAG_Advancement|TestDSCDriven_DAG_Advancement' ./tests/integration/platform/...
```
