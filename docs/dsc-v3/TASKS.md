# DataScienceCluster v3 operator tasks

This file tracks the operator-side prerequisites and deliveries required for
[RHOAIENG-94812](https://redhat.atlassian.net/browse/RHOAIENG-94812), under the
[RHOAIENG-94804 DSC v3 epic](https://redhat.atlassian.net/browse/RHOAIENG-94804).

It is a read-only snapshot of the relevant Jira issues, Jira comments, spike
document, and Google Docs comments as reviewed on **2026-09-17**. Do not edit
Jira from this file.

`Task status` is repository-local. It describes whether the dependency is ready
for the operator implementation; it is not the live Jira workflow status.

Legend:

- `✅` complete locally
- `🚧` in progress locally
- `📝` todo
- `⛔` blocked by another tracked task
- `⚠️` contract discrepancy or confirmation required
- `🚫` intentionally outside this tracker

## Status table

| Jira | Deliverable | Jira status | Task status | Required before |
| --- | --- | --- | --- | --- |
| `RHOAIENG-94805` | Final Dashboard v3 contract | New | `📝 Todo` | `RHOAIENG-94809`, `RHOAIENG-95342` |
| `RHOAIENG-95339` | Final Data/Feature Store/Data Registry contract | New | `⚠️ Todo; discrepancy open` | `RHOAIENG-94809`, `RHOAIENG-95346` |
| `RHOAIENG-95340` | Final AI Hub/Model Registry contract | New | `⚠️ Todo; discrepancy open` | `RHOAIENG-94809`, `RHOAIENG-95344` |
| `RHOAIENG-94809` | Consolidated public API and conversion matrix | New | `⛔ Blocked by component contracts` | `RHOAIENG-94812`, all component handlers |
| `RHOAIENG-94814` | Safe DSC v1 retirement design | New | `📝 Todo` | `RHOAIENG-94812` v1 removal |
| `RHOAIENG-95342` | Dashboard module handler and CRD | New | `⛔ Blocked by 94805 and 94809` | `RHOAIENG-94812` completion |
| `RHOAIENG-95344` | AI Hub module handler and CRD | New | `⛔ Blocked by 95340 and 94809` | `RHOAIENG-94812` completion |
| `RHOAIENG-95346` | Data module handler and CRD | New | `⛔ Blocked by 95339 and 94809` | `RHOAIENG-94812` completion |
| `RHOAIENG-94812` | Implement DSC v3 in the operator | New | `⛔ Blocked` | Epic completion and verification |

## Dependency order

```text
RHOAIENG-94805 ─┐
RHOAIENG-95339 ─┼─> RHOAIENG-94809 ─┬─> RHOAIENG-95342 ─┐
RHOAIENG-95340 ─┘                    ├─> RHOAIENG-95344 ─┼─> RHOAIENG-94812
                                    └─> RHOAIENG-95346 ─┘
RHOAIENG-94814 ──────────────────────────────────────────┘
```

The platform-side v3 package, mechanical internal-type migration, and test
inventory can be prepared while contracts are being finalized. Generated
schemas and changed-field conversion logic must wait for RHOAIENG-94809.

## Task entries

### `RHOAIENG-94805` Finalize Dashboard spec for DSC v3

- Jira status: `New`
- Task status: `Todo`
- Assignee: Andy Stoneberg
- Blocks: RHOAIENG-94809 and RHOAIENG-95342

Required output:

- One approved Dashboard v3 spec and status shape.
- Final JSON field names for the core Dashboard and MaaS portal submodule.
- V2-to-v3 spec, status, condition, default, and update mappings.
- Approval from Dashboard, Workbenches, and Platform owners.

Open choices recorded in Jira and the spike:

- Retain `dashboard.managementState` and add the portal beneath it; or replace
  the top-level state with independently managed Dashboard subcomponents.
- `openshiftAI` may be unsuitable when DSC is exposed outside OpenShift.
- `maasCustomerPortal` may be too narrow because the application also has an
  administrative scenario. The Google Docs comments suggested considering a
  name such as `modelsAsAServicePortal`, but did not approve it.

Jira comment alignment:

- The issue comment explicitly says public API finalization must precede the
  Dashboard handler and CRD update.
- Approval of DSC v3 in the original spike did not approve the provisional
  Dashboard field names.

Local interpretation:

- Do not add provisional Dashboard JSON tags to v3 or the module CRD.
- Once approved, add conversion cases that preserve the existing v2 Dashboard
  and `maasConsumerPortal` behavior in both round-trip directions.

### `RHOAIENG-95339` Finalize Feast/Data and Data Registry spec for DSC v3

- Jira status: `New`
- Task status: `Todo; discrepancy open`
- Assignee: Umberto Manganiello
- Blocks: RHOAIENG-94809 and RHOAIENG-95346

Proposed contract:

```yaml
# v2 compatibility shape
components:
  feastoperator:
    managementState: Managed
    dataRegistry:
      managementState: Managed
```

```yaml
# v3 shape
components:
  data:
    featureStore:
      managementState: Managed
    dataRegistry:
      managementState: Managed
```

Required output:

- Approval that `data` is a structural group with no top-level management
  state and that its two subcomponents are independent.
- Approval of the proposed v2 compatibility field.
- Complete spec, status, condition, absent-field, and management-state mapping.
- Resolution of every ambiguous or unsupported state combination.
- Approval from Data, Feast, Data Registry, and Platform owners.

Discrepancies to resolve:

- RHOAIENG-94809 currently describes a parent Data `Removed` state overriding
  its subcomponents, while this issue removes the parent state entirely.
- The spike document proposed stashing Data Registry state in a conversion
  annotation. This issue instead makes the state representable in v2.
- A Google Docs comment notes that Data Registry had not shipped in an earlier
  release, questioning the need for an annotation-based compatibility path.

Jira comment alignment:

- Comments state that this API contract must precede both RHOAIENG-94809 and
  the Data handler/CRD implementation.
- DCH and internal module CR/metadata renaming are explicitly deferred to 3.7.

Local interpretation:

- Prefer the explicit v2 compatibility field if the final contract approves
  it. Do not retain an annotation stash without an unrepresentable field.
- Treat Feature Store and Data Registry as independent lifecycle and status
  entries; do not implement parent gating unless the consolidated contract
  restores a parent state.

### `RHOAIENG-95340` Finalize AI Hub / Model Registry spec for DSC v3

- Jira status: `New`
- Task status: `Todo; discrepancy open`
- Assignee: Paul Boyd
- Blocks: RHOAIENG-94809 and RHOAIENG-95344

Proposed contract:

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

Required output:

- Approval of `aiHub` as the v3 JSON name.
- Validation, optionality, default, immutability, and update behavior for
  `registriesNamespace`, including the absent-v2-field case.
- Spec and status mapping plus the approved Model Registry-to-AI Hub condition
  rename.
- Proof that an existing v2 Model Registry configuration retains its effective
  behavior.
- Approval from AI Hub, Model Registry, and Platform owners.

Discrepancy to resolve:

- The epic and RHOAIENG-94809 currently say `hub`; this newer issue requires
  `aiHub` and explicitly excludes both `hub` and `modelregistry` from v3.

Jira comment alignment:

- Comments state that this public contract must precede both RHOAIENG-94809
  and the AI Hub handler/CRD implementation.
- Internal module CR and metadata naming remain unchanged for RHOAI 3.6.

Local interpretation:

- Use the final public v3 name only at the DSC boundary. Continue projecting
  into the existing internal AI Hub/Model Registry resource until the separate
  3.7 migration.

### `RHOAIENG-94809` Finalize the public API and conversion contract

- Jira status: `New`
- Task status: `Blocked by RHOAIENG-94805, RHOAIENG-95339, and RHOAIENG-95340`
- Assignee: unassigned
- Blocks: RHOAIENG-94812, RHOAIENG-95342, RHOAIENG-95344, and RHOAIENG-95346

Required output:

- A single authoritative v3 component schema.
- Field-by-field v2-to-v3 and v3-to-v2 spec matrix.
- Status field and condition-type matrix.
- Defaults for absent and empty management states.
- Collision/precedence rules for legacy KServe MaaS and canonical AI Gateway
  MaaS values.
- Explicit behavior for v2 `trainingoperator` and `llamastackoperator` values
  that have no v3 field.
- A fidelity mechanism for every v3 field during a v2 read-modify-write.
- Approval from Platform, Dashboard, Data, Feast, Data Registry, AI Hub, Model
  Registry, and AI Gateway owners.

Jira comment alignment:

- Three comments confirm that the consolidated contract precedes each of the
  Dashboard, AI Hub, and Data implementation tasks.

Local interpretation:

- This issue is the contract authority for implementation. It must be updated
  to resolve the `hub`/`aiHub`, Data parent-state, Data Registry preservation,
  Dashboard naming, removed-field, and MaaS precedence questions before code
  generation begins.

### `RHOAIENG-94814` Identify the scope of DSC v1 retirement

- Jira status: `New`
- Task status: `Todo`
- Assignee: unassigned
- Blocks: the v1-removal portion of RHOAIENG-94812

Required output:

- Inventory of v1 scheme registration, conversion, admissions, tests,
  generated schema, samples, and runtime consumers.
- The observed CRD and OLM ordering for upgrades from every supported source
  release.
- An idempotent method to rewrite existing DSC storage and remove v1 from
  `status.storedVersions` before removing it from `spec.versions`.
- Retry, partial-failure, fresh-install, and rollback behavior.
- A testable decision on whether retirement requires more than one delivered
  phase.

Kubernetes constraint:

- An old CRD version cannot be removed while it remains in
  `status.storedVersions`. Merely changing `storage: true` to v3 does not rewrite
  existing objects.

Local interpretation:

- Do not delete the v1 CRD schema or conversion code in the same change that
  first makes v3 storage unless the spike demonstrates that the previous
  release already removed every v1 stored version or provides another safe
  ordering.

### `RHOAIENG-95342` Update Dashboard module handler and CRD for DSC v3

- Jira status: `New`
- Task status: `Blocked by RHOAIENG-94805 and RHOAIENG-94809`
- Assignee: Andy Stoneberg
- Blocks: RHOAIENG-94812 completion

Checks required in the repository:

- Module CRD matches the approved Dashboard v3 contract.
- Handler projects the final v3 structure without changing existing v2
  behavior.
- Core Dashboard and portal status/conditions propagate correctly.
- Conversion, handler, schema-compliance, and generated-artifact tests pass.

Left out:

- Internal module CR and metadata renaming, which is deferred to 3.7.

### `RHOAIENG-95344` Update AI Hub module handler and CRD for DSC v3

- Jira status: `New`
- Task status: `Blocked by RHOAIENG-95340 and RHOAIENG-94809`
- Assignee: Paul Boyd
- Blocks: RHOAIENG-94812 completion

Checks required in the repository:

- Public `aiHub` state and `registriesNamespace` project into the existing
  internal module resource.
- Existing v2 Model Registry behavior is unchanged.
- Status, condition, conversion, handler, CRD-schema, and generated-artifact
  tests pass.

Left out:

- Internal CR/GVK and module metadata convergence, tracked for 3.7 by
  RHOAIENG-95349.

### `RHOAIENG-95346` Update Data module handler and CRD for DSC v3

- Jira status: `New`
- Task status: `Blocked by RHOAIENG-95339 and RHOAIENG-94809`
- Assignee: Umberto Manganiello
- Blocks: RHOAIENG-94812 completion

Checks required in the repository:

- Handler and module CRD support Feature Store and Data Registry independently.
- Existing v2 Feast behavior is unchanged.
- Data, Feature Store, and Data Registry status and conditions propagate
  according to the final matrix.
- Conversion, handler, CRD-schema, and generated-artifact tests pass.

Left out:

- DCH integration and internal Data/Feast CR or metadata renaming.
- The deferred internal rename is tracked for 3.7 by RHOAIENG-95350.

### `RHOAIENG-94812` Implement DSC v3 API

- Jira status: `New`
- Task status: `Blocked`
- Assignee: unassigned

Blocked by:

- RHOAIENG-94809 for the authoritative public and conversion contract.
- RHOAIENG-94814 for safe v1 retirement.
- RHOAIENG-95342, RHOAIENG-95344, and RHOAIENG-95346 for the required module
  handler and CRD changes.

Operator checklist:

- Add the v3 API package and make it the conversion hub/storage version.
- Turn v2 into the sole served spoke and retain v2 admissions.
- Implement complete, lossless v2-to-v3 spec/status/condition conversion.
- Migrate production controllers, component registries, modules, status
  handling, utilities, generators, and tests to typed v3 objects.
- Add v3 admissions.
- Integrate the Dashboard, AI Hub, and Data handler deliveries.
- Execute the approved v1 storage migration and remove v1 serving, scheme,
  webhooks, and conversion support.
- Regenerate CRDs, bundles, API docs, samples, deepcopy code, and webhook
  manifests for both ODH and RHOAI.
- Add unit, round-trip, webhook, handler, generated-schema, and upgrade E2E
  coverage.
- Run the mandatory generation, formatting, linting, unit, and relevant E2E
  quality gates.

Completion condition:

- V2 clients preserve all v3 state, existing v2 configurations retain their
  effective behavior, v3 is the only storage/internal version, v1 is safely
  retired, all required component handlers are integrated, and all mandatory
  gates pass.

## Intentionally outside this tracker

- `RHOAIENG-94813`: downstream upgrade, compatibility, and rollback
  verification after the operator implementation is available. The operator
  test cases it depends on remain part of RHOAIENG-94812's completion criteria.
- `RHOAIENG-95349`: internal AI Hub CR/CRD and metadata rename, deferred to
  RHOAI 3.7.
- `RHOAIENG-95350`: internal Data/Feast CR/CRD and metadata rename, deferred to
  RHOAI 3.7.
- DCH integration, deferred beyond the RHOAI 3.6 DSC v3 contract.
- Release notes, product documentation, and `odh-cli` changes owned outside the
  operator repository, although the operator contract must provide their source
  mapping.
