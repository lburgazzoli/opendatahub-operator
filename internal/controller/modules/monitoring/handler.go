package monitoring

import (
	"context"
	"errors"
	"fmt"

	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
)

const (
	moduleName = serviceApi.MonitoringServiceName
	crName     = serviceApi.MonitoringInstanceName
)

type handler struct {
	modules.BaseHandler
}

func NewHandler() *handler {
	return &handler{
		BaseHandler: modules.BaseHandler{
			Config: modules.ModuleConfig{
				Name:              moduleName,
				CRName:            crName,
				ReleaseName:       "odh-observability",
				ChartDir:          "odh-observability",
				NamespaceValueKey: "operatorNamespace",
				GVK:               gvk.Monitoring,
				RelatedImages: []string{
					"RELATED_IMAGE_ODH_KUBE_RBAC_PROXY_IMAGE",
					"RELATED_IMAGE_OSE_PROM_LABEL_PROXY_IMAGE",
					"RELATED_IMAGE_CLI_IMAGE",
					"RELATED_IMAGE_PERSES_IMAGE",
				},
			},
		},
	}
}

// IsEnabled checks whether the monitoring module should be deployed.
// In DSC mode (DSCI present), reads DSCI.Spec.Monitoring.ManagementState.
// In Platform mode (xKS), reads Platform.Spec.Modules.Monitoring.ManagementState.
func (h *handler) IsEnabled(platform *modules.PlatformContext) bool {
	if platform == nil {
		return false
	}
	if platform.DSCI != nil {
		return platform.DSCI.Spec.Monitoring.ManagementState == operatorv1.Managed
	}
	if platform.Platform != nil {
		return platform.Platform.Spec.Modules.Monitoring.ManagementState == operatorv1.Managed
	}
	return false
}

func (h *handler) ApplyManagementState(ctx *modules.PlatformContext, spec *configv1alpha1.PlatformModules) {
	if ctx == nil || ctx.DSCI == nil {
		return
	}
	spec.Monitoring = common.ManagementSpec{
		ManagementState: ctx.DSCI.Spec.Monitoring.ManagementState,
	}
}

// BuildModuleCR projects platform monitoring configuration onto the module CR.
// In DSC mode, the full DSCIMonitoring struct is converted directly.
// In Platform mode, a minimal spec with ManagementState is projected.
func (h *handler) BuildModuleCR(
	ctx context.Context,
	cli client.Client,
	platform *modules.PlatformContext,
) (*unstructured.Unstructured, error) {
	if platform == nil {
		return nil, errors.New("platform context is nil, cannot build monitoring CR")
	}

	var spec map[string]any

	switch {
	case platform.DSCI != nil:
		monitoring := serviceApi.Monitoring{
			Spec: serviceApi.MonitoringSpec{
				MonitoringCommonSpec: serviceApi.MonitoringCommonSpec{
					Namespace: platform.DSCI.Spec.Monitoring.Namespace,
				},
			},
		}

		metricsEnabled := platform.DSCI.Spec.Monitoring.Metrics != nil && platform.DSCI.Spec.Monitoring.Metrics.Storage != nil
		tracesEnabled := platform.DSCI.Spec.Monitoring.Traces != nil

		if metricsEnabled {
			monitoring.Spec.Metrics = platform.DSCI.Spec.Monitoring.Metrics
		}

		if tracesEnabled {
			monitoring.Spec.Traces = platform.DSCI.Spec.Monitoring.Traces
			if monitoring.Spec.Traces.TLS != nil && !monitoring.Spec.Traces.TLS.Enabled {
				monitoring.Spec.Traces.TLS = nil
			}
		}

		monitoring.Spec.Alerting = platform.DSCI.Spec.Monitoring.Alerting

		if metricsEnabled || tracesEnabled {
			if platform.DSCI.Spec.Monitoring.CollectorReplicas != 0 {
				monitoring.Spec.CollectorReplicas = platform.DSCI.Spec.Monitoring.CollectorReplicas
			} else {
				if cluster.IsSingleNodeCluster(ctx, cli) {
					monitoring.Spec.CollectorReplicas = 1
				} else {
					monitoring.Spec.CollectorReplicas = 2
				}
			}
		}
		var err error
		spec, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&monitoring.Spec)
		if err != nil {
			return nil, fmt.Errorf("failed to convert MonitoringSpec to unstructured: %w", err)
		}
	case platform.Platform != nil:
		spec = map[string]any{
			"managementState": string(platform.Platform.Spec.Modules.Monitoring.ManagementState),
		}
	default:
		return nil, errors.New("neither DSCI nor Platform is available, cannot build monitoring CR")
	}

	u := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	u.SetGroupVersionKind(h.Config.GVK)
	u.SetName(h.Config.CRName)

	return u, nil
}
