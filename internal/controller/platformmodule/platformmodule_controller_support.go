package platformmodule

import (
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

// resourceRefsFrom converts a slice of unstructured resources to ResourceRefs
// for tracking in PlatformModule.Status.Resources.
func resourceRefsFrom(rs []unstructured.Unstructured) []configv1alpha1.ResourceRef {
	if len(rs) == 0 {
		return nil
	}
	refs := make([]configv1alpha1.ResourceRef, 0, len(rs))
	for _, r := range rs {
		k := r.GroupVersionKind()
		refs = append(refs, configv1alpha1.ResourceRef{
			Group:     k.Group,
			Version:   k.Version,
			Kind:      k.Kind,
			Namespace: r.GetNamespace(),
			Name:      r.GetName(),
		})
	}
	return refs
}

// ensureConfigMap returns the index of the ConfigMap with the given name in
// resources, or appends a new empty ConfigMap and returns its index.
func ensureConfigMap(rs *[]unstructured.Unstructured, name string, namespace string) (int, error) {
	configMapGVK := gvk.ConfigMap
	for i, r := range *rs {
		if r.GroupVersionKind() == configMapGVK && r.GetName() == name {
			return i, nil
		}
	}

	cm := modules.BuildPlatformConfigMap(name, namespace, "")
	u, err := resources.ToUnstructured(cm)
	if err != nil {
		return 0, err
	}

	*rs = append(*rs, *u)

	return len(*rs) - 1, nil
}

// deletePropagationPolicy returns the propagation policy to use when deleting
// stale resources. Foreground propagation is used in production so that
// dependent objects are cleaned up before the owner is removed.
// Envtest does not support foreground deletion, so Background is used when
// KUBEBUILDER_ASSETS is set.
func deletePropagationPolicy() metav1.DeletionPropagation {
	if _, ok := os.LookupEnv("KUBEBUILDER_ASSETS"); ok {
		return metav1.DeletePropagationBackground
	}

	return metav1.DeletePropagationForeground
}
