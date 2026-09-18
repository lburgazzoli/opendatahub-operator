# DSC v3 development rules

These rules apply to every agent implementing a task under `docs/dsc-v3/tasks`.
Read this file and [decisions.md](decisions.md) before editing code. Accepted
decisions are authoritative and must be honored.

## Task lifecycle

Front-matter dependency semantics:

- `depends_on` and `blocked_by` participate in task completion scheduling.
- `subtasks` is a roll-up edge: a parent cannot complete until every listed
  child completes, but children do not depend on the parent merely by naming it.
- `promotion_requires` is a release-only edge. It never blocks implementation
  completion and must be excluded from dependency-cycle resolution.
- Use `status: not_started` while waiting only for another numbered task. Use
  `status: blocked` only when `blocked_by` names an unavailable external input
  or proposed decision.

Before starting:

1. Read the task, PLAN, this file, and all accepted decisions referenced by the
   task.
2. Update the task front matter: set `status: in_progress`, `start_date` to the
   current ISO-8601 date/time, `assignee`, and `last_updated`.
3. Record the starting branch/commit and any pre-existing worktree changes.
4. If a required decision is not accepted, set `status: blocked`, describe the
   blocker in the task, and continue only independent work.

Before completing:

1. Add every durable implementation decision to `decisions.md` before relying
   on it in the final code.
2. Run the required tests and quality gates and record exact results.
3. Fill the task's Outcomes section with changed behavior, generated artifacts,
   decisions used/created, test evidence, deviations, limitations, and follow-up
   work.
4. Set `status: completed`, `end_date`, and `last_updated` only when the full
   definition of done is satisfied. A partially implemented or externally
   blocked task remains `in_progress` or `blocked` and has no `end_date`.

## Reuse before invention

- Search existing code in the same package and neighboring components before
  adding a helper, abstraction, conversion mechanism, predicate, action, status
  writer, or test utility.
- Prefer the established controller-runtime/Kubebuilder APIs, repository
  helpers under `pkg`, and
  `github.com/opendatahub-io/odh-platform-utilities/framework` over local
  reimplementations.
- Prefer repository façade packages that already wrap framework utilities,
  such as `pkg/controller`, `pkg/resources`, and `pkg/cluster`, so behavior and
  dependency boundaries remain consistent.
- Do not copy framework or `odh-platform-utilities` implementations into this
  repository. If an existing facility is close but incomplete, extend the
  narrowest appropriate abstraction or record why reuse is unsafe.
- Do not add a new dependency when the standard library, controller-runtime,
  the framework, platform utilities, or an existing repository dependency
  already provides the behavior.

Useful discovery commands:

```bash
rg -n '<concept or symbol>' internal pkg api cmd -g '*.go'
rg -n 'odh-platform-utilities|framework/' internal pkg cmd api -g '*.go'
```

## Keep implementation simple

- Choose explicit, readable code over reflection, metaprogramming, implicit
  mutation, or a generic abstraction that hides conversion behavior.
- Avoid `unsafe` and JSON serialization for typed v2/v3 conversion.
  DEC-024 uses a literal marker and requires no JSON payload or snapshot.
- Keep functions focused, error paths visible, and data flow easy to follow.
- Add an abstraction only when it removes real duplication without obscuring
  Kubernetes lifecycle or conversion semantics.
- Preserve behavior outside the task. Do not combine opportunistic refactors
  with an API-version migration.
- Keep garbage collection last in every controller action chain.

## Idiomatic and modern Go

- `go.mod` is the version source of truth; it currently specifies Go `1.26.5`.
  Code must compile with that version and must not require a newer language or
  standard-library feature.
- Before editing a Go file, use the available `use-modern-go` skill/Modern Go
  Guidelines CLI with `list --file-path <file>`, read the complete result, and
  apply all relevant Go 1.26 guidance.
- Use standard Go error wrapping (`fmt.Errorf("context: %w", err)`), clear
  ownership, narrow interfaces, and existing repository conventions.
- Use typed APIs rather than unstructured data unless the module framework
  requires `unstructured.Unstructured` at an external CR boundary.
- Do not introduce clever generic helpers merely because Go supports them.
  Prefer straightforward typed conversion code whose mappings are reviewable.
- Record any intentional deviation from Modern Go guidance in the task outcome,
  including why following it would not compile or would change behavior.

## Makefile-first workflow

Use repository Make targets rather than invoking Go tools directly when a
target exists. The Makefile pins tools and carries ODH/RHOAI build settings.

Mandatory gates after code changes:

```bash
make generate manifests api-docs
make fmt
make lint
make unit-test
make build
git diff --check
```

Run focused tests during development, then the complete relevant Make targets
before completion. Include generated diffs. Test both `odh` and `rhoai` behavior
when build-tag-specific types, defaults, schemas, or generated artifacts are
affected.

