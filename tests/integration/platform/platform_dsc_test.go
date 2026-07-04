package platform_test

import (
	"context"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	rrtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
	prom "github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/prometheus"

	. "github.com/onsi/gomega"
)

func TestDSCDriven_ComponentsAndModules_Installed(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newAIGatewayModuleHandler(testModuleAGVK))

	componentReg := &cr.Registry{}
	componentReg.Add(&cr.BaseComponentHandler{
		Name: "dashboard",
		GVK:  gvk.Dashboard,
		IsEnabledFn: func(_ *dscv2.DataScienceCluster) bool { return true },
		NewCRObjectFn: func(_ context.Context, _ client.Client, _ *dscv2.DataScienceCluster) (common.PlatformObject, error) {
			return &componentApi.Dashboard{
				ObjectMeta: metav1.ObjectMeta{Name: "default-dashboard"},
			}, nil
		},
		UpdateDSCStatusFn: func(_ context.Context, _ *rrtypes.ReconciliationRequest) (metav1.ConditionStatus, error) {
			return metav1.ConditionTrue, nil
		},
	})

	et, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: componentReg,
		provisionReg: provision.NewRegistry(),
	})

	registerModuleCRD(t, et, testModuleAGVK)
	createGatewayConfig(t, tc)
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

	wt := tc.NewWithT(t)
	platformKey := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	// Component CR created by DSC's provisionComponents.
	wt.Get(gvk.Dashboard, types.NamespacedName{Name: "default-dashboard"}).
		Eventually().Should(Succeed())

	// Module operand CR created by DSC's provisionModuleCRs.
	wt.Get(testModuleAGVK, types.NamespacedName{Name: "default-aigateway"}).
		Eventually().Should(Succeed())

	// Platform CR synced with aigateway=Managed.
	wt.Get(gvk.Platform, platformKey).Eventually().Should(
		jq.Match(`.spec.modules.aigateway.managementState == "Managed"`),
	)

	// PlatformModule CR created by Platform controller.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(Succeed())
}

