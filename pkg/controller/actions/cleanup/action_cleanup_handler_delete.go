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

// SkipReason represents why a resource was skipped during cleanup.
type SkipReason string

const (
	ProcessResource     SkipReason = ""                 // Empty string means process the resource
	SkipUnmanaged       SkipReason = "unmanaged"        // Resource marked as unmanaged
	SkipNotOwned        SkipReason = "not_owned"        // Resource not owned by controller
	SkipPredicateFailed SkipReason = "predicate_failed" // Object predicate returned false
	SkipAlreadyDeleting SkipReason = "already_deleting" // Resource already being deleted
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

func (o *DeleteHandlerOptions) shouldProcessObject(rr *odhTypes.ReconciliationRequest, obj unstructured.Unstructured) (SkipReason, error) {
	// Skip deleting object being deleted
	if !obj.GetDeletionTimestamp().IsZero() {
		return SkipAlreadyDeleting, nil
	}

	// Check managed annotation
	if *o.CheckManagedAnnotation && resources.HasAnnotation(&obj, odhAnnotations.ManagedByODHOperator, "false") {
		return SkipUnmanaged, nil
	}

	// Check ownership if enabled
	if *o.CheckOwnership {
		owned, err := resources.IsOwnedByType(&obj, rr.Kind)
		if err != nil {
			return ProcessResource, err
		}
		if !owned {
			return SkipNotOwned, nil
		}
	}

	ok, err := o.ObjectPredicate(rr, obj)
	if err != nil {
		return ProcessResource, err
	}
	if !ok {
		return SkipPredicateFailed, nil
	}

	return ProcessResource, nil
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

// DeleteHandler creates a cleanup handler that deletes resources after applying
// validation checks in the following order:
//
// 1. Managed Annotation Check (if enabled):
//   - Skips resources with annotation "platform.opendatahub.io/managed: false"
//   - Default: enabled
//
// 2. Ownership Check (if enabled):
//   - Skips resources not owned by the reconciling instance
//   - Uses the instance GVK from ReconciliationRequest
//   - Default: enabled
//
// 3. Object Predicate:
//   - Custom logic for resource selection (version, generation, etc.)
//   - Default: DefaultObjectPredicate (checks platform metadata)
//
// ANY check returning false will skip the resource. Checks short-circuit on first false.
//
// Common patterns:
//
//	DeleteHandler() - Strict mode: only deletes owned, managed, outdated resources
//	DeleteHandler(WithOwnershipCheck(false)) - Deletes managed, outdated resources regardless of ownership
//	DeleteHandler(WithManagedAnnotationCheck(false)) - Deletes owned, outdated resources regardless of managed flag
func DeleteHandler(opts ...DeleteHandlerOption) HandlerFn {
	config := DeleteHandlerOptions{
		CheckManagedAnnotation: ptr.To(true),
		ObjectPredicate:        DefaultObjectPredicate,
		CheckOwnership:         ptr.To(true),
		PropagationPolicy:      metav1.DeletePropagationForeground,
	}

	for _, opt := range opts {
		opt.ApplyToDeleteHandler(&config)
	}

	return func(ctx context.Context, rr *odhTypes.ReconciliationRequest, obj unstructured.Unstructured) (bool, error) {
		l := logf.FromContext(ctx)

		// Check if object should be processed based on handler options
		skipReason, err := config.shouldProcessObject(rr, obj)
		if err != nil {
			return false, fmt.Errorf("failed to check if object should be processed: %w", err)
		}

		if skipReason != ProcessResource {
			l.V(2).Info("skipping cleanup",
				"gvk", obj.GroupVersionKind(),
				"obj", resources.FormatObjectName(&obj),
				"reason", string(skipReason),
			)
			return false, nil
		}

		l.Info("delete",
			"gvk", obj.GroupVersionKind(),
			"obj", resources.FormatObjectName(&obj),
		)

		err = rr.Client.Delete(ctx, &obj, client.PropagationPolicy(config.PropagationPolicy))
		if err != nil && !k8serr.IsNotFound(err) {
			return false, fmt.Errorf(
				"cannot delete resources gvk: %s, obj: %s, reason: %w",
				obj.GroupVersionKind(),
				resources.FormatObjectName(&obj),
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
//
// Behavior:
// - Respects managed annotations (skips unmanaged resources)
// - Requires ownership by the reconciling instance
// - Uses DefaultObjectPredicate for version/generation checks
//
// Equivalent to DeleteHandler() with the specified propagation policy.
func DefaultDeleteHandler(propagationPolicy metav1.DeletionPropagation) HandlerFn {
	return DeleteHandler(
		&DeleteHandlerOptions{
			CheckManagedAnnotation: ptr.To(true),
			CheckOwnership:         ptr.To(true),
			PropagationPolicy:      propagationPolicy,
		},
	)
}
