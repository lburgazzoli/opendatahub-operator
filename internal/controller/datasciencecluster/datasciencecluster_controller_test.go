package datasciencecluster_test

import (
	"context"
	"path/filepath"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/datasciencecluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	rrtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/testf"
	"github.com/opendatahub-io/opendatahub-operator/v2/tests/envtestutil"

	. "github.com/onsi/gomega"
)

// testModuleGVK is the GVK of the test-only CRD in platformmodule/testdata.
var testModuleGVK = schema.GroupVersionKind{
	Group:   "components.platform.opendatahub.io",
	Version: "v1alpha1",
	Kind:    "TestModule",
}

const testModuleCRName = "default-testmodule"

// startDSCController starts an envtest environment with the DSC reconciler using
// the provided component and module registries. No component/module controllers
// are registered — DSC operates alone.
func startDSCController(t *testing.T, compReg *cr.Registry, modReg *modules.Registry) *testf.TestContext {
	t.Helper()
	g := NewWithT(t)

	ctx := t.Context()

	cluster.SetRelease(common.Release{Name: cluster.OpenDataHub})
	t.Cleanup(func() { cluster.SetRelease(common.Release{}) })

	root, err := envtestutil.FindProjectRoot()
	g.Expect(err).NotTo(HaveOccurred())

	et, err := envt.New(
		envt.WithCRDPaths(
			filepath.Join(root, "config", "crd", "bases"),
			// TestModule CRD for module CR tests.
			filepath.Join(root, "internal", "controller", "platformmodule", "testdata", "manifests", "testmodule"),
		),
		envt.WithManager(ctrl.Options{
			Controller: ctrlconfig.Controller{SkipNameValidation: ptr.To(true)},
		}),
		envt.WithRegisterControllers(func(mgr ctrl.Manager) error {
			return datasciencecluster.NewDataScienceClusterReconciler(ctx, mgr,
				datasciencecluster.WithComponentRegistry(compReg),
				datasciencecluster.WithModuleRegistry(modReg),
				datasciencecluster.WithDeletePropagationPolicy(metav1.DeletePropagationBackground),
			)
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { _ = et.Stop() })

	et.StartManager(t, ctx)

	tc, err := et.NewTestContext(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	return tc
}

// createDSCI creates the required DSCI singleton. checkPreConditions requires it.
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

// createDSC creates the DSC singleton and registers cleanup.
func createDSC(t *testing.T, tc *testf.TestContext, spec dscv2.DataScienceClusterSpec) *dscv2.DataScienceCluster {
	t.Helper()
	g := NewWithT(t)

	dsc := &dscv2.DataScienceCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsc"},
		Spec:       spec,
	}
	g.Expect(tc.Client().Create(context.Background(), dsc)).Should(Succeed())
	t.Cleanup(func() { _ = tc.Client().Delete(context.Background(), dsc) })
	return dsc
}

// TestDSCReconciler_ComponentCRsCreated verifies that the DSC controller creates
// component CRs for enabled in-tree components. NewCRObject returns a Dashboard
// CR which the deploy action SSA-applies to the real Kubernetes API server.
func TestDSCReconciler_ComponentCRsCreated(t *testing.T) {
	compReg := &cr.Registry{}
	compReg.Add(&cr.BaseComponentHandler{
		Name:        "dashboard",
		GVK:         gvk.Dashboard,
		IsEnabledFn: func(_ *dscv2.DataScienceCluster) bool { return true },
		NewCRObjectFn: func(_ context.Context, _ client.Client, _ *dscv2.DataScienceCluster) (common.PlatformObject, error) {
			return &componentApi.Dashboard{ObjectMeta: metav1.ObjectMeta{Name: "default-dashboard"}}, nil
		},
	})

	tc := startDSCController(t, compReg, modules.NewRegistry())
	wt := tc.NewWithT(t)

	createDSCI(t, tc)
	createDSC(t, tc, dscv2.DataScienceClusterSpec{})

	// Dashboard CR must be created by provisionComponents + deploy.
	wt.Get(gvk.Dashboard, types.NamespacedName{Name: "default-dashboard"}).
		Eventually().Should(Succeed())
}

// TestDSCReconciler_ComponentStatusReportedToDSC verifies that UpdateDSCStatus
// return values are aggregated into ComponentsReady on the DSC.
// UpdateDSCStatus is the k8s read: each component handler reads its own CR.
// Here we control the return value via the updateDSCFunc field.
func TestDSCReconciler_ComponentStatusReportedToDSC(t *testing.T) {
	compReg := &cr.Registry{}

	compReg.Add(&cr.BaseComponentHandler{
		Name:        "comp-a",
		GVK:         gvk.Dashboard,
		IsEnabledFn: func(_ *dscv2.DataScienceCluster) bool { return true },
		UpdateDSCStatusFn: func(_ context.Context, _ *rrtypes.ReconciliationRequest) (metav1.ConditionStatus, error) {
			return metav1.ConditionTrue, nil
		},
	})

	// comp-b reports not ready — DSC should reflect this.
	compReg.Add(&cr.BaseComponentHandler{
		Name:        "comp-b",
		GVK:         gvk.Kserve,
		IsEnabledFn: func(_ *dscv2.DataScienceCluster) bool { return true },
		UpdateDSCStatusFn: func(_ context.Context, _ *rrtypes.ReconciliationRequest) (metav1.ConditionStatus, error) {
			return metav1.ConditionFalse, nil
		},
	})

	tc := startDSCController(t, compReg, modules.NewRegistry())
	wt := tc.NewWithT(t)

	createDSCI(t, tc)
	createDSC(t, tc, dscv2.DataScienceClusterSpec{})

	// ComponentsReady=False because comp-b reported ConditionFalse.
	wt.Get(gvk.DataScienceCluster, types.NamespacedName{Name: "default-dsc"}).
		Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeComponentsReady, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .message | contains("comp-b")`,
			status.ConditionTypeComponentsReady),
	))
}

// testModuleHandler wraps modules.BaseHandler and only overrides BuildModuleCR
// and IsEnabled. GetModuleStatus, GetModuleCRState and other methods use the
// real BaseHandler implementations that read from Kubernetes.
type testModuleHandler struct {
	modules.BaseHandler
}

func (h *testModuleHandler) BuildModuleCR(_ context.Context, _ client.Client, _ *modules.PlatformContext) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(h.Config.GVK)
	u.SetName(h.Config.CRName)
	return u, nil
}

func (h *testModuleHandler) IsEnabled(_ *modules.PlatformContext) bool { return true }

func newTestModuleHandler() *testModuleHandler {
	return &testModuleHandler{
		BaseHandler: modules.BaseHandler{
			Config: modules.ModuleConfig{
				Name:   "testmodule",
				GVK:    testModuleGVK,
				CRName: testModuleCRName,
			},
		},
	}
}

// TestDSCReconciler_ModuleCRsCreated verifies that the DSC controller creates
// module operand CRs for enabled modules. BuildModuleCR returns a TestModule CR
// which the deploy action SSA-applies to the real Kubernetes API server.
func TestDSCReconciler_ModuleCRsCreated(t *testing.T) {
	modReg := modules.NewRegistry()
	modReg.Add(newTestModuleHandler())

	tc := startDSCController(t, &cr.Registry{}, modReg)
	wt := tc.NewWithT(t)

	createDSCI(t, tc)
	createDSC(t, tc, dscv2.DataScienceClusterSpec{})

	// TestModule CR must be created by provisionModuleCRs + deploy.
	wt.Get(testModuleGVK, types.NamespacedName{Name: testModuleCRName}).
		Eventually().Should(Succeed())
}

// TestDSCReconciler_ModuleStatusReportedToDSC verifies that module CR status
// read from Kubernetes by GetModuleStatus (BaseHandler) is aggregated into
// ModulesReady on the DSC.
//
// Phase 1: TestModule CR has no Ready condition → ModulesReady=False.
// Phase 2: patch TestModule CR status Ready=True → ModulesReady=True.
func TestDSCReconciler_ModuleStatusReportedToDSC(t *testing.T) {
	modReg := modules.NewRegistry()
	modReg.Add(newTestModuleHandler())

	tc := startDSCController(t, &cr.Registry{}, modReg)
	wt := tc.NewWithT(t)

	createDSCI(t, tc)
	dsc := createDSC(t, tc, dscv2.DataScienceClusterSpec{})
	dscKey := types.NamespacedName{Name: dsc.Name}

	// Phase 1: TestModule CR exists (created by deploy) but has no Ready condition.
	// GetModuleStatus (BaseHandler) reads from Kubernetes → empty conditions → not ready.
	wt.Get(testModuleGVK, types.NamespacedName{Name: testModuleCRName}).
		Eventually().Should(Succeed())

	wt.Get(gvk.DataScienceCluster, dscKey).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeModulesReady, metav1.ConditionFalse),
	)

	// Phase 2: patch the TestModule CR status with Ready=True directly via Kubernetes.
	// GetModuleStatus reads from Kubernetes on the next reconcile → ModulesReady=True.
	testModule := &unstructured.Unstructured{}
	testModule.SetGroupVersionKind(testModuleGVK)
	NewWithT(t).Expect(
		tc.Client().Get(context.Background(), types.NamespacedName{Name: testModuleCRName}, testModule),
	).Should(Succeed())
	_ = unstructured.SetNestedSlice(testModule.Object, []any{
		map[string]any{
			"type":               status.ConditionTypeReady,
			"status":             string(metav1.ConditionTrue),
			"reason":             "Ready",
			"lastTransitionTime": metav1.Now().UTC().Format("2006-01-02T15:04:05Z"),
		},
	}, "status", "conditions")
	NewWithT(t).Expect(tc.Client().Status().Update(context.Background(), testModule)).Should(Succeed())

	// Trigger re-reconcile so updateStatus picks up the new CR status.
	latestDSC := &dscv2.DataScienceCluster{}
	NewWithT(t).Expect(tc.Client().Get(context.Background(), dscKey, latestDSC)).Should(Succeed())
	if latestDSC.Annotations == nil {
		latestDSC.Annotations = map[string]string{}
	}
	latestDSC.Annotations["test/trigger"] = "ready"
	NewWithT(t).Expect(tc.Client().Update(context.Background(), latestDSC)).Should(Succeed())

	wt.Get(gvk.DataScienceCluster, dscKey).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeModulesReady, metav1.ConditionTrue),
	)
}

// TestDSCReconciler_PlatformCRSyncedWithEnabledModules verifies that
// syncPlatformModules SSA-patches Platform.Spec.Modules to match the DSC spec.
func TestDSCReconciler_PlatformCRSyncedWithEnabledModules(t *testing.T) {
	tc := startDSCController(t, &cr.Registry{}, modules.NewRegistry())
	wt := tc.NewWithT(t)

	createDSCI(t, tc)
	createDSC(t, tc, dscv2.DataScienceClusterSpec{
		Components: dscv2.Components{
			AIGateway: componentApi.DSCAIGateway{
				ManagementSpec: common.ManagementSpec{
					ManagementState: operatorv1.Managed,
				},
			},
		},
	})

	platformKey := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	// Platform CR must be created with aigateway=Managed.
	wt.Get(gvk.Platform, platformKey).Eventually().Should(
		jq.Match(`.spec.modules.aigateway.managementState == "Managed"`),
	)
}
