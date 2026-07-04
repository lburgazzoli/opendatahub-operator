package common_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"

	. "github.com/onsi/gomega"
)

func TestGetConditions_ParsesFromUnstructured(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "test"},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"type":               "Ready",
					"status":             "True",
					"reason":             "AllGood",
					"message":            "everything is fine",
					"observedGeneration": int64(3),
					"lastTransitionTime": "2025-01-01T00:00:00Z",
				},
				map[string]any{
					"type":   "Progressing",
					"status": "False",
					"reason": "Idle",
				},
			},
		},
	}}

	obj := common.NewUnstructuredPlatformObject(u)
	conds := obj.GetConditions()

	g.Expect(conds).To(HaveLen(2))

	g.Expect(conds[0].Type).To(Equal("Ready"))
	g.Expect(conds[0].Status).To(Equal(metav1.ConditionTrue))
	g.Expect(conds[0].Reason).To(Equal("AllGood"))
	g.Expect(conds[0].Message).To(Equal("everything is fine"))
	g.Expect(conds[0].ObservedGeneration).To(Equal(int64(3)))
	g.Expect(conds[0].LastTransitionTime.IsZero()).To(BeFalse())

	g.Expect(conds[1].Type).To(Equal("Progressing"))
	g.Expect(conds[1].Status).To(Equal(metav1.ConditionFalse))
	g.Expect(conds[1].Reason).To(Equal("Idle"))
}

func TestGetConditions_EmptyOnMissingStatus(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "test"},
	}}

	obj := common.NewUnstructuredPlatformObject(u)
	g.Expect(obj.GetConditions()).To(BeNil())
}

func TestGetConditions_SkipsIncompleteEntries(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "test"},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready"},
				map[string]any{"status": "True"},
				map[string]any{"type": "Valid", "status": "True"},
			},
		},
	}}

	obj := common.NewUnstructuredPlatformObject(u)
	conds := obj.GetConditions()
	g.Expect(conds).To(HaveLen(1))
	g.Expect(conds[0].Type).To(Equal("Valid"))
}

func TestGetConditions_ObservedGenerationFloat64(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "test"},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"type":               "Ready",
					"status":             "True",
					"observedGeneration": float64(7),
				},
			},
		},
	}}

	obj := common.NewUnstructuredPlatformObject(u)
	g.Expect(obj.GetConditions()[0].ObservedGeneration).To(Equal(int64(7)))
}

func TestGetStatus_PhaseAndObservedGeneration(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "test"},
		"status": map[string]any{
			"phase":              "Running",
			"observedGeneration": int64(5),
		},
	}}

	obj := common.NewUnstructuredPlatformObject(u)
	s := obj.GetStatus()

	g.Expect(s.Phase).To(Equal("Running"))
	g.Expect(s.ObservedGeneration).To(Equal(int64(5)))
	g.Expect(s.Conditions).To(BeNil())
}

func TestGetStatus_ObservedGenerationFloat64(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "test"},
		"status": map[string]any{
			"observedGeneration": float64(42),
		},
	}}

	obj := common.NewUnstructuredPlatformObject(u)
	g.Expect(obj.GetStatus().ObservedGeneration).To(Equal(int64(42)))
}

func TestSetConditions_WritesBack(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "test"},
	}}

	obj := common.NewUnstructuredPlatformObject(u)
	obj.SetConditions([]common.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "Test",
			Message:            "hello",
			ObservedGeneration: 2,
			LastTransitionTime: metav1.NewTime(metav1.Now().Time),
		},
	})

	conds := obj.GetConditions()
	g.Expect(conds).To(HaveLen(1))
	g.Expect(conds[0].Type).To(Equal("Ready"))
	g.Expect(conds[0].Status).To(Equal(metav1.ConditionTrue))
	g.Expect(conds[0].Reason).To(Equal("Test"))
	g.Expect(conds[0].Message).To(Equal("hello"))
	g.Expect(conds[0].ObservedGeneration).To(Equal(int64(2)))

	rawConds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	g.Expect(rawConds).To(HaveLen(1))
}

func TestSetConditions_OmitsZeroValues(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "test"},
	}}

	obj := common.NewUnstructuredPlatformObject(u)
	obj.SetConditions([]common.Condition{
		{Type: "Ready", Status: metav1.ConditionTrue},
	})

	rawConds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	m := rawConds[0].(map[string]any)
	g.Expect(m).NotTo(HaveKey("reason"))
	g.Expect(m).NotTo(HaveKey("message"))
	g.Expect(m).NotTo(HaveKey("observedGeneration"))
	g.Expect(m).NotTo(HaveKey("lastTransitionTime"))
	g.Expect(m).NotTo(HaveKey("severity"))
}

func TestSetConditions_PreservesSeverity(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "test"},
	}}

	obj := common.NewUnstructuredPlatformObject(u)
	obj.SetConditions([]common.Condition{
		{Type: "Info", Status: metav1.ConditionFalse, Severity: common.ConditionSeverityInfo},
	})

	conds := obj.GetConditions()
	g.Expect(conds[0].Severity).To(Equal(common.ConditionSeverityInfo))
}

func TestDeepCopyObject_ReturnsSeparateCopy(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "original"},
	}}

	obj := common.NewUnstructuredPlatformObject(u)
	clone := obj.DeepCopyObject()

	cloneObj, ok := clone.(*common.UnstructuredPlatformObject)
	g.Expect(ok).To(BeTrue())
	g.Expect(cloneObj.GetName()).To(Equal("original"))

	cloneObj.SetName("modified")
	g.Expect(obj.GetName()).To(Equal("original"))
}

func TestPlatformObjectInterface(t *testing.T) {
	g := NewWithT(t)

	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "test/v1",
		"kind":       "Foo",
		"metadata":   map[string]any{"name": "test"},
	}}

	var po common.PlatformObject = common.NewUnstructuredPlatformObject(u)
	g.Expect(po.GetName()).To(Equal("test"))
	g.Expect(po.GetStatus()).NotTo(BeNil())
}
