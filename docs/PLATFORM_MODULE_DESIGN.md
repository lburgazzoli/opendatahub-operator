# Platform CR as Sole Module Entry Point

## Context

The module reconciler currently operates in two modes: **DSC mode** (OpenShift — reads DSC/DSCI) and **Platform mode** (xKS — reads Platform CR). This creates dual code paths in every module handler. Additionally, the module reconciler both installs module operators AND creates module CRs, mixing two concerns.

The goal: **Platform CR becomes the single entry point for module operator installation**. On OpenShift, DSC and DSCI controllers project module enablement into Platform CR via SSA. On xKS, users write Platform CR directly. The module reconciler always reads Platform CR — one code path, one concern (operator lifecycle).

Module CRs (Monitoring CR, AIGateway CR) remain the responsibility of DSC/DSCI controllers. The module reconciler never touches them.

## Architecture

```
OpenShift:
  User → DSC/DSCI → SSA writes → Platform CR → Platform Controller → creates PlatformModule CRs
                  → creates module CRs directly (Monitoring, AIGateway)
  PlatformModule CRs → PlatformModule Reconciler → installs module operators

xKS:
  User → Platform CR → Platform Controller → creates PlatformModule CRs
       → creates module CRs manually
  PlatformModule CRs → PlatformModule Reconciler → installs module operators
```

### Responsibility Split

| Concern | Owner |
|---------|-------|
| Module operator install/remove | Platform controller (reads Platform CR) + PlatformModule reconciler (deploys operators) |
| Module CR create/configure/delete | DSC controller (DSC-owned) or DSCI controller (DSCI-owned) |
| Module CR status tracking | DSC/DSCI controllers |
| DAG orchestration | Platform controller (sole DAG orchestrator for both components and modules) |
| Cleanup (operator resources) | Owner references on PlatformModule CR + drift cleanup via `status.resources` |
| Cleanup (module CR operands) | Module operator (via its own finalizers/GC) |

## Changes

### 1. Keep PlatformModules struct, add module fields

**File:** `api/config/v1alpha1/platform_types.go`

Keep the existing `PlatformModules` struct — the Platform CR is already deployed and changing to a list would be a breaking API change. Add new `ManagementSpec` fields per module as they're onboarded.

```go
type PlatformModules struct {
    Monitoring common.ManagementSpec `json:"monitoring,omitempty"`
    AIGateway  common.ManagementSpec `json:"aigateway,omitempty"`
    // Add new module fields here as modules are onboarded.
}
```

SSA field managers own individual struct fields:
- DSCI controller (field manager `dsci`) owns `.spec.modules.monitoring`
- DSC controller (field manager `dsc`) owns `.spec.modules.aigateway`
- On xKS, kubectl/user owns all fields

Update `EnabledModules()` to include AIGateway. Consider migrating to a list-based representation in a future v1alpha2 when the module count grows (all components become modules).

> **Future migration note:** When in-tree components migrate to modules (16+ fields), switch `PlatformModules` to a `+listType=map` list with `+listMapKey=name`. This is a breaking change acceptable in v1alpha1 but better deferred to a planned API version bump.

### 2. Introduce PlatformModule tracker CRD

**New file:** `api/config/v1alpha1/platformmodule_types.go`

One CR per installed module operator. Cluster-scoped. Created/owned by Platform controller.

**Naming convention:** The PlatformModule CR `metadata.name` IS the module name. This creates a 1:1 mapping:
- Handler `GetName()` → `"aigateway"`
- `PlatformModules.AIGateway` field → ManagementState
- PlatformModule CR → `metadata.name: "aigateway"`
- No spec field needed — the name carries the identity.

```go
// PlatformModuleSpec is intentionally empty. The CR name (metadata.name)
// IS the module name — it matches the handler's GetName() and the
// PlatformModules struct field. No spec fields needed.
type PlatformModuleSpec struct {}

type PlatformModuleStatus struct {
    // Resources tracked for per-reconcile drift cleanup.
    // +optional
    Resources []ResourceRef `json:"resources,omitempty"`
    // Standard conditions (e.g., Ready, Degraded).
    Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ResourceRef struct {
    schema.GroupVersionKind `json:",inline"`
    Namespace string `json:"namespace,omitempty"`
    Name      string `json:"name"`
}
```

Lifecycle:
- Platform controller creates PlatformModule CR (named after module) when module is `Managed` in Platform.Spec.Modules
- PlatformModule reconciler deploys operator resources with ownerReferences pointing to the PlatformModule CR, and records them in `status.resources`
- Per reconcile: compare rendered resources against `status.resources` — delete anything tracked that's no longer rendered (drift cleanup for version changes), then update the list
- Platform controller deletes PlatformModule CR when module is `Removed` → Kubernetes GC cascade-deletes all owned resources automatically (no finalizer needed)

### 3. Simplify ModuleHandler interface

**File:** `internal/controller/modules/types.go`

Remove module CR concerns. The handler only knows about operator deployment:

```go
type ModuleHandler interface {
    GetName() string
    GetOperatorManifests(platform *PlatformContext) OperatorManifests
    GetRelatedImages() []string
}
```

