package reconciler

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/handlers"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/component"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

type forInput struct {
	object     client.Object
	gvk        schema.GroupVersionKind
	predicates []predicate.Predicate
}

type DynamicPredicate func(context.Context, *types.ReconciliationRequest) bool

type watchInput struct {
	object       client.Object
	gvk          schema.GroupVersionKind
	eventHandler handler.EventHandler
	predicates   []predicate.Predicate
	owned        bool
	dynamic      bool
	dynamicPred  []DynamicPredicate
}

type WatchOpts func(*watchInput)

func WithPredicates(values ...predicate.Predicate) WatchOpts {
	return func(a *watchInput) {
		a.predicates = append(a.predicates, values...)
	}
}

func WithEventHandler(value handler.EventHandler) WatchOpts {
	return func(a *watchInput) {
		a.eventHandler = value
	}
}

func WithEventMapper(value handler.MapFunc) WatchOpts {
	return func(a *watchInput) {
		a.eventHandler = handler.EnqueueRequestsFromMapFunc(value)
	}
}

func Dynamic(predicates ...DynamicPredicate) WatchOpts {
	return func(a *watchInput) {
		a.dynamic = true
		a.dynamicPred = slices.Clone(predicates)
	}
}

type ForOpts func(input *forInput)

func WithForPredicates(values ...predicate.Predicate) ForOpts {
	return func(a *forInput) {
		a.predicates = append(a.predicates, values...)
	}
}

type ReconcilerBuilder[T common.PlatformObject] struct {
	mgr                 types.ControllerManager
	input               forInput
	watches             []watchInput
	predicates          []predicate.Predicate
	instanceName        string
	actions             []actions.Fn
	finalizers          []actions.Fn
	errors              error
	happyCondition      string
	dependantConditions []string
}

func ReconcilerFor[T common.PlatformObject](mgr types.ControllerManager, object T, opts ...ForOpts) *ReconcilerBuilder[T] {
	crb := ReconcilerBuilder[T]{
		mgr:                 mgr,
		happyCondition:      status.ConditionTypeReady,
		dependantConditions: []string{status.ConditionTypeProvisioningSucceeded},
	}

	gvk, err := resources.GetGroupVersionKindForObject(mgr.GetScheme(), object)
	if err != nil {
		crb.errors = multierror.Append(crb.errors, fmt.Errorf("unable to determine GVK: %w", err))
	}

	crb.input = forInput{
		object: object,
		gvk:    gvk,
	}

	for _, opt := range opts {
		opt(&crb.input)
	}

	if len(crb.input.predicates) == 0 {
		crb.input.predicates = append(crb.input.predicates, predicates.DefaultPredicate)
	}

	return &crb
}

func (b *ReconcilerBuilder[T]) WithConditions(dependants ...string) *ReconcilerBuilder[T] {
	b.dependantConditions = append(b.dependantConditions, dependants...)
	return b
}

func (b *ReconcilerBuilder[T]) WithInstanceName(instanceName string) *ReconcilerBuilder[T] {
	b.instanceName = instanceName
	return b
}

func (b *ReconcilerBuilder[T]) WithAction(value actions.Fn) *ReconcilerBuilder[T] {
	b.actions = append(b.actions, value)
	return b
}

func (b *ReconcilerBuilder[T]) WithFinalizer(value actions.Fn) *ReconcilerBuilder[T] {
	b.finalizers = append(b.finalizers, value)
	return b
}

func (b *ReconcilerBuilder[T]) Watches(object client.Object, opts ...WatchOpts) *ReconcilerBuilder[T] {
	gvk, err := resources.GetGroupVersionKindForObject(b.mgr.GetScheme(), object)
	if err != nil {
		b.errors = multierror.Append(b.errors, fmt.Errorf("unable to determine GVK: %w", err))
		return b
	}

	in := watchInput{}
	in.object = object
	in.owned = false
	in.gvk = gvk

	for _, opt := range opts {
		opt(&in)
	}

	if in.eventHandler == nil {
		// use the platform.opendatahub.io/instance.name label to find out
		// the owner
		in.eventHandler = handlers.AnnotationToName(annotations.InstanceName)
	}

	if len(in.predicates) == 0 {
		in.predicates = append(in.predicates, predicate.And(
			predicates.DefaultPredicate,
			// use the platform.opendatahub.io/part-of label to filter
			// events not related to the owner type
			component.ForLabel(labels.PlatformPartOf, strings.ToLower(b.input.gvk.Kind)),
		))
	}

	b.watches = append(b.watches, in)

	return b
}

