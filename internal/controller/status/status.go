/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package status contains different conditions, phases and progresses,
// being used by DataScienceCluster and DSCInitialization's controller
package status

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	cond "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	pkgstatus "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/status"
)

// conditionsWrapper implements common.ConditionsAccessor for a slice of conditions.
type conditionsWrapper struct {
	conditions *[]common.Condition
}

func (w *conditionsWrapper) GetConditions() []common.Condition {
	return *w.conditions
}

func (w *conditionsWrapper) SetConditions(conditions []common.Condition) {
	*w.conditions = conditions
}

// Re-exported constants from pkg/controller/status for backward compatibility.
const (
	PhaseNotReady    = pkgstatus.PhaseNotReady
	PhaseProgressing = pkgstatus.PhaseProgressing
	PhaseError       = pkgstatus.PhaseError
	PhaseReady       = pkgstatus.PhaseReady
)

const (
	ReconcileFailed           = pkgstatus.ReconcileFailed
	ReconcileInit             = pkgstatus.ReconcileInit
	ReconcileCompleted        = pkgstatus.ReconcileCompleted
	ReconcileCompletedMessage = pkgstatus.ReconcileCompletedMessage
)

const (
	ConditionTypeAvailable                       = pkgstatus.ConditionTypeAvailable
	ConditionTypeProgressing                     = pkgstatus.ConditionTypeProgressing
	ConditionTypeDegraded                        = pkgstatus.ConditionTypeDegraded
	ConditionTypeUpgradeable                     = pkgstatus.ConditionTypeUpgradeable
	ConditionTypeReady                           = pkgstatus.ConditionTypeReady
	ConditionTypeReconcileComplete               = pkgstatus.ConditionTypeReconcileComplete
	ConditionTypeProvisioningSucceeded           = pkgstatus.ConditionTypeProvisioningSucceeded
	ConditionDeploymentsNotAvailableReason       = pkgstatus.ConditionDeploymentsNotAvailableReason
	ConditionMaaSPrerequisitesAvailable          = pkgstatus.ConditionMaaSPrerequisitesAvailable
	ConditionDeploymentsAvailable                = pkgstatus.ConditionDeploymentsAvailable
	ConditionDependenciesAvailable               = pkgstatus.ConditionDependenciesAvailable
	ConditionArgoWorkflowAvailable               = pkgstatus.ConditionArgoWorkflowAvailable
	ConditionTypeComponentsReady                 = pkgstatus.ConditionTypeComponentsReady
	ConditionMonitoringReady                     = pkgstatus.ConditionMonitoringReady
	ConditionMonitoringAvailable                 = pkgstatus.ConditionMonitoringAvailable
	ConditionMonitoringStackAvailable            = pkgstatus.ConditionMonitoringStackAvailable
	ConditionTempoAvailable                      = pkgstatus.ConditionTempoAvailable
	ConditionOpenTelemetryCollectorAvailable     = pkgstatus.ConditionOpenTelemetryCollectorAvailable
	ConditionInstrumentationAvailable            = pkgstatus.ConditionInstrumentationAvailable
	ConditionAlertingAvailable                   = pkgstatus.ConditionAlertingAvailable
	ConditionThanosQuerierAvailable              = pkgstatus.ConditionThanosQuerierAvailable
	ConditionPersesAvailable                     = pkgstatus.ConditionPersesAvailable
	ConditionPersesTempoDataSourceAvailable      = pkgstatus.ConditionPersesTempoDataSourceAvailable
	ConditionPersesPrometheusDataSourceAvailable = pkgstatus.ConditionPersesPrometheusDataSourceAvailable
	ConditionNodeMetricsEndpointAvailable        = pkgstatus.ConditionNodeMetricsEndpointAvailable
	ConditionImageStreamsAvailable               = pkgstatus.ConditionImageStreamsAvailable
	ConditionImageStreamsNotAvailableReason      = pkgstatus.ConditionImageStreamsNotAvailableReason
	ConditionDependenciesReady                   = pkgstatus.ConditionDependenciesReady
	ConditionGatewayAPIReady                     = pkgstatus.ConditionGatewayAPIReady
	ConditionCertManagerReady                    = pkgstatus.ConditionCertManagerReady
	ConditionLWSReady                            = pkgstatus.ConditionLWSReady
	ConditionSailOperatorReady                   = pkgstatus.ConditionSailOperatorReady
)

