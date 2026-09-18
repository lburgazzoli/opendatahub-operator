# RHOAIENG-94812: DataScienceCluster v3 operator implementation plan

This is the repository-local implementation guide for DataScienceCluster
(DSC) v3. The authoritative decisions are in `decisions.md`. This guide
contains the relevant conclusions from the DSC v3 epic,
implementation task, spike, follow-up tasks, comments, and wiki review as of
**2026-09-18**. An implementation agent must be able to use this plan and
[TASKS.md](TASKS.md) without access to Jira, Google Docs, or the wiki. External
links at the end are provenance only.

[decisions.md](decisions.md) is the authoritative decision record, and
[development.md](development.md) defines mandatory implementation rules. If a
summary in this plan conflicts with an accepted decision, the decision wins.

The plan intentionally does not invent public API decisions that remain open.
Such decisions are hard gates in the decision record below. Work that does not
depend on an open gate may proceed independently.

## Agent operating protocol

1. Read `AGENTS.md`, `CONTRIBUTING.md`, `docs/DESIGN.md`,
   `docs/COMPONENT_INTEGRATION.md`, [development.md](development.md),
   [decisions.md](decisions.md), this file, and [TASKS.md](TASKS.md).
2. Select the first unblocked item in the TASKS dependency order. Do not use a
   provisional example as an approved API contract.
3. Use the repository map and task entry as the starting point. Search for all
   remaining versioned imports and generated consumers before editing.
4. If work reaches a `Proposed` decision, stop only that workstream. Continue
   any independent task; never choose a public JSON name, conversion
   precedence, default, or release-rollout strategy on behalf of API owners.
5. When an external decision is supplied, first record it as `Accepted` in
   `decisions.md`, then update the corresponding summary/gate in this plan and
   TASKS before implementing it.
6. Update the numbered task file's front matter and Outcomes, then mirror its
   status in TASKS. Jira workflow status is informational and need not be
   updated by an agent.
7. Keep changes scoped. Preserve unrelated working-tree changes. When an
   action chain is changed, garbage collection must remain the final action.
8. Treat conversion and conversion tests as part of every v3 data-model
   change. A v3 field/status/condition change is incomplete without explicit
   v2 <-> v3 behavior and tests in the same change.
9. After every code change, run the mandatory generation, formatting, and lint
   gates listed under Verification and include generated diffs.

## Outcome and boundaries

The completed operator must:

- serve `datasciencecluster.opendatahub.io/v3` as the sole storage version and
  use typed v3 DSC objects throughout production reconciliation;
- continue serving v2 through an explicit v2-to-v3 conversion webhook and
  preserve v2 admission compatibility while marking v2 deprecated;
- remove v1 from the new operator and generated artifacts in Task 001; keep
  those artifacts ineligible for upgrade promotion until separate Task 010
  qualifies the odh-cli gate that migrates v1 storage to v2;
- preserve the effective behavior of existing v2 objects and preserve all v3
  state through a v2 read-modify-write;
- implement the accepted Dashboard, AI Hub, Feature Store, and Data Registry
  public shapes and project them into the existing module APIs;
- regenerate both ODH and RHOAI CRDs, bundles, webhooks, samples, API docs, and
  Go-generated artifacts; and
- prove fresh-install and supported-upgrade behavior with automated tests.

The following are not part of RHOAI 3.6 DSC v3:

- renaming the internal AI Hub/Model Registry CR, GVK, or module metadata;
- renaming the internal Data/Feast CR, GVK, or module metadata;
- Data Connection Hub integration;
- removal of v2 serving or v2 admissions; and
- implementation of the odh-cli gate, product documentation, or release notes
  outside this repository. The gate contract and operator-side integration and
  qualification tests remain in scope.

## Decision summary

This is a convenience summary. [decisions.md](decisions.md) is authoritative.
`Accepted` means implementation may rely on the decision. `Proposed` is a hard
gate. Issue keys identify provenance and ownership; they are not required
reading.

