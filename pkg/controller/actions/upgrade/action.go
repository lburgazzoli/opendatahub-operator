/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package upgrade

import (
	"context"
	"errors"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	odherrors "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/errors"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

const eventRecorderName = "upgrade-action"

// ReasonUpgradeFailed is the condition reason used when upgrade actions fail.
const ReasonUpgradeFailed = "UpgradeFailed"

// ConditionTypeUpgradeFailed is the condition type for upgrade failures.
// Used in non-blocking mode to track upgrade errors while allowing reconciliation to continue.
const ConditionTypeUpgradeFailed = "UpgradeFailed"

// Fn is the signature for upgrade functions.
// cli is a non-caching client for both reads and writes.
type Fn func(ctx context.Context, cli client.Client) error

// Option configures an upgrade Action.
type Option func(*Action)

// WithFn adds an upgrade function to execute.
func WithFn(fn Fn) Option {
	return func(a *Action) {
		a.fns = append(a.fns, fn)
	}
}

// NonBlocking makes the action non-blocking.
// On failure: logs error, emits event, returns standard error (not StopError).
// This allows the reconciler to mark provisioning as failed without stopping the chain.
func NonBlocking() Option {
	return func(a *Action) {
		a.nonBlocking = true
	}
}

// Action wraps upgrade functions with run-once semantics.
// Marks done only on success; retries on failure.
// Thread-safe for concurrent reconciliations.
// By default, wraps errors to StopError. Use NonBlocking() to return standard errors.
type Action struct {
	mu          sync.Mutex
	done        bool
	mgr         ctrl.Manager
	recorder    record.EventRecorder
	fns         []Fn
	nonBlocking bool
}

// NewAction creates an upgrade action with functional options.
// Uses the Controller's DirectClient (non-caching) for API operations.
// Multiple functions are executed in order; errors are combined.
func NewAction(mgr ctrl.Manager, opts ...Option) *Action {
	a := &Action{mgr: mgr}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Run implements actions.Fn interface.
// Executes the wrapped actions once; if all succeed, marks done.
// In blocking mode (default): wraps combined errors to StopError.
// In non-blocking mode: returns combined errors as standard error.
func (a *Action) Run(ctx context.Context, rr *types.ReconciliationRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.done {
		return nil
	}

	var errs []error
	for _, fn := range a.fns {
		if err := fn(ctx, rr.Controller.GetDirectClient()); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return a.handleError(ctx, rr, errors.Join(errs...))
	}

	a.done = true
	return nil
}

func (a *Action) handleError(ctx context.Context, rr *types.ReconciliationRequest, err error) error {
	a.emitEvent(rr, err)

	if !a.nonBlocking {
		return odherrors.WrapStopErrorWithReason(ReasonUpgradeFailed, err)
	}

	log := ctrl.LoggerFrom(ctx)
	log.Error(err, "upgrade failed, continuing with reconciliation")

	// In non-blocking mode, set the UpgradeFailed condition directly
	// so it's tracked even though we're not returning a StopError.
	if rr.Conditions != nil {
		rr.Conditions.MarkFalse(
			ConditionTypeUpgradeFailed,
			conditions.WithReason(ReasonUpgradeFailed),
			conditions.WithMessage("%s", err.Error()),
			conditions.WithSeverity(common.ConditionSeverityInfo),
			conditions.WithObservedGeneration(rr.Instance.GetGeneration()),
		)
	}

	return err
}

func (a *Action) emitEvent(rr *types.ReconciliationRequest, err error) {
	if a.recorder == nil {
		a.recorder = a.mgr.GetEventRecorderFor(eventRecorderName)
	}

	a.recorder.Event(
		rr.Instance,
		corev1.EventTypeWarning,
		"UpgradeFailed",
		err.Error(),
	)
}
