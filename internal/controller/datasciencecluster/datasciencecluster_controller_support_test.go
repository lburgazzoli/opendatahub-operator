//nolint:testpackage
package datasciencecluster

import (
	"encoding/json"
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/mocks"

	. "github.com/onsi/gomega"
)

// newMock creates a MockComponentHandler with only the methods exercised by
// computeComponentsStatus: GetName, IsEnabled, UpdateDSCStatus.
func newMock(name string, enabled bool, cs metav1.ConditionStatus) *mocks.MockComponentHandler {
	return mocks.NewMockComponentHandler(func(m *mocks.MockComponentHandler) {
		m.On("GetName").Return(name)
		m.On("IsEnabled", mock.Anything).Return(enabled)
		m.On("UpdateDSCStatus", mock.Anything, mock.Anything).Return(cs, nil)
	})
}

func newRegistry(handlers ...cr.ComponentHandler) *cr.Registry {
	reg := &cr.Registry{}
	for _, h := range handlers {
		reg.Add(h)
	}
	return reg
}

func newDSC() *dscv2.DataScienceCluster {
	dsc := &dscv2.DataScienceCluster{}
	dsc.SetGroupVersionKind(gvk.DataScienceCluster)
	dsc.SetName("test-dsc")
	return dsc
}

func unifiedBatchNames(g Gomega, reg *provision.UnifiedRegistry) []string {
	batches, err := reg.ResolvedBatches()
	g.Expect(err).ShouldNot(HaveOccurred())

	var names []string
	for _, batch := range batches {
		for _, node := range batch {
			names = append(names, node.GetName())
		}
	}

	return names
}

func TestComputeComponentsStatus(t *testing.T) {
	t.Run("all managed components ready should set ComponentsReady=True", func(t *testing.T) {
		g := NewWithT(t)
		dsc := newDSC()
		reg := newRegistry(
			newMock("comp-a", true, metav1.ConditionTrue),
			newMock("comp-b", true, metav1.ConditionTrue),
		)

		rr := &types.ReconciliationRequest{
			Instance:   dsc,
			Conditions: conditions.NewManager(dsc, status.ConditionTypeComponentsReady),
		}

		err := computeComponentsStatus(t.Context(), rr, reg)
		g.Expect(err).ShouldNot(HaveOccurred())

		g.Expect(dsc).Should(WithTransform(json.Marshal,
			jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
				status.ConditionTypeComponentsReady, metav1.ConditionTrue),
		))
	})

	t.Run("ConditionUnknown from enabled component should set ComponentsReady=False", func(t *testing.T) {
		g := NewWithT(t)
		dsc := newDSC()
		reg := newRegistry(
			newMock("comp-a", true, metav1.ConditionTrue),
			newMock("comp-b", true, metav1.ConditionUnknown),
		)

		rr := &types.ReconciliationRequest{
			Instance:   dsc,
			Conditions: conditions.NewManager(dsc, status.ConditionTypeComponentsReady),
		}

		err := computeComponentsStatus(t.Context(), rr, reg)
		g.Expect(err).ShouldNot(HaveOccurred())

		g.Expect(dsc).Should(WithTransform(json.Marshal, And(
			jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
				status.ConditionTypeComponentsReady, metav1.ConditionFalse),
			jq.Match(`.status.conditions[] | select(.type == "%s") | .message | contains("comp-b")`,
				status.ConditionTypeComponentsReady),
		)))
	})

	t.Run("ConditionFalse from enabled component should set ComponentsReady=False", func(t *testing.T) {
		g := NewWithT(t)
		dsc := newDSC()
		reg := newRegistry(
			newMock("comp-a", true, metav1.ConditionTrue),
			newMock("comp-b", true, metav1.ConditionFalse),
		)

		rr := &types.ReconciliationRequest{
			Instance:   dsc,
			Conditions: conditions.NewManager(dsc, status.ConditionTypeComponentsReady),
		}

		err := computeComponentsStatus(t.Context(), rr, reg)
		g.Expect(err).ShouldNot(HaveOccurred())

		g.Expect(dsc).Should(WithTransform(json.Marshal, And(
			jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
				status.ConditionTypeComponentsReady, metav1.ConditionFalse),
			jq.Match(`.status.conditions[] | select(.type == "%s") | .message | contains("comp-b")`,
				status.ConditionTypeComponentsReady),
		)))
	})

	t.Run("disabled component returning ConditionFalse should count as not ready", func(t *testing.T) {
		g := NewWithT(t)
		dsc := newDSC()
		reg := newRegistry(
			newMock("comp-a", true, metav1.ConditionTrue),
			newMock("comp-stuck", false, metav1.ConditionFalse),
		)

		rr := &types.ReconciliationRequest{
			Instance:   dsc,
			Conditions: conditions.NewManager(dsc, status.ConditionTypeComponentsReady),
		}

		err := computeComponentsStatus(t.Context(), rr, reg)
		g.Expect(err).ShouldNot(HaveOccurred())

		g.Expect(dsc).Should(WithTransform(json.Marshal, And(
			jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
				status.ConditionTypeComponentsReady, metav1.ConditionFalse),
			jq.Match(`.status.conditions[] | select(.type == "%s") | .message | contains("comp-stuck")`,
				status.ConditionTypeComponentsReady),
		)))
	})

	t.Run("disabled component returning ConditionUnknown should be skipped", func(t *testing.T) {
		g := NewWithT(t)
		dsc := newDSC()
		reg := newRegistry(
			newMock("comp-a", true, metav1.ConditionTrue),
			newMock("comp-disabled", false, metav1.ConditionUnknown),
		)

		rr := &types.ReconciliationRequest{
			Instance:   dsc,
			Conditions: conditions.NewManager(dsc, status.ConditionTypeComponentsReady),
		}

		err := computeComponentsStatus(t.Context(), rr, reg)
		g.Expect(err).ShouldNot(HaveOccurred())

		g.Expect(dsc).Should(WithTransform(json.Marshal,
			jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
				status.ConditionTypeComponentsReady, metav1.ConditionTrue),
		))
	})

	t.Run("no managed components should set ComponentsReady=True with info severity", func(t *testing.T) {
		g := NewWithT(t)
		dsc := newDSC()
		reg := newRegistry(
			newMock("comp-a", false, metav1.ConditionUnknown),
		)

		rr := &types.ReconciliationRequest{
			Instance:   dsc,
			Conditions: conditions.NewManager(dsc, status.ConditionTypeComponentsReady),
		}

		err := computeComponentsStatus(t.Context(), rr, reg)
		g.Expect(err).ShouldNot(HaveOccurred())

		g.Expect(dsc).Should(WithTransform(json.Marshal, And(
			jq.Match(`.status.conditions[] | select(.type == "%s") | .status == "%s"`,
				status.ConditionTypeComponentsReady, metav1.ConditionTrue),
			jq.Match(`.status.conditions[] | select(.type == "%s") | .reason == "%s"`,
				status.ConditionTypeComponentsReady, status.NoManagedComponentsReason),
		)))
	})
}

