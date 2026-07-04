package common

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/gomega"
)

func TestUnmarshalCondition_AllFields(t *testing.T) {
	g := NewWithT(t)

	m := map[string]any{
		"type":               "Ready",
		"status":             "True",
		"reason":             "AllGood",
		"message":            "all systems go",
		"observedGeneration": int64(5),
		"lastTransitionTime": "2025-06-15T10:30:00Z",
		"severity":           "Info",
	}

	c := unmarshalCondition(m)

	g.Expect(c.Type).To(Equal("Ready"))
	g.Expect(c.Status).To(Equal(metav1.ConditionTrue))
	g.Expect(c.Reason).To(Equal("AllGood"))
	g.Expect(c.Message).To(Equal("all systems go"))
	g.Expect(c.ObservedGeneration).To(Equal(int64(5)))
	g.Expect(c.LastTransitionTime.IsZero()).To(BeFalse())
	g.Expect(c.Severity).To(Equal(ConditionSeverityInfo))
}

func TestUnmarshalCondition_MinimalFields(t *testing.T) {
	g := NewWithT(t)

	m := map[string]any{
		"type":   "Progressing",
		"status": "False",
	}

	c := unmarshalCondition(m)

	g.Expect(c.Type).To(Equal("Progressing"))
	g.Expect(c.Status).To(Equal(metav1.ConditionFalse))
	g.Expect(c.Reason).To(BeEmpty())
	g.Expect(c.Message).To(BeEmpty())
	g.Expect(c.ObservedGeneration).To(BeZero())
	g.Expect(c.LastTransitionTime.IsZero()).To(BeTrue())
	g.Expect(c.Severity).To(BeEmpty())
}

func TestUnmarshalCondition_ObservedGenerationFloat64(t *testing.T) {
	g := NewWithT(t)

	m := map[string]any{
		"type":               "Ready",
		"status":             "True",
		"observedGeneration": float64(42),
	}

	c := unmarshalCondition(m)
	g.Expect(c.ObservedGeneration).To(Equal(int64(42)))
}

func TestUnmarshalCondition_InvalidTimestamp(t *testing.T) {
	g := NewWithT(t)

	m := map[string]any{
		"type":               "Ready",
		"status":             "True",
		"lastTransitionTime": "not-a-timestamp",
	}

	c := unmarshalCondition(m)
	g.Expect(c.LastTransitionTime.IsZero()).To(BeTrue())
}

func TestUnmarshalCondition_EmptyMap(t *testing.T) {
	g := NewWithT(t)

	c := unmarshalCondition(map[string]any{})
	g.Expect(c.Type).To(BeEmpty())
	g.Expect(c.Status).To(BeEmpty())
}

func TestMarshalCondition_AllFields(t *testing.T) {
	g := NewWithT(t)

	c := Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "Test",
		Message:            "hello",
		ObservedGeneration: 3,
		LastTransitionTime: metav1.NewTime(metav1.Now().Time),
		Severity:           ConditionSeverityInfo,
	}

	m := marshalCondition(c)

	g.Expect(m["type"]).To(Equal("Ready"))
	g.Expect(m["status"]).To(Equal("True"))
	g.Expect(m["reason"]).To(Equal("Test"))
	g.Expect(m["message"]).To(Equal("hello"))
	g.Expect(m["observedGeneration"]).To(Equal(int64(3)))
	g.Expect(m).To(HaveKey("lastTransitionTime"))
	g.Expect(m["severity"]).To(Equal("Info"))
}

func TestMarshalCondition_OmitsZeroValues(t *testing.T) {
	g := NewWithT(t)

	c := Condition{
		Type:   "Ready",
		Status: metav1.ConditionTrue,
	}

	m := marshalCondition(c)

	g.Expect(m).To(HaveKey("type"))
	g.Expect(m).To(HaveKey("status"))
	g.Expect(m).NotTo(HaveKey("reason"))
	g.Expect(m).NotTo(HaveKey("message"))
	g.Expect(m).NotTo(HaveKey("observedGeneration"))
	g.Expect(m).NotTo(HaveKey("lastTransitionTime"))
	g.Expect(m).NotTo(HaveKey("severity"))
}

func TestMarshalUnmarshal_RoundTrip(t *testing.T) {
	g := NewWithT(t)

	original := Condition{
		Type:               "Available",
		Status:             metav1.ConditionFalse,
		Reason:             "Deploying",
		Message:            "waiting for pods",
		ObservedGeneration: 7,
		LastTransitionTime: metav1.NewTime(metav1.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC).Time),
		Severity:           ConditionSeverityInfo,
	}

	m := marshalCondition(original)
	restored := unmarshalCondition(m)

	g.Expect(restored.Type).To(Equal(original.Type))
	g.Expect(restored.Status).To(Equal(original.Status))
	g.Expect(restored.Reason).To(Equal(original.Reason))
	g.Expect(restored.Message).To(Equal(original.Message))
	g.Expect(restored.ObservedGeneration).To(Equal(original.ObservedGeneration))
	g.Expect(restored.Severity).To(Equal(original.Severity))
	g.Expect(restored.LastTransitionTime.UTC()).To(Equal(original.LastTransitionTime.UTC()))
}
