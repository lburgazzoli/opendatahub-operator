package common

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func unmarshalCondition(cm map[string]any) Condition {
	c := Condition{}

	if v, ok := cm["type"].(string); ok {
		c.Type = v
	}
	if v, ok := cm["status"].(string); ok {
		c.Status = metav1.ConditionStatus(v)
	}
	if v, ok := cm["reason"].(string); ok {
		c.Reason = v
	}
	if v, ok := cm["message"].(string); ok {
		c.Message = v
	}
	if v, ok := cm["observedGeneration"].(int64); ok {
		c.ObservedGeneration = v
	} else if v, ok := cm["observedGeneration"].(float64); ok {
		c.ObservedGeneration = int64(v)
	}
	if v, ok := cm["lastTransitionTime"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			c.LastTransitionTime = metav1.NewTime(t)
		}
	}
	if v, ok := cm["severity"].(string); ok {
		c.Severity = ConditionSeverity(v)
	}

	return c
}

func marshalCondition(c Condition) map[string]any {
	m := map[string]any{
		"type":   c.Type,
		"status": string(c.Status),
	}

	if c.Reason != "" {
		m["reason"] = c.Reason
	}
	if c.Message != "" {
		m["message"] = c.Message
	}
	if c.ObservedGeneration != 0 {
		m["observedGeneration"] = c.ObservedGeneration
	}
	if !c.LastTransitionTime.IsZero() {
		m["lastTransitionTime"] = c.LastTransitionTime.UTC().Format(time.RFC3339)
	}
	if c.Severity != "" {
		m["severity"] = string(c.Severity)
	}

	return m
}
