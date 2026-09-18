# DataScienceCluster v3 operator tasks

This is the repository-local task tracker for implementing DSC v3 under
RHOAIENG-94812. It embeds the relevant task descriptions, spike conclusions,
comments, dependencies, current code behavior, and acceptance criteria as of
**2026-09-18**. Jira, Google Docs, and wiki access is not required. Issue keys
are stable work-package identifiers and optional provenance only.

Read [development.md](development.md), [decisions.md](decisions.md), and
[PLAN.md](PLAN.md) before selecting a task. Accepted entries in `decisions.md`
are normative; proposed decisions are hard gates and must not be guessed.
The PLAN contains a cross-cutting high-risk file map, and every implementation
task contains indicative starting files, ownership boundaries, and test entry
points so an agent can begin without Jira discovery or a preliminary scan of
the entire repository.

## How an autonomous agent uses this tracker

1. Select the first numbered task with `status: not_started` whose `depends_on`
   tasks are complete and whose required decisions are all `Accepted` in
   `decisions.md`.
2. Read the task's embedded inputs, current repository state, steps, and
   definition of done. External issue reading is not an entry condition.
3. Change the numbered task file's status: `not_started` -> `in_progress` ->
   `completed`, and mirror it in this summary. Add branch/commit and
   verification evidence to that task's Outcomes.
4. If a required decision is pending, set `status: blocked`, add the decision
   ID to `blocked_by`, and proceed to another independent task. Do not invent a
   lifecycle status such as `Blocked (Gx)` or copy a provisional example into
   the public API.
5. When an approved decision is provided, record the exact normative contract
   in `decisions.md` first. Then update PLAN/TASKS summaries and clear the
   corresponding blocker. PLAN or TASKS never becomes a competing authority.
6. Preserve unrelated worktree changes and keep garbage collection last in
   every controller action chain.
7. For every v3 data-model change, update or explicitly confirm both conversion
   directions and add focused forward/backward/round-trip tests in the same
   change. Handler tests alone are insufficient.
8. A code task is not complete until required generated changes are included
   and its Outcomes contain the commands and results.

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
| [`DSC-V3-001`](tasks/001.md) v3 machinery and v1 removal | `🚧 In progress` | None; Task 010 separately blocks upgrade release | Identical v3 API, v2 <-> v3 identity conversion, typed v3 runtime, and v1-free API |
| [`RHOAIENG-94805`](tasks/002.md) Dashboard contract | `⛔ Blocked; decision task` | Dashboard/Workbenches/Platform owner choice | Freeze G1 and Dashboard parts of G7 |
| [`RHOAIENG-95339`](tasks/003.md) Data contract | `⛔ Blocked; decision task` | Data/Feast/Data Registry/Platform owner choice | Freeze G3 and Data parts of G7 |
| [`RHOAIENG-95340`](tasks/004.md) AI Hub contract | `⛔ Blocked; decision task` | AI Hub/Model Registry/Platform owner choice | Freeze G2/G4 and AI Hub parts of G7 |
| [`RHOAIENG-94809`](tasks/005.md) consolidated contract | `⛔ Blocked` | `94805`, `95339`, `95340`, G5/G6 | Freeze G1-G7 and exact conversion matrix |
| [`RHOAIENG-94814`](tasks/010.md) v1 retirement qualification | `⛔ Blocked` | odh-cli implementation, DEC-018 promotion control, DEC-020 execution matrix | Produce release evidence and qualify the gate/rollback boundary |
| [`RHOAIENG-95342`](tasks/007.md) Dashboard handler/CRD | `📝 Not started` | Waits for Task 006 | Dashboard projection and status |
| [`RHOAIENG-95344`](tasks/008.md) AI Hub handler/CRD | `📝 Not started` | Waits for Task 006 | AI Hub projection and status |
| [`RHOAIENG-95346`](tasks/009.md) Data handler/CRD | `📝 Not started` | Waits for Task 006 | Feature Store/Data Registry projection |
| [`RHOAIENG-94812`](tasks/006.md) contract implementation | `⛔ Blocked after M1` | G1-G7 | Implement approved v3 API and conversion |
| [`RHOAIENG-94812`](tasks/011.md) final qualification | `📝 Not started` | Waits for Tasks 006-010 | Complete DSC v3 delivery |

