package bdd

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestCtxKey struct{}

type TestContext struct {
	config         *Config
	restConfig     *rest.Config
	client         client.Client
	resourceClient *ResourceClient
	discovery      discovery.DiscoveryInterface
	resolver       *ResourceResolver
	variables      *VariableManager
	decoder        runtime.Decoder
	decoderOnce    sync.Once
}

func NewTestContext(restConfig *rest.Config) (*TestContext, error) {
	if restConfig == nil {
		return nil, errors.New("restConfig cannot be nil")
	}

	k8sClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	resolver, err := NewResourceResolver(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource resolver: %w", err)
	}

	cfg, err := LoadBDDConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &TestContext{
		config:     &cfg,
		restConfig: restConfig,
		client:     k8sClient,
		resourceClient: &ResourceClient{
			client:   k8sClient,
			resolver: resolver,
		},
		discovery: discoveryClient,
		resolver:  resolver,
		variables: NewVariableManager(),
	}, nil
}

func (tc *TestContext) Config() *Config {
	return tc.config
}

func (tc *TestContext) Client() client.Client {
	return tc.client
}

func (tc *TestContext) ResourceClient() *ResourceClient {
	return tc.resourceClient
}

func (tc *TestContext) Discovery() discovery.DiscoveryInterface {
	return tc.discovery
}

func (tc *TestContext) Resolver() *ResourceResolver {
	return tc.resolver
}

func (tc *TestContext) Variables() *VariableManager {
	return tc.variables
}

func (tc *TestContext) JQ() *JQ {
	return &JQ{
		vm: tc.variables,
	}
}

func (tc *TestContext) Decoder() runtime.Decoder {
	tc.decoderOnce.Do(func() {
		scheme := tc.client.Scheme()
		tc.decoder = serializer.NewCodecFactory(scheme).UniversalDeserializer()
	})
	return tc.decoder
}

func TestCtx(ctx context.Context) *TestContext {
	//nolint:forcetypeassert,errcheck
	return ctx.Value(TestCtxKey{}).(*TestContext)
}
