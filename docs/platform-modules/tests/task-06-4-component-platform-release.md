---
task: "06-4 - Tracked components publish platform release entry"
file: api/components/v1alpha1/
depends_on: task-06-2-platformmodule-tracker-only.md
started:
completed:
---

# Task 06-4: Tracked components publish platform release entry

Standardize component-backed tracked CRs so they publish a `status.releases[]` row
with `name = "platform"`.

## Goal

Give tracker-only `PlatformModule` the same version extraction contract for
component-backed entries that module-backed entries already use.

## Test Goals

Validate that component-backed tracked CRs:

- publish `status.releases[name="platform"]`
- keep that release row updated consistently
- allow `PlatformModule` to report the platform version for tracker-only entries

## Suggested Coverage

Add unit and/or integration checks that:

- patch a component-backed CR status with a `platform` release row
- verify tracker-only `PlatformModule` reads that version
- verify absence/presence of the row affects version reporting deterministically

This should cover all Platform-tracked component-backed CR types that embed
`common.ComponentReleaseStatus`, not just one representative component.

## Verification

```bash
go test -failfast -count=1 ./internal/controller/platformmodule/... ./tests/integration/platform/...
```
