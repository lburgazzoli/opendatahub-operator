# DataScienceCluster v3 operator tasks

This is the repository-local task tracker for implementing DSC v3 under
RHOAIENG-94812. It embeds the relevant task descriptions, spike conclusions,
comments, dependencies, current code behavior, and acceptance criteria as of
**2026-09-18**. Jira, Google Docs, and wiki access is not required. Issue keys
are stable work-package identifiers and optional provenance only.

Read [PLAN.md](PLAN.md) before selecting a task. Its decision record is
normative: `Frozen` decisions may be implemented; `Pending` decisions are hard
gates and must not be guessed.

## How an autonomous agent uses this tracker

1. Select the first `Todo` task whose `Blocked by` column is empty or whose
   listed gates are all `Frozen` in PLAN.
2. Read the task's embedded inputs, current repository state, steps, and
   definition of done. External issue reading is not an entry condition.
3. Change only the task status being worked: `Todo` -> `In progress` ->
   `Complete`. Add the branch/commit and verification evidence to the task.
4. If a required decision is pending, mark the task `Blocked (Gx)` and proceed
   to another independent task. Do not resolve it by copying a provisional
   example into the public API.
5. When an approved decision is provided, update PLAN's local decision record
   and exact contract first; then clear the corresponding blocker here.
6. Preserve unrelated worktree changes and keep garbage collection last in
   every controller action chain.
7. A code task is not complete until required generated changes are included
   and the verification record contains the commands and results.

Legend:

- `✅ Complete`: definition of done and evidence recorded
- `🚧 In progress`: actively being implemented
- `📝 Todo`: ready when its listed gates/dependencies are satisfied
- `⛔ Blocked`: a required contract or delivery is unavailable
- `⚠️ Decision`: owner decision must be encoded locally before implementation
- `🚫 Out of scope`: recorded for clarity; do not implement here

## Status and dependency table

The external issues were all in `New` state at the 2026-09-18 snapshot. Local
status below, not external workflow state, controls agent execution.

| Work package | Local status | Blocked by | Repository output |
| --- | --- | --- | --- |
| `RHOAIENG-94805` Dashboard contract | `⚠️ Decision` | Dashboard/Workbenches/Platform owner choice | Freeze G1 and Dashboard parts of G7 |
| `RHOAIENG-95339` Data contract | `⚠️ Decision` | Data/Feast/Data Registry/Platform owner choice | Freeze G3 and Data parts of G7 |
| `RHOAIENG-95340` AI Hub contract | `⚠️ Decision` | AI Hub/Model Registry/Platform owner choice | Freeze G2/G4 and AI Hub parts of G7 |
| `RHOAIENG-94809` consolidated contract | `⛔ Blocked` | `94805`, `95339`, `95340`, G5/G6 | Freeze G1-G7 and exact conversion matrix |
| `RHOAIENG-94814` v1 retirement design | `⚠️ Decision` | Release/OLM ordering evidence | Freeze G8 and migration design |
| `RHOAIENG-95342` Dashboard handler/CRD | `⛔ Blocked` | G1/G7, typed v3 context | Dashboard projection and status |
| `RHOAIENG-95344` AI Hub handler/CRD | `⛔ Blocked` | G2/G4/G7, typed v3 context | AI Hub projection and status |
| `RHOAIENG-95346` Data handler/CRD | `⛔ Blocked` | G3/G7, typed v3 context | Feature Store/Data Registry projection |
| `RHOAIENG-94812` operator implementation | `⛔ Blocked in part` | G1-G8 and component deliveries | Complete DSC v3 delivery |

```text
94805 Dashboard ─┐
95339 Data ──────┼─> 94809 consolidated contract ─┬─> 95342 Dashboard ─┐
95340 AI Hub ────┘                                 ├─> 95344 AI Hub ───┼─> 94812
                                                   └─> 95346 Data ─────┘
94814 v1 retirement ────────────────────────────────────────────────────┘
```

