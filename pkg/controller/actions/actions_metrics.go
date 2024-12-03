package actions

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	metricsutil "github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/metrics"
)

const (
	ActionLabel = "action"
)

var (
	// ActionExecutionTime is a prometheus metric which keeps track of the duration of an action per controller.
	// It has two labels.
	// controller label refers to the controller name.
	// action label refers to the action name.
	ActionExecutionTime = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "action_execution_time_seconds",
		Help:    "Length of execution time per action per controller",
		Buckets: prometheus.ExponentialBuckets(10e-9, 10, 10)},
		[]string{
			metricsutil.ControllerLabel,
			ActionLabel,
		},
	)
)

// init register metrics to the global registry from controller-runtime/pkg/metrics.
// see https://book.kubebuilder.io/reference/metrics#publishing-additional-metrics
//
//nolint:gochecknoinits
func init() {
	metrics.Registry.MustRegister(ActionExecutionTime)
}