func (b *ReconcilerBuilder[T]) WatchesGVK(gvk schema.GroupVersionKind, opts ...WatchOpts) *ReconcilerBuilder[T] {
	return b.Watches(resources.GvkToUnstructured(gvk), opts...)
}

func (b *ReconcilerBuilder[T]) Owns(object client.Object, opts ...WatchOpts) *ReconcilerBuilder[T] {
	gvk, err := resources.GetGroupVersionKindForObject(b.mgr.GetScheme(), object)
	if err != nil {
		b.errors = multierror.Append(b.errors, fmt.Errorf("unable to determine GVK: %w", err))
		return b
	}

	in := watchInput{}
	in.object = object
	in.owned = true
	in.gvk = gvk

	for _, opt := range opts {
		opt(&in)
	}

	if in.eventHandler == nil {
		in.eventHandler = handler.EnqueueRequestForOwner(
			b.mgr.GetScheme(),
			b.mgr.GetRESTMapper(),
			b.input.object,
			handler.OnlyControllerOwner(),
		)
	}

	if len(in.predicates) == 0 {
		in.predicates = append(in.predicates, predicates.DefaultPredicate)
	}

	b.watches = append(b.watches, in)

	return b
}

func (b *ReconcilerBuilder[T]) WithEventFilter(p predicate.Predicate) *ReconcilerBuilder[T] {
	b.predicates = append(b.predicates, p)
	return b
}

func (b *ReconcilerBuilder[T]) OwnsGVK(gvk schema.GroupVersionKind, opts ...WatchOpts) *ReconcilerBuilder[T] {
	return b.Owns(resources.GvkToUnstructured(gvk), opts...)
}

func (b *ReconcilerBuilder[T]) Build(_ context.Context) (*Reconciler, error) {
	if b.errors != nil {
		return nil, b.errors
	}
	name := b.instanceName
	if name == "" {
		name = strings.ToLower(b.input.gvk.Kind)
	}

	obj, ok := b.input.object.(T)
	if !ok {
		return nil, errors.New("invalid type for object")
	}

	r, err := NewReconciler(b.mgr, name, obj, WithConditionsManagerFactory(b.happyCondition, b.dependantConditions...))
	if err != nil {
		return nil, fmt.Errorf("failed to create reconciler for component %s: %w", name, err)
	}

	c := ctrl.NewControllerManagedBy(b.mgr)
	c = c.Named(name)

	c = c.WatchesRawSource(b.mgr.Source(
		b.input.object,
		&handler.EnqueueRequestForObject{},
		b.input.predicates...,
	))

	for _, w := range b.watches {
		if w.owned {
			kinds, _, err := b.mgr.GetScheme().ObjectKinds(w.object)
			if err != nil {
				return nil, err
			}

			for i := range kinds {
				r.AddOwnedType(kinds[i])
			}
		}

		// if the watch is dynamic, then the watcher will be registered
		// at later stage
		if w.dynamic {
			continue
		}

		c = c.WatchesRawSource(b.mgr.Source(
			w.object,
			w.eventHandler,
			w.predicates...,
		))
	}

	for i := range b.predicates {
		c = c.WithEventFilter(b.predicates[i])
	}

	for i := range b.actions {
		r.AddAction(b.actions[i])
	}
	for i := range b.finalizers {
		r.AddFinalizer(b.finalizers[i])
	}

	cc, err := c.Build(r)
	if err != nil {
		return nil, err
	}

	// internal action
	r.AddAction(
		newDynamicWatchAction(
			func(in watchInput) error {
				return cc.Watch(b.mgr.Source(
					in.object,
					in.eventHandler,
					in.predicates...,
				))
			},
			b.watches,
		),
	)

	return r, nil
}