Mechanical discovery, test inventories, and non-public internal migration
preparation may proceed while gates are pending. Public types, JSON tags,
changed-field conversion, generated schemas, and v1 removal may not cross their
listed gates.

## Completion evidence template

Copy this block into the active task and fill it in before marking complete:

```text
Implementation:
- Branch/commit:
- Frozen decision IDs used:
- Main changed areas:

Verification:
- make generate manifests api-docs:
- make fmt:
- make lint:
- make unit-test:
- make build:
- focused tests:
- git diff --check:

Remaining CI/E2E (must state why not run locally):
-
```

## Contract work packages

### `RHOAIENG-94805`: freeze the Dashboard v3 contract

- Local status: `Decision`
- Decision owners: Dashboard, Workbenches, and Platform API owners
- Blocks: G1, Dashboard parts of G7, `94809`, and `95342`

Purpose:

- Produce one exact public Dashboard v3 spec/status/condition contract while
  preserving the behavior of both current v2 lifecycle controls.

Embedded source record:

- V2 currently exposes `dashboard.managementState` for the core Dashboard and
  `dashboard.maasConsumerPortal.managementState` for the portal.
- One proposal retains the top-level state. The other makes core Dashboard and
  portal independent children of a structural `dashboard` group.
- `openshiftAI` was proposed for the core child, but feedback says that name is
  inappropriate for DSC on non-OpenShift Kubernetes.
- `maasCustomerPortal` was proposed for the portal, but feedback says
  “customer” is too narrow because the application includes administration.
  `core` and `modelsAsAServicePortal` were suggestions only.
- Approval of the overall DSC v3 direction did not approve these field names.
  The component handler must not precede the field-level decision.

Current repository state:

- `api/components/v1alpha1/dashboard_types.go` defines `DSCDashboard` with an
  embedded top-level management spec and `DashboardCommonSpec` containing
  `maasConsumerPortal`; the portal defaults to `Removed`.
- `internal/controller/modules/dashboard/handler.go` enables the Dashboard
  operator when either core Dashboard or portal is managed.
- The handler projects the current Dashboard spec plus component maps,
  notebooks/model-registry namespaces, and gateway domain into the existing
  module CR.
- It mirrors module condition `MaaSConsumerPortalAvailable` to the same DSC
  condition and status field `MaaSConsumerPortal`.

Required decision record:

- Exact v3 YAML and Go field/type names for core and portal.
- Whether `dashboard` has a parent management state or only children.
- Exact v2 <-> v3 mappings for spec and status.
- Exact condition type names and aggregation behavior.
- Empty/absent/default behavior and every valid independent state combination.
- Update/validation rules and the behavior of a v2 read-modify-write.

Definition of done:

- G1 and the Dashboard portion of G7 in PLAN are `Frozen` with exact shapes,
  mappings, approval date, and owner evidence.
- No proposed name remains in the normative contract.
- `95342` can implement every state without making another product decision.

### `RHOAIENG-95339`: freeze the Data, Feature Store, and Data Registry contract

- Local status: `Decision; discrepancy open`
- Decision owners: Data, Feast, Data Registry, and Platform API owners
- Blocks: G3, Data parts of G7, `94809`, and `95346`

Latest proposal:

```yaml
# v2 compatibility
components:
  feastoperator:
    managementState: Managed
    dataRegistry:
      managementState: Managed
```

```yaml
# v3
components:
  data:
    featureStore:
      managementState: Managed
    dataRegistry:
      managementState: Managed
```

Embedded source record:

- The latest proposal makes `data` a structural group with no parent state;
  Feature Store and Data Registry have independent lifecycle controls.
- An older consolidated proposal gave `data` a parent state whose `Removed`
  value overrode both children. This conflicts and must not be implemented.
- The original spike proposed an annotation stash for v3-only Data Registry
  state. The newer proposal makes that state explicitly representable in v2.
- Data Registry had not shipped in the earlier release referenced by comments,
  weakening the need for annotation compatibility. Prefer the explicit field
  if approved; use a stash only for a final unrepresentable value.
- DCH and internal Data/Feast CR and metadata renames are deferred beyond 3.6.

