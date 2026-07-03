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

	createPlatform(t, tc, configv1alpha1.PlatformSpec{
		Modules: configv1alpha1.PlatformModules{
			Monitoring: common.ManagementSpec{ManagementState: operatorv1.Managed},
			AIGateway:  common.ManagementSpec{ManagementState: operatorv1.Managed},
		},
	})

	wt := tc.NewWithT(t)
	nn := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	// Both PlatformModule CRs should be created.
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(Succeed())

	// monitoring (RL10) becomes Ready=True first (no manifests, Info-severity
	// conditions only). Then walkModuleDAG clears RL10 and aigateway (RL20)
	// becomes Ready=True as well. Final state: ModulesReady=True, Ready=True.
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
