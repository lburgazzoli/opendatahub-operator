package platform_test

import (
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"

	. "github.com/onsi/gomega"
)

func managedMonitoringSpec() dsciv2.DSCInitializationSpec {
	return dsciv2.DSCInitializationSpec{
		ApplicationsNamespace: "default",
		Monitoring: serviceApi.DSCIMonitoring{
			ManagementSpec: common.ManagementSpec{ManagementState: operatorv1.Managed},
			MonitoringCommonSpec: serviceApi.MonitoringCommonSpec{
				Namespace: "monitoring",
			},
		},
	}
}

func TestDSCIDriven_PlatformReflectsDSCI(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newMonitoringModuleHandler())

	_, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: &cr.Registry{},
		provisionReg: provision.NewRegistry(),
		startDSCI:    true,
	})

	createGatewayConfig(t, tc)
	createDSCIWithSpec(t, tc, managedMonitoringSpec())

	wt := tc.NewWithT(t)
	wt.Get(gvk.Platform, types.NamespacedName{Name: configv1alpha1.PlatformInstanceName}).Eventually().Should(
		jq.Match(`.spec.modules[] | select(.name == "monitoring") | .managementState == "Managed"`),
	)
}

func TestDSCIDriven_ServiceModuleCreated(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newMonitoringModuleHandler())

	_, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: &cr.Registry{},
		provisionReg: provision.NewRegistry(),
		startDSCI:    true,
	})

	createGatewayConfig(t, tc)
	createDSCIWithSpec(t, tc, managedMonitoringSpec())

	wt := tc.NewWithT(t)
	cli := tc.Client()

	wt.Get(gvk.Monitoring, types.NamespacedName{Name: serviceApi.MonitoringInstanceName}).
		Eventually().Should(Succeed())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())

	wt.Eventually(func(g Gomega) {
		dsci := &dsciv2.DSCInitialization{}
		g.Expect(cli.Get(t.Context(), types.NamespacedName{Name: "default-dsci"}, dsci)).To(Succeed())

		monitoringCR := &serviceApi.Monitoring{}
		g.Expect(cli.Get(t.Context(), types.NamespacedName{Name: serviceApi.MonitoringInstanceName}, monitoringCR)).To(Succeed())
		g.Expect(metav1.IsControlledBy(monitoringCR, dsci)).To(BeTrue())
	}).Should(Succeed())
}

func TestDSCIDriven_DisableModule_Cleanup(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newMonitoringModuleHandler())

	_, tc := startAllControllers(t, suiteOpts{
		moduleReg:    moduleReg,
		componentReg: &cr.Registry{},
		provisionReg: provision.NewRegistry(),
		startDSCI:    true,
	})

	createGatewayConfig(t, tc)
	createDSCIWithSpec(t, tc, managedMonitoringSpec())

	wt := tc.NewWithT(t)
	cli := tc.Client()

	wt.Get(gvk.Monitoring, types.NamespacedName{Name: serviceApi.MonitoringInstanceName}).
		Eventually().Should(Succeed())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(Succeed())

	dsci := &dsciv2.DSCInitialization{}
	wt.Expect(cli.Get(t.Context(), types.NamespacedName{Name: "default-dsci"}, dsci)).To(Succeed())
	dsci.Spec.Monitoring.ManagementState = operatorv1.Removed
	wt.Expect(cli.Update(t.Context(), dsci)).To(Succeed())

	wt.Get(gvk.Monitoring, types.NamespacedName{Name: serviceApi.MonitoringInstanceName}).
		Eventually().Should(BeNil())
	wt.Get(gvk.PlatformModule, types.NamespacedName{Name: "monitoring"}).
		Eventually().Should(BeNil())
}
