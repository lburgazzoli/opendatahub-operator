package reconciler

import (
	"context"
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"slices"
	"strings"

	"github.com/hashicorp/go-multierror"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/handlers"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/component"
	pr "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

type forInput struct {
	object         client.Object
	gvk            schema.GroupVersionKind
	predicates     []predicate.Predicate
	wrapPredicates bool
}

func (in forInput) Predicates(scheme *runtime.Scheme) []predicate.Predicate {
	if !in.wrapPredicates {
		return in.predicates
	}

	return []predicate.Predicate{
		// ensure that the predicates are receiving the expected type,
		pr.TypeAdapter(scheme, in.object, in.predicates...),
	}
}

type DynamicPredicate func(context.Context, *types.ReconciliationRequest) bool

type watchInput struct {
	object           client.Object
	gvk              schema.GroupVersionKind
	eventHandler     handler.EventHandler
	wrapEventHandler bool
	predicates       []predicate.Predicate
	wrapPredicates   bool
	owned            bool
	dynamic          bool
	dynamicPred      []DynamicPredicate
}

func (in watchInput) Predicates(scheme *runtime.Scheme) []predicate.Predicate {
	if !in.wrapPredicates {
		return in.predicates
	}

	return []predicate.Predicate{
		// ensure that the predicates are receiving the expected type,
		pr.TypeAdapter(scheme, in.object, in.predicates...),
	}
}

func (in watchInput) EventHandler(scheme *runtime.Scheme) handler.EventHandler {
	if !in.wrapEventHandler {
		return in.eventHandler
	}

	// ensure that the handler is receiving the expected type,
	return handlers.TypedAdapter(scheme, in.object, in.eventHandler)
}

type WatchOpts func(*watchInput)

func WithPredicates(values ...predicate.Predicate) WatchOpts {
	return func(a *watchInput) {
		a.predicates = append(a.predicates, values...)
	}
}
func WithPredicatesWrapper(value bool) WatchOpts {
	return func(a *watchInput) {
		a.wrapPredicates = value
	}
}

func WithEventHandler(value handler.EventHandler) WatchOpts {
	return func(a *watchInput) {
		a.eventHandler = value
	}
}
func WithEventHandlerWrapper(value bool) WatchOpts {
	return func(a *watchInput) {
		a.wrapEventHandler = value
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
	mgr                 ctrl.Manager
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

func ReconcilerFor[T common.PlatformObject](mgr ctrl.Manager, object T, opts ...ForOpts) *ReconcilerBuilder[T] {
	crb := ReconcilerBuilder[T]{
		mgr:                 mgr,
		happyCondition:      status.ConditionTypeReady,
		dependantConditions: []string{status.ConditionTypeProvisioningSucceeded},
	}

	gvk, err := mgr.GetClient().GroupVersionKindFor(object)
	if err != nil {
		crb.errors = multierror.Append(crb.errors, fmt.Errorf("unable to determine GVK: %w", err))
		return &crb
	}

	crb.input = forInput{}
	crb.input.object = object
	crb.input.gvk = gvk

	for _, opt := range opts {
		opt(&crb.input)
	}

	if len(crb.input.predicates) == 0 {
		crb.input.predicates = append(crb.input.predicates, predicates.DefaultPredicate)
	} else {
		crb.input.wrapPredicates = true
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

func (b *ReconcilerBuilder[T]) WatchesGVK(gvk schema.GroupVersionKind, opts ...WatchOpts) *ReconcilerBuilder[T] {
	return b.Watches(resources.GvkToUnstructured(gvk), opts...)
}

func (b *ReconcilerBuilder[T]) Watches(obj client.Object, opts ...WatchOpts) *ReconcilerBuilder[T] {
	gvk, err := resources.GetGroupVersionKindForObject(b.mgr.GetScheme(), obj)
	if err != nil {
		b.errors = multierror.Append(b.errors, fmt.Errorf("unable to determine GVK: %w", err))
		return b
	}

	in := watchInput{}
	in.object = obj
	in.gvk = gvk
	in.owned = false

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
			// The envelope of object(s) that is propagated to the predicate, depends on
			// how the watch is being configured, so as an example, if the watcher is
			// set up using an Unstructured object, then the event would carry an Unstructured
			// object as well, this may lead to some misbehavior so this adapter ensures that
			// the event carries the object in a type expected by the consumer
			component.ForLabel(labels.PlatformPartOf, strings.ToLower(b.input.gvk.Kind)),
		))
	}

	b.watches = append(b.watches, in)

	return b
}

func (b *ReconcilerBuilder[T]) OwnsGVK(gvk schema.GroupVersionKind, opts ...WatchOpts) *ReconcilerBuilder[T] {
	return b.Owns(resources.GvkToUnstructured(gvk), opts...)
}

func (b *ReconcilerBuilder[T]) Owns(obj client.Object, opts ...WatchOpts) *ReconcilerBuilder[T] {
	gvk, err := resources.GetGroupVersionKindForObject(b.mgr.GetScheme(), obj)
	if err != nil {
		b.errors = multierror.Append(b.errors, fmt.Errorf("unable to determine GVK: %w", err))
		return b
	}

	in := watchInput{}
	in.object = obj
	in.gvk = gvk
	in.owned = true

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

	c := ctrl.NewControllerManagedBy(b.mgr).
		For(
			resources.GvkToUnstructured(b.input.gvk),
			builder.WithPredicates(
				b.input.Predicates(b.mgr.GetScheme())...,
			),
		)

	for i := range b.watches {
		if b.watches[i].owned {
			r.AddOwnedType(b.watches[i].gvk)
		}

		// if the watch is dynamic, then the watcher will be registered
		// at later stage
		if b.watches[i].dynamic {
			continue
		}

		c = c.Watches(
			resources.GvkToUnstructured(b.watches[i].gvk),
			b.watches[i].EventHandler(b.mgr.GetScheme()),
			builder.WithPredicates(
				b.watches[i].Predicates(b.mgr.GetScheme())...,
			),
		)
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
			func(w watchInput) error {
				src := source.TypedKind[client.Object](
					b.mgr.GetCache(),
					resources.GvkToUnstructured(w.gvk),
					w.EventHandler(b.mgr.GetScheme()),
					w.Predicates(b.mgr.GetScheme())...,
				)

				return cc.Watch(src)
			},
			b.watches,
		),
	)

	return r, nil
}