| ID | State | Normative decision | Unblocks |
| --- | --- | --- | --- |
| `DEC-001` | Accepted | v3 is the hub/storage/internal type and v2 remains served. | API and internal migration |
| `DEC-002` | Accepted | Initial v3 is wire-equivalent to v2 and conversion is semantic identity. | Machinery milestone |
| `DEC-003`, `DEC-007` | Accepted | Every future v3 data change includes bidirectional conversion, tests, and complete OpenAPI difference coverage. | All future v3 changes |
| `DEC-004` | Accepted | V2/v3 use distinct GVK constants and admissions. | Admissions |
| `DEC-005`, `DEC-006` | Accepted | Register conversion through v2 and use ConversionReview protocol `[v1]`. | Conversion webhook |
| `DEC-008` | Accepted | Final v3 code/generated API removes v1. | V1-free end state |
| `DEC-009` | Accepted (`94814`) | Run the idempotent odh-cli migration gate before OLM applies the v1-free CRD, then migrate storage from v2 to v3 after upgrade. | Task 010 and upgrade release |
| `DEC-010` | Accepted | Empty `managementState` has effective value `Removed`. | Lifecycle behavior |
| `DEC-011`, `DEC-012` | Accepted | Keep internal module identities and exclude DCH in 3.6. | Component handlers |
| `DEC-013`, `DEC-014`, `DEC-016` | Accepted | Task 001 removes v1 without odh-cli work; the separate gate blocks upgrade release, and evolving v3 public types remain isolated. | Safe implementation sequence |
| `DEC-015`, `DEC-019` | Accepted | Keep v2 served but deprecated and use the exact warning literal recorded in DEC-019. | V2 migration path |
| `DEC-016` | Accepted | Task 001 excludes odh-cli work and may complete; future Task 010 blocks upgrade release until qualification passes. | Independent machinery development |
| `DEC-017` | Accepted | Even with no DSC object, remove and verify v1 in `status.storedVersions`; only v1 already absent is mutation-free success. | Correct Task 010 gate behavior |
| `DEC-018`, `DEC-020` | Proposed | Name the enforceable promotion control and complete supported execution matrix. | Task 010 start/completion and upgrade release |
| `DEC-021` | Accepted | Do not refresh the independent v2 baseline for v3-only changes. | Reliable schema guard |
| G1 | Proposed (`94805`) | Choose the Dashboard v3 spec/status shape and final names for the core Dashboard and portal. | Dashboard schema, conversion, handler |
| G2 | Proposed (`95340`, `94809`) | Confirm the public JSON name `aiHub` versus the older `hub` proposal and approve its status and condition names. | AI Hub schema, conversion, handler |
| G3 | Proposed (`95339`, `94809`) | Confirm that `data` has no parent `managementState`, versus the older parent-gating proposal. | Data schema, conversion, handler |
| G4 | Proposed (`95340`, `94809`) | Define `aiHub.registriesNamespace` optionality, defaults, immutability, update rules, and absent-v2-field behavior. | AI Hub admissions and conversion |
| G5 | Proposed (`94809`) | Define conversion precedence when deprecated `kserve.modelsAsService` and canonical `aigateway.modelsAsAService` conflict. | MaaS conversion |
| G6 | Proposed (`94809`) | Define v3 read/write behavior for non-empty v2 `trainingoperator` and `llamastackoperator`, which have no proposed v3 fields. | Conversion and schema |
| G7 | Proposed (`94805`, `95339`, `95340`, `94809`) | Approve every changed spec, status, and condition mapping, including absent and empty values. | Generated schema and component completion |

To close a gate, add an accepted decision with exact normative behavior to
`decisions.md`, then replace `Proposed` here and update TASKS. A statement that
the overall v3 direction is approved does not close a field-level gate.

## Embedded contract proposals

