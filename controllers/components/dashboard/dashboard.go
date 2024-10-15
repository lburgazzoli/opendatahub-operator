package dashboard

import (
	"context"
	"fmt"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	componentsv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/components/v1"
	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	odhtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	odhdeploy "github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
)

const (
	ComponentName           = "dashboard"
	ComponentNameUpstream   = ComponentName
	ComponentNameDownstream = "rhods-dashboard"
)

var (
	PathUpstream          = odhdeploy.DefaultManifestPath + "/" + ComponentNameUpstream + "/odh"
	PathDownstream        = odhdeploy.DefaultManifestPath + "/" + ComponentNameUpstream + "/rhoai"
	PathSelfDownstream    = PathDownstream + "/onprem"
	PathManagedDownstream = PathDownstream + "/addon"

	dashboardID = types.NamespacedName{Name: componentsv1.DashboardInstanceName}

	adminGroups = map[cluster.Platform]string{
		cluster.SelfManagedRhods: "rhods-admins",
		cluster.ManagedRhods:     "dedicated-admins",
		cluster.OpenDataHub:      "odh-admins",
		cluster.Unknown:          "odh-admins",
	}

	sectionTitle = map[cluster.Platform]string{
		cluster.SelfManagedRhods: "OpenShift Self Managed Services",
		cluster.ManagedRhods:     "OpenShift Managed Services",
		cluster.OpenDataHub:      "OpenShift Open Data Hub",
		cluster.Unknown:          "OpenShift Open Data Hub",
	}

	baseConsoleURL = map[cluster.Platform]string{
		cluster.SelfManagedRhods: "https://rhods-dashboard-",
		cluster.ManagedRhods:     "https://rhods-dashboard-",
		cluster.OpenDataHub:      "https://odh-dashboard-",
		cluster.Unknown:          "https://odh-dashboard-",
	}

	manifestPaths = map[cluster.Platform]string{
		cluster.SelfManagedRhods: PathSelfDownstream,
		cluster.ManagedRhods:     PathManagedDownstream,
		cluster.OpenDataHub:      PathUpstream,
		cluster.Unknown:          PathUpstream,
	}

	imagesMap = map[string]string{
		"odh-dashboard-image": "RELATED_IMAGE_ODH_DASHBOARD_IMAGE",
	}
)

func Init(platform cluster.Platform) error {
	mi := defaultManifestInfo(platform)

	if err := odhdeploy.ApplyParams(mi.ManifestsPath(), imagesMap); err != nil {
		return fmt.Errorf("failed to update images on path %s: %w", manifestPaths[platform], err)
	}

	return nil
}

func GetDashboard(dsc *dscv1.DataScienceCluster) *componentsv1.Dashboard {
	dashboardAnnotations := make(map[string]string)

	switch dsc.Spec.Components.Dashboard.ManagementState {
	case operatorv1.Managed:
		dashboardAnnotations[annotations.ManagementStateAnnotation] = string(operatorv1.Managed)
	case operatorv1.Removed:
		dashboardAnnotations[annotations.ManagementStateAnnotation] = string(operatorv1.Removed)
	case operatorv1.Unmanaged:
		dashboardAnnotations[annotations.ManagementStateAnnotation] = string(operatorv1.Unmanaged)
	default:
		dashboardAnnotations[annotations.ManagementStateAnnotation] = "Unknown"
	}

	return &componentsv1.Dashboard{
		TypeMeta: metav1.TypeMeta{
			Kind:       componentsv1.DashboardKind,
			APIVersion: componentsv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        componentsv1.DashboardInstanceName,
			Annotations: dashboardAnnotations,
		},
		Spec: componentsv1.DashboardSpec{
			DSCDashboard: dsc.Spec.Components.Dashboard,
		},
	}
}

func defaultManifestInfo(p cluster.Platform) odhtypes.ManifestInfo {
	return odhtypes.ManifestInfo{
		Path:       manifestPaths[p],
		ContextDir: "",
		SourcePath: "",
	}
}

func updateKustomizeVariable(ctx context.Context, cli client.Client, platform cluster.Platform, dscispec *dsciv1.DSCInitializationSpec) (map[string]string, error) {
	consoleLinkDomain, err := cluster.GetDomain(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("error getting console route URL %s : %w", consoleLinkDomain, err)
	}

	return map[string]string{
		"admin_groups":  adminGroups[platform],
		"dashboard-url": baseConsoleURL[platform] + dscispec.ApplicationsNamespace + "." + consoleLinkDomain,
		"section-title": sectionTitle[platform],
	}, nil
}
