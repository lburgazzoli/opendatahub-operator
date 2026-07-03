# Platform Integration Tests Plan

## Goal

Create integration tests in `tests/integration/platform/` that exercise the full
Platform, PlatformModule, and DataScienceCluster controller pipelines end-to-end
using envtest. No real modules or components -- only test-only fixtures built from
the existing `BaseHandler`, `BaseComponentHandler`, and dynamically registered CRDs.

## Architecture

Three controllers run in a single envtest manager:

- **Platform controller** (`internal/controller/platform/`) -- reconciles Platform CR,
  creates/deletes PlatformModule CRs, walks the module DAG, aggregates status.
- **PlatformModule controller** (`internal/controller/platformmodule/`) -- reconciles
  PlatformModule CRs, deploys module operator manifests, checks operand CR health,
  enforces runlevel gating.
- **DSC controller** (`internal/controller/datasciencecluster/`) -- reconciles
  DataScienceCluster CR, creates component CRs, creates module operand CRs,
  SSA-patches Platform.Spec.Modules, aggregates status.

```
DSC Controller
  ├── provisionComponents → creates component CRs (test-only)
  ├── provisionModuleCRs  → creates module operand CRs (test-only)
  ├── syncPlatformModules → SSA-patches Platform.Spec.Modules
  └── updateStatus        → aggregates ComponentsReady + ModulesReady

Platform Controller
  ├── enableModules         → syncs registry from Platform.Spec.Modules
  ├── syncPlatformModuleCRs → creates PlatformModule CRs
  ├── walkModuleDAG         → clears runlevels when batches ready
  └── aggregateStatus       → ModulesReady from PlatformModule conditions

PlatformModule Controller
  ├── RunlevelGateAction  → blocks deploy until runlevel cleared
  ├── provision           → renders operator manifests
  ├── deploy              → SSA-applies resources
  └── syncModuleCRStatus  → reflects operand CR health into PlatformModule status
```

## Test Fixtures

### Test Modules

Use `modules.BaseHandler` with `ModuleConfig` (same pattern as
`internal/controller/platformmodule/platformmodule_controller_test.go`):

```go
type testModuleHandler struct {
    modules.BaseHandler
}

func (h *testModuleHandler) BuildModuleCR(...) (*unstructured.Unstructured, error) {
    u := &unstructured.Unstructured{}
    u.SetGroupVersionKind(h.Config.GVK)
    u.SetName(h.Config.CRName)
    return u, nil
}

func (h *testModuleHandler) IsEnabled(_ *modules.PlatformContext) bool { return true }
```

Module CRDs are **dynamic** -- registered at runtime via `et.RegisterCRD()` with
`envt.WithPermissiveSchema()` so status subresource is available.

### Test Components

Use `cr.BaseComponentHandler` (same pattern as
`internal/controller/datasciencecluster/datasciencecluster_controller_test.go`):

```go
compReg.Add(&cr.BaseComponentHandler{
    Name:        "testcomp",
    GVK:         testCompGVK,
    IsEnabledFn: func(_ *dscv2.DataScienceCluster) bool { return true },
    NewCRObjectFn: func(...) (common.PlatformObject, error) {
        return &componentApi.Dashboard{ObjectMeta: metav1.ObjectMeta{Name: "default-testcomp"}}, nil
    },
    UpdateDSCStatusFn: func(...) (metav1.ConditionStatus, error) {
        return metav1.ConditionTrue, nil
    },
})
```

Component CRDs are **static** -- loaded from `config/crd/bases` before the
controllers start (existing Dashboard/Kserve CRDs can be reused for test
component GVKs).

## Key References

| What | Where |
|------|-------|
| envtest helper | `pkg/utils/test/envt/envt.go` |
| test context / WithT | `pkg/utils/test/testf/testf.go` |
| jq matchers | `pkg/utils/test/matchers/jq/` |
| Platform controller + options | `internal/controller/platform/platform_controller.go`, `*_options.go` |
| PlatformModule controller + options | `internal/controller/platformmodule/platformmodule_controller.go`, `*_options.go` |
| DSC controller + options | `internal/controller/datasciencecluster/datasciencecluster_controller.go`, `*_options.go` |
| Platform types | `api/config/v1alpha1/platform_types.go` |
| PlatformModule types | `api/config/v1alpha1/platformmodule_types.go` |
| DSC v2 types | `api/datasciencecluster/v2/datasciencecluster_types.go` |
| Module handler interface | `internal/controller/modules/types.go` |
| Module registry | `internal/controller/modules/registry.go` |
| BaseHandler / ModuleConfig | `internal/controller/modules/base.go` |
| BaseComponentHandler | `internal/controller/components/registry/base.go` |
| Component registry | `internal/controller/components/registry/registry.go` |
| Provision / UnifiedRegistry | `pkg/controller/provision/unified.go` |
| DAG / Runlevel | `pkg/controller/dag/` |
| RunlevelTracker | `pkg/controller/provision/runlevel_tracker.go` |
| Status condition types | `internal/controller/status/status.go` |
| Precondition gate | `pkg/controller/precondition/runlevel_gate.go` |
| GVK constants | `pkg/cluster/gvk/` |
| Existing Platform DAG test | `internal/controller/platform/platform_controller_dag_test.go` |
| Existing DSC test | `internal/controller/datasciencecluster/datasciencecluster_controller_test.go` |
| Existing PlatformModule test | `internal/controller/platformmodule/platformmodule_controller_test.go` |
| Test module manifests | `internal/controller/platformmodule/testdata/manifests/testmodule/` |

