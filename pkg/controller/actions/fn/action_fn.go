package fn

import (
	"context"
	"reflect"
	"runtime"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

type Fn func(ctx context.Context, rr *types.ReconciliationRequest) error

type WrapperAction struct {
	fn     Fn
	fnName string
}

func (a *WrapperAction) Execute(ctx context.Context, rr *types.ReconciliationRequest) error {
	return a.fn(ctx, rr)
}

func (a *WrapperAction) String() string {
	return a.fnName
}

func New(fn Fn) *WrapperAction {
	action := WrapperAction{
		fn:     fn,
		fnName: runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name(),
	}

	return &action
}
