# Platform CR as Low-Level Inventory and Operator Entry Point

## Context

The module reconciler currently operates in two modes: **DSC mode** (OpenShift — reads DSC/DSCI)
and **Platform mode** (xKS — reads Platform CR). This creates dual code paths in every module
handler. Additionally, the module reconciler both installs module operators AND creates module
CRs, mixing two concerns.

The goal: **Platform CR becomes the single entry point for module operator installation**. On
OpenShift, DSC and DSCI controllers project module enablement into Platform CR via SSA. On xKS,
users write Platform CR directly. The module reconciler always reads Platform CR — one code path,
one concern (operator lifecycle).

Module CRs (Monitoring CR, AIGateway CR) remain the responsibility of DSC/DSCI controllers. The
module reconciler never touches them.

## Planned Architecture Update

**Status:** design target only, **not yet implemented**. The remainder of this document still
contains branch-state sections that describe the currently implemented struct-based API and
controller split. Treat this section as the follow-up target architecture and the later sections
as current-state documentation unless explicitly updated.

The remainder of this document describes the current branch implementation and the original
transition plan. The agreed next-step architecture is slightly different and should be treated as
the target design for follow-up work:

- `Platform` becomes the low-level desired inventory for all tracked entries.
- Higher-level controllers such as DSC, DSCI, or GitOps write low-level intent into
  `Platform.spec.modules`.
- `Platform` never creates operand CRs. DSC and DSCI remain responsible for creating and
  configuring component/module/service CRs.
- `PlatformModule` exists once per declared `Platform.spec.modules[]` entry. It deploys resources
  only for module-backed operators and acts as tracker-only for internal-controller-backed
  entries.
- `Platform.status.modules` is a compact summary and must not mirror full
  `PlatformModule.status.conditions`.

### Target Platform API

`Platform.spec.modules` should move from a struct to a `+listType=map` / `+listMapKey=name` list.
Each entry should contain:

- `name`
- `managementState` with default `Removed`
- optional opaque `config,omitempty` for low-level/direct consumers only

`Platform.status.modules` should mirror the list shape and contain:

- `name`
- `runlevel`
- `version`
- `status.ready`
- `status.reason`
- `status.message`

Entries should be reported in deterministic order: runlevel, then name.

This is a **breaking API change** relative to the current struct-based `PlatformModules` shape
described later in this document. If adopted, it needs an explicit migration plan (or a version
bump) rather than being treated as a transparent follow-up refactor.

### Projection Rules

Projection from DSC and DSCI into `Platform.spec.modules` should stay intentionally narrow and
reflection-friendly:

- use `module:"..."` only as canonical name mapping
- project only `{name, managementState}`
- do not project per-entry config from DSC or DSCI

This keeps the tag as a naming aid, not a behavioral switch.

### PlatformModule Modes

`PlatformModule` should support two internal behaviors:

- **deployer**: render/apply manifests, inject images/config, and track owned operator resources
- **trackerOnly**: do not create resources; read the underlying CR/object status and compute
  readiness/version only

The selection mechanism for these behaviors is **registry-driven**. The current
`PlatformModuleSpec` remains empty; the reconciler should decide deployer vs tracker-only mode
from whether the entry is module-backed or not, using internal registry metadata keyed by entry
name rather than public API fields.

In tracker-only mode the reconciler still runs and still reads the relevant CR to compute
status, but it must not render/apply resources or create the platform config ConfigMap.

If the tracked CR for a tracker-only entry is missing, the reconciler cannot determine readiness,
so the entry should be reported as not ready with an explicit reason/message rather than treated as
deployed or healthy.

Tracker-only entries should **not** use `runlevelGateAction` in the same way deployer-backed
entries do. They still reconcile, but they exist to read CR status and report readiness/version,
not to gate manifest deployment.

