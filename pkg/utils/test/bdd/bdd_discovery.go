package bdd

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"

	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v1"
	featuresv1 "github.com/opendatahub-io/opendatahub-operator/v2/api/features/v1"
	gvkConstants "github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

var (
	commonGVs = []schema.GroupVersion{
		corev1.SchemeGroupVersion,
		appsv1.SchemeGroupVersion,
		rbacv1.SchemeGroupVersion,
		apiextensionsv1.SchemeGroupVersion,
		dscv1.GroupVersion,
		dsciv1.GroupVersion,
		featuresv1.GroupVersion,
	}

	shortNames = map[string]schema.GroupVersionKind{
		"pod":                  gvkConstants.Pod,
		"pods":                 gvkConstants.Pod,
		"svc":                  gvkConstants.Service,
		"service":              gvkConstants.Service,
		"services":             gvkConstants.Service,
		"cm":                   gvkConstants.ConfigMap,
		"configmap":            gvkConstants.ConfigMap,
		"configmaps":           gvkConstants.ConfigMap,
		"secret":               gvkConstants.Secret,
		"secrets":              gvkConstants.Secret,
		"ns":                   gvkConstants.Namespace,
		"namespace":            gvkConstants.Namespace,
		"namespaces":           gvkConstants.Namespace,
		"node":                 gvkConstants.Node,
		"nodes":                gvkConstants.Node,
		"pv":                   gvkConstants.PersistentVolume,
		"pvc":                  gvkConstants.PersistentVolumeClaim,
		"deploy":               gvkConstants.Deployment,
		"deployment":           gvkConstants.Deployment,
		"deployments":          gvkConstants.Deployment,
		"rs":                   gvkConstants.ReplicaSet,
		"replicaset":           gvkConstants.ReplicaSet,
		"replicasets":          gvkConstants.ReplicaSet,
		"ds":                   gvkConstants.DaemonSet,
		"daemonset":            gvkConstants.DaemonSet,
		"daemonsets":           gvkConstants.DaemonSet,
		"sts":                  gvkConstants.StatefulSet,
		"statefulset":          gvkConstants.StatefulSet,
		"statefulsets":         gvkConstants.StatefulSet,
		"role":                 gvkConstants.Role,
		"roles":                gvkConstants.Role,
		"clusterrole":          gvkConstants.ClusterRole,
		"clusterroles":         gvkConstants.ClusterRole,
		"rolebinding":          gvkConstants.RoleBinding,
		"rolebindings":         gvkConstants.RoleBinding,
		"clusterrolebinding":   gvkConstants.ClusterRoleBinding,
		"clusterrolebindings":  gvkConstants.ClusterRoleBinding,
		"dsc":                  gvkConstants.DataScienceCluster,
		"dsci":                 gvkConstants.DSCInitialization,
		"featuretracker":       gvkConstants.FeatureTracker,
		"dashboard":            gvkConstants.Dashboard,
		"workbenches":          gvkConstants.Workbenches,
		"kserve":               gvkConstants.Kserve,
		"modelcontroller":      gvkConstants.ModelController,
		"modelmeshserving":     gvkConstants.ModelMeshServing,
		"datasciencepipelines": gvkConstants.DataSciencePipelines,
		"codeflare":            gvkConstants.CodeFlare,
		"ray":                  gvkConstants.Ray,
		"trustyai":             gvkConstants.TrustyAI,
		"modelregistry":        gvkConstants.ModelRegistry,
		"trainingoperator":     gvkConstants.TrainingOperator,
		"monitoring":           gvkConstants.Monitoring,
		"auth":                 gvkConstants.Auth,
		"crd":                  gvkConstants.CustomResourceDefinition,
		"crds":                 gvkConstants.CustomResourceDefinition,
	}
)

type ResourceResolver struct {
	discovery  discovery.DiscoveryInterface
	restMapper *restmapper.DeferredDiscoveryRESTMapper
	cache      map[string]*meta.RESTMapping
	mu         sync.RWMutex
}

// NewResourceResolver creates a new resource resolver with the provided discovery client.
func NewResourceResolver(discoveryClient discovery.DiscoveryInterface) (*ResourceResolver, error) {
	if discoveryClient == nil {
		return nil, errors.New("discovery client cannot be nil")
	}

	// Create a cached discovery client and deferred REST mapper for better dynamic resource handling
	cachedDiscoveryClient := memory.NewMemCacheClient(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient)

	return &ResourceResolver{
		discovery:  discoveryClient,
		restMapper: restMapper,
		cache:      make(map[string]*meta.RESTMapping),
	}, nil
}

func (r *ResourceResolver) mapper() meta.RESTMapper {
	r.restMapper.Reset()
	return r.restMapper
}

