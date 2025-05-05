package handlers

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/metrics"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

func LabelToName(key string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
		values := a.GetLabels()
		if len(values) == 0 {
			return []reconcile.Request{}
		}

		name := values[key]
		if name == "" {
			return []reconcile.Request{}
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: name,
			},
		}}
	})
}

func AnnotationToName(key string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		values := obj.GetAnnotations()
		if len(values) == 0 {
			return []reconcile.Request{}
		}

		name := values[key]
		if name == "" {
			return []reconcile.Request{}
		}

		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: name,
			},
		}}
	})
}

func Fn(fn func(ctx context.Context, a client.Object) []reconcile.Request) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(fn)
}

func ToNamed(name string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name: name,
			},
		}}
	})
}

func RequestFromObject() handler.EventHandler {
	return Fn(func(ctx context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{{
			NamespacedName: resources.NamespacedNameFromObject(obj),
		}}
	})
}

type typedHandleAdapterFn struct {
	CreateFunc  func(context.Context, event.TypedCreateEvent[client.Object], workqueue.TypedRateLimitingInterface[reconcile.Request])
	UpdateFunc  func(context.Context, event.TypedUpdateEvent[client.Object], workqueue.TypedRateLimitingInterface[reconcile.Request])
	DeleteFunc  func(context.Context, event.TypedDeleteEvent[client.Object], workqueue.TypedRateLimitingInterface[reconcile.Request])
	GenericFunc func(context.Context, event.TypedGenericEvent[client.Object], workqueue.TypedRateLimitingInterface[reconcile.Request])
}

func (h typedHandleAdapterFn) Create(
	ctx context.Context,
	e event.TypedCreateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	if h.DeleteFunc != nil {
		h.CreateFunc(ctx, e, q)
	}
}

func (h typedHandleAdapterFn) Update(
	ctx context.Context,
	e event.TypedUpdateEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	if h.UpdateFunc != nil {
		h.UpdateFunc(ctx, e, q)
	}
}

func (h typedHandleAdapterFn) Delete(
	ctx context.Context,
	e event.TypedDeleteEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	if h.DeleteFunc != nil {
		h.DeleteFunc(ctx, e, q)
	}
}

func (h typedHandleAdapterFn) Generic(
	ctx context.Context,
	e event.TypedGenericEvent[client.Object],
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	if h.GenericFunc != nil {
		h.GenericFunc(ctx, e, q)
	}
}

func TypedAdapter(
	s *runtime.Scheme,
	obj client.Object,
	h handler.EventHandler,
) handler.EventHandler {
	// keep a copy of the input object as a template
	ref := obj.DeepCopyObject()

	convert := func(in client.Object, out *client.Object) bool {
		_, inIsPartialMeta := in.(*metav1.PartialObjectMetadata)
		_, outIsPartialMeta := (*out).(*metav1.PartialObjectMetadata)
		if inIsPartialMeta && outIsPartialMeta {
			*out, _ = in.DeepCopyObject().(*metav1.PartialObjectMetadata)
			return true
		}

		if err := s.Convert(in, *out, nil); err != nil {
			return false
		}

		metrics.ConvertedResourcesTotal.WithLabelValues(
			in.GetObjectKind().GroupVersionKind().GroupVersion().String(),
			in.GetObjectKind().GroupVersionKind().Kind,
			"handler",
		).Inc()

		return true
	}

	return typedHandleAdapterFn{
		GenericFunc: func(
			ctx context.Context,
			e event.TypedGenericEvent[client.Object],
			q workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			if e.Object == nil {
				h.Generic(ctx, e, q)
			}

			ne := event.TypedGenericEvent[client.Object]{}
			ne.Object, _ = ref.DeepCopyObject().(client.Object)

			if !convert(e.Object, &ne.Object) {
				return
			}

			h.Generic(ctx, ne, q)
		},
		CreateFunc: func(
			ctx context.Context,
			e event.TypedCreateEvent[client.Object],
			q workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			if e.Object == nil {
				h.Create(ctx, e, q)
			}

			ne := event.TypedCreateEvent[client.Object]{}
			ne.Object, _ = ref.DeepCopyObject().(client.Object)

			if !convert(e.Object, &ne.Object) {
				return
			}

			h.Create(ctx, ne, q)
		},
		DeleteFunc: func(
			ctx context.Context,
			e event.TypedDeleteEvent[client.Object],
			q workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			if e.Object == nil {
				h.Delete(ctx, e, q)
			}

			ne := event.TypedDeleteEvent[client.Object]{}
			ne.Object, _ = ref.DeepCopyObject().(client.Object)

			if !convert(e.Object, &ne.Object) {
				return
			}

			h.Delete(ctx, ne, q)
		},
		UpdateFunc: func(
			ctx context.Context,
			e event.TypedUpdateEvent[client.Object],
			q workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			if e.ObjectOld == nil && e.ObjectNew == nil {
				h.Update(ctx, e, q)
			}

			ne := event.TypedUpdateEvent[client.Object]{}

			if e.ObjectOld != nil {
				ne.ObjectOld, _ = ref.DeepCopyObject().(client.Object)

				if !convert(e.ObjectOld, &ne.ObjectOld) {
					return
				}
			}

			if e.ObjectNew != nil {
				ne.ObjectNew, _ = ref.DeepCopyObject().(client.Object)

				if !convert(e.ObjectNew, &ne.ObjectNew) {
					return
				}
			}

			h.Update(ctx, ne, q)
		},
	}
}

// NewEventHandlerForGVK creates an event handler that watches for events on resources of the specified GroupVersionKind.
// It uses the provided client to list resources and applies any additional list options.
//
// Parameters:
//   - cli: The Kubernetes client used to list resources
//   - gvk: The GroupVersionKind to watch for events
//   - options: Optional list options to filter the resources (e.g., namespace, label selectors)
//
// Returns:
//   - handler.EventHandler: An event handler that can be used with controller-runtime's controller.Watch
//
// Example:
//
//	gvk := schema.GroupVersionKind{
//		Group:   "apps",
//		Version: "v1",
//		Kind:    "Deployment",
//	}
//
//	ctrl.Watch(
//		&source.Kind{Type: &appsv1.Deployment{}},
//		NewEventHandlerForGVK(
//			cli,
//			gvk,
//		),
//	)
func NewEventHandlerForGVK(
	cli client.Client,
	gvk schema.GroupVersionKind,
	options ...client.ListOption,
) handler.EventHandler {
	return Fn(func(ctx context.Context, _ client.Object) []reconcile.Request {
		list := unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)

		err := cli.List(ctx, &list, options...)
		switch {
		case err != nil:
			return []reconcile.Request{}
		case len(list.Items) == 0:
			return []reconcile.Request{}
		default:
			requests := make([]reconcile.Request, len(list.Items))
			for i := range list.Items {
				requests[i] = reconcile.Request{
					NamespacedName: resources.NamespacedNameFromObject(&list.Items[i]),
				}
			}

			return requests
		}
	})
}
