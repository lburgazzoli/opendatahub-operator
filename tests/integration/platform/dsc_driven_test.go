package platform_test

import (
	"context"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	rrtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"

	. "github.com/onsi/gomega"
)

func TestDSCDriven_ComponentsAndModules_Installed(t *testing.T) {
	moduleReg := modules.NewRegistry()
	moduleReg.Add(newTestModuleHandler("aigateway", testModuleAGVK))

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

func TestDSCDriven_PlatformReflectsDSC(t *testing.T) {
	_, tc := startAllControllers(t, suiteOpts{
		moduleReg:    modules.NewRegistry(),
		componentReg: &cr.Registry{},
		provisionReg: provision.NewRegistry(),
	})

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