## Conventions

- Use `go test` standard tests (`func TestXxx(t *testing.T)`), not Ginkgo.
- Use dot-import for Gomega: `import . "github.com/onsi/gomega"`.
- Use `jq.Match()` for status condition assertions on unstructured GVK lookups.
- Use `envt.CleanupDelete()` for reliable cleanup in `t.Cleanup`.
- Use `metav1.DeletePropagationBackground` -- envtest has no GC controller.
- Use `ctrlconfig.Controller{SkipNameValidation: ptr.To(true)}` in manager options.
- Set `cluster.SetRelease()` before starting the manager; reset in `t.Cleanup`.
- Set `viper.Set("rhai-applications-namespace", "default")` for PlatformModule.
- All registries (module, component, service, provision) must be isolated per-test.
- The `GatewayConfig` CR must be created because the PlatformModule controller
  registers a dynamic watch on GatewayConfig CRs (service registry watches).
  Without it, the controller starts fine but domain-dependent platform config
  keys will be empty.
- The global `provision.GetRunlevelTracker()` must be reset between tests to
  avoid cleared runlevels leaking across test cases. Call
  `provision.GetRunlevelTracker().Reset()` in setup and `t.Cleanup`.
- When using a custom `provisionReg`, set `moduleReg.ProvisionRegistry = provisionReg`
  so that `EnableFromList()` propagates enable/disable signals to the correct
  registry instead of the global default.

## Tasks

### Group 01: Shared Test Infrastructure (`suite_test.go`)

| # | Task | Status |
|---|------|--------|
| 01-1 | [Core Types, Constants, startAllControllers](task-01-1-core.md) | done |
| 01-2 | [Helper Functions](task-01-2-helpers.md) | done |

### Group 02: Platform-Only Scenario (`platform_only_test.go`)

| # | Task | Status |
|---|------|--------|
| 02-1 | [TestPlatformOnly_TwoModules_Created](task-02-1-two-modules-created.md) | done |
| 02-2 | [TestPlatformOnly_DAG_Advancement](task-02-2-dag-advancement.md) | done |
| 02-3 | [TestPlatformOnly_DAG_Gating_ComponentBlocksModule](task-02-3-dag-gating.md) | done |
| 02-4 | [TestPlatformOnly_DisableModule_Cleanup](task-02-4-disable-cleanup.md) | done |

### Group 03: DSC-Driven Scenario (`dsc_driven_test.go`)

| # | Task | Status |
|---|------|--------|
| 03-1 | [TestDSCDriven_PlatformReflectsDSC](task-03-1-platform-reflects-dsc.md) | done |
| 03-2 | [TestDSCDriven_ComponentsAndModules_Installed](task-03-2-components-modules-installed.md) | done |
| 03-3 | [TestDSCDriven_StatusAggregation](task-03-3-status-aggregation.md) | done |
| 03-4 | [TestDSCDriven_DAG_Advancement](task-03-4-dag-advancement.md) | done |
| 03-5 | [TestDSCDriven_DAG_Gating_ModuleBlocksModule](task-03-5-dag-gating.md) | done |
| 03-6 | [TestDSCDriven_DisableComponent_Cleanup](task-03-6-disable-cleanup.md) | pending |

### Dependencies

- **01-1** must be completed first (creates the file).
- **01-2** depends on 01-1 (adds helpers to the same file).
- **02-1** depends on 01-2 (creates `platform_only_test.go`).
- **02-2, 02-3, 02-4** each depend on 02-1 (adds tests to the same file) but are independent of each other.
- **03-1** depends on 01-2 (creates `dsc_driven_test.go`).
- **03-2 through 03-6** each depend on 03-1 (adds tests to the same file) but are independent of each other.
- Groups 02 and 03 are independent and can be executed in parallel.
