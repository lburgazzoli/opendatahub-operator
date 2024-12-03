package actions

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

//
// Common
//

const (
	ActionGroup                  = "action"
	packageActionNamePrefix      = "/pkg/controller/actions/"
	packageActionNameSuffix      = ".(*Action).run-fm"
	packageControllersNamePrefix = "/controllers/components/"
)

type Fn func(ctx context.Context, rr *types.ReconciliationRequest) error

type ActionOpt func(a *Action)

func WithName(value string) ActionOpt {
	return func(a *Action) {
		a.name = value
	}
}

type Action struct {
	fn   Fn
	name string
}

func (a *Action) Name() string {
	return a.name
}

func (a *Action) Run(ctx context.Context, rr *types.ReconciliationRequest) error {
	actionStartTS := time.Now()
	defer func() {
		ActionExecutionTime.WithLabelValues(
			rr.ControllerName(),
			a.Name(),
		).Observe(
			time.Since(actionStartTS).Seconds(),
		)
	}()

	return a.fn(ctx, rr)
}

func (a *Action) String() string {
	return a.Name()
}

func NewAction(fn Fn, opts ...ActionOpt) *Action {
	a := Action{
		fn: fn,
	}

	for _, opt := range opts {
		opt(&a)
	}

	// TODO: we should find a better name
	if a.name == "" {
		// The function name includes the full package, i.e:
		//
		//   github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/render/kustomize.(*Action).run-fm
		//   github.com/opendatahub-io/opendatahub-operator/v2/controllers/components/modelregistry.updateStatus
		//
		// Which gets normalized to i.e:
		//
		//   render_kustomize
		//   modelregistry_updateStatus
		//
		n := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()

		if i := strings.Index(n, packageControllersNamePrefix); i != -1 {
			n = n[i+len(packageControllersNamePrefix):]

			if i := strings.Index(n, "/"); i != -1 {
				n = n[i+1:]
			}
			if i := strings.Index(n, "."); i != -1 {
				n = n[i+1:]
			}

			n = strings.ReplaceAll(n, ".", "_")
		} else if i := strings.Index(n, packageActionNamePrefix); i != -1 {
			n = n[i+len(packageActionNamePrefix):]

			if i := strings.Index(n, packageActionNameSuffix); i != -1 {
				n = n[:i]
			}

			n = strings.ReplaceAll(n, "/", "_")
		}

		a.name = n
	}

	return &a
}
