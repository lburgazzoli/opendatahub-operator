package cleanup

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	odhTypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	odhAnnotations "github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

type DeownHandlerOption interface {
	ApplyToDeownHandler(opts *DeownHandlerOptions)
}

type DeownHandlerOptions struct {
	CheckManagedAnnotation *bool
	ObjectPredicate        ObjectPredicateFn
}

func (o *DeownHandlerOptions) ApplyToDeownHandler(config *DeownHandlerOptions) {
	if o.CheckManagedAnnotation != nil {
		config.CheckManagedAnnotation = o.CheckManagedAnnotation
	}
	if o.ObjectPredicate != nil {
		config.ObjectPredicate = o.ObjectPredicate
	}
}

func (o *DeownHandlerOptions) setDefaults() {
	if o.CheckManagedAnnotation == nil {
		o.CheckManagedAnnotation = ptr.To(true)
	}
	if o.ObjectPredicate == nil {
		o.ObjectPredicate = DefaultObjectPredicate
	}
}

func (o *DeownHandlerOptions) shouldProcessObject(rr *odhTypes.ReconciliationRequest, obj unstructured.Unstructured) (bool, error) {
	// Check managed annotation
	if *o.CheckManagedAnnotation && resources.HasAnnotation(&obj, odhAnnotations.ManagedByODHOperator, "false") {
		return false, nil
	}

	owned, err := resources.IsOwnedByType(&obj, rr.Kind)
	if err != nil {
		return false, err
	}
	if !owned {
		return false, nil
	}

	ok, err := o.ObjectPredicate(rr, obj)
	if err != nil {
		return false, err
	}

	return ok, nil
}

func (o *DeownHandlerOptions) createOwnerRefPredicate(rr *odhTypes.ReconciliationRequest) func(metav1.OwnerReference) bool {
	// Filter by computed GVK
	return func(ownerRef metav1.OwnerReference) bool {
		// Parse the APIVersion to get Group and Version
		gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
		if err != nil {
			return false
		}

		return gv.Group == rr.Kind.Group &&
			gv.Version == rr.Kind.Version &&
			ownerRef.Kind == rr.Kind.Kind
	}
}

// DeownHandler creates a cleanup handler that removes owner references based on the provided options.
// If CheckOwnership is enabled, only owner references matching the instance GVK will be removed.
// If CheckOwnership is disabled, it behaves like RemoveOwnerRefHandler with a predicate that always returns true.
func DeownHandler(opts ...DeownHandlerOption) HandlerFn {
	config := DeownHandlerOptions{}

	for _, opt := range opts {
		opt.ApplyToDeownHandler(&config)
	}

	config.setDefaults()

	return func(ctx context.Context, rr *odhTypes.ReconciliationRequest, obj unstructured.Unstructured) (bool, error) {
		// Check if object should be processed based on handler options
		shouldProcess, err := config.shouldProcessObject(rr, obj)
		if err != nil {
			return false, fmt.Errorf("failed to check if object should be processed: %w", err)
		}
		if !shouldProcess {
			return false, nil
		}

		// Create the predicate for removing owner references
		predicate := config.createOwnerRefPredicate(rr)

		logf.FromContext(ctx).Info(
			"removing owner references",
			"object", obj.GetName(),
			"namespace", obj.GetNamespace(),
			"gvk", obj.GroupVersionKind(),
		)

		err = resources.RemoveOwnerReferences(ctx, rr.Client, &obj, predicate)
		if err != nil {
			return false, fmt.Errorf(
				"cannot remove owner references from object gvk: %s, namespace: %s, name: %s, reason: %w",
				obj.GroupVersionKind().String(),
				obj.GetNamespace(),
				obj.GetName(),
				err,
			)
		}

		DeownedTotal.WithLabelValues(
			strings.ToLower(rr.Kind.Kind),
		).Inc()

		return true, nil
	}
}
