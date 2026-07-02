package platformmodule

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/deploy"
	helmrender "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/render/helm"
	kustomizerender "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/render/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/precondition"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/reconciler"
)

// Reconciler reconciles PlatformModule CRs. Holding the registry as a field
// (rather than accessing the global modules.DefaultRegistry()) allows tests to
// inject a custom registry with controlled handlers without touching global state.
type Reconciler struct {
	registry *modules.Registry
}

// New creates and registers the PlatformModule controller.
// reg is the module handler registry; pass modules.DefaultRegistry() for
// production use or a custom registry in tests. The reconciler uses dynamic
// ownership so all deployed operator resources carry owner references to their
// PlatformModule CR, enabling Kubernetes GC cascade deletion without a finalizer.
//
// For each registered module handler, a dynamic watch is added for the module CR
// GVK using reconciler.Dynamic(reconciler.CrdExists(gvk)). This means:
//   - The watch activates only once the module CRD is installed (the CRD is
//     installed by the module operator, which PlatformModule itself deploys).
//   - CRD creation events trigger a reconcile so the watch is registered promptly.
//   - ResourceVersionChangedPredicate is used instead of DefaultPredicate because
//     module CR status updates do not bump generation (only spec changes do), so
//     GenerationChangedPredicate would silently miss all status-only changes.
func New(ctx context.Context, mgr ctrl.Manager, reg *modules.Registry) error {
	r := &Reconciler{
		registry: reg,
	}

	b := reconciler.ReconcilerFor(mgr, &configv1alpha1.PlatformModule{}).
		// CRDs are excluded from ownership: they are installed by the module
		// operator and must persist independently. Deleting a PlatformModule CR
		// must not cascade-delete the CRD (which would destroy all CRs of that
		// type cluster-wide). Dynamic ownership watches CRDs but does not set
		// owner references on them.
		WithDynamicOwnership(reconciler.ExcludeGVKs(gvk.CustomResourceDefinition)).
		WithConditions(status.ConditionDeploymentsAvailable, status.ConditionTypeOperandAvailable).
		WithPeriodicSync(1 * time.Minute).
		// Actions
		WithAction(precondition.RunlevelGateAction()).
		WithAction(r.provision).
		WithAction(helmrender.NewAction()).
		WithAction(kustomizerender.NewAction()).
		WithAction(modules.InjectModuleEnv).
		WithAction(r.injectPlatformConfig).
		WithAction(deploy.NewAction(
			deploy.WithCache(),
			deploy.WithApplyOrder(),
			deploy.WithContinueOnError(),
		)).
		WithAction(r.driftCleanup).
		WithAction(r.checkOperatorDeployments).
		WithAction(r.syncModuleCRStatus)

	// CRD watch: any CRD change triggers reconciliation for all registered modules.
	// ExcludeGVKs(gvk.CustomResourceDefinition) above suppresses the automatic
	// CRD watch from WithDynamicOwnership, so we add it manually here. Without
	// this, the Dynamic(CrdExists) guard on each module CR watch would never
	// re-evaluate after the module operator installs its CRD.
	b = b.WatchesGVK(
		gvk.CustomResourceDefinition,
		reconciler.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		reconciler.WithEventMapper(func(_ context.Context, _ client.Object) []reconcile.Request {
			var reqs []reconcile.Request
			reg.ForEachEnabled(func(h modules.ModuleHandler) {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: h.GetName()},
				})
			})
			return reqs
		}),
	)

	// Per-module CR watch: activates only once the module CRD is installed.
	// ResourceVersionChangedPredicate is required because module CR status updates
	// do not bump generation (only spec changes do).
	_ = reg.ForAll(func(h modules.ModuleHandler, _ bool) error {
		moduleGVK := h.GetGVK()
		moduleName := h.GetName()

		b = b.WatchesGVK(
			moduleGVK,
			reconciler.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
			reconciler.WithEventMapper(func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{Name: moduleName},
				}}
			}),
			reconciler.Dynamic(reconciler.CrdExists(moduleGVK)),
		)
		return nil
	})

	_, err := b.Build(ctx)
	if err != nil {
		return fmt.Errorf("failed to create PlatformModule reconciler: %w", err)
	}

	return nil
}