Remove: `IsEnabled()` (registry reads from Platform CR), `BuildModuleCR()`, `GetGVK()`, `GetModuleStatus()`, `GetModuleCRState()`, `DeleteModuleCR()`, `DeleteOperatorResources()` (replaced by owner-ref cascade + drift cleanup via PlatformModule CR).

Keep optional interfaces: `ContainerNamer`, `ControllerImager`, `InitContainerNamer`, `DeploymentNamer`.

### 4. Simplify PlatformContext

**File:** `internal/controller/modules/types.go`

Remove DSC/DSCI fields. The module reconciler never reads them.

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

### 5. Platform reconciler: orchestrates PlatformModule CRs

**File:** `internal/controller/modules/modules_controller.go`

Remove DSC-mode vs Platform-mode split. Single reconciler for Platform CR. Its sole job is creating/deleting PlatformModule CRs based on `Platform.Spec.Modules` and orchestrating the DAG.

Action chain:
1. Read `Platform.Spec.Modules` fields
2. For each `Managed` module: ensure PlatformModule CR exists (create if absent)
3. For each `Removed` module: delete PlatformModule CR
4. Walk DAG runlevels: clear runlevel in `RunlevelTracker` when all entries at that level are Ready
5. Aggregate PlatformModule CR conditions into Platform CR status

**DAG advancement:** Shared `UnifiedRegistry` + `RunlevelTracker` pattern (same as today):

- **Shared DAG:** Components and modules are in the same `UnifiedRegistry` (`pkg/controller/provision/unified.go`). Both controllers resolve the same unified batches so cross-type ordering works (a module at RL(33) waits for components at RL(31)).
- **Platform controller** is the sole DAG orchestrator. It calls `WalkBatches` on unified batches, checks readiness of both component CRs and PlatformModule CRs via `CompositeChecker`, and clears runlevels for everything.
- **DSC controller** no longer walks the DAG. It creates/deletes component CRs and module CRs without ordering.
- **PlatformModule reconciler** includes `RunlevelGateAction` as its first action — checks `tracker.IsCleared()`, sets `SkipDeploy` if runlevel not reached, requeues after configurable interval (default 30s).
- **Watches:** Platform controller watches PlatformModule CRs (automatic via `Owns()`) and component CRs (explicit `Watches()` with status-change predicate) for DAG advancement. It does NOT watch module CRs (Monitoring, AIGateway) — those are DSC/DSCI's domain.
- **Triggering:** Event-driven (component/PlatformModule status changes) + `WalkBatches` requeue when blocked on unready entries + configurable periodic resync (default 5min) as a safety net.

The Platform controller **does not deploy operator resources** — that's the PlatformModule reconciler's job.

Remove from current module reconciler: `initializeModules`, `cleanupDisabledModules`, `provisionModules`, `BuildModuleCR` calls, `updateModuleStatus`, `gc.NewAction()`, all Helm/Kustomize render actions, `deploy.NewAction()`, `injectModuleEnv`, `injectPlatformConfig`.

### 6. PlatformModule reconciler: deploys module operators

**New file:** `internal/controller/modules/platformmodule_controller.go`

Watches PlatformModule CRs. Each PlatformModule CR triggers independent reconciliation:

Action chain:
1. `RunlevelGateAction` — check `RunlevelTracker`, set `SkipDeploy` if runlevel not cleared
2. Look up module handler by `spec.moduleName` in the registry
3. Call `handler.GetOperatorManifests()` → render Helm/Kustomize
4. Inject RELATED_IMAGE_* env vars (`handler.GetRelatedImages()`)
5. Inject platform config ConfigMap
6. Deploy via SSA with ownerReferences pointing to PlatformModule CR
7. Drift cleanup: compare rendered resources against `status.resources`, delete stale entries
8. Update `status.resources` with current set
9. Check operator Deployment readiness → update conditions

**No GC action, no finalizer.** Owner references on deployed resources enable Kubernetes GC cascade deletion when the PlatformModule CR is deleted. `status.resources` handles per-reconcile drift cleanup (resource removed between versions).

This mirrors the component controller pattern: Platform creates PlatformModule CRs (like DSC creates component CRs), and the PlatformModule reconciler handles deployment (like component controllers handle deployment).

### 7. DSC controller: write to Platform CR + create module CRs

**File:** `internal/controller/datasciencecluster/datasciencecluster_controller_actions.go`

Add action (early in chain) that SSA-patches Platform CR:
- Write `Platform.Spec.Modules.AIGateway.ManagementState` from `DSC.Spec.Components.AIGateway.ManagementState`
- Field manager: `dsc-controller`

Move `BuildModuleCR()` logic for AIGateway into DSC controller actions — DSC creates the AIGateway module CR directly (like it creates component CRs today).

### 8. DSCI controller: write to Platform CR

**File:** `internal/controller/dscinitialization/dscinitialization_controller.go`

Add action that SSA-patches Platform CR:
- Write `Platform.Spec.Modules.Monitoring.ManagementState` from `DSCI.Spec.Monitoring.ManagementState`
- Field manager: `dsci-controller`

