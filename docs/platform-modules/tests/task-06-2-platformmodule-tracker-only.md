---
task: "06-2 - PlatformModule tracker-only mode"
file: internal/controller/platformmodule/platformmodule_controller_test.go
depends_on: task-06-1-platform-list-status.md
started:
completed:
---

# Task 06-2: PlatformModule tracker-only mode

Add coverage for the new `trackerOnly` behavior in the PlatformModule reconciler.

## Test Goals

Validate that tracker-only entries:

- still reconcile
- still read the underlying CR/object status to compute readiness
- still report version information
- do **not** render/apply operator resources
- do **not** create the platform config ConfigMap
- do **not** populate deployer-only tracked resources
- do **not** run deploy-style `runlevelGateAction`

The preferred implementation keeps one `PlatformModule` reconcile pipeline and uses a
conditional action wrapper/helper to invoke deploy-only actions only for module-backed entries.

## Suggested Coverage

### Unit coverage

Add focused controller tests in
`internal/controller/platformmodule/platformmodule_controller_test.go` to verify:

- tracker-only mode skips deploy side effects
- tracker-only mode computes `Ready` / reason / message from the tracked CR
- tracker-only mode reads `status.releases[name="platform"]` when present
- tracker-only mode reports not ready when the tracked CR is absent
- tracker-only mode handles a missing `status.releases[name="platform"]` the same way the
  module-backed path does today
- conditional action wrapping skips render/deploy/drift-cleanup/config-injection actions for
  tracker-only entries while still running status-reading actions

### Integration coverage

Add an integration scenario in `tests/integration/platform/` where:

- a Platform entry is mapped to an internal-controller-backed tracked CR
- a `PlatformModule` object is created for it
- the tracked CR status drives the `PlatformModule` summary
- no owned resources/configmaps are created by `PlatformModule`

## Verification

```bash
go test -failfast -count=1 ./internal/controller/platformmodule/... ./tests/integration/platform/...
```
