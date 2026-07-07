package registry

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/operatorconfig"
)

// BaseComponentHandler is a configurable ComponentHandler whose behaviour is
// controlled via function fields. Name and GVK are set as plain struct fields.
// Unset function fields fall back to safe no-op defaults.
//
// Embed or instantiate directly in tests and lightweight implementations that
// do not need a full per-component controller.
type BaseComponentHandler struct {
	Name string
	GVK  schema.GroupVersionKind

	InitFn                   func(common.Platform, operatorconfig.OperatorSettings) error
	IsEnabledFn              func(*dscv2.DataScienceCluster) bool
	NewCRObjectFn            func(context.Context, client.Client, *dscv2.DataScienceCluster) (common.PlatformObject, error)
	NewComponentReconcilerFn func(context.Context, ctrl.Manager, *provision.RunlevelTracker) error
	UpdateDSCStatusFn        func(context.Context, *types.ReconciliationRequest) (metav1.ConditionStatus, error)
}

func (h *BaseComponentHandler) GetName() string                              { return h.Name }
func (h *BaseComponentHandler) GetGroupVersionKind() schema.GroupVersionKind { return h.GVK }

func (h *BaseComponentHandler) Init(platform common.Platform, cfg operatorconfig.OperatorSettings) error {
	if h.InitFn != nil {
		return h.InitFn(platform, cfg)
	}
	return nil
}

func (h *BaseComponentHandler) IsEnabled(dsc *dscv2.DataScienceCluster) bool {
	if h.IsEnabledFn != nil {
		return h.IsEnabledFn(dsc)
	}
	return false
}

func (h *BaseComponentHandler) NewCRObject(ctx context.Context, cli client.Client, dsc *dscv2.DataScienceCluster) (common.PlatformObject, error) {
	if h.NewCRObjectFn != nil {
		return h.NewCRObjectFn(ctx, cli, dsc)
	}
	return nil, nil
}

func (h *BaseComponentHandler) NewComponentReconciler(ctx context.Context, mgr ctrl.Manager, tracker *provision.RunlevelTracker) error {
	if h.NewComponentReconcilerFn != nil {
		return h.NewComponentReconcilerFn(ctx, mgr, tracker)
	}
	return nil
}

func (h *BaseComponentHandler) UpdateDSCStatus(ctx context.Context, rr *types.ReconciliationRequest) (metav1.ConditionStatus, error) {
	if h.UpdateDSCStatusFn != nil {
		return h.UpdateDSCStatusFn(ctx, rr)
	}
	return metav1.ConditionTrue, nil
}