Current repository state:

- V2 `Components` exposes `feastoperator`; its shared type currently contains
  only an embedded management state.
- `internal/controller/modules/feastoperator/handler.go` enables the existing
  Feast module from that state.
- Its internal FeastOperator CR spec currently projects only external-OIDC
  issuer configuration derived from GatewayConfig. It does not project Feature
  Store or Data Registry lifecycle.

Required decision record:

- Confirm or replace the exact v2 compatibility and v3 YAML shapes above.
- Decide definitively whether `data` has a parent management state.
- Define independent/combined `Managed`, `Removed`, empty, and absent behavior.
- Define exact spec, status, condition names, conversion, defaulting,
  validation, update behavior, and v2 read-modify-write fidelity.
- State whether any conversion annotation is still necessary.

Definition of done:

- G3 and the Data portion of G7 are `Frozen`; parent-state conflict is gone.
- The v2 compatibility mechanism is exact and lossless.
- `95346` can implement internal projection and status without adding DCH or
  renaming internal resources.

### `RHOAIENG-95340`: freeze the AI Hub/Model Registry contract

- Local status: `Decision; naming discrepancy open`
- Decision owners: AI Hub, Model Registry, and Platform API owners
- Blocks: G2, G4, AI Hub parts of G7, `94809`, and `95344`

Latest proposal:

```yaml
# v2
components:
  modelregistry:
    managementState: Managed
    registriesNamespace: model-registry
```

```yaml
# v3
components:
  aiHub:
    managementState: Managed
    registriesNamespace: model-registry
```

Embedded source record:

- The latest component proposal requires public name `aiHub` and excludes both
  `hub` and `modelregistry` from v3.
- The older epic/consolidated description uses `hub`; that is a conflict, not
  an alternative an implementation agent may choose.
- Internal module CR, GVK, manifest directory, and metadata naming stay as-is
  for 3.6; their convergence is deferred to 3.7.

Current repository state:

- V2 uses `modelregistry` in spec/status and condition `ModelRegistryReady`.
- When Model Registry is managed with an empty `registriesNamespace`, v2
  defaulting chooses `odh-model-registries` for ODH or
  `rhoai-model-registries` for RHOAI.
- Validation keeps `registriesNamespace` immutable while Model Registry is
  managed.
- `internal/controller/modules/modelregistry/handler.go` manages internal GVK
  `AIHub`, CR name `default-aihub`, module/manifest name `modelregistry`, and
  projects public `registriesNamespace` to internal `instancesNamespace`.
- If the public namespace is empty, the handler falls back to the applications
  namespace. It also mirrors the namespace into legacy DSC status.

Required decision record:

- Confirm `aiHub` or provide the final public JSON name.
- Define v3 spec/status fields and ready condition name.
- Define namespace optionality, build-specific defaulting, fallback,
  validation, immutability, and update behavior.
- Define exact v2 <-> v3 mapping for absent, empty, defaulted, custom,
  `Managed`, and `Removed` cases.
- State which legacy DSC status fields remain during compatibility.

Definition of done:

- G2/G4 and the AI Hub portion of G7 are `Frozen` with an exact bidirectional
  matrix.
- Public and internal names are clearly separated.
- `95344` can implement the handler without choosing defaults or status names.

### `RHOAIENG-94809`: freeze the consolidated API and conversion matrix

- Local status: `Blocked by 94805, 95339, 95340, G5, and G6`
- Decision owners: Platform plus all affected component API owners
- Blocks: changed public schemas, `94812`, `95342`, `95344`, and `95346`

Purpose:

- Merge component decisions into one authoritative contract and eliminate
  contradictions before types, conversion, or generated CRDs encode them.

Inputs already captured locally:

- D1-D6 and the complete proposals/discrepancies in PLAN.
- Dashboard decision record from `94805`.
- Data decision record from `95339`.
- AI Hub decision record from `95340`.
- Current MaaS behavior: runtime prefers explicit
  `aigateway.modelsAsAService` and otherwise falls back to
  `kserve.modelsAsService`; v1 conversion has additional migration/stash logic.
