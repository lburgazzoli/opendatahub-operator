# RHOAIENG-94812: DataScienceCluster v3 operator implementation plan

This is the repository-local implementation authority for DataScienceCluster
(DSC) v3. It contains the relevant conclusions from the DSC v3 epic,
implementation task, spike, follow-up tasks, comments, and wiki review as of
**2026-09-18**. An implementation agent must be able to use this plan and
[TASKS.md](TASKS.md) without access to Jira, Google Docs, or the wiki. External
links at the end are provenance only.

The plan intentionally does not invent public API decisions that remain open.
Such decisions are hard gates in the decision record below. Work that does not
depend on an open gate may proceed independently.

## Agent operating protocol

1. Read `AGENTS.md`, `CONTRIBUTING.md`, `docs/DESIGN.md`,
   `docs/COMPONENT_INTEGRATION.md`, this file, and [TASKS.md](TASKS.md).
2. Select the first unblocked item in the TASKS dependency order. Do not use a
   provisional example as an approved API contract.
3. Use the repository map and task entry as the starting point. Search for all
   remaining versioned imports and generated consumers before editing.
4. If work reaches a `Pending` decision, stop only that workstream. Continue
   any independent task; never choose a public JSON name, conversion
   precedence, default, or retirement sequence on behalf of API owners.
5. When an external decision is supplied, first replace the corresponding
   `Pending` row in this document with the exact approved schema and behavior,
   including approval date and evidence. Then update TASKS and implement it.
6. Update repository-local task status and completion evidence in TASKS. Jira
   workflow status is informational and need not be updated by an agent.
7. Keep changes scoped. Preserve unrelated working-tree changes. When an
   action chain is changed, garbage collection must remain the final action.
8. After every code change, run the mandatory generation, formatting, and lint
   gates listed under Verification and include generated diffs.

## Outcome and boundaries

The completed operator must:

- serve `datasciencecluster.opendatahub.io/v3` as the sole storage version and
  use typed v3 DSC objects throughout production reconciliation;
- continue serving v2 through an explicit v2-to-v3 conversion webhook and
  preserve v2 admission compatibility;
- retire v1 only after stored-object migration makes removal valid;
- preserve the effective behavior of existing v2 objects and preserve all v3
  state through a v2 read-modify-write;
- implement the frozen Dashboard, AI Hub, Feature Store, and Data Registry
  public shapes and project them into the existing module APIs;
- regenerate both ODH and RHOAI CRDs, bundles, webhooks, samples, API docs, and
  Go-generated artifacts; and
- prove fresh-install and supported-upgrade behavior with automated tests.

The following are not part of RHOAI 3.6 DSC v3:

- renaming the internal AI Hub/Model Registry CR, GVK, or module metadata;
- renaming the internal Data/Feast CR, GVK, or module metadata;
- Data Connection Hub integration;
- removal of v2 serving or v2 admissions; and
- `odh-cli`, product-documentation, or release-note implementation outside
  this repository.

## Local decision record

`Frozen` means implementation may rely on the decision. `Pending` is a hard
gate. Issue keys identify provenance and ownership; they are not required
reading.

| ID | State | Normative decision | Unblocks |
| --- | --- | --- | --- |
| D1 | Frozen | v3 is the controller-runtime hub, CRD storage version, and production-internal DSC type. | API and internal migration |
| D2 | Frozen | v2 stays served, becomes the only conversion spoke after v1 retirement, and retains version-compatible defaulting and validation. | Conversion and admissions |
| D3 | Frozen | Empty `managementState` has the existing effective meaning `Removed`. | All handlers and conversions |
| D4 | Frozen | Fields not explicitly changed by the final matrix retain v2 JSON names and semantics and are copied losslessly, including metadata, release data, conditions, and related objects. | Conversion |
| D5 | Frozen | Public DSC naming may change, but internal AI Hub/Model Registry and Data/Feast module CR names and metadata remain unchanged in 3.6. | Component handlers |
| D6 | Frozen | DCH is excluded. Feature Store and Data Registry are the only proposed children of `data`. | Data handler |
| G1 | Pending (`94805`) | Choose the Dashboard v3 spec/status shape and final names for the core Dashboard and portal. | Dashboard schema, conversion, handler |
| G2 | Pending (`95340`, `94809`) | Confirm the public JSON name `aiHub` versus the older `hub` proposal and approve its status and condition names. | AI Hub schema, conversion, handler |
| G3 | Pending (`95339`, `94809`) | Confirm that `data` has no parent `managementState`, versus the older parent-gating proposal. | Data schema, conversion, handler |
| G4 | Pending (`95340`, `94809`) | Define `aiHub.registriesNamespace` optionality, defaults, immutability, update rules, and absent-v2-field behavior. | AI Hub admissions and conversion |
| G5 | Pending (`94809`) | Define conversion precedence when deprecated `kserve.modelsAsService` and canonical `aigateway.modelsAsAService` conflict. | MaaS conversion |
| G6 | Pending (`94809`) | Define v3 read/write behavior for non-empty v2 `trainingoperator` and `llamastackoperator`, which have no proposed v3 fields. | Conversion and schema |
| G7 | Pending (`94805`, `95339`, `95340`, `94809`) | Approve every changed spec, status, and condition mapping, including absent and empty values. | Generated schema and component completion |
| G8 | Pending (`94814`) | Select a release/OLM-safe v1 storage migration and retirement staging strategy, including rollback boundaries. | Removal of v1 |

