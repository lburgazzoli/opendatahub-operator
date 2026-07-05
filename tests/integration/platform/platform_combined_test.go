package platform_test

import (
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"

	. "github.com/onsi/gomega"
)

func managedAIGatewaySpec() dscv2.DataScienceClusterSpec {
	return dscv2.DataScienceClusterSpec{
		Components: dscv2.Components{
			AIGateway: componentApi.DSCAIGateway{
				ManagementSpec: common.ManagementSpec{ManagementState: operatorv1.Managed},
			},
		},
	}
}

func TestCombined_DSCAndDSCI_ModulesCombined(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newMonitoringModuleHandler())
	moduleReg.Add(newAIGatewayModuleHandler(testModuleAGVK))

	et, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: &cr.Registry{},
		provisionReg: provision.NewRegistry(),
		startDSCI:    true,
	})

	registerModuleCRD(t, et, testModuleAGVK)
	createGatewayConfig(t, tc)
	createDSCIWithSpec(t, tc, managedMonitoringSpec())
	createDSC(t, tc, managedAIGatewaySpec())

	wt := tc.NewWithT(t)
	platformKey := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	wt.Get(gvk.Platform, platformKey).Eventually().Should(And(
		jq.Match(`.spec.modules.monitoring.managementState == "Managed"`),
		jq.Match(`.spec.modules.aigateway.managementState == "Managed"`),
	))

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(Succeed())
}

func TestCombined_DAG_DSCIModuleGatesDSCModule(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newMonitoringModuleHandler())
	moduleReg.Add(newAIGatewayModuleHandler(testModuleAGVK))

	provisionReg := provision.NewRegistry()
	provisionReg.Add("monitoring", provision.KindModule, dag.RL(10))
	provisionReg.Add("aigateway", provision.KindModule, dag.RL(20))
	provisionReg.Enable("monitoring")
	provisionReg.Enable("aigateway")

	et, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: &cr.Registry{},
		provisionReg: provisionReg,
		startDSCI:    true,
	})

	registerModuleCRD(t, et, testModuleAGVK)
	createGatewayConfig(t, tc)
	createDSCIWithSpec(t, tc, managedMonitoringSpec())
	createDSC(t, tc, managedAIGatewaySpec())

	wt := tc.NewWithT(t)
	cli := tc.Client()
	platformKey := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "aigateway"}).
		Eventually().Should(Succeed())

	wt.Get(gvk.Platform, platformKey).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "False"`),
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .message | contains("monitoring")`),
	))

	monitoringCR := &unstructured.Unstructured{}
	monitoringCR.SetGroupVersionKind(gvk.Monitoring)
	monitoringCR.SetName(serviceApi.MonitoringInstanceName)
	setUnstructuredReady(t, cli, monitoringCR, true)

	wt.Get(gvk.Platform, platformKey).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "ProvisioningProgress") | .status == "True"`),
	)

	aigatewayCR := &unstructured.Unstructured{}
	aigatewayCR.SetGroupVersionKind(testModuleAGVK)
	aigatewayCR.SetName("default-aigateway")
	setUnstructuredReady(t, cli, aigatewayCR, true)

	wt.Get(gvk.Platform, platformKey).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
		jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "True"`),
	))
}

func TestCombined_StatusAggregation(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newMonitoringModuleHandler())
	moduleReg.Add(newAIGatewayModuleHandler(testModuleAGVK))

	et, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: &cr.Registry{},
		provisionReg: provision.NewRegistry(),
		startDSCI:    true,
	})

	registerModuleCRD(t, et, testModuleAGVK)
	createGatewayConfig(t, tc)
	createDSCIWithSpec(t, tc, managedMonitoringSpec())
	createDSC(t, tc, managedAIGatewaySpec())

	wt := tc.NewWithT(t)
	cli := tc.Client()
	dsciKey := types.NamespacedName{Name: "default-dsci"}
	dscKey := types.NamespacedName{Name: "default-dsc"}
	platformKey := types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}

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

	aigatewayCR := &unstructured.Unstructured{}
	aigatewayCR.SetGroupVersionKind(testModuleAGVK)
	aigatewayCR.SetName("default-aigateway")
	setUnstructuredReady(t, cli, aigatewayCR, true)

	wt.Get(gvk.DSCInitialization, dsciKey).Eventually().Should(And(
		jq.Match(`.status.conditions[] | select(.type == "Ready") | .status == "False"`),
		jq.Match(`.status.conditions[] | select(.type == "MonitoringReady") | .status == "True"`),
	))

	wt.Get(gvk.DataScienceCluster, dscKey).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "True"`),
	)

	wt.Get(gvk.Platform, platformKey).Eventually().Should(
		jq.Match(`.status.conditions[] | select(.type == "ModulesReady") | .status == "False"`),
	)
}
