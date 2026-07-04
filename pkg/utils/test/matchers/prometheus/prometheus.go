package prometheus

import (
	"fmt"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// HaveValue succeeds if the prometheus Collector's current scalar value
// equals expected. Works with Gauge, Counter, and any single-valued Collector.
func HaveValue(expected float64) types.GomegaMatcher {
	return &valueMatcher{expected: expected}
}

// HaveValueWith succeeds if the prometheus Collector's current scalar value
// satisfies the comparison (e.g. ">=", 2).
func HaveValueWith(comparator string, threshold float64) types.GomegaMatcher {
	return &numericMatcher{
		comparator: comparator,
		threshold:  threshold,
	}
}

// HaveGaugeVecValue extracts the child metric for the given label values
// from a *prometheus.GaugeVec and asserts its value equals expected.
func HaveGaugeVecValue(expected float64, labels ...string) types.GomegaMatcher {
	return &gaugeVecMatcher{expected: expected, labels: labels}
}

// GaugeVecValue returns a polling function suitable for use with
// g.Eventually(...). It reads the current value of the labeled child
// from the GaugeVec on each call.
func GaugeVecValue(vec *prometheus.GaugeVec, labels ...string) func() float64 {
	return func() float64 {
		return testutil.ToFloat64(vec.WithLabelValues(labels...))
	}
}

// ---------------------------------------------------------------------------
// valueMatcher — exact equality on a Collector
// ---------------------------------------------------------------------------

var _ types.GomegaMatcher = &valueMatcher{}

type valueMatcher struct {
	expected float64
	actual   float64
}

func (m *valueMatcher) Match(actual any) (bool, error) {
	c, ok := actual.(prometheus.Collector)
	if !ok {
		return false, fmt.Errorf("HaveValue expects a prometheus.Collector, got %T", actual)
	}
	m.actual = testutil.ToFloat64(c)
	return m.actual == m.expected, nil
}

func (m *valueMatcher) FailureMessage(_ any) string {
	return format.Message(m.actual, "to equal", m.expected)
}

func (m *valueMatcher) NegatedFailureMessage(_ any) string {
	return format.Message(m.actual, "not to equal", m.expected)
}

// ---------------------------------------------------------------------------
// numericMatcher — comparator-based check on a Collector
// ---------------------------------------------------------------------------

var _ types.GomegaMatcher = &numericMatcher{}

type numericMatcher struct {
	comparator string
	threshold  float64
	actual     float64
}

func (m *numericMatcher) Match(actual any) (bool, error) {
	c, ok := actual.(prometheus.Collector)
	if !ok {
		return false, fmt.Errorf("HaveValueWith expects a prometheus.Collector, got %T", actual)
	}
	m.actual = testutil.ToFloat64(c)

	switch m.comparator {
	case "==":
		return m.actual == m.threshold, nil
	case "!=":
		return m.actual != m.threshold, nil
	case "<":
		return m.actual < m.threshold, nil
	case "<=":
		return m.actual <= m.threshold, nil
	case ">":
		return m.actual > m.threshold, nil
	case ">=":
		return m.actual >= m.threshold, nil
	default:
		return false, fmt.Errorf("unsupported comparator %q", m.comparator)
	}
}

func (m *numericMatcher) FailureMessage(_ any) string {
	return format.Message(m.actual, fmt.Sprintf("to be %s", m.comparator), m.threshold)
}

func (m *numericMatcher) NegatedFailureMessage(_ any) string {
	return format.Message(m.actual, fmt.Sprintf("not to be %s", m.comparator), m.threshold)
}

// ---------------------------------------------------------------------------
// gaugeVecMatcher — labeled child value equality
// ---------------------------------------------------------------------------

var _ types.GomegaMatcher = &gaugeVecMatcher{}

type gaugeVecMatcher struct {
	expected float64
	labels   []string
	actual   float64
}

func (m *gaugeVecMatcher) Match(actual any) (bool, error) {
	vec, ok := actual.(*prometheus.GaugeVec)
	if !ok {
		return false, fmt.Errorf("HaveGaugeVecValue expects a *prometheus.GaugeVec, got %T", actual)
	}
	m.actual = testutil.ToFloat64(vec.WithLabelValues(m.labels...))
	return m.actual == m.expected, nil
}

func (m *gaugeVecMatcher) FailureMessage(_ any) string {
	return format.Message(m.actual, fmt.Sprintf("to equal (labels=%v)", m.labels), m.expected)
}

func (m *gaugeVecMatcher) NegatedFailureMessage(_ any) string {
	return format.Message(m.actual, fmt.Sprintf("not to equal (labels=%v)", m.labels), m.expected)
}
