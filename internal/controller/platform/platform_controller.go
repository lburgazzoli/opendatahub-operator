package platform

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	sr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/services/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/deploy"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/handlers"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/dependent"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/reconciler"
)

// Reconciler reconciles the Platform CR.
// It creates and deletes PlatformModule CRs based on Platform.Spec.Modules,
// walks the module DAG to clear runlevels, and aggregates PlatformModule
// status into the Platform Ready condition.
type Reconciler struct {
	moduleRegistry    *modules.Registry
	componentRegistry *cr.Registry
	serviceRegistry   *sr.Registry
	stuckTracker      *dag.StuckTracker
	deletePolicy      metav1.DeletionPropagation
}

// New creates and registers the Platform controller with the given manager.
// Options let callers override the module, component, and service registries
// (defaults to the package-level singletons). Follow the same pattern as the
// PlatformModule controller: production callers pass no options; tests inject
// isolated registries via WithModuleRegistry / WithComponentRegistry / WithServiceRegistry.
func New(ctx context.Context, mgr ctrl.Manager, opts ...Option) error {
	o := Options{
		ModuleRegistry:    modules.DefaultRegistry(),
		ComponentRegistry: cr.DefaultRegistry(),
		ServiceRegistry:   sr.DefaultRegistry(),
		StuckTracker:      dag.NewStuckTracker(),
		DeletePropagation: metav1.DeletePropagationForeground,
	}
	for _, opt := range opts {
		opt.applyOption(&o)
	}

	r := &Reconciler{
		moduleRegistry:    o.ModuleRegistry,
		componentRegistry: o.ComponentRegistry,
		serviceRegistry:   o.ServiceRegistry,
		stuckTracker:      o.StuckTracker,
		deletePolicy:      o.DeletePropagation,
	}

	b := reconciler.ReconcilerFor(mgr, &configv1alpha1.Platform{}).
		WithConditions(status.ConditionTypeModulesReady).
		// deploy.NewAction sets owner references on PlatformModule CRs so
		// Owns() fires on status changes and GC cascade-deletes them.
		WithDynamicOwnership().
		WithPeriodicSync(1*time.Minute).
		// WithWatchStatus so that PlatformModule status-only updates (which don't
		// bump generation) still requeue Platform for DAG re-evaluation.
		Owns(&configv1alpha1.PlatformModule{}, reconciler.WithPredicates(dependent.New(dependent.WithWatchStatus(true))))

	// Watch DSC dynamically — the CRD may not exist on xKS. When DSC changes
	// the component readiness checker in walkModuleDAG needs to re-evaluate.
	b = b.WatchesGVK(
		gvk.DataScienceCluster,
		reconciler.Dynamic(reconciler.CrdExists(gvk.DataScienceCluster)),
		reconciler.WithEventHandler(handlers.ToNamed(configv1alpha1.PlatformInstanceName)),
		reconciler.WithPredicates(dependent.New(dependent.WithWatchStatus(true))),
	)

	// Watch all in-tree component CRs via the component registry. Component
	// status changes may unblock a DAG runlevel that depends on components
	// being Ready first.
	_ = r.componentRegistry.ForEach(func(h cr.ComponentHandler) error {
		b = b.WatchesGVK(
			h.GroupVersionKind(),
			reconciler.WithEventHandler(handlers.ToNamed(configv1alpha1.PlatformInstanceName)),
			reconciler.WithPredicates(dependent.New(dependent.WithWatchStatus(true))),
		)
		return nil
	})

	// Watch service CRs (Auth, Monitoring, GatewayConfig) that have a GVK.
	// Services without a CR return an empty GVK which is skipped.
	_ = r.serviceRegistry.ForEach(func(h sr.ServiceHandler) error {
		if k := h.GroupVersionKind(); k.Kind != "" {
			b = b.WatchesGVK(
				k,
				reconciler.WithEventHandler(handlers.ToNamed(configv1alpha1.PlatformInstanceName)),
				reconciler.WithPredicates(dependent.New(dependent.WithWatchStatus(true))),
			)
		}
		return nil
	})

	_, err := b.
		WithAction(r.enableModules).
		WithAction(r.syncPlatformModuleCRs).
		WithAction(deploy.NewAction(
			deploy.WithCache(),
			deploy.WithApplyOrder(),
			deploy.WithContinueOnError(),
		)).
		WithAction(r.cleanupDisabledModules).
		WithAction(r.walkModuleDAG).
		WithAction(r.aggregateStatus).
		Build(ctx)

	if err != nil {
		return fmt.Errorf("failed to create Platform reconciler: %w", err)
	}

	return nil
}