These proposals capture all known input. They are implementation guidance only
after corresponding accepted decisions are recorded in `decisions.md`.

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
| Dashboard | `dashboard.managementState`; `dashboard.maasConsumerPortal.managementState`; status fields `dashboard` and `maasConsumerPortal` | Final grouping and names pending G1/G7 | Preserve both existing effective states in both round trips; map status and `MaaSConsumerPortalAvailable` only after names are accepted. |
| AI Hub | `modelregistry.managementState`; `modelregistry.registriesNamespace`; status `modelregistry`; condition `ModelRegistryReady` | `aiHub.managementState`; `aiHub.registriesNamespace`; final status/condition pending G2/G4/G7 | One-to-one spec mapping; retain namespace and effective state; map status/condition explicitly. Internal AIHub CR stays unchanged. |
| Feature Store | `feastoperator.managementState` | `data.featureStore.managementState` | One-to-one lifecycle mapping. |
| Data Registry | Proposed v2 compatibility field `feastoperator.dataRegistry.managementState` | `data.dataRegistry.managementState` | One-to-one lifecycle mapping if the compatibility field is accepted. Use a conversion annotation only if the final contract leaves state unrepresentable in v2. |
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

- In the machinery milestone, v2 and v3 have identical Go/JSON schemas. The
  v2 <-> v3 converter is therefore a semantic no-op: it copies the complete
  object without renames, drops, defaults, or normalization.
- Transforming conversion begins only after G1-G7 have accepted decisions and the v3 types
  are changed by the component-contract milestone.
- Conversion is deterministic, idempotent, and performs no cluster lookups.
- `ObjectMeta` and every unchanged spec and status field are copied in both
  directions. A field may be deliberately dropped only when the accepted matrix
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

### Required rule for every future v3 data-model change

Every change to a v3 DSC spec/status type, nested component type, JSON name,
optionality, default, validation, or condition is also a conversion change,
even when the correct converter code remains an identity copy. The same pull
request or commit must:

1. identify the old v2 representation and the new v3 representation;
2. define `v2 -> v3` behavior for populated, absent, empty, and invalid legacy
   inputs;
3. define `v3 -> v2` behavior, including how a v2 read-modify-write preserves
   any value v2 cannot represent directly;
4. update both conversion directions, or document why the existing explicit
   copy is still correct;
5. add focused forward, backward, and both round-trip tests for the changed
   data, including status/condition mappings when applicable;
6. update defaulting, validation, CRD schema, samples, and API docs when the
   wire contract changes; and
7. update a structural difference/coverage guard so a new v2/v3 field
   difference cannot be added silently without an explicit mapping and test.

A v3 data-model change must not merge with only handler or schema tests. The
conversion tests are part of its definition of done.

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
must have no production hits after the machinery milestone.

## Executable implementation milestones

### Milestone 1: establish v3 machinery without contract changes

This is the first implementation step and is ready now. It deliberately uses
the current v2 schema unchanged so it is independent of G1-G7.
The executable task and living outcome record is
[tasks/001.md](tasks/001.md).

Task 001 is a roll-up. Execute its five records independently where their
dependencies allow: [v3 API/baseline](tasks/001-01.md),
[atomic conversion/v1 removal](tasks/001-02.md),
[typed-v3 runtime migration](tasks/001-03.md),
[v2 deprecation/generated delivery](tasks/001-04.md), and
[milestone integration](tasks/001-05.md). Tests land with the subtask that
changes behavior; the integration subtask does not defer them.

#### 1. Introduce v3

- Add `api/datasciencecluster/v3` by reproducing the current v2 DSC spec,
  status, components, markers, validation annotations, group registration, and
  list types exactly. Do not introduce Dashboard, AI Hub, Data, MaaS, or legacy
  field changes in this milestone.
- Generate v3 deepcopy code and register v3 in `PROJECT`, `cmd/main.go`, envtest
  schemes, and all version-aware helpers.
