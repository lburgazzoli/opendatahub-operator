package platform_test

import (
	"context"
	"path/filepath"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/platform"
	sr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/services/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/mocks"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/testf"
	"github.com/opendatahub-io/opendatahub-operator/v2/tests/envtestutil"

	. "github.com/onsi/gomega"
)

// startPlatformController starts an envtest environment with the Platform
// reconciler wired to empty component and service registries. The module
// registry is injectable per-test for isolation.
func startPlatformController(t *testing.T, moduleReg *modules.Registry) *testf.WithT {
	t.Helper()
	g := NewWithT(t)

	ctx := t.Context()

	if moduleReg.Lookup("monitoring") == nil {
		moduleReg.Add(mocks.NewDefaultMockModuleHandler("monitoring", gvk.Monitoring))
	}
	if moduleReg.Lookup("aigateway") == nil {
		moduleReg.Add(mocks.NewDefaultMockModuleHandler("aigateway", gvk.AIGateway))
	}

	cluster.SetRelease(common.Release{Name: cluster.OpenDataHub})
	t.Cleanup(func() { cluster.SetRelease(common.Release{}) })

	root, err := envtestutil.FindProjectRoot()
	g.Expect(err).NotTo(HaveOccurred())

	et, err := envt.New(
		envt.WithCRDPaths(filepath.Join(root, "config", "crd", "bases")),
		envt.WithManager(ctrl.Options{
			Controller: ctrlconfig.Controller{SkipNameValidation: ptr.To(true)},
		}),
		envt.WithRegisterControllers(func(mgr ctrl.Manager) error {
			return platform.New(ctx, mgr,
				platform.WithModuleRegistry(moduleReg),
				platform.WithComponentRegistry(&cr.Registry{}),
				platform.WithServiceRegistry(&sr.Registry{}),
				platform.WithDeletePropagationPolicy(metav1.DeletePropagationBackground),
			)
		}),
	)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { _ = et.Stop() })

	et.StartManager(t, ctx)

	tc, err := et.NewTestContext(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	return tc.NewWithT(t)
}

func createPlatform(t *testing.T, wt *testf.WithT, spec configv1alpha1.PlatformSpec) {
	t.Helper()

	p := &configv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: configv1alpha1.PlatformInstanceName},
		Spec:       spec,
	}
	wt.Expect(wt.Client().Create(wt.Context(), p)).Should(Succeed())
	envt.CleanupDelete(t, NewWithT(t), context.Background(), wt.Client(), p)
}

func managedPlatformModules(names ...string) configv1alpha1.PlatformModules {
	modules := make(configv1alpha1.PlatformModules, 0, len(names))
	for _, name := range names {
		modules = append(modules, configv1alpha1.PlatformModuleConfig{
			Name:            name,
			ManagementState: operatorv1.Managed,
		})
	}
	return modules
}

// setPlatformModuleReady patches a PlatformModule's Ready condition via the
// status subresource so Platform's aggregateStatus can read it.
func setPlatformModuleReady(t *testing.T, wt *testf.WithT, name string, ready bool) {
	t.Helper()
	g := NewWithT(t)

	// Use Eventually in case the PlatformModule CR is not yet visible in the cache.
	pm := &configv1alpha1.PlatformModule{}
	g.Eventually(func() error {
		return wt.Client().Get(wt.Context(), types.NamespacedName{Name: name}, pm)
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
	g.Expect(wt.Client().Status().Update(wt.Context(), pm)).Should(Succeed())
}

// TestPlatformReconciler_CreatesPlatformModuleCRs: enabling a module in spec
// causes the reconciler to create a corresponding PlatformModule CR.
func TestPlatformReconciler_CreatesPlatformModuleCRs(t *testing.T) {
	wt := startPlatformController(t, modules.NewRegistry())

	createPlatform(t, wt, configv1alpha1.PlatformSpec{
		Modules: managedPlatformModules("monitoring"),
	})

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())
}

// TestPlatformReconciler_DeletesDisabledModuleCRs: disabling a module removes
// its PlatformModule CR.
func TestPlatformReconciler_DeletesDisabledModuleCRs(t *testing.T) {
	g := NewWithT(t)
	wt := startPlatformController(t, modules.NewRegistry())

	createPlatform(t, wt, configv1alpha1.PlatformSpec{
		Modules: managedPlatformModules("monitoring"),
	})

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())

	// Disable monitoring by removing the entry entirely.
	p := &configv1alpha1.Platform{}
	wt.Expect(wt.Client().Get(wt.Context(), types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}, p)).Should(Succeed())
	p.Spec.Modules = nil
	wt.Expect(wt.Client().Update(wt.Context(), p)).Should(Succeed())

	// PlatformModule CR must be gone (client.Get returns IsNotFound).
	pm := &configv1alpha1.PlatformModule{}
	g.Eventually(func() error {
		return wt.Client().Get(wt.Context(), types.NamespacedName{Name: "monitoring"}, pm)
	}).Should(MatchError(ContainSubstring("not found")))
}

