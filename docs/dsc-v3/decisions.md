# DSC v3 authoritative decisions

This file is the authoritative decision record for DSC v3 development. Agents
must read it before starting a task and must honor every `Accepted` decision.
If another DSC v3 document conflicts with this file, this file wins. Repository
and system-level safety instructions still take precedence.

## How to maintain this record

- Use immutable IDs in the form `DEC-NNN`.
- Allowed states are `Proposed`, `Accepted`, `Superseded`, and `Rejected`.
- Only `Accepted` decisions authorize implementation. A `Proposed` decision is
  a blocker where code behavior depends on it.
- Do not rewrite an accepted decision's history. Add a new decision that names
  and supersedes the old one.
- Record decisions as soon as they are made during development, before relying
  on them in code. Update the index and add the full entry.
- Every task must list the decision IDs it used in its Outcomes section.

## Decision index

| ID | State | Date | Decision |
| --- | --- | --- | --- |
| `DEC-001` | Accepted | 2026-09-18 | V3 is the hub, storage, and production-internal DSC version; v2 remains served. |
| `DEC-002` | Accepted | 2026-09-18 | The first v3 schema is wire-equivalent to v2 and conversion is identity-only. |
| `DEC-003` | Accepted | 2026-09-18 | Every future v3 data change includes bidirectional conversion and tests. |
| `DEC-004` | Accepted | 2026-09-18 | V2 and v3 use distinct GVK constants and admissions. |
| `DEC-005` | Accepted | 2026-09-18 | Conversion webhook registration moves to the v2 spoke. |
| `DEC-006` | Accepted | 2026-09-18 | CRD `conversionReviewVersions` contains Kubernetes protocol version `v1`, not DSC versions. |
| `DEC-007` | Accepted | 2026-09-18 | Conversion coverage is guarded by complete OpenAPI schema differences, not field names alone. |
| `DEC-008` | Accepted | 2026-09-18 | V1 is absent from the final DSC v3 code and generated API surface. |
| `DEC-009` | Accepted | 2026-09-18 | An idempotent odh-cli pre-upgrade gate migrates v1 storage to v2 and removes v1 from `status.storedVersions` before the v1-free CRD is installed. |
| `DEC-010` | Accepted | 2026-09-18 | Empty DSC management state has effective value `Removed`. |
| `DEC-011` | Accepted | 2026-09-18 | Internal AI Hub/Model Registry and Data/Feast resource identities are not renamed in 3.6. |
| `DEC-012` | Accepted | 2026-09-18 | Data Connection Hub is excluded from the 3.6 DSC v3 contract. |
| `DEC-013` | Accepted | 2026-09-18 | Task 001 removes v1; the odh-cli pre-upgrade gate is the required migration boundary. |
| `DEC-014` | Accepted | 2026-09-18 | V3-evolving public types are version-owned and v2 has an independent OpenAPI baseline. |
| `DEC-015` | Accepted | 2026-09-18 | V2 remains served but is marked deprecated with a migration warning directing clients to v3. |
| `DEC-016` | Accepted | 2026-09-18 | Task 001 excludes odh-cli work and may complete independently; future Task 010 gates upgrade release. |
| `DEC-017` | Accepted | 2026-09-18 | A no-DSC gate run must still remove and verify v1 in `status.storedVersions`; only v1 already absent is mutation-free success. |
| `DEC-018` | Proposed | 2026-09-18 | Name the concrete CI/release control that prevents promotion or installation of the v1-free upgrade before Task 010 passes. |
| `DEC-019` | Accepted | 2026-09-18 | Use the exact v2 CRD deprecation warning recorded below. |
| `DEC-020` | Proposed | 2026-09-18 | Accept the complete supported-source, artifact, cluster, permission, odh-cli, and CI execution matrix for Task 010. |
| `DEC-021` | Accepted | 2026-09-18 | The independent v2 OpenAPI baseline is immutable for v3-only changes. |

## Accepted decisions

### DEC-001: v3 is storage and internal; v2 remains served

V3 implements `conversion.Hub`, is the only CRD storage version, and is the
typed DSC version used by production controllers, registries, modules, status
writers, utilities, and generators. V2 remains served as the compatibility
spoke with version-specific admission behavior.

### DEC-002: initial v3 and v2 schemas are equivalent

The machinery task introduces v3 with the current v2 spec, status, JSON names,
markers, defaults, validations, and deprecated fields unchanged. Initial v2
<-> v3 conversion copies object metadata, spec, and status without renaming,
dropping, defaulting, validation, normalization, or cluster access. Only the
expected target API version/GVK may differ.

Unresolved component stanzas must not alter v3 until their contract decisions
are accepted and recorded here.

### DEC-003: every v3 data change owns conversion and tests

