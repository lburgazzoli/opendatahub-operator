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
	// Use a component at RL10 as a gate so we can observe the DAG blocked,
	// then manually advance it and verify the module at RL20 proceeds.
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

	createPlatform(t, tc, configv1alpha1.PlatformSpec{
		Modules: configv1alpha1.PlatformModules{
			Monitoring: common.ManagementSpec{ManagementState: operatorv1.Managed},
		},
	})

	wt := tc.NewWithT(t)
	cli := tc.Client()
	nn := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	// Step 1: PlatformModule created, but DAG blocked at RL10 (no Dashboard CR).
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())

	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "False"`),
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .message | contains("dashboard")`),
	))

	// Step 2: Create Dashboard CR and mark it Ready → RL10 clears → RL20 unblocked.
	dashboard := &unstructured.Unstructured{}
	dashboard.SetGroupVersionKind(gvk.Dashboard)
	dashboard.SetName("default-dashboard")
	NewWithT(t).Expect(cli.Create(context.Background(), dashboard)).Should(Succeed())
	t.Cleanup(func() { _ = cli.Delete(context.Background(), dashboard) })

	setUnstructuredReady(t, cli, dashboard, true)

	// Step 3: ProvisioningProgress advances to True.
	wt.Get(gvk.Platform, nn).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "True"`),
	)

	// Step 4: monitoring PlatformModule becomes Ready (auto — no manifests),
	// Platform reaches ModulesReady=True.
	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
		jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
	))
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

	createPlatform(t, tc, configv1alpha1.PlatformSpec{
		Modules: configv1alpha1.PlatformModules{
			Monitoring: common.ManagementSpec{ManagementState: operatorv1.Managed},
		},
	})

	wt := tc.NewWithT(t)
	cli := tc.Client()
	nn := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	// Step 1: PlatformModule created but DAG blocked — no Dashboard CR.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())

	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "False"`),
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .message | contains("dashboard")`),
	))

	// Step 2: Create Dashboard CR (no Ready condition) — still blocked.
	dashboard := &unstructured.Unstructured{}
	dashboard.SetGroupVersionKind(gvk.Dashboard)
	dashboard.SetName("default-dashboard")
	NewWithT(t).Expect(cli.Create(context.Background(), dashboard)).Should(Succeed())
	t.Cleanup(func() { _ = cli.Delete(context.Background(), dashboard) })

	// Step 3: Mark Dashboard Ready=True → RL10 clears.
	setUnstructuredReady(t, cli, dashboard, true)

	wt.Get(gvk.Platform, nn).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "True"`),
	)

	// Step 4: Mark monitoring PlatformModule Ready → ModulesReady=True.
	setPlatformModuleReady(t, cli, "monitoring", true)

	wt.Get(gvk.Platform, nn).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
		jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
	))
}
