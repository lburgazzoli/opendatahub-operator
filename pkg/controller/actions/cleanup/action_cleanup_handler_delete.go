package cleanup

import (
	"context"
	"fmt"
	"strings"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	odhTypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	odhAnnotations "github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

type DeleteHandlerOption interface {
	ApplyToDeleteHandler(opts *DeleteHandlerOptions)
}

type DeleteHandlerOptions struct {
	CheckManagedAnnotation *bool
	ObjectPredicate        ObjectPredicateFn
	CheckOwnership         *bool
	PropagationPolicy      metav1.DeletionPropagation
}

func (o *DeleteHandlerOptions) ApplyToDeleteHandler(config *DeleteHandlerOptions) {
	if o.CheckManagedAnnotation != nil {
		config.CheckManagedAnnotation = o.CheckManagedAnnotation
	}
	if o.ObjectPredicate != nil {
		config.ObjectPredicate = o.ObjectPredicate
	}
	if o.CheckOwnership != nil {
		config.CheckOwnership = o.CheckOwnership
	}
	if o.PropagationPolicy != "" {
		config.PropagationPolicy = o.PropagationPolicy
	}
}

func (o *DeleteHandlerOptions) setDefaults() {
	if o.CheckManagedAnnotation == nil {
		o.CheckManagedAnnotation = ptr.To(true)
	}
	if o.ObjectPredicate == nil {
		o.ObjectPredicate = DefaultObjectPredicate
	}
	if o.CheckOwnership == nil {
		o.CheckOwnership = ptr.To(true)
	}
	if o.PropagationPolicy == "" {
		o.PropagationPolicy = metav1.DeletePropagationForeground
	}
}

func (o *DeleteHandlerOptions) shouldProcessObject(rr *odhTypes.ReconciliationRequest, obj unstructured.Unstructured) (bool, error) {
	// Check managed annotation
	if *o.CheckManagedAnnotation && resources.HasAnnotation(&obj, odhAnnotations.ManagedByODHOperator, "false") {
		return false, nil
	}

	// Check ownership if enabled
	if *o.CheckOwnership {
		owned, err := resources.IsOwnedByType(&obj, rr.Kind)
		if err != nil {
			return false, err
		}
		if !owned {
			return false, nil
		}
	}

	ok, err := o.ObjectPredicate(rr, obj)
	if err != nil {
		return false, err
	}

	return ok, nil
}

type propagationPolicyOption struct {
	policy metav1.DeletionPropagation
}

func (o propagationPolicyOption) ApplyToDeleteHandler(config *DeleteHandlerOptions) {
	config.PropagationPolicy = o.policy
}

//nolint:ireturn
func WithPropagationPolicy(policy metav1.DeletionPropagation) DeleteHandlerOption {
	return propagationPolicyOption{policy: policy}
}

func DeleteHandler(opts ...DeleteHandlerOption) HandlerFn {
	config := DeleteHandlerOptions{}

	for _, opt := range opts {
		opt.ApplyToDeleteHandler(&config)
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

		logf.FromContext(ctx).Info(
			"delete",
			"gvk", obj.GroupVersionKind(),
			"ns", obj.GetNamespace(),
			"name", obj.GetName(),
		)

		err = rr.Client.Delete(ctx, &obj, client.PropagationPolicy(config.PropagationPolicy))
		if err != nil && !k8serr.IsNotFound(err) {
			return false, fmt.Errorf(
				"cannot delete resources gvk: %s, namespace: %s, name: %s, reason: %w",
				obj.GroupVersionKind().String(),
				obj.GetNamespace(),
				obj.GetName(),
				err,
			)
		}

		DeletedTotal.WithLabelValues(
			strings.ToLower(rr.Kind.Kind),
		).Inc()

		return true, nil
	}
}

// DefaultDeleteHandler creates a DeleteHandler with default settings for backward compatibility.
// It enables ownership checking (using the instance GVK from the reconciliation request)
// and managed annotation checking.
func DefaultDeleteHandler(propagationPolicy metav1.DeletionPropagation) HandlerFn {
	return DeleteHandler(
		&DeleteHandlerOptions{
			CheckManagedAnnotation: ptr.To(true),
			CheckOwnership:         ptr.To(true),
			PropagationPolicy:      propagationPolicy,
		},
	)
}
