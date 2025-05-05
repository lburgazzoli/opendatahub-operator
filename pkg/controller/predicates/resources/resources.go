package resources

import (
	"reflect"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/metrics"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/generation"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

func NewDeploymentPredicate() predicate.Predicate {
	return predicate.Or(
		generation.New(),
		PathDriftPredicate([]string{
			"status.replicas",
			"status.readyReplicas",
		}),
	)
}

func Deleted() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// Content predicates moved from original controller.
var CMContentChangedPredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldCM, _ := e.ObjectOld.(*corev1.ConfigMap)
		newCM, _ := e.ObjectNew.(*corev1.ConfigMap)
		return !reflect.DeepEqual(oldCM.Data, newCM.Data)
	},
}

var SecretContentChangedPredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldSecret, _ := e.ObjectOld.(*corev1.Secret)
		newSecret, _ := e.ObjectNew.(*corev1.Secret)
		return !reflect.DeepEqual(oldSecret.Data, newSecret.Data)
	},
}

var DSCDeletionPredicate = predicate.Funcs{
	DeleteFunc: func(e event.DeleteEvent) bool {
		return true
	},
}

var DSCComponentUpdatePredicate = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldDSC, ok := e.ObjectOld.(*dscv1.DataScienceCluster)
		if !ok {
			return false
		}
		newDSC, ok := e.ObjectNew.(*dscv1.DataScienceCluster)
		if !ok {
			return false
		}
		// if .spec.components is changed, return true.
		if !reflect.DeepEqual(oldDSC.Spec.Components, newDSC.Spec.Components) {
			return true
		}

		// if new condition from component is added or removed, return true
		oldConditions := oldDSC.Status.Conditions
		newConditions := newDSC.Status.Conditions
		if len(oldConditions) != len(newConditions) {
			return true
		}

		// compare type one by one with their status if not equal return true
		for _, nc := range newConditions {
			for _, oc := range oldConditions {
				if nc.Type == oc.Type {
					if !reflect.DeepEqual(nc.Status, oc.Status) {
						return true
					}
				}
			}
		}
		return false
	},
}

var DSCIReadiness = predicate.Funcs{
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldObj, ok := e.ObjectOld.(*dsciv1.DSCInitialization)
		if !ok {
			return false
		}
		newObj, ok := e.ObjectNew.(*dsciv1.DSCInitialization)
		if !ok {
			return false
		}

		return oldObj.Status.Phase != newObj.Status.Phase
	},
	CreateFunc: func(e event.CreateEvent) bool {
		return false
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return false
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return false
	},
}

func AnnotationChanged(name string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return resources.GetAnnotation(e.ObjectNew, name) != resources.GetAnnotation(e.ObjectOld, name)
		},
	}
}

// PathDriftPredicate creates a predicate that checks for drifts in specific paths of unstructured objects.
// It automatically converts any client.Object to unstructured.Unstructured if needed.
//
// The predicate checks for changes in the specified paths between the old and new object states.
// Paths should be specified as dot-separated strings (e.g., "spec.template.spec.containers").
//
// By default, the predicate accepts both create and delete events. This behavior can be configured
// using PredicateOption functions:
//   - WithAcceptCreate(): Enable create event handling
//   - WithAcceptDelete(): Enable delete event handling
//
// For update events, the predicate:
//   - Converts both objects to unstructured if they aren't already
//   - Checks each specified path for changes
//   - Returns true if any path has changed or if the path exists in one object but not the other
//   - Skips paths that cannot be accessed due to errors
//
// Example usage:
//
//	predicate := PathDriftPredicate(
//	    []string{
//	        "spec.template.spec.containers",
//	        "metadata.labels",
//	    },
//	    WithAcceptCreate(),
//	    WithAcceptDelete(),
//	)
func PathDriftPredicate(paths []string, opts ...predicates.PredicateOption) predicate.Funcs {
	// Pre-process paths to avoid splitting them in every update
	pathParts := make([][]string, len(paths))
	for i, path := range paths {
		pathParts[i] = strings.Split(path, ".")
	}

	// Default options - both create and delete events are accepted by default
	options := predicates.PredicateOptions{
		AcceptCreate: true,
		AcceptDelete: true,
	}

	// Apply provided options
	for _, opt := range opts {
		opt(&options)
	}

	return predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return options.AcceptCreate
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return options.AcceptDelete
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			switch {
			case e.ObjectOld == nil && e.ObjectNew == nil:
				return false
			case e.ObjectOld == nil || e.ObjectNew == nil:
				return true
			}

			// Convert both objects to unstructured
			oldUnstructured, err := resources.ToUnstructured(e.ObjectOld)
			if err != nil {
				return false
			}
			newUnstructured, err := resources.ToUnstructured(e.ObjectNew)
			if err != nil {
				return false
			}

			// Check each path for drift
			for _, parts := range pathParts {
				// Get values from both objects
				oldValue, oldFound, oldErr := unstructured.NestedFieldNoCopy(oldUnstructured.Object, parts...)
				newValue, newFound, newErr := unstructured.NestedFieldNoCopy(newUnstructured.Object, parts...)

				// Handle the different cases for field access
				switch {
				case oldErr != nil || newErr != nil:
					// Skip this path if there was an error accessing either object
					continue

				case oldFound != newFound:
					// Path exists in one object but not the other
					return true

				case !oldFound && !newFound:
					// Neither path exists in both objects
					continue

				case !reflect.DeepEqual(oldValue, newValue):
					return true
				}
			}

			return false
		},
	}
}

