package platform

import (
	"context"
	"fmt"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
)

// platformModuleReadinessChecker implements dag.ReadinessChecker by reading
// PlatformModule CR status conditions directly. Used by walkModuleDAG so the
// Platform controller drives DAG gating based on PlatformModule health.
type platformModuleReadinessChecker struct {
	cli             client.Client
	platformVersion string
}

func (c *platformModuleReadinessChecker) IsReady(ctx context.Context, name string) (bool, error) {
	pm := &configv1alpha1.PlatformModule{}
	if err := c.cli.Get(ctx, client.ObjectKey{Name: name}, pm); err != nil {
		if k8serr.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("getting PlatformModule %s: %w", name, err)
	}

	if !conditions.IsStatusConditionTrue(pm, status.ConditionTypeReady) {
		return false, nil
	}

	if c.platformVersion != "" && pm.Status.Release.Version.String() != c.platformVersion {
		return false, nil
	}

	return true, nil
}

var _ dag.ReadinessChecker = (*platformModuleReadinessChecker)(nil)
