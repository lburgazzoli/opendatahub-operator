package feastoperator_test

import (
	"context"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	configv1alpha2 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha2"
	dscv3 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v3"
	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules/feastoperator"

	. "github.com/onsi/gomega"
)

func newPlatformModules(mgmtState operatorv1.ManagementState) *configv1alpha2.PlatformModules {
	return &configv1alpha2.PlatformModules{
		Data: common.ManagementSpec{
			ManagementState: mgmtState,
		},
	}
}

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = configv1.Install(scheme)
	_ = serviceApi.AddToScheme(scheme)
	return scheme
}

func TestIsEnabled_Managed(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	g.Expect(h.IsEnabled(newPlatformModules(operatorv1.Managed))).Should(BeTrue())
}

func TestIsEnabled_Removed(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	g.Expect(h.IsEnabled(newPlatformModules(operatorv1.Removed))).Should(BeFalse())
}

func TestIsEnabled_NilModules(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	g.Expect(h.IsEnabled(nil)).Should(BeFalse())
}

func TestIsEnabled_EmptyModules(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	g.Expect(h.IsEnabled(&configv1alpha2.PlatformModules{})).Should(BeFalse())
}

func TestBuildModuleCR_NilClientReturnsError_BothNil(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	_, err := h.BuildModuleCR(context.Background(), nil, nil, nil)
	g.Expect(err).Should(HaveOccurred())
}

func TestBuildModuleCR_NilClientReturnsError(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()

	_, err := h.BuildModuleCR(context.Background(), nil, nil, nil)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err.Error()).Should(ContainSubstring("kubernetes client is nil"))
}

func TestBuildModuleCR_NonOIDCCluster(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()

	cli := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()

	u, err := h.BuildModuleCR(context.Background(), cli, nil, nil)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(u.GetName()).Should(Equal(componentApi.FeastOperatorInstanceName))
	g.Expect(u.GetKind()).Should(Equal(componentApi.FeastOperatorKind))
	g.Expect(u.GetAPIVersion()).Should(Equal("components.platform.opendatahub.io/v1alpha1"))
}

func TestBuildModuleCR_OIDCIssuerProjected(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()

	cli := fake.NewClientBuilder().
		WithScheme(newTestScheme()).
		WithObjects(
			&configv1.Authentication{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       configv1.AuthenticationSpec{Type: "OIDC"},
			},
			&serviceApi.GatewayConfig{
				ObjectMeta: metav1.ObjectMeta{Name: serviceApi.GatewayConfigName},
				Spec: serviceApi.GatewayConfigSpec{
					OIDC: &serviceApi.OIDCConfig{
						IssuerURL: "https://keycloak.example.com/realms/odh",
					},
				},
			},
		).
		Build()

	u, err := h.BuildModuleCR(context.Background(), cli, nil, nil)
	g.Expect(err).ShouldNot(HaveOccurred())

	spec, _ := unstructuredNestedMap(u.Object, "spec")
	oidc, ok := spec["oidc"].(map[string]any)
	g.Expect(ok).To(BeTrue(), "spec.oidc should exist")
	g.Expect(oidc["issuerURL"]).To(Equal("https://keycloak.example.com/realms/odh"))
}

func TestBuildModuleCR_InvalidIssuerReturnsError(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()

	cli := fake.NewClientBuilder().
		WithScheme(newTestScheme()).
		WithObjects(
			&configv1.Authentication{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       configv1.AuthenticationSpec{Type: "OIDC"},
			},
			&serviceApi.GatewayConfig{
				ObjectMeta: metav1.ObjectMeta{Name: serviceApi.GatewayConfigName},
				Spec: serviceApi.GatewayConfigSpec{
					OIDC: &serviceApi.OIDCConfig{
						IssuerURL: "http://not-https.example.com",
					},
				},
			},
		).
		Build()

	_, err := h.BuildModuleCR(context.Background(), cli, nil, nil)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err.Error()).Should(ContainSubstring("https"))
}