To close a gate, replace `Pending` with `Frozen (YYYY-MM-DD)`, replace the
proposal below with exact normative behavior, and record the approving source
or owner in TASKS. A statement that the overall v3 direction is approved does
not close a field-level gate.

## Embedded contract proposals

These proposals capture all known input. They are implementation guidance only
after the corresponding gates above are frozen.

### Unchanged surface

The following v2 spec entries currently exist and are expected to retain their
JSON names and semantics in v3 unless the final matrix explicitly says
otherwise: `workbenches`, `aipipelines`, `kserve`, `kueue`, `ray`, `trustyai`,
`ogx`, `mlflowoperator`, `trainer`, `sparkoperator`, `aigateway`, and
`mcplifecycleoperator`. The common DSC status fields, release information,
conditions, related objects, and error message must also survive conversion.

### Proposed changed surface

| Concern | Current v2 shape | Latest proposed v3 shape | Required mapping |
| --- | --- | --- | --- |
| Dashboard | `dashboard.managementState`; `dashboard.maasConsumerPortal.managementState`; status fields `dashboard` and `maasConsumerPortal` | Final grouping and names pending G1/G7 | Preserve both existing effective states in both round trips; map status and `MaaSConsumerPortalAvailable` only after names are frozen. |
| AI Hub | `modelregistry.managementState`; `modelregistry.registriesNamespace`; status `modelregistry`; condition `ModelRegistryReady` | `aiHub.managementState`; `aiHub.registriesNamespace`; final status/condition pending G2/G4/G7 | One-to-one spec mapping; retain namespace and effective state; map status/condition explicitly. Internal AIHub CR stays unchanged. |
| Feature Store | `feastoperator.managementState` | `data.featureStore.managementState` | One-to-one lifecycle mapping. |
| Data Registry | Proposed v2 compatibility field `feastoperator.dataRegistry.managementState` | `data.dataRegistry.managementState` | One-to-one lifecycle mapping if the compatibility field is frozen. Use a conversion annotation only if the final contract leaves state unrepresentable in v2. |
| MaaS | Canonical `aigateway.modelsAsAService`; deprecated `kserve.modelsAsService` also exists | Only `aigateway.modelsAsAService` | Canonical value is the current runtime preference when explicitly set, but conversion conflict behavior remains gated by G5. |
| Training Operator | Deprecated `trainingoperator` | Field omitted | Behavior for a non-empty legacy value remains gated by G6. |
| Llama Stack Operator | Deprecated `llamastackoperator` | Field omitted; `ogx` remains | Behavior for a non-empty legacy value remains gated by G6. |

Latest Data proposal:

```yaml
# v2 compatibility shape
components:
  feastoperator:
    managementState: Managed
    dataRegistry:
      managementState: Managed
```

```yaml
# v3 proposal; data is a structural group with no parent state
components:
  data:
    featureStore:
      managementState: Managed
    dataRegistry:
      managementState: Managed
```

Latest AI Hub proposal:

```yaml
# v2
components:
  modelregistry:
    managementState: Managed
    registriesNamespace: model-registry
```

```yaml
# v3 proposal
components:
  aiHub:
    managementState: Managed
    registriesNamespace: model-registry
```

Dashboard has two unresolved structural alternatives:

