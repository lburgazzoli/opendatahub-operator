package v2

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"
	operatorv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	dscv3 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v3"
)

func populatedV2DSC() *DataScienceCluster {
	src := &DataScienceCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: "DataScienceCluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "default-dsc",
			Labels:          map[string]string{"owner": "conversion-test"},
			Annotations:     map[string]string{"example": "kept"},
			Finalizers:      []string{"example/finalizer"},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "v1", Kind: "ConfigMap", Name: "owner", UID: "uid"}},
		},
		Status: DataScienceClusterStatus{
			Status: common.Status{
				Phase:              "Ready",
				ObservedGeneration: 7,
				Conditions:         []common.Condition{{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"}},
			},
			RelatedObjects: []corev1.ObjectReference{{APIVersion: "v1", Kind: "Secret", Name: "related"}},
			ErrorMessage:   "retained",
			Release:        common.Release{Name: "OpenDataHub"},
		},
	}
	src.Spec.Components.Dashboard.ManagementState = operatorv1.Managed
	src.Spec.Components.Workbenches.ManagementState = operatorv1.Managed
	src.Spec.Components.AIPipelines.ManagementState = operatorv1.Managed
	src.Spec.Components.Kserve.ManagementState = operatorv1.Managed
	src.Spec.Components.Kueue.ManagementState = operatorv1.Managed
	src.Spec.Components.Ray.ManagementState = operatorv1.Managed
	src.Spec.Components.TrustyAI.ManagementState = operatorv1.Managed
	src.Spec.Components.ModelRegistry.ManagementState = operatorv1.Managed
	src.Spec.Components.TrainingOperator.ManagementState = operatorv1.Managed
	src.Spec.Components.FeastOperator.ManagementState = operatorv1.Managed
	src.Spec.Components.LlamaStackOperator.ManagementState = operatorv1.Managed
	src.Spec.Components.OGX.ManagementState = operatorv1.Managed
	src.Spec.Components.MLflowOperator.ManagementState = operatorv1.Managed
	src.Spec.Components.Trainer.ManagementState = operatorv1.Managed
	src.Spec.Components.SparkOperator.ManagementState = operatorv1.Managed
	src.Spec.Components.AIGateway.ManagementState = operatorv1.Managed
	src.Spec.Components.MCPLifecycleOperator.ManagementState = operatorv1.Managed
	src.Status.Components.Dashboard.ManagementState = operatorv1.Managed
	src.Status.Components.MaaSConsumerPortal.ManagementState = operatorv1.Managed
	src.Status.Components.Workbenches.ManagementState = operatorv1.Managed
	src.Status.Components.WorkbenchesV2.ManagementState = operatorv1.Managed
	src.Status.Components.AIPipelines.ManagementState = operatorv1.Managed
	src.Status.Components.Kserve.ManagementState = operatorv1.Managed
	src.Status.Components.Kueue.ManagementState = operatorv1.Managed
	src.Status.Components.Ray.ManagementState = operatorv1.Managed
	src.Status.Components.TrustyAI.ManagementState = operatorv1.Managed
	src.Status.Components.ModelRegistry.ManagementState = operatorv1.Managed
	src.Status.Components.TrainingOperator.ManagementState = operatorv1.Managed
	src.Status.Components.FeastOperator.ManagementState = operatorv1.Managed
	src.Status.Components.LlamaStackOperator.ManagementState = operatorv1.Managed
	src.Status.Components.OGX.ManagementState = operatorv1.Managed
	src.Status.Components.MLflowOperator.ManagementState = operatorv1.Managed
	src.Status.Components.Trainer.ManagementState = operatorv1.Managed
	src.Status.Components.SparkOperator.ManagementState = operatorv1.Managed
	src.Status.Components.AIGateway.ManagementState = operatorv1.Managed
	src.Status.Components.ModelsAsAService.ManagementState = operatorv1.Managed
	src.Status.Components.BatchGateway.ManagementState = operatorv1.Managed
	src.Status.Components.MCPLifecycleOperator.ManagementState = operatorv1.Managed
	src.Spec.Components.Dashboard.MaaSConsumerPortal.ManagementState = operatorv1.Managed
	src.Spec.Components.Kserve.ModelsAsService.ManagementState = operatorv1.Removed //nolint:staticcheck
	src.Spec.Components.AIGateway.ModelsAsAService.ManagementState = operatorv1.Managed
	src.Spec.Components.AIPipelines.ArgoWorkflowsControllers = &componentApi.ArgoWorkflowsControllersSpec{ManagementState: operatorv1.Removed}
	src.Spec.Components.ModelRegistry.RegistriesNamespace = "custom-registries"
	src.Spec.Components.AIGateway.BatchGateway.ManagementState = operatorv1.Managed
	src.Spec.Components.Kserve.RawDeploymentServiceConfig = componentApi.KserveRawHeaded
	src.Spec.Components.Kserve.OAuthProxy = &componentApi.OAuthProxyConfig{
		Resources: &componentApi.OAuthProxyResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
			Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("256Mi")},
		},
	}
	src.Spec.Components.Kserve.NIM = componentApi.NimSpec{ManagementState: operatorv1.Removed, AirGapped: true}
	src.Spec.Components.Kserve.WVA.ManagementState = operatorv1.Managed
	src.Spec.Components.Kserve.EnableLLMInferenceServiceTLS = new(false)
	src.Spec.Components.Kserve.EnableLLMInferenceServiceConsoleDashboards = new(true)
	src.Spec.Components.Kserve.ModelCache = &componentApi.ModelCacheSpec{
		ManagementState: operatorv1.Managed,
		CacheSize:       new(resource.MustParse("10Gi")),
		NodeNames:       []string{"worker-a", "worker-b"},
		NodeSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"cache": "enabled"}},
	}
	src.Status.Components.Workbenches.WorkbenchesCommonStatus = &componentApi.WorkbenchesCommonStatus{WorkbenchNamespace: "notebooks"}
	src.Status.Components.LlamaStackOperator.ManagementState = operatorv1.Removed
	return src
}

