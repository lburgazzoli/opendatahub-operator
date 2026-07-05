package dscinitialization_test

import (
	"context"
	"path/filepath"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
	dscictrl "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/dscinitialization"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	monitoringModule "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules/monitoring"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/operatorconfig"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/testf"
	"github.com/opendatahub-io/opendatahub-operator/v2/tests/envtestutil"

	. "github.com/onsi/gomega"
)

func startDSCIModuleController(t *testing.T, modReg *modules.Registry) *testf.TestContext {
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
			filepath.Join(root, "config", "crd", "external"),
		),
		envt.WithManager(ctrl.Options{
			Controller: ctrlconfig.Controller{SkipNameValidation: new(true)},
		}),
		envt.WithRegisterControllers(func(mgr ctrl.Manager) error {
			return (&dscictrl.DSCInitializationReconciler{
				Client:   mgr.GetClient(),
				Scheme:   mgr.GetScheme(),
				Recorder: mgr.GetEventRecorder("dscinitialization-controller"),
				OperatorSettings: operatorconfig.OperatorSettings{
					ManifestsBasePath: filepath.Join(root, "config"),
				},
				ModuleRegistry: modReg,
			}).SetupWithManager(ctx, mgr)
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { _ = et.Stop() })

	et.StartManager(t, ctx)

	tc, err := et.NewTestContext(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	return tc
}

func createMonitoringDSCI(t *testing.T, tc *testf.TestContext, state operatorv1.ManagementState) *dsciv2.DSCInitialization {
	t.Helper()
	g := NewWithT(t)

	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec: dsciv2.DSCInitializationSpec{
			ApplicationsNamespace: "default",
			Monitoring: serviceApi.DSCIMonitoring{
				ManagementSpec: common.ManagementSpec{
					ManagementState: state,
				},
				MonitoringCommonSpec: serviceApi.MonitoringCommonSpec{
					Namespace: "monitoring",
				},
			},
		},
	}
	g.Expect(tc.Client().Create(t.Context(), dsci)).Should(Succeed())
	t.Cleanup(func() { _ = tc.Client().Delete(context.Background(), dsci) })
	return dsci
}

func TestDSCIReconciler_ServiceModuleCRCreated(t *testing.T) {
	modReg := modules.NewRegistry()
	modReg.Add(monitoringModule.NewHandler())

	tc := startDSCIModuleController(t, modReg)
	wt := tc.NewWithT(t)
	cli := tc.Client()

	dsci := createMonitoringDSCI(t, tc, operatorv1.Managed)

	wt.Get(gvk.Monitoring, types.NamespacedName{Name: serviceApi.MonitoringInstanceName}).
		Eventually().Should(Succeed())

	wt.Eventually(func(g Gomega) {
		monitoringCR := &serviceApi.Monitoring{}
		g.Expect(cli.Get(t.Context(), types.NamespacedName{Name: serviceApi.MonitoringInstanceName}, monitoringCR)).To(Succeed())
		g.Expect(metav1.IsControlledBy(monitoringCR, dsci)).To(BeTrue())
	}).Should(Succeed())
}

func TestDSCIReconciler_ModuleStatusReportedToDSCI(t *testing.T) {
	modReg := modules.NewRegistry()
	modReg.Add(monitoringModule.NewHandler())

	tc := startDSCIModuleController(t, modReg)
	wt := tc.NewWithT(t)
	cli := tc.Client()

	createMonitoringDSCI(t, tc, operatorv1.Managed)

	dsciKey := types.NamespacedName{Name: "default-dsci"}
	wt.Get(gvk.Monitoring, types.NamespacedName{Name: serviceApi.MonitoringInstanceName}).
		Eventually().Should(Succeed())

	wt.Get(gvk.DSCInitialization, dsciKey).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionMonitoringReady, metav1.ConditionUnknown),
	)

	wt.Eventually(func(g Gomega) {
		monitoringCR := &serviceApi.Monitoring{}
		g.Expect(cli.Get(t.Context(), types.NamespacedName{Name: serviceApi.MonitoringInstanceName}, monitoringCR)).To(Succeed())
		monitoringCR.Status.Conditions = []common.Condition{{
			Type:               status.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             status.NotReadyReason,
			Message:            "Monitoring stack is not ready",
			LastTransitionTime: metav1.Now(),
		}}
		g.Expect(cli.Status().Update(t.Context(), monitoringCR)).To(Succeed())
	}).Should(Succeed())

	wt.Get(gvk.DSCInitialization, dsciKey).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeReady, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionMonitoringReady, metav1.ConditionTrue),
	))
}

func TestDSCIReconciler_PlatformCRSyncedWithEnabledModules(t *testing.T) {
	modReg := modules.NewRegistry()
	modReg.Add(monitoringModule.NewHandler())

	tc := startDSCIModuleController(t, modReg)
	wt := tc.NewWithT(t)

	createMonitoringDSCI(t, tc, operatorv1.Managed)

	wt.Get(gvk.Platform, types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}).Eventually().Should(
		jq.Match(`.spec.modules.monitoring.managementState == "Managed"`),
	)
}

func TestDSCIReconciler_CleanupDisabledServiceModules_DeletesOwnedCR(t *testing.T) {
	modReg := modules.NewRegistry()
	modReg.Add(monitoringModule.NewHandler())

	tc := startDSCIModuleController(t, modReg)
	wt := tc.NewWithT(t)
	cli := tc.Client()

	createMonitoringDSCI(t, tc, operatorv1.Managed)

	wt.Get(gvk.Monitoring, types.NamespacedName{Name: serviceApi.MonitoringInstanceName}).
		Eventually().Should(Succeed())

	wt.Eventually(func(g Gomega) {
		dsci := &dsciv2.DSCInitialization{}
		g.Expect(cli.Get(t.Context(), types.NamespacedName{Name: "default-dsci"}, dsci)).To(Succeed())
		dsci.Spec.Monitoring.ManagementState = operatorv1.Removed
		g.Expect(cli.Update(t.Context(), dsci)).To(Succeed())
	}).Should(Succeed())

	wt.Eventually(func(g Gomega) {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk.Monitoring)
		err := cli.Get(t.Context(), types.NamespacedName{Name: serviceApi.MonitoringInstanceName}, u)
		if err != nil {
			g.Expect(err).To(MatchError(ContainSubstring("not found")))
			return
		}
		g.Expect(u.GetDeletionTimestamp()).NotTo(BeNil())
	}).Should(Succeed())
}
