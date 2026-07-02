//nolint:testpackage
package platformmodule

import (
	"context"
	"testing"

	semver "github.com/blang/semver/v4"
	libversion "github.com/operator-framework/api/pkg/lib/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	odhtype "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"

	. "github.com/onsi/gomega"
)

// --- resourceRefsFrom ---

func TestResourceRefsFrom_Empty(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	g.Expect(resourceRefsFrom(nil)).Should(BeNil())
	g.Expect(resourceRefsFrom([]unstructured.Unstructured{})).Should(BeNil())
}

func TestResourceRefsFrom_PreservesGVKAndCoordinates(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	resources := []unstructured.Unstructured{
		makeUnstructured(gvk.Deployment, "opendatahub", "ai-gateway-operator"),
		makeUnstructured(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, "", "ai-gateway-operator"),
	}

	refs := resourceRefsFrom(resources)

	g.Expect(refs).Should(HaveLen(2))
	g.Expect(refs[0]).Should(Equal(configv1alpha1.ResourceRef{
		Group: "apps", Version: "v1", Kind: "Deployment",
		Namespace: "opendatahub", Name: "ai-gateway-operator",
	}))
	g.Expect(refs[1]).Should(Equal(configv1alpha1.ResourceRef{
		Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole",
		Name: "ai-gateway-operator",
	}))
}

// --- ensureConfigMap ---

func TestEnsureConfigMap_FindsExisting(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	resources := []unstructured.Unstructured{
		makeUnstructured(gvk.Deployment, "opendatahub", "my-operator"),
		makeUnstructured(gvk.ConfigMap, "opendatahub", "odh-aigateway-config"),
	}

	idx, err := ensureConfigMap(&resources, "odh-aigateway-config", "opendatahub")

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(idx).Should(Equal(1))
	g.Expect(resources).Should(HaveLen(2)) // no new entry added
}

func TestEnsureConfigMap_AppendsWhenAbsent(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	resources := []unstructured.Unstructured{
		makeUnstructured(gvk.Deployment, "opendatahub", "my-operator"),
	}

	idx, err := ensureConfigMap(&resources, "odh-aigateway-config", "opendatahub")

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(idx).Should(Equal(1))
	g.Expect(resources).Should(HaveLen(2))
	g.Expect(resources[1].GetName()).Should(Equal("odh-aigateway-config"))
	g.Expect(resources[1].GetNamespace()).Should(Equal("opendatahub"))
}

func TestEnsureConfigMap_IgnoresNonConfigMapWithSameName(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// A Secret with the same name should not be matched.
	secret := makeUnstructured(schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, "opendatahub", "odh-aigateway-config")
	resources := []unstructured.Unstructured{secret}

	idx, err := ensureConfigMap(&resources, "odh-aigateway-config", "opendatahub")

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(idx).Should(Equal(1))         // new entry appended
	g.Expect(resources).Should(HaveLen(2)) // secret preserved, ConfigMap added
	g.Expect(resources[1].GetKind()).Should(Equal("ConfigMap"))
}

// --- driftCleanup ---

func TestPlatformModuleDriftCleanup_SkipsWhenSkipDeploy(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	pm := newPlatformModule("monitoring", configv1alpha1.ResourceRef{
		Group: "apps", Version: "v1", Kind: "Deployment",
		Namespace: "opendatahub", Name: "old-deployment",
	})
	rr := &odhtype.ReconciliationRequest{
		Instance:   pm,
		SkipDeploy: true,
		// rr.Resources intentionally empty — should not be touched
	}

	err := (&Reconciler{}).driftCleanup(context.Background(), rr)

	g.Expect(err).ShouldNot(HaveOccurred())
	// Status.Resources must be unchanged when gated.
	g.Expect(pm.Status.Resources).Should(HaveLen(1))
}

func TestPlatformModuleDriftCleanup_DeletesStaleResource(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	staleObj := makeK8sDeployment("old-operator", "opendatahub")
	cl, err := fakeclient.New(fakeclient.WithObjects(staleObj))
	g.Expect(err).ShouldNot(HaveOccurred())

	staleRef := configv1alpha1.ResourceRef{
		Group: "apps", Version: "v1", Kind: "Deployment",
		Namespace: "opendatahub", Name: "old-operator",
	}
	pm := newPlatformModule("aigateway", staleRef)

	// Current render has no resources — old-operator is stale.
	rr := &odhtype.ReconciliationRequest{
		Instance:  pm,
		Client:    cl,
		Resources: []unstructured.Unstructured{},
	}

	err = (&Reconciler{}).driftCleanup(context.Background(), rr)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Stale deployment must be deleted from the cluster.
	dep := &unstructured.Unstructured{}
	dep.SetGroupVersionKind(gvk.Deployment)
	getErr := cl.Get(context.Background(), client.ObjectKey{Name: "old-operator", Namespace: "opendatahub"}, dep)
	g.Expect(getErr).Should(HaveOccurred()) // should be NotFound

	// Status.Resources must reflect the (empty) current render.
	g.Expect(pm.Status.Resources).Should(BeEmpty())
}

