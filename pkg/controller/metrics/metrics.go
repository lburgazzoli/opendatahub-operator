package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

//nolint:promlinter
var StoredResourcesTotal = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "stored_resources_total",
		Help: "TODO",
	},
	[]string{
		"name",
		"apiVersion",
		"kind",
		"envelope",
	},
)

//nolint:promlinter
var ConvertedResourcesTotal = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "converted_resources_total",
		Help: "TODO",
	},
	[]string{
		"apiVersion",
		"kind",
		"context",
	},
)

//nolint:gochecknoinits
func init() {
	metrics.Registry.MustRegister(StoredResourcesTotal)
	metrics.Registry.MustRegister(ConvertedResourcesTotal)
}

type StoredResource struct {
	schema.GroupVersionKind
	envelope string
}

func NewInstrumentedInformerFn(
	scheme *runtime.Scheme,
	name string,
) func(cache.ListerWatcher, runtime.Object, time.Duration, cache.Indexers) cache.SharedIndexInformer {
	handlers := make(map[StoredResource]struct{})
	handlerM := sync.Mutex{}

	return func(watcher cache.ListerWatcher, obj runtime.Object, duration time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
		objGVK, err := apiutil.GVKForObject(obj, scheme)
		if err != nil {
			panic(err)
		}

		sres := StoredResource{
			GroupVersionKind: objGVK,
		}

		kind := objGVK.Kind
		apiVersion := objGVK.GroupVersion().String()

		i := cache.NewSharedIndexInformer(watcher, obj, duration, indexers)

		handlerM.Lock()
		defer handlerM.Unlock()

		switch obj.(type) {
		case *metav1.PartialObjectMetadata:
			sres.envelope = "partial"
		case *unstructured.Unstructured:
			sres.envelope = "unstructured"
		default:
			sres.envelope = "typed"
		}

		if _, ok := handlers[sres]; !ok {
			_, err = i.AddEventHandler(cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					StoredResourcesTotal.WithLabelValues(name, apiVersion, kind, sres.envelope).Inc()
				},
				DeleteFunc: func(obj interface{}) {
					StoredResourcesTotal.WithLabelValues(name, apiVersion, kind, sres.envelope).Dec()
				},
			})

			handlers[sres] = struct{}{}
		}

		if err != nil {
			panic(err)
		}

		return i
	}
}
