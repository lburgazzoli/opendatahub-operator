package actions

import (
	"context"
	"time"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

//
// Common
//

const (
	ActionGroup = "action"
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
		a.name = ""
	}

	return &a
}