func TestPlatformModuleDriftCleanup_KeepsCurrentResources(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	currentObj := makeK8sDeployment("current-operator", "opendatahub")
	cl, err := fakeclient.New(fakeclient.WithObjects(currentObj))
	g.Expect(err).ShouldNot(HaveOccurred())

	currentRef := configv1alpha1.ResourceRef{
		Group: "apps", Version: "v1", Kind: "Deployment",
		Namespace: "opendatahub", Name: "current-operator",
	}
	pm := newPlatformModule("aigateway", currentRef)

	rr := &odhtype.ReconciliationRequest{
		Instance: pm,
		Client:   cl,
		Resources: []unstructured.Unstructured{
			makeUnstructured(gvk.Deployment, "opendatahub", "current-operator"),
		},
	}

	err = (&Reconciler{}).driftCleanup(context.Background(), rr)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Current resource must not be deleted.
	dep := &unstructured.Unstructured{}
	dep.SetGroupVersionKind(gvk.Deployment)
	g.Expect(cl.Get(context.Background(), client.ObjectKey{Name: "current-operator", Namespace: "opendatahub"}, dep)).Should(Succeed())

	g.Expect(pm.Status.Resources).Should(ConsistOf(currentRef))
}

func TestPlatformModuleDriftCleanup_ToleratesAlreadyGoneResource(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cl, err := fakeclient.New() // empty cluster — resource already gone
	g.Expect(err).ShouldNot(HaveOccurred())

	pm := newPlatformModule("aigateway", configv1alpha1.ResourceRef{
		Group: "apps", Version: "v1", Kind: "Deployment",
		Namespace: "opendatahub", Name: "already-gone",
	})

	rr := &odhtype.ReconciliationRequest{
		Instance:  pm,
		Client:    cl,
		Resources: []unstructured.Unstructured{},
	}

	err = (&Reconciler{}).driftCleanup(context.Background(), rr)
	g.Expect(err).ShouldNot(HaveOccurred()) // NotFound must be tolerated
	g.Expect(pm.Status.Resources).Should(BeEmpty())
}

// --- syncModuleCRStatus ---

func TestSyncModuleCRStatus_NoHandler_SetsOperandReady(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	// Empty registry — no handler for "unknown-module".
	r := &Reconciler{registry: modules.NewRegistry()}
	pm := &configv1alpha1.PlatformModule{ObjectMeta: metav1.ObjectMeta{Name: "unknown-module"}}
	conds := newTestConditions()
	rr := &odhtype.ReconciliationRequest{Instance: pm, Conditions: conds}

	err := r.syncModuleCRStatus(context.Background(), rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	// OperandReady must be True when no handler is registered.
	cond := conds.GetCondition(status.ConditionTypeOperandAvailable)
	g.Expect(cond).ShouldNot(BeNil())
	g.Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
}

func TestSyncModuleCRStatus_CRAbsent_MovesForward(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	cl, err := fakeclient.New() // empty cluster — no module CR
	g.Expect(err).ShouldNot(HaveOccurred())

	reg := modules.NewRegistry()
	h := newNoopHandlerWithGVK("mymodule", gvk.Monitoring)
	reg.Add(&h)

	release := testRelease(2, 20, 0)
	r := &Reconciler{registry: reg}
	pm := &configv1alpha1.PlatformModule{ObjectMeta: metav1.ObjectMeta{Name: "mymodule"}}
	conds := newTestConditions()
	rr := &odhtype.ReconciliationRequest{Instance: pm, Client: cl, Conditions: conds, Release: release}

	err = r.syncModuleCRStatus(context.Background(), rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	// CR absent → OperandAvailable=False with OperandAbsent reason.
	cond := conds.GetCondition(status.ConditionTypeOperandAvailable)
	g.Expect(cond).ShouldNot(BeNil())
	g.Expect(cond.Status).Should(Equal(metav1.ConditionFalse))
	g.Expect(cond.Reason).Should(Equal("OperandAbsent"))
	// When CR is absent, release reflects the platform release (rr.Release)
	// since there's no module CR to handshake with.
	g.Expect(pm.Status.Release).Should(Equal(release))
}

// --- helpers ---

func makeUnstructured(k schema.GroupVersionKind, namespace, name string) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(k)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

func makeK8sDeployment(name, namespace string) client.Object {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func newPlatformModule(name string, refs ...configv1alpha1.ResourceRef) *configv1alpha1.PlatformModule {
	pm := &configv1alpha1.PlatformModule{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	pm.Status.Resources = refs
	return pm
}

func testRelease(major, minor, patch uint64) common.Release {
	return common.Release{
		Name: cluster.OpenDataHub,
		Version: libversion.OperatorVersion{
			Version: semver.Version{Major: major, Minor: minor, Patch: patch},
		},
	}
}

// newTestConditions creates a conditions.Manager backed by a fresh PlatformModule.
func newTestConditions() *conditions.Manager {
	pm := &configv1alpha1.PlatformModule{}
	return conditions.NewManager(pm, status.ConditionTypeOperandAvailable)
}

// noopHandlerWithGVK is a ModuleHandler that uses a specific GVK and returns
// no manifests or related images. Used to test GVK-based dispatch in unit tests.
type noopHandlerWithGVK struct {
	modules.BaseHandler
}

func (noopHandlerWithGVK) BuildModuleCR(_ context.Context, _ client.Client, _ *modules.PlatformContext) (*unstructured.Unstructured, error) {
	return nil, nil
}

func (noopHandlerWithGVK) IsEnabled(_ *modules.PlatformContext) bool { return true }

func newNoopHandlerWithGVK(name string, k schema.GroupVersionKind) noopHandlerWithGVK {
	return noopHandlerWithGVK{
		BaseHandler: modules.BaseHandler{
			Config: modules.ModuleConfig{
				Name:   name,
				GVK:    k,
				CRName: "default-" + name,
			},
		},
	}
}