- Removed v3 candidates: deprecated `trainingoperator` and
  `llamastackoperator`; `ogx` remains the Llama Stack replacement.

Required matrix columns for every changed field:

| Field/condition | v2 JSON/type | v3 JSON/type | v2 -> v3 | v3 -> v2 | absent/empty/default | validation/update | fidelity mechanism |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Dashboard core | From `94805` | From `94805` | Required | Required | Required | Required | Required |
| Dashboard portal | From `94805` | From `94805` | Required | Required | Required | Required | Required |
| Model Registry/AI Hub | From `95340` | From `95340` | Required | Required | Required | Required | Required |
| Feast/Feature Store | `feastoperator` | Proposed `data.featureStore` | Required | Required | Required | Required | Required |
| Data Registry | Proposed v2 compatibility child | Proposed `data.dataRegistry` | Required | Required | Required | Required | Required |
| MaaS | legacy and canonical fields | canonical field | Required, including conflict | Required | Required | Required | Required |
| Training Operator | deprecated field | absent | Required | Required | Required | Required | Required |
| Llama Stack Operator | deprecated field | absent | Required | Required | Required | Required | Required |
| Changed status/conditions | current names | final names | Required | Required | Required | n/a | Required |

Required decisions not owned by a component task:

- G5: when legacy KServe MaaS and canonical AI Gateway MaaS are both present,
  define priority for every empty/Managed/Removed combination and whether a
  conflicting write warns, validates, normalizes, or preserves both for v2.
- G6: define observable behavior when v2 clients read or write non-empty
  Training Operator or Llama Stack Operator values that v3 cannot expose.
- For every v3-only value, choose an explicit v2 field where possible; approve
  an annotation key, lifecycle, and cleanup only if no field can represent it.

Definition of done:

- G1-G7 are all `Frozen` in PLAN.
- The completed matrix has no “TBD”, implied mapping, unnamed condition, or
  unspecified collision.
- Each round-trip has a stated fidelity mechanism and test case.
- Version-specific type ownership prevents accidental changes to v2/internal
  CR schemas.

### `RHOAIENG-94814`: freeze the v1 retirement design

- Local status: `Decision`
- Decision owners: Platform release/OLM and operator upgrade owners
- Blocks: G8 and the v1-removal portion of `94812`

Non-negotiable Kubernetes constraint:

- An API version must not be removed from CRD `spec.versions` while it appears
  in `status.storedVersions`. Marking v3 as storage does not rewrite existing
  objects.

Current repository inventory:

- `api/datasciencecluster/v1` contains the API, v1-to-v2 conversion, and tests.
- `cmd/main.go` registers v1 and v2 schemes.
- `PROJECT` declares v1/v2 API, defaulting, and validation metadata.
- `internal/webhook/webhook.go` and
  `internal/webhook/datasciencecluster/v1` register and implement v1
  admissions; shared envtest helpers also register v1.
- ODH/RHOAI generated CRDs and conversion patches serve v1/v2 and store v2.
- The current v1 conversion owns compatibility behavior for AI Gateway MaaS,
  deprecated KServe MaaS, portal status, and the
  `conversion.opendatahub.io/aigateway-state` annotation.
- V1/v2 references also exist in tests, samples, compare utilities, upgrade
  checks, and bundle/CSV generated content.

Design work:

1. Document actual OLM ordering for CRD and operator deployment on upgrades
   from each supported source release.
2. Select an idempotent mechanism to rewrite the singleton DSC into v3 storage.
3. Define how the mechanism verifies storage migration and removes v1 from
   `status.storedVersions` before a manifest omits v1.
4. Define retries after operator restart, partial failure, object absence,
   webhook unavailability, and already-completed migration.
5. Decide whether delivery needs two releases/phases because the CRD may be
   applied before the new controller runs.
6. Define fresh-install and rollback behavior and the last safe rollback point.

Definition of done:

