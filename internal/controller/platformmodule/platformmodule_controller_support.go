package platformmodule

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	configv1alpha1 "github.com/opendatahub-io/opendatahub-operator/v2/api/config/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	odherrors "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/errors"
	odhtype "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

type entryMode string

const (
	entryModeUnknown     entryMode = "unknown"
	entryModeDeployer    entryMode = "deployer"
	entryModeTrackerOnly entryMode = "trackerOnly"
)

func (r *Reconciler) modeFor(name string) entryMode {
	switch {
	case r.Registry != nil && r.Registry.Lookup(name) != nil:
		return entryModeDeployer
	case r.ComponentRegistry != nil && r.ComponentRegistry.Lookup(name) != nil:
		return entryModeTrackerOnly
	default:
		return entryModeUnknown
	}
}

func (r *Reconciler) trackedGVKFor(name string) (schema.GroupVersionKind, entryMode, bool) {
	if r.Registry != nil {
		if handler := r.Registry.Lookup(name); handler != nil {
			return handler.GetGroupVersionKind(), entryModeDeployer, true
		}
	}

	if r.ComponentRegistry != nil {
		if handler := r.ComponentRegistry.Lookup(name); handler != nil {
			return handler.GroupVersionKind(), entryModeTrackerOnly, true
		}
	}

	return schema.GroupVersionKind{}, entryModeUnknown, false
}

func (r *Reconciler) onDeployer(action actions.Fn) actions.Fn {
	return func(ctx context.Context, rr *odhtype.ReconciliationRequest) error {
		if r.modeFor(rr.Instance.GetName()) != entryModeDeployer {
			return nil
		}

		return action(ctx, rr)
	}
}

func (r *Reconciler) validateMode(_ context.Context, rr *odhtype.ReconciliationRequest) error {
	if mode := r.modeFor(rr.Instance.GetName()); mode == entryModeUnknown {
		return fmt.Errorf("platform entry %q is not registered in the module/component registries", rr.Instance.GetName())
	}

	return nil
}

func (r *Reconciler) gateEntryRunlevel(_ context.Context, rr *odhtype.ReconciliationRequest) error {
	order, found := r.ProvisionReg.LookupOrder(rr.Instance.GetName())
	if !found {
		return nil
	}

	version := rr.Release.Version.String()
	if r.Tracker.IsCleared(version, order) {
		return nil
	}

	rr.SkipDeploy = true

	return odherrors.NewRequeueAfterError(30 * time.Second)
}

// resourceRefsFrom converts a slice of unstructured resources to ResourceRefs
// for tracking in PlatformModule.Status.Resources.
func resourceRefsFrom(rs []unstructured.Unstructured) []configv1alpha1.ResourceRef {
	if len(rs) == 0 {
		return nil
	}
	refs := make([]configv1alpha1.ResourceRef, 0, len(rs))
	for _, r := range rs {
		k := r.GroupVersionKind()
		refs = append(refs, configv1alpha1.ResourceRef{
			Group:     k.Group,
			Version:   k.Version,
			Kind:      k.Kind,
			Namespace: r.GetNamespace(),
			Name:      r.GetName(),
		})
	}
	return refs
}

func shouldTrackResourceRef(k schema.GroupVersionKind) bool {
	switch k {
	case gvk.CustomResourceDefinition:
		return false
	case gvk.Namespace:
		return false
	default:
		return true
	}
}

func trackedResourceRefsFrom(rs []unstructured.Unstructured) []configv1alpha1.ResourceRef {
	return filterTrackedResourceRefs(resourceRefsFrom(rs))
}

func filterTrackedResourceRefs(refs []configv1alpha1.ResourceRef) []configv1alpha1.ResourceRef {
	if len(refs) == 0 {
		return nil
	}

	tracked := make([]configv1alpha1.ResourceRef, 0, len(refs))
	for _, ref := range refs {
		if !shouldTrackResourceRef(ref.GroupVersionKind()) {
			continue
		}
		tracked = append(tracked, ref)
	}

	if len(tracked) == 0 {
		return nil
	}

	return tracked
}

func getTrackedSingletonObject(
	ctx context.Context,
	cli client.Client,
	gvk schema.GroupVersionKind,
) (*common.UnstructuredModule, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)

	if err := cluster.GetSingleton(ctx, cli, u); err != nil {
		return nil, err
	}

	return common.NewUnstructuredModule(u), nil
}

func trackedReleaseVersion(obj common.WithReleases) string {
	releases := obj.GetReleaseStatus()
	if releases == nil {
		return ""
	}

	for _, release := range *releases {
		if release.Name == "platform" {
			return release.Version
		}
	}

	return ""
}

// ensureConfigMap returns the index of the ConfigMap with the given name in
// resources, or appends a new empty ConfigMap and returns its index.
func ensureConfigMap(rs *[]unstructured.Unstructured, name string, namespace string) (int, error) {
	configMapGVK := gvk.ConfigMap
	for i, r := range *rs {
		if r.GroupVersionKind() == configMapGVK && r.GetName() == name {
			return i, nil
		}
	}

	cm := modules.BuildPlatformConfigMap(name, namespace, "")
	u, err := resources.ToUnstructured(cm)
	if err != nil {
		return 0, err
	}

	*rs = append(*rs, *u)

	return len(*rs) - 1, nil
}
