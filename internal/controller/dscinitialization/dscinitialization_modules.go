package dscinitialization

import (
	"context"
	"fmt"
	"reflect"

	operatorv1 "github.com/openshift/api/operator/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

func (r *DSCInitializationReconciler) moduleRegistry() *modules.Registry {
	if r.ModuleRegistry != nil {
		return r.ModuleRegistry
	}

	return modules.DefaultRegistry()
}

func (r *DSCInitializationReconciler) buildServicePlatformContext(
	ctx context.Context,
	instance *dsciv2.DSCInitialization,
) *modules.PlatformContext {
	platformCtx := &modules.PlatformContext{
		ApplicationsNamespace: instance.Spec.ApplicationsNamespace,
		Release:               cluster.GetRelease(),
		DSCI:                  instance,
	}

	if gatewayDomain, err := resources.GetGatewayDomain(ctx, r.Client); err == nil {
		platformCtx.GatewayDomain = gatewayDomain
	}

	return platformCtx
}

func (r *DSCInitializationReconciler) syncPlatformServices(
	ctx context.Context,
	instance *dsciv2.DSCInitialization,
) error {
	reg := r.moduleRegistry()
	if !reg.HasEntries() {
		return nil
	}

	platform := &configv1alpha1.Platform{}
	platform.Name = configv1alpha1.PlatformInstanceName
	platform.TypeMeta = metav1.TypeMeta{
		APIVersion: configv1alpha1.GroupVersion.String(),
		Kind:       configv1alpha1.PlatformKind,
	}

	platform.Spec.Modules = projectPlatformModulesFromTaggedSpec(instance.Spec)

	if err := resources.Apply(
		ctx,
		r.Client,
		platform,
		client.FieldOwner("dscinitialization"),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("failed to patch Platform modules from DSCI: %w", err)
	}

	return nil
}

func (r *DSCInitializationReconciler) provisionServiceModuleCRs(
	ctx context.Context,
	instance *dsciv2.DSCInitialization,
) error {
	reg := r.moduleRegistry()
	if !reg.HasEntries() {
		return nil
	}

	managed := modules.ManagedModuleNames(instance.Spec)
	platformCtx := r.buildServicePlatformContext(ctx, instance)

	return reg.ForAll(func(h modules.ModuleHandler, _ bool) error {
		if !managed.Has(h.GetName()) {
			return nil
		}
		if !h.IsEnabled(platformCtx) {
			return nil
		}

		moduleCR, err := h.BuildModuleCR(ctx, r.Client, platformCtx)
		if err != nil {
			return fmt.Errorf("failed to build %s module CR: %w", h.GetName(), err)
		}
		if moduleCR == nil {
			return fmt.Errorf("module %s returned nil CR", h.GetName())
		}

		if err := ctrl.SetControllerReference(instance, moduleCR, r.Scheme); err != nil {
			return fmt.Errorf("failed to set DSCI owner reference on %s module CR: %w", h.GetName(), err)
		}

		if err := resources.Apply(
			ctx,
			r.Client,
			moduleCR,
			client.FieldOwner(fieldManager),
			client.ForceOwnership,
		); err != nil {
			return fmt.Errorf("failed to apply %s module CR: %w", h.GetName(), err)
		}

		return nil
	})
}

func (r *DSCInitializationReconciler) cleanupDisabledServiceModules(
	ctx context.Context,
	instance *dsciv2.DSCInitialization,
) error {
	reg := r.moduleRegistry()
	if !reg.HasEntries() {
		return nil
	}

	managed := modules.ManagedModuleNames(instance.Spec)
	platformCtx := r.buildServicePlatformContext(ctx, instance)

	return reg.ForAll(func(h modules.ModuleHandler, _ bool) error {
		if !managed.Has(h.GetName()) {
			return nil
		}
		if h.IsEnabled(platformCtx) {
			return nil
		}

		if err := resources.DeleteAllOwnedBy(ctx, r.Client, h.GetGroupVersionKind(), instance, metav1.DeletePropagationForeground); err != nil {
			return fmt.Errorf("failed to delete disabled %s module CRs: %w", h.GetName(), err)
		}

		return nil
	})
}

func (r *DSCInitializationReconciler) computeServiceModulesStatus(
	ctx context.Context,
	instance *dsciv2.DSCInitialization,
) ([]DSCInitializationCondition, error) {
	reg := r.moduleRegistry()
	if !reg.HasEntries() {
		return nil, nil
	}

	managed := modules.ManagedModuleNames(instance.Spec)
	platformCtx := r.buildServicePlatformContext(ctx, instance)
	conditions := make([]DSCInitializationCondition, 0)

	err := reg.ForAll(func(h modules.ModuleHandler, _ bool) error {
		if !managed.Has(h.GetName()) {
			return nil
		}

		if h.GetName() == serviceApi.MonitoringServiceName {
			conditions = append(conditions, r.computeMonitoringStatus(ctx, h, platformCtx)...)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return conditions, nil
}

func (r *DSCInitializationReconciler) computeMonitoringStatus(
	ctx context.Context,
	handler modules.ModuleHandler,
	platformCtx *modules.PlatformContext,
) []DSCInitializationCondition {
	if !handler.IsEnabled(platformCtx) {
		return []DSCInitializationCondition{{
			Type:         status.ConditionMonitoringReady,
			ReadyReason:  status.RemovedReason,
			ReadyMessage: "Monitoring is not enabled",
			ReadyStatus:  metav1.ConditionFalse,
		}}
	}

	moduleStatus, err := handler.GetModuleStatus(ctx, r.Client)
	if err != nil {
		switch {
		case k8serr.IsNotFound(err):
			return []DSCInitializationCondition{{
				Type:         status.ConditionMonitoringReady,
				ReadyReason:  status.NotReadyReason,
				ReadyMessage: "Monitoring stack is initializing",
				ReadyStatus:  metav1.ConditionUnknown,
			}}
		case meta.IsNoMatchError(err):
			return []DSCInitializationCondition{{
				Type:         status.ConditionMonitoringReady,
				ReadyReason:  status.NotReadyReason,
				ReadyMessage: "Monitoring CRD is not available",
				ReadyStatus:  metav1.ConditionUnknown,
			}}
		default:
			return []DSCInitializationCondition{{
				Type:         status.ConditionMonitoringReady,
				ReadyReason:  status.NotReadyReason,
				ReadyMessage: fmt.Sprintf("Failed to retrieve Monitoring CR status: %v", err),
				ReadyStatus:  metav1.ConditionUnknown,
			}}
		}
	}

	return monitoringConditionsFromModuleStatus(moduleStatus)
}

func projectPlatformModulesFromTaggedSpec(spec any) configv1alpha1.PlatformModules {
	var projected configv1alpha1.PlatformModules

	value := reflect.ValueOf(spec)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return projected
	}

	valueType := value.Type()
	for i := range value.NumField() {
		field := valueType.Field(i)
		name := field.Tag.Get("module")
		if name == "" {
			continue
		}

		managementStateField := value.Field(i).FieldByName("ManagementState")
		managementState := operatorv1.Removed
		if managementStateField.IsValid() {
			if state, ok := managementStateField.Interface().(operatorv1.ManagementState); ok && state != "" {
				managementState = state
			}
		}

		projected.Set(configv1alpha1.PlatformModuleConfig{
			Name:            name,
			ManagementState: managementState,
		})
	}

	return projected
}

func monitoringConditionsFromModuleStatus(moduleStatus *modules.ModuleStatus) []DSCInitializationCondition {
	if moduleStatus == nil {
		return []DSCInitializationCondition{{
			Type:         status.ConditionMonitoringReady,
			ReadyReason:  status.NotReadyReason,
			ReadyMessage: "Monitoring stack is initializing",
			ReadyStatus:  metav1.ConditionUnknown,
		}}
	}

	conditions := make([]DSCInitializationCondition, 0, len(moduleStatus.Conditions)+1)
	for _, c := range moduleStatus.Conditions {
		switch c.Type {
		case status.ConditionTypeReady,
			status.ConditionTypeProvisioningSucceeded,
			status.ConditionMonitoringStackAvailable,
			status.ConditionThanosQuerierAvailable,
			status.ConditionOpenTelemetryCollectorAvailable,
			status.ConditionTempoAvailable,
			status.ConditionPersesAvailable,
			status.ConditionAlertingAvailable,
			status.ConditionNodeMetricsEndpointAvailable:
			conditions = append(conditions, DSCInitializationCondition{
				Type:         c.Type,
				ReadyReason:  c.Reason,
				ReadyMessage: c.Message,
				ReadyStatus:  c.Status,
			})
		}
	}

	if len(conditions) == 0 {
		return []DSCInitializationCondition{{
			Type:         status.ConditionMonitoringReady,
			ReadyReason:  status.NotReadyReason,
			ReadyMessage: "Monitoring stack is initializing",
			ReadyStatus:  metav1.ConditionUnknown,
		}}
	}

	conditions = append(conditions, DSCInitializationCondition{
		Type:         status.ConditionMonitoringReady,
		ReadyReason:  status.ReadyReason,
		ReadyMessage: "Monitoring stack is initialized",
		ReadyStatus:  metav1.ConditionTrue,
	})

	return conditions
}

// GetMonitoringReadyCondition is kept as a compatibility wrapper for callers and
// tests that still exercise monitoring-specific condition extraction directly.
func (r *DSCInitializationReconciler) GetMonitoringReadyCondition(ctx context.Context) []DSCInitializationCondition {
	monitoring := &serviceApi.Monitoring{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: serviceApi.MonitoringInstanceName}, monitoring)
	if err != nil {
		switch {
		case k8serr.IsNotFound(err):
			return []DSCInitializationCondition{{
				Type:         status.ConditionMonitoringReady,
				ReadyReason:  status.RemovedReason,
				ReadyMessage: "Monitoring is not enabled",
				ReadyStatus:  metav1.ConditionFalse,
			}}
		case meta.IsNoMatchError(err):
			return []DSCInitializationCondition{{
				Type:         status.ConditionMonitoringReady,
				ReadyReason:  status.NotReadyReason,
				ReadyMessage: "Monitoring CRD is not available",
				ReadyStatus:  metav1.ConditionUnknown,
			}}
		default:
			return []DSCInitializationCondition{{
				Type:         status.ConditionMonitoringReady,
				ReadyReason:  status.NotReadyReason,
				ReadyMessage: fmt.Sprintf("Failed to retrieve Monitoring CR status: %v", err),
				ReadyStatus:  metav1.ConditionUnknown,
			}}
		}
	}

	return monitoringConditionsFromModuleStatus(&modules.ModuleStatus{
		Conditions: monitoring.GetConditions(),
	})
}
