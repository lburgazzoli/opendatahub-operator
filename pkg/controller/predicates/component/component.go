package component

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
)

func ForComponent(legacyComponentName string, componentName string) predicate.Funcs {
	label := labels.ODH.Component(legacyComponentName)
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			labelList := e.Object.GetLabels()

			if value, exist := labelList[labels.ComponentName]; exist && value == componentName {
				return true
			}
			if value, exist := labelList[label]; exist && value == "true" {
				return true
			}

			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldLabels := e.ObjectOld.GetLabels()

			if value, exist := oldLabels[labels.ComponentName]; exist && value == componentName {
				return true
			}
			if value, exist := oldLabels[label]; exist && value == "true" {
				return true
			}

			newLabels := e.ObjectNew.GetLabels()

			if value, exist := newLabels[labels.ComponentName]; exist && value == componentName {
				return true
			}
			if value, exist := newLabels[label]; exist && value == "true" {
				return true
			}

			return false
		},
	}
}
