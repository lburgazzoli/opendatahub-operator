---
task: "06-3 - Reflection-based projection of name and managementState"
file: internal/controller/datasciencecluster/datasciencecluster_controller_actions.go
depends_on: task-06-1-platform-list-status.md
started:
completed:
---

# Task 06-3: Reflection-based projection of name and managementState

Update DSC and DSCI projection tests for the new rule that higher-level controllers
project only `{name, managementState}` into `Platform.spec.modules`.

## Test Goals

Validate that:

- DSC and DSCI use `module:"..."` only as canonical name mapping
- projection remains reflection-driven
- only `name` and `managementState` are written into `Platform.spec.modules`
- higher-level controllers do not populate per-entry `config`

## Suggested Coverage

### DSC projection

Add or update tests around
`internal/controller/datasciencecluster/datasciencecluster_controller_test.go` and
`tests/integration/platform/platform_dsc_test.go` to confirm:

- managed fields become list entries keyed by module name
- `managementState` is preserved
- `config` remains empty/omitted

### DSCI projection

Add or update tests around
`internal/controller/dscinitialization/dscinitialization_module_test.go` and
`tests/integration/platform/platform_dsci_test.go` to confirm the same behavior for
DSCI-driven service entries.

## Verification

```bash
go test -failfast -count=1 ./internal/controller/datasciencecluster/... ./internal/controller/dscinitialization/... ./tests/integration/platform/...
```