func wireWithoutVersion(t *testing.T, obj any) map[string]any {
	t.Helper()
	data, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	delete(wire, "apiVersion")
	return wire
}

func TestConversionPreservesUnchangedFields(t *testing.T) {
	g := NewWithT(t)
	original := populatedV2DSC()

	hub := &dscv3.DataScienceCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: dscv3.GroupVersion.String(), Kind: "DataScienceCluster"},
	}
	g.Expect(original.ConvertTo(hub)).To(Succeed())
	expectedWire := wireWithoutVersion(t, original)
	delete(expectedWire["spec"].(map[string]any)["components"].(map[string]any)["kserve"].(map[string]any), "modelsAsService")
	g.Expect(wireWithoutVersion(t, hub)).To(Equal(expectedWire))

	back := &DataScienceCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: "DataScienceCluster"},
	}
	g.Expect(back.ConvertFrom(hub)).To(Succeed())
	g.Expect(back).To(Equal(original))

	fromHub := &DataScienceCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: "DataScienceCluster"},
	}
	g.Expect(fromHub.ConvertFrom(hub)).To(Succeed())
	g.Expect(fromHub).To(Equal(original))

	hubAgain := &dscv3.DataScienceCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: dscv3.GroupVersion.String(), Kind: "DataScienceCluster"},
	}
	g.Expect(fromHub.ConvertTo(hubAgain)).To(Succeed())
	g.Expect(hubAgain).To(Equal(hub))

	// Conversions must not alias mutable metadata or nested status slices.
	hub.Labels["owner"] = "changed"
	hub.Status.Conditions[0].Reason = "Changed"
	hub.Spec.Components.Kserve.ModelCache.NodeNames[0] = "changed"
	hub.Spec.Components.Kserve.ModelCache.NodeSelector.MatchLabels["cache"] = "changed"
	hub.Spec.Components.Kserve.OAuthProxy.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("1")
	*hub.Spec.Components.Kserve.EnableLLMInferenceServiceTLS = true
	back.Spec.Components.Kserve.ModelCache.NodeNames[1] = "changed"
	g.Expect(original.Labels["owner"]).To(Equal("conversion-test"))
	g.Expect(original.Status.Conditions[0].Reason).To(Equal("Ready"))
	g.Expect(original.Spec.Components.Kserve.ModelCache.NodeNames).To(Equal([]string{"worker-a", "worker-b"}))
	g.Expect(original.Spec.Components.Kserve.ModelCache.NodeSelector.MatchLabels).To(HaveKeyWithValue("cache", "enabled"))
	g.Expect(original.Spec.Components.Kserve.OAuthProxy.Resources.Requests[corev1.ResourceCPU]).To(Equal(resource.MustParse("100m")))
	g.Expect(*original.Spec.Components.Kserve.EnableLLMInferenceServiceTLS).To(BeFalse())
}