// TestPlatformReconciler_NoModules: empty spec → ModulesReady=True immediately.
func TestPlatformReconciler_NoModules(t *testing.T) {
	wt := startPlatformController(t, modules.NewRegistry())

	createPlatform(t, wt, configv1alpha1.PlatformSpec{})

	wt.Get(gvk.Platform, types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}).
		Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeModulesReady, metav1.ConditionTrue),
	)
}

// TestPlatformReconciler_AggregatesModuleStatus: ModulesReady reflects the
// Ready condition of the PlatformModule CRs.
func TestPlatformReconciler_AggregatesModuleStatus(t *testing.T) {
	wt := startPlatformController(t, modules.NewRegistry())

	createPlatform(t, wt, configv1alpha1.PlatformSpec{
		Modules: managedPlatformModules("monitoring"),
	})

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())

	// ModulesReady=False — PlatformModule not yet Ready.
	wt.Get(gvk.Platform, types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}).
		Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeModulesReady, metav1.ConditionFalse),
	)

	setPlatformModuleReady(t, wt, "monitoring", true)

	// ModulesReady=True after PlatformModule becomes Ready.
	wt.Get(gvk.Platform, types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}).
		Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeModulesReady, metav1.ConditionTrue),
	)
}

// TestPlatformReconciler_ReportsEnabledModulesInStatus: status.modules reflects
// which modules are currently enabled in spec.
func TestPlatformReconciler_ReportsEnabledModulesInStatus(t *testing.T) {
	wt := startPlatformController(t, modules.NewRegistry())

	createPlatform(t, wt, configv1alpha1.PlatformSpec{
		Modules: managedPlatformModules("monitoring"),
	})

	nn := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	// status.modules must include the "monitoring" summary row.
	wt.Get(gvk.Platform, nn).Eventually().Should(
		jq.Match(`.status.modules[] | select(.name == "monitoring") | .name == "monitoring"`),
	)

	// Disable monitoring by removing the entry — status.modules must become empty.
	p := &configv1alpha1.Platform{}
	wt.Expect(wt.Client().Get(wt.Context(), nn, p)).Should(Succeed())
	p.Spec.Modules = nil
	wt.Expect(wt.Client().Update(wt.Context(), p)).Should(Succeed())

	wt.Get(gvk.Platform, nn).Eventually().Should(
		jq.Match(`.status.modules == null or (.status.modules | length) == 0`),
	)
}

// TestPlatformReconciler_ModulesReadyReflectsAllModules: with two modules
// enabled, ModulesReady=True only when both are Ready; one not-ready fails
// the condition and names the failing module.
func TestPlatformReconciler_ModulesReadyReflectsAllModules(t *testing.T) {
	wt := startPlatformController(t, modules.NewRegistry())

	createPlatform(t, wt, configv1alpha1.PlatformSpec{
		Modules: managedPlatformModules("monitoring", "aigateway"),
	})

	nn := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	// Both PlatformModule CRs must be created.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).Eventually().Should(Succeed())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).Eventually().Should(Succeed())

	// Mark both Ready.
	setPlatformModuleReady(t, wt, "monitoring", true)
	setPlatformModuleReady(t, wt, "aigateway", true)

	wt.Get(gvk.Platform, nn).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeModulesReady, metav1.ConditionTrue),
	)

	// Mark aigateway not-ready — ModulesReady must flip to False, naming "aigateway".
	setPlatformModuleReady(t, wt, "aigateway", false)

	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeModulesReady, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .message | contains("aigateway")`,
			status.ConditionTypeModulesReady),
	))
}