func TestSyncComponentDAGStateForDSC(t *testing.T) {
	newDashboardHandler := func() *cr.BaseComponentHandler {
		return &cr.BaseComponentHandler{
			Name: "dashboard",
			GVK:  gvk.Dashboard,
			IsEnabledFn: func(dsc *dscv2.DataScienceCluster) bool {
				return dsc.Spec.Components.Dashboard.ManagementState == operatorv1.Managed
			},
		}
	}

	t.Run("empty removed and managed states update unified DAG", func(t *testing.T) {
		g := NewWithT(t)
		reg := &cr.Registry{}
		reg.Add(newDashboardHandler(), cr.WithRunlevel(dag.RL(10)))

		provisionReg := provision.NewRegistry()
		provisionReg.Add("dashboard", provision.KindComponent, dag.RL(10))
		provisionReg.Add("aigateway", provision.KindModule, dag.RL(20))

		dsc := newDSC()
		dsc.Spec.Components.AIGateway.ManagementState = operatorv1.Managed

		err := syncComponentDAGStateForDSC(dsc, reg, provisionReg)
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(unifiedBatchNames(g, provisionReg)).Should(ConsistOf("aigateway"))

		dsc.Spec.Components.Dashboard.ManagementState = operatorv1.Managed
		err = syncComponentDAGStateForDSC(dsc, reg, provisionReg)
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(unifiedBatchNames(g, provisionReg)).Should(ConsistOf("dashboard", "aigateway"))

		dsc.Spec.Components.Dashboard.ManagementState = operatorv1.Removed
		err = syncComponentDAGStateForDSC(dsc, reg, provisionReg)
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(unifiedBatchNames(g, provisionReg)).Should(ConsistOf("aigateway"))
	})

	t.Run("operator suppression wins over DSC managed state", func(t *testing.T) {
		g := NewWithT(t)
		reg := &cr.Registry{}
		reg.Add(newDashboardHandler(), cr.WithRunlevel(dag.RL(10)))
		reg.Disable("dashboard")

		provisionReg := provision.NewRegistry()
		provisionReg.Add("dashboard", provision.KindComponent, dag.RL(10))
		provisionReg.Add("aigateway", provision.KindModule, dag.RL(20))

		dsc := newDSC()
		dsc.Spec.Components.Dashboard.ManagementState = operatorv1.Managed
		dsc.Spec.Components.AIGateway.ManagementState = operatorv1.Managed

		err := syncComponentDAGStateForDSC(dsc, reg, provisionReg)
		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(unifiedBatchNames(g, provisionReg)).Should(ConsistOf("aigateway"))
	})
}
