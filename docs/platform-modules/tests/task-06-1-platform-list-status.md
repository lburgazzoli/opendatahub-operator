---
task: "06-1 - List-based Platform spec/status summary"
file: api/config/v1alpha1/platform_types.go
depends_on:
started:
completed:
---

# Task 06-1: List-based Platform spec/status summary

Refactor the Platform API so `spec.modules` and `status.modules` both use a list-based
shape keyed by `name`.

## Scope

Update the Platform API and its supporting helpers so:

- `Platform.spec.modules` is a `+listType=map` / `+listMapKey=name` list
- each spec row contains:
  - `name`
  - `managementState` with default `Removed`
  - optional `config,omitempty`
- `Platform.status.modules` mirrors the list shape
- each status row contains:
  - `name`
  - `runlevel`
  - `version`
  - nested `status.ready`
  - nested `status.reason`
  - nested `status.message`

## Expected Tests

Add or update tests to verify:

- list items are keyed by `name`
- an existing item with no `managementState` defaults to `Removed`
- `Platform.status.modules` uses the nested `status` object
- status rows are sorted by runlevel, then name

## Verification

```bash
go test -failfast -count=1 ./internal/controller/platform/... ./tests/integration/platform/...
```
