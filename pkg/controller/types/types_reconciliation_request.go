package types

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/apis/components"
	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	odhClient "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/client"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/manager"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

type ReconciliationRequestOpts func(rr *ReconciliationRequest)

func WithClient(value *odhClient.Client) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.Client = value
	}
}

func WithControllerName(value string) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.controllerName = value
	}
}

func WithManager(value *manager.Manager) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.manager = value
	}
}

func WithRelease(value cluster.Release) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.Release = value
	}
}

func WithInstance(value components.ComponentObject) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.Instance = value
	}
}

func WithDSC(value *dscv1.DataScienceCluster) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.DSC = value
	}
}

func WithDSCI(value *dsciv1.DSCInitialization) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.DSCI = value
	}
}

func WithResources(values ...unstructured.Unstructured) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.Resources = append(rr.Resources, values...)
	}
}

func WithGenerated(value bool) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.Generated = value
	}
}

func WithManifests(values ...ManifestInfo) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.Manifests = append(rr.Manifests, values...)
	}
}

func WithTemplates(values ...TemplateInfo) ReconciliationRequestOpts {
	return func(rr *ReconciliationRequest) {
		rr.Templates = append(rr.Templates, values...)
	}
}

func NewReconciliationRequest(opts ...ReconciliationRequestOpts) *ReconciliationRequest {
	rr := ReconciliationRequest{}
	rr.Manifests = make([]ManifestInfo, 0)
	rr.Templates = make([]TemplateInfo, 0)
	rr.Resources = make([]unstructured.Unstructured, 0)

	for _, opt := range opts {
		opt(&rr)
	}

	return &rr
}

type ReconciliationRequest struct {
	*odhClient.Client

	controllerName string
	manager        *manager.Manager

	Release   cluster.Release
	Instance  components.ComponentObject
	DSC       *dscv1.DataScienceCluster
	DSCI      *dsciv1.DSCInitialization
	Manifests []ManifestInfo

	//
	// TODO: unify templates and resources.
	//
	// Unfortunately, the kustomize APIs do not yet support a FileSystem that is
	// backed by golang's fs.Fs so it is not simple to have a single abstraction
	// for both the manifests types.
	//
	// it would be nice to have a structure like:
	//
	// struct {
	//   FS  fs.FS
	//   URI net.URL
	// }
	//
	// where the URI could be something like:
	// - kustomize:///path/to/overlay
	// - template:///path/to/resource.tmpl.yaml
	//
	// and use the scheme as discriminator for the rendering engine
	//
	Templates []TemplateInfo
	Resources []unstructured.Unstructured

	// TODO: this has been added to reduce GC work and only run when
	//       resources have been generated. It should be removed and
	//       replaced with a better way of describing resources and
	//       their origin
	Generated bool
}

func (rr *ReconciliationRequest) ControllerName() string {
	return rr.controllerName
}

func (rr *ReconciliationRequest) Manager() *manager.Manager {
	return rr.manager
}

// AddResources adds one or more resources to the ReconciliationRequest's Resources slice.
// Each provided client.Object is normalized by ensuring it has the appropriate GVK and is
// converted into an unstructured.Unstructured format before being appended to the list.
func (rr *ReconciliationRequest) AddResources(values ...client.Object) error {
	for i := range values {
		if values[i] == nil {
			continue
		}

		err := resources.EnsureGroupVersionKind(rr.Client.Scheme(), values[i])
		if err != nil {
			return fmt.Errorf("cannot normalize object: %w", err)
		}

		u, err := resources.ToUnstructured(values[i])
		if err != nil {
			return fmt.Errorf("cannot convert object to Unstructured: %w", err)
		}

		rr.Resources = append(rr.Resources, *u)
	}

	return nil
}

// ForEachResource iterates over each resource in the ReconciliationRequest's Resources slice,
// invoking the provided function `fn` for each resource. The function `fn` takes a pointer to
// an unstructured.Unstructured object and returns a boolean and an error.
//
// The iteration stops early if:
//   - `fn` returns an error.
//   - `fn` returns `true` as the first return value (`stop`).
func (rr *ReconciliationRequest) ForEachResource(fn func(*unstructured.Unstructured) (bool, error)) error {
	for i := range rr.Resources {
		stop, err := fn(&rr.Resources[i])
		if err != nil {
			return fmt.Errorf("cannot process resource %s: %w", rr.Resources[i].GroupVersionKind(), err)
		}
		if stop {
			break
		}
	}

	return nil
}

// RemoveResources removes resources from the ReconciliationRequest's Resources slice
// based on a provided predicate function. The predicate determines whether a resource
// should be removed.
//
// Parameters:
//   - predicate: A function that takes a pointer to an unstructured.Unstructured object
//     and returns a boolean indicating whether the resource should be removed.
func (rr *ReconciliationRequest) RemoveResources(predicate func(*unstructured.Unstructured) bool) error {
	filtered := rr.Resources[:0] // Create a slice with zero length but full capacity

	for i := range rr.Resources {
		if predicate(&rr.Resources[i]) {
			continue
		}

		filtered = append(filtered, rr.Resources[i])
	}

	rr.Resources = filtered

	return nil
}
