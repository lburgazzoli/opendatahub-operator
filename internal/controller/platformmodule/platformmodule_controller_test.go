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

// testModuleGVK is the GVK of the minimal test-only CRD in testdata/.
// It includes status.conditions and status.releases so the release
// handshake can be fully exercised without schema pruning.
var testModuleGVK = schema.GroupVersionKind{
	Group:   "components.platform.opendatahub.io",
	Version: "v1alpha1",
	Kind:    "TestModule",
}

// testModuleCRName is the singleton name of the test module CR.
// Derived by newNoopHandlerWithGVK: "default-" + handler name.
const testModuleCRName = "default-testmodule"

// startPlatformModuleControllerWith starts an envtest environment with the
// PlatformModule reconciler using the provided registry. Using an isolated
// registry per test avoids shared global state and makes tests hermetic.
//
// Application namespace is resolved from the RHAI_APPLICATIONS_NAMESPACE env
// var so no DSCI is required. The testdata/ CRD is loaded alongside the default
// CRDs so tests can use the minimal TestModule CR with full schema support.
func startPlatformModuleControllerWith(t *testing.T, reg *modules.Registry) (*envt.EnvT, *testf.WithT) {
	t.Helper()
	g := NewWithT(t)

	ctx := t.Context()

	// ApplicationNamespace() reads from viper key "rhai-applications-namespace".
	// Set it directly so the reconciler doesn't need a DSCI instance.
	viper.Set("rhai-applications-namespace", "default")
	t.Cleanup(func() { viper.Set("rhai-applications-namespace", "") })

	// Inject a known release so tests can assert on status.release values.
	cluster.SetRelease(testRelease(2, 20, 0))
	t.Cleanup(func() { cluster.SetRelease(common.Release{}) })

	root, err := envtestutil.FindProjectRoot()
	g.Expect(err).NotTo(HaveOccurred())

	et, err := envt.New(
		// Load both the default project CRDs and the minimal test CRD from testdata/.
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

	// Create a GatewayConfig and set its status domain so GetGatewayDomain()
	// resolves cleanly. Status must be written via the status subresource.
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

// createTestModuleCR creates an unstructured TestModule CR using the test-only CRD
// in testdata/. Conditions are stored directly in the status as []any so the
// API server accepts them without type conversion errors.
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

// setTestModuleRelease patches the release version into the TestModule CR's
// status.releases slice. The test CRD schema includes this field so it is not pruned.
func setTestModuleRelease(wt *testf.WithT, releaseVersion string) {
	cr := &unstructured.Unstructured{}
	cr.SetGroupVersionKind(testModuleGVK)

	wt.Expect(wt.Client().Get(wt.Context(), types.NamespacedName{Name: testModuleCRName}, cr)).Should(Succeed())
	_ = unstructured.SetNestedSlice(cr.Object, []any{
		map[string]any{"name": "platform", "version": releaseVersion},
	}, "status", "releases")
	wt.Expect(wt.Client().Status().Update(wt.Context(), cr)).Should(Succeed())
}

// TestPlatformModuleReconciler_UnknownHandler verifies that a PlatformModule CR
// whose name has no registered handler reconciles without error. The provision
// action returns early, nothing is deployed, and the DeploymentsAvailable
// condition is eventually set.
func TestPlatformModuleReconciler_UnknownHandler(t *testing.T) {
	_, wt := startPlatformModuleControllerWith(t, modules.NewRegistry())

	createPlatformModuleCR(t, wt, "unregistered-module")

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "unregistered-module"}).
		Eventually().Should(
		jq.Match(`[.status.conditions[] | select(.type == "%s")] | length > 0`,
			status.ConditionDeploymentsAvailable),
	)
}

