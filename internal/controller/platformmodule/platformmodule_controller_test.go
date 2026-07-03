//nolint:testpackage
package platformmodule

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/precondition"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	opmanager "github.com/opendatahub-io/opendatahub-operator/v2/pkg/manager"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/testf"
	"github.com/opendatahub-io/opendatahub-operator/v2/tests/envtestutil"

	. "github.com/onsi/gomega"
)

var testModuleGVK = schema.GroupVersionKind{
	Group:   "components.platform.opendatahub.io",
	Version: "v1alpha1",
	Kind:    "TestModule",
}

var testModule1GVK = schema.GroupVersionKind{
	Group:   "components.platform.opendatahub.io",
	Version: "v1alpha1",
	Kind:    "TestModule1",
}

var testModule2GVK = schema.GroupVersionKind{
	Group:   "components.platform.opendatahub.io",
	Version: "v1alpha1",
	Kind:    "TestModule2",
}

const testModuleCRName = "default-testmodule"

func startPlatformModuleControllerWith(t *testing.T, opts ...Option) (*envt.EnvT, *testf.WithT) {
	t.Helper()
	g := NewWithT(t)

	ctx := t.Context()

	viper.Set("rhai-applications-namespace", "default")
	t.Cleanup(func() { viper.Set("rhai-applications-namespace", "") })

	cluster.SetRelease(testRelease(2, 20, 0))
	t.Cleanup(func() { cluster.SetRelease(common.Release{}) })

	root, err := envtestutil.FindProjectRoot()
	g.Expect(err).NotTo(HaveOccurred())

	et, err := envt.New(
		envt.WithCRDPaths(
			filepath.Join(root, "config", "crd", "bases"),
		),
		envt.WithOpManagerOptions(
			opmanager.WithManifestsBasePath(
				filepath.Join(root, "internal", "controller", "platformmodule", "testdata", "manifests"),
			),
		),
		envt.WithManager(ctrl.Options{
			Controller: ctrlconfig.Controller{SkipNameValidation: ptr.To(true)},
		}),
		envt.WithRegisterControllers(func(mgr ctrl.Manager) error {
			// Background deletion avoids foregroundDeletion finalizer in envtest.
			return New(ctx, mgr, append([]Option{
				WithDeletePropagationPolicy(metav1.DeletePropagationBackground),
			}, opts...)...)
		}),
	)

	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { _ = et.Stop() })

	et.StartManager(t, ctx)

	tc, err := et.NewTestContext(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	wt := tc.NewWithT(t)

	gwCfg := &serviceApi.GatewayConfig{}
	gwCfg.SetName(serviceApi.GatewayConfigName)
	wt.Expect(wt.Client().Create(wt.Context(), gwCfg)).Should(Succeed())
	envt.CleanupDelete(t, NewWithT(t), context.Background(), wt.Client(), gwCfg)

	gwCfg.Status.Domain = "example.com"
	wt.Expect(wt.Client().Status().Update(wt.Context(), gwCfg)).Should(Succeed())

	return et, wt
}

