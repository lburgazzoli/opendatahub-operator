package platform

import (
	"context"
	"fmt"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/status"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/conditions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
)

// componentReadinessChecker implements dag.ReadinessChecker for in-tree
// component CRs. It uses cluster.GetSingleton to find the singleton CR by
// GVK — no DSC required.
type componentReadinessCheckerType struct {
	registry *cr.Registry
	cli      client.Client
}

func componentReadinessChecker(cli client.Client, reg *cr.Registry) *componentReadinessCheckerType {
	return &componentReadinessCheckerType{registry: reg, cli: cli}
}

func (c *componentReadinessCheckerType) IsReady(ctx context.Context, name string) (bool, error) {
	handler := c.registry.Lookup(name)
	if handler == nil {
		return false, fmt.Errorf("component %q: %w", name, dag.ErrUnknownNode)
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(handler.GroupVersionKind())

	switch err := cluster.GetSingleton(ctx, c.cli, u); {
	case err == nil:
		return isUnstructuredReady(u), nil
	case k8serr.IsNotFound(err), meta.IsNoMatchError(err):
		return false, nil
	default:
		return false, fmt.Errorf("component %q: %w", name, err)
	}
}

var _ dag.ReadinessChecker = (*componentReadinessCheckerType)(nil)

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

// isUnstructuredReady returns true if the unstructured object has a Ready
// condition with status "True".
func isUnstructuredReady(u *unstructured.Unstructured) bool {
	conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, raw := range conds {
		c, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if c["type"] == status.ConditionTypeReady && c["status"] == string(metav1.ConditionTrue) {
			return true
		}
	}
	return false
}