- Leave v2 as the temporary hub/storage version while 001-01 adds v3. In
  001-02, move `+kubebuilder:storageversion` and `conversion.Hub` to v3
  atomically with v2 spoke conversion activation and v1 deletion. Never leave a
  compiling boundary where the existing v1 converter requires v2 as hub after
  that marker has been removed.
- Point component-codegen and new DSC fixtures/samples at v3.
- Keep top-level v3 API types version-owned. Do not change shared component
  types to introduce v3 behavior; create v3-owned nested types when a stanza
  later diverges and protect v2 with an independent OpenAPI baseline.

#### 2. Add identity v2 <-> v3 conversion

- Implement `ConvertTo` and `ConvertFrom` on v2 with v3 as the hub.
- Because the schemas are identical in this milestone, copy the complete
  object semantically unchanged. Do not apply defaults, normalize management
  states, rename fields, drop legacy fields, or invoke any cluster lookup.
- Add whole-object tests showing `v2 -> v3 -> v2` and `v3 -> v2 -> v3`
  semantic equality for populated spec, status, metadata, conditions, releases,
  related objects, annotations, empty values, and deprecated fields. Ignore
  only the expected target-version `TypeMeta`/GVK representation.
- Register controller-runtime conversion through the v2 spoke, introduce
  distinct v2/v3 GVK constants, and keep each admission handler bound to its
  own GVK.
- Generate `conversionReviewVersions: [v1]`; this is the Kubernetes protocol
  version, not a list of DSC versions.
- Exercise real `/convert` `ConversionReview` requests in both directions with
  one and multiple objects.

#### 3. Move production code from v2 to v3

- Change the DSC reconciler, component registry, module `DSCContext`, handler
  interfaces, base-handler reflection/status writers, all component/module
  handlers, predicates, initial-install helpers, comparison utilities,
  generators, and production utilities to typed v3.
- Update tests and fixtures alongside each package. After this step, v2 imports
  are allowed only in v2 conversion, v2 admissions, and explicit compatibility
  tests.
- Re-check every touched action chain and keep garbage collection last.

#### 4. Remove v1 and record the separate release gate

- Remove v1 in Task 001 without implementing or qualifying odh-cli. Task 010 is
  the separate future release-gate task.
- Remove the v1 API, conversion, admissions, scheme and `PROJECT` registration,
  generated serving, fixtures, and obsolete version-specific tests. Preserve
  behavioral coverage by moving relevant cases to v2/v3 tests.
- Move shared `/convert` registration to the v2 spoke before deleting its v1
  registration path. No v1 <-> v3 converter is required in the new operator.
- Delete the AI Gateway conversion annotation only after confirming that no
  v2/v3 or other consumer requires it.
- Mark the v1-free artifacts ineligible for upgrade promotion until Task 010
  proves DEC-009's odh-cli migration, `status.storedVersions`, OLM ordering,
  retry, and rollback contract.

#### 5. Amend tests and generated artifacts

- Add identity conversion unit and webhook/envtest round-trip coverage.
- Add a structural coverage test that compares complete normalized v2/v3
  generated OpenAPI schemas, including path, type, requiredness, nullability,
  default, enum, CEL, and list/map semantics. Its expected-difference set is
  empty in Milestone 1; later differences must link to focused conversion cases.
- Port controller, handler, admission, initial-install, comparison, and E2E
  helpers to v3 while retaining explicit v2 client compatibility tests.
- Assert that v3 is the only storage version, v2/v3 are served, v1 is absent,
  `/convert` is configured through v2, `conversionReviewVersions` is `[v1]`,
  and v2/v3 admissions remain version-correct. Assert v2 has
  `deprecated: true` and DEC-019's exact `deprecationWarning` literal.
- Regenerate every ODH and RHOAI artifact and run all mandatory gates.

Milestone 1 exit criteria:

- V2 and v3 expose the same schema and identity conversion is lossless in both
  directions.
- Production reconciliation exclusively uses typed v3 objects.
- V1 code and serving are absent. Upgrade release remains blocked until Task
  010 proves v1 storage is removed before the v1-free CRD is installed.