// TestPlatformModuleReconciler_OperandAvailableWhenCRAbsent verifies that when
// no module CR exists (fresh install — not yet created by DSC/DSCI), the
// PlatformModule reports OperandAvailable=False with Info severity.
func TestPlatformModuleReconciler_OperandAvailableWhenCRAbsent(t *testing.T) {
	_, wt := startPlatformModuleControllerWith(t, modules.NewRegistry())

	createPlatformModuleCR(t, wt, "operand-test-module")

	nn := types.NamespacedName{Name: "operand-test-module"}

	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		jq.Match(`[.status.conditions[] | select(.type == "%s")] | length > 0`,
			status.ConditionTypeOperandAvailable),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .severity == "Info"`,
			status.ConditionTypeOperandAvailable),
	))
}

// TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRReady verifies that when
// the module CR exists and reports Ready=True, OperandAvailable is set to True.
func TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRReady(t *testing.T) {
	h := newNoopHandlerWithGVK("testmodule", testModuleGVK)

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, reg)
	createPlatformModuleCR(t, wt, "testmodule")

	nn := types.NamespacedName{Name: "testmodule"}

	// Wait for first reconcile (CR absent initially → OperandAvailable=False+Info).
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`[.status.conditions[] | select(.type == "%s")] | length > 0`,
			status.ConditionTypeOperandAvailable),
	)

	// Create the module CR with Ready=True.
	createTestModuleCR(t, wt, metav1.Condition{
		Type:   status.ConditionTypeReady,
		Status: metav1.ConditionTrue,
		Reason: "Ready",
	})

	// OperandAvailable must become True now that the module CR is ready.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionTrue),
	)
}

// TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRNotReady verifies that
// when the module CR exists but Ready=False, OperandAvailable is False (no Info — real problem).
func TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRNotReady(t *testing.T) {
	h := newNoopHandlerWithGVK("testmodule", testModuleGVK)

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, reg)
	createPlatformModuleCR(t, wt, "testmodule")

	nn := types.NamespacedName{Name: "testmodule"}

	// Wait for first reconcile before creating module CR.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`[.status.conditions[] | select(.type == "%s")] | length > 0`,
			status.ConditionTypeOperandAvailable),
	)

	createTestModuleCR(t, wt, metav1.Condition{
		Type:   status.ConditionTypeReady,
		Status: metav1.ConditionFalse,
		Reason: "Reconciling",
	})

	// OperandAvailable must be False without Info severity (real problem, not just absent).
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionFalse),
		jq.Match(`[.status.conditions[] | select(.type == "%s" and .severity == "Info")] | length == 0`,
			status.ConditionTypeOperandAvailable),
	))
}

// TestPlatformModuleReconciler_ReleaseReflectsModuleCRVersion verifies that
// pm.Status.Release.Version always mirrors the version the module CR reports in
// status.releases[{name:"platform"}].version. This is what the DAG readiness
// checker uses to decide whether to advance past this module's runlevel:
//   - module reports "2.19.0" → DAG waits (doesn't match platform "2.20.0")
//   - module reports "2.20.0" → DAG advances (handshake complete)
//
// The test sets the module CR to an older version first, verifies the reflection,
// then updates to the current version and verifies again.
func TestPlatformModuleReconciler_ReleaseReflectsModuleCRVersion(t *testing.T) {
	h := newNoopHandlerWithGVK("testmodule", testModuleGVK)

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, reg)
	createPlatformModuleCR(t, wt, "testmodule")

	nn := types.NamespacedName{Name: "testmodule"}

	// Create module CR Ready=True first.
	createTestModuleCR(t, wt, metav1.Condition{
		Type:   status.ConditionTypeReady,
		Status: metav1.ConditionTrue,
		Reason: "Ready",
	})

	// Phase 1: module reports an OLDER version. status.release.version must mirror it.
	setTestModuleRelease(wt, "2.19.0")
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionTrue),
		jq.Match(`.status.release.version == "2.19.0"`),
	))

	// Phase 2: module upgrades and now reports the CURRENT platform version.
	// status.release.version must update to reflect the handshake completion.
	setTestModuleRelease(wt, "2.20.0")
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`.status.release.version == "2.20.0"`),
	)
}

// TestPlatformModuleReconciler_DriftCleanup verifies that resources in
// status.resources that are absent from the current render are removed on the
// next reconcile.
func TestPlatformModuleReconciler_DriftCleanup(t *testing.T) {
	_, wt := startPlatformModuleControllerWith(t, modules.NewRegistry())

	createPlatformModuleCR(t, wt, "drift-test-module")

	nn := types.NamespacedName{Name: "drift-test-module"}

	// Wait for first reconcile before injecting stale state.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`[.status.conditions[] | select(.type == "%s")] | length > 0`,
			status.ConditionDeploymentsAvailable),
	)

	// Inject a stale resource ref into status.
	pm := &configv1alpha1.PlatformModule{}
	wt.Expect(wt.Client().Get(wt.Context(), nn, pm)).Should(Succeed())
	pm.Status.Resources = []configv1alpha1.ResourceRef{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default", Name: "phantom"},
	}
	wt.Expect(wt.Client().Status().Update(wt.Context(), pm)).Should(Succeed())

	// Touch spec to trigger reconcile (status-only updates don't bump generation).
	wt.Expect(wt.Client().Get(wt.Context(), nn, pm)).Should(Succeed())
	pm.Annotations = map[string]string{"trigger": "reconcile"}
	wt.Expect(wt.Client().Update(wt.Context(), pm)).Should(Succeed())

	// After drift cleanup, the phantom Deployment must be gone from status.resources.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`[.status.resources[] | select(.kind == "Deployment")] | length == 0`),
	)
}

// TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRHasNoConditions verifies
// that when the module CR exists but has no conditions yet (operator still initializing),
// OperandAvailable is False with reason OperandInitializing and NO Info severity —
// this blocks the DAG until the module operator reports health.
func TestPlatformModuleReconciler_OperandAvailable_WhenModuleCRHasNoConditions(t *testing.T) {
	h := newNoopHandlerWithGVK("testmodule", testModuleGVK)

	reg := modules.NewRegistry()
	reg.Add(&h)

	_, wt := startPlatformModuleControllerWith(t, reg)
	createPlatformModuleCR(t, wt, "testmodule")

	nn := types.NamespacedName{Name: "testmodule"}

	// Wait for first reconcile (CR absent initially).
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`[.status.conditions[] | select(.type == "%s")] | length > 0`,
			status.ConditionTypeOperandAvailable),
	)

	// Create the module CR with NO conditions (simulates a freshly created CR
	// before the module operator has reconciled it).
	createTestModuleCR(t, wt)

	// OperandAvailable must be False with reason OperandInitializing, no Info severity.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .reason == "OperandInitializing"`,
			status.ConditionTypeOperandAvailable),
		jq.Match(`[.status.conditions[] | select(.type == "%s" and .severity == "Info")] | length == 0`,
			status.ConditionTypeOperandAvailable),
	))
}