func createPlatformModuleCR(t *testing.T, wt *testf.WithT, name string) {
	t.Helper()

	pm := &configv1alpha1.PlatformModule{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	wt.Expect(wt.Client().Create(wt.Context(), pm)).Should(Succeed())
	envt.CleanupDelete(t, NewWithT(t), context.Background(), wt.Client(), pm)
}

// createTestModuleCR creates an unstructured TestModule CR. Uses wt.Create()
// which retries via Eventually — the CRD may be deployed by the reconciler
// (not pre-loaded) and take a moment to become available.
func createTestModuleCR(t *testing.T, wt *testf.WithT, conditions ...metav1.Condition) {
	t.Helper()

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(testModuleGVK)
	u.SetName(testModuleCRName)

	// Retry: the CRD may be deployed by the reconciler and not yet available.
	NewWithT(t).Eventually(func() error {
		return wt.Client().Create(wt.Context(), u)
	}).Should(Succeed())
	envt.CleanupDelete(t, NewWithT(t), context.Background(), wt.Client(), u)

	if len(conditions) > 0 {
		now := metav1.Now().UTC().Format(time.RFC3339)
		conds := make([]any, 0, len(conditions))
		for _, c := range conditions {
			conds = append(conds, map[string]any{
				"type":               c.Type,
				"status":             string(c.Status),
				"reason":             c.Reason,
				"lastTransitionTime": now,
			})
		}
		_ = unstructured.SetNestedSlice(u.Object, conds, "status", "conditions")
		wt.Expect(wt.Client().Status().Update(wt.Context(), u)).Should(Succeed())
	}
}

func setTestModuleRelease(wt *testf.WithT, releaseVersion string) {
	cr := &unstructured.Unstructured{}
	cr.SetGroupVersionKind(testModuleGVK)
	wt.Expect(wt.Client().Get(wt.Context(), types.NamespacedName{Name: testModuleCRName}, cr)).Should(Succeed())
	_ = unstructured.SetNestedSlice(cr.Object, []any{
		map[string]any{"name": "platform", "version": releaseVersion},
	}, "status", "releases")
	wt.Expect(wt.Client().Status().Update(wt.Context(), cr)).Should(Succeed())
}

func TestPlatformModuleReconciler_UnknownHandler(t *testing.T) {
	_, wt := startPlatformModuleControllerWith(t)

	createPlatformModuleCR(t, wt, "unregistered-module")

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "unregistered-module"}).
		Eventually().Should(
		jq.Match(`[.status.conditions[] | select(.type == "%s")] | length > 0`,
			status.ConditionDeploymentsAvailable),
	)
}

// TestPlatformModuleReconciler_OperandAvailableWhenCRAbsent: handler registered but
// no module CR exists (fresh install — not yet created by DSC/DSCI).
// OperandAvailable=False+Info (non-blocking), Ready=True (Info conditions don't block).
func TestPlatformModuleReconciler_OperandAvailableWhenCRAbsent(t *testing.T) {
	h := newManifestHandler("testmodule", testModuleGVK, "testmodule")

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, WithRegistry(reg))
	createPlatformModuleCR(t, wt, "testmodule")

	nn := types.NamespacedName{Name: "testmodule"}

	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .severity == "Info"`,
			status.ConditionTypeOperandAvailable),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .reason == "OperandAbsent"`,
			status.ConditionTypeOperandAvailable),
		// Info severity doesn't block Ready.
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeReady, metav1.ConditionTrue),
	))
}

// TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRReady: module CR Ready=True.
// OperandAvailable=True (no severity), Ready=True.
func TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRReady(t *testing.T) {
	h := newManifestHandler("testmodule", testModuleGVK, "testmodule")

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, WithRegistry(reg))
	createPlatformModuleCR(t, wt, "testmodule")
	createTestModuleCR(t, wt, metav1.Condition{
		Type:   status.ConditionTypeReady,
		Status: metav1.ConditionTrue,
		Reason: "Ready",
	})

	nn := types.NamespacedName{Name: "testmodule"}

	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionTrue),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeReady, metav1.ConditionTrue),
	))
}

// TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRNotReady: module CR Ready=False.
// OperandAvailable=False (no Info — blocks DAG), Ready=False.
func TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRNotReady(t *testing.T) {
	h := newManifestHandler("testmodule", testModuleGVK, "testmodule")

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, WithRegistry(reg))
	createPlatformModuleCR(t, wt, "testmodule")
	createTestModuleCR(t, wt, metav1.Condition{
		Type:   status.ConditionTypeReady,
		Status: metav1.ConditionFalse,
		Reason: "Reconciling",
	})

	nn := types.NamespacedName{Name: "testmodule"}

	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .reason == "OperandNotReady"`,
			status.ConditionTypeOperandAvailable),
		// No Info severity — this is a real problem that blocks the DAG.
		jq.Match(`[.status.conditions[] | select(.type == "%s" and .severity == "Info")] | length == 0`,
			status.ConditionTypeOperandAvailable),
		// Ready must reflect the blocking condition.
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeReady, metav1.ConditionFalse),
	))
}

func TestPlatformModuleReconciler_ReleaseReflectsModuleCRVersion(t *testing.T) {
	h := newManifestHandler("testmodule", testModuleGVK, "testmodule")

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, WithRegistry(reg))
	createPlatformModuleCR(t, wt, "testmodule")

	nn := types.NamespacedName{Name: "testmodule"}

	createTestModuleCR(t, wt, metav1.Condition{
		Type:   status.ConditionTypeReady,
		Status: metav1.ConditionTrue,
		Reason: "Ready",
	})

	// Phase 1: module reports an OLDER version.
	setTestModuleRelease(wt, "2.19.0")
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionTrue),
		jq.Match(`.status.release.version == "2.19.0"`),
	))

	// Phase 2: module reports the CURRENT platform version.
	setTestModuleRelease(wt, "2.20.0")
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`.status.release.version == "2.20.0"`),
	)
}

func TestPlatformModuleReconciler_DriftCleanup(t *testing.T) {
	_, wt := startPlatformModuleControllerWith(t)

	createPlatformModuleCR(t, wt, "drift-test-module")

	nn := types.NamespacedName{Name: "drift-test-module"}

	// Wait for first reconcile before injecting stale state (needed to avoid
	// status update conflict with the reconciler's SSA write).
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`[.status.conditions[] | select(.type == "%s")] | length > 0`,
			status.ConditionDeploymentsAvailable),
	)

	pm := &configv1alpha1.PlatformModule{}
	wt.Expect(wt.Client().Get(wt.Context(), nn, pm)).Should(Succeed())
	pm.Status.Resources = []configv1alpha1.ResourceRef{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default", Name: "phantom"},
	}
	wt.Expect(wt.Client().Status().Update(wt.Context(), pm)).Should(Succeed())

	wt.Expect(wt.Client().Get(wt.Context(), nn, pm)).Should(Succeed())
	pm.Annotations = map[string]string{"trigger": "reconcile"}
	wt.Expect(wt.Client().Update(wt.Context(), pm)).Should(Succeed())

	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`[.status.resources[] | select(.kind == "Deployment")] | length == 0`),
	)
}

// TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRHasNoConditions: CR exists,
// no conditions. OperandAvailable=False (no Info — blocks DAG), Ready=False.
func TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRHasNoConditions(t *testing.T) {
	h := newManifestHandler("testmodule", testModuleGVK, "testmodule")

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, WithRegistry(reg))
	createPlatformModuleCR(t, wt, "testmodule")
	createTestModuleCR(t, wt) // no conditions

	nn := types.NamespacedName{Name: "testmodule"}

	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		// OperandAvailable
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .reason == "OperandInitializing"`,
			status.ConditionTypeOperandAvailable),
		jq.Match(`[.status.conditions[] | select(.type == "%s" and .severity == "Info")] | length == 0`,
			status.ConditionTypeOperandAvailable),
		// Blocks Ready.
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeReady, metav1.ConditionFalse),
	))
}