- retain `dashboard.managementState` and place the portal under `dashboard`;
  or
- make core Dashboard and portal independently managed children beneath a
  structural `dashboard` group.

The suggested names `openshiftAI` and `maasCustomerPortal` are not approved.
Feedback noted that `openshiftAI` is unsuitable for non-OpenShift Kubernetes
and that “customer” is too narrow for a portal with administration scenarios.
`core` and `modelsAsAServicePortal` were suggestions, not decisions.

### Conversion invariants

- Conversion is deterministic, idempotent, and performs no cluster lookups.
- `ObjectMeta` and every unchanged spec and status field are copied in both
  directions. A field may be deliberately dropped only when the frozen matrix
  states how compatibility and effective behavior are preserved.
- `v2 -> v3 -> v2` preserves every value representable by v2.
- `v3 -> v2 -> v3` preserves every v3 value, including independent child
  states, during a v2 read-modify-write.
- Prefer an explicit compatibility field over annotation stashing. Use an
  internal conversion annotation only for a final, demonstrably
  unrepresentable value, and remove stale stash data when it is consumed.
- Defaulting and validation stay outside conversion.
- Empty management state is normalized to `Removed` only when code needs an
  effective state; do not rewrite absent API fields merely for normalization.

## Current repository baseline

This map records the implementation state an agent would otherwise have to
derive before starting.

| Area | Current behavior and primary touchpoints |
| --- | --- |
| Public DSC API | `api/datasciencecluster/v2/datasciencecluster_types.go` is the storage API and implements `conversion.Hub`. `api/datasciencecluster/v1` is a spoke. Shared component types are in `api/components/v1alpha1`; do not change a shared type if that would unintentionally alter v1/v2 or an internal module schema. |
| Registration/codegen | `PROJECT` registers DSC v1 and v2 with defaulting/validation. `cmd/main.go` registers both schemes. `cmd/component-codegen/cmd/generator/generator.go` hard-codes the v2 DSC types path. |
| Runtime DSC type | `internal/controller/datasciencecluster`, `internal/controller/components/registry/registry.go`, `internal/controller/modules/types.go`, `internal/controller/modules/base.go`, status helpers, predicates, initial-install helpers, and tests use typed v2 objects. |
| Conversion compatibility | v1 conversion contains the existing KServe-to-AI-Gateway MaaS migration and uses `conversion.opendatahub.io/aigateway-state` from `pkg/metadata/annotations/annotations.go` to preserve otherwise unrepresentable state. `pkg/dsc/compare/v2only.go` only compares v1/v2 differences. |
| Admissions | `internal/webhook/webhook.go` registers `dsc-v1` and `dsc-v2`. Each version has defaulting, validation, and envtest coverage under `internal/webhook/datasciencecluster`. |
| Generated CRD | ODH and RHOAI generated DSC CRDs currently serve v1 and v2, with v2 storage. Conversion patches under `config/crd/patches` and `config/rhoai/crd/patches` use `/convert` and list review versions v1/v2. |
| Samples | Primary samples are `config/samples/datasciencecluster_v2_datasciencecluster.yaml` and the RHOAI equivalent. |
| Dashboard | `api/components/v1alpha1/dashboard_types.go` embeds a top-level management state and `maasConsumerPortal`, defaulted to `Removed`. `internal/controller/modules/dashboard/handler.go` deploys the operator when either core or portal is managed, projects the current Dashboard spec plus namespaces/gateway data, and mirrors condition `MaaSConsumerPortalAvailable` to status field `MaaSConsumerPortal`. |
| AI Hub | Public DSC types and module registration still use Model Registry naming. `internal/controller/modules/modelregistry/handler.go` manages internal AIHub GVK `AIHub`, CR name `default-aihub`, module/manifest name `modelregistry`, and condition `ModelRegistryReady`. It maps `registriesNamespace` to internal `instancesNamespace`, falling back to the applications namespace, and mirrors it into legacy DSC status. |
| Data | `api/components/v1alpha1/feastoperator_types.go` currently contains only Feast lifecycle state. `internal/controller/modules/feastoperator/handler.go` enables the existing Feast module and builds a FeastOperator CR whose spec currently contains only derived external-OIDC configuration; it does not project child lifecycle state. |
| MaaS | AI Gateway runtime logic uses `aigateway.modelsAsAService` when set and otherwise falls back to deprecated `kserve.modelsAsService`. `internal/controller/modules/kserve/handler.go` removes `modelsAsService` before projecting the KServe module CR. |
| Defaults/validation | v2 defaults Model Registry `registriesNamespace` when Model Registry is managed: `odh-model-registries` for ODH and `rhoai-model-registries` for RHOAI. It defaults KServe NIM to managed when KServe is managed. Validation enforces DSC singleton creation, denies Kueue managed, warns for deprecated KServe MaaS, prevents re-enabling that deprecated field after removal, and keeps Model Registry namespace immutable while managed. |
| Tests | Unit/envtest coverage exists beside APIs, handlers, and webhooks. `tests/e2e/v2tov3upgrade_test.go` is legacy-named and does not yet prove DSC API v2-to-v3 storage conversion; do not count it as the new upgrade acceptance test without rewriting it. |