A practical implementation approach is to keep one `PlatformModule` reconciler pipeline and wrap
deployer-only actions in a small conditional action/helper. That allows the reconciler to invoke
render/deploy/drift-cleanup steps only for module-backed entries while still running the
status-reading/tracking steps for tracker-only entries.

`Platform` itself should not care about whether a tracked entry is backed by a module-style
operator or an internal component controller. `PlatformModule` is the adapter that normalizes
status reporting and presents one consistent readiness/version contract back to `Platform`.

### ModuleHandler Simplification

As `Platform` becomes the low-level desired inventory, `ModuleHandler` should be simplified to
low-level tracking/deployer concerns only. It should no longer own:

- `IsEnabled`
- `BuildModuleCR`
- `DeleteModuleCR`
- `ApplyManagementState`

Those responsibilities move to `Platform` and the higher-level DSC/DSCI projection code.

### Release Reporting Contract

To keep version tracking consistent across component-backed and module-backed entries:

- tracked component CRs should publish `status.releases[]` with an entry named `"platform"`
- tracked module CRs should continue using the same convention
- `PlatformModule` should use that entry as the canonical platform upgrade/version signal

That `name = "platform"` release entry is internal tracker/controller data and should **not**
surface through DSC aggregated release reporting, because Dashboard consumes DSC status and would
otherwise receive an internal release row that does not represent a user-facing component release.

This contract applies to **all Platform-tracked entries/CRs**, not just a subset.

If `status.releases[name="platform"]` is missing, that should be handled consistently with the
module-backed path today: version reporting remains incomplete, but the behavior should match the
existing module semantics rather than introducing a special component-only rule.

### Unknown Entries

If `Platform.spec.modules[]` contains an entry name that does not exist in the internal registry,
that is treated as invalid desired state:

- `Platform` should become Degraded and `Ready=False`
- the user (or higher-level controller) must correct the entry
- `Platform` should still delete tracker instances for entries removed from `Platform.spec.modules`
- existing tracker CRs whose names are unknown to the registry should otherwise be left alone

### Decisions Made

The following target-architecture decisions are now fixed for the implementation:

1. **Tracker-only selection is registry-driven**
   - `PlatformModuleSpec` stays empty
   - deployer vs tracker-only is resolved from whether the entry is module-backed or not
2. **`Platform` should read low-level readiness from `PlatformModule`**
   - `PlatformModule` reads the underlying CR/object and computes readiness/version
   - `Platform` aggregates `PlatformModule` summaries rather than reading raw component CRs as the
     steady-state design
3. **Tracker-only entries do not require deploy-style `runlevelGateAction`**
   - they reconcile to report status, not to unblock manifest deployment
4. **List migration happens now in `v1alpha1`**
   - this work is treated as a POC and the breaking change is acceptable
5. **xKS still has an internal controller for non-module component entries**
   - for tracker-only entries on xKS, the tracker reads the CR created for that internal
     controller-backed component
6. **`status.releases[name="platform"]` is required consistently**
   - all Platform-tracked entries/CRs should publish it
   - DSC aggregated output must still filter it out
7. **Unknown entry names are invalid desired state**
   - unknown names degrade `Platform`
   - removed names still lead to tracker cleanup
   - unknown existing tracker CRs are otherwise left untouched

## Implementation Status

| Symbol | Meaning |
|--------|---------|
| ✅ | Implemented and merged to branch |
| 🚧 | Partially implemented |
| ❌ | Not yet implemented |

## Architecture

```
OpenShift:
  User → DSC  → syncPlatformModules (SSA) → Platform CR
             → provisionModuleCRs         → AIGateway CR (direct)
             → provisionComponents        → Component CRs (Dashboard, KServe, …)

  User → DSCI → syncPlatformServices       (SSA) → Platform CR
              → provisionServiceModuleCRs        → Monitoring CR (direct, DSCI-owned)
              → computeServiceModulesStatus      → DSCI status conditions

  Platform CR → Platform Controller → creates/deletes PlatformModule CRs
                                   → walks unified DAG (WalkBatches)
                                   → checks readiness of component CRs + PlatformModule CRs

  PlatformModule CRs → PlatformModule Reconciler → installs module operators
                                                 → status.resources drift cleanup

xKS:
  User → Platform CR → Platform Controller → creates PlatformModule CRs
       → creates module CRs manually
  PlatformModule CRs → PlatformModule Reconciler → installs module operators
```

