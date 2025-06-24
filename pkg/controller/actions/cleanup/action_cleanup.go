package cleanup

import (
	"context"
	"fmt"
	"strings"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	odhTypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	odhLabels "github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/rules"
)

type HandlerFn func(ctx context.Context, rr *odhTypes.ReconciliationRequest, obj unstructured.Unstructured) (bool, error)
type TypePredicateFn func(*odhTypes.ReconciliationRequest, schema.GroupVersionKind) (bool, error)
type ActionOpts func(*Action)

type Action struct {
	labels           map[string]string
	selector         labels.Selector
	protectedTypes   map[schema.GroupVersionKind]struct{}
	typePredicateFn  TypePredicateFn
	cleanupHandlerFn HandlerFn
	namespaceFn      actions.StringGetter
}

func WithLabel(name string, value string) ActionOpts {
	return func(action *Action) {
		if action.labels == nil {
			action.labels = map[string]string{}
		}

		action.labels[name] = value
	}
}

func WithLabels(values map[string]string) ActionOpts {
	return func(action *Action) {
		if action.labels == nil {
			action.labels = map[string]string{}
		}

		for k, v := range values {
			action.labels[k] = v
		}
	}
}

func WithProtectedTypes(items ...schema.GroupVersionKind) ActionOpts {
	return func(action *Action) {
		for _, item := range items {
			action.protectedTypes[item] = struct{}{}
		}
	}
}

func WithTypePredicate(value TypePredicateFn) ActionOpts {
	return func(action *Action) {
		if value == nil {
			return
		}

		action.typePredicateFn = value
	}
}

func WithHandler(value HandlerFn) ActionOpts {
	return func(action *Action) {
		if value == nil {
			return
		}

		action.cleanupHandlerFn = value
	}
}

func InNamespace(ns string) ActionOpts {
	return func(action *Action) {
		action.namespaceFn = func(_ context.Context, _ *odhTypes.ReconciliationRequest) (string, error) {
			return ns, nil
		}
	}
}

func InNamespaceFn(fn actions.StringGetter) ActionOpts {
	return func(action *Action) {
		if fn == nil {
			return
		}
		action.namespaceFn = fn
	}
}

func (a *Action) run(ctx context.Context, rr *odhTypes.ReconciliationRequest) error {
	// To avoid the expensive cleanup, run it only when resources have
	// been generated
	if !rr.Generated {
		return nil
	}

	l := logf.FromContext(ctx)

	// TODO: use cacher to avoid computing cleanable types
	//       on each run
	items, err := a.computeCleanableTypes(ctx, rr)
	if err != nil {
		return fmt.Errorf("unable to refresh cleanable resources: %w", err)
	}

	controllerName := strings.ToLower(rr.Kind.Kind)

	CyclesTotal.WithLabelValues(controllerName).Inc()

	lo := metav1.ListOptions{
		LabelSelector: a.getOrComputeSelector(controllerName).String(),
	}

	l.V(3).Info("run", "selector", lo.LabelSelector)

	for _, res := range items {
		canBeCleaned, err := a.isTypeCleanable(rr, res.GroupVersionKind())
		if err != nil {
			return fmt.Errorf("cannot determine if resource %s can be cleaned: %w", res.String(), err)
		}

		if !canBeCleaned {
			continue
		}

		items, err := a.listResources(ctx, rr.Controller.GetDynamicClient(), res, lo)
		if err != nil {
			return fmt.Errorf("cannot list child resources %s: %w", res.String(), err)
		}

		if len(items) == 0 {
			continue
		}

		if err = a.cleanupResources(ctx, rr, items); err != nil {
			return fmt.Errorf("error processing items to cleanup: %w", err)
		}
	}

	return nil
}

func (a *Action) computeCleanableTypes(ctx context.Context, rr *odhTypes.ReconciliationRequest) ([]resources.Resource, error) {
	res, err := resources.ListAvailableAPIResources(rr.Controller.GetDiscoveryClient())
	if err != nil {
		return nil, fmt.Errorf("failure discovering resources: %w", err)
	}

	ns, err := a.namespaceFn(ctx, rr)
	if err != nil {
		return nil, fmt.Errorf("unable to compute namespace: %w", err)
	}

	items, err := rules.ListAuthorizedDeletableResources(ctx, rr.Client, res, ns)
	if err != nil {
		return nil, fmt.Errorf("failure listing authorized deletable resources: %w", err)
	}

	return items, nil
}

func (a *Action) listResources(
	ctx context.Context,
	dc dynamic.Interface,
	res resources.Resource,
	opts metav1.ListOptions,
) ([]unstructured.Unstructured, error) {
	items, err := dc.Resource(res.GroupVersionResource()).Namespace("").List(ctx, opts)
	switch {
	case k8serr.IsForbidden(err) || k8serr.IsMethodNotSupported(err) || k8serr.IsNotFound(err):
		logf.FromContext(ctx).V(3).Info(
			"cannot list resource",
			"reason", err.Error(),
			"gvk", res.GroupVersionKind(),
		)

		return nil, nil
	case err != nil:
		return nil, err
	default:
		return items.Items, nil
	}
}

func (a *Action) isTypeCleanable(
	rr *odhTypes.ReconciliationRequest,
	gvk schema.GroupVersionKind,
) (bool, error) {
	if a.isProtectedType(gvk) {
		return false, nil
	}

	return a.typePredicateFn(rr, gvk)
}

func (a *Action) cleanupResources(
	ctx context.Context,
	rr *odhTypes.ReconciliationRequest,
	items []unstructured.Unstructured,
) error {
	for i := range items {
		// Skip protected types
		if a.isProtectedType(items[i].GroupVersionKind()) {
			continue
		}

		// Skip objects that are already being deleted
		if !items[i].GetDeletionTimestamp().IsZero() {
			continue
		}

		_, err := a.cleanupHandlerFn(ctx, rr, items[i])
		if err != nil {
			return fmt.Errorf("cleanup handler failed for object %s in namespace %q: %w",
				items[i].GetName(),
				items[i].GetNamespace(),
				err,
			)
		}
	}

	return nil
}

// getOrComputeSelector returns the existing label selector if provided, or, it generates
// a new selector using the provided value and 'platform.opendatahub.io/part-of' as a key.
//
// Parameters:
//   - controllerName: the name of the controller to associate with the selector.
//
// Returns:
//   - labels.Selector: either the cached selector or a newly constructed one.
func (a *Action) getOrComputeSelector(partOf string) labels.Selector {
	if a.selector != nil {
		return a.selector
	}

	return labels.SelectorFromSet(map[string]string{
		odhLabels.PlatformPartOf: partOf,
	})
}

func (a *Action) isProtectedType(gvk schema.GroupVersionKind) bool {
	_, ok := a.protectedTypes[gvk]
	return ok
}

func NewAction(opts ...ActionOpts) actions.Fn {
	action := Action{}
	action.typePredicateFn = DefaultTypePredicate
	action.cleanupHandlerFn = DefaultDeleteHandler(metav1.DeletePropagationForeground)
	action.namespaceFn = actions.OperatorNamespace

	// default protected types
	action.protectedTypes = make(map[schema.GroupVersionKind]struct{})
	action.protectedTypes[gvk.CustomResourceDefinition] = struct{}{}
	action.protectedTypes[gvk.Lease] = struct{}{}

	for _, opt := range opts {
		opt(&action)
	}

	if len(action.labels) > 0 {
		action.selector = labels.SelectorFromSet(action.labels)
	}

	return action.run
}
