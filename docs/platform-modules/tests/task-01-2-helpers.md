---
task: "01-2 - Helper Functions"
file: tests/integration/platform/suite_test.go
depends_on: task-01-1-core.md
started:
completed:
---

# Task 01-2: Helper Functions

Add helper functions to `tests/integration/platform/suite_test.go`.

Read `docs/platform-tests/plan.md` for architecture context and key references.

## File to Modify

`tests/integration/platform/suite_test.go` (created in task-01-1)

## Deliverables

### 1. `createDSCI`

Copy from `internal/controller/datasciencecluster/datasciencecluster_controller_test.go:91-103`.

```go
func createDSCI(t *testing.T, tc *testf.TestContext) {
	t.Helper()
	g := NewWithT(t)

	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec: dsciv2.DSCInitializationSpec{
			ApplicationsNamespace: "default",
		},
	}
	g.Expect(tc.Client().Create(context.Background(), dsci)).Should(Succeed())
	t.Cleanup(func() { _ = tc.Client().Delete(context.Background(), dsci) })
}
```

### 2. `createGatewayConfig`

Copy from `internal/controller/platformmodule/platformmodule_controller_test.go:104-111`.

```go
func createGatewayConfig(t *testing.T, tc *testf.TestContext) {
	t.Helper()
	g := NewWithT(t)

	gwCfg := &serviceApi.GatewayConfig{}
	gwCfg.SetName(serviceApi.GatewayConfigName)
	g.Expect(tc.Client().Create(context.Background(), gwCfg)).Should(Succeed())
	envt.CleanupDelete(t, g, context.Background(), tc.Client(), gwCfg)

	gwCfg.Status.Domain = "example.com"
	g.Expect(tc.Client().Status().Update(context.Background(), gwCfg)).Should(Succeed())
}
```

### 3. `createPlatform`

```go
func createPlatform(t *testing.T, tc *testf.TestContext, spec configv1alpha1.PlatformSpec) {
	t.Helper()
	g := NewWithT(t)

	p := &configv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: configv1alpha1.PlatformInstanceName},
		Spec:       spec,
	}
	g.Expect(tc.Client().Create(context.Background(), p)).Should(Succeed())
	envt.CleanupDelete(t, g, context.Background(), tc.Client(), p)
}
```

### 4. `createDSC`

Copy from `internal/controller/datasciencecluster/datasciencecluster_controller_test.go:106-117`.

```go
func createDSC(t *testing.T, tc *testf.TestContext, spec dscv2.DataScienceClusterSpec) {
	t.Helper()
	g := NewWithT(t)

	dsc := &dscv2.DataScienceCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsc"},
		Spec:       spec,
	}
	g.Expect(tc.Client().Create(context.Background(), dsc)).Should(Succeed())
	envt.CleanupDelete(t, g, context.Background(), tc.Client(), dsc)
}
```

### 5. `setPlatformModuleReady`

Copy from `internal/controller/platform/platform_controller_test.go:85-107`.

```go
func setPlatformModuleReady(t *testing.T, cli client.Client, name string, ready bool) {
	t.Helper()
	g := NewWithT(t)

	pm := &configv1alpha1.PlatformModule{}
	g.Eventually(func() error {
		return cli.Get(context.Background(), types.NamespacedName{Name: name}, pm)
	}).Should(Succeed())

	condStatus := metav1.ConditionFalse
	if ready {
		condStatus = metav1.ConditionTrue
	}

	pm.Status.Conditions = []common.Condition{{
		Type:               status.ConditionTypeReady,
		Status:             condStatus,
		Reason:             "Test",
		LastTransitionTime: metav1.Now(),
	}}
	g.Expect(cli.Status().Update(context.Background(), pm)).Should(Succeed())
}
```

### 6. `setUnstructuredReady`

Copy from `internal/controller/platform/platform_controller_dag_test.go:85-107`.

```go
func setUnstructuredReady(t *testing.T, cli client.Client, u *unstructured.Unstructured, ready bool) {
	t.Helper()
	g := NewWithT(t)

	g.Expect(cli.Get(context.Background(), client.ObjectKeyFromObject(u), u)).Should(Succeed())

	condStatus := string(metav1.ConditionFalse)
	if ready {
		condStatus = string(metav1.ConditionTrue)
	}

	_ = unstructured.SetNestedSlice(u.Object, []any{
		map[string]any{
			"type":               status.ConditionTypeReady,
			"status":             condStatus,
			"reason":             "Test",
			"lastTransitionTime": metav1.Now().UTC().Format("2006-01-02T15:04:05Z"),
		},
	}, "status", "conditions")

	g.Expect(cli.Status().Update(context.Background(), u)).Should(Succeed())
}
```

### 7. `registerModuleCRD`

Registers a dynamic CRD for a test module GVK.

```go
func registerModuleCRD(t *testing.T, et *envt.EnvT, gvkVal schema.GroupVersionKind) {
	t.Helper()
	g := NewWithT(t)

	plural := strings.ToLower(gvkVal.Kind) + "s"
	singular := strings.ToLower(gvkVal.Kind)

	crd, err := et.RegisterCRD(
		context.Background(),
		gvkVal,
		plural, singular,
		apiextensionsv1.ClusterScoped,
		envt.WithPermissiveSchema(),
	)
	g.Expect(err).NotTo(HaveOccurred())
	envt.CleanupDelete(t, g, context.Background(), et.Client(), crd)
}
```

## Verification

```bash
go build ./tests/integration/platform/...
```

Must compile with no errors. No tests to run yet.
