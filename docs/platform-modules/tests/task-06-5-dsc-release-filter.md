---
task: "06-5 - DSC release aggregation filters internal platform release"
file: internal/controller/datasciencecluster/datasciencecluster_controller_test.go
depends_on: task-06-4-component-platform-release.md
started:
completed:
---

# Task 06-5: DSC release aggregation filters internal platform release

Protect the existing Dashboard-facing DSC release contract by filtering out the
internal `name = "platform"` release row during DSC aggregated release reporting.

## Goal

Components and trackers may use `status.releases[name="platform"]` internally for
upgrade/version tracking, but DSC should not expose that row to downstream consumers
such as Dashboard.

## Test Goals

Validate that:

- component-backed CRs may contain a `platform` release row internally
- DSC aggregated status excludes that row
- existing user-facing release rows continue to flow through unchanged

## Suggested Coverage

Add DSC controller tests that:

- seed component release status with both user-facing releases and a `platform` row
- run aggregation
- assert DSC status contains only the user-facing releases

Add an integration test if needed to confirm the filter works end-to-end through
the DSC reconciliation path.

## Verification

```bash
go test -failfast -count=1 ./internal/controller/datasciencecluster/... ./tests/integration/platform/...
```
