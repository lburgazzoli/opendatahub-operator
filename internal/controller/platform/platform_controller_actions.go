package platform

import (
	"context"
	"fmt"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	odherrors "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/errors"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	odhtype "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

// enableModules syncs the module registry's enabled set to match the Platform
// spec. Must run before any action that checks registry enablement (e.g.
// walkModuleDAG, cleanupDisabledModules).
func (r *Reconciler) enableModules(_ context.Context, rr *odhtype.ReconciliationRequest) error {
	platform, ok := rr.Instance.(*configv1alpha1.Platform)
	if !ok {
		return fmt.Errorf("expected *Platform, got %T", rr.Instance)
	}

	r.moduleRegistry.EnableFromList(platform.Spec.Modules.EnabledModules())

	return nil
}

// syncPlatformModuleCRs adds a PlatformModule CR for each enabled module to
// rr.Resources. deploy.NewAction() applies them via SSA, creating or updating
// as needed. The CR is intentionally spec-empty — the PlatformModule reconciler
// owns all deployment logic.
func (r *Reconciler) syncPlatformModuleCRs(_ context.Context, rr *odhtype.ReconciliationRequest) error {
	platform, ok := rr.Instance.(*configv1alpha1.Platform)
	if !ok {
		return fmt.Errorf("expected *Platform, got %T", rr.Instance)
	}

	for _, name := range platform.Spec.Modules.EnabledModules() {
		pm := &configv1alpha1.PlatformModule{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		if err := rr.AddResources(pm); err != nil {
			return fmt.Errorf("adding PlatformModule %s to resources: %w", name, err)
		}
	}

	return nil
}

// cleanupDisabledModules deletes PlatformModule CRs for modules that are no
// longer enabled. It compares the current desired set (from spec) against the
// live set and deletes the difference.
func (r *Reconciler) cleanupDisabledModules(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	platform, ok := rr.Instance.(*configv1alpha1.Platform)
	if !ok {
		return fmt.Errorf("expected *Platform, got %T", rr.Instance)
	}

	desired := sets.New(platform.Spec.Modules.EnabledModules()...)

	existing := &configv1alpha1.PlatformModuleList{}
	if err := rr.Client.List(ctx, existing); err != nil {
		return fmt.Errorf("listing PlatformModule CRs: %w", err)
	}

	for i := range existing.Items {
		pm := &existing.Items[i]
		if desired.Has(pm.Name) {
			continue
		}
		if err := rr.Client.Delete(ctx, pm, client.PropagationPolicy(r.deletePolicy)); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("deleting PlatformModule %s: %w", pm.Name, err)
		}
	}

	return nil
}

// walkModuleDAG walks the unified module DAG in runlevel order and clears
// runlevels as each batch becomes ready. PlatformModule readiness is
// determined by reading the PlatformModule status conditions directly —
// the PlatformModule reconciler has already aggregated operator deployment
// health and operand CR health into a single Ready condition.
//
// When DSC exists (OpenShift mode), a composite checker spans both in-tree
// components and module operators so the DAG correctly gates across both.
// On xKS (no DSC), only the module readiness checker is used.
//
// When a batch is not yet ready, the action schedules a requeue after the
// remaining gating timeout so the check fires even without external events.
func (r *Reconciler) walkModuleDAG(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	if !r.moduleRegistry.HasEntries() {
		return nil
	}

	var checker dag.ReadinessChecker = &platformModuleReadinessChecker{
		cli:             rr.Client,
		platformVersion: rr.Release.Version.String(),
	}

	// When DSC exists (OpenShift), include in-tree component readiness in
	// DAG gating. On xKS (no DSC CRD), only modules participate.
	dsc, err := cluster.GetDSC(ctx, rr.Client)

	switch {
	case err == nil:
		checker = provision.NewCompositeChecker(
			cr.NewReadinessChecker(r.componentRegistry, rr.Client, dsc),
			checker,
		)
	case k8serr.IsNotFound(err):
		// xKS: no DSC CRD or instance — modules-only DAG.
	case meta.IsNoMatchError(err):
		// xKS: no DSC CRD or instance — modules-only DAG.
	default:
		return fmt.Errorf("failed to get DSC for DAG gating: %w", err)
	}

	requeueAfter, walkErr := provision.WalkBatches(
		ctx,
		checker,
		r.stuckTracker,
		string(rr.Instance.GetUID()),
		rr.Conditions,
		func(batch []provision.UnifiedNode) error {
			provision.GetRunlevelTracker().MarkCleared(
				rr.Release.Version.String(),
				batch[0].GetRunlevel().Order,
			)
			return nil
		},
	)

	if walkErr != nil {
		return walkErr
	}

	if requeueAfter > 0 {
		return odherrors.NewRequeueAfterError(requeueAfter)
	}

	return nil
}

// aggregateStatus reflects the overall module readiness into the Platform
// ModulesReady condition by checking all PlatformModule CRs.
func (r *Reconciler) aggregateStatus(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	platform, ok := rr.Instance.(*configv1alpha1.Platform)
	if !ok {
		return fmt.Errorf("expected *Platform, got %T", rr.Instance)
	}

	// Always reflect enabled modules into status so operators can observe
	// which modules are active without reading the spec.
	platform.Status.Modules = platform.Spec.Modules.EnabledModules()

	// No modules enabled — nothing to wait for.
	if len(platform.Status.Modules) == 0 {
		rr.Conditions.MarkTrue(status.ConditionTypeModulesReady)
		return nil
	}

	existing := &configv1alpha1.PlatformModuleList{}
	if err := rr.Client.List(ctx, existing); err != nil {
		return fmt.Errorf("listing PlatformModule CRs for status aggregation: %w", err)
	}

	// Index the ready set from existing PlatformModule CRs.
	readyModules := sets.New[string]()
	for i := range existing.Items {
		if conditions.IsStatusConditionTrue(&existing.Items[i], status.ConditionTypeReady) {
			readyModules.Insert(existing.Items[i].Name)
		}
	}

	// Any enabled module not yet Ready (or not yet created) blocks aggregation.
	notReady := sets.New(platform.Status.Modules...).Difference(readyModules)

	if notReady.Len() > 0 {
		rr.Conditions.MarkFalse(status.ConditionTypeModulesReady,
			conditions.WithReason(status.NotReadyReason),
			conditions.WithMessage("%d module(s) not ready: %v", notReady.Len(), sets.List(notReady)),
		)
		return nil
	}

	rr.Conditions.MarkTrue(status.ConditionTypeModulesReady)

	return nil
}