const (
	MissingOperatorReason     = pkgstatus.MissingOperatorReason
	ConfiguredReason          = pkgstatus.ConfiguredReason
	RemovedReason             = pkgstatus.RemovedReason
	UnmanagedReason           = pkgstatus.UnmanagedReason
	CapabilityFailed          = pkgstatus.CapabilityFailed
	ArgoWorkflowExist         = pkgstatus.ArgoWorkflowExist
	NoManagedComponentsReason = pkgstatus.NoManagedComponentsReason
	AvailableReason           = pkgstatus.AvailableReason
	NotReadyReason            = pkgstatus.NotReadyReason
	ReadyReason               = pkgstatus.ReadyReason
	DeletingReason            = pkgstatus.DeletingReason
	DeletingMessage           = pkgstatus.DeletingMessage
)

const (
	ReadySuffix = pkgstatus.ReadySuffix
)

const (
	DataSciencePipelinesDoesntOwnArgoCRDReason        = pkgstatus.DataSciencePipelinesDoesntOwnArgoCRDReason
	DataSciencePipelinesArgoWorkflowsNotManagedReason = pkgstatus.DataSciencePipelinesArgoWorkflowsNotManagedReason
	DataSciencePipelinesArgoWorkflowsCRDMissingReason = pkgstatus.DataSciencePipelinesArgoWorkflowsCRDMissingReason

	DataSciencePipelinesDoesntOwnArgoCRDMessage        = pkgstatus.DataSciencePipelinesDoesntOwnArgoCRDMessage
	DataSciencePipelinesArgoWorkflowsNotManagedMessage = pkgstatus.DataSciencePipelinesArgoWorkflowsNotManagedMessage
	DataSciencePipelinesArgoWorkflowsCRDMissingMessage = pkgstatus.DataSciencePipelinesArgoWorkflowsCRDMissingMessage
)

const (
	KueueStateManagedNotSupported        = pkgstatus.KueueStateManagedNotSupported
	KueueStateManagedNotSupportedMessage = pkgstatus.KueueStateManagedNotSupportedMessage
	KueueOperatorNotInstalleReason       = pkgstatus.KueueOperatorNotInstalleReason
	KueueOperatorNotInstalledMessage     = pkgstatus.KueueOperatorNotInstalledMessage
)

const (
	ISVCMissingCRDReason  = pkgstatus.ISVCMissingCRDReason
	ISVCMissingCRDMessage = pkgstatus.ISVCMissingCRDMessage
)

