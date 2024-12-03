package reconciler

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/opendatahub-io/opendatahub-operator/v2/apis/components"
	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	odherrors "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/errors"
	odhClient "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/client"
	odhManager "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/manager"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

type ComponentReconciler struct {
	Client     *odhClient.Client
	Scheme     *runtime.Scheme
	Log        logr.Logger
	Controller controller.Controller
	Recorder   record.EventRecorder
	Release    cluster.Release

	actions         []*actions.Action
	finalizer       []*actions.Action
	name            string
	m               *odhManager.Manager
	instanceFactory func() (components.ComponentObject, error)
}

func NewComponentReconciler(
	mgr manager.Manager,
	name string,
	object components.ComponentObject,
) (*ComponentReconciler, error) {
	oc, err := odhClient.NewFromManager(mgr)
	if err != nil {
		return nil, err
	}

	cc := ComponentReconciler{
		Client:   oc,
		Scheme:   mgr.GetScheme(),
		Log:      ctrl.Log.WithName("controllers").WithName(name),
		Recorder: mgr.GetEventRecorderFor(name),
		Release:  cluster.GetRelease(),
		name:     name,
		m:        odhManager.New(mgr),
		instanceFactory: func() (components.ComponentObject, error) {
			t := reflect.TypeOf(object).Elem()
			res, ok := reflect.New(t).Interface().(components.ComponentObject)
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
	r.m.AddGVK(gvk, true)
}

func (r *ComponentReconciler) Owns(obj client.Object) bool {
	return r.m.Owns(obj.GetObjectKind().GroupVersionKind())
}

func (r *ComponentReconciler) AddAction(action *actions.Action) {
	r.actions = append(r.actions, action)
}

func (r *ComponentReconciler) AddFinalizer(action *actions.Action) {
	r.finalizer = append(r.finalizer, action)
}

func (r *ComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	l.Info("reconcile")

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

	rr := types.NewReconciliationRequest(
		types.WithClient(r.Client),
		types.WithControllerName(strings.ToLower(res.GetObjectKind().GroupVersionKind().Kind)),
		types.WithManager(r.m),
		types.WithRelease(r.Release),
		types.WithDSCI(&dscil.Items[0]),
		types.WithDSC(&dscl.Items[0]),
		types.WithInstance(res),
	)

	// Handle deletion
	if !res.GetDeletionTimestamp().IsZero() {
		// Execute finalizers
		for _, action := range r.finalizer {
			l.V(3).Info("Executing finalizer", "action", action)

			actx := log.IntoContext(
				ctx,
				l.WithName(actions.ActionGroup).WithName(action.String()),
			)

			if err := action.Run(actx, rr); err != nil {
				se := odherrors.StopError{}
				if !errors.As(err, &se) {
					l.Error(err, "Failed to execute finalizer", "action", action)
					return ctrl.Result{}, err
				}

				l.V(3).Info("detected stop marker", "action", action)
				break
			}
		}

		return ctrl.Result{}, nil
	}

	// Execute actions
	for _, action := range r.actions {
		l.Info("Executing action", "action", action)

		actx := log.IntoContext(
			ctx,
			l.WithName(actions.ActionGroup).WithName(action.String()),
		)

		if err := action.Run(actx, rr); err != nil {
			se := odherrors.StopError{}
			if !errors.As(err, &se) {
				l.Error(err, "Failed to execute action", "action", action)
				return ctrl.Result{}, err
			}

			l.V(3).Info("detected stop marker", "action", action)
			break
		}
	}

	err = r.Client.ApplyStatus(
		ctx,
		rr.Instance,
		client.FieldOwner(r.name),
		client.ForceOwnership,
	)

	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, err
}
