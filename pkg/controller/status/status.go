package status

// These constants represent the overall Phase as used by .Status.Phase.
const (
	PhaseNotReady    = "Not Ready"
	PhaseProgressing = "Progressing"
	PhaseError       = "Error"
	PhaseReady       = "Ready"
)

// List of constants to show different reconciliation messages and statuses.
const (
	ReconcileFailed           = "ReconcileFailed"
	ReconcileInit             = "ReconcileInit"
	ReconcileCompleted        = "ReconcileCompleted"
	ReconcileCompletedMessage = "Reconcile completed successfully"
)

const (
	ConditionTypeAvailable         = "Available"
	ConditionTypeProgressing       = "Progressing"
	ConditionTypeDegraded          = "Degraded"
	ConditionTypeUpgradeable       = "Upgradeable"
	ConditionTypeReady             = "Ready"
	ConditionTypeReconcileComplete = "ReconcileComplete"

	ConditionTypeProvisioningSucceeded           = "ProvisioningSucceeded"
	ConditionDeploymentsNotAvailableReason       = "DeploymentsNotReady"
	ConditionMaaSPrerequisitesAvailable          = "MaaSPrerequisitesAvailable"
	ConditionDeploymentsAvailable                = "DeploymentsAvailable"
	ConditionDependenciesAvailable               = "DependenciesAvailable"
	ConditionArgoWorkflowAvailable               = "ArgoWorkflowAvailable"
	ConditionTypeComponentsReady                 = "ComponentsReady"
	ConditionMonitoringReady                     = "MonitoringReady"
	ConditionMonitoringAvailable                 = "MonitoringAvailable"
	ConditionMonitoringStackAvailable            = "MonitoringStackAvailable"
	ConditionTempoAvailable                      = "TempoAvailable"
	ConditionOpenTelemetryCollectorAvailable     = "OpenTelemetryCollectorAvailable"
	ConditionInstrumentationAvailable            = "InstrumentationAvailable"
	ConditionAlertingAvailable                   = "AlertingAvailable"
	ConditionThanosQuerierAvailable              = "ThanosQuerierAvailable"
	ConditionPersesAvailable                     = "PersesAvailable"
	ConditionPersesTempoDataSourceAvailable      = "PersesTempoDataSourceAvailable"
	ConditionPersesPrometheusDataSourceAvailable = "PersesPrometheusDataSourceAvailable"
	ConditionNodeMetricsEndpointAvailable        = "NodeMetricsEndpointAvailable"
	ConditionImageStreamsAvailable               = "ImageStreamsAvailable"
	ConditionImageStreamsNotAvailableReason      = "ImageStreamsNotReady"

	ConditionDependenciesReady = "DependenciesReady"
	ConditionGatewayAPIReady   = "GatewayAPIReady"
	ConditionCertManagerReady  = "CertManagerReady"
	ConditionLWSReady          = "LWSReady"
	ConditionSailOperatorReady = "SailOperatorReady"
)

const (
	MissingOperatorReason     string = "MissingOperator"
	ConfiguredReason          string = "Configured"
	RemovedReason             string = "Removed"
	UnmanagedReason           string = "Unmanaged"
	CapabilityFailed          string = "CapabilityFailed"
	ArgoWorkflowExist         string = "ArgoWorkflowExist"
	NoManagedComponentsReason        = "NoManagedComponents"

	AvailableReason = "Available"
	NotReadyReason  = "NotReady"
	ReadyReason     = "Ready"
	DeletingReason  = "Deleting"
	DeletingMessage = "Component CR is being deleted"
)

const (
	ReadySuffix = "Ready"
)

