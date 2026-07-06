package prometheus_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	prom "github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/prometheus"

	. "github.com/onsi/gomega"
)

func TestHaveValue_Gauge(t *testing.T) {
	g := NewWithT(t)

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_value",
		Help: "test",
	})

	gauge.Set(42)
	g.Expect(gauge).Should(prom.HaveValue(42))

	gauge.Set(0)
	g.Expect(gauge).Should(prom.HaveValue(0))
	g.Expect(gauge).ShouldNot(prom.HaveValue(1))
}

func TestHaveValue_Counter(t *testing.T) {
	g := NewWithT(t)

	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_total",
		Help: "test",
	})

	g.Expect(counter).Should(prom.HaveValue(0))

	counter.Add(5)
	g.Expect(counter).Should(prom.HaveValue(5))
	g.Expect(counter).ShouldNot(prom.HaveValue(3))
}

func TestHaveValue_RejectsNonCollector(t *testing.T) {
	g := NewWithT(t)
	_, err := prom.HaveValue(1).Match("not-a-collector")
	g.Expect(err).To(HaveOccurred())
}

func TestHaveValueWith(t *testing.T) {
	g := NewWithT(t)

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_numeric_value",
		Help: "test",
	})

	gauge.Set(10)

	g.Expect(gauge).Should(prom.HaveValueWith(">=", 5))
	g.Expect(gauge).Should(prom.HaveValueWith(">=", 10))
	g.Expect(gauge).Should(prom.HaveValueWith("<=", 10))
	g.Expect(gauge).Should(prom.HaveValueWith("==", 10))
	g.Expect(gauge).Should(prom.HaveValueWith(">", 9))
	g.Expect(gauge).Should(prom.HaveValueWith("<", 11))
	g.Expect(gauge).Should(prom.HaveValueWith("!=", 5))

	g.Expect(gauge).ShouldNot(prom.HaveValueWith(">", 10))
	g.Expect(gauge).ShouldNot(prom.HaveValueWith("<", 10))
}

func TestHaveValueWith_UnsupportedComparator(t *testing.T) {
	g := NewWithT(t)

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_bad_compare",
		Help: "test",
	})
	_, err := prom.HaveValueWith("~", 1).Match(gauge)
	g.Expect(err).To(HaveOccurred())
}

func TestHaveGaugeVecValue(t *testing.T) {
	g := NewWithT(t)

	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_vec",
		Help: "test",
	}, []string{"level", "status"})

	vec.WithLabelValues("10", "processed").Set(1)
	vec.WithLabelValues("10", "blocked").Set(0)
	vec.WithLabelValues("20", "blocked").Set(1)

	g.Expect(vec).Should(prom.HaveGaugeVecValue(1, "10", "processed"))
	g.Expect(vec).Should(prom.HaveGaugeVecValue(0, "10", "blocked"))
	g.Expect(vec).Should(prom.HaveGaugeVecValue(1, "20", "blocked"))
	g.Expect(vec).ShouldNot(prom.HaveGaugeVecValue(1, "20", "processed"))
}

func TestHaveGaugeVecValue_RejectsNonGaugeVec(t *testing.T) {
	g := NewWithT(t)

	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "test_plain",
		Help: "test",
	})
	_, err := prom.HaveGaugeVecValue(1, "foo").Match(gauge)
	g.Expect(err).To(HaveOccurred())
}

func TestGaugeVecValue_PollingFunction(t *testing.T) {
	g := NewWithT(t)

	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "test_poll",
		Help: "test",
	}, []string{"key"})

	vec.WithLabelValues("a").Set(7)

	fn := prom.GaugeVecValue(vec, "a")
	g.Expect(fn()).To(Equal(float64(7)))

	vec.WithLabelValues("a").Set(99)
	g.Expect(fn()).To(Equal(float64(99)))
}