// TestPlatformModuleReconciler_DynamicWatchActivatesOnCRDCreation: CRD absent at startup,
// then installed dynamically. OperandAbsent+Info initially, then OperandAvailable=True
// after module CR with Ready=True is created.
func TestPlatformModuleReconciler_DynamicWatchActivatesOnCRDCreation(t *testing.T) {
	dynamicGVK := schema.GroupVersionKind{
		Group:   "components.platform.opendatahub.io",
		Version: "v1alpha1",
		Kind:    "DynamicTestModule",
	}
	dynamicCRName := "default-dynamictestmodule"

	h := newNoopHandlerWithGVK("dynamictestmodule", dynamicGVK)
	h.Config.CRName = dynamicCRName

	reg := modules.NewRegistry()
	reg.Add(&h)

	et, wt := startPlatformModuleControllerWith(t, WithRegistry(reg))

	createPlatformModuleCR(t, wt, "dynamictestmodule")
	nn := types.NamespacedName{Name: "dynamictestmodule"}

	// Step 1: CRD does not exist → OperandAbsent+Info, message says CRD not installed.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		// OperandAvailable
		jq.Match(`.status.conditions[] | select(.type == "%s") | .reason == "OperandAbsent"`,
			status.ConditionTypeOperandAvailable),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .severity == "Info"`,
			status.ConditionTypeOperandAvailable),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .message == "module CRD not installed"`,
			status.ConditionTypeOperandAvailable),
		// Ready
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeReady, metav1.ConditionTrue),
	))

	// Step 2: Install the CRD dynamically. After the CRD watch fires and the
	// reconciler re-evaluates, the state should remain OperandAbsent+Info
	// (CRD now exists but CR still doesn't).
	crd, err := et.RegisterCRD(wt.Context(), dynamicGVK,
		"dynamictestmodules", "dynamictestmodule",
		apiextensionsv1.ClusterScoped,
		envt.WithPermissiveSchema())

	wt.Expect(err).NotTo(HaveOccurred())
	envt.CleanupDelete(t, NewWithT(t), context.Background(), wt.Client(), crd)

	// Still OperandAbsent+Info — CRD installed but no CR yet. Message changes.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		// OperandAvailable
		jq.Match(`.status.conditions[] | select(.type == "%s") | .reason == "OperandAbsent"`,
			status.ConditionTypeOperandAvailable),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .severity == "Info"`,
			status.ConditionTypeOperandAvailable),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .message == "module CR not yet created"`,
			status.ConditionTypeOperandAvailable),
		// Ready
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeReady, metav1.ConditionTrue),
	))

	// Step 3: Create the module CR with Ready=True.
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(dynamicGVK)
	u.SetName(dynamicCRName)
	wt.Expect(wt.Client().Create(wt.Context(), u)).Should(Succeed())
	envt.CleanupDelete(t, NewWithT(t), context.Background(), wt.Client(), u)

	_ = unstructured.SetNestedSlice(u.Object, []any{
		map[string]any{
			"type":               status.ConditionTypeReady,
			"status":             string(metav1.ConditionTrue),
			"reason":             "Ready",
			"lastTransitionTime": metav1.Now().UTC().Format(time.RFC3339),
		},
	}, "status", "conditions")
	wt.Expect(wt.Client().Status().Update(wt.Context(), u)).Should(Succeed())

	// OperandAvailable=True, Ready=True — dynamic watch activated.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		// OperandAvailable
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionTrue),
		// Ready
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeReady, metav1.ConditionTrue),
	))
}

// TestPlatformModuleReconciler_RunlevelGate_BlocksDeployment: when a module's
// runlevel is not yet cleared, no resources are deployed and PlatformReady=False.
// Once the runlevel is cleared, resources appear and PlatformReady=True.
func TestPlatformModuleReconciler_RunlevelGate_BlocksDeployment(t *testing.T) {
	// Fully isolated: custom provision registry and tracker — no global state touched.
	provReg := provision.NewRegistry()
	provReg.Add("testmodule", provision.KindModule, dag.RL(31))

	tracker := provision.NewRunlevelTracker()
	tracker.MarkCleared("2.20.0", 20)

	h := newManifestHandler("testmodule", testModuleGVK, "testmodule")
	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, WithRegistry(reg), WithProvisionRegistry(provReg), WithTracker(tracker))
	createPlatformModuleCR(t, wt, "testmodule")

	nn := types.NamespacedName{Name: "testmodule"}

	operatorSvc := types.NamespacedName{Name: "testmodule-operator", Namespace: "default"}

	// Gate fires: PlatformReady=False, no resources recorded, operator Service absent.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "False"`,
			precondition.PlatformReadyConditionType),
		jq.Match(`.status.resources == null or (.status.resources | length == 0)`),
	))
	wt.Get(gvk.Service, operatorSvc).Eventually().Should(BeNil())

	// Advance tracker — gate should clear on next reconcile.
	tracker.MarkCleared("2.20.0", 31)

	pm := &configv1alpha1.PlatformModule{}
	wt.Expect(wt.Client().Get(wt.Context(), nn, pm)).Should(Succeed())
	pm.Annotations = map[string]string{"trigger": "reconcile"}
	wt.Expect(wt.Client().Update(wt.Context(), pm)).Should(Succeed())

	// Gate lifted: PlatformReady=True, operator Service deployed, tracked in status.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "True"`,
			precondition.PlatformReadyConditionType),
		jq.Match(`[.status.resources[] | select(.kind == "Service" and .name == "testmodule-operator")] | length > 0`),
	))
	wt.Get(gvk.Service, operatorSvc).Eventually().Should(Not(BeNil()))
}