Before considering the internal migration complete, use targeted searches such
as:

```bash
rg -n 'datasciencecluster/v2|dscv2' api cmd internal pkg tests -g '*.go'
rg -n 'datasciencecluster/v1|dscv1' api cmd internal pkg tests -g '*.go'
rg -n 'modelregistry|ModelRegistry|feastoperator|FeastOperator|MaaSConsumerPortal|modelsAsService' \
  api internal pkg config tests -g '*.go' -g '*.yaml'
```

The first search should end with a small, documented allowlist containing only
v2 conversion/admission compatibility and version-specific tests. The second
must have no production hits after the approved v1 retirement phase.

## Executable implementation batches

### Batch 0: freeze and encode the public contract

Entry: component-owner decisions are available for G1-G7.

- Update the local decision record first.
- Replace all provisional schemas with exact Go/JSON shapes.
- Add a field-by-field matrix covering spec, status, condition names, absent
  values, empty values, defaults, validation, update rules, and both conversion
  directions.
- State the resolution for Dashboard names, `aiHub`/`hub`, Data parent state,
  registries namespace, MaaS conflicts, and removed legacy fields.

Exit: no changed public field or compatibility behavior is marked pending, and
the component handler tasks have an unambiguous contract.

### Batch 1: add the v3 API and v2 spoke

Entry: Batch 0 is complete.

- Add `api/datasciencecluster/v3` with group/version registration, DSC/list,
  spec/status/components, hub marker, and generated deepcopy code.
- Start from v2 and apply only frozen differences. Use version-specific types
  where sharing would leak a v3 schema change into v2 or module APIs.
- Move the storage marker and `Hub()` implementation to v3. Implement
  `ConvertTo`/`ConvertFrom` on v2 against v3.
- Keep v2 public JSON stable except for a compatibility field explicitly
  frozen by Batch 0.
- Update `PROJECT`, scheme registration, and component-codegen's DSC target.

Exit: conversion unit tests cover every matrix row and both full round trips;
generated CRD assertions show one storage version (v3) while all versions
required for the retirement stage remain served.

### Batch 2: migrate production reconciliation to typed v3

Entry: v3 types compile and conversion tests pass.

- Change the DSC reconciler, registry interfaces, `DSCContext`, module handler
  status interfaces/reflection, status aggregation, predicates,
  initial-install helpers, comparison utilities, and all component handlers to
  v3.
- Update unit fixtures and helpers as each subsystem moves; retain explicit v2
  objects only in compatibility tests and conversion/admission code.
- Re-check every controller action chain and keep garbage collection last.

Exit: production v2 imports match the documented compatibility allowlist; the
operator builds and unit tests pass for both ODH and RHOAI build behavior.

### Batch 3: integrate component-owned projections

Entry: relevant G1-G7 rows are frozen and Batch 2 provides typed v3 context.

- Dashboard: project the frozen core/portal shape into the existing Dashboard
  module CR and mirror each approved status and condition while preserving v2
  behavior.
- AI Hub: project public AI Hub state/namespace into the existing `default-aihub`
  resource and existing internal naming. Preserve build-specific namespace
  defaults and legacy status only as required by the frozen matrix.
- Data: project Feature Store and Data Registry independently into the existing
  Data/Feast module API. Preserve Feast OIDC projection. Add no DCH fields and
  perform no internal rename.
- Update vendored/module CRD schemas, schema-compliance tests, handler tests,
  lifecycle/status tests, and generated artifacts for each component.

