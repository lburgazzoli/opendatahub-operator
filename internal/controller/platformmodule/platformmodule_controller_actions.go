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

	if r.modeFor(pm.Name) != entryModeDeployer {
		return nil
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

	if r.modeFor(pm.Name) == entryModeTrackerOnly {
		rr.Conditions.MarkFalse(status.ConditionDeploymentsAvailable,
			conditions.WithReason("NotApplicable"),
			conditions.WithSeverity(common.ConditionSeverityInfo),
			conditions.WithMessage("entry does not manage operator deployments"),
		)

		return nil
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

	trackedGVK, mode, found := r.trackedGVKFor(pm.Name)
	if !found {
		return fmt.Errorf("platform entry %q is not registered in the module/component registries", pm.Name)
	}

	if mode == entryModeTrackerOnly {
		pm.Status.Resources = nil
	}

	tracked, err := getTrackedSingletonObject(ctx, rr.Client, trackedGVK)
	switch {
	case meta.IsNoMatchError(err):
		pm.Status.Release = rr.Release
		rr.Conditions.MarkFalse(status.ConditionTypeOperandAvailable,
			conditions.WithReason("TrackedResourceMissing"),
			conditions.WithSeverity(common.ConditionSeverityInfo),
			conditions.WithMessage(status.TrackedResourceCRDMissingMessage),
		)
		return nil
	case k8serr.IsNotFound(err):
		pm.Status.Release = rr.Release
		rr.Conditions.MarkFalse(status.ConditionTypeOperandAvailable,
			conditions.WithReason("TrackedResourceMissing"),
			conditions.WithSeverity(common.ConditionSeverityInfo),
			conditions.WithMessage(status.TrackedResourceNotCreatedMessage),
		)
		return nil
	case err != nil:
		return fmt.Errorf("failed to get tracked CR status: %w", err)
	default:
		return r.reflectTrackedObjectStatus(pm, rr, tracked.GetConditions(), trackedReleaseVersion(tracked))
	}
}

func (r *Reconciler) reflectTrackedObjectStatus(
	pm *configv1alpha1.PlatformModule,
	rr *odhtype.ReconciliationRequest,
	conditionsList []common.Condition,
	releaseVersion string,
) error {
	pm.Status.Release = rr.Release
	if releaseVersion != "" {
		if v, err := semver.ParseTolerant(releaseVersion); err == nil {
			pm.Status.Release.Version = libversion.OperatorVersion{Version: v}
		}
	}

	trackedStatus := common.Status{}
	trackedStatus.SetConditions(conditionsList)

	switch {
	case len(conditionsList) == 0:
		rr.Conditions.MarkFalse(status.ConditionTypeOperandAvailable,
			conditions.WithReason("OperandInitializing"),
			conditions.WithMessage(status.TrackedResourceInitializingMessage),
		)
		return nil
	case !conditions.IsStatusConditionTrue(trackedStatus, status.ConditionTypeReady):
		if readyCondition := conditions.FindStatusCondition(trackedStatus, status.ConditionTypeReady); readyCondition != nil {
			rr.Conditions.MarkFalse(status.ConditionTypeOperandAvailable,
				conditions.WithReason(readyCondition.Reason),
				conditions.WithSeverity(readyCondition.Severity),
				conditions.WithMessage("%s", readyCondition.Message),
			)
			return nil
		}
		rr.Conditions.MarkFalse(status.ConditionTypeOperandAvailable,
			conditions.WithReason(status.NotReadyReason),
			conditions.WithMessage(status.TrackedResourceReadyConditionMessage),
		)
		return nil
	default:
		rr.Conditions.MarkTrue(status.ConditionTypeOperandAvailable)
		return nil
	}
}
