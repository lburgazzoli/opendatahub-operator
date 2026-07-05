package platform

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	odherrors "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/errors"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
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

	r.ModuleRegistry.EnableFromList(platform.Spec.Modules.EnabledModules())

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
		if err := rr.Client.Delete(ctx, pm, client.PropagationPolicy(r.DeletePropagation)); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("deleting PlatformModule %s: %w", pm.Name, err)
		}
	}

	return nil
}

// walkModuleDAG walks the unified module DAG in runlevel order and advances
// the shared runlevel admission frontier used by RunlevelGateAction.
// PlatformModule readiness is determined by reading the PlatformModule status
// conditions directly — the PlatformModule reconciler has already aggregated
// operator deployment health and operand CR health into a single Ready
// condition.
//
// When DSC exists (OpenShift mode), a composite checker spans both in-tree
// components and module operators so the DAG correctly gates across both.
// On xKS (no DSC), only the module readiness checker is used.
//
// When a batch is not yet ready, the action schedules a requeue after the
// remaining gating timeout so the check fires even without external events.
func (r *Reconciler) walkModuleDAG(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
	if !r.ModuleRegistry.HasEntries() {
		return nil
	}

	// CompositeChecker spans in-tree components and module operators.
	// TODO: This is an architectural compromise. The shared frontier is
	// currently advanced here, which means Platform needs direct visibility into
	// DSC-managed component readiness. Ideally Platform should not know about
	// DSC-owned components and would instead consume a higher-level readiness
	// signal for DAG advancement.
	checker := provision.NewCompositeChecker(
		componentReadinessChecker(rr.Client, r.ComponentRegistry),
		moduleReadinessChecker(rr.Client, rr.Release.Version.String()),
	)

	requeueAfter, walkErr := provision.WalkBatches(
		ctx,
		checker,
		r.StuckTracker,
		string(rr.Instance.GetUID()),
		rr.Conditions,
		func(batch []provision.UnifiedNode) error {
			// MarkCleared advances the admission frontier for this runlevel after
			// all prior runlevels have been observed ready by WalkBatches. Leaf
			// reconcilers use that frontier to decide whether the current runlevel
			// may deploy; it does not mean the current batch is already Ready.
			r.Tracker.MarkCleared(
				rr.Release.Version.String(),
				batch[0].GetRunlevel().Order,
			)
			return nil
		},
		r.ProvisionReg,
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
