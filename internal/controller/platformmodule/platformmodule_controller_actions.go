package platformmodule

import (
	"context"
	"fmt"

	semver "github.com/blang/semver/v4"
	libversion "github.com/operator-framework/api/pkg/lib/version"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	odhtype "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

// provision sets up the reconcile request for a single module: it looks up
// the module handler by PlatformModule name in the reconciler's registry,
// fetches operator manifests, and builds ModuleEnvInjection so the
// InjectModuleEnv action knows which Deployment to patch with RELATED_IMAGE_* vars.
func (r *Reconciler) provision(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	pm, ok := rr.Instance.(*configv1alpha1.PlatformModule)
	if !ok {
		return fmt.Errorf("expected *PlatformModule, got %T", rr.Instance)
	}

	handler := r.Registry.Lookup(pm.Name)
	if handler == nil {
		// No handler registered — module may have been unregistered after CR was
		// created. Nothing to do; drift cleanup will eventually remove the CR.
		return nil
	}

	appNS, err := cluster.ApplicationNamespace(ctx, rr.Client)
	if err != nil {
		return fmt.Errorf("failed to resolve application namespace: %w", err)
	}

	manifests := handler.GetOperatorManifests(&modules.PlatformContext{
		ApplicationsNamespace: appNS,
		Release:               rr.Release,
		ChartsBasePath:        rr.ChartsBasePath,
		ManifestsBasePath:     rr.ManifestsBasePath,
	})

	rr.HelmCharts = append(rr.HelmCharts, manifests.HelmCharts...)
	rr.Manifests = append(rr.Manifests, manifests.Manifests...)

	rr.ModuleEnvInjection = &odhtype.ModuleEnvInjection{
		ApplicationsNamespace: appNS,
		PerModuleImages: []odhtype.ModuleImages{
			{
				DeploymentName:    modules.DeploymentNameFor(handler, manifests),
				ContainerName:     modules.ContainerNameFor(handler),
				ControllerImage:   modules.ControllerImageFor(handler),
				InitContainerName: modules.InitContainerNameFor(handler),
				Images:            handler.GetRelatedImages(),
			},
		},
	}

	return nil
}

// injectPlatformConfig ensures the per-module platform config ConfigMap
// (odh-<modulename>-config) carries the platform-managed keys.
//
// If the module's Helm/Kustomize render already produced a ConfigMap with
// that name (an optional controller configuration ConfigMap), the platform
// keys are merged into it in-place so there is exactly one entry in
// rr.Resources for that name. If no such ConfigMap was rendered, a new one
// is appended. Either way, deploy.NewAction() applies a single ConfigMap
// with one field manager, avoiding SSA field-manager conflicts.
func (r *Reconciler) injectPlatformConfig(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	if rr.SkipDeploy {
		return nil
	}

	pm, ok := rr.Instance.(*configv1alpha1.PlatformModule)
	if !ok {
		return fmt.Errorf("expected *PlatformModule, got %T", rr.Instance)
	}

	appNS, err := cluster.ApplicationNamespace(ctx, rr.Client)
	if err != nil {
		return fmt.Errorf("failed to resolve application namespace: %w", err)
	}

	idx, err := ensureConfigMap(&rr.Resources, modules.PlatformConfigName(pm.Name), appNS)
	if err != nil {
		return fmt.Errorf("ensuring platform config ConfigMap for %s: %w", pm.Name, err)
	}
	modules.MergePlatformKeys(&rr.Resources[idx], rr.Release.Version.String())

	return nil
}

// driftCleanup compares the resources just rendered (rr.Resources) against
// those recorded in the previous reconcile (status.Resources). Any resource
// that was tracked before but is absent from the current render is deleted —
// this handles the case where a chart removes a resource between operator
// versions. After cleanup, status.Resources is updated to the current set so
// the next reconcile has an accurate baseline.
//
// The action is a no-op when SkipDeploy is set (render was skipped, so
// rr.Resources is empty and deleting everything would be wrong).
func (r *Reconciler) driftCleanup(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	if rr.SkipDeploy {
		return nil
	}

	pm, ok := rr.Instance.(*configv1alpha1.PlatformModule)
	if !ok {
		return fmt.Errorf("expected *PlatformModule, got %T", rr.Instance)
	}

	log := logf.FromContext(ctx)

	currentRefs := trackedResourceRefsFrom(rr.Resources)
	savedRefs := sets.New(currentRefs...)
	stale := sets.New(pm.Status.Resources...).Difference(savedRefs)

	policy := r.DeletePropagation
	for ref := range stale {
		if !shouldTrackResourceRef(ref.GroupVersionKind()) {
			log.Info("skipping protected stale module operator resource",
				"module", pm.Name,
				"gvk", ref.GroupVersionKind().String(),
				"namespace", ref.Namespace,
				"name", ref.Name)
			continue
		}

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(ref.GroupVersionKind())
		u.SetName(ref.Name)
		u.SetNamespace(ref.Namespace)

		switch err := rr.Client.Delete(ctx, u, &client.DeleteOptions{PropagationPolicy: &policy}); {
		case err == nil:
			log.Info("deleted stale module operator resource",
				"module", pm.Name,
				"gvk", ref.GroupVersionKind().String(),
				"namespace", ref.Namespace,
				"name", ref.Name)
		case k8serr.IsNotFound(err) || meta.IsNoMatchError(err):
			// already gone or CRD removed — nothing to do
			continue
		default:
			log.Error(err, "failed to delete stale module operator resource",
				"module", pm.Name,
				"gvk", ref.GroupVersionKind().String(),
				"namespace", ref.Namespace,
				"name", ref.Name)
			return fmt.Errorf("drift cleanup: %w", err)
		}
	}

	pm.Status.Resources = currentRefs

	return nil
}

// checkOperatorDeployments sets the DeploymentsAvailable condition by checking
// only the Deployments tracked in pm.Status.Resources for this specific module
// operator. Unlike the generic statusdeployments.NewAction, this scopes the
// check to exactly the resources deployed by this PlatformModule reconciler,
// avoiding false positives from other module operators in the same namespace.
func (r *Reconciler) checkOperatorDeployments(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	pm, ok := rr.Instance.(*configv1alpha1.PlatformModule)
	if !ok {
		return fmt.Errorf("expected *PlatformModule, got %T", rr.Instance)
	}

	var notReady []string
	deployments := make([]string, 0, len(pm.Status.Resources))

	for _, ref := range pm.Status.Resources {
		if ref.GroupVersionKind() != gvk.Deployment {
			continue
		}
		deployments = append(deployments, ref.Name)

		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk.Deployment)

		if err := rr.Client.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}, u); err != nil {
			notReady = append(notReady, ref.Name)
			continue
		}

		readyReplicas, _, err := unstructured.NestedInt64(u.Object, "status", "readyReplicas")
		if err != nil {
			return fmt.Errorf("reading readyReplicas for deployment %s: %w", ref.Name, err)
		}
		replicas, _, err := unstructured.NestedInt64(u.Object, "status", "replicas")
		if err != nil {
			return fmt.Errorf("reading replicas for deployment %s: %w", ref.Name, err)
		}

		if replicas == 0 || readyReplicas < replicas {
			notReady = append(notReady, ref.Name)
		}
	}

	switch {
	case len(notReady) > 0:
		rr.Conditions.MarkFalse(status.ConditionDeploymentsAvailable,
			conditions.WithReason(status.ConditionDeploymentsNotAvailableReason),
			conditions.WithMessage("%d/%d deployments ready", len(deployments)-len(notReady), len(deployments)),
		)
	case len(pm.Status.Resources) > 0 && len(deployments) == 0:
		// Module has resources but no Deployments — informational, not blocking.
		rr.Conditions.MarkFalse(status.ConditionDeploymentsAvailable,
			conditions.WithReason("ModuleWithoutDeployments"),
			conditions.WithSeverity(common.ConditionSeverityInfo),
			conditions.WithMessage("module has no operator Deployments"),
		)
	default:
		rr.Conditions.MarkTrue(status.ConditionDeploymentsAvailable)
	}

	return nil
}