// TestPlatformModuleReconciler_DynamicWatchActivatesOnCRDCreation verifies the
// end-to-end CRD watch → dynamic module CR watch → status reflection flow:
// 1. The module CRD does not exist at startup → OperandAbsent
// 2. CRD is installed dynamically → triggers reconcile
// 3. Module CR is created with Ready=True → OperandAvailable=True
//
// This proves the CRD watch fires and enables the dynamic module CR watch.
func TestPlatformModuleReconciler_DynamicWatchActivatesOnCRDCreation(t *testing.T) {
	// Use a unique GVK whose CRD is NOT loaded from testdata/ at startup.
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

	// Step 1: CRD does not exist → OperandAbsent (Info severity, non-blocking).
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .reason == "OperandAbsent"`,
			status.ConditionTypeOperandAvailable),
	)

	// Step 2: Install the CRD dynamically. The CRD watch fires, the dynamic module
	// CR watch activates on the next reconcile.
	crd, err := et.RegisterCRD(wt.Context(), dynamicGVK,
		"dynamictestmodules", "dynamictestmodule",
		apiextensionsv1.ClusterScoped,
		envt.WithPermissiveSchema())

	NewWithT(t).Expect(err).NotTo(HaveOccurred())
	envt.CleanupDelete(t, NewWithT(t), context.Background(), wt.Client(), crd)

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

	// OperandAvailable must become True — proving the dynamic watch activated.
	wt.Get(gvk.PlatformModule, nn).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeOperandAvailable, metav1.ConditionTrue),
	)
}