```text
001-01 v3 API ─┬─> 001-02 atomic conversion/v1 removal ─┐
               └─> 001-03 runtime migration ──────────┼─> 001-04 artifacts ─> 001-05 integration
001-05 ──> 001 roll-up complete

002 Dashboard contract ─┐
003 Data contract ──────┼─> 005 consolidated G1-G7 matrix
004 AI Hub contract ────┘

001 complete ─┬─> 006 API/conversion ─┬─> 007 Dashboard projection ─┐
005 complete ─┘                       ├─> 008 AI Hub projection ────┼─> 011 final qualification
                                      └─> 009 Data projection ──────┘
001 complete ─> 010 future odh-cli qualification ────────────────┘
```

Task 001 is the first executable work and does not wait for component
contracts. It copies the v2 shape to v3, adds identity conversion, moves
production code to v3, and removes v1. The external odh-cli gate runs before
OLM installs that v1-free CRD, but its implementation and qualification are the
separate future Task 010 and do not block Task 001 implementation completion.
Public stanza changes and transforming conversion remain blocked until G1-G7
have accepted decisions.

## Numbered task execution order

Each file is a living execution record. Dependencies, not numeric order alone,
control concurrency.

`promotion_requires` is not a completion dependency: it prevents promotion of
an artifact after implementation completes. Task orchestrators must exclude it
from DAG cycle detection. Task 001 uses it to point to future Task 010.

| Task | Purpose | Depends on |
| --- | --- | --- |
| [001](tasks/001.md) | Roll up the initial machinery milestone | 001-01 through 001-05 |
| [001-01](tasks/001-01.md) | Capture baseline and introduce identical v3 API | None |
| [001-02](tasks/001-02.md) | Atomically switch hub/storage, add conversion/webhooks, and remove v1 | 001-01 |
| [001-03](tasks/001-03.md) | Migrate production runtime from typed v2 to typed v3 | 001-01 |
| [001-04](tasks/001-04.md) | Deprecate v2 and finalize v1-free generated artifacts | 001-02, 001-03 |
| [001-05](tasks/001-05.md) | Integrate and verify the machinery milestone | 001-02 through 001-04 |
| [002](tasks/002.md) | Freeze Dashboard public contract | Owner approval |
| [003](tasks/003.md) | Freeze Data public contract | Owner approval |
| [004](tasks/004.md) | Freeze AI Hub public contract | Owner approval |
| [005](tasks/005.md) | Consolidate G1-G7 and conversion matrix | 002-004 plus MaaS/legacy decisions |
| [006](tasks/006.md) | Implement accepted v3 API, conversion, and admissions | 001, 005 |
| [007](tasks/007.md) | Implement Dashboard projection/status | 006 |
| [008](tasks/008.md) | Implement AI Hub projection/status | 006 |
| [009](tasks/009.md) | Implement Data projection/status | 006 |
| [010](tasks/010.md) | Future: qualify odh-cli gate, storage migration, and rollback | 001 plus external gate |
| [011](tasks/011.md) | Integrate and qualify the final release | 006-010 |

## Completion evidence template

Copy this block into the active task and fill it in before marking complete:

```text
Implementation:
- Branch/commit:
- Accepted decision IDs used:
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

## Mandatory checklist for every future v3 data change

Use this checklist for any addition, removal, move, rename, type change, JSON
tag, optionality, default, validation, status field, or condition change under
the v3 DSC API. All boxes must be satisfied in the same change:

- [ ] Record the v2 field path/type and the new v3 field path/type.
- [ ] Define and implement `v2 -> v3` for populated, absent, empty, and legacy
      inputs.
- [ ] Define and implement `v3 -> v2`, including lossless preservation through
      a v2 read-modify-write when v2 cannot represent the value directly.
- [ ] Add focused direct-conversion tests in both directions.
- [ ] Add `v2 -> v3 -> v2` and `v3 -> v2 -> v3` round-trip tests.
- [ ] Cover changed status fields and condition names, not only spec fields.
- [ ] Update the explicit v2/v3 structural-difference inventory and prove that
      every listed difference has a conversion test.
- [ ] Give every difference a stable `case_id` in the shared machine-readable
      registry and prove the runner executed forward, backward, v2-round-trip,
      and v3-round-trip categories for it.
- [ ] Leave the independent v2 baseline unchanged unless an accepted decision
      explicitly changes the v2 wire contract and lists/tests those paths.
- [ ] Update defaulting, validation, generated CRDs, samples, and API docs when
      the wire contract changes.
- [ ] Run the focused conversion suite and mandatory repository gates and
      record the results in the task evidence.

If converter code remains unchanged because the new data uses an existing
explicit identity-copy path, add a test for the new field and record that
decision. “No converter change required” never means “no conversion test
required.”

## First executable work package

### `DSC-V3-001`: establish v3 machinery

- Local status: `In progress`
- Blocked by: nothing
- Must not include: any unresolved Dashboard, AI Hub, Data, MaaS, Training
  Operator, or Llama Stack schema/behavior change

Detailed executable instructions, lifecycle metadata, acceptance criteria, and
implementation outcomes are maintained in [tasks/001.md](tasks/001.md). That
file is the task execution record; this section is its summary.

Purpose:

- Establish the final versioning architecture before component contracts are
  resolved. V3 initially has exactly the v2 API shape, making v2/v3 conversion
  a semantic no-op. Move the operator to typed v3 and remove v1 from the new
  code/artifacts. Odh-cli work is deferred to separate Task 010.

Terminology:

- The new operator converter is **v2 <-> v3**. It does not need a v1 <-> v3
  converter. Future Task 010 qualifies the external gate that completes v1 ->
  v2 storage migration while the old operator is still installed.

#### Step 1: introduce an identical v3 API

- Add `api/datasciencecluster/v3/groupversion_info.go` for version `v3`.
- Add v3 `DataScienceCluster`, list, spec, status, and component types by
  reproducing the current v2 public shape exactly, including deprecated fields,
  JSON tags, markers, status entries, release fields, conditions, related
  objects, and error message.
- Do not apply any provisional component proposal. In particular, v3 still
  uses the current v2 Dashboard, `modelregistry`, `feastoperator`, legacy MaaS,
  `trainingoperator`, and `llamastackoperator` fields in this milestone.
- Leave v2 as the temporary hub/storage version during 001-01. In 001-02, move
  `+kubebuilder:storageversion` and `conversion.Hub` to v3 atomically with v2
  conversion activation and v1 deletion.
- Generate v3 deepcopy/object code.
- Register v3 in `PROJECT`, `cmd/main.go`, webhook/envtest schemes, and any
  scheme builders or object factories that enumerate DSC versions.
- Change `cmd/component-codegen/cmd/generator/generator.go` so scaffolding uses
  the v3 DSC types file.
- Keep v3 top-level types version-owned. Do not edit shared component types to
  create v3 behavior; introduce v3-owned nested types when a stanza diverges.

#### Step 2: add no-op v2 <-> v3 conversion

- Make v2 the conversion spoke and implement `ConvertTo`/`ConvertFrom` against
  the v3 hub.
- Copy complete object metadata, spec, and status without renaming, dropping,
  defaulting, validation, normalization, or cluster calls. The implementation
  must use explicit typed copy/conversion code so later accepted mappings have a
  clear extension point; do not use JSON serialization or `unsafe`. Equality
  of externally observable fields is the acceptance contract.
- Add whole-object round-trip tests with every component/status populated,
  conditions, releases, related objects, annotations/labels/finalizers,
  empty/Managed/Removed management states, optional values, and deprecated
  fields.
- Assert both `v2 -> v3 -> v2` and `v3 -> v2 -> v3` semantic equality and
  conversion idempotence, ignoring only the expected target-version
  `TypeMeta`/GVK representation.
- Add direct one-hop equality tests so mutually inverse but incorrect mappings
  cannot pass only through round-trip cancellation.
- Move controller-runtime `/convert` registration to the v2 spoke, use distinct
  v2/v3 GVK constants, and test each version's admissions independently.
- Delete the v1 API/converter in this same compiling change because the current
  v1 converter asserts that v2 is the hub. Do not add a temporary v1/v3
  converter.
- Generate `conversionReviewVersions: [v1]`; it is the Kubernetes protocol
  version and must not contain DSC versions `v2` or `v3`.

#### Step 3: amend production code to use v3

- Replace typed v2 usage in `internal/controller/datasciencecluster` and its
  reconciliation/status tests.
- Replace v2 in `internal/controller/components/registry/registry.go` and every
  in-tree component handler signature/fixture.
- Replace v2 in `internal/controller/modules/types.go`, `base.go`, module
  registry/lifecycle/status code, and every module handler/test.
- Replace v2 in status helpers, readiness aggregation, predicates,
  initial-install helpers, comparison utilities, upgrade helpers, E2E object
  factories, and component-codegen.
- Preserve behavior because v3 is shape-identical. Do not opportunistically
  rename fields or change defaults/validation.
- Keep garbage collection last in every touched action chain.

After migration, this search must return only conversion, v2 admission, and
explicit compatibility tests, with each remaining production hit justified in
the completion record:

```bash
rg -n 'datasciencecluster/v2|dscv2' api cmd internal pkg tests -g '*.go'
```

#### Step 4: remove v1 and hand off upgrade gating

- Do not implement or qualify odh-cli in Task 001. Future Task 010 owns the
  DEC-009 gate and must complete before the v1-free CRD is promoted or installed
  as an upgrade.
- Move shared conversion webhook registration to v2 before deleting the v1
  registration path.
- Remove the v1 API, converter, scheme, admissions, `PROJECT` entries,
  generated serving, fixtures, samples, and obsolete version-specific tests.
- Preserve useful compatibility behavior in v2/v3 tests rather than deleting
  it with v1. Delete the AI Gateway conversion annotation only if it has no
  remaining consumer.
- Record the v1-free artifacts and assumptions handed to Task 010. Task 010
  owns stored-version migration, OLM ordering, retry, and rollback tests.

#### Step 5: amend admissions, tests, and generated assertions

- Add v3 defaulting/validation registration by porting the existing v2 rules
  unchanged, because the schemas are identical. Retain v2 admissions for v2
  clients and identify v2 as deprecated in supported warnings/documentation.
- Replace/adapt `pkg/dsc/compare/v2only.go` with a v2/v3 structural coverage
  guard that compares complete normalized generated OpenAPI: paths, types,
  requiredness, nullability, defaults, enums, CEL, and list/map semantics. The
  Task 001 expected set is empty; every future difference must have focused
  conversion coverage.
- Update unit/envtest/integration tests so the controller operates on v3 and a
  v2 API request converts losslessly to/from v3 storage.
- Assert the generated DSC CRD has exactly:
  - v2: `served: true`, `storage: false`, `deprecated: true`, with DEC-019's
    exact `deprecationWarning` literal;
  - v3: `served: true`, `storage: true`;
  - no v1 entry;
  - webhook conversion path `/convert`; and
  - `conversionReviewVersions: [v1]`.
- Update the primary ODH/RHOAI DSC samples to `apiVersion:
  datasciencecluster.opendatahub.io/v3` without changing their component
  stanza shapes.
- Do not treat the existing `tests/e2e/v2tov3upgrade_test.go` name as evidence;
  add or rewrite coverage so it actually verifies DSC v2 API requests against
  v3 storage.

Definition of done:

- V3 and v2 schemas are structurally identical in generated CRDs.
- V3 is the hub, only storage version, and only typed DSC version used by
  production reconciliation.
- V2 is served but deprecated, with lossless identity conversion and compatible
  admissions.
- V1 code, conversion, admission, scheme, and generated serving are absent.
- The artifact is explicitly ineligible for upgrade promotion until Task 010
  qualifies the external odh-cli gate; that qualification is not Task 001 work.
- The structural coverage guard proves that v2/v3 are identical and establishes
  the required inventory for future differences.
- No G1-G7 component contract is implemented prematurely.
- All focused tests and mandatory repository gates pass, and completion
  evidence is recorded.

## Contract work packages

### `RHOAIENG-94805`: freeze the Dashboard v3 contract

- Local status: `Blocked; decision task`
- Detailed task and outcome record: [tasks/002.md](tasks/002.md)
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

- G1 and the Dashboard portion of G7 have accepted decisions with exact shapes,
  mappings, approval date, and owner evidence.
- No proposed name remains in the normative contract.
- `95342` can implement every state without making another product decision.

### `RHOAIENG-95339`: freeze the Data, Feature Store, and Data Registry contract

- Local status: `Blocked; decision task; discrepancy open`
- Detailed task and outcome record: [tasks/003.md](tasks/003.md)
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

- G3 and the Data portion of G7 have accepted decisions; the parent-state conflict is gone.
- The v2 compatibility mechanism is exact and lossless.
- `95346` can implement internal projection and status without adding DCH or
  renaming internal resources.

### `RHOAIENG-95340`: freeze the AI Hub/Model Registry contract

- Local status: `Blocked; decision task; naming discrepancy open`
- Detailed task and outcome record: [tasks/004.md](tasks/004.md)
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

- G2/G4 and the AI Hub portion of G7 have accepted decisions with an exact bidirectional
  matrix.
- Public and internal names are clearly separated.
- `95344` can implement the handler without choosing defaults or status names.

### `RHOAIENG-94809`: freeze the consolidated API and conversion matrix

- Local status: `Blocked by 94805, 95339, 95340, G5, and G6`
- Detailed task and outcome record: [tasks/005.md](tasks/005.md)
- Decision owners: Platform plus all affected component API owners
- Blocks: changed public schemas, `94812`, `95342`, `95344`, and `95346`

Purpose:

- Merge component decisions into one authoritative contract and eliminate
  contradictions before types, conversion, or generated CRDs encode them.

Inputs already captured locally:

- All accepted decisions, including DEC-017, DEC-019, and DEC-021, plus the
  complete proposals/discrepancies in PLAN. DEC-018 and DEC-020 apply only to
  future Task 010/release work.
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

- G1-G7 all have accepted decisions recorded in `decisions.md`.
- The completed matrix has no “TBD”, implied mapping, unnamed condition, or
  unspecified collision.
- Each round-trip has a stated fidelity mechanism and test case.
- Every intentional v2/v3 structural difference has an inventory entry and
  named forward, backward, and round-trip test cases.
- Version-specific type ownership prevents accidental changes to v2/internal
  CR schemas.

### `RHOAIENG-94814`: qualify v1 removal for release

- Local status: `Future; blocked by odh-cli implementation, DEC-018, and DEC-020`
- Detailed task and outcome record: [tasks/010.md](tasks/010.md)
- Decision owners: Platform release/OLM and operator upgrade owners
- Release dependency: the external odh-cli gate must be available and pass
  qualification before shipping the v1-removing generated CRD as an upgrade
- Promotion dependency: DEC-018 must name an enforceable release/CI control;
  documentation alone must not permit publication
- Execution dependency: DEC-020 must enumerate exact supported sources,
  artifacts, clusters, permissions, odh-cli invocation, and CI jobs locally

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

Accepted odh-cli gate contract:

1. Run before OLM applies the new CRD, against the installed v1/v2 CRD while
   the old operator and its conversion webhook are healthy.
2. Read the CRD and `status.storedVersions`. Mutation-free success is allowed
   only when v1 is already absent.
3. If v1 is recorded and no DSC exists, skip object migration but patch the CRD
   status to remove v1 and verify it is absent.
4. If v1 is recorded and the singleton exists, read and update the DSC through
   v2 without changing its spec, status, or metadata, then verify it no longer
   depends on v1 storage.
5. Only after verification, patch the CRD status subresource to remove v1 from
   `status.storedVersions`; read it back and require v1 to be absent.
6. Block the upgrade on an unsupported source, conversion/webhook failure,
   concurrent-conflict exhaustion, failed verification, or failed status
   update. Every partial state must be safe to retry.
7. Allow OLM to apply the v1-free CRD only after the gate succeeds. The new
   CRD serves deprecated v2 and v3 and stores only v3.
8. After `/convert` through v2 is healthy, update the DSC through v3, verify v3
   storage, and remove v2 from `status.storedVersions` only when no v2-stored
   object remains. Continue serving v2.
8. Report the last safe rollback point. Before the v3 rewrite, rollback to the
   v1/v2 release is supported. Afterwards, require a tested v3 -> v2
   down-migration or declare/block rollback as unsupported.

Qualification work:

- Verify the gate is positioned before OLM CRD replacement for every supported
  ODH and RHOAI source-release path.
- Test no DSC, v1-stored, already migrated, interrupted/retried, webhook down,
  object conflict, status-patch failure, and verification-failure cases.
- Verify the new operator never needs v1 code and registers `/convert` through
  v2 before the post-upgrade v3 rewrite.
- Exercise both supported and deliberately blocked rollback paths.

Definition of done:

- The odh-cli gate implementation matches accepted DEC-009 and is available in
  the supported upgrade workflow before the v1-free CRD can be applied.
- Fresh install, supported upgrade, interrupted retry, already-migrated, and
  final `status.storedVersions` tests are specified.
- The design never depends on removing conversion support before stored data
  no longer needs it.

## Component delivery work packages

### `RHOAIENG-95342`: implement the Dashboard handler and module CRD

- Local status: `Not started; waits for DSC-V3-006`
- Detailed task and outcome record: [tasks/007.md](tasks/007.md)
- Out of scope: renaming the existing internal Dashboard CR or metadata

Implementation steps after unblocking:

1. Add/use version-specific public Dashboard types matching the accepted shape;
   do not mutate shared v2 types unintentionally.
2. Update `internal/controller/modules/dashboard` to read typed v3 fields while
   preserving its existing component maps, namespace, gateway, and module-CR
   projection behavior.
3. Preserve operator enablement when any accepted Dashboard subcomponent needs
   the shared operator; implement independent child state exactly as specified.
4. Mirror the accepted status fields and source/DSC condition types and keep
   readiness aggregation correct for removed children.
5. Update the module CRD/schema input and handler/schema-compliance tests.
6. Add v2 compatibility round-trip tests for the existing top-level Dashboard
   and `maasConsumerPortal` states.

Definition of done:

- Every accepted Dashboard state maps to the expected operator lifecycle, module
  CR, status, and conditions.
- Existing v2 Dashboard behavior is unchanged through conversion.
- The mandatory future-v3-change checklist is complete for every Dashboard
  spec/status/condition difference.
- Focused unit/schema tests and mandatory generated gates pass; evidence is
  recorded using the template.

### `RHOAIENG-95344`: implement the AI Hub handler and module CRD

- Local status: `Not started; waits for DSC-V3-006`
- Detailed task and outcome record: [tasks/008.md](tasks/008.md)
- Out of scope: internal CR/GVK/module metadata rename (`95349`, deferred 3.7)

Implementation steps after unblocking:

1. Add/use the accepted public AI Hub type and v3 field while keeping the
   existing internal GVK `AIHub`, CR `default-aihub`, and module name
   `modelregistry`.
2. Update `internal/controller/modules/modelregistry` to read v3 public state,
   project `registriesNamespace` to internal `instancesNamespace`, and apply
   only the accepted fallback/default rules.
3. Update ready-condition and DSC status mapping to the accepted public names;
   retain legacy status mirroring only where the matrix requires it.
4. Preserve platform-module enablement, application namespace, gateway domain,
   releases, and management-state annotation behavior.
5. Add conversion cases for v2 `modelregistry` and handler/schema-compliance
   tests for empty/default/custom namespace and Managed/Removed states.

Definition of done:

- Public v3 naming does not leak into the unchanged internal resource identity.
- V2 Model Registry and v3 AI Hub objects produce equivalent internal behavior.
- Status/conditions/defaults match the accepted matrix and all required gates
  pass with evidence recorded.
- The mandatory future-v3-change checklist is complete for every AI Hub
  spec/status/condition difference.

### `RHOAIENG-95346`: implement the Data handler and module CRD

- Local status: `Not started; waits for DSC-V3-006`
- Detailed task and outcome record: [tasks/009.md](tasks/009.md)
- Out of scope: DCH and internal Data/Feast CR or metadata rename (`95350`)

Implementation steps after unblocking:

1. Add/use the accepted `data`/Feature Store/Data Registry public types and the
   approved v2 compatibility field without changing unrelated schemas.
2. Update `internal/controller/modules/feastoperator` and the existing internal
   module schema to project each child lifecycle independently.
3. Preserve the existing external-OIDC issuer projection and error behavior.
4. Define shared-operator enablement from child states exactly as accepted; empty
   state continues to have effective value `Removed`.
5. Mirror each accepted child status and condition independently, including
   removal and readiness aggregation.
6. Add conversion, handler, lifecycle, schema-compliance, and generated CRD
   tests for every two-child state combination.

Definition of done:

- Feature Store and Data Registry can be independently managed and removed as
  defined by the contract, with v2 Feast behavior preserved.
- No DCH or deferred rename is introduced.
- The mandatory future-v3-change checklist is complete for every Data, Feature
  Store, and Data Registry spec/status/condition difference.
- Focused and mandatory gates pass with completion evidence recorded.

## `RHOAIENG-94812`: implement and deliver DSC v3

- Local status: `Milestone 1 ready; later milestones blocked in part`
- Detailed task records: [machinery task 001](tasks/001.md),
  [API/conversion task 006](tasks/006.md), and
  [final qualification task 011](tasks/011.md)
- Milestone 1 dependencies: none
- Release dependency: accepted DEC-009's odh-cli gate and qualification
- Later contract dependencies: G1-G7 plus `95342`, `95344`, and `95346`

Follow the milestones in PLAN in order. The detailed `94812/M1` package above
is the first work to implement; it must land without waiting for stanza tasks.

### Milestone 1: versioning machinery

- Introduce v3 with exactly the v2 shape and make it hub/storage/runtime.
- Add semantic identity v2 <-> v3 conversion.
- Move production controllers, registries, modules, handlers, status, helpers,
  generators, and tests from typed v2 to typed v3.
- Remove v1 API/conversion/admission/serving. Do not implement odh-cli here;
  future Task 010 separately qualifies v1 storage migration before upgrade
  release.
- Port v2 admission behavior to identical v3 fields, retain v2 compatibility,
  regenerate all artifacts, and prove lossless whole-object round trips.

### Milestone 2 and 3: accepted contract changes

- After G1-G7 freeze, change the v3 schema for Dashboard, AI Hub, Data, MaaS,
  and removed legacy fields.
- Evolve the identity converter only for those approved differences while
  retaining exact copy behavior for everything else.
- Integrate `95342`, `95344`, and `95346`, then adapt v3 admissions and all
  handler/status/schema tests.

### Milestone 4: final release qualification

- Run fresh-install, supported-upgrade, interrupted-migration/retry, rollback,
  v2-client, and ODH/RHOAI platform-variant tests.

Final definition of done:

- V3 is the sole storage and production-internal DSC version.
- V2 remains served and preserves all v3 state and existing effective behavior.
- V1 is absent from code and generated APIs; release tests prove existing
  storage is migrated before an installed CRD drops v1.
- Dashboard, AI Hub, Feature Store, and Data Registry accepted contracts are
  fully projected with correct status and conditions.
- Every v2/v3 difference is explicitly inventoried and covered by direct and
  round-trip conversion tests as required by DEC-003 and DEC-007.
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
- The odh-cli executable and packaging (`🚫` in this repo): external owners
  implement the accepted gate. Its contract, release dependency, and
  operator-side upgrade qualification remain part of this plan. Release notes
  and product documentation are also externally owned.

## Optional provenance

- [RHOAIENG-94804 epic](https://redhat.atlassian.net/browse/RHOAIENG-94804)
- [RHOAIENG-94812 implementation](https://redhat.atlassian.net/browse/RHOAIENG-94812)
- [RHOAIENG-85262 spike](https://redhat.atlassian.net/browse/RHOAIENG-85262)
- [Spike findings document](https://docs.google.com/document/d/1IvAHo3xRd4fHBmzU0Vnpgiq7M1K2w0OWdt7ZRhUIuk4/edit)
