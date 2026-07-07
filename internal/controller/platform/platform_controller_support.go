package platform

import (
	"context"
	"fmt"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/base"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
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

type platformReadinessCheckerType struct {
	moduleChecker     *moduleReadinessCheckerType
	componentRegistry base.Registry[cr.ComponentHandler]
	cli               client.Client
}

func platformReadinessChecker(
	cli client.Client,
	platformVersion string,
	componentRegistry base.Registry[cr.ComponentHandler],
) *platformReadinessCheckerType {
	return &platformReadinessCheckerType{
		moduleChecker:     moduleReadinessChecker(cli, platformVersion),
		componentRegistry: componentRegistry,
		cli:               cli,
	}
}

func (c *platformReadinessCheckerType) IsReady(ctx context.Context, name string) (bool, error) {
	if c.componentRegistry != nil {
		if handler := c.componentRegistry.Lookup(name); handler != nil {
			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(handler.GetGroupVersionKind())

			switch err := cluster.GetSingleton(ctx, c.cli, u); {
			case err == nil:
			case k8serr.IsNotFound(err), meta.IsNoMatchError(err):
				return false, nil
			default:
				return false, fmt.Errorf("getting tracked component %s: %w", name, err)
			}

			return conditions.IsStatusConditionTrue(
				common.NewUnstructuredPlatformObject(u),
				status.ConditionTypeReady,
			), nil
		}
	}

	return c.moduleChecker.IsReady(ctx, name)
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
var _ dag.ReadinessChecker = (*platformReadinessCheckerType)(nil)
