package platform_test

import (
	"context"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
	prom "github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/prometheus"

	. "github.com/onsi/gomega"
)

func TestPlatformOnly_TwoModules_Created(t *testing.T) {
	_, tc := startAllControllers(t, suiteOpts{
		moduleReg:    modules.NewRegistry(),
		componentReg: &cr.Registry{},
		provisionReg: provision.NewRegistry(),
	})

	createGatewayConfig(t, tc)

	createPlatform(t, tc, configv1alpha1.PlatformSpec{
		Modules: configv1alpha1.PlatformModules{
			Monitoring: common.ManagementSpec{ManagementState: operatorv1.Managed},
			AIGateway:  common.ManagementSpec{ManagementState: operatorv1.Managed},
		},
	})

	wt := tc.NewWithT(t)
	nn := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(jq.Match(`.metadata.name == "monitoring"`))

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(jq.Match(`.metadata.name == "aigateway"`))

	wt.Get(gvk.Platform, nn).Eventually().Should(
		jq.Match(`.status.modules | sort == ["aigateway","monitoring"]`),
	)
}

func TestPlatformOnly_DAG_Advancement(t *testing.T) {
	// Two modules at different runlevels. Each module's operand CR is created
	// upfront with no conditions, which triggers OperandInitializing (blocking).
	// This lets us manually control when each module becomes Ready and verify
	// that the DAG advances level by level.
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newTestModuleHandler("monitoring", testModuleAGVK))
	moduleReg.Add(newTestModuleHandler("aigateway", testModuleBGVK))

	provisionReg := provision.NewRegistry()
	provisionReg.Add("monitoring", provision.KindModule, dag.RL(10))
	provisionReg.Add("aigateway", provision.KindModule, dag.RL(20))
	provisionReg.Enable("monitoring")
	provisionReg.Enable("aigateway")

	et, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: &cr.Registry{},
		provisionReg: provisionReg,
	})

	registerModuleCRD(t, et, testModuleAGVK)
	registerModuleCRD(t, et, testModuleBGVK)
	createGatewayConfig(t, tc)
	resetDAGMetrics()

	cli := tc.Client()

	// Pre-create module operand CRs with no conditions. syncModuleCRStatus
	// sees "CR exists, zero conditions" → OperandInitializing → blocks Ready.
	monitoringCR := &unstructured.Unstructured{}
	monitoringCR.SetGroupVersionKind(testModuleAGVK)
	monitoringCR.SetName("default-monitoring")
	NewWithT(t).Expect(cli.Create(t.Context(), monitoringCR)).Should(Succeed())
	t.Cleanup(func() { _ = cli.Delete(context.Background(), monitoringCR) })

	aigateCR := &unstructured.Unstructured{}
	aigateCR.SetGroupVersionKind(testModuleBGVK)
	aigateCR.SetName("default-aigateway")
	NewWithT(t).Expect(cli.Create(t.Context(), aigateCR)).Should(Succeed())
	t.Cleanup(func() { _ = cli.Delete(context.Background(), aigateCR) })

	createPlatform(t, tc, configv1alpha1.PlatformSpec{
		Modules: configv1alpha1.PlatformModules{
			Monitoring: common.ManagementSpec{ManagementState: operatorv1.Managed},
			AIGateway:  common.ManagementSpec{ManagementState: operatorv1.Managed},
		},
	})

	wt := tc.NewWithT(t)
	g := NewWithT(t)
	nn := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}
	batchesBefore := captureBatchCount()

	// Step 1: Both PlatformModule CRs created; neither is Ready yet
	// (OperandInitializing blocks). ModulesReady=False listing both.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(Succeed())

	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "False"`),
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .reason == "NotReady"`),
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .message | contains("monitoring")`),
	))

	// Metrics: RL10 processed, RL20 blocked — DAG stuck at runlevel boundary.
	g.Eventually(func(g Gomega) {
		g.Expect(provision.RunlevelStatus).To(And(
			prom.HaveGaugeVecValue(1, "10", provision.StatusProcessed),
			prom.HaveGaugeVecValue(0, "10", provision.StatusBlocked),
			prom.HaveGaugeVecValue(1, "20", provision.StatusBlocked),
			prom.HaveGaugeVecValue(0, "20", provision.StatusProcessed),
		))
		g.Expect(provision.RunlevelCleared).To(prom.HaveValue(10))
		g.Expect(provision.RunlevelBlocked).To(prom.HaveValue(20))
	}).Should(Succeed())

	// Step 2: Mark monitoring operand CR Ready → monitoring PlatformModule
	// becomes Ready → walkModuleDAG clears RL10.
	setUnstructuredReady(t, cli, monitoringCR, true)

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(
			jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
		)

	// aigateway PlatformModule should still NOT be Ready: its operand CR
	// has no conditions → OperandInitializing blocks Ready.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(And(
			jq.Match(`.status.conditions[] | select(.type == "OperandAvailable") | .status == "False"`),
			jq.Match(`.status.conditions[] | select(.type == "OperandAvailable") | .reason == "OperandInitializing"`),
			jq.Match(`.status.conditions[] | select(.type == "OperandAvailable") | .message == "module CR has no conditions yet"`),
		))

	// Metrics: RL20 should now be processed (monitoring Ready unblocked it).
	g.Eventually(func(g Gomega) {
		g.Expect(provision.RunlevelStatus).To(And(
			prom.HaveGaugeVecValue(1, "20", provision.StatusProcessed),
			prom.HaveGaugeVecValue(0, "20", provision.StatusBlocked),
		))
		g.Expect(provision.RunlevelCleared).To(prom.HaveValue(20))
		g.Expect(provision.RunlevelBlocked).To(prom.HaveValue(0))
	}).Should(Succeed())

	// Step 3: Mark aigateway operand CR Ready → aigateway PlatformModule
	// becomes Ready → ModulesReady=True.
	setUnstructuredReady(t, cli, aigateCR, true)

	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
		jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
	))

	// Metrics: final state — fully advanced, nothing blocked.
	g.Eventually(func(g Gomega) {
		g.Expect(provision.RunlevelCleared).To(prom.HaveValue(20))
		g.Expect(provision.RunlevelBlocked).To(prom.HaveValue(0))
		g.Expect(captureBatchCount() - batchesBefore).To(BeNumerically(">=", 2))
	}).Should(Succeed())
}

func TestPlatformOnly_DisableModule_Cleanup(t *testing.T) {
	_, tc := startAllControllers(t, suiteOpts{
		moduleReg:    modules.NewRegistry(),
		componentReg: &cr.Registry{},
		provisionReg: provision.NewRegistry(),
	})

	createGatewayConfig(t, tc)

	createPlatform(t, tc, configv1alpha1.PlatformSpec{
		Modules: configv1alpha1.PlatformModules{
			Monitoring: common.ManagementSpec{ManagementState: operatorv1.Managed},
			AIGateway:  common.ManagementSpec{ManagementState: operatorv1.Managed},
		},
	})

	wt := tc.NewWithT(t)
	cli := tc.Client()

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(Succeed())

	// Disable monitoring by updating Platform spec.
	p := &configv1alpha1.Platform{}
	NewWithT(t).Expect(cli.Get(t.Context(),
		types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}, p)).Should(Succeed())
	p.Spec.Modules.Monitoring = common.ManagementSpec{ManagementState: operatorv1.Removed}
	NewWithT(t).Expect(cli.Update(t.Context(), p)).Should(Succeed())

	// monitoring PlatformModule should be deleted.
	NewWithT(t).Eventually(func() error {
		return cli.Get(t.Context(),
			types.NamespacedName{Name: "monitoring"}, &configv1alpha1.PlatformModule{})
	}).Should(MatchError(ContainSubstring("not found")))

	// aigateway PlatformModule should still exist.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(Succeed())

	// Platform status.modules should only contain aigateway.
	wt.Get(gvk.Platform, types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}).
		Eventually().Should(
			jq.Match(`.status.modules == ["aigateway"]`),
		)
}

func TestPlatformOnly_DAG_Gating_ComponentBlocksModule(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newTestModuleHandler("monitoring", testModuleAGVK))

	componentReg := &cr.Registry{}
	componentReg.Add(&cr.BaseComponentHandler{
		Name: "dashboard",
		GVK:  gvk.Dashboard,
	})

	provisionReg := provision.NewRegistry()
	provisionReg.Add("dashboard", provision.KindComponent, dag.RL(10))
	provisionReg.Add("monitoring", provision.KindModule, dag.RL(20))
	provisionReg.Enable("dashboard")
	provisionReg.Enable("monitoring")

	et, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: componentReg,
		provisionReg: provisionReg,
	})

	registerModuleCRD(t, et, testModuleAGVK)
	createGatewayConfig(t, tc)
	resetDAGMetrics()

	createPlatform(t, tc, configv1alpha1.PlatformSpec{
		Modules: configv1alpha1.PlatformModules{
			Monitoring: common.ManagementSpec{ManagementState: operatorv1.Managed},
		},
	})

	wt := tc.NewWithT(t)
	g := NewWithT(t)
	cli := tc.Client()
	nn := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}
	batchesBefore := captureBatchCount()

	// Step 1: PlatformModule created but DAG blocked at RL10 — no Dashboard CR.
	// Note: ModulesReady and Ready may already be True because the gating
	// condition (PlatformReady) uses Info severity, which does not block the
	// PlatformModule's Ready computation. Only ProvisioningProgress reflects
	// the DAG gating state on the Platform CR.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())

	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "False"`),
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .reason == "AwaitingReadiness"`),
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .message | contains("dashboard")`),
	))

	// Metrics: RL10 processed, RL20 blocked — DAG stuck at runlevel boundary.
	g.Eventually(func(g Gomega) {
		g.Expect(provision.RunlevelStatus).To(And(
			prom.HaveGaugeVecValue(1, "10", provision.StatusProcessed),
			prom.HaveGaugeVecValue(0, "10", provision.StatusBlocked),
			prom.HaveGaugeVecValue(1, "20", provision.StatusBlocked),
			prom.HaveGaugeVecValue(0, "20", provision.StatusProcessed),
		))
		g.Expect(provision.RunlevelCleared).To(prom.HaveValue(10))
		g.Expect(provision.RunlevelBlocked).To(prom.HaveValue(20))
	}).Should(Succeed())

	// Step 2: Create Dashboard CR (no Ready condition) — still blocked.
	dashboard := &unstructured.Unstructured{}
	dashboard.SetGroupVersionKind(gvk.Dashboard)
	dashboard.SetName("default-dashboard")
	g.Expect(cli.Create(t.Context(), dashboard)).Should(Succeed())
	t.Cleanup(func() { _ = cli.Delete(context.Background(), dashboard) })

	// Step 3: Mark Dashboard Ready=True → RL10 clears, RL20 unblocks.
	setUnstructuredReady(t, cli, dashboard, true)

	wt.Get(gvk.Platform, nn).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "True"`),
	)

	// Metrics: RL20 transitioned from blocked → processed.
	g.Eventually(func(g Gomega) {
		g.Expect(provision.RunlevelStatus).To(And(
			prom.HaveGaugeVecValue(1, "20", provision.StatusProcessed),
			prom.HaveGaugeVecValue(0, "20", provision.StatusBlocked),
		))
		g.Expect(provision.RunlevelCleared).To(prom.HaveValue(20))
		g.Expect(provision.RunlevelBlocked).To(prom.HaveValue(0))
	}).Should(Succeed())

	// Step 4: Mark monitoring PlatformModule Ready → ModulesReady=True.
	setPlatformModuleReady(t, cli, "monitoring", true)

	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
		jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
	))

	// Metrics: final state — fully advanced.
	g.Eventually(func(g Gomega) {
		g.Expect(captureBatchCount() - batchesBefore).To(BeNumerically(">=", 2))
	}).Should(Succeed())
}
