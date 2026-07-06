package platform_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/spf13/viper"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/datasciencecluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/dscinitialization"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/platform"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/platformmodule"
	sr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/services/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/operatorconfig"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/testf"
	"github.com/opendatahub-io/opendatahub-operator/v2/tests/envtestutil"

	. "github.com/onsi/gomega"
)

// ---------------------------------------------------------------------------
// GVK constants for test-only module CRDs (registered dynamically).
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Test module handler — minimal handler backed by BaseHandler.
// ---------------------------------------------------------------------------

type testModuleHandler struct {
	modules.BaseHandler

	applyFn     func(*modules.PlatformContext, *configv1alpha1.PlatformModules)
	isEnabledFn func(*modules.PlatformContext) bool
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

func (h *testModuleHandler) ApplyManagementState(ctx *modules.PlatformContext, spec *configv1alpha1.PlatformModules) {
	if h.applyFn != nil {
		h.applyFn(ctx, spec)
	}
}

func (h *testModuleHandler) IsEnabled(ctx *modules.PlatformContext) bool {
	if h.isEnabledFn != nil {
		return h.isEnabledFn(ctx)
	}
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

// newAIGatewayModuleHandler creates a test handler named "aigateway" with an
// ApplyManagementState implementation that reads from DSC.Spec.Components.AIGateway,
// mirroring the real aigateway handler's behaviour in integration tests.
func newAIGatewayModuleHandler(gvkVal schema.GroupVersionKind) *testModuleHandler {
	return &testModuleHandler{
		BaseHandler: modules.BaseHandler{
			Config: modules.ModuleConfig{
				Name:   "aigateway",
				GVK:    gvkVal,
				CRName: "default-aigateway",
			},
		},
		applyFn: func(ctx *modules.PlatformContext, spec *configv1alpha1.PlatformModules) {
			if ctx == nil || ctx.DSC == nil {
				return
			}
			spec.Set(configv1alpha1.PlatformModuleConfig{
				Name:            "aigateway",
				ManagementState: ctx.DSC.Spec.Components.AIGateway.ManagementState,
			})
		},
	}
}

func newMonitoringModuleHandler() *testModuleHandler {
	return &testModuleHandler{
		BaseHandler: modules.BaseHandler{
			Config: modules.ModuleConfig{
				Name:   serviceApi.MonitoringServiceName,
				GVK:    gvk.Monitoring,
				CRName: serviceApi.MonitoringInstanceName,
			},
		},
		applyFn: func(ctx *modules.PlatformContext, spec *configv1alpha1.PlatformModules) {
			if ctx == nil || ctx.DSCI == nil {
				return
			}
			spec.Set(configv1alpha1.PlatformModuleConfig{
				Name:            serviceApi.MonitoringServiceName,
				ManagementState: ctx.DSCI.Spec.Monitoring.ManagementState,
			})
		},
		isEnabledFn: func(ctx *modules.PlatformContext) bool {
			return ctx != nil && ctx.DSCI != nil && ctx.DSCI.Spec.Monitoring.ManagementState == operatorv1.Managed
		},
	}
}

func managedPlatformEntries(names ...string) configv1alpha1.PlatformModules {
	entries := make(configv1alpha1.PlatformModules, 0, len(names))
	for _, name := range names {
		entries = append(entries, configv1alpha1.PlatformModuleConfig{
			Name:            name,
			ManagementState: operatorv1.Managed,
		})
	}
	return entries
}

// ---------------------------------------------------------------------------
// Suite options and controller bootstrap.
// ---------------------------------------------------------------------------

type suiteOpts struct {
	moduleReg    *modules.Registry
	componentReg *cr.Registry
	provisionReg *provision.UnifiedRegistry
	startDSCI    bool
}

func startAllControllers(t *testing.T, opts suiteOpts) (*envt.EnvT, *testf.TestContext) {
	t.Helper()
	g := NewWithT(t)

	ctx := t.Context()

	cluster.SetRelease(common.Release{Name: cluster.OpenDataHub})
	t.Cleanup(func() { cluster.SetRelease(common.Release{}) })

	viper.Set("rhai-applications-namespace", "default")
	t.Cleanup(func() { viper.Set("rhai-applications-namespace", "") })

	provision.GetRunlevelTracker().Reset()
	t.Cleanup(func() { provision.GetRunlevelTracker().Reset() })

	if opts.provisionReg != nil && opts.moduleReg != nil {
		opts.moduleReg.ProvisionRegistry = opts.provisionReg
	}
	if opts.provisionReg != nil && opts.componentReg != nil {
		opts.componentReg.ProvisionRegistry = opts.provisionReg
	}

	root, err := envtestutil.FindProjectRoot()
	g.Expect(err).NotTo(HaveOccurred())

	et, err := envt.New(
		envt.WithCRDPaths(
			filepath.Join(root, "config", "crd", "bases"),
			filepath.Join(root, "config", "crd", "external"),
		),
		envt.WithManager(ctrl.Options{
			Controller: ctrlconfig.Controller{
				SkipNameValidation: new(true),
			},
		}),
		envt.WithRegisterControllers(func(mgr ctrl.Manager) error {
			if err := platform.New(ctx, mgr,
				platform.WithModuleRegistry(opts.moduleReg),
				platform.WithComponentRegistry(opts.componentReg),
				platform.WithServiceRegistry(&sr.Registry{}),
				platform.WithProvisionRegistry(opts.provisionReg),
				platform.WithDeletePropagationPolicy(metav1.DeletePropagationBackground),
				platform.WithStuckTracker(dag.NewStuckTracker()),
			); err != nil {
				return err
			}

			if err := platformmodule.New(ctx, mgr,
				platformmodule.WithRegistry(opts.moduleReg),
				platformmodule.WithComponentRegistry(opts.componentReg),
				platformmodule.WithServiceRegistry(&sr.Registry{}),
				platformmodule.WithProvisionRegistry(opts.provisionReg),
				platformmodule.WithTracker(provision.GetRunlevelTracker()),
				platformmodule.WithDeletePropagationPolicy(metav1.DeletePropagationBackground),
			); err != nil {
				return err
			}

			if err := datasciencecluster.NewDataScienceClusterReconciler(ctx, mgr,
				datasciencecluster.WithComponentRegistry(opts.componentReg),
				datasciencecluster.WithModuleRegistry(opts.moduleReg),
				datasciencecluster.WithProvisionRegistry(opts.provisionReg),
				datasciencecluster.WithDeletePropagationPolicy(metav1.DeletePropagationBackground),
			); err != nil {
				return err
			}

			if opts.startDSCI {
				if err := (&dscinitialization.DSCInitializationReconciler{
					Client:   mgr.GetClient(),
					Scheme:   mgr.GetScheme(),
					Recorder: mgr.GetEventRecorder("dscinitialization-controller"),
					OperatorSettings: operatorconfig.OperatorSettings{
						ManifestsBasePath: filepath.Join(root, "config"),
					},
					ModuleRegistry: opts.moduleReg,
				}).SetupWithManager(ctx, mgr); err != nil {
					return err
				}
			}

			return nil
		}),
	)

	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { _ = et.Stop() })

	et.StartManager(t, ctx)

	tc, err := et.NewTestContext(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	return et, tc
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func createDSCI(t *testing.T, tc *testf.TestContext) {
	t.Helper()
	createDSCIWithSpec(t, tc, dsciv2.DSCInitializationSpec{
		ApplicationsNamespace: "default",
	})
}

func createDSCIWithSpec(t *testing.T, tc *testf.TestContext, spec dsciv2.DSCInitializationSpec) {
	t.Helper()
	g := NewWithT(t)

	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       spec,
	}
	g.Expect(tc.Client().Create(t.Context(), dsci)).Should(Succeed())
	t.Cleanup(func() { _ = tc.Client().Delete(context.Background(), dsci) })
}

func createGatewayConfig(t *testing.T, tc *testf.TestContext) {
	t.Helper()
	g := NewWithT(t)

	gwCfg := &serviceApi.GatewayConfig{}
	gwCfg.SetName(serviceApi.GatewayConfigName)
	g.Expect(tc.Client().Create(t.Context(), gwCfg)).Should(Succeed())
	envt.CleanupDelete(t, g, context.Background(), tc.Client(), gwCfg)

	gwCfg.Status.Domain = "example.com"
	g.Expect(tc.Client().Status().Update(t.Context(), gwCfg)).Should(Succeed())
}

func createPlatform(t *testing.T, tc *testf.TestContext, spec configv1alpha1.PlatformSpec) {
	t.Helper()
	g := NewWithT(t)

	p := &configv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: configv1alpha1.PlatformInstanceName},
		Spec:       spec,
	}
	g.Expect(tc.Client().Create(t.Context(), p)).Should(Succeed())
	envt.CleanupDelete(t, g, context.Background(), tc.Client(), p)
}

