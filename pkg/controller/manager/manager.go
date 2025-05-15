package manager

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
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

// CacheOption is a function that configures cache-specific options
type CacheOption func(*cacheOptions)

// cacheOptions configures cache-specific behavior
type cacheOptions struct {
	// localCacheGVKs is a set of GVKs that should use this cache
	localCacheGVKs map[schema.GroupVersionKind]struct{}
}

// WithLocalTypes configures which GVKs should use this cache
func WithLocalTypes(gvks ...schema.GroupVersionKind) CacheOption {
	return func(o *cacheOptions) {
		for _, gvk := range gvks {
			o.localCacheGVKs[gvk] = struct{}{}
		}
	}
}

// options configures the Manager
type options struct {
	// cache is a custom cache to use instead of the one from the delegate manager
	cache cache.Cache
	// cacheOpts contains the configuration for the custom cache
	cacheOpts *cacheOptions
	// typedGVKs is a set of GVKs that should be handled as typed objects
	typedGVKs map[schema.GroupVersionKind]struct{}
	// unstructuredGVKs is a set of GVKs that should be handled as unstructured objects
	unstructuredGVKs map[schema.GroupVersionKind]struct{}
}

// Option is a function that configures options
type Option func(*options)

// WithCache sets a custom cache for the Manager with specific cache options
func WithCache(cache cache.Cache, opts ...CacheOption) Option {
	return func(o *options) {
		o.cache = cache
		o.cacheOpts = &cacheOptions{
			localCacheGVKs: make(map[schema.GroupVersionKind]struct{}),
		}
		for _, opt := range opts {
			opt(o.cacheOpts)
		}
	}
}

// WithTypedTypes configures GVKs that should be handled as typed objects
func WithTypedTypes(gvks ...schema.GroupVersionKind) Option {
	return func(o *options) {
		for _, gvk := range gvks {
			o.typedGVKs[gvk] = struct{}{}
		}
	}
}

// WithUnstructuredTypes configures GVKs that should be handled as unstructured objects
func WithUnstructuredTypes(gvks ...schema.GroupVersionKind) Option {
	return func(o *options) {
		for _, gvk := range gvks {
			o.unstructuredGVKs[gvk] = struct{}{}
		}
	}
}

// Manager implements manager.Manager by delegating to an underlying manager.Manager
// This allows for extension or interception of manager behavior.
type Manager struct {
	delegate manager.Manager
	cache    cache.Cache
	client   client.Client
	// cacheOpts contains the configuration for the custom cache
	cacheOpts *cacheOptions
	// typedGVKs is a set of GVKs that should be handled as typed objects
	typedGVKs map[schema.GroupVersionKind]struct{}
	// unstructuredGVKs is a set of GVKs that should be handled as unstructured objects
	unstructuredGVKs map[schema.GroupVersionKind]struct{}
}

// New returns a new Manager that wraps the given manager.Manager
func New(delegate manager.Manager, opts ...Option) *Manager {
	options := &options{
		typedGVKs:        make(map[schema.GroupVersionKind]struct{}),
		unstructuredGVKs: make(map[schema.GroupVersionKind]struct{}),
	}
	for _, opt := range opts {
		opt(options)
	}

	m := Manager{
		delegate:         delegate,
		cache:            options.cache,
		cacheOpts:        options.cacheOpts,
		typedGVKs:        options.typedGVKs,
		unstructuredGVKs: options.unstructuredGVKs,
	}

	m.client = NewClient(delegate.GetClient(), &m)

	return &m
}

// IsTypedObject returns true if the given GVK should be handled as a typed object
func (d *Manager) IsTypedObject(gvk schema.GroupVersionKind) bool {
	_, ok := d.typedGVKs[gvk]
	return ok
}

// IsUnstructuredObject returns true if the given GVK should be handled as an unstructured object
func (d *Manager) IsUnstructuredObject(gvk schema.GroupVersionKind) bool {
	_, ok := d.unstructuredGVKs[gvk]
	return ok
}

// GetTypedGVKs returns a slice of all GVKs that should be handled as typed objects
func (d *Manager) GetTypedGVKs() []schema.GroupVersionKind {
	out := make([]schema.GroupVersionKind, 0, len(d.typedGVKs))
	for k := range d.typedGVKs {
		out = append(out, k)
	}
	return out
}

// GetUnstructuredGVKs returns a slice of all GVKs that should be handled as unstructured objects
func (d *Manager) GetUnstructuredGVKs() []schema.GroupVersionKind {
	out := make([]schema.GroupVersionKind, 0, len(d.unstructuredGVKs))
	for k := range d.unstructuredGVKs {
		out = append(out, k)
	}
	return out
}

// Manager interface methods
func (d *Manager) Add(r manager.Runnable) error {
	return d.delegate.Add(r)
}

func (d *Manager) Elected() <-chan struct{} {
	return d.delegate.Elected()
}

func (d *Manager) AddMetricsServerExtraHandler(path string, handler http.Handler) error {
	return d.delegate.AddMetricsServerExtraHandler(path, handler)
}

func (d *Manager) AddHealthzCheck(name string, check healthz.Checker) error {
	return d.delegate.AddHealthzCheck(name, check)
}

func (d *Manager) AddReadyzCheck(name string, check healthz.Checker) error {
	return d.delegate.AddReadyzCheck(name, check)
}

func (d *Manager) Start(ctx context.Context) error {
	return d.delegate.Start(ctx)
}

func (d *Manager) GetWebhookServer() webhook.Server {
	return d.delegate.GetWebhookServer()
}

func (d *Manager) GetLogger() logr.Logger {
	return d.delegate.GetLogger()
}

func (d *Manager) GetControllerOptions() config.Controller {
	return d.delegate.GetControllerOptions()
}

// Cluster interface methods
func (d *Manager) GetHTTPClient() *http.Client {
	return d.delegate.GetHTTPClient()
}

func (d *Manager) GetConfig() *rest.Config {
	return d.delegate.GetConfig()
}

// GetCache returns the cache to use for the given GVK
func (d *Manager) GetCache() cache.Cache {
	if d.cache == nil {
		return d.delegate.GetCache()
	}
	return d.cache
}

// GetCacheForType returns the cache to use for the given GVK
func (d *Manager) GetCacheForType(gvk schema.GroupVersionKind) cache.Cache {
	if d.cache == nil || d.cacheOpts == nil {
		return d.delegate.GetCache()
	}

	if _, ok := d.cacheOpts.localCacheGVKs[gvk]; ok {
		return d.cache
	}
	return d.delegate.GetCache()
}

// GetCacheForObject returns the cache to use for the given object
func (d *Manager) GetCacheForObject(obj runtime.Object) cache.Cache {
	return d.GetCacheForType(obj.GetObjectKind().GroupVersionKind())
}

func (d *Manager) GetScheme() *runtime.Scheme {
	return d.GetClient().Scheme()
}

func (d *Manager) GetClient() client.Client {
	return d.delegate.GetClient()
}

func (d *Manager) GetFieldIndexer() client.FieldIndexer {
	return d.delegate.GetFieldIndexer()
}

func (d *Manager) GetEventRecorderFor(name string) record.EventRecorder {
	return d.delegate.GetEventRecorderFor(name)
}

func (d *Manager) GetRESTMapper() meta.RESTMapper {
	return d.delegate.GetRESTMapper()
}

func (d *Manager) GetAPIReader() client.Reader {
	return d.delegate.GetAPIReader()
}
