package common

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ PlatformObject = &UnstructuredPlatformObject{}

// UnstructuredPlatformObject adapts an *unstructured.Unstructured to the
// PlatformObject interface, providing typed access to .status, .status.phase,
// .status.observedGeneration, and .status.conditions without requiring a
// Go type for the underlying CR.
type UnstructuredPlatformObject struct {
	*unstructured.Unstructured
}

// NewUnstructuredPlatformObject wraps an existing Unstructured as a PlatformObject.
func NewUnstructuredPlatformObject(u *unstructured.Unstructured) *UnstructuredPlatformObject {
	return &UnstructuredPlatformObject{Unstructured: u}
}

func (o *UnstructuredPlatformObject) GetStatus() *Status {
	s := &Status{}

	if v, ok, _ := unstructured.NestedString(o.Object, "status", "phase"); ok {
		s.Phase = v
	}

	if v, ok, _ := unstructured.NestedInt64(o.Object, "status", "observedGeneration"); ok {
		s.ObservedGeneration = v
	} else if v, ok, _ := unstructured.NestedFloat64(o.Object, "status", "observedGeneration"); ok {
		s.ObservedGeneration = int64(v)
	}

	s.Conditions = o.GetConditions()

	return s
}

func (o *UnstructuredPlatformObject) GetConditions() []Condition {
	rawConditions, found, err := unstructured.NestedSlice(o.Object, "status", "conditions")
	if err != nil || !found {
		return nil
	}

	conds := make([]Condition, 0, len(rawConditions))

	for _, raw := range rawConditions {
		cm, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		c := unmarshalCondition(cm)
		if c.Type == "" || c.Status == "" {
			continue
		}

		conds = append(conds, c)
	}

	return conds
}

func (o *UnstructuredPlatformObject) SetConditions(conditions []Condition) {
	raw := make([]any, len(conditions))
	for i := range conditions {
		raw[i] = marshalCondition(conditions[i])
	}

	_ = unstructured.SetNestedSlice(o.Object, raw, "status", "conditions")
}

func (o *UnstructuredPlatformObject) DeepCopyObject() runtime.Object {
	return &UnstructuredPlatformObject{
		Unstructured: o.Unstructured.DeepCopy(),
	}
}