func createDSC(t *testing.T, tc *testf.TestContext, spec dscv2.DataScienceClusterSpec) {
	t.Helper()
	g := NewWithT(t)

	dsc := &dscv2.DataScienceCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsc"},
		Spec:       spec,
	}
	g.Expect(tc.Client().Create(t.Context(), dsc)).Should(Succeed())
	envt.CleanupDelete(t, g, context.Background(), tc.Client(), dsc)
}

func setPlatformModuleReady(t *testing.T, cli client.Client, name string, ready bool) {
	t.Helper()
	g := NewWithT(t)

	condStatus := metav1.ConditionFalse
	if ready {
		condStatus = metav1.ConditionTrue
	}

	g.Eventually(func() error {
		pm := &configv1alpha1.PlatformModule{}
		if err := cli.Get(t.Context(), types.NamespacedName{Name: name}, pm); err != nil {
			return err
		}
		pm.Status.Conditions = []common.Condition{{
			Type:               status.ConditionTypeReady,
			Status:             condStatus,
			Reason:             "Test",
			LastTransitionTime: metav1.Now(),
		}}
		return cli.Status().Update(t.Context(), pm)
	}).Should(Succeed())
}

func setUnstructuredReady(t *testing.T, cli client.Client, u *unstructured.Unstructured, ready bool) { //nolint:unparam
	t.Helper()
	g := NewWithT(t)

	condStatus := metav1.ConditionFalse
	if ready {
		condStatus = metav1.ConditionTrue
	}

	g.Eventually(func() error {
		if err := cli.Get(t.Context(), client.ObjectKeyFromObject(u), u); err != nil {
			return err
		}

		obj := common.NewUnstructuredPlatformObject(u)
		obj.SetConditions([]common.Condition{{
			Type:               status.ConditionTypeReady,
			Status:             condStatus,
			Reason:             "Test",
			LastTransitionTime: metav1.Now(),
		}})

		return cli.Status().Update(t.Context(), u)
	}).Should(Succeed())
}

func registerModuleCRD(t *testing.T, et *envt.EnvT, gvkVal schema.GroupVersionKind) {
	t.Helper()
	g := NewWithT(t)

	plural := strings.ToLower(gvkVal.Kind) + "s"
	singular := strings.ToLower(gvkVal.Kind)

	crd, err := et.RegisterCRD(
		t.Context(),
		gvkVal,
		plural, singular,
		apiextensionsv1.ClusterScoped,
		envt.WithPermissiveSchema(),
	)
	g.Expect(err).NotTo(HaveOccurred())
	envt.CleanupDelete(t, g, context.Background(), et.Client(), crd)
}

// ---------------------------------------------------------------------------
// DAG metric helpers
// ---------------------------------------------------------------------------

func resetDAGMetrics() {
	provision.RunlevelStatus.Reset()
	provision.RunlevelDurationSeconds.Reset()
	provision.RunlevelCleared.Set(0)
	provision.RunlevelBlocked.Set(0)
	provision.RunlevelTimeoutTotal.Reset()
}

func captureBatchCount() float64 {
	return testutil.ToFloat64(provision.BatchesProcessedTotal)
}