Any v3 spec, status, nested type, JSON tag, optionality, default, validation,
condition, addition, move, rename, or removal is a conversion change. The same
change must:

- define and implement or explicitly confirm `v2 -> v3` and `v3 -> v2`;
- preserve v3-only data through a v2 read-modify-write;
- add focused direct tests in both directions;
- add `v2 -> v3 -> v2` and `v3 -> v2 -> v3` tests; and
- cover status and condition mappings when applicable.

If explicit identity-copy code remains correct, the new data still requires a
conversion test.

### DEC-004: keep version-specific GVK and admission behavior

Use distinct `DataScienceClusterV2` and `DataScienceClusterV3` GVK constants.
Each admission handler validates against its own request GVK. Changing a global
DSC GVK to v3 must not cause retained v2 admission requests to fail.

### DEC-005: register conversion from the v2 spoke

After v3 becomes the hub, v2 is the surviving `conversion.Convertible` spoke.
Register the controller-runtime conversion webhook with the v2 DSC type before
deleting v1 webhook registration. Test the real `/convert` endpoint with
Kubernetes `ConversionReview` requests containing one and multiple objects in
both desired API-version directions.

### DEC-006: use the Kubernetes ConversionReview protocol version

`spec.conversion.webhook.conversionReviewVersions` describes the Kubernetes
`ConversionReview` protocol, not served DSC API versions. Generate and assert:

```yaml
conversionReviewVersions:
  - v1
```

Do not add `v2` or `v3` to this list.

### DEC-007: guard the complete v2/v3 schema contract

The conversion coverage guard compares normalized, generated v2 and v3 OpenAPI
schemas. It must detect differences in paths, types, requiredness, nullability,
defaults, enums, CEL validation, and list/map semantics. Keep an independent v2
schema baseline so changing a shared Go type cannot silently mutate both APIs.

The expected difference set is empty for the initial machinery task. Every
future difference must be explicit and paired with direct and round-trip
conversion cases. A field-name-only reflection comparison is insufficient.

### DEC-008: final v3 delivery removes v1

The final DSC v3 code and generated API surface do not contain the v1 DSC API,
v1 conversion, v1 admissions, v1 scheme registration, or a served v1 CRD
version. Remove v1-specific compatibility annotations only after proving they
have no remaining consumers.

### DEC-010: empty management state means Removed

The effective lifecycle value of an empty DSC component or subcomponent
`managementState` is `Removed`. Normalize only when behavior requires a concrete
state; conversion does not rewrite an absent wire value merely to normalize it.

### DEC-011: retain internal module resource identities in 3.6

Public DSC naming may change after component contracts are accepted, but the
internal AI Hub/Model Registry and Data/Feast CR names, GVKs, and module metadata
remain unchanged in RHOAI 3.6. Their internal renames are deferred work.

### DEC-012: exclude Data Connection Hub

Data Connection Hub is not part of the RHOAI 3.6 DSC v3 `data` stanza. Do not
add its types, conversion, handler projection, status, conditions, or generated
schema as part of these tasks.

### DEC-013: remove v1 in Task 001 behind the odh-cli gate

Task 001 introduces v3, makes it storage/internal, adds v2 <-> v3 identity
conversion, migrates production code, and removes v1 code, conversion,
admissions, scheme registration, tests, and generated serving. The new
operator does not retain or adapt v1.

The required migration boundary is the accepted odh-cli pre-upgrade gate in
DEC-009. It runs while the old v1/v2 operator and its conversion support are
still available, converts any v1-stored DSC to v2, and prevents installation of
the v1-free CRD until v1 is absent from `status.storedVersions`.

### DEC-014: isolate evolving v3 public types

V3 owns its top-level DSC API types. Any nested public stanza that diverges
from v2 must become v3-owned before it changes. Shared
`api/components/v1alpha1` or internal module types must not be edited as a
shortcut to change v3 because doing so can silently change v2 or internal CRDs.

Keep an independent normalized v2 OpenAPI baseline and compare it with generated
v3 OpenAPI as required by DEC-007.

### DEC-015: deprecate v2 while keeping it served

The generated v2 CRD version remains `served: true` and `storage: false`, with
`deprecated: true` and a stable `deprecationWarning` that directs clients to
`datasciencecluster.opendatahub.io/v3`; DEC-019 supplies its exact value. V2
admission and bidirectional v2/v3 conversion remain supported for the
deprecation window. Deprecation does not authorize removal; a later v2
retirement requires its own compatibility period, odh-cli storage gate,
accepted decision, and upgrade/rollback qualification.

### DEC-016: odh-cli qualification is separate future work

Task 001 creates and verifies the v1-free operator and generated artifacts but
does not implement, integrate, or qualify odh-cli. It may be marked complete
when its repository subtasks and tests pass. Fresh installation of those
artifacts does not require migration from v1.