DSCI already creates the Monitoring CR directly — keep that path.

### 9. Simplify DSC: CR-based lifecycle, no DAG

**File:** `internal/controller/datasciencecluster/datasciencecluster_controller_actions.go`

Replace `gc.NewAction()` and `WalkBatches` with a simple CR lifecycle loop. DSC no longer walks the DAG — the Platform controller is the sole DAG orchestrator.

DSC action chain simplifies to:
1. For each `Managed` component: ensure component CR exists (create if absent, no ordering)
2. For each `Removed` component: delete component CR → component controller handles operand cleanup
3. SSA-write module enablement to Platform CR
4. For each DSC-owned `Managed` module: ensure module CR exists
5. For each DSC-owned `Removed` module: delete module CR + write `Removed` to Platform CR
6. Aggregate component/module status into DSC conditions

No `WalkBatches`, no `StuckTracker`, no `gc.NewAction()`. Each CR owner handles its own resource lifecycle. Component controllers self-gate via `RunlevelGateAction` using the `RunlevelTracker` cleared by the Platform controller.

**Watch Platform CR:** DSC and DSCI controllers `Watches()` the Platform CR with a status-change predicate. When module operators become Ready (reflected in Platform CR status via PlatformModule conditions), DSC/DSCI know it's safe to create module CRs (CRD is installed). This replaces blind retry on `IsNoMatchError` with informed, event-driven module CR creation.

**Safety net:** DSC/DSCI controllers include configurable periodic resync (same as Platform controller) to guard against missed events, accidental CR deletion, or failed SSA writes to Platform CR. On each resync: verify all expected CRs exist, re-apply SSA writes if needed, re-aggregate status.

**CRD-not-ready handling:** When creating a module CR fails with `IsNoMatchError` (CRD not yet installed), check the corresponding PlatformModule CR status:
- PlatformModule CR **not Ready** (operator still deploying) → expected, requeue after configurable delay (e.g., 10s)
- PlatformModule CR **Ready** but CRD missing → operator is broken, report error condition on DSC/DSCI, do not blindly retry

This distinction prevents masking a real operator failure as a transient "CRD not yet installed" state.

**StuckTracker:** The `StuckTracker` (timeout detection for runlevels blocked beyond policy limits) stays in the Platform controller's `WalkBatches` call. When a runlevel is stuck past its timeout, `WalkBatches` marks timed-out entries and allows subsequent runlevels to proceed — same behavior as today, just centralized in the Platform controller instead of split between DSC and module reconciler.

### 10. Platform CR creation

SSA creates Platform CR implicitly on first write. No explicit creation step needed. Both DSCI and DSC controllers use SSA (create-or-update), so whichever reconciles first creates the CR.

## Files to Modify

| File | Change |
|------|--------|
| `api/config/v1alpha1/platform_types.go` | Add AIGateway field to `PlatformModules`, update `EnabledModules()` |
| `api/config/v1alpha1/platformmodule_types.go` | **New** — PlatformModule CRD |
| `internal/controller/modules/types.go` | Simplify ModuleHandler, PlatformContext |
| `internal/controller/modules/modules_controller.go` | Remove mode split, single Platform reconciler (creates/deletes PlatformModule CRs, DAG orchestration) |
| `internal/controller/modules/platformmodule_controller.go` | **New** — PlatformModule reconciler (deploys operators per CR) |
| `internal/controller/modules/modules_controller_actions.go` | Simplify to PlatformModule CR creation/deletion + status aggregation |
| `internal/controller/modules/base.go` | Simplify BaseHandler (remove CR methods) |
| `internal/controller/modules/registry.go` | Read enabled state from Platform CR fields |
| `internal/controller/modules/monitoring/handler.go` | Remove IsEnabled/BuildModuleCR |
| `internal/controller/modules/aigateway/handler.go` | Remove IsEnabled/BuildModuleCR |
| `internal/controller/datasciencecluster/datasciencecluster_controller_actions.go` | Add SSA write to Platform CR, add AIGateway CR creation, replace GC/DAG with CR lifecycle loop |
| `internal/controller/dscinitialization/dscinitialization_controller.go` | Add SSA write to Platform CR |
| `cmd/main.go` | Remove DSC/Platform mode conditional for module reconciler |

## Verification

1. `make generate manifests api-docs` — regenerate after type changes
2. `make lint` — pass linter
3. Unit tests: module handlers with simplified interface
4. Unit tests: SSA projection from DSC/DSCI to Platform CR
5. Unit tests: PlatformModule CR lifecycle (create/delete/owner-ref cascade)
6. E2E (OpenShift): DSC+DSCI → Platform CR → module operators installed, module CRs created by DSC/DSCI
7. E2E (xKS): user writes Platform CR → operators installed; user creates module CRs manually
8. Verify SSA ownership: `kubectl get platform default -o json | jq '.metadata.managedFields'`
9. Verify cleanup: remove module from Platform CR → PlatformModule CR deleted → operator resources gone
10. Verify DAG ordering: RL(20) modules deploy before RL(31); cross-type gating works
11. Verify CRD-not-ready handling: module CR creation retries with delay when operator not yet installed