// syncModuleCRStatus reflects the operand CR (e.g. Monitoring, AIGateway) health
// into the PlatformModule OperandReady condition and updates status.release when
// the module CR's version handshake is complete.
//
//   - CR absent (fresh install or not yet created by DSC/DSCI): OperandReady=True,
//     reason=OperandAbsent. No handshake needed; move forward.
//   - CR exists: reflect its Ready/Degraded conditions. Set status.release only
//     when the module CR acknowledges the current platform version.
func (r *Reconciler) syncModuleCRStatus(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	pm, ok := rr.Instance.(*configv1alpha1.PlatformModule)
	if !ok {
		return fmt.Errorf("expected *PlatformModule, got %T", rr.Instance)
	}

	handler := r.Registry.Lookup(pm.Name)
	if handler == nil {
		// No handler — unknown module, nothing to reflect. Info severity so the
		// DAG is not blocked.
		rr.Conditions.MarkFalse(status.ConditionTypeOperandAvailable,
			conditions.WithReason("UnknownModule"),
			conditions.WithSeverity(common.ConditionSeverityInfo),
		)
		return nil
	}

	moduleStatus, err := handler.GetModuleStatus(ctx, rr.Client)

	switch {
	case meta.IsNoMatchError(err):
		// CRD not installed — module operator has not run yet.
		//
		// Reflect platform release; no module CR to handshake with.
		pm.Status.Release = rr.Release
		rr.Conditions.MarkFalse(status.ConditionTypeOperandAvailable,
			conditions.WithReason("OperandAbsent"),
			conditions.WithSeverity(common.ConditionSeverityInfo),
			conditions.WithMessage("module CRD not installed"),
		)
	case k8serr.IsNotFound(err):
		// CRD installed but CR not yet created by DSC/DSCI/user.
		//
		// In this case we shoiuld just satisfy the handshake, since the
		// module CR is not there
		pm.Status.Release = rr.Release
		rr.Conditions.MarkFalse(status.ConditionTypeOperandAvailable,
			conditions.WithReason("OperandAbsent"),
			conditions.WithSeverity(common.ConditionSeverityInfo),
			conditions.WithMessage("module CR not yet created"),
		)
	case err != nil:
		return fmt.Errorf("failed to get module CR status: %w", err)
	case len(moduleStatus.Conditions) == 0:
		// CR exists but no conditions yet — the module operator is running but
		// has not reported health yet.
		//
		// Block the DAG until conditions appear.
		rr.Conditions.MarkFalse(status.ConditionTypeOperandAvailable,
			conditions.WithReason("OperandInitializing"),
			conditions.WithMessage("module CR has no conditions yet"),
		)
	case !conditions.IsStatusConditionTrue(moduleStatus, status.ConditionTypeReady):
		// CR exists, but not ready
		rr.Conditions.MarkFalse(status.ConditionTypeOperandAvailable,
			conditions.WithReason("OperandNotReady"),
			conditions.WithMessage("module operand CR is not ready"),
		)

		// Always reflect the module CR's actual reported release so the DAG readiness
		// checker can compare pm.Status.Release.Version against the expected platform
		// version. The module's ReleaseVersion string is stored verbatim; the Name
		// comes from the platform identity (unchanged by the module operator).
		pm.Status.Release = rr.Release

		if moduleStatus.ReleaseVersion != "" {
			if v, err := semver.ParseTolerant(moduleStatus.ReleaseVersion); err == nil {
				pm.Status.Release.Version = libversion.OperatorVersion{Version: v}
			}
		}
	default:
		rr.Conditions.MarkTrue(status.ConditionTypeOperandAvailable)

		// Always reflect the module CR's actual reported release so the DAG readiness
		// checker can compare pm.Status.Release.Version against the expected platform
		// version. The module's ReleaseVersion string is stored verbatim; the Name
		// comes from the platform identity (unchanged by the module operator).
		pm.Status.Release = rr.Release

		if moduleStatus.ReleaseVersion != "" {
			if v, err := semver.ParseTolerant(moduleStatus.ReleaseVersion); err == nil {
				pm.Status.Release.Version = libversion.OperatorVersion{Version: v}
			}
		}
	}

	return nil
}
