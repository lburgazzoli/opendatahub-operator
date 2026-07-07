package platformmodule

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	sr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/services/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/deploy"
	helmrender "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/render/helm"
	kustomizerender "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/render/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/dependent"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/reconciler"
)

// Reconciler reconciles PlatformModule CRs.
type Reconciler struct {
	Options
}

// New creates and registers the PlatformModule controller. mgr is required.
// Pass any Option values — either an Options{} struct literal, named
// constructors (WithRegistry, WithProvisionRegistry, WithTracker), or a mix.
// Nil/unset fields default to the package-level singletons.
//
// Examples:
//
//	New(ctx, mgr)                             // all singletons
//	New(ctx, mgr, Options{Registry: reg})     // struct style
//	New(ctx, mgr, WithRegistry(reg))          // functional style
//	New(ctx, mgr, Options{Registry: reg}, WithTracker(t)) // mixed
//
// The reconciler uses dynamic ownership so all deployed operator resources carry
// owner references to their PlatformModule CR, enabling Kubernetes GC cascade
// deletion without a finalizer.
//
// For each registered module handler, a dynamic watch is added for the module CR
// GVK using reconciler.Dynamic(reconciler.CrdExists(gvk)). This means:
//   - The watch activates only once the module CRD is installed (the CRD is
//     installed by the module operator, which PlatformModule itself deploys).
//   - CRD creation events trigger a reconcile so the watch is registered promptly.
//   - ResourceVersionChangedPredicate is used instead of DefaultPredicate because
//     module CR status updates do not bump generation (only spec changes do), so
//     GenerationChangedPredicate would silently miss all status-only changes.
func New(ctx context.Context, mgr ctrl.Manager, fns ...Option) error {
	r := &Reconciler{
		Options: Options{
			Registry:          modules.DefaultRegistry(),
			ComponentRegistry: cr.DefaultRegistry(),
			ServiceRegistry:   sr.DefaultRegistry(),
			ProvisionReg:      provision.DefaultRegistry(),
			Tracker:           provision.GetRunlevelTracker(),
			DeletePropagation: metav1.DeletePropagationForeground,
		},
	}

	for _, fn := range fns {
		fn.applyOption(&r.Options)
	}

	b := reconciler.ReconcilerFor(mgr, &configv1alpha1.PlatformModule{}).
		// CRDs are excluded from ownership: they are installed by the module
		// operator and must persist independently. Deleting a PlatformModule CR
		// must not cascade-delete the CRD (which would destroy all CRs of that
		// type cluster-wide). Dynamic ownership watches CRDs but does not set
		// owner references on them.
		WithDynamicOwnership(reconciler.ExcludeGVKs(gvk.CustomResourceDefinition)).
		WithConditions(
			status.ConditionDeploymentsAvailable,
			status.ConditionTypeOperandAvailable,
		).
		WithPeriodicSync(1 * time.Minute).
		// Actions
		WithAction(r.validateMode).
		WithAction(r.gateEntryRunlevel).
		WithAction(r.onDeployer(r.provision)).
		WithAction(r.onDeployer(helmrender.NewAction())).
		WithAction(r.onDeployer(kustomizerender.NewAction())).
		WithAction(r.onDeployer(modules.InjectModuleEnv)).
		WithAction(r.onDeployer(r.injectPlatformConfig)).
		WithAction(r.onDeployer(deploy.NewAction(
			deploy.WithCache(),
			deploy.WithApplyOrder(),
			deploy.WithContinueOnError(),
		))).
		WithAction(r.onDeployer(r.driftCleanup)).
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
			_ = forAll[sr.ServiceHandler](r.ServiceRegistry, func(h sr.ServiceHandler) {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: h.GetName(),
					},
				})
			})
			_ = forAll[modules.ModuleHandler](r.Registry, func(h modules.ModuleHandler) {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: h.GetName(),
					},
				})
			})
			_ = forAll[cr.ComponentHandler](r.ComponentRegistry, func(h cr.ComponentHandler) {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: h.GetName(),
					},
				})
			})

			return reqs
		}),
	)

	// Service CR watches: a change to any enabled service CR (e.g. GatewayConfig
	// domain update) may require all modules to refresh their platform config.
	// Handlers with an empty GVK have no dedicated CR and are skipped.
	_ = forAll[sr.ServiceHandler](r.ServiceRegistry, func(h sr.ServiceHandler) {
		b = b.WatchesGVK(
			h.GetGroupVersionKind(),
			reconciler.Dynamic(reconciler.CrdExists(h.GetGroupVersionKind())),
			reconciler.WithPredicates(dependent.New(dependent.WithWatchStatus(true))),
			reconciler.WithEventMapper(func(_ context.Context, _ client.Object) []reconcile.Request {
				var reqs []reconcile.Request
				_ = forAll[modules.ModuleHandler](r.Registry, func(h modules.ModuleHandler) {
					reqs = append(reqs, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name: h.GetName(),
						},
					})
				})
				return reqs
			}),
		)
	})

	// Per-module CR watch: activates only once the module CRD is installed.
	// ResourceVersionChangedPredicate is required because module CR status updates
	// do not bump generation (only spec changes do).
	_ = forAll[modules.ModuleHandler](r.Registry, func(h modules.ModuleHandler) {
		b = b.WatchesGVK(
			h.GetGroupVersionKind(),
			reconciler.Dynamic(reconciler.CrdExists(h.GetGroupVersionKind())),
			reconciler.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
			reconciler.WithEventMapper(func(_ context.Context, obj client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name: h.GetName(),
					},
				}}
			}),
		)
	})

	// Component CR watches drive tracker-only PlatformModule entries.
	_ = forAll[cr.ComponentHandler](r.ComponentRegistry, func(h cr.ComponentHandler) {
		b = b.WatchesGVK(
			h.GetGroupVersionKind(),
			reconciler.Dynamic(reconciler.CrdExists(h.GetGroupVersionKind())),
			reconciler.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
			reconciler.WithEventMapper(func(_ context.Context, _ client.Object) []reconcile.Request {
				return []reconcile.Request{{
					NamespacedName: types.NamespacedName{
						Name: h.GetName(),
					},
				}}
			}),
		)
	})

	_, err := b.Build(ctx)
	if err != nil {
		return fmt.Errorf("failed to create PlatformModule reconciler: %w", err)
	}

	return nil
}
