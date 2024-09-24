package client

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"

	odhCli "github.com/opendatahub-io/opendatahub-operator/v2/pkg/client/clientset/versioned"
)

const (
	DiscoveryLimiterBurst = 30
)

var scaleConverter = scale.NewScaleConverter()
var codecs = serializer.NewCodecFactory(scaleConverter.Scheme())

// type aliases to avoid interface names duplications
type ctrClient = ctrl.Client
type k8sClient = kubernetes.Interface
type odhClient = odhCli.Interface

type Client struct {
	ctrClient
	k8sClient
	odhClient

	Discovery discovery.DiscoveryInterface

	dynamic          *dynamic.DynamicClient
	scheme           *runtime.Scheme
	config           *rest.Config
	rest             rest.Interface
	mapper           meta.RESTMapper
	discoveryCache   discovery.CachedDiscoveryInterface
	discoveryLimiter *rate.Limiter
}

func NewClient(cfg *rest.Config, scheme *runtime.Scheme, cc ctrl.Client) (*Client, error) {
	discoveryCl, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a Discovery client: %w", err)
	}

	kubeCl, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a Kubernetes client: %w", err)
	}

	restCl, err := newRESTClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a REST client: %w", err)
	}

	dynCl, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a Dynamic client: %w", err)
	}

	oCl, err := odhCli.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a ODH client: %w", err)
	}

	c := Client{
		ctrClient: cc,
		k8sClient: kubeCl,
		Discovery: discoveryCl,
		odhClient: oCl,
		dynamic:   dynCl,
		scheme:    scheme,
		config:    cfg,
		rest:      restCl,
	}

	c.discoveryLimiter = rate.NewLimiter(rate.Every(time.Second), DiscoveryLimiterBurst)
	c.discoveryCache = memory.NewMemCacheClient(discoveryCl)
	c.mapper = restmapper.NewDeferredDiscoveryRESTMapper(c.discoveryCache)

	return &c, nil
}

func newRESTClientForConfig(config *rest.Config) (*rest.RESTClient, error) {
	cfg := rest.CopyConfig(config)
	// so that the RESTClientFor doesn't complain
	cfg.GroupVersion = &schema.GroupVersion{}
	cfg.NegotiatedSerializer = codecs.WithoutConversion()

	if len(cfg.UserAgent) == 0 {
		cfg.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	rc, err := rest.RESTClientFor(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to construct a REST client: %w", err)
	}

	return rc, nil
}

//nolint:ireturn
func (c *Client) Dynamic(obj ctrl.Object) (dynamic.ResourceInterface, error) {
	if c.discoveryLimiter.Allow() {
		c.discoveryCache.Invalidate()
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to identify preferred resource mapping for %s/%s: %w",
			gvk.GroupKind(),
			gvk.Version,
			err)
	}

	var dr dynamic.ResourceInterface

	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if obj.GetNamespace() == "" {
			return nil, fmt.Errorf(
				"missing required filed: namespace, gvk=%s, name=%s",
				obj.GetObjectKind().GroupVersionKind().String(),
				obj.GetName())
		}

		dr = c.dynamic.Resource(mapping.Resource).Namespace(obj.GetNamespace())
	} else {
		dr = c.dynamic.Resource(mapping.Resource)
	}

	return dr, nil
}

func (c *Client) Apply(ctx context.Context, obj *unstructured.Unstructured, opts metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	d, err := c.Dynamic(obj)
	if err != nil {
		return nil, fmt.Errorf("unable to create dynamic client: %w", err)
	}

	u := obj.DeepCopy()
	u.SetResourceVersion("")
	u.SetManagedFields(nil)

	ret, err := d.Apply(ctx, obj.GetName(), u, opts)
	if err != nil {
		return nil, fmt.Errorf("unable to apply object %s: %w", obj, err)
	}

	return ret, nil
}