func WithPredicates(
	s *runtime.Scheme,
	obj client.Object,
	preds ...predicate.Predicate,
) builder.Predicates {
	return builder.WithPredicates(
		TypeAdapter(s, obj, preds...),
	)
}

func TypeAdapter(
	s *runtime.Scheme,
	obj client.Object,
	preds ...predicate.Predicate,
) predicate.Funcs {
	return TypedPredicateAdapter(
		s,
		obj,
		// safety copy
		predicate.And(
			slices.Clone(preds)...,
		),
	)
}

// TypedPredicateAdapter creates a predicate.Funcs adapter that handles type conversion and
// forwarding.
//
// It will:
// - Forward events directly if the object is already of type T
// - Convert unstructured objects to requested type if possible
// - Return false for any other type
// - Use PredicateOptions to configure create and delete event handling
//
// Example usage:
//
//		predicate := TypedPredicateAdapter(
//	        scheme,
//	        appsv1.Deployment{},
//		    predicate.TypedPredicate{
//		        CreateFunc: func(e event.CreateEvent) bool {
//		            return true
//		        },
//		    },
//		    WithAcceptCreate(),
//		    WithAcceptDelete(),
//		)
func TypedPredicateAdapter(
	s *runtime.Scheme,
	obj client.Object,
	p predicate.Predicate,
) predicate.Funcs {
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
			"predicate",
		).Inc()

		return true
	}

	return predicate.Funcs{
		GenericFunc: func(e event.GenericEvent) bool {
			if e.Object == nil {
				return p.Generic(e)
			}

			ne := event.GenericEvent{}
			ne.Object, _ = ref.DeepCopyObject().(client.Object)

			if !convert(e.Object, &ne.Object) {
				return false
			}

			return p.Generic(ne)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			if e.Object == nil {
				return p.Create(e)
			}

			ne := event.CreateEvent{}
			ne.Object, _ = ref.DeepCopyObject().(client.Object)

			if !convert(e.Object, &ne.Object) {
				return false
			}

			return p.Create(ne)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Object == nil {
				return p.Delete(e)
			}

			ne := event.DeleteEvent{}
			ne.Object, _ = ref.DeepCopyObject().(client.Object)

			if !convert(e.Object, &ne.Object) {
				return false
			}

			return p.Delete(ne)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil && e.ObjectNew == nil {
				return p.Update(e)
			}

			ne := event.UpdateEvent{}

			if e.ObjectOld != nil {
				ne.ObjectOld, _ = ref.DeepCopyObject().(client.Object)

				if !convert(e.ObjectOld, &ne.ObjectOld) {
					return false
				}
			}

			if e.ObjectNew != nil {
				ne.ObjectNew, _ = ref.DeepCopyObject().(client.Object)

				if !convert(e.ObjectNew, &ne.ObjectNew) {
					return false
				}
			}

			return p.Update(ne)
		},
	}
}

func CreatedOrDeleted() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
		// disabled
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