Exit: all state combinations produce the frozen module CR, operator lifecycle,
DSC status, and conditions, including independent child removal where defined.

### Batch 4: add v3 admissions and preserve v2 behavior

Entry: v3 schema is frozen.

- Register v3 defaulting/validation handlers, markers, routes, and envtest
  scheme support.
- Port common v2 rules and adapt only rules whose fields changed.
- Keep v2 admissions and warnings compatible with conversion to v3 storage.
- Remove v1 registration and tests only in the retirement batch selected by
  G8. Do not use conversion failures as validation.

Exit: create/update/default/validation tests pass through v2 and v3, including
empty fields, immutable fields, deprecated MaaS warnings, and platform-specific
defaults.

### Batch 5: retire v1 safely

Entry: G8 is frozen with a tested delivery sequence.

Kubernetes requires an old version to be absent from `status.storedVersions`
before it is removed from CRD `spec.versions`; changing `storage: true` alone
does not rewrite stored objects. The selected implementation must therefore:

1. serve v3 as storage while conversion still supports every possibly stored
   version;
2. rewrite the singleton DSC through v3, or use a migration mechanism supported
   by every target OpenShift version;
3. idempotently verify and remove v1 from `status.storedVersions`, with retries
   safe after interruption;
4. only then stop serving v1 and remove its CRD schema, scheme, webhooks,
   conversion, and production tests; and
5. enforce the approved rollback boundary.

If OLM applies the CRD before the new operator can migrate storage, these steps
cannot be delivered as a single manifest transition. Implement the staged
strategy recorded by G8 rather than collapsing it.

Exit: fresh install, upgrade, retry-after-interruption, final storedVersions,
and supported rollback tests pass.

### Batch 6: generated deliverables and end-to-end proof

- Regenerate ODH and RHOAI CRD bases, conversion patches, webhook manifests,
  RBAC if changed, bundle/CSV content, API references, deepcopy/object code,
  and samples.
- Make v3 the primary ODH and RHOAI sample. Retain focused v2 fixtures only for
  compatibility/upgrade tests.
- Add upgrade coverage from RHOAI 3.5 v2 storage to 3.6 v3 storage for both
  platform variants, including Dashboard, Model Registry/AI Hub, Feast/Data,
  legacy MaaS, v2/v3 read-update cycles, status/conditions, and independent
  removals.
- Verify the final served/storage version set and `status.storedVersions`.

Exit: all acceptance tests and quality gates below pass with reviewed generated
diffs.

## Verification and acceptance

Minimum automated scenarios:

- table-driven spec/status/condition conversion for every changed field;
- `v2 -> v3 -> v2` preservation of every v2-representable value;
- `v3 -> v2 -> v3` preservation of every v3 value;
- `Managed`, `Removed`, empty, absent, and conflicting inputs;
- v2 and v3 admission defaulting, validation, warnings, and update rules;
- Dashboard, AI Hub, Feature Store, and Data Registry handler projections,
  lifecycle, readiness, releases, legacy status, and removal;
- generated schema: exactly one storage version, correct served versions,
  `/convert` configured, and no premature v1 removal;
- ODH and RHOAI fresh install and supported upgrade; and
- idempotent storage migration, interrupted retry, and approved rollback.

Mandatory repository gates after code changes:

```bash
make generate manifests api-docs
make fmt
make lint
make unit-test
make build
git diff --check
```

Run the relevant E2E targets when a cluster is available. A task may be marked
complete only when TASKS records the commands run, their result, the important
generated diff, and any environment-limited test that remains for CI.

## Provenance (optional reading)

- [RHOAIENG-94804: DSC v3 epic](https://redhat.atlassian.net/browse/RHOAIENG-94804)
- [RHOAIENG-94812: operator implementation](https://redhat.atlassian.net/browse/RHOAIENG-94812)
- [RHOAIENG-85262: original spike](https://redhat.atlassian.net/browse/RHOAIENG-85262)
- [Spike findings document](https://docs.google.com/document/d/1IvAHo3xRd4fHBmzU0Vnpgiq7M1K2w0OWdt7ZRhUIuk4/edit)
- [AI Core Platform wiki summary](https://redhat.atlassian.net/wiki/spaces/~712020ae5659d45615471ca83c37c75652ea3e/pages/460554960/AI+Core+Platform+Features+and+Epics+In-Progress+Summary)
- [Kubernetes CRD versioning](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/)