### Responsibility Split

| Concern | Owner |
|---------|-------|
| Module operator install/remove | Platform controller (creates PlatformModule CRs) + PlatformModule reconciler (deploys) |
| Module CR create/configure/delete | DSC controller (AIGateway) or DSCI controller (Monitoring) |
| Module CR status tracking | DSC controller (`ComputeModulesStatus`) or DSCI controller (`computeServiceModulesStatus`) |
| DAG orchestration | Platform controller — sole orchestrator for components and modules |
| Cleanup (operator resources) | Owner references on PlatformModule CR + `status.resources` drift cleanup |
| Cleanup (module CR operands) | Module operator (via its own finalizers/GC) |
| Cleanup (component CRs) | DSC controller (`cleanupDisabledComponents`) |
| Cleanup (module operand CRs) | DSC controller (`cleanupDisabledModules`) or DSCI controller (`cleanupDisabledServiceModules`) |

## Changes

### 1. ✅ Current branch: keep PlatformModules struct, add module fields

**File:** `api/config/v1alpha1/platform_types.go`

The existing `PlatformModules` struct is kept — the Platform CR is already deployed and changing
to a list would be a breaking API change. New `ManagementSpec` fields are added per module.

```go
type PlatformModules struct {
    Monitoring common.ManagementSpec `json:"monitoring,omitempty"`
    AIGateway  common.ManagementSpec `json:"aigateway,omitempty"`
    // Add new module fields here as modules are onboarded.
}
```

SSA field managers own individual struct fields:
- DSCI controller (field manager `dsci-controller`) owns `.spec.modules.monitoring`
- DSC controller (field manager `dsc-controller`) owns `.spec.modules.aigateway`
- On xKS, kubectl/user owns all fields

`EnabledModules()` includes AIGateway.

> **Current branch note:** This reflects the branch as implemented today.
> The target design described above instead moves to a list-based low-level inventory API and
> should be treated as a future/breaking change until it is actually implemented.

### 2. ✅ Current branch: introduce PlatformModule tracker CRD

**New file:** `api/config/v1alpha1/platformmodule_types.go`

One CR per installed module operator. Cluster-scoped. Created/owned by Platform controller.

**Naming convention:** The PlatformModule CR `metadata.name` IS the module name:
- Handler `GetName()` → `"aigateway"`
- `PlatformModules.AIGateway` field → ManagementState
- PlatformModule CR → `metadata.name: "aigateway"`

```go
// Current branch implementation: PlatformModuleSpec is intentionally empty.
// The target architecture above may require explicit mode metadata if
// tracker-only and deployer behaviors both need to coexist.
type PlatformModuleSpec struct {}

type PlatformModuleStatus struct {
    Resources  []ResourceRef      `json:"resources,omitempty"`
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}
```

Lifecycle:
- Platform controller creates PlatformModule CR when module is `Managed` in Platform.Spec.Modules
- PlatformModule reconciler deploys operator resources owned by the PlatformModule CR and records
  them in `status.resources`
- Per reconcile: compare rendered resources against `status.resources`, delete stale entries
  (drift cleanup for version changes), update the list
- Platform controller deletes PlatformModule CR when module is `Removed` → Kubernetes GC
  cascade-deletes all owned resources (no finalizer needed)

### 3. ❌ Simplify ModuleHandler interface

**File:** `internal/controller/modules/types.go`

