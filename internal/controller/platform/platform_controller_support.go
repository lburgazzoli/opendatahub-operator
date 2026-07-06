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

// moduleReadinessChecker implements dag.ReadinessChecker for PlatformModule
// CRs. It checks both the Ready condition and the platform version handshake
// so the DAG only advances once the module operator acknowledges the upgrade.
type moduleReadinessCheckerType struct {
	cli             client.Client
	platformVersion string
}

func moduleReadinessChecker(cli client.Client, platformVersion string) *moduleReadinessCheckerType {
	return &moduleReadinessCheckerType{cli: cli, platformVersion: platformVersion}
}

func (c *moduleReadinessCheckerType) IsReady(ctx context.Context, name string) (bool, error) {
	pm := &configv1alpha1.PlatformModule{}

	switch err := c.cli.Get(ctx, client.ObjectKey{Name: name}, pm); {
	case err == nil:
	case k8serr.IsNotFound(err):
		return false, nil
	default:
		return false, fmt.Errorf("getting PlatformModule %s: %w", name, err)
	}

	if !conditions.IsStatusConditionTrue(pm, status.ConditionTypeReady) {
		return false, nil
	}

	// Block DAG advancement until the module operator acknowledges the upgrade.
	if c.platformVersion != "" && pm.Status.Release.Version.String() != c.platformVersion {
		return false, nil
	}

	return true, nil
}

var _ dag.ReadinessChecker = (*moduleReadinessCheckerType)(nil)
