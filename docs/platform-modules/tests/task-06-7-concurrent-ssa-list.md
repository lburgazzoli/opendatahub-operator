---
task: "06-7 - Concurrent DSC and DSCI SSA writes on list-based Platform spec"
file: tests/integration/platform/platform_combined_test.go
depends_on: task-06-1-platform-list-status.md
started:
completed:
---

# Task 06-7: Concurrent DSC and DSCI SSA writes on list-based Platform spec

Add coverage for concurrent or overlapping SSA writes from DSC and DSCI against the
new list-based `Platform.spec.modules` shape.

## Goal

Validate that `+listType=map` / `+listMapKey=name` semantics allow DSC and DSCI to own
their own entries without clobbering each other.

## Test Goals

Validate that:

- DSC and DSCI can both write to `Platform.spec.modules`
- each controller preserves the other controller's entry
- combined reconciliation results in the union of list entries keyed by `name`
- managed fields / SSA ownership remain stable across repeated reconciles

## Suggested Coverage

Add a combined integration test that:

- starts both DSC and DSCI controllers
- drives one entry from DSC and one entry from DSCI
- triggers repeated reconciles from both sides
- asserts the final `Platform.spec.modules` list contains both entries
- asserts updates from one side do not remove or overwrite the other side's row

## Verification

```bash
go test -failfast -count=1 ./tests/integration/platform/...
```