func TestImageHandling(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()

	g.Expect(h.GetControllerImage()).Should(Equal("RELATED_IMAGE_ODH_FEAST_MODULE_OPERATOR_IMAGE"))

	g.Expect(h.GetRelatedImages()).Should(ConsistOf(
		"RELATED_IMAGE_ODH_FEAST_OPERATOR_IMAGE",
		"RELATED_IMAGE_ODH_FEATURE_SERVER_IMAGE",
	))

	g.Expect(h.GetRelatedImages()).ShouldNot(ContainElement("RELATED_IMAGE_ODH_FEAST_MODULE_OPERATOR_IMAGE"))
}

func TestGetName(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	g.Expect(h.GetName()).Should(Equal(componentApi.FeastOperatorComponentName))
}

func TestGetGVK(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	gvk := h.GetGVK()
	g.Expect(gvk.Group).Should(Equal("components.platform.opendatahub.io"))
	g.Expect(gvk.Version).Should(Equal("v1alpha1"))
	g.Expect(gvk.Kind).Should(Equal("FeastOperator"))
}

// --- PopulatePlatformModule OR-rule tests ---

func newDSCContext(fsState, drState operatorv1.ManagementState) *modules.DSCContext {
	return &modules.DSCContext{
		DSC: &dscv3.DataScienceCluster{
			Spec: dscv3.DataScienceClusterSpec{
				Components: dscv3.Components{
					Data: componentApi.DSCData{
						FeatureStore: componentApi.DSCFeatureStore{
							ManagementSpec: common.ManagementSpec{ManagementState: fsState},
						},
						DataRegistry: componentApi.DSCDataRegistry{
							ManagementSpec: common.ManagementSpec{ManagementState: drState},
						},
					},
				},
			},
		},
	}
}

func TestPopulatePlatformModule_BothManaged(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	pm := &configv1alpha2.PlatformModules{}
	h.PopulatePlatformModule(pm, newDSCContext(operatorv1.Managed, operatorv1.Managed))
	g.Expect(pm.Data.ManagementState).Should(Equal(operatorv1.Managed))
}

func TestPopulatePlatformModule_FSManagedDRRemoved(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	pm := &configv1alpha2.PlatformModules{}
	h.PopulatePlatformModule(pm, newDSCContext(operatorv1.Managed, operatorv1.Removed))
	g.Expect(pm.Data.ManagementState).Should(Equal(operatorv1.Managed))
}

func TestPopulatePlatformModule_FSRemovedDRManaged(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	pm := &configv1alpha2.PlatformModules{}
	h.PopulatePlatformModule(pm, newDSCContext(operatorv1.Removed, operatorv1.Managed))
	g.Expect(pm.Data.ManagementState).Should(Equal(operatorv1.Managed))
}

func TestPopulatePlatformModule_BothRemoved(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	pm := &configv1alpha2.PlatformModules{}
	h.PopulatePlatformModule(pm, newDSCContext(operatorv1.Removed, operatorv1.Removed))
	g.Expect(pm.Data.ManagementState).Should(Equal(operatorv1.Removed))
}

func TestPopulatePlatformModule_EmptyFSManagedDR(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	pm := &configv1alpha2.PlatformModules{}
	h.PopulatePlatformModule(pm, newDSCContext("", operatorv1.Managed))
	g.Expect(pm.Data.ManagementState).Should(Equal(operatorv1.Managed))
}

func TestPopulatePlatformModule_BothEmpty(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	pm := &configv1alpha2.PlatformModules{}
	h.PopulatePlatformModule(pm, newDSCContext("", ""))
	g.Expect(pm.Data.ManagementState).Should(Equal(operatorv1.Removed))
}

func TestPopulatePlatformModule_NilPM(t *testing.T) {
	h := feastoperator.NewHandler()
	// Should not panic
	h.PopulatePlatformModule(nil, newDSCContext(operatorv1.Managed, operatorv1.Managed))
}

func TestPopulatePlatformModule_NilDSCContext(t *testing.T) {
	h := feastoperator.NewHandler()
	pm := &configv1alpha2.PlatformModules{}
	// Should not panic
	h.PopulatePlatformModule(pm, nil)
}

func TestPopulatePlatformModule_NilDSC(t *testing.T) {
	h := feastoperator.NewHandler()
	pm := &configv1alpha2.PlatformModules{}
	h.PopulatePlatformModule(pm, &modules.DSCContext{})
}

// --- BuildModuleCR capabilities projection tests ---