func TestDSCDriven_StatusAggregation(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newAIGatewayModuleHandler(testModuleAGVK))

	componentReg := &cr.Registry{}
	componentReg.Add(&cr.BaseComponentHandler{
		Name: "dashboard",
		GVK:  gvk.Dashboard,
		IsEnabledFn: func(_ *dscv2.DataScienceCluster) bool { return true },
		NewCRObjectFn: func(_ context.Context, _ client.Client, _ *dscv2.DataScienceCluster) (common.PlatformObject, error) {
			return &componentApi.Dashboard{
				ObjectMeta: metav1.ObjectMeta{Name: "default-dashboard"},
			}, nil
		},
		UpdateDSCStatusFn: func(_ context.Context, _ *rrtypes.ReconciliationRequest) (metav1.ConditionStatus, error) {
			return metav1.ConditionTrue, nil
		},
	})

	et, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: componentReg,
		provisionReg: provision.NewRegistry(),
	})

	registerModuleCRD(t, et, testModuleAGVK)
	createGatewayConfig(t, tc)
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

	wt := tc.NewWithT(t)
	g := NewWithT(t)
	cli := tc.Client()
	dscKey := types.NamespacedName{Name: "default-dsc"}

	// Phase 1: ComponentsReady=True (dashboard returns ConditionTrue),
	// ModulesReady=False (module CR has no Ready condition yet).
	wt.Get(gvk.DataScienceCluster, dscKey).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ComponentsReady") | .status == "True"`),
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "False"`),
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .reason == "NotReady"`),
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .message | contains("aigateway")`),
	))

	// Phase 2: Patch module operand CR with Ready=True.
	moduleCR := &unstructured.Unstructured{}
	moduleCR.SetGroupVersionKind(testModuleAGVK)
	g.Eventually(func() error {
		return cli.Get(t.Context(), types.NamespacedName{Name: "default-aigateway"}, moduleCR)
	}).Should(Succeed())

	setUnstructuredReady(t, cli, moduleCR, true)

	// Trigger DSC re-reconcile so updateStatus picks up the new CR status.
	latestDSC := &dscv2.DataScienceCluster{}
	g.Expect(cli.Get(t.Context(), dscKey, latestDSC)).Should(Succeed())
	if latestDSC.Annotations == nil {
		latestDSC.Annotations = map[string]string{}
	}
	g.Expect(cli.Update(t.Context(), latestDSC)).Should(Succeed())

	wt.Get(gvk.DataScienceCluster, dscKey).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
	)
}

func TestDSCDriven_DAG_Advancement(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newAIGatewayModuleHandler(testModuleAGVK))

	componentReg := &cr.Registry{}
	componentReg.Add(&cr.BaseComponentHandler{
		Name: "dashboard",
		GVK:  gvk.Dashboard,
	})

	provisionReg := provision.NewRegistry()
	provisionReg.Add("dashboard", provision.KindComponent, dag.RL(10))
	provisionReg.Add("aigateway", provision.KindModule, dag.RL(20))
	provisionReg.Enable("dashboard")
	provisionReg.Enable("aigateway")

	et, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: componentReg,
		provisionReg: provisionReg,
	})

	registerModuleCRD(t, et, testModuleAGVK)
	createGatewayConfig(t, tc)
	createDSCI(t, tc)
	resetDAGMetrics()

	createDSC(t, tc, dscv2.DataScienceClusterSpec{
		Components: dscv2.Components{
			AIGateway: componentApi.DSCAIGateway{
				ManagementSpec: common.ManagementSpec{
					ManagementState: operatorv1.Managed,
				},
			},
		},
	})

	wt := tc.NewWithT(t)
	g := NewWithT(t)
	cli := tc.Client()
	nn := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}
	batchesBefore := captureBatchCount()

	// Step 1: PlatformModule created but DAG blocked at RL10 — no Dashboard CR.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
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

	// Step 2: Create Dashboard CR and mark Ready=True → RL10 clears.
	dashboard := &unstructured.Unstructured{}
	dashboard.SetGroupVersionKind(gvk.Dashboard)
	dashboard.SetName("default-dashboard")
	g.Expect(cli.Create(t.Context(), dashboard)).Should(Succeed())
	t.Cleanup(func() { _ = cli.Delete(context.Background(), dashboard) })

	setUnstructuredReady(t, cli, dashboard, true)

	// Step 3: ProvisioningProgress=True → RL10 cleared, RL20 unblocks.
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

	// Step 4: aigateway PlatformModule still not ready — DSC created the module
	// operand CR with no conditions → OperandInitializing blocks.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(And(
			jq.Match(`.status.conditions[] | select(.type == "OperandAvailable") | .status == "False"`),
			jq.Match(`.status.conditions[] | select(.type == "OperandAvailable") | .reason == "OperandInitializing"`),
			jq.Match(`.status.conditions[] | select(.type == "OperandAvailable") | .message == "module CR has no conditions yet"`),
		))

	// Step 5: Mark module operand CR Ready → PlatformModule Ready → ModulesReady=True.
	moduleCR := &unstructured.Unstructured{}
	moduleCR.SetGroupVersionKind(testModuleAGVK)
	g.Eventually(func() error {
		return cli.Get(t.Context(), types.NamespacedName{Name: "default-aigateway"}, moduleCR)
	}).Should(Succeed())

	setUnstructuredReady(t, cli, moduleCR, true)

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

func TestDSCDriven_DAG_Gating_ModuleBlocksModule(t *testing.T) {
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

	// Phase 1: Both PlatformModule CRs created.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(Succeed())

	// Metrics: at least RL10 has been processed (first batch always runs).
	g.Eventually(func(g Gomega) {
		g.Expect(provision.RunlevelStatus).To(prom.HaveGaugeVecValue(1, "10", provision.StatusProcessed))
		g.Expect(provision.RunlevelStatus).To(prom.HaveGaugeVecValue(0, "20", provision.StatusProcessed))
	}).Should(Succeed())

	// Phase 2: monitoring at RL10 becomes Ready=True (no manifests, module CR
	// absent → OperandAbsent+Info → Ready=True).
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(And(
			jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
			jq.Match(`.status.conditions[] | select(.type == "OperandAvailable") | .reason == "OperandAbsent"`),
		))

	// Metrics: monitoring Ready unblocks RL20.
	g.Eventually(func(g Gomega) {
		g.Expect(provision.RunlevelCleared).To(prom.HaveValueWith(">=", 10))
		g.Expect(provision.RunlevelStatus).To(prom.HaveGaugeVecValue(0, "20", provision.StatusPending))
	}).Should(Succeed())

	// Phase 3: aigateway at RL20 unblocked, also becomes Ready=True.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(And(
			jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
			jq.Match(`.status.conditions[] | select(.type == "OperandAvailable") | .reason == "OperandAbsent"`),
		))

	// Metrics: RL20 processed, DAG fully cleared.
	g.Eventually(func(g Gomega) {
		g.Expect(provision.RunlevelStatus).To(prom.HaveGaugeVecValue(1, "20", provision.StatusProcessed))
		g.Expect(provision.RunlevelCleared).To(prom.HaveValue(20))
		g.Expect(provision.RunlevelBlocked).To(prom.HaveValue(0))
	}).Should(Succeed())

	// Phase 4: Platform ModulesReady=True, Ready=True.
	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
		jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
	))

	g.Eventually(func(g Gomega) {
		g.Expect(captureBatchCount() - batchesBefore).To(BeNumerically(">=", 2))
	}).Should(Succeed())
}

func TestDSCDriven_DisableComponent_Cleanup(t *testing.T) {
	componentReg := &cr.Registry{}
	componentReg.Add(&cr.BaseComponentHandler{
		Name: "dashboard",
		GVK:  gvk.Dashboard,
		IsEnabledFn: func(dsc *dscv2.DataScienceCluster) bool {
			return dsc.Spec.Components.Dashboard.ManagementState == operatorv1.Managed
		},
		NewCRObjectFn: func(_ context.Context, _ client.Client, _ *dscv2.DataScienceCluster) (common.PlatformObject, error) {
			return &componentApi.Dashboard{
				ObjectMeta: metav1.ObjectMeta{Name: "default-dashboard"},
			}, nil
		},
		UpdateDSCStatusFn: func(_ context.Context, _ *rrtypes.ReconciliationRequest) (metav1.ConditionStatus, error) {
			return metav1.ConditionTrue, nil
		},
	})

	_, tc := startAllControllers(t, suiteOpts{
		moduleReg:    modules.NewRegistry(),
		componentReg: componentReg,
		provisionReg: provision.NewRegistry(),
	})

	createGatewayConfig(t, tc)
	createDSCI(t, tc)

	createDSC(t, tc, dscv2.DataScienceClusterSpec{
		Components: dscv2.Components{
			Dashboard: componentApi.DSCDashboard{
				ManagementSpec: common.ManagementSpec{ManagementState: operatorv1.Managed},
			},
		},
	})

	wt := tc.NewWithT(t)
	g := NewWithT(t)
	cli := tc.Client()

	// Dashboard CR created by DSC.
	wt.Get(gvk.Dashboard, types.NamespacedName{Name: "default-dashboard"}).
		Eventually().Should(Succeed())

	// Disable Dashboard by updating DSC.
	dsc := &dscv2.DataScienceCluster{}
	g.Expect(cli.Get(t.Context(),
		types.NamespacedName{Name: "default-dsc"}, dsc)).Should(Succeed())
	dsc.Spec.Components.Dashboard.ManagementState = operatorv1.Removed
	g.Expect(cli.Update(t.Context(), dsc)).Should(Succeed())

	// Dashboard CR should be deleted.
	g.Eventually(func() error {
		return cli.Get(t.Context(),
			types.NamespacedName{Name: "default-dashboard"}, &componentApi.Dashboard{})
	}).Should(MatchError(ContainSubstring("not found")))
}

func TestDSCDriven_PlatformReflectsDSC(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newAIGatewayModuleHandler(testModuleAGVK))

	et, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: &cr.Registry{},
		provisionReg: provision.NewRegistry(),
	})

	registerModuleCRD(t, et, testModuleAGVK)

	createGatewayConfig(t, tc)
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

	wt := tc.NewWithT(t)
	platformKey := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	wt.Get(gvk.Platform, platformKey).Eventually().Should(
		jq.Match(`.spec.modules.aigateway.managementState == "Managed"`),
	)
}