## Required test layers

Every change needs the lowest-cost test that proves its behavior. Intermediate
implementation tasks require complete unit and integration coverage; the
consolidated real-cluster E2E implementation and execution are deliberately
deferred to final qualification in DSC-V3-011 under DEC-023. DSC-V3-010 remains
the separate exception for its odh-cli/upgrade qualification boundary.

### Unit tests

- Cover each conversion mapping directly in both directions.
- Cover both round trips, absent/empty values, status, conditions, metadata,
  deprecated fields, error paths, and idempotence.
- Use table-driven tests and the repository's Gomega conventions.
- Test handlers, lifecycle decisions, status writers, predicates, and helpers
  without requiring a cluster where possible.

### Integration tests

- Use envtest for CRD schema, defaulting, validation, conversion webhook,
  controller reconciliation, and status-subresource behavior.
- Exercise the real `/convert` endpoint with Kubernetes `ConversionReview`
  requests in both desired-version directions and with multiple objects.
- Verify generated OpenAPI and served/storage-version assertions, not only Go
  struct shape.
- Verify retained v2 admissions independently from v3 admissions.

### End-to-end tests

- DSC-V3-011 owns the consolidated E2E code, fixtures, execution, and result
  recording for changes delivered by Tasks 001 and 006-009/012. Intermediate
  tasks must record the behaviors and fixtures that final qualification needs,
  but they do not need to add or run E2E tests before completion when their
  integration coverage is complete.
  Task 012's only E2E code changes may be compile fixture fixes; new E2E
  scenarios belong to Task 011.
- Use a real supported cluster for installation, OLM/upgrade ordering, storage
  migration, `status.storedVersions`, retry, and rollback behavior.
- For v1 retirement, honor DEC-009: the external odh-cli gate runs before OLM
  applies the v1-free CRD. Test the gate contract and packaging boundary even
  though the odh-cli implementation is outside this repository. This work is
  owned by future DSC-V3-010, not by Task 001.
- Exercise both v2 and v3 clients against v3 storage and verify that v3-only
  data survives a v2 read-modify-write.
- Cover ODH and RHOAI variants and the affected component lifecycle/readiness
  behavior.

During DSC-V3-011, if a cluster-dependent test cannot run locally, specify the
exact CI/E2E test, required environment, and expected assertion in the task
outcome. Lack of a local cluster may defer execution to CI, but final
qualification must still contain the E2E implementation.

## API and conversion discipline

- Follow `DEC-003` for every v3 data change. Conversion and conversion tests
  are part of the change, not follow-up work.
- Keep v2 compatibility explicit. Never rely on a shared mutable type to make
  v2 and v3 change together.
- Treat the independent v2 OpenAPI baseline as immutable for v3-only changes.
  Change it only under DEC-021 for an explicitly accepted v2 wire-contract
  change with separately listed paths and focused tests.
- Maintain the complete OpenAPI difference inventory required by `DEC-007`.
- Back the inventory with one machine-readable test registry. Every entry has a
  stable `case_id`, v2 path, v3 path, changed OpenAPI semantics, and executable
  forward, backward, v2-round-trip, and v3-round-trip cases. The guard must fail
  if an inventory entry lacks any case ID or if the shared runner did not
  execute all four categories; a prose test name or allowlist entry is
  insufficient.
- Conversion must be deterministic and must not perform defaulting, validation,
  logging-dependent recovery, or Kubernetes API calls.
- Use admission webhooks and CRD schema for validation/defaulting.
- For MaaS, apply DEC-022 forward precedence with DEC-024's literal
  `legacy-managed` marker. V2 ignores incoming markers and sets one iff current
  legacy L is Managed, regardless of effective precedence. Reverse keeps C/A
  migrated and returns legacy Managed iff marked, else Removed including empty.
- V3 UPDATE compares old/new canonical, KServe parent, and AI Gateway parent
  management states: any change permanently deletes the marker, even after a
  later revert; otherwise `OldObject` is authoritative. Native CREATE strips
  supplied reserved metadata. Reject unknown interpreted marker values.
- Reuse existing `componentApi` AI Gateway/canonical value types without
  canonical pointers or duplicated v2/v3 wrappers. Do not restore original
  C/A/absence. Keep OpenAPI and the independent v2 baseline unchanged. Test
  normalized fixtures and schema defaulting separately, with admission through
  envtest. No supported installed intermediate-v3 release needs legacy fallback.

## Kubernetes error and portability rules

- Handle OpenShift-only APIs using three branches: found,
  `IsNotFound`/`IsNoMatchError`, and other errors.
- Wrap errors with operation and resource context while preserving the cause.
- Use `go-multierror` where established code collects independent failures.
- Treat empty DSC management state as `Removed`.