// TestPlatformModuleReconciler_RunlevelGate_ProgressesByRunlevel: two modules at
// different runlevels deploy in order. Module-a (runlevel 20) deploys after 20 is
// cleared while module-b (runlevel 31) stays gated. Module-b deploys only after 31
// is cleared.
func TestPlatformModuleReconciler_RunlevelGate_ProgressesByRunlevel(t *testing.T) {
	// Fully isolated: custom provision registry and tracker — no global state touched.
	provReg := provision.NewRegistry()
	provReg.Add("testmodule1", provision.KindModule, dag.RL(20))
	provReg.Add("testmodule2", provision.KindModule, dag.RL(31))

	tracker := provision.NewRunlevelTracker()

	h1 := newNoopHandlerWithGVK("testmodule1", testModule1GVK)
	h2 := newNoopHandlerWithGVK("testmodule2", testModule2GVK)

	reg := modules.NewRegistry()
	reg.Add(&h1)
	reg.Add(&h2)

	_, wt := startPlatformModuleControllerWith(t, WithRegistry(reg), WithProvisionRegistry(provReg), WithTracker(tracker))
	createPlatformModuleCR(t, wt, "testmodule1")
	createPlatformModuleCR(t, wt, "testmodule2")

	nn1 := types.NamespacedName{Name: "testmodule1"}
	nn2 := types.NamespacedName{Name: "testmodule2"}

	// Phase 1: no runlevel cleared — both modules gated.
	wt.Get(gvk.PlatformModule, nn1).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "False"`,
			precondition.PlatformReadyConditionType),
	)
	wt.Get(gvk.PlatformModule, nn2).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "False"`,
			precondition.PlatformReadyConditionType),
	)

	// Phase 2: clear runlevel 20 — testmodule1 deploys, testmodule2 stays gated.
	tracker.MarkCleared("2.20.0", 20)

	pm1 := &configv1alpha1.PlatformModule{}
	wt.Expect(wt.Client().Get(wt.Context(), nn1, pm1)).Should(Succeed())
	pm1.Annotations = map[string]string{"trigger": "phase2"}
	wt.Expect(wt.Client().Update(wt.Context(), pm1)).Should(Succeed())

	pm2 := &configv1alpha1.PlatformModule{}
	wt.Expect(wt.Client().Get(wt.Context(), nn2, pm2)).Should(Succeed())
	pm2.Annotations = map[string]string{"trigger": "phase2"}
	wt.Expect(wt.Client().Update(wt.Context(), pm2)).Should(Succeed())

	wt.Get(gvk.PlatformModule, nn1).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "True"`,
			precondition.PlatformReadyConditionType),
		jq.Match(`.status.resources | length > 0`),
	))
	wt.Get(gvk.PlatformModule, nn2).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "False"`,
			precondition.PlatformReadyConditionType),
	)

	// Phase 3: clear runlevel 31 — testmodule2 deploys.
	tracker.MarkCleared("2.20.0", 31)

	wt.Expect(wt.Client().Get(wt.Context(), nn2, pm2)).Should(Succeed())
	pm2.Annotations = map[string]string{"trigger": "phase3"}
	wt.Expect(wt.Client().Update(wt.Context(), pm2)).Should(Succeed())

	wt.Get(gvk.PlatformModule, nn2).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "True"`,
			precondition.PlatformReadyConditionType),
	)
}