**Not yet implemented.** ModuleHandler still has the full set of methods including `BuildModuleCR`,
`GetModuleStatus`, `GetModuleCRState`, `DeleteModuleCR`, `DeleteOperatorResources`, `IsEnabled`.

Target interface (after simplification):

```go
type ModuleHandler interface {
    GetName() string
    GetGroupVersionKind() schema.GroupVersionKind
    GetOperatorManifests(platform *PlatformContext) OperatorManifests
    GetRelatedImages() []string
}
```

`IsEnabled` will be removed (the Platform controller reads enabled state from Platform CR directly).
`BuildModuleCR`, `GetModuleStatus`, `GetModuleCRState`, `DeleteModuleCR`, `DeleteOperatorResources`
will be removed (DSC/DSCI own module CRs directly; PlatformModule reconciler owns cleanup via
owner-ref cascade).

Optional interfaces `ContainerNamer`, `ControllerImager`, `InitContainerNamer`, `DeploymentNamer`
remain.

### 4. ❌ Simplify PlatformContext

**File:** `internal/controller/modules/types.go`

**Not yet implemented.** PlatformContext still carries `DSC` and `DSCI` fields that module
handlers use for `IsEnabled` and `BuildModuleCR`.

Target struct (after simplification):

```go
type PlatformContext struct {
    ApplicationsNamespace string
    GatewayDomain         string
    Release               common.Release
    Platform              *configv1alpha1.Platform
    ChartsBasePath        string
    ManifestsBasePath     string
}
```

### 5. ✅ Current branch: Platform controller orchestrates PlatformModule CRs

**File:** `internal/controller/platform/platform_controller.go`

Single reconciler for Platform CR. Uses `Options` pattern (same as PlatformModule) with injectable
registries for testing.

Action chain:
1. For each `Managed` module in `Platform.Spec.Modules`: ensure PlatformModule CR exists
2. For each `Removed` module: delete PlatformModule CR
3. Walk unified DAG (`WalkBatches`) with `CompositeChecker` spanning component CRs and PlatformModule CRs
4. Aggregate PlatformModule CR conditions into Platform CR status

**Readiness checkers:**
- `componentReadinessChecker`: reads component CR via `cluster.GetSingleton` + unstructured Get
- `moduleReadinessChecker`: reads PlatformModule CR directly + version handshake

> **Target architecture note:** In the future design described above, `Platform` should aggregate
> tracker summaries only. The direct component readiness checker is part of the current branch
> implementation and would need to be retired or explicitly kept as transitional behavior.

**Watches:**
- `Owns(gvk.PlatformModule)` — requeue when PlatformModule status changes
- `WatchesGVK(componentGVK, ...)` with status-change predicate — requeue when component CR status changes
- `WatchesGVK(gvk.CustomResourceDefinition, ...)` — requeue when CRDs installed/removed
- Dynamic watch registration via `registry.ForEach` and `modReg.ForAll`

**DAG advancement:**
- `StuckTracker` (per-component timeout) detects runlevels blocked beyond policy limits
- `WalkBatches` marks timed-out entries and allows subsequent runlevels to proceed
- `ProvisioningProgress` condition on Platform CR written by `WalkBatches`

The Platform controller **does not deploy operator resources** — that remains the PlatformModule
reconciler's sole concern.

### 6. ✅ Current branch: PlatformModule reconciler deploys module operators

**New file:** `internal/controller/platformmodule/platformmodule_controller.go`

Watches PlatformModule CRs. Each PlatformModule CR triggers independent reconciliation:

Action chain:
1. `runlevelGateAction` — check `provision.DefaultRegistry()` runlevel status; block until cleared
2. Look up module handler by `metadata.name` in the registry
3. Call `handler.GetOperatorManifests()` → render Helm/Kustomize
4. Inject RELATED_IMAGE_* env vars and platform config
5. Deploy via SSA with ownerReferences pointing to PlatformModule CR
6. Drift cleanup: delete resources in `status.resources` no longer in the rendered set
7. Update `status.resources` with current rendered set
8. Check operator Deployment readiness → update conditions

