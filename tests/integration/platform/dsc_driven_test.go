package platform_test

import (
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
)

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