- G8 is `Frozen` with a step-by-step delivery and rollback sequence.
- Fresh install, supported upgrade, interrupted retry, already-migrated, and
  final `status.storedVersions` tests are specified.
- The design never depends on removing conversion support before stored data
  no longer needs it.

## Component delivery work packages

### `RHOAIENG-95342`: implement the Dashboard handler and module CRD

- Local status: `Blocked by G1/G7 and typed v3 context`
- Out of scope: renaming the existing internal Dashboard CR or metadata

Implementation steps after unblocking:

1. Add/use version-specific public Dashboard types matching the frozen shape;
   do not mutate shared v2 types unintentionally.
2. Update `internal/controller/modules/dashboard` to read typed v3 fields while
   preserving its existing component maps, namespace, gateway, and module-CR
   projection behavior.
3. Preserve operator enablement when any frozen Dashboard subcomponent needs
   the shared operator; implement independent child state exactly as specified.
4. Mirror the frozen status fields and source/DSC condition types and keep
   readiness aggregation correct for removed children.
5. Update the module CRD/schema input and handler/schema-compliance tests.
6. Add v2 compatibility round-trip tests for the existing top-level Dashboard
   and `maasConsumerPortal` states.

Definition of done:

- Every frozen Dashboard state maps to the expected operator lifecycle, module
  CR, status, and conditions.
- Existing v2 Dashboard behavior is unchanged through conversion.
- Focused unit/schema tests and mandatory generated gates pass; evidence is
  recorded using the template.

### `RHOAIENG-95344`: implement the AI Hub handler and module CRD

- Local status: `Blocked by G2/G4/G7 and typed v3 context`
- Out of scope: internal CR/GVK/module metadata rename (`95349`, deferred 3.7)

Implementation steps after unblocking:

1. Add/use the frozen public AI Hub type and v3 field while keeping the
   existing internal GVK `AIHub`, CR `default-aihub`, and module name
   `modelregistry`.
2. Update `internal/controller/modules/modelregistry` to read v3 public state,
   project `registriesNamespace` to internal `instancesNamespace`, and apply
   only the frozen fallback/default rules.
3. Update ready-condition and DSC status mapping to the frozen public names;
   retain legacy status mirroring only where the matrix requires it.
4. Preserve platform-module enablement, application namespace, gateway domain,
   releases, and management-state annotation behavior.
5. Add conversion cases for v2 `modelregistry` and handler/schema-compliance
   tests for empty/default/custom namespace and Managed/Removed states.

Definition of done:

- Public v3 naming does not leak into the unchanged internal resource identity.
- V2 Model Registry and v3 AI Hub objects produce equivalent internal behavior.
- Status/conditions/defaults match the frozen matrix and all required gates
  pass with evidence recorded.

### `RHOAIENG-95346`: implement the Data handler and module CRD

- Local status: `Blocked by G3/G7 and typed v3 context`
- Out of scope: DCH and internal Data/Feast CR or metadata rename (`95350`)

Implementation steps after unblocking:

1. Add/use the frozen `data`/Feature Store/Data Registry public types and the
   approved v2 compatibility field without changing unrelated schemas.
2. Update `internal/controller/modules/feastoperator` and the existing internal
   module schema to project each child lifecycle independently.
3. Preserve the existing external-OIDC issuer projection and error behavior.
4. Define shared-operator enablement from child states exactly as frozen; empty
   state continues to have effective value `Removed`.
5. Mirror each frozen child status and condition independently, including
   removal and readiness aggregation.
6. Add conversion, handler, lifecycle, schema-compliance, and generated CRD
   tests for every two-child state combination.

Definition of done:

- Feature Store and Data Registry can be independently managed and removed as
  defined by the contract, with v2 Feast behavior preserved.
- No DCH or deferred rename is introduced.
- Focused and mandatory gates pass with completion evidence recorded.

## `RHOAIENG-94812`: implement and deliver DSC v3

- Local status: `Blocked in part; mechanical preparation is available`
- Depends on: G1-G8 plus `95342`, `95344`, and `95346`

This work package integrates the contract and component deliveries. Follow the
batches in PLAN in order; a batch may start only when its entry criteria are
met.