const (
	MetricsNotConfiguredReason                   = pkgstatus.MetricsNotConfiguredReason
	MetricsNotConfiguredMessage                  = pkgstatus.MetricsNotConfiguredMessage
	TracesNotConfiguredReason                    = pkgstatus.TracesNotConfiguredReason
	TracesNotConfiguredMessage                   = pkgstatus.TracesNotConfiguredMessage
	AlertingNotConfiguredReason                  = pkgstatus.AlertingNotConfiguredReason
	AlertingNotConfiguredMessage                 = pkgstatus.AlertingNotConfiguredMessage
	TempoOperatorMissingMessage                  = pkgstatus.TempoOperatorMissingMessage
	COOMissingMessage                            = pkgstatus.COOMissingMessage
	OpenTelemetryCollectorOperatorMissingMessage = pkgstatus.OpenTelemetryCollectorOperatorMissingMessage
	GatewayNotFoundMessage                       = pkgstatus.GatewayNotFoundMessage
	GatewayNotReadyMessage                       = pkgstatus.GatewayNotReadyMessage
	GatewayReadyMessage                          = pkgstatus.GatewayReadyMessage
	AuthProxyDeployedMessage                     = pkgstatus.AuthProxyDeployedMessage
	AuthProxyFailedDeployMessage                 = pkgstatus.AuthProxyFailedDeployMessage
	AuthProxyFailedOAuthClientMessage            = pkgstatus.AuthProxyFailedOAuthClientMessage
	AuthProxyFailedCallbackRouteMessage          = pkgstatus.AuthProxyFailedCallbackRouteMessage
	AuthProxyFailedGenerateSecretMessage         = pkgstatus.AuthProxyFailedGenerateSecretMessage
	AuthProxyOIDCModeWithoutConfigMessage        = pkgstatus.AuthProxyOIDCModeWithoutConfigMessage
	AuthProxyOIDCClientIDEmptyMessage            = pkgstatus.AuthProxyOIDCClientIDEmptyMessage
	AuthProxyOIDCIssuerURLEmptyMessage           = pkgstatus.AuthProxyOIDCIssuerURLEmptyMessage
	AuthProxyOIDCSecretRefNameEmptyMessage       = pkgstatus.AuthProxyOIDCSecretRefNameEmptyMessage
	AuthProxyExternalAuthNoDeploymentMessage     = pkgstatus.AuthProxyExternalAuthNoDeploymentMessage
)

const (
	CodeFlarePresentMessage = pkgstatus.CodeFlarePresentMessage
)

const (
	JobSetOperatorNotInstalledMessage = pkgstatus.JobSetOperatorNotInstalledMessage
	JobSetCRDMissingMessage           = pkgstatus.JobSetCRDMissingMessage
	JobSetOperatorCRNotFoundMessage   = pkgstatus.JobSetOperatorCRNotFoundMessage
)

// setConditions is a helper function to set multiple conditions at once.
func setConditions(wrapper *conditionsWrapper, conditions []common.Condition) {
	for _, c := range conditions {
		cond.SetStatusCondition(wrapper, c)
	}
}

// SetProgressingCondition sets the ProgressingCondition to True and other conditions to false or
// Unknown. Used when we are just starting to reconcile, and there are no existing conditions.
func SetProgressingCondition(conditions *[]common.Condition, reason string, message string) {
	wrapper := &conditionsWrapper{conditions: conditions}
	setConditions(wrapper, []common.Condition{
		{
			Type:    ConditionTypeReconcileComplete,
			Status:  metav1.ConditionUnknown,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeAvailable,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeProgressing,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeDegraded,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeUpgradeable,
			Status:  metav1.ConditionUnknown,
			Reason:  reason,
			Message: message,
		},
	})
}

// SetErrorCondition sets the ConditionTypeReconcileComplete to False in case of any errors
// during the reconciliation process.
func SetErrorCondition(conditions *[]common.Condition, reason string, message string) {
	wrapper := &conditionsWrapper{conditions: conditions}
	setConditions(wrapper, []common.Condition{
		{
			Type:    ConditionTypeReconcileComplete,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeAvailable,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeProgressing,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeDegraded,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeUpgradeable,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		},
	})
}

// SetCompleteCondition sets the ConditionTypeReconcileComplete to True and other Conditions
// to indicate that the reconciliation process has completed successfully.
func SetCompleteCondition(conditions *[]common.Condition, reason string, message string) {
	wrapper := &conditionsWrapper{conditions: conditions}
	setConditions(wrapper, []common.Condition{
		{
			Type:    ConditionTypeReconcileComplete,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeAvailable,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeProgressing,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeDegraded,
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: message,
		},
		{
			Type:    ConditionTypeUpgradeable,
			Status:  metav1.ConditionTrue,
			Reason:  reason,
			Message: message,
		},
	})
}

// SetCondition is a general purpose function to update any type of condition.
func SetCondition(conditions *[]common.Condition, conditionType string, reason string, message string, status metav1.ConditionStatus) {
	wrapper := &conditionsWrapper{conditions: conditions}
	cond.SetStatusCondition(wrapper, common.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}
