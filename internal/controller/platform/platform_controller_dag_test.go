//nolint:testpackage
package platform

import (
	"context"
	"path/filepath"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	pmctrl "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/platformmodule"
	sr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/services/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/mocks"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/testf"
	"github.com/opendatahub-io/opendatahub-operator/v2/tests/envtestutil"

	. "github.com/onsi/gomega"
)

// startPlatformControllerFull starts the Platform controller with fully
// injectable registries for DAG gating integration tests.
func startPlatformControllerFull(
	t *testing.T,
	moduleReg *modules.Registry,
	componentReg *cr.Registry,
	provisionReg *provision.UnifiedRegistry,
) *testf.TestContext {
	t.Helper()
	g := NewWithT(t)

	ctx := t.Context()

	viper.Set("rhai-applications-namespace", "default")
	t.Cleanup(func() { viper.Set("rhai-applications-namespace", "") })

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
			tracker := provision.GetRunlevelTracker()

			if err := New(ctx, mgr,
				WithModuleRegistry(moduleReg),
				WithComponentRegistry(componentReg),
				WithServiceRegistry(&sr.Registry{}),
				WithProvisionRegistry(provisionReg),
				WithTracker(tracker),
				WithDeletePropagationPolicy(metav1.DeletePropagationBackground),
				WithStuckTracker(dag.NewStuckTracker()),
			); err != nil {
				return err
			}

			return pmctrl.New(ctx, mgr,
				pmctrl.WithRegistry(moduleReg),
				pmctrl.WithComponentRegistry(componentReg),
				pmctrl.WithProvisionRegistry(provisionReg),
				pmctrl.WithTracker(tracker),
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

// setUnstructuredReady patches an unstructured CR's status.conditions with
// a Ready condition via the status subresource.
func setUnstructuredReady(t *testing.T, tc *testf.TestContext, u *unstructured.Unstructured, ready bool) {
	t.Helper()
	g := NewWithT(t)

	// Re-fetch to get the latest resourceVersion before updating status.
	g.Expect(tc.Client().Get(context.Background(), client.ObjectKeyFromObject(u), u)).Should(Succeed())

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

	g.Expect(tc.Client().Status().Update(context.Background(), u)).Should(Succeed())
}

// TestPlatformReconciler_DAGGating_ComponentBlocksModule verifies that the
// composite checker (componentReadinessChecker + moduleReadinessChecker) gates
// DAG advancement correctly when components and modules are at different runlevels.
//
// Setup: Dashboard operand at RL10, monitoring module at RL20.
//
// Expected behaviour:
//  1. No component CR → dashboard operand not ready, DAG blocked
//  2. Component CR exists but Ready=False → still blocked
//  3. Component CR Ready=True → dashboard operand becomes ready, RL10 clears
//  4. Monitoring PlatformModule Ready=True → ModulesReady=True
func TestPlatformReconciler_DAGGating_ComponentBlocksModule(t *testing.T) {
	// Provision registry: Dashboard at RL10, monitoring at RL20.
	provReg := provision.NewRegistry()
	provReg.Add("dashboard", provision.KindComponent, dag.RL(10))
	provReg.Add("monitoring", provision.KindModule, dag.RL(20))
	provReg.Enable("dashboard")
	provReg.Enable("monitoring")

	// Component registry with a mock Dashboard handler.
	componentReg := &cr.Registry{}
	componentReg.Add(mocks.NewDefaultMockComponentHandler("dashboard", gvk.Dashboard))

	// Module registry needs at least one entry so walkModuleDAG doesn't return early.
	moduleReg := modules.NewRegistry()
	moduleReg.Add(mocks.NewDefaultMockModuleHandler("monitoring", gvk.Monitoring))

	tc := startPlatformControllerFull(t, moduleReg, componentReg, provReg)
	cli := tc.Client()
	ctx := context.Background()

	// Create Platform CR with both dashboard (tracker-only) and monitoring.
	p := &configv1alpha1.Platform{
		ObjectMeta: metav1.ObjectMeta{Name: configv1alpha1.PlatformInstanceName},
		Spec: configv1alpha1.PlatformSpec{
			Modules: configv1alpha1.PlatformModules{
				{Name: "dashboard", ManagementState: operatorv1.Managed},
				{Name: "monitoring", ManagementState: operatorv1.Managed},
			},
		},
	}
	NewWithT(t).Expect(cli.Create(ctx, p)).Should(Succeed())
	t.Cleanup(func() { _ = cli.Delete(context.Background(), p) })

	wt := tc.NewWithT(t)
	nn := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	// Step 1: No Dashboard CR → dashboard operand is not ready, so DAG stays blocked at RL10.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "dashboard"}).
		Eventually().Should(Succeed())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())

	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		// ModulesReady blocked — dashboard operand cannot report readiness yet.
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeModulesReady, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .message | contains("monitoring")`,
			status.ConditionTypeModulesReady),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .message | contains("dashboard")`,
			status.ConditionTypeModulesReady),
		// ProvisioningProgress blocked — dashboard operand at RL10 not ready.
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeProvisioningProgress, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .reason == "%s"`,
			status.ConditionTypeProvisioningProgress, status.AwaitingReadinessReason),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .message | contains("dashboard")`,
			status.ConditionTypeProvisioningProgress),
		// Ready=False (real block, not Info severity).
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeReady, metav1.ConditionFalse),
	))

	// Step 2: Create Dashboard CR (no Ready condition yet) — DAG still blocked.
	dashboard := &unstructured.Unstructured{}
	dashboard.SetGroupVersionKind(gvk.Dashboard)
	dashboard.SetName("default-dashboard")
	NewWithT(t).Expect(cli.Create(ctx, dashboard)).Should(Succeed())
	t.Cleanup(func() { _ = cli.Delete(context.Background(), dashboard) })

	// Dashboard exists but has no conditions → operand still not ready → still blocked.
	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeModulesReady, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeProvisioningProgress, metav1.ConditionFalse),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .message | contains("dashboard")`,
			status.ConditionTypeProvisioningProgress),
	))

	// Step 3: Mark Dashboard Ready=True → dashboard operand becomes ready and RL10 clears.
	// ProvisioningProgress flips to True.
	setUnstructuredReady(t, tc, dashboard, true)

	wt.Get(gvk.Platform, nn).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeProvisioningProgress, metav1.ConditionTrue),
	)

	// Step 4: Mark PlatformModule Ready=True → aggregateStatus flips ModulesReady=True.
	pm := &configv1alpha1.PlatformModule{}
	NewWithT(t).Eventually(func() error {
		return cli.Get(ctx, types.NamespacedName{Name: "monitoring"}, pm)
	}).Should(Succeed())
	pm.Status.Conditions = []common.Condition{{
		Type:               status.ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Test",
		LastTransitionTime: metav1.Now(),
	}}
	NewWithT(t).Expect(cli.Status().Update(ctx, pm)).Should(Succeed())

	// ModulesReady=True + Ready=True once both component and module are ready.
	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeModulesReady, metav1.ConditionTrue),
		jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
			status.ConditionTypeReady, metav1.ConditionTrue),
	))
}
