---
task: "01-1 - Core Types, Constants, and startAllControllers"
file: tests/integration/platform/suite_test.go
started:
completed:
---

# Task 01-1: Core Types, Constants, and startAllControllers

Create `tests/integration/platform/suite_test.go` (package `platform_test`) with
the core types, GVK constants, test module handler, and `startAllControllers`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

## File to Create

`tests/integration/platform/suite_test.go`

## Deliverables

### 1. GVK Constants

```go
var testModuleAGVK = schema.GroupVersionKind{
	Group:   "test.platform.opendatahub.io",
	Version: "v1alpha1",
	Kind:    "TestModuleA",
}

var testModuleBGVK = schema.GroupVersionKind{
	Group:   "test.platform.opendatahub.io",
	Version: "v1alpha1",
	Kind:    "TestModuleB",
}
```

### 2. Test Module Handler

Copy the `noopHandlerWithGVK` pattern from
`internal/controller/platformmodule/platformmodule_controller_actions_test.go:321-356`.

```go
type testModuleHandler struct {
	modules.BaseHandler
}

func (h *testModuleHandler) BuildModuleCR(
	_ context.Context,
	_ client.Client,
	_ *modules.PlatformContext,
) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(h.Config.GVK)
	u.SetName(h.Config.CRName)
	return u, nil
}

func (h *testModuleHandler) IsEnabled(_ *modules.PlatformContext) bool {
	return true
}

func newTestModuleHandler(name string, gvkVal schema.GroupVersionKind) *testModuleHandler {
	return &testModuleHandler{
		BaseHandler: modules.BaseHandler{
			Config: modules.ModuleConfig{
				Name:   name,
				GVK:    gvkVal,
				CRName: "default-" + name,
			},
		},
	}
}
```

### 3. `suiteOpts` and `startAllControllers`

```go
type suiteOpts struct {
	moduleReg    *modules.Registry
	componentReg *cr.Registry
	provisionReg *provision.UnifiedRegistry
}
```

`startAllControllers` creates an envtest with all three controllers and returns
the envtest handle (for `RegisterCRD`) and a `*testf.TestContext`.

Implementation steps:

1. `cluster.SetRelease(common.Release{Name: cluster.OpenDataHub})` + cleanup.
2. `viper.Set("rhai-applications-namespace", "default")` + cleanup.
3. `envtestutil.FindProjectRoot()` to get root.
4. `envt.New(...)` with:
   - `envt.WithCRDPaths(filepath.Join(root, "config", "crd", "bases"))` -- loads
     Platform, PlatformModule, DSC, DSCI, GatewayConfig, and all component CRDs.
   - `envt.WithManager(ctrl.Options{Controller: ctrlconfig.Controller{SkipNameValidation: ptr.To(true)}})`.
   - A single `envt.WithRegisterControllers(func(mgr ctrl.Manager) error {...})` that
     registers all three controllers using the registries from `suiteOpts`.
5. `t.Cleanup(func() { _ = et.Stop() })`.
6. `et.StartManager(t, ctx)`.
7. `et.NewTestContext(ctx)` to create `*testf.TestContext`.

Controller registration details (inside the callback):

**Before registering controllers**, add these steps:

1. Reset the global runlevel tracker to prevent cross-test leaks:
   ```go
   provision.GetRunlevelTracker().Reset()
   t.Cleanup(func() { provision.GetRunlevelTracker().Reset() })
   ```

2. Wire the module registry's provision registry to the custom one so that
   `EnableFromList()` propagates to the correct registry:
   ```go
   if opts.provisionReg != nil {
       opts.moduleReg.ProvisionRegistry = opts.provisionReg
   }
   ```

**Platform controller** -- call `platform.New(ctx, mgr, ...)` with these options:
- `platform.WithModuleRegistry(opts.moduleReg)`
- `platform.WithComponentRegistry(opts.componentReg)`
- `platform.WithServiceRegistry(&sr.Registry{})` -- empty, no real services
- `platform.WithProvisionRegistry(opts.provisionReg)`
- `platform.WithDeletePropagationPolicy(metav1.DeletePropagationBackground)`
- `platform.WithStuckTracker(dag.NewStuckTracker())`

**PlatformModule controller** -- call `platformmodule.New(ctx, mgr, ...)` with:
- `platformmodule.WithRegistry(opts.moduleReg)`
- `platformmodule.WithServiceRegistry(&sr.Registry{})` -- empty
- `platformmodule.WithProvisionRegistry(opts.provisionReg)`
- `platformmodule.WithTracker(provision.GetRunlevelTracker())` -- **must use the
  global tracker** because `walkModuleDAG` in the Platform controller writes to
  `provision.GetRunlevelTracker()` via `WalkBatches`, and the PlatformModule's
  `RunlevelGateAction` reads from its tracker to decide if deploy is gated.
- `platformmodule.WithDeletePropagationPolicy(metav1.DeletePropagationBackground)`

**DSC controller** -- call `datasciencecluster.NewDataScienceClusterReconciler(ctx, mgr, ...)` with:
- `datasciencecluster.WithComponentRegistry(opts.componentReg)`
- `datasciencecluster.WithModuleRegistry(opts.moduleReg)`
- `datasciencecluster.WithDeletePropagationPolicy(metav1.DeletePropagationBackground)`

Return `(*envt.EnvT, *testf.TestContext)`. The caller uses `*envt.EnvT` for
`RegisterCRD` (dynamic module CRDs) and `*testf.TestContext` for assertions.

## Verification

```bash
go build ./tests/integration/platform/...
```

Must compile with no errors. No tests to run yet.
