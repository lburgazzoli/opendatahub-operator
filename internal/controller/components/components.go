package components

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	odhcache "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/cache"
	ctrlmanager "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/manager"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"

	// Side effect imports to ensure component packages are registered.
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/codeflare"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/dashboard"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/datasciencepipelines"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/feastoperator"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/kserve"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/kueue"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/modelcontroller"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/modelmeshserving"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/modelregistry"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/ray"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/trainingoperator"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/trustyai"
	_ "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/workbenches"
)

type ComponentManagerOption func(*cache.Options)

func WithDefaultNamespaces(namespaces map[string]cache.Config) ComponentManagerOption {
	return func(o *cache.Options) {
		o.DefaultNamespaces = namespaces
	}
}

func WithByObject(byObject map[client.Object]cache.ByObject) ComponentManagerOption {
	return func(o *cache.Options) {
		if o.ByObject == nil {
			o.ByObject = make(map[client.Object]cache.ByObject)
		}
		for obj, cfg := range byObject {
			o.ByObject[obj] = cfg
		}
	}
}

func WithDefaultLabelSelector(selector labels.Selector) ComponentManagerOption {
	return func(o *cache.Options) {
		o.DefaultLabelSelector = selector
	}
}

//nolint:ireturn
func createComponentManager(
	_ context.Context,
	mgr manager.Manager,
	opts ...ComponentManagerOption,
) (types.ControllerManager, error) {
	// options
	co := cache.Options{
		HTTPClient:                  mgr.GetHTTPClient(),
		Scheme:                      mgr.GetScheme(),
		Mapper:                      mgr.GetRESTMapper(),
		ReaderFailOnMissingInformer: true,
		DefaultTransform:            odhcache.DefaultTransformFn,
	}

	for _, opt := range opts {
		opt(&co)
	}

	// Create cache with configured options
	cc, err := cache.New(mgr.GetConfig(), co)
	if err != nil {
		return nil, fmt.Errorf("unable to create cache: %w", err)
	}

	if err := mgr.Add(cc); err != nil {
		return nil, fmt.Errorf("unable to add the components cache to the manager: %w", err)
	}

	// Create a specialized manager that shares component types using the base manager
	cm, err := ctrlmanager.Wrap(
		mgr,
		ctrlmanager.WithTypedTypes(
			gvk.ConfigMap,
			gvk.Template,
		),
		ctrlmanager.WithTypedTypes(
			gvk.CoreSharedTypes...,
		),
		ctrlmanager.WithUnstructuredTypes(
			gvk.Deployment,
		),
		ctrlmanager.WithCache(
			cc,
			ctrlmanager.WithSharedTypes(gvk.PlatformTypes...),
			ctrlmanager.WithSharedTypes(gvk.CoreSharedTypes...),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create specialized manager: %w", err)
	}

	return cm, nil
}

func CreateComponentReconcilers(
	ctx context.Context,
	mgr manager.Manager,
	opts ...ComponentManagerOption,
) error {
	l := logf.FromContext(ctx)

	// Create a specialized manager for components with default namespaces
	componentMgr, err := createComponentManager(ctx, mgr, opts...)
	if err != nil {
		return fmt.Errorf("unable to create component manager: %w", err)
	}

	return cr.ForEach(func(ch cr.ComponentHandler) error {
		l.Info("creating reconciler", "type", "component", "name", ch.GetName())
		if err := ch.NewComponentReconciler(ctx, componentMgr); err != nil {
			return fmt.Errorf("error creating %s component reconciler: %w", ch.GetName(), err)
		}

		return nil
	})
}