DSC-V3-010 is a separate future task that starts from Task 001's completed
artifacts and the externally implemented odh-cli gate. It owns storage
migration, OLM ordering, failure/retry, `status.storedVersions`, and rollback
qualification. Until Task 010 completes, the v1-free artifacts must not be
promoted or installed as an upgrade over releases that may have v1 storage.

### DEC-017: no object does not imply no stored-version cleanup

This decision clarifies and supersedes the no-object shortcut in DEC-009. If no
DSC exists and v1 is present in CRD `status.storedVersions`, the gate skips the
object rewrite but must patch the CRD status subresource, read it back, and
verify v1 is absent before permitting CRD replacement. Mutation-free success is
allowed only when v1 is already absent from `status.storedVersions`.

### DEC-019: exact v2 deprecation warning

The v2 CRD version uses this exact `deprecationWarning` value:

```text
datasciencecluster.opendatahub.io/v2 DataScienceCluster is deprecated; use datasciencecluster.opendatahub.io/v3 DataScienceCluster
```

Tests compare the complete string, including capitalization and punctuation.

### DEC-021: protect the v2 OpenAPI baseline

A v3-only public API change must not update the independent normalized v2
OpenAPI baseline. The baseline may change only when an accepted decision
explicitly changes the v2 wire contract. Such a change must list every modified
v2 path separately, update v2 admission/conversion compatibility, and include
focused tests proving the v2 change. Regenerating both sides to make a v2/v3
difference disappear is prohibited.

### DEC-009: use an odh-cli gate before removing v1

- State: `Accepted`
- Applies to: future DSC-V3-010/RHOAIENG-94814 qualification and upgrade
  delivery; not Task 001 implementation

Kubernetes rejects removal of a CRD version while it remains in
`status.storedVersions`, and changing the storage marker does not rewrite stored
objects. Therefore the v1-free CRD must not be applied until the following
idempotent gate succeeds:

1. Before OLM applies the new CRD, odh-cli runs against the installed v1/v2 CRD
   while the old operator and its conversion webhook are healthy.
2. The gate reads the DSC CRD and its `status.storedVersions`. It succeeds
   without mutation only when v1 is already absent. DEC-017 defines the
   no-object case.
3. If v1 is recorded and the singleton DSC exists, the gate reads and updates
   that object through the served v2 API, preserving spec, status, and metadata,
   so the API server rewrites it in v2 storage. The gate verifies the rewrite;
   it must not infer success from the CRD storage marker alone.
4. Only after every stored DSC is verified as no longer requiring v1 does the
   gate patch the CRD status subresource to remove v1 from
   `status.storedVersions`, then read it back and verify v1 is absent.
5. Any unsupported source state, failed conversion, webhook unavailability,
   conflicting mutation, incomplete verification, or status update failure
   blocks the upgrade. Re-running the gate is safe after partial completion.
6. OLM may then install the new CRD, which serves deprecated v2 and v3, stores
   only v3, and contains no v1. The new operator registers `/convert` through
   the v2 spoke with `conversionReviewVersions: [v1]`.
7. After the new webhook is healthy, the upgrade flow reads and updates the DSC
   through v3 to force v3 storage, verifies v3 is present in
   `status.storedVersions`, and removes v2 from that status only after no object
   remains stored as v2. V2 remains served and deprecated.

The gate must explicitly report its last safe rollback point. Before the
post-upgrade v3 rewrite, rollback to a v1/v2 release is supported. After data is
stored as v3, rollback requires a tested v3 -> v2 down-migration before the old
CRD/operator is restored; otherwise rollback is unsupported and must be
blocked. Supported source releases and both ODH/RHOAI packaging paths must be
covered by qualification tests.

## Pending decisions

### DEC-018: enforce upgrade-promotion blocking

- State: `Proposed`
- Blocks: DSC-V3-010 completion and any upgrade promotion/install of the
  v1-free CRD; it does not block Task 001 implementation or fresh install

Release engineering must record the exact control that consumes Task 010's
result and prevents a failed or missing gate from publishing the upgrade bundle
or advancing OLM. Record its repository/system owner, configuration or workflow
identifier, required evidence, failure signal, retry behavior, and a test that
proves failure cannot be bypassed. A prose warning or task status is not an
enforcement mechanism. Until this is accepted, Task 001 may be developed and
marked implementation-complete on its feature branch, but it must not merge to
any branch whose automation publishes or syncs the v1-free upgrade artifact.

### DEC-020: freeze Task 010's supported execution matrix

- State: `Proposed`
- Blocks: starting DSC-V3-010

Record locally every supported ODH/RHOAI source release, source and target
bundle/catalog/image reference, supported cluster version, packaging entry
point, required permissions, odh-cli version/invocation, and CI/E2E job. Task
010 must not discover or choose this matrix from Jira while executing.