**No GC action, no finalizer.** Owner references on deployed resources enable Kubernetes GC
cascade deletion when the PlatformModule CR is deleted.

> **Target architecture note:** The current branch only supports deployer behavior. The future
> tracker-only mode described above is not implemented and requires an explicit selection mechanism.

### 7. ✅ Current branch: DSC controller writes to Platform CR and creates module CRs

**Files:** `internal/controller/datasciencecluster/datasciencecluster_controller_actions.go`,
`datasciencecluster_controller.go`, `datasciencecluster_controller_options.go`

The DSC controller was refactored with injectable registries:

```go
type Options struct {
    ComponentRegistry *cr.Registry
    ModuleRegistry    *modules.Registry
    DeletePropagation metav1.DeletionPropagation
}
type Reconciler struct { Options }

func NewDataScienceClusterReconciler(ctx context.Context, mgr ctrl.Manager, opts ...Option) error
```

Action chain:
1. `initialize` — remove legacy finalizer
2. `checkPreConditions` — verify DSCI and DSC exist
3. `updateStatus` — aggregate component and module status (`computeComponentsStatus` + `ComputeModulesStatus`)
4. `provisionComponents` — create component CRs for enabled components (no ordering)
5. `provisionModuleCRs` — create module operand CRs (AIGateway, etc.) for enabled modules
6. `syncPlatformModules` — SSA-patch `Platform.Spec.Modules` from DSC spec
7. `deploy` — SSA-apply all resources collected in `rr.Resources`
8. `cleanupDisabledComponents` — delete component CRs for disabled components
9. `cleanupDisabledModules` — call `handler.DeleteModuleCR()` for disabled modules

`gc.NewAction()` and `WalkBatches` are removed from the DSC controller. The Platform controller
is the sole DAG orchestrator.

`updateStatus` calls:
- `computeComponentsStatus(ctx, rr, r.ComponentRegistry)` — injectable registry
- `modules.ComputeModulesStatus(ctx, rr, r.ModuleRegistry)` — injectable registry

### 8. ✅ Current branch: DSCI controller writes to Platform CR and manages service module CRs

**File:** `internal/controller/dscinitialization/dscinitialization_controller.go`

The DSCI controller now mirrors the DSC module flow structurally while staying on the
existing DSCI controller framework:

1. `syncPlatformServices` — iterates tagged service modules from `DSCI.Spec`, calls
   `ApplyManagementState`, and SSA-patches `Platform.Spec.Modules` with field manager
   `dscinitialization`
2. `provisionServiceModuleCRs` — creates DSCI-owned service module CRs for enabled
   tagged modules (starting with Monitoring)
3. `cleanupDisabledServiceModules` — deletes owned service module CRs for disabled
   tagged modules
4. `computeServiceModulesStatus` — reads module CR status through the module registry
   and writes DSCI conditions directly

The implementation intentionally keeps `syncPlatformServices` local to DSCI rather
than extracting a shared helper with DSC. The overlap is small and the controller
entry points still differ enough that a common projection helper was not worth
introducing in this pass.

The Monitoring CR remains owned by the DSCI instance in all cases. Tests for this work
assume monitoring is already available as a module-backed service and validate behavior
through injected module handlers and test CRDs rather than the real monitoring controller.

The monitoring module handler also preserves the existing DSCI-specific CR projection
behavior when building the Monitoring CR:
- copies `metrics`, `traces`, and `alerting` from `DSCI.Spec.Monitoring`
- strips disabled traces TLS blocks
- preserves the collector replica defaulting rule: `1` on single-node clusters,
  `2` on multi-node clusters, unless the user sets `collectorReplicas`

`GetMonitoringReadyCondition()` is retained as a compatibility wrapper for existing
monitoring status tests, but the main DSCI reconcile path now uses
`computeServiceModulesStatus`.

