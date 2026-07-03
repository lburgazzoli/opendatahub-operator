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

const testModuleCRName = "default-testmodule"

func startPlatformModuleControllerWith(t *testing.T, reg *modules.Registry) (*envt.EnvT, *testf.WithT) {
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
			filepath.Join(root, "internal", "controller", "platformmodule", "testdata"),
		),
		envt.WithManager(ctrl.Options{
			Controller: ctrlconfig.Controller{SkipNameValidation: ptr.To(true)},
		}),
		envt.WithRegisterControllers(func(mgr ctrl.Manager) error {
			return New(ctx, mgr, reg)
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

func createTestModuleCR(t *testing.T, wt *testf.WithT, conditions ...metav1.Condition) {
	t.Helper()

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(testModuleGVK)
	u.SetName(testModuleCRName)
	wt.Expect(wt.Client().Create(wt.Context(), u)).Should(Succeed())
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
	_, wt := startPlatformModuleControllerWith(t, modules.NewRegistry())

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
	h := newNoopHandlerWithGVK("testmodule", testModuleGVK)

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, reg)
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
	h := newNoopHandlerWithGVK("testmodule", testModuleGVK)

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, reg)
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
	h := newNoopHandlerWithGVK("testmodule", testModuleGVK)

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, reg)
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
	h := newNoopHandlerWithGVK("testmodule", testModuleGVK)

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, reg)
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
	_, wt := startPlatformModuleControllerWith(t, modules.NewRegistry())

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
	h := newNoopHandlerWithGVK("testmodule", testModuleGVK)

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, reg)
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

	et, wt := startPlatformModuleControllerWith(t, reg)

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

	now := metav1.Now().UTC().Format(time.RFC3339)
	_ = unstructured.SetNestedSlice(u.Object, []any{
		map[string]any{
			"type":               status.ConditionTypeReady,
			"status":             string(metav1.ConditionTrue),
			"reason":             "Ready",
			"lastTransitionTime": now,
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