- Generated CRDs serve deprecated v2 and v3 with v3 as the sole storage version.
- G1-G7 remain proposed and no proposed component stanza is encoded.

### Milestone 2: freeze and encode component contracts

Entry: Milestone 1 is complete and component-owner decisions are available for
G1-G7.

Execution records: [Dashboard contract](tasks/002.md),
[Data contract](tasks/003.md), [AI Hub contract](tasks/004.md),
[consolidated matrix](tasks/005.md), and
[API/conversion implementation](tasks/006.md).

- Update the local decision record before code.
- Replace provisional schemas with exact Go/JSON shapes and complete the
  field-by-field spec/status/condition/conversion matrix.
- Implement the Dashboard, AI Hub, Data, MaaS, and legacy-field changes in v3.
- Change the formerly identity converter only for accepted schema differences;
  unchanged fields must retain the Milestone 1 copy behavior.
- Apply DEC-003 and DEC-007 to every changed spec/status/condition field: conversion code,
  structural-difference inventory, and focused bidirectional/round-trip tests
  land together.
- Update v2 compatibility types only where the accepted contract explicitly
  requires a representable compatibility field.

Exit: G1-G7 have accepted decisions, every changed field has deterministic bidirectional
conversion, and generated v2/v3 schemas match the approved contract.

### Milestone 3: integrate component projections and admissions

Execution records: [Dashboard](tasks/007.md), [AI Hub](tasks/008.md), and
[Data](tasks/009.md).

- Dashboard: project the accepted core/portal shape into the existing Dashboard
  module CR and mirror the approved status/conditions.
- AI Hub: project the public AI Hub shape into the existing `default-aihub`
  internal resource without performing the deferred rename.
- Data: project Feature Store and Data Registry independently into the existing
  Data/Feast module while retaining OIDC behavior and excluding DCH.
- Adapt v3 defaulting/validation to changed fields. Keep v2 admission behavior
  compatible with v3 storage.
- Add handler, lifecycle, readiness, status, admission, conversion, and schema
  tests for all accepted state combinations.

Exit: each public stanza produces the approved internal resource behavior and
DSC status while v2 clients remain compatible.

### Milestone 4: final release and upgrade qualification

Execution records: [odh-cli gate qualification](tasks/010.md) and
[final integration/release qualification](tasks/011.md).

- Qualify the odh-cli pre-upgrade gate from every supported source release and
  prove OLM never applies the v1-free CRD while v1 remains in
  `status.storedVersions`.
- Add upgrade coverage from the gate's v2 storage state to RHOAI 3.6 v3 storage
  for ODH and RHOAI, including v2/v3 read-update cycles and all changed
  component behaviors.
- Verify final served/storage versions and `status.storedVersions`.

Exit: fresh install, supported upgrade, interrupted retry, and rollback tests
pass with reviewed generated artifacts.

## Verification and acceptance

Minimum automated scenarios:

- Milestone 1 whole-object identity conversion in both directions while v2 and
  v3 schemas are identical;
- a v2/v3 structural-difference guard whose explicit inventory has focused
  conversion coverage for every listed path;
- table-driven spec/status/condition conversion for every changed field;
- `v2 -> v3 -> v2` preservation of every v2-representable value;
- `v3 -> v2 -> v3` preservation of every v3 value;
- `Managed`, `Removed`, empty, absent, and conflicting inputs;
- v2 and v3 admission defaulting, validation, warnings, and update rules;
- Dashboard, AI Hub, Feature Store, and Data Registry handler projections,
  lifecycle, readiness, releases, legacy status, and removal;
- generated Milestone 1 schema: deprecated v2 and v3 served, exactly v3
  storage, v1 absent, `/convert` registered through v2, and ConversionReview
  protocol `[v1]`;
- odh-cli pre-upgrade gating before the v1-free CRD, followed by verified v3
  storage migration and safe `status.storedVersions` cleanup;
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
