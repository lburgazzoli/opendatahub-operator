package platform

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	odherrors "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/errors"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	odhtype "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

const maxPlatformRunlevel = int(^uint32(0) >> 1)

func toPlatformRunlevel(order int, name string) (int32, error) {
	if order > maxPlatformRunlevel {
		return 0, fmt.Errorf("runlevel %d for %q exceeds int32 range", order, name)
	}

	//nolint:gosec // The explicit bounds check above guarantees this conversion is safe.
	return int32(order), nil
}

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
		if !r.isTrackedEntry(name) {
			continue
		}

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
	platform, ok := rr.Instance.(*configv1alpha1.Platform)
	if !ok {
		return fmt.Errorf("expected *Platform, got %T", rr.Instance)
	}

	if len(platform.Spec.Modules.EnabledModules()) == 0 {
		return nil
	}

	checker := moduleReadinessChecker(rr.Client, rr.Release.Version.String())

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

	platform.Status.Modules = nil

	// No entries declared — nothing to wait for.
	if len(platform.Spec.Modules) == 0 {
		rr.Conditions.MarkFalse(status.ConditionTypeDegraded,
			conditions.WithReason(status.ConfiguredReason),
			conditions.WithMessage("all platform entries are registered"),
			conditions.WithSeverity(common.ConditionSeverityInfo),
		)
		rr.Conditions.MarkTrue(status.ConditionTypeModulesReady)
		return nil
	}

	notReady := sets.New[string]()
	unknown := sets.New[string]()

	for _, entry := range platform.Spec.Modules {
		summary := configv1alpha1.PlatformModuleSummary{
			Name: entry.Name,
		}

		if order, ok := r.ProvisionReg.LookupOrder(entry.Name); ok {
			runlevel, err := toPlatformRunlevel(order, entry.Name)
			if err != nil {
				return err
			}
			summary.Runlevel = runlevel
		}

		switch {
		case !r.isTrackedEntry(entry.Name):
			summary.Status = configv1alpha1.PlatformModuleConditionSummary{
				Reason:  "UnknownEntry",
				Message: fmt.Sprintf("platform entry %q is not registered", entry.Name),
			}
			unknown.Insert(entry.Name)
		case !entry.IsManaged():
			summary.Status = configv1alpha1.PlatformModuleConditionSummary{
				Reason:  status.RemovedReason,
				Message: "entry is not managed",
			}
		default:
			pm := &configv1alpha1.PlatformModule{}
			if err := rr.Client.Get(ctx, client.ObjectKey{Name: entry.Name}, pm); err != nil {
				if client.IgnoreNotFound(err) != nil {
					return fmt.Errorf("getting PlatformModule %s for status aggregation: %w", entry.Name, err)
				}
				summary.Status = configv1alpha1.PlatformModuleConditionSummary{
					Reason:  "TrackerMissing",
					Message: "tracker is not created yet",
				}
				notReady.Insert(entry.Name)
				break
			}

			summary.Version = pm.Status.Release.Version.String()
			if ready := conditions.FindStatusCondition(pm, status.ConditionTypeReady); ready != nil {
				summary.Status = configv1alpha1.PlatformModuleConditionSummary{
					Ready:   ready.Status == metav1.ConditionTrue,
					Reason:  ready.Reason,
					Message: ready.Message,
				}
				if ready.Status != metav1.ConditionTrue {
					notReady.Insert(entry.Name)
				}
			} else {
				summary.Status = configv1alpha1.PlatformModuleConditionSummary{
					Reason:  status.NotReadyReason,
					Message: "tracker has not reported readiness yet",
				}
				notReady.Insert(entry.Name)
			}
		}

		platform.Status.Modules = append(platform.Status.Modules, summary)
	}

	sort.Slice(platform.Status.Modules, func(i int, j int) bool {
		left := platform.Status.Modules[i]
		right := platform.Status.Modules[j]

		switch left.Runlevel {
		case right.Runlevel:
			return left.Name < right.Name
		default:
			return left.Runlevel < right.Runlevel
		}
	})

	switch {
	case len(platform.Spec.Modules) == 0:
		rr.Conditions.MarkFalse(status.ConditionTypeDegraded,
			conditions.WithReason(status.ConfiguredReason),
			conditions.WithMessage("no platform modules are configured"),
			conditions.WithSeverity(common.ConditionSeverityInfo),
		)
	case unknown.Len() > 0:
		rr.Conditions.MarkTrue(status.ConditionTypeDegraded,
			conditions.WithReason("UnknownEntry"),
			conditions.WithMessage("%d platform entrie(s) are not registered: %v", unknown.Len(), sets.List(unknown)),
		)
	default:
		rr.Conditions.MarkFalse(status.ConditionTypeDegraded,
			conditions.WithReason(status.ConfiguredReason),
			conditions.WithMessage("all platform entries are recognized by the registries"),
			conditions.WithSeverity(common.ConditionSeverityInfo),
		)
	}

	switch {
	case notReady.Len() > 0:
		rr.Conditions.MarkFalse(status.ConditionTypeModulesReady,
			conditions.WithReason(status.NotReadyReason),
			conditions.WithMessage("%d managed entrie(s) not ready: %v", notReady.Len(), sets.List(notReady)),
		)
	default:
		rr.Conditions.MarkTrue(status.ConditionTypeModulesReady)
	}

	return nil
}

func (r *Reconciler) isTrackedEntry(name string) bool {
	return r.ModuleRegistry.Lookup(name) != nil || r.ComponentRegistry.Lookup(name) != nil
}
