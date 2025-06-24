package cleanup

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// DeletedTotal is a prometheus counter metrics which holds the total number
	// of resource deleted by the cleanup action per controller. It has one label.
	// controller label refers to the controller name.
	DeletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "action_cleanup_deleted_total",
			Help: "Number of cleanup deleted resources",
		},
		[]string{
			"controller",
		},
	)

	// DeownedTotal is a prometheus counter metrics which holds the total number
	// of resources that had owner references removed by the cleanup action per controller.
	// It has one label. controller label refers to the controller name.
	DeownedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "action_cleanup_deowned_total",
			Help: "Number of resources that had owner references removed",
		},
		[]string{
			"controller",
		},
	)

	// CyclesTotal is a prometheus counter metrics which holds the total number
	// cleanup cycles per controller. It has one label.
	// controller label refers to the controller name.
	CyclesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "action_cleanup_cycles_total",
			Help: "Number of cleanup cycles",
		},
		[]string{
			"controller",
		},
	)
)

// init register metrics to the global registry from controller-runtime/pkg/metrics.
// see https://book.kubebuilder.io/reference/metrics#publishing-additional-metrics
//
//nolint:gochecknoinits
func init() {
	metrics.Registry.MustRegister(DeletedTotal)
	metrics.Registry.MustRegister(DeownedTotal)
	metrics.Registry.MustRegister(CyclesTotal)
}