### 9. ✅ DSC controller: simplified, no DAG

The DSC controller no longer walks the unified DAG. `gc.NewAction()` and `StuckTracker` are gone.
Cleanup is explicit: `cleanupDisabledComponents` and `cleanupDisabledModules` run as named actions.
The Platform controller handles all DAG-ordered orchestration.

**Watches:** DSC watches component CRs (via dynamic `OwnsGVK` for each registered component GVK)
and module CRs (via `WatchesGVK` for Monitoring, `OwnsGVK` for others). CRD watch
(`WatchesGVK(gvk.CustomResourceDefinition, ...)`) triggers re-evaluation when module CRDs are
installed.

**Module ownership distinction:**
- Monitoring → `WatchesGVK` only (owned by DSCI, not DSC)
- All other modules → `OwnsGVK` (owned by DSC)

### 10. ✅ Platform CR creation

SSA creates Platform CR implicitly on first write. Both DSCI and DSC controllers use SSA
(create-or-update), so whichever reconciles first creates the CR. No explicit creation step needed.

## New Infrastructure (not in original design)

### ✅ BaseComponentHandler

**New file:** `internal/controller/components/registry/base.go`

A configurable `ComponentHandler` implementation with function fields for every interface method.
`Name` and `GVK` are plain struct fields; unset function fields fall back to safe defaults.

```go
type BaseComponentHandler struct {
    Name string
    GVK  schema.GroupVersionKind

    InitFn                   func(common.Platform, operatorconfig.OperatorSettings) error
    IsEnabledFn              func(*dscv2.DataScienceCluster) bool
    NewCRObjectFn            func(context.Context, client.Client, *dscv2.DataScienceCluster) (common.PlatformObject, error)
    NewComponentReconcilerFn func(context.Context, ctrl.Manager) error
    UpdateDSCStatusFn        func(context.Context, *types.ReconciliationRequest) (metav1.ConditionStatus, error)
}
```

Defaults: `Init`→nil (no-op), `IsEnabled`→false, `NewCRObject`→nil (no CR),
`NewComponentReconciler`→nil (no controller), `UpdateDSCStatus`→ConditionTrue.

Replaces ad-hoc test structs in tests that need a lightweight `ComponentHandler` without a full
per-component controller.

### ✅ ProvisionRegistry field on component and module registries

**Files:** `internal/controller/components/registry/registry.go`,
`internal/controller/modules/registry.go`

Both `Registry` types now carry:

```go
// ProvisionRegistry is the unified DAG registry used for cache invalidation.
// If nil, provision.DefaultRegistry() is used.
ProvisionRegistry *provision.UnifiedRegistry
```

`InvalidateCache`, `Enable`, `Disable` route through the field when set, else fall back to the
global `provision.DefaultRegistry()` singleton. This allows integration tests to inject isolated
provision registries and avoid polluting the global DAG state across parallel test runs.

### ✅ DSC controller integration tests

**New file:** `internal/controller/datasciencecluster/datasciencecluster_controller_test.go`

Five envtest integration tests covering the full DSC reconcile action chain. No component or
module controllers are registered — the DSC controller operates alone against a real Kubernetes
API server.

| Test | What it exercises |
|------|------------------|
| `TestDSCReconciler_ComponentCRsCreated` | `provisionComponents` + `deploy`: Dashboard CR appears after DSC creation |
| `TestDSCReconciler_ComponentStatusReportedToDSC` | `updateStatus` → `computeComponentsStatus`: `ComponentsReady=False` when one handler returns False |
| `TestDSCReconciler_ModuleCRsCreated` | `provisionModuleCRs` + `deploy`: TestModule CR appears after DSC creation |
| `TestDSCReconciler_ModuleStatusReportedToDSC` | `updateStatus` → `ComputeModulesStatus` via real k8s reads (`BaseHandler.GetModuleStatus`): `ModulesReady` transitions False→True after patching CR status |
| `TestDSCReconciler_PlatformCRSyncedWithEnabledModules` | `syncPlatformModules`: Platform CR `.spec.modules.aigateway.managementState` matches DSC spec |

