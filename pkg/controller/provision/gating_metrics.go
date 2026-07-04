package provision

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	LabelRunlevel = "runlevel"
	LabelStatus   = "status"

	StatusPending   = "pending"
	StatusProcessed = "processed"
	StatusBlocked   = "blocked"
	StatusTimedOut  = "timed_out"
)

var (
	// RunlevelStatus is an info-style gauge reporting the state of each
	// runlevel in the DAG. For each runlevel, exactly one status label
	// has value 1; the rest are 0.
	RunlevelStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "odh_dag_runlevel_status",
			Help: "Per-runlevel DAG state: 1 for the active status, 0 for others",
		},
		[]string{LabelRunlevel, LabelStatus},
	)

	// RunlevelDurationSeconds reports how long (in seconds) a runlevel
	// has been in the DAG walk, measured from the walk start time. For
	// processed runlevels this is the elapsed time at completion; for
	// blocked runlevels this is the elapsed time at the gating check.
	RunlevelDurationSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "odh_dag_runlevel_duration_seconds",
			Help: "Seconds the runlevel has been in its current state",
		},
		[]string{LabelRunlevel},
	)

	// RunlevelCleared reports the highest runlevel order that has been
	// fully processed in the current walk.
	RunlevelCleared = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "odh_dag_runlevel_cleared",
			Help: "Highest runlevel order fully processed in the current DAG walk",
		},
	)

	// RunlevelBlocked reports the runlevel order currently blocked.
	// 0 when not blocked.
	RunlevelBlocked = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "odh_dag_runlevel_blocked",
			Help: "Runlevel order currently blocked on (0 = not blocked)",
		},
	)

	// BatchesProcessedTotal is incremented for each batch successfully
	// processed by WalkBatches.
	BatchesProcessedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "odh_dag_batches_processed_total",
			Help: "Cumulative number of DAG batches processed",
		},
	)

	// RunlevelTimeoutTotal is incremented each time a runlevel times
	// out and is force-advanced.
	RunlevelTimeoutTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "odh_dag_runlevel_timeout_total",
			Help: "Number of times a runlevel timed out and was skipped",
		},
		[]string{LabelRunlevel},
	)
)

// setRunlevelStatusByLabel sets the active status to 1 and all others
// to 0 for the given pre-computed runlevel label string.
func setRunlevelStatusByLabel(rl string, active string) {
	switch active {
	case StatusPending:
		RunlevelStatus.WithLabelValues(rl, StatusPending).Set(1)
		RunlevelStatus.WithLabelValues(rl, StatusProcessed).Set(0)
		RunlevelStatus.WithLabelValues(rl, StatusBlocked).Set(0)
		RunlevelStatus.WithLabelValues(rl, StatusTimedOut).Set(0)
	case StatusProcessed:
		RunlevelStatus.WithLabelValues(rl, StatusPending).Set(0)
		RunlevelStatus.WithLabelValues(rl, StatusProcessed).Set(1)
		RunlevelStatus.WithLabelValues(rl, StatusBlocked).Set(0)
		RunlevelStatus.WithLabelValues(rl, StatusTimedOut).Set(0)
	case StatusBlocked:
		RunlevelStatus.WithLabelValues(rl, StatusPending).Set(0)
		RunlevelStatus.WithLabelValues(rl, StatusProcessed).Set(0)
		RunlevelStatus.WithLabelValues(rl, StatusBlocked).Set(1)
		RunlevelStatus.WithLabelValues(rl, StatusTimedOut).Set(0)
	case StatusTimedOut:
		RunlevelStatus.WithLabelValues(rl, StatusPending).Set(0)
		RunlevelStatus.WithLabelValues(rl, StatusProcessed).Set(0)
		RunlevelStatus.WithLabelValues(rl, StatusBlocked).Set(0)
		RunlevelStatus.WithLabelValues(rl, StatusTimedOut).Set(1)
	}
}

//nolint:gochecknoinits
func init() {
	metrics.Registry.MustRegister(
		RunlevelStatus,
		RunlevelDurationSeconds,
		RunlevelCleared,
		RunlevelBlocked,
		BatchesProcessedTotal,
		RunlevelTimeoutTotal,
	)
}
