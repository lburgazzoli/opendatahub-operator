package servicemesh

import (
	"context"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	pr "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

type ServiceMeshReconciler struct {
	Client   client.Client
	Recorder record.EventRecorder
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceMeshReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	logf.FromContext(ctx).Info("Adding controller for ServiceMesh.")

	return ctrl.NewControllerManagedBy(mgr).
		Named("servicemesh-controller").
		For(resources.GvkToUnstructured(gvk.DSCInitialization), builder.WithPredicates(pr.PathDriftPredicate([]string{"spec.serviceMesh"}))).
		Complete(reconcile.AsReconciler(mgr.GetClient(), r))
}

func (r *ServiceMeshReconciler) Reconcile(
	ctx context.Context,
	obj *dsciv1.DSCInitialization,
) (ctrl.Result, error) {
	logf.FromContext(ctx).Info("Reconciling ServiceMesh controller")

	if !obj.DeletionTimestamp.IsZero() {
		// DSCI is being deleted, remove ServiceMesh
		if err := r.removeServiceMesh(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// Apply Service Mesh configurations
	if err := r.configureServiceMesh(ctx, obj); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