Key design decisions:
- `BaseComponentHandler` (not testify mock) for component handlers — function fields control
  `UpdateDSCStatus` and `NewCRObject` return values
- `testModuleHandler` embeds `modules.BaseHandler` — `GetModuleStatus` and `GetModuleCRState`
  read directly from Kubernetes, no mock k8s reads
- Module status test uses `cli.Status().Update` to set Ready=True on the TestModule CR, then
  triggers a re-reconcile via annotation — proves the real k8s read path

## Files Status

| File | Status | Notes |
|------|--------|-------|
| `api/config/v1alpha1/platform_types.go` | ✅ Done | AIGateway field added, `EnabledModules()` updated |
| `api/config/v1alpha1/platformmodule_types.go` | ✅ Done | PlatformModule CRD |
| `internal/controller/components/registry/base.go` | ✅ Done | New — `BaseComponentHandler` |
| `internal/controller/components/registry/registry.go` | ✅ Done | `ProvisionRegistry` field |
| `internal/controller/modules/registry.go` | ✅ Done | `ProvisionRegistry` field |
| `internal/controller/modules/types.go` | 🚧 Partial | ModuleHandler still has BuildModuleCR/GetModuleStatus/etc.; PlatformContext still has DSC/DSCI fields |
| `internal/controller/modules/modules_controller_actions.go` | 🚧 Partial | `ComputeModulesStatus` accepts `*Registry`; provisionModules still present for platform mode |
| `internal/controller/modules/base.go` | 🚧 Partial | `BaseHandler` still has CR methods |
| `internal/controller/modules/monitoring/handler.go` | ❌ Pending | Still has `IsEnabled`/`BuildModuleCR` |
| `internal/controller/modules/aigateway/handler.go` | ❌ Pending | Still has `IsEnabled`/`BuildModuleCR` |
| `internal/controller/platform/platform_controller.go` | ✅ Done | `Options` pattern, DAG orchestrator, composite readiness checkers, CRD watch |
| `internal/controller/platformmodule/platformmodule_controller.go` | ✅ Done | Operator deployment, drift cleanup, runlevel gating |
| `internal/controller/datasciencecluster/datasciencecluster_controller.go` | ✅ Done | Injectable registries, dynamic GVK watches |
| `internal/controller/datasciencecluster/datasciencecluster_controller_actions.go` | ✅ Done | provisionModuleCRs, syncPlatformModules, cleanupDisabled*, no DAG |
| `internal/controller/datasciencecluster/datasciencecluster_controller_options.go` | ✅ Done | New — Options/WithXxx pattern |
| `internal/controller/datasciencecluster/datasciencecluster_controller_test.go` | ✅ Done | New — 5 envtest integration tests |
| `internal/controller/dscinitialization/dscinitialization_controller.go` | ✅ Done | DSCI now syncs Platform modules and manages service module CR lifecycle/status |
| `internal/controller/dscinitialization/dscinitialization_modules.go` | ✅ Done | New — DSCI module sync/provision/cleanup/status helpers |
| `internal/controller/dscinitialization/dscinitialization_module_test.go` | ✅ Done | New standalone `testing.T` integration tests for DSCI module flow |
| `tests/integration/platform/platform_dsci_test.go` | ✅ Done | New DSCI-driven Platform integration tests |
| `tests/integration/platform/platform_combined_test.go` | ✅ Done | New combined DSC + DSCI Platform integration tests |
| `pkg/utils/test/mocks/types.go` | ✅ Done | `MockModuleHandler`, `NewDefaultMock*` constructors |
| `cmd/main.go` | ❌ Pending | Still has DSC/Platform mode conditional for module reconciler |

## Remaining Work