const (
	DataSciencePipelinesDoesntOwnArgoCRDReason        = "DataSciencePipelinesDoesntOwnArgoCRD"
	DataSciencePipelinesArgoWorkflowsNotManagedReason = "DataSciencePipelinesArgoWorkflowsNotManaged"
	DataSciencePipelinesArgoWorkflowsCRDMissingReason = "DataSciencePipelinesArgoWorkflowsCRDMissing"

	DataSciencePipelinesDoesntOwnArgoCRDMessage = "Failed upgrade: workflows.argoproj.io CRD already exists but not deployed by this operator " +
		"remove existing Argo workflows or set `spec.components.aipipelines.managementState` to Removed to proceed"
	DataSciencePipelinesArgoWorkflowsNotManagedMessage = "Argo Workflows controllers are not managed by this operator"
	DataSciencePipelinesArgoWorkflowsCRDMissingMessage = "Argo Workflows controllers are not managed by this operator, but the CRD is missing"
)

const (
	KueueStateManagedNotSupported        = "KueueStateManagedNotSupported"
	KueueStateManagedNotSupportedMessage = "Kueue managementState Managed is not supported, please use Removed or Unmanaged"
	KueueOperatorNotInstalleReason       = "KueueOperatorNotInstalleReason"
	KueueOperatorNotInstalledMessage     = "Kueue operator not installed, install it or change kueue component state to Removed"
)

const (
	ISVCMissingCRDReason  = "InferenceServiceCRDMissing"
	ISVCMissingCRDMessage = "InferenceServices CRD does not exist, please enable serving component first"
)

const (
	MetricsNotConfiguredReason  = "MetricsNotConfigured"
	MetricsNotConfiguredMessage = "Metrics not configured in DSCI CR"
	TracesNotConfiguredReason   = "TracesNotConfigured"
	TracesNotConfiguredMessage  = "Traces not configured in DSCI CR"

	AlertingNotConfiguredReason  = "AlertingNotConfigured"
	AlertingNotConfiguredMessage = "Alerting not configured in DSCI CR"

	TempoOperatorMissingMessage                  = "Tempo operator must be installed for traces configuration"
	COOMissingMessage                            = "ClusterObservability operator must be installed for metrics configuration"
	OpenTelemetryCollectorOperatorMissingMessage = "OpenTelemetryCollector operator must be installed for OpenTelemetry configuration"

	GatewayNotFoundMessage = "Gateway resource not found"
	GatewayNotReadyMessage = "Gateway is not ready"
	GatewayReadyMessage    = "Gateway is ready"

	AuthProxyDeployedMessage                 = "Auth proxy deployed successfully"
	AuthProxyFailedDeployMessage             = "Failed to deploy auth proxy"
	AuthProxyFailedOAuthClientMessage        = "Failed to create OAuth client"
	AuthProxyFailedCallbackRouteMessage      = "Failed to create auth callback route"
	AuthProxyFailedGenerateSecretMessage     = "Failed to generate client secret"
	AuthProxyOIDCModeWithoutConfigMessage    = "Cluster is in OIDC mode but GatewayConfig has no OIDC configuration"
	AuthProxyOIDCClientIDEmptyMessage        = "OIDC clientID cannot be empty"
	AuthProxyOIDCIssuerURLEmptyMessage       = "OIDC issuerURL cannot be empty"
	AuthProxyOIDCSecretRefNameEmptyMessage   = "OIDC clientSecretRef.name cannot be empty" //nolint:gosec // This is an error message, not a credential
	AuthProxyExternalAuthNoDeploymentMessage = "Cluster uses external authentication, no gateway auth proxy deployed"
)

const (
	CodeFlarePresentMessage = `Failed upgrade: CodeFlare component is present in the cluster. It must be uninstalled to proceed with Ray component upgrade.
To uninstall it, you should delete all RayClusters resources from the cluster, delete the CodeFlare component resource and recreate the RayClusters.`
)

const (
	JobSetOperatorNotInstalledMessage = "JobSet operator not installed, please install it first"
	JobSetCRDMissingMessage           = "JobSet CRD does not exist, please inspect JobSetOperator CR status conditions or JobSet controller Pod logs for more details"
	JobSetOperatorCRNotFoundMessage   = "JobSetOperator CR with name 'cluster' not found, please create it first"
)
