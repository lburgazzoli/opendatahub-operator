package precondition

import (
	"context"
	"fmt"
	"strings"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	odherrors "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/errors"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

const PlatformReadyConditionType = "PlatformReady"

// NameFunc derives the provision-registry lookup key from a reconciliation request.
type NameFunc func(*types.ReconciliationRequest) string

// RunlevelGateOption configures RunlevelGateAction.
type RunlevelGateOption func(*runlevelGateConfig)

type runlevelGateConfig struct {
	nameFn   NameFunc
	registry *provision.UnifiedRegistry
	tracker  *provision.RunlevelTracker
}

// WithNameFunc overrides the default Kind-based component name derivation.
// Use when the reconciled type shares a Kind across logical names (e.g. PlatformModule).
func WithNameFunc(f NameFunc) RunlevelGateOption {
	return func(c *runlevelGateConfig) { c.nameFn = f }
}

// WithRegistry replaces the default global provision registry.
// Useful in tests to inject an isolated registry without touching global state.
func WithRegistry(r *provision.UnifiedRegistry) RunlevelGateOption {
	return func(c *runlevelGateConfig) { c.registry = r }
}

// WithTracker replaces the default global RunlevelTracker.
// Useful in tests to inject an isolated tracker without touching global state.
func WithTracker(t *provision.RunlevelTracker) RunlevelGateOption {
	return func(c *runlevelGateConfig) {
		if t != nil {
			c.tracker = t
		}
	}
}

// InstanceName derives the component name from rr.Instance.GetName().
// Use in reconcilers where all instances share a common Kind but differ by name
// (e.g. PlatformModule, where the module name is the instance name).
func InstanceName(rr *types.ReconciliationRequest) string {
	return rr.Instance.GetName()
}

// RunlevelGateAction returns an action that checks whether the platform
// orchestrator has reached this component's runlevel. When the runlevel
// has not been cleared, it sets rr.SkipDeploy so that render/deploy/GC
// actions become no-ops while status-reporting actions continue to run.
//
// PlatformReady is an informational condition (Info severity) that does
// not affect the component's Ready status.
//
// By default the component name is derived from strings.ToLower(Kind) and
// the global provision.DefaultRegistry() is used. Pass options to override
// either: WithNameFunc for a custom name derivation, WithRegistry for an
// isolated registry (e.g. in tests).
func RunlevelGateAction(opts ...RunlevelGateOption) actions.Fn {
	cfg := &runlevelGateConfig{
		nameFn: func(rr *types.ReconciliationRequest) string {
			return strings.ToLower(rr.Instance.GetObjectKind().GroupVersionKind().Kind)
		},
		registry: provision.DefaultRegistry(),
		tracker:  provision.GetRunlevelTracker(),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return func(ctx context.Context, rr *types.ReconciliationRequest) error {
		componentName := cfg.nameFn(rr)

		order, found := cfg.registry.LookupOrder(componentName)
		if !found {
			rr.Conditions.MarkTrue(PlatformReadyConditionType,
				conditions.WithSeverity(common.ConditionSeverityInfo),
			)

			return nil
		}

		version := rr.Release.Version.String()
		if !cfg.tracker.IsCleared(version, order) {
			rr.SkipDeploy = true

			msg := fmt.Sprintf("provisioning order %d not yet reached at version %s; waiting for platform orchestrator", order, version)
			logf.FromContext(ctx).Info("Runlevel not cleared, skipping deploy", "order", order, "version", version)

			rr.Conditions.MarkFalse(PlatformReadyConditionType,
				conditions.WithReason("RunlevelNotCleared"),
				conditions.WithMessage("%s", msg),
				conditions.WithSeverity(common.ConditionSeverityInfo),
			)

			// Requeue so the controller rechecks when the tracker advances.
			return odherrors.NewRequeueAfterError(30 * time.Second)
		}

		rr.Conditions.MarkTrue(PlatformReadyConditionType,
			conditions.WithSeverity(common.ConditionSeverityInfo),
		)

		return nil
	}
}
