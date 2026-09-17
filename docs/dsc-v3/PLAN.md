# RHOAIENG-94812: DataScienceCluster v3 operator implementation plan

This document describes the operator-side work required to implement
[RHOAIENG-94812](https://redhat.atlassian.net/browse/RHOAIENG-94812) as part of
[RHOAIENG-94804](https://redhat.atlassian.net/browse/RHOAIENG-94804).

The plan is based on Jira, the spike document, its Jira and Google Docs
comments, and the relevant wiki material as reviewed on **2026-09-17**. The
component contracts are not final while their `Finalize ...` tasks remain open.
Where those sources disagree, this plan records a decision gate instead of
choosing a contract implicitly.

## Sources

- [RHOAIENG-94804: DSC v3 epic](https://redhat.atlassian.net/browse/RHOAIENG-94804)
- [RHOAIENG-94812: operator implementation](https://redhat.atlassian.net/browse/RHOAIENG-94812)
- [RHOAIENG-85262: original spike](https://redhat.atlassian.net/browse/RHOAIENG-85262)
- [Spike findings document](https://docs.google.com/document/d/1IvAHo3xRd4fHBmzU0Vnpgiq7M1K2w0OWdt7ZRhUIuk4/edit)
- [AI Core Platform wiki summary](https://redhat.atlassian.net/wiki/spaces/~712020ae5659d45615471ca83c37c75652ea3e/pages/460554960/AI+Core+Platform+Features+and+Epics+In-Progress+Summary)
- [Kubernetes CRD versioning and old-version removal](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/)

The implementation dependencies and their current state are tracked in
[TASKS.md](TASKS.md).

## Goals

- Serve `datasciencecluster.opendatahub.io/v3` and make it the only storage and
  operator-internal DSC version.
- Continue serving v2 through a v2-to-v3 conversion webhook and v2-compatible
  admission behavior.
- Retire v1 without making the CRD upgrade invalid or leaving v1 in
  `status.storedVersions`.
- Preserve the effective behavior of existing v2 configurations and preserve
  v3 state across v2 reads and writes.
- Move all production controllers, handlers, registries, status writers, and
  utilities to typed v3 objects.
- Implement the final Dashboard, AI Hub, Data, Feature Store, and Data Registry
  public contracts and their internal module projections.
- Regenerate and validate every derived CRD, bundle, sample, API reference, and
  deepcopy artifact.

## Non-goals

- Renaming the internal AI Hub/Model Registry module CR or module metadata;
  that work is deferred to RHOAI 3.7 by RHOAIENG-95349.
- Renaming the internal Data/Feast module CR or module metadata; that work is
  deferred to RHOAI 3.7 by RHOAIENG-95350.
- Integrating Data Connection Hub (DCH), which is deferred beyond the 3.6 DSC
  contract.
- Removing v2 serving or v2 admission compatibility.
- Using conversion to validate resources. Validation remains the responsibility
  of CRD schemas and admission webhooks.

## Contract freeze gates

Do not treat the examples in this document as the final API until all of the
following have been approved and incorporated into RHOAIENG-94809:

1. RHOAIENG-94805 defines the final Dashboard structure and field names.
2. RHOAIENG-95339 defines the final Data, Feature Store, Data Registry, and v2
   compatibility structures.
3. RHOAIENG-95340 defines the final AI Hub structure, validation, optionality,
   defaults, status, and conditions.
4. RHOAIENG-94809 publishes one authoritative spec, status, condition, default,
   collision, and conversion matrix.
5. RHOAIENG-94814 defines a safe v1 retirement procedure that works with the
   repository's OLM and CRD delivery ordering.

Schema implementation, generated artifacts, and component handler completion
must use the approved RHOAIENG-94809 matrix. Existing code can be inventoried
and mechanical v3 migration work can be prepared in parallel, but no unresolved
field name or conversion precedence should be embedded in generated APIs.

## Provisional public API

Fields not listed below retain their v2 semantics and JSON names in v3.

| Area | v2 | Provisional v3 | Contract owner |
| --- | --- | --- | --- |
| Dashboard | `dashboard.managementState` and `dashboard.maasConsumerPortal` | Final structure and names are pending | RHOAIENG-94805 |
| Model Registry / AI Hub | `modelregistry.managementState`, `modelregistry.registriesNamespace` | `aiHub.managementState`, `aiHub.registriesNamespace` | RHOAIENG-95340 |
| Feature Store | `feastoperator.managementState` | `data.featureStore.managementState` | RHOAIENG-95339 |
| Data Registry | Proposed v2 compatibility field `feastoperator.dataRegistry.managementState` | `data.dataRegistry.managementState` | RHOAIENG-95339 |
| MaaS | Canonical `aigateway.modelsAsAService`; deprecated `kserve.modelsAsService` remains present | Only `aigateway.modelsAsAService` | RHOAIENG-94809 |
| Training Operator | Deprecated `trainingoperator` | Not exposed | RHOAIENG-94809 |
| Llama Stack Operator | Deprecated `llamastackoperator` | Not exposed; use `ogx` | RHOAIENG-94809 |

The Data proposal makes `data` a structural grouping with no top-level
`managementState`. Feature Store and Data Registry are independently managed.
Empty component or subcomponent management state continues to mean `Removed`.

### Known source discrepancies

RHOAIENG-94809 must explicitly resolve these discrepancies before the contract
is considered frozen:

- The epic and the current RHOAIENG-94809 description use `hub`; the newer
  RHOAIENG-95340 contract requires `aiHub` and rejects both `hub` and
  `modelregistry` in v3.
- RHOAIENG-94809 currently says a parent Data `Removed` state overrides its
  children; RHOAIENG-95339 says `data` has no parent management state.
- The spike proposed an annotation stash for v3-only Data Registry state. The
  newer Data task instead proposes an explicit v2
  `feastoperator.dataRegistry.managementState` compatibility field.
- Dashboard comments question both `openshiftAI` and
  `maasCustomerPortal`. The final names and the mapping from the existing v2
  fields are still open.
- The exact behavior for legacy v2 fields that are `Managed` when written or
  read through v3 is not yet specified. The conversion matrix must define this
  without silently changing effective component state.
- When both deprecated `kserve.modelsAsService` and canonical
  `aigateway.modelsAsAService` are present, the matrix must state which value
  wins and how conflicting writes are handled.

## Implementation

### 1. Define version-specific API types

- Add `api/datasciencecluster/v3` with `DataScienceCluster`, list, spec, status,
  components, group/version registration, and deepcopy support.
- Start from the v2 API and apply only the contract changes approved in
  RHOAIENG-94809. Preserve all unaffected fields, validation, status data,
  release data, and related objects.
- Introduce explicitly named DSC v3 component types where the public structure
  differs. Do not mutate a shared component type in a way that accidentally
  changes the v2 schema or an internal module CR schema.
- Keep the v2 public shape stable except for compatibility fields explicitly
  approved by the contract, such as the proposed Data Registry field.
- Move the `+kubebuilder:storageversion` marker and `conversion.Hub`
  implementation from v2 to v3. V2 becomes a conversion spoke.
- Update project metadata and the component-code generator's DSC target so new
  component scaffolding uses the internal v3 API.

### 2. Implement explicit v2-to-v3 conversion

Implement `ConvertTo` and `ConvertFrom` on the v2 DSC type, targeting the v3
hub. Conversion must:

- Copy `ObjectMeta`, all unchanged spec fields, common status, conditions,
  related objects, error message, release data, and unchanged component status
  without loss.
- Apply the approved Dashboard spec, status, and condition mappings.
- Map v2 Model Registry spec/status to v3 AI Hub spec/status and map the ready
  condition name approved by RHOAIENG-94809.
- Map v2 Feast/compatibility fields to the independent v3 Feature Store and
  Data Registry fields, including status and conditions.
- Map the legacy KServe MaaS field to the canonical AI Gateway field according
  to the contract's precedence rule.
- Apply the approved behavior for removed Training Operator and Llama Stack
  Operator fields.
- Normalize empty management state to the existing `Removed` semantics wherever
  a concrete state is required.
- Preserve v3-only values through a v2 read-modify-write. Prefer explicit v2
  compatibility fields where approved. Use an internal conversion annotation
  only for state the final contract proves cannot be represented in v2.
- Keep conversion deterministic, idempotent, and free of cluster lookups.

Add table-driven tests for both round trips:

- v2 -> v3 -> v2 preserves every v2-representable field and effective state.
- v3 -> v2 -> v3 preserves every v3 field, including independently managed
  subcomponents.
- Empty, `Managed`, and `Removed` states; absent optional fields; status and
  condition renames; conflicting legacy/canonical MaaS inputs; and annotations
  are covered explicitly.

### 3. Migrate the operator's internal DSC type to v3

Replace typed v2 usage with v3 throughout production code, including:

- Scheme registration and the DSC controller/reconciler.
- In-tree component registry interfaces and component handlers.
- Module `DSCContext`, handler interfaces, base handler status reflection,
  lifecycle actions, readiness aggregation, and legacy status writers.
- Dashboard, AI Hub/Model Registry, Data/Feast, AI Gateway, KServe, and all
  unchanged module handlers.
- Status helpers, GVK/constants, comparison utilities, initialization helpers,
  tests, and code generators.

After migration, production v2 imports are allowed only in the v2 API conversion
and v2 admission compatibility paths. Production code must have no v1 DSC
imports after the v1 retirement phase.

### 4. Integrate component-owned handler and CRD changes

These deliveries may proceed in parallel after their public contracts are
frozen, but all are required before RHOAIENG-94812 is complete:

- **Dashboard (RHOAIENG-95342):** project the approved v3 Dashboard structure
  into the existing Dashboard module CR, and propagate core Dashboard and
  portal state, status, and conditions while preserving v2 behavior.
- **AI Hub (RHOAIENG-95344):** project public `components.aiHub` into the
  existing internal module CR and preserve `registriesNamespace`, status,
  conditions, and v2 Model Registry behavior. Do not rename the internal CR or
  metadata in 3.6.
- **Data (RHOAIENG-95346):** project `data.featureStore` and
  `data.dataRegistry` independently into the existing internal module API,
  including independent lifecycle, status, and condition propagation. Do not
  add DCH or perform the deferred internal CR rename.

Each component delivery must update its vendored/module CRD schema, handler
tests, schema-compliance tests, and generated artifacts.

### 5. Add v3 admission and retain v2 compatibility

- Add v3 mutating/defaulting and validating registrations, markers, routes, and
  envtest coverage.
- Port common v2 behavior to v3, then replace version-specific checks with the
  rules approved for the new shapes.
- Retain v2 admission handlers for supported v2 clients. Their defaults and
  validations must remain compatible with conversion into v3 storage.
- Remove v1 admission registration, markers, webhook handlers, and v1-only
  admission tests only as part of the approved retirement phase.
- Ensure conversion failures are not used as a substitute for admission
  validation.

### 6. Retire v1 safely

RHOAIENG-94814 must turn this section into a release-compatible procedure before
implementation. Kubernetes does not allow an old CRD version to be removed
while it remains in `status.storedVersions`, and changing the storage marker
does not rewrite existing objects automatically.

The approved procedure must cover this sequence and its OLM ordering:

1. Make v3 served and the storage version while a conversion path still exists
   for every version that may be present in storage.
2. Rewrite the singleton DSC through the new storage version, or use another
   migration mechanism supported by all target OpenShift versions.
3. Verify that no DSC object remains encoded at v1 and remove v1 from the CRD's
   `status.storedVersions` using an idempotent, retry-safe mechanism.
4. Stop serving v1, remove it from `spec.versions`, unregister its Go scheme and
   webhooks, and delete the v1 conversion implementation.
5. Prove fresh install, supported upgrade, interrupted migration retry, and
   operator rollback behavior.

If the CRD manifest is applied before the new operator can perform the required
migration, v1 cannot be removed from that manifest in the same rollout. The
retirement spike must select and test a staging strategy rather than assuming a
single apply is safe.

### 7. Regenerate delivered artifacts

- Update ODH and RHOAI CRD bases and conversion patches so v3 is the only
  storage version, v2 remains served, and v1 matches the approved retirement
  phase.
- Update scheme/project metadata, RBAC where required, webhook manifests,
  bundles/CSVs, API documentation, deepcopy code, and generated object code.
- Replace the primary ODH and RHOAI DSC samples with v3 examples. Retain focused
  v2 compatibility fixtures only where tests or upgrade documentation require
  them.
- Update documentation and release-note inputs for removed public fields and
  the supported v2-to-v3 mappings. Track required `odh-cli` work outside this
  repository.

## Test and acceptance matrix

### Unit and envtest

- Exact spec, status, and condition mapping for every changed component.
- Full round-trip preservation for v2 and v3, including v2 read-modify-write of
  v3 storage.
- Defaulting and validation through both served versions.
- Conversion and admission behavior for absent fields, empty management state,
  all supported state combinations, and conflicting legacy MaaS fields.
- Handler projection and status propagation for Dashboard, AI Hub, Feature
  Store, and Data Registry.
- Generated CRD assertions: exactly one storage version, expected served
  versions, conversion webhook configured, and no premature v1 removal.
- A production-import guard demonstrating that v2 remains only in approved
  compatibility packages and v1 is absent after retirement.

### Upgrade and end-to-end

- Upgrade RHOAI 3.5 DSC v2 storage to RHOAI 3.6 DSC v3 storage on both ODH and
  RHOAI variants.
- Cover legacy Dashboard, Model Registry, Feast, and KServe MaaS configurations
  and verify the same effective component lifecycle after upgrade.
- Read and update the same stored object through v2 and v3, verifying v3-only
  state is preserved.
- Verify Dashboard, AI Hub, Data, Feature Store, and Data Registry readiness,
  status fields, condition names, and independent removal behavior.
- Verify storage migration and the final `status.storedVersions` value.
- Exercise interrupted/retried migration and document or test the supported
  rollback boundary.
- Verify a fresh 3.6 install exposes v2 and v3, stores v3, and does not expose
  v1 after the retirement phase is complete.

## Delivery order

1. Finalize RHOAIENG-94805, RHOAIENG-95339, and RHOAIENG-95340.
2. Consolidate and approve RHOAIENG-94809; complete the RHOAIENG-94814
   retirement design in parallel.
3. Implement v3 types, conversion, admission, and the internal v3 migration.
4. Integrate RHOAIENG-95342, RHOAIENG-95344, and RHOAIENG-95346.
5. Execute the approved v1 retirement sequence and regenerate all artifacts.
6. Run `make generate manifests api-docs`, `make fmt`, `make lint`, relevant
   unit/envtest suites, and the ODH/RHOAI upgrade E2E matrix.
7. Complete RHOAIENG-94812 only after every acceptance criterion and generated
   diff is reviewed.
