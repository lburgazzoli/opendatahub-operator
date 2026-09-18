package v2_test

import (
	"fmt"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	dscv3 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v3"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules/aigateway"

	. "github.com/onsi/gomega"
)

type maasConversionCase struct {
	canonical operatorv1.ManagementState
	kserve    operatorv1.ManagementState
	legacy    operatorv1.ManagementState
	gateway   operatorv1.ManagementState
}

func maasConversionCases() []maasConversionCase {
	states := []operatorv1.ManagementState{"", operatorv1.Managed, operatorv1.Removed}
	stateCount := len(states)
	caseCount := stateCount * stateCount * stateCount * stateCount
	cases := make([]maasConversionCase, 0, caseCount)

	for caseIndex := range caseCount {
		// Each base-N digit selects one field's state, enumerating every combination.
		var combination [4]operatorv1.ManagementState
		remaining := caseIndex

		for field := range combination {
			combination[field] = states[remaining%stateCount]
			remaining /= stateCount
		}

		cases = append(cases, maasConversionCase{
			canonical: combination[0],
			kserve:    combination[1],
			legacy:    combination[2],
			gateway:   combination[3],
		})
	}

	return cases
}

func TestMaaSConversionMatrix(t *testing.T) {
	for _, tc := range maasConversionCases() {
		name := fmt.Sprintf("canonical=%s/kserve=%s/legacy=%s/gateway=%s", tc.canonical, tc.kserve, tc.legacy, tc.gateway)

		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			source := &dscv2.DataScienceCluster{}
			source.Spec.Components.Kserve.ManagementState = tc.kserve
			source.Spec.Components.Kserve.ModelsAsService.ManagementState = tc.legacy //nolint:staticcheck
			source.Spec.Components.AIGateway.ManagementState = tc.gateway
			source.Spec.Components.AIGateway.ModelsAsAService.ManagementState = tc.canonical
			original := source.DeepCopy()

			// These are the pre-migration handler's three independent decisions.
			projected := tc.canonical
			if projected == "" && tc.kserve == operatorv1.Managed {
				projected = tc.legacy
			}

			parent := tc.gateway
			if parent == "" && tc.kserve == operatorv1.Managed && tc.legacy == operatorv1.Managed {
				parent = operatorv1.Managed
			}

			effectiveParent := parent
			if effectiveParent == "" {
				effectiveParent = operatorv1.Removed
			}

			hub := &dscv3.DataScienceCluster{}
			g.Expect(source.ConvertTo(hub)).To(Succeed())
			g.Expect(source).To(Equal(original))
			g.Expect(hub.Spec.Components.AIGateway.ModelsAsAService.ManagementState).To(Equal(projected))
			g.Expect(hub.Spec.Components.AIGateway.ManagementState).To(Equal(parent))
			g.Expect(hub.ReadMaaSV2State()).To(Equal(tc.legacy == operatorv1.Managed))

			expectMaaSHandlerBehavior(t, hub, projected, effectiveParent)

			// The v2 read retains canonical migration and only restores legacy Managed.
			back := &dscv2.DataScienceCluster{}
			g.Expect(back.ConvertFrom(hub)).To(Succeed())

			expected := original.DeepCopy()
			expected.Spec.Components.AIGateway.ModelsAsAService.ManagementState = projected
			expected.Spec.Components.AIGateway.ManagementState = parent
			if tc.legacy != operatorv1.Managed {
				expected.Spec.Components.Kserve.ModelsAsService.ManagementState = operatorv1.Removed //nolint:staticcheck
			}

			g.Expect(back).To(Equal(expected))

			// Empty legacy state may normalize to Removed on the next forward conversion.
			again := &dscv3.DataScienceCluster{}
			g.Expect(back.ConvertTo(again)).To(Succeed())

			expectedHub := hub.DeepCopy()
			if projected == "" && tc.kserve == operatorv1.Managed {
				expectedHub.Spec.Components.AIGateway.ModelsAsAService.ManagementState = operatorv1.Removed
			}

			g.Expect(again).To(Equal(expectedHub))

			// Further conversions must be stable.
			g.Expect(back.ConvertFrom(again)).To(Succeed())
			stable := &dscv3.DataScienceCluster{}
			g.Expect(back.ConvertTo(stable)).To(Succeed())
			g.Expect(stable).To(Equal(again))
		})
	}
}