// Resolve resolves a resource reference to a Resource type.
// Supports various input formats:
// - Short names: "pod", "svc", "cm", "ns"
// - Plural names: "pods", "services", "configmaps"
// - Kind names: "Pod", "Service", "ConfigMap"
// - Full resource names: "pods.v1", "services.v1", "configmaps.v1"
// - Fully qualified: "pods.v1.", "services.v1.", etc.
func (r *ResourceResolver) Resolve(resourceRef string) (resources.Resource, error) {
	if resourceRef == "" {
		return resources.Resource{}, errors.New("resource reference cannot be empty")
	}

	// Check cache first
	r.mu.RLock()
	if mapping, exists := r.cache[resourceRef]; exists {
		r.mu.RUnlock()
		return resources.Resource{RESTMapping: *mapping}, nil
	}
	r.mu.RUnlock()

	// Try to resolve using different strategies
	resource, err := r.resolveResource(resourceRef)
	if err != nil {
		return resources.Resource{}, err
	}

	// Cache the result
	r.mu.Lock()
	r.cache[resourceRef] = &resource.RESTMapping
	r.mu.Unlock()

	return resource, nil
}

func (r *ResourceResolver) resolveResource(resourceRef string) (resources.Resource, error) {
	if resource, ok := r.tryShortNameMapping(resourceRef); ok {
		return resource, nil
	}

	if resource, err := r.tryAsKind(resourceRef); err == nil {
		return resource, nil
	}

	if resource, err := r.tryAsResource(resourceRef); err == nil {
		return resource, nil
	}

	if resource, err := r.tryCommonGroupVersions(resourceRef); err == nil {
		return resource, nil
	}

	return resources.Resource{}, fmt.Errorf("unable to resolve resource reference '%s'", resourceRef)
}

func (r *ResourceResolver) tryShortNameMapping(resourceRef string) (resources.Resource, bool) {
	gvk, exists := shortNames[strings.ToLower(resourceRef)]
	if !exists {
		return resources.Resource{}, false
	}

	mapping, err := r.mapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return resources.Resource{}, false
	}

	return resources.Resource{RESTMapping: *mapping}, true
}

func (r *ResourceResolver) tryAsKind(resourceRef string) (resources.Resource, error) {
	for _, gv := range commonGVs {
		gvk := gv.WithKind(resourceRef)
		mapping, err := r.mapper().RESTMapping(gvk.GroupKind(), gvk.Version)
		if err == nil {
			return resources.Resource{RESTMapping: *mapping}, nil
		}
	}

	return resources.Resource{}, fmt.Errorf("kind not found: %s", resourceRef)
}

func (r *ResourceResolver) tryAsResource(resourceRef string) (resources.Resource, error) {
	// Support formats:
	// - resource.group/version (e.g., "codeflares.components.platform.opendatahub.io/v1alpha1")
	// - resource.group (e.g., "codeflares.components.platform.opendatahub.io") - version will be empty

	gvr := schema.GroupVersionResource{}
	gvr.Resource = resourceRef

	if i := strings.Index(gvr.Resource, "/"); i != -1 {
		gvr.Version = gvr.Resource[i+1:]
		gvr.Resource = gvr.Resource[:i]
	}

	if i := strings.Index(gvr.Resource, "."); i != -1 {
		gvr.Group = gvr.Resource[i+1:]
		gvr.Resource = gvr.Resource[:i]
	}

	if gvr.Resource == "" {
		return resources.Resource{}, errors.New("invalid resource format")
	}

	mapper := r.mapper()

	k, err := mapper.KindFor(gvr)
	if err != nil {
		return resources.Resource{}, err
	}

	mapping, err := mapper.RESTMapping(k.GroupKind(), k.Version)
	if err != nil {
		return resources.Resource{}, err
	}

	return resources.Resource{RESTMapping: *mapping}, nil
}

func (r *ResourceResolver) tryCommonGroupVersions(resourceRef string) (resources.Resource, error) {
	// For now, return an error as this is a fallback that's not critical
	// Most resources should be resolved by the previous strategies
	return resources.Resource{}, fmt.Errorf("no common group/version combinations found for resource: %s", resourceRef)
}

func ValidateResourceScope(mapping *meta.RESTMapping, hasNamespace bool) error {
	if mapping == nil {
		return errors.New("RESTMapping cannot be nil")
	}

	isNamespaced := mapping.Scope.Name() == meta.RESTScopeNamespace.Name()

	if isNamespaced && !hasNamespace {
		return fmt.Errorf("resource type '%s' is namespaced but no namespace was provided", mapping.GroupVersionKind.Kind)
	}

	if !isNamespaced && hasNamespace {
		return fmt.Errorf("resource type '%s' is cluster-scoped but namespace was provided", mapping.GroupVersionKind.Kind)
	}

	return nil
}

func (r *ResourceResolver) IsNamespaced(resourceRef string) (bool, error) {
	resource, err := r.Resolve(resourceRef)
	if err != nil {
		return false, err
	}

	return resource.IsNamespaced(), nil
}

func (r *ResourceResolver) GetGroupVersionResource(resourceRef string) (schema.GroupVersionResource, error) {
	resource, err := r.Resolve(resourceRef)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	return resource.GroupVersionResource(), nil
}

func (r *ResourceResolver) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(map[string]*meta.RESTMapping)
}