1. **Simplify ModuleHandler** (change 3): remove `BuildModuleCR`, `GetModuleStatus`,
   `GetModuleCRState`, `DeleteModuleCR`, `DeleteOperatorResources`, `IsEnabled`. The DSC
   controller owns module CR lifecycle directly; cleanup is via owner-ref cascade.

2. **Simplify PlatformContext** (change 4): drop `DSC` and `DSCI` fields once no handler
   reads them for `IsEnabled`/`BuildModuleCR`.

3. **Migrate aigateway handler**: remove `IsEnabled` and `BuildModuleCR`; DSC controller
   creates AIGateway CR from `DSC.Spec.Components.AIGateway` directly.

4. **Add DSCI-focused tests**: add standalone DSCI controller tests and extend
   `tests/integration/platform/` with DSCI-driven and combined DSC+DSCI scenarios.

5. **Simplify cmd/main.go**: remove DSC/Platform mode conditional for the module reconciler
   once the module reconciler is removed or reduced to platform-mode only.

## DAG Metrics

The DAG walking system (`WalkBatches`) emits Prometheus metrics registered in
the controller-runtime metrics registry. These are production metrics, not
test-only constructs.

**File:** `pkg/controller/provision/gating_metrics.go`

### Per-runlevel metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `odh_dag_runlevel_status` | GaugeVec | `runlevel`, `status` | Info-style state indicator. `status` is one of `pending`, `processed`, `blocked`, `timed_out`. Active state = `1`, others = `0`. |
| `odh_dag_runlevel_duration_seconds` | GaugeVec | `runlevel` | Seconds spent in the current state. For `processed`: batch processing time. For `blocked`: time spent waiting (from `StuckTracker`). |

### Aggregate metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `odh_dag_runlevel_cleared` | Gauge | — | Highest runlevel order fully processed in the current walk. |
| `odh_dag_runlevel_blocked` | Gauge | — | Runlevel currently blocked on. `0` when not blocked. |
| `odh_dag_batches_processed_total` | Counter | — | Cumulative batches processed across all reconcile cycles. |
| `odh_dag_runlevel_timeout_total` | CounterVec | `runlevel` | Incremented each time a runlevel times out and is skipped. |

### Example PromQL

- All blocked runlevels: `odh_dag_runlevel_status{status="blocked"} == 1`
- Timed-out runlevels: `odh_dag_runlevel_status{status="timed_out"} == 1`
- Alert on stuck DAG: `odh_dag_runlevel_blocked > 0`
- Time stuck: `odh_dag_runlevel_duration_seconds{runlevel="31"}`

## Integration Test Plan

See [docs/platform-modules/tests/plan.md](tests/plan.md) for the full envtest
integration test plan covering Platform, PlatformModule, DSC, and DSCI controller
pipelines. Tests are in `tests/integration/platform/`.

## Verification

1. `make generate manifests api-docs` — regenerate after type changes
2. `make lint` — pass linter
3. Unit tests: `TestDSCReconciler_*` (5 envtest tests) confirm the full DSC action chain
4. Unit tests: `TestPlatformReconciler_*` confirm DAG gating and readiness propagation
5. Unit tests: `TestPlatformModuleReconciler_*` confirm operator deployment and drift cleanup
6. Unit tests: `TestDSCIReconciler_*` confirm the DSCI module sync/create/delete/status flow
7. Unit tests: `TestDSCIDriven_*` and `TestCombined_*` confirm DSCI-driven and combined Platform behavior
8. E2E (OpenShift): DSC+DSCI → Platform CR → module operators installed, module CRs created
9. E2E (xKS): user writes Platform CR → operators installed; user creates module CRs manually
10. Verify SSA ownership: `kubectl get platform default -o json | jq '.metadata.managedFields'`
11. Verify cleanup: remove module from Platform CR → PlatformModule CR deleted → operator resources gone
12. Verify DAG ordering: RL(20) modules deploy before RL(31); cross-type gating works