package datasciencecluster

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

// computeComponentsStatus checks the status of all registered components in a DataScienceCluster instance
// and updates the status condition accordingly.
//
// Parameters:
// - ctx: The context for managing request deadlines and cancellation.
// - instance: The DataScienceCluster instance being reconciled.
// - reg: The registry containing all component handlers.
//
// Returns:
// - error: An error if any component status retrieval or update fails.
func computeComponentsStatus(
	ctx context.Context,
	rr *types.ReconciliationRequest,
	reg *cr.Registry,
) error {
	instance, ok := rr.Instance.(*dscv2.DataScienceCluster)
	if !ok {
		return errors.New("failed to convert to DataScienceCluster")
	}

	notReadyComponents := make([]string, 0)
	managedComponent := 0

	err := reg.ForEach(func(component cr.ComponentHandler) error {
		cs, err := component.UpdateDSCStatus(ctx, rr)
		if err != nil {
			notReadyComponents = append(notReadyComponents, component.GetName())
			return err
		}

		enabled := component.IsEnabled(instance)
		if !enabled && cs != metav1.ConditionFalse {
			return nil
		}

		if enabled {
			managedComponent++
		}

		if cs != metav1.ConditionTrue {
			notReadyComponents = append(notReadyComponents, component.GetName())
		}

		return nil
	})

	switch {
	case len(notReadyComponents) > 0:
		rr.Conditions.SetCondition(common.Condition{
			Type:    status.ConditionTypeComponentsReady,
			Status:  metav1.ConditionFalse,
			Reason:  status.NotReadyReason,
			Message: fmt.Sprintf("Some components are not ready: %s", strings.Join(notReadyComponents, ", ")),
		})
	case managedComponent == 0:
		rr.Conditions.SetCondition(common.Condition{
			Type:     status.ConditionTypeComponentsReady,
			Status:   metav1.ConditionTrue,
			Severity: common.ConditionSeverityInfo,
			Reason:   status.NoManagedComponentsReason,
			Message:  "All registered components have ManagementState Removed or are not configured",
		})
	default:
		rr.Conditions.MarkTrue(status.ConditionTypeComponentsReady)
	}

	if err != nil {
		return err
	}

	return nil
}

func filterInternalPlatformReleases(components *dscv2.ComponentsStatus) {
	if components == nil {
		return
	}

	filterPlatformReleaseValue(reflect.ValueOf(components))
}

func filterPlatformReleaseValue(value reflect.Value) {
	if !value.IsValid() {
		return
	}

	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return
		}
		filterPlatformReleaseValue(value.Elem())
		return
	}

	if value.Kind() != reflect.Struct {
		return
	}

	valueType := value.Type()
	for i := range value.NumField() {
		fieldValue := value.Field(i)
		fieldType := valueType.Field(i)

		if fieldType.Name == "Releases" && fieldValue.CanSet() {
			filtered := reflect.MakeSlice(fieldValue.Type(), 0, fieldValue.Len())
			for j := range fieldValue.Len() {
				entry := fieldValue.Index(j)
				name := entry.FieldByName("Name")
				if name.IsValid() && name.Kind() == reflect.String && name.String() == "platform" {
					continue
				}
				filtered = reflect.Append(filtered, entry)
			}
			fieldValue.Set(filtered)
		}

		filterPlatformReleaseValue(fieldValue)
	}
}

func syncComponentDAGStateForDSC(
	instance *dscv2.DataScienceCluster,
	reg *cr.Registry,
	provisionReg *provision.UnifiedRegistry,
) error {
	if instance == nil {
		return errors.New("failed to convert to DataScienceCluster")
	}

	return reg.ForAll(func(handler cr.ComponentHandler, registryEnabled bool) error {
		enabled := registryEnabled && handler.IsEnabled(instance)
		if enabled {
			provisionReg.Enable(handler.GetName())
		} else {
			provisionReg.Disable(handler.GetName())
		}
		return nil
	})
}

func projectPlatformModulesFromComponents(components dscv2.Components) configv1alpha1.PlatformModules {
	var projected configv1alpha1.PlatformModules

	value := reflect.ValueOf(components)
	valueType := value.Type()

	for i := range value.NumField() {
		field := valueType.Field(i)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		if jsonName == "" || jsonName == "-" {
			continue
		}

		componentValue := value.Field(i)
		managementStateField := componentValue.FieldByName("ManagementState")
		if !managementStateField.IsValid() {
			continue
		}

		managementState, ok := managementStateField.Interface().(operatorv1.ManagementState)
		if !ok || managementState == "" {
			managementState = operatorv1.Removed
		}

		projected.Set(configv1alpha1.PlatformModuleConfig{
			Name:            jsonName,
			ManagementState: managementState,
		})
	}

	return projected
}
