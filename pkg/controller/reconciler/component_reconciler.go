package reconciler

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	odhClient "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/client"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

type ComponentReconciler struct {
	Client     *odhClient.Client
	Scheme     *runtime.Scheme
	Actions    []actions.Action
	Finalizer  []actions.Action
	Log        logr.Logger
	Manager    manager.Manager
	Controller controller.Controller
	Recorder   record.EventRecorder
	Release    cluster.Release

	owned           map[schema.GroupVersionKind]struct{}
	instanceFactory func() (client.Object, error)
}

func NewComponentReconciler[T types.ResourceObject](ctx context.Context, mgr manager.Manager, name string) (*ComponentReconciler, error) {
	oc, err := odhClient.NewFromManager(ctx, mgr)
	if err != nil {
		return nil, err
	}

	cc := ComponentReconciler{
		Client:   oc,
		Scheme:   mgr.GetScheme(),
		Log:      ctrl.Log.WithName("controllers").WithName(name),
		Manager:  mgr,
		Recorder: mgr.GetEventRecorderFor(name),
		Release:  cluster.GetRelease(),
		owned:    map[schema.GroupVersionKind]struct{}{},
		instanceFactory: func() (client.Object, error) {
			t := reflect.TypeOf(*new(T)).Elem()
			res, ok := reflect.New(t).Interface().(T)
			if !ok {
				return res, fmt.Errorf("unable to construct instance of %v", t)
			}

			return res, nil
		},
	}

	return &cc, nil
}

func (r *ComponentReconciler) GetRelease() cluster.Release {
	return r.Release
}

func (r *ComponentReconciler) GetLogger() logr.Logger {
	return r.Log
}

func (r *ComponentReconciler) AddOwnedType(gvk schema.GroupVersionKind) {
	r.owned[gvk] = struct{}{}
}

func (r *ComponentReconciler) Owns(obj client.Object) bool {
	_, ok := r.owned[obj.GetObjectKind().GroupVersionKind()]
	return ok
}

func (r *ComponentReconciler) AddAction(action actions.Action) {
	r.Actions = append(r.Actions, action)
}

func (r *ComponentReconciler) AddFinalizer(action actions.Action) {
	r.Finalizer = append(r.Finalizer, action)
}

func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	res, err := r.instanceFactory()
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name}, res); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	dscl := dscv1.DataScienceClusterList{}
	if err := r.Client.List(ctx, &dscl); err != nil {
		return ctrl.Result{}, err
	}

	if len(dscl.Items) != 1 {
		return ctrl.Result{}, errors.New("unable to find DataScienceCluster")
	}

	dscil := dsciv1.DSCInitializationList{}
	if err := r.Client.List(ctx, &dscil); err != nil {
		return ctrl.Result{}, err
	}

	if len(dscil.Items) != 1 {
		return ctrl.Result{}, errors.New("unable to find DSCInitialization")
	}

	rr := types.ReconciliationRequest{
		Client:    r.Client,
		Instance:  res,
		DSC:       &dscl.Items[0],
		DSCI:      &dscil.Items[0],
		Release:   r.Release,
		Manifests: make([]types.ManifestInfo, 0),
		IsOwned:   r.Owns,
	}

	// Handle deletion
	if !res.GetDeletionTimestamp().IsZero() {
		// Execute finalizers
		for _, action := range r.Finalizer {
			l.Info("Executing finalizer", "action", action)

			actx := log.IntoContext(
				ctx,
				l.WithName(actions.ActionGroup).WithName(action.String()),
			)

			if err := action.Execute(actx, &rr); err != nil {
				l.Error(err, "Failed to execute finalizer", "action", action)
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Execute actions
	for _, action := range r.Actions {
		l.Info("Executing action", "action", action)

		actx := log.IntoContext(
			ctx,
			l.WithName(actions.ActionGroup).WithName(action.String()),
		)

		if err := action.Execute(actx, &rr); err != nil {
			l.Error(err, "Failed to execute action", "action", action)
			return ctrl.Result{}, err
		}
	}

	// update status
	err = r.Client.ApplyStatus(
		ctx,
		rr.Instance,
		client.FieldOwner(rr.Instance.GetName()),
		client.ForceOwnership,
	)

	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
