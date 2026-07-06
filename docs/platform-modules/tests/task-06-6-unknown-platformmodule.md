---
task: "06-6 - Unknown or unregistered PlatformModule handling"
file: internal/controller/platform/platform_controller_test.go
depends_on: task-06-1-platform-list-status.md
started:
completed:
---

# Task 06-6: Unknown or unregistered PlatformModule handling

Add coverage for invalid desired-state entries and unknown tracker names.

## Goal

Treat unknown names in `Platform.spec.modules[]` as invalid desired state while avoiding
destructive cleanup of unrelated tracker CRs.

## Test Goals

Validate that:

- if `Platform.spec.modules[]` contains a name that is not present in the internal registry,
  `Platform` becomes Degraded and `Ready=False`
- the degraded state includes a useful reason/message pointing to the bad entry
- if an entry is removed from `Platform.spec.modules[]`, the Platform controller deletes the
  corresponding tracker instance
- existing `PlatformModule` CRs whose names are unknown to the registry are otherwise left alone

## Suggested Coverage

Add Platform controller tests that:

- create a `Platform` with one known and one unknown entry name
- verify `Platform` status becomes degraded because of the unknown name
- create an extra unknown `PlatformModule` CR and verify it is not deleted just because it is
  unknown to the registry
- remove a known entry from `Platform.spec.modules[]` and verify its tracker is deleted

## Verification

```bash
go test -failfast -count=1 ./internal/controller/platform/... ./tests/integration/platform/...
```