### API and conversion

- Add `api/datasciencecluster/v3` and make it the hub/storage API.
- Make v2 an explicit spoke with complete `ConvertTo`/`ConvertFrom` coverage.
- Copy all unchanged spec/status/metadata fields and implement every frozen
  changed-field matrix row.
- Keep conversion deterministic and free of admission or cluster lookups.
- Preserve all v3 state through v2 reads/writes using explicit fields first and
  approved annotations only where unavoidable.
- Update `PROJECT`, scheme registration, component-codegen target, comparison
  helpers, deepcopy generation, and version constants.

### Internal v3 migration

- Migrate the DSC controller, in-tree registry, all module handler interfaces,
  `DSCContext`, base-handler status/reflection, readiness/status aggregation,
  predicates, initial-install helpers, and production utilities to typed v3.
- Migrate all unchanged component handlers and tests, not only the renamed
  components.
- Restrict production v2 imports to the documented conversion/admission
  compatibility allowlist; remove production v1 imports after retirement.
- Keep garbage collection last in every touched action chain.

### Admissions and v1 retirement

- Add v3 mutating/defaulting and validating registration and envtest coverage.
- Preserve compatible v2 admissions, including existing singleton, Kueue,
  deprecated MaaS, KServe NIM, and build-specific registry namespace behavior
  wherever the frozen contract retains it.
- Execute G8's storage migration before removing v1 schema, serving, scheme,
  webhooks, conversion, and v1-only tests.

### Generated and test deliverables

- Regenerate ODH/RHOAI CRDs, webhook patches/manifests, bundles/CSVs, RBAC if
  affected, API docs, deepcopy/object code, and samples.
- Replace primary DSC samples with v3 and retain v2 only as focused
  compatibility/upgrade fixtures.
- Add full conversion round trips, admissions, handler/status, generated CRD,
  fresh-install, v2-to-v3 upgrade, v1 storage migration/retry, and platform
  variant tests.

Final definition of done:

- V3 is the sole storage and production-internal DSC version.
- V2 remains served and preserves all v3 state and existing effective behavior.
- V1 is absent only after its stored version is safely migrated and removed.
- Dashboard, AI Hub, Feature Store, and Data Registry frozen contracts are
  fully projected with correct status and conditions.
- ODH and RHOAI generated deliverables are consistent.
- `make generate manifests api-docs`, `make fmt`, `make lint`,
  `make unit-test`, `make build`, focused tests, and `git diff --check` pass;
  required cluster E2E results or explicit CI handoff are recorded.

## Out-of-scope and downstream records

- `RHOAIENG-94813` (`🚫`): downstream upgrade, compatibility, and rollback
  verification after the operator implementation is available. Operator-owned
  upgrade tests remain part of `94812`; downstream product execution does not.
- `RHOAIENG-95349` (`🚫`, RHOAI 3.7): rename/converge internal AI Hub CR/CRD,
  GVK, and module metadata. Public v3 mapping in 3.6 must use the existing
  internal identity.
- `RHOAIENG-95350` (`🚫`, RHOAI 3.7): rename/converge internal Data/Feast
  CR/CRD, GVK, and metadata. Public v3 mapping in 3.6 must use the existing
  internal identity.
- Data Connection Hub (`🚫`): not part of the 3.6 `data` stanza.
- `odh-cli`, release notes, and product documentation (`🚫` in this repo):
  external owners consume the frozen mapping; their implementation is not an
  operator task.

## Optional provenance

- [RHOAIENG-94804 epic](https://redhat.atlassian.net/browse/RHOAIENG-94804)
- [RHOAIENG-94812 implementation](https://redhat.atlassian.net/browse/RHOAIENG-94812)
- [RHOAIENG-85262 spike](https://redhat.atlassian.net/browse/RHOAIENG-85262)
- [Spike findings document](https://docs.google.com/document/d/1IvAHo3xRd4fHBmzU0Vnpgiq7M1K2w0OWdt7ZRhUIuk4/edit)
