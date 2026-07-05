package setup

import (
	"context"
	"fmt"

	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
)

const (
	ServiceName = "setupcontroller"
)

func NewHandler() *serviceHandler { return &serviceHandler{} }

type serviceHandler struct {
}

func (h *serviceHandler) Init(_ common.Platform) error {
	return nil
}

func (h *serviceHandler) GroupVersionKind() schema.GroupVersionKind { return schema.GroupVersionKind{} }

func (h *serviceHandler) GetName() string {
	return ServiceName
}

func (h *serviceHandler) GetManagementState(_ common.Platform, _ *dsciv2.DSCInitialization) operatorv1.ManagementState {
	return operatorv1.Managed
}

func (h *serviceHandler) NewReconciler(_ context.Context, mgr ctrl.Manager, _ *provision.RunlevelTracker) error {
	rec := &SetupControllerReconciler{
		Client: mgr.GetClient(),
	}

	if err := rec.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("could not create the %s controller: %w", ServiceName, err)
	}

	return nil
}