func expectMaaSHandlerBehavior(
	t *testing.T,
	dsc *dscv3.DataScienceCluster,
	expectedMaaS operatorv1.ManagementState,
	expectedParent operatorv1.ManagementState,
) {
	t.Helper()
	g := NewWithT(t)

	handler := aigateway.NewHandler()
	dscContext := &modules.DSCContext{DSC: dsc}

	// The module CR receives the selected canonical MaaS state.
	moduleCR, err := handler.BuildModuleCR(t.Context(), nil, dscContext, nil)
	g.Expect(err).NotTo(HaveOccurred())

	maasState, _, err := unstructured.NestedString(moduleCR.Object, "spec", "modelsAsAService", "managementState")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(maasState).To(Equal(string(expectedMaaS)))

	// Readiness tracking follows MaaS, independently of its parent's lifecycle.
	maasCondition := handler.Config.SubmoduleConditions[0]
	g.Expect(maasCondition.StatusFieldName).To(Equal("ModelsAsAService"))
	g.Expect(maasCondition.IsEnabled(dscContext)).To(Equal(expectedMaaS == operatorv1.Managed))

	// Platform module lifecycle follows the separately selected parent state.
	platform := &configv1alpha1.PlatformModules{}
	handler.PopulatePlatformModule(platform, dscContext)

	g.Expect(platform.AIGateway.ManagementState).To(Equal(expectedParent))
}

func TestMaaSConversionIgnoresIncomingProvenance(t *testing.T) {
	g := NewWithT(t)

	source := &dscv2.DataScienceCluster{}
	source.Annotations = map[string]string{dscv3.MaaSV2StateAnnotation: "legacy-managed", "example": "kept"}
	source.Spec.Components.Kserve.ModelsAsService.ManagementState = operatorv1.Removed //nolint:staticcheck

	hub := &dscv3.DataScienceCluster{}
	g.Expect(source.ConvertTo(hub)).To(Succeed())
	g.Expect(hub.Annotations).To(Equal(map[string]string{"example": "kept"}))

	back := &dscv2.DataScienceCluster{}
	g.Expect(back.ConvertFrom(hub)).To(Succeed())
	g.Expect(back.Spec.Components.Kserve.ModelsAsService.ManagementState).To(Equal(operatorv1.Removed)) //nolint:staticcheck

	hub.Annotations[dscv3.MaaSV2StateAnnotation] = "unsupported"
	g.Expect(back.ConvertFrom(hub)).To(MatchError(ContainSubstring("provenance marker")))
}

func TestMaaSConversionNormalizesEmptyLegacyState(t *testing.T) {
	g := NewWithT(t)

	source := &dscv2.DataScienceCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: dscv2.GroupVersion.String(), Kind: "DataScienceCluster"},
	}
	hub := &dscv3.DataScienceCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: dscv3.GroupVersion.String(), Kind: "DataScienceCluster"},
	}
	g.Expect(source.ConvertTo(hub)).To(Succeed())
	g.Expect(hub.ReadMaaSV2State()).To(BeFalse())

	back := &dscv2.DataScienceCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: dscv2.GroupVersion.String(), Kind: "DataScienceCluster"},
	}
	g.Expect(back.ConvertFrom(hub)).To(Succeed())

	expected := source.DeepCopy()
	expected.Spec.Components.Kserve.ModelsAsService.ManagementState = operatorv1.Removed //nolint:staticcheck
	g.Expect(back).To(Equal(expected))
}