func newDSCContextWithNamespace(fsState, drState operatorv1.ManagementState, ns string) *modules.DSCContext {
	return &modules.DSCContext{
		DSC: &dscv3.DataScienceCluster{
			Spec: dscv3.DataScienceClusterSpec{
				Components: dscv3.Components{
					Data: componentApi.DSCData{
						FeatureStore: componentApi.DSCFeatureStore{
							ManagementSpec: common.ManagementSpec{ManagementState: fsState},
						},
						DataRegistry: componentApi.DSCDataRegistry{
							ManagementSpec: common.ManagementSpec{ManagementState: drState},
							Namespace:      ns,
						},
					},
				},
			},
		},
	}
}

func TestBuildModuleCR_CapabilitiesProjected(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	cli := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()

	dscCtx := newDSCContextWithNamespace(operatorv1.Removed, operatorv1.Managed, "catalog-prod")

	u, err := h.BuildModuleCR(context.Background(), cli, dscCtx, nil)
	g.Expect(err).ShouldNot(HaveOccurred())

	spec, ok := unstructuredNestedMap(u.Object, "spec")
	g.Expect(ok).To(BeTrue())

	caps, ok := spec["capabilities"].(map[string]any)
	g.Expect(ok).To(BeTrue(), "spec.capabilities should exist")

	fs, ok := caps["featureStore"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(fs["managementState"]).To(Equal("Removed"))

	dr, ok := caps["dataRegistry"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(dr["managementState"]).To(Equal("Managed"))
	g.Expect(dr["namespace"]).To(Equal("catalog-prod"))
}

func TestBuildModuleCR_CapabilitiesNoNamespace(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	cli := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()

	dscCtx := newDSCContextWithNamespace(operatorv1.Managed, operatorv1.Managed, "")

	u, err := h.BuildModuleCR(context.Background(), cli, dscCtx, nil)
	g.Expect(err).ShouldNot(HaveOccurred())

	spec, ok := unstructuredNestedMap(u.Object, "spec")
	g.Expect(ok).To(BeTrue())

	caps, ok := spec["capabilities"].(map[string]any)
	g.Expect(ok).To(BeTrue())

	dr, ok := caps["dataRegistry"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(dr["managementState"]).To(Equal("Managed"))
	_, hasNS := dr["namespace"]
	g.Expect(hasNS).To(BeFalse(), "namespace should be omitted when empty")
}

func TestBuildModuleCR_CapabilitiesEmptyDefaults(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	cli := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()

	dscCtx := newDSCContextWithNamespace("", "", "")

	u, err := h.BuildModuleCR(context.Background(), cli, dscCtx, nil)
	g.Expect(err).ShouldNot(HaveOccurred())

	spec, ok := unstructuredNestedMap(u.Object, "spec")
	g.Expect(ok).To(BeTrue())

	caps, ok := spec["capabilities"].(map[string]any)
	g.Expect(ok).To(BeTrue())

	fs, ok := caps["featureStore"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(fs["managementState"]).To(Equal("Removed"))

	dr, ok := caps["dataRegistry"].(map[string]any)
	g.Expect(ok).To(BeTrue())
	g.Expect(dr["managementState"]).To(Equal("Removed"))
}

func TestBuildModuleCR_NoDSCContextNoCapabilities(t *testing.T) {
	g := NewWithT(t)
	h := feastoperator.NewHandler()
	cli := fake.NewClientBuilder().WithScheme(newTestScheme()).Build()

	u, err := h.BuildModuleCR(context.Background(), cli, nil, nil)
	g.Expect(err).ShouldNot(HaveOccurred())

	spec, ok := unstructuredNestedMap(u.Object, "spec")
	g.Expect(ok).To(BeTrue())
	_, hasCaps := spec["capabilities"]
	g.Expect(hasCaps).To(BeFalse(), "capabilities should be absent when dscCtx is nil")
}

func unstructuredNestedMap(obj map[string]any, fields ...string) (map[string]any, bool) {
	val := obj
	for _, f := range fields {
		next, ok := val[f]
		if !ok {
			return nil, false
		}
		m, ok := next.(map[string]any)
		if !ok {
			return nil, false
		}
		val = m
	}
	return val, true
}
