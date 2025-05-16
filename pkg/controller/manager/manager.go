package manager

import (
	"context"
	"fmt"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	ctrlclient "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/client"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type CacheOption func(*cacheOptions)

type cacheOptions struct {
	sharedCacheGVKs map[schema.GroupVersionKind]struct{}
}

func WithSharedTypes(gvks ...schema.GroupVersionKind) CacheOption {
	return func(o *cacheOptions) {
		for _, gvk := range gvks {
			o.sharedCacheGVKs[gvk] = struct{}{}
		}
	}
}

type options struct {
	cache            cache.Cache
	cacheOpts        *cacheOptions
	typedGVKs        map[schema.GroupVersionKind]struct{}
	unstructuredGVKs map[schema.GroupVersionKind]struct{}
}

type Option func(*options)

func WithCache(cache cache.Cache, opts ...CacheOption) Option {
	return func(o *options) {
		o.cache = cache
		o.cacheOpts = &cacheOptions{
			sharedCacheGVKs: make(map[schema.GroupVersionKind]struct{}),
		}
		for _, opt := range opts {
			opt(o.cacheOpts)
		}
	}
}

func WithTypedTypes(gvks ...schema.GroupVersionKind) Option {
	return func(o *options) {
		for _, gvk := range gvks {
			o.typedGVKs[gvk] = struct{}{}
		}
	}
}

func WithUnstructuredTypes(gvks ...schema.GroupVersionKind) Option {
	return func(o *options) {
		for _, gvk := range gvks {
			o.unstructuredGVKs[gvk] = struct{}{}
		}
	}
}

type Manager struct {
	delegate         manager.Manager
	cache            cache.Cache
	client           client.Client
	cacheOpts        *cacheOptions
	typedGVKs        map[schema.GroupVersionKind]struct{}
	unstructuredGVKs map[schema.GroupVersionKind]struct{}
}

// New returns a new Manager that wraps the given manager.Manager
func New(delegate manager.Manager, opts ...Option) (*Manager, error) {
	options := &options{
		typedGVKs:        make(map[schema.GroupVersionKind]struct{}),
		unstructuredGVKs: make(map[schema.GroupVersionKind]struct{}),
	}
	for _, opt := range opts {
		opt(options)
	}

	m := Manager{
		delegate:         delegate,
		client:           delegate.GetClient(),
		cache:            options.cache,
		cacheOpts:        options.cacheOpts,
		typedGVKs:        options.typedGVKs,
		unstructuredGVKs: options.unstructuredGVKs,
	}

	if m.cache != nil {
		c, err := client.New(delegate.GetConfig(), client.Options{
			HTTPClient: delegate.GetHTTPClient(),
			Scheme:     delegate.GetScheme(),
			Mapper:     delegate.GetRESTMapper(),
			Cache: &client.CacheOptions{
				Reader: m.cache,
			},
		})

		if err != nil {
			return nil, fmt.Errorf("failed to create client: %w", err)
		}

		m.client = ctrlclient.New(delegate.GetClient()).WithFuncs(ctrlclient.Funcs{
			Get:  m.get(c),
			List: m.list(c),
		})
	}

	return &m, nil
}

func (m *Manager) IsTypedObject(gvk schema.GroupVersionKind) bool {
	_, ok := m.typedGVKs[gvk]
	return ok
}

func (m *Manager) IsUnstructuredObject(gvk schema.GroupVersionKind) bool {
	_, ok := m.unstructuredGVKs[gvk]
	return ok
}

func (m *Manager) Add(r manager.Runnable) error {
	return m.delegate.Add(r)
}

func (m *Manager) Elected() <-chan struct{} {
	return m.delegate.Elected()
}

func (m *Manager) AddMetricsServerExtraHandler(path string, handler http.Handler) error {
	return m.delegate.AddMetricsServerExtraHandler(path, handler)
}

func (m *Manager) AddHealthzCheck(name string, check healthz.Checker) error {
	return m.delegate.AddHealthzCheck(name, check)
}

func (m *Manager) AddReadyzCheck(name string, check healthz.Checker) error {
	return m.delegate.AddReadyzCheck(name, check)
}

func (m *Manager) Start(ctx context.Context) error {
	return m.delegate.Start(ctx)
}

func (m *Manager) GetWebhookServer() webhook.Server {
	return m.delegate.GetWebhookServer()
}

func (m *Manager) GetLogger() logr.Logger {
	return m.delegate.GetLogger()
}

func (m *Manager) GetControllerOptions() config.Controller {
	return m.delegate.GetControllerOptions()
}

func (m *Manager) GetHTTPClient() *http.Client {
	return m.delegate.GetHTTPClient()
}

func (m *Manager) GetConfig() *rest.Config {
	return m.delegate.GetConfig()
}

// GetCache returns the cache to use for the given GVK
func (m *Manager) GetCache() cache.Cache {
	if m.cache == nil {
		return m.delegate.GetCache()
	}
	return m.cache
}

// GetCacheForType returns the cache to use for the given GVK
func (m *Manager) GetCacheForType(gvk schema.GroupVersionKind) cache.Cache {
	if m.cache == nil || m.cacheOpts == nil {
		return m.delegate.GetCache()
	}

	if _, ok := m.cacheOpts.sharedCacheGVKs[gvk]; ok {
		return m.delegate.GetCache()
	}
	return m.cache
}

func (m *Manager) GetScheme() *runtime.Scheme {
	return m.GetClient().Scheme()
}

func (m *Manager) GetClient() client.Client {
	return m.client
}

func (m *Manager) GetFieldIndexer() client.FieldIndexer {
	return m.delegate.GetFieldIndexer()
}

func (m *Manager) GetEventRecorderFor(name string) record.EventRecorder {
	return m.delegate.GetEventRecorderFor(name)
}

func (m *Manager) GetRESTMapper() meta.RESTMapper {
	return m.delegate.GetRESTMapper()
}

func (m *Manager) GetAPIReader() client.Reader {
	return m.delegate.GetAPIReader()
}

func (m *Manager) Source(
	obj client.Object,
	eh handler.EventHandler,
	predicates ...predicate.Predicate,
) source.Source {
	// assuming gvk is always set to the object
	gvk := obj.GetObjectKind().GroupVersionKind()

	var wo client.Object

	switch {
	case m.IsTypedObject(gvk):
		wo = obj
	case m.IsUnstructuredObject(gvk):
		wo = resources.GvkToUnstructured(gvk)
	default:
		wo = resources.GvkToPartial(gvk)
	}

	return source.Kind(
		m.GetCacheForType(gvk),
		wo,
		eh,
		predicates...,
	)
}

func (m *Manager) get(xc client.Client) func(
	cli client.Client,
	ctx context.Context,
	key client.ObjectKey,
	out client.Object,
	opts ...client.GetOption,
) error {
	return func(
		cli client.Client,
		ctx context.Context,
		key client.ObjectKey,
		out client.Object,
		opts ...client.GetOption,
	) error {
		gvk := out.GetObjectKind().GroupVersionKind()

		if _, ok := m.cacheOpts.sharedCacheGVKs[gvk]; ok {
			return cli.Get(ctx, key, out, opts...)
		}

		switch {
		case m.IsTypedObject(gvk):
			return xc.Get(ctx, key, out, opts...)

		case m.IsUnstructuredObject(gvk):
			u, err := ToUnstructured(xc.Scheme(), out)
			if err != nil {
				return err
			}

			if err := xc.Get(ctx, key, u, opts...); err != nil {
				return err
			}

			return FromUnstructured(xc.Scheme(), u, out)

		default:
			p, err := ToPartial(xc.Scheme(), out)
			if err != nil {
				return err
			}

			if err := xc.Get(ctx, key, p, opts...); err != nil {
				return err
			}

			return FromPartial(xc.Scheme(), p, out)
		}
	}
}

func (m *Manager) list(xc client.Client) func(
	cli client.Client,
	ctx context.Context,
	out client.ObjectList,
	opts ...client.ListOption,
) error {
	return func(
		cli client.Client,
		ctx context.Context,
		out client.ObjectList,
		opts ...client.ListOption,
	) error {
		gvk, err := resources.GetGroupVersionKindForList(m.client.Scheme(), out)
		if err != nil {
			return err
		}

		if _, ok := m.cacheOpts.sharedCacheGVKs[gvk]; ok {
			return cli.List(ctx, out, opts...)
		}

		switch {
		case m.IsTypedObject(gvk):
			return xc.List(ctx, out, opts...)

		case m.IsUnstructuredObject(gvk):
			l, err := ToUnstructuredList(xc.Scheme(), out)
			if err != nil {
				return err
			}

			if err := xc.List(ctx, l, opts...); err != nil {
				return err
			}

			return FromUnstructuredList(xc.Scheme(), *l, out)

		default:
			l, err := ToPartialList(xc.Scheme(), out)
			if err != nil {
				return err
			}

			if err := xc.List(ctx, l, opts...); err != nil {
				return err
			}

			return FromPartialList(xc.Scheme(), *l, out)
		}
	}
}
