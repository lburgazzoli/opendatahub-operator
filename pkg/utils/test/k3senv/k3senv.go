package k3senv

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/scheme"
	"github.com/opendatahub-io/opendatahub-operator/v2/tests/envtestutil"
)

const (
	DefaultK3sImage          = "rancher/k3s:v1.32.9-k3s1"
	DefaultWebhookPort       = 9443
	DefaultWebhookServerHost = "0.0.0.0"
	DefaultCertDirPrefix     = "/tmp/k3senv-certs-"
	DefaultCertValidity      = 24 * time.Hour

	CACertFileName = "cert-ca.pem"
	CertFileName   = "cert-tls.pem"
	KeyFileName    = "key-tls.pem"

	WebhookURLScheme   = "https"
	WebhookConvertPath = "/convert"

	CRDEstablishmentPollInterval = 500 * time.Millisecond
	WebhookReadyTimeout          = 30 * time.Second
	WebhookReadyPollInterval     = 500 * time.Millisecond
	WebhookHealthCheckTimeout    = 2 * time.Second
)

var (
	// CertificateSANs defines the Subject Alternative Names for generated TLS certificates.
	// Includes common Docker gateway IPs and hostnames used by testcontainers.
	CertificateSANs = []string{
		"host.docker.internal",
		"host.testcontainers.internal",
		"localhost",
		"*.*.svc",
		"*.*.svc.cluster.local",
		"127.0.0.1",
		"172.17.0.1", // Common Docker gateway
		"172.18.0.1",
		"172.19.0.1",
		"172.20.0.1",
	}
)

type TeardownTask func(context.Context) error

type K3sEnv struct {
	container *k3s.K3sContainer
	cfg       *rest.Config
	cli       client.Client
	scheme    *runtime.Scheme
	certDir   string
	certData  *CertData

	k3sImage            string
	kustomizationPaths  []string
	objects             []client.Object
	autoInstallWebhooks bool
	webhookPort         int
	webhookHost         string
	certValidity        time.Duration
	manifestCategories  ManifestCategories
	teardownTasks       []TeardownTask
}

type ManifestCategories struct {
	CRDs           []unstructured.Unstructured
	WebhookConfigs []unstructured.Unstructured
}

func New(opts ...Option) (*K3sEnv, error) {
	options := &Options{
		WebhookPort:  DefaultWebhookPort,
		K3sImage:     DefaultK3sImage,
		CertValidity: DefaultCertValidity,
	}
	options.ApplyOptions(opts)

	// Validate required fields
	if options.WebhookPort < 1 || options.WebhookPort > 65535 {
		return nil, fmt.Errorf("webhookPort must be in range 1-65535, got %d", options.WebhookPort)
	}
	if options.K3sImage == "" {
		return nil, errors.New("k3sImage cannot be empty")
	}

	env := &K3sEnv{
		scheme:              options.Scheme,
		certDir:             options.CertDir,
		teardownTasks:       []TeardownTask{},
		k3sImage:            options.K3sImage,
		kustomizationPaths:  options.KustomizationPaths,
		objects:             options.Objects,
		autoInstallWebhooks: options.AutoInstallWebhooks,
		webhookPort:         options.WebhookPort,
		certValidity:        options.CertValidity,
	}

	if env.scheme == nil {
		s, err := scheme.New()
		if err != nil {
			return nil, fmt.Errorf("unable to build runtime.Scheme: %w", err)
		}

		env.scheme = s
	}

	return env, nil
}

// Start initializes and starts the k3s test environment.
//
// This method orchestrates the following steps:
//  1. Starts the k3s container
//  2. Retrieves and configures kubeconfig
//  3. Creates Kubernetes clients
//  4. Generates TLS certificates for webhook testing
//  5. Renders kustomizations and loads objects into manifest categories
//  6. Installs CRDs and waits for them to be established
//  7. Optionally installs and configures webhooks (if AutoInstallWebhooks is enabled)
//
// After Start() completes successfully, callers MUST call Stop() to clean up resources.
// It's recommended to use defer or test cleanup handlers to ensure Stop() is called.
//
// Example:
//
//	env, err := k3senv.New(...)
//	if err != nil {
//	    return err
//	}
//	if err := env.Start(ctx); err != nil {
//	    return err
//	}
//	defer env.Stop(ctx)
func (e *K3sEnv) Start(ctx context.Context) error {
	if err := e.startK3sContainer(ctx); err != nil {
		return err
	}

	if err := e.setupKubeConfig(ctx); err != nil {
		return err
	}

	if err := e.createKubernetesClients(); err != nil {
		return err
	}

	if err := e.setupCertificates(); err != nil {
		return err
	}

	if err := e.prepareManifests(); err != nil {
		return err
	}

	if err := e.installCRDsIfNeeded(ctx); err != nil {
		return err
	}

	if e.autoInstallWebhooks {
		if err := e.InstallWebhooks(ctx); err != nil {
			return fmt.Errorf("failed to auto-install webhooks: %w", err)
		}
	}

	return nil
}

func (e *K3sEnv) startK3sContainer(ctx context.Context) error {
	container, err := k3s.Run(ctx, e.k3sImage)
	if err != nil {
		return fmt.Errorf("failed to start k3s container: %w", err)
	}
	e.container = container
	return nil
}

func (e *K3sEnv) setupKubeConfig(ctx context.Context) error {
	kubeconfig, err := e.container.GetKubeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create REST config: %w", err)
	}
	e.cfg = cfg
	return nil
}

func (e *K3sEnv) createKubernetesClients() error {
	cli, err := client.New(e.cfg, client.Options{Scheme: e.scheme})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	e.cli = cli

	return nil
}

func (e *K3sEnv) setupCertificates() error {
	autoGeneratedCertDir := false
	if e.certDir == "" {
		e.certDir = fmt.Sprintf("%s%s", DefaultCertDirPrefix, e.container.GetContainerID())
		autoGeneratedCertDir = true
	}

	certData, err := generateCertificates(e.certDir, e.certValidity)
	if err != nil {
		return fmt.Errorf("failed to generate certificates: %w", err)
	}

	e.certData = certData

	// Add cleanup for auto-generated cert directories
	if autoGeneratedCertDir {
		certDirToClean := e.certDir
		e.AddTeardownFn(func(ctx context.Context) error {
			return os.RemoveAll(certDirToClean)
		})
	}

	return nil
}

func (e *K3sEnv) prepareManifests() error {
	// Clear/initialize manifest categories
	e.manifestCategories = ManifestCategories{}

	// Load from kustomization paths
	if len(e.kustomizationPaths) > 0 {
		if err := e.renderKustomizations(e.kustomizationPaths); err != nil {
			return fmt.Errorf("failed to render kustomizations: %w", err)
		}
	}

	// Load from direct objects
	if len(e.objects) > 0 {
		if err := e.loadObjectsToManifests(e.objects); err != nil {
			return fmt.Errorf("failed to load objects: %w", err)
		}
	}

	return nil
}

func (e *K3sEnv) installCRDsIfNeeded(ctx context.Context) error {
	if len(e.manifestCategories.CRDs) == 0 {
		return nil
	}

	if err := e.installCRDs(ctx); err != nil {
		return fmt.Errorf("failed to install CRDs: %w", err)
	}

	if err := e.waitForCRDsEstablished(ctx, extractNames(e.manifestCategories.CRDs)); err != nil {
		return fmt.Errorf("failed waiting for CRDs to be established: %w", err)
	}

	return nil
}

func (e *K3sEnv) Stop(ctx context.Context) error {
	var errs []error

	// Run teardown tasks in reverse order
	for i := len(e.teardownTasks) - 1; i >= 0; i-- {
		if err := e.teardownTasks[i](ctx); err != nil {
			errs = append(errs, fmt.Errorf("teardown task %d failed: %w", i, err))
		}
	}

	// Stop container
	if e.container != nil {
		if err := testcontainers.TerminateContainer(e.container); err != nil {
			errs = append(errs, fmt.Errorf("failed to terminate container: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (e *K3sEnv) AddTeardown(task TeardownTask) {
	e.teardownTasks = append(e.teardownTasks, task)
}

func (e *K3sEnv) AddTeardownFn(fn func(context.Context) error) {
	e.AddTeardown(fn)
}

func (e *K3sEnv) Config() *rest.Config {
	return e.cfg
}

func (e *K3sEnv) Client() client.Client {
	return e.cli
}

func (e *K3sEnv) Scheme() *runtime.Scheme {
	return e.scheme
}

func (e *K3sEnv) CertDir() string {
	return e.certDir
}

func (e *K3sEnv) CABundle() []byte {
	if e.certData == nil {
		return nil
	}

	return e.certData.CABundle()
}

// GetKubeconfig returns the kubeconfig data for the k3s cluster.
// Returns an error if called before Start().
func (e *K3sEnv) GetKubeconfig(ctx context.Context) ([]byte, error) {
	if e.container == nil {
		return nil, errors.New("cluster not started - call Start() first")
	}

	return e.container.GetKubeConfig(ctx)
}

// GetCRDs returns a defensive copy of the CRDs loaded from manifests.
// DeepCopy is used to prevent callers from accidentally mutating internal state.
func (e *K3sEnv) GetCRDs() []unstructured.Unstructured {
	if len(e.manifestCategories.CRDs) == 0 {
		return nil
	}

	result := make([]unstructured.Unstructured, len(e.manifestCategories.CRDs))
	for i := range e.manifestCategories.CRDs {
		result[i] = *e.manifestCategories.CRDs[i].DeepCopy()
	}

	return result
}

// GetWebhookConfigs returns a defensive copy of the webhook configurations loaded from manifests.
// DeepCopy is used to prevent callers from accidentally mutating internal state.
func (e *K3sEnv) GetWebhookConfigs() []unstructured.Unstructured {
	if len(e.manifestCategories.WebhookConfigs) == 0 {
		return nil
	}

	result := make([]unstructured.Unstructured, len(e.manifestCategories.WebhookConfigs))
	for i := range e.manifestCategories.WebhookConfigs {
		result[i] = *e.manifestCategories.WebhookConfigs[i].DeepCopy()
	}

	return result
}

// GetWebhookHost returns the webhook server host:port string for accessing the webhook server from within the container.
// It determines the Docker gateway IP by inspecting the container's network settings and caches the result.
// The returned value is in the format "host:port" (e.g., "172.17.0.1:9443").
// Returns an error if the container is not initialized or if the gateway IP cannot be determined.
func (e *K3sEnv) GetWebhookHost(ctx context.Context) (string, error) {
	if e.webhookHost != "" {
		return e.webhookHost, nil
	}

	if e.container == nil {
		return "", errors.New("container not initialized")
	}

	inspect, err := e.container.Inspect(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	for _, network := range inspect.NetworkSettings.Networks {
		if network.Gateway != "" {
			e.webhookHost = net.JoinHostPort(network.Gateway, strconv.Itoa(e.webhookPort))
			return e.webhookHost, nil
		}
	}

	return "", errors.New("no gateway IP found in container network settings")
}

func (e *K3sEnv) WebhookServer() ctrlwebhook.Server {
	return ctrlwebhook.NewServer(ctrlwebhook.Options{
		Port:     e.webhookPort,
		Host:     DefaultWebhookServerHost,
		CertDir:  e.certDir,
		CertName: CertFileName,
		KeyName:  KeyFileName,
		TLSOpts: []func(*tls.Config){
			func(config *tls.Config) {
				config.MinVersion = tls.VersionTLS12
			},
		},
	})
}

func (e *K3sEnv) InstallWebhooks(ctx context.Context) error {
	webhookHostPort, err := e.GetWebhookHost(ctx)
	if err != nil {
		return fmt.Errorf("failed to get webhook host: %w", err)
	}

	// 1. Patch webhook configurations
	if err := e.patchWebhookConfigurations(webhookHostPort); err != nil {
		return fmt.Errorf("failed to patch webhook configurations: %w", err)
	}

	// 2. Apply webhook configurations
	for i := range e.manifestCategories.WebhookConfigs {
		wh := &e.manifestCategories.WebhookConfigs[i]
		if err := e.cli.Create(ctx, wh); err != nil {
			return fmt.Errorf("failed to create webhook config %s: %w", wh.GetName(), err)
		}
	}

	// 3. Determine convertible CRDs and patch them with conversion webhooks
	convertibleCRDs, err := determineConvertibleCRDs(e.manifestCategories.CRDs, e.scheme)
	if err != nil {
		return fmt.Errorf("failed to determine convertible CRDs: %w", err)
	}

	if len(convertibleCRDs) > 0 {
		if err := e.patchAndUpdateCRDConversions(ctx, convertibleCRDs, webhookHostPort); err != nil {
			return fmt.Errorf("failed to patch and update CRD conversions: %w", err)
		}

		// 3.5. Wait for patched CRDs to be re-established
		if err := e.waitForCRDsEstablished(ctx, extractNames(convertibleCRDs)); err != nil {
			return fmt.Errorf("failed waiting for converted CRDs to be re-established: %w", err)
		}
	}

	// 4. Wait for webhooks to be ready
	return e.waitForWebhooksReady(ctx, webhookHostPort)
}

func (e *K3sEnv) renderKustomizations(paths []string) error {
	engine := kustomize.NewEngine()

	for _, path := range paths {
		// If path is relative, prepend project root
		resolvedPath := path
		if !filepath.IsAbs(path) {
			projectRoot, err := envtestutil.FindProjectRoot()
			if err != nil {
				return fmt.Errorf("failed to find project root for relative path %s: %w", path, err)
			}
			resolvedPath = filepath.Join(projectRoot, path)
		}

		// Validate that the resolved path exists
		if _, err := os.Stat(resolvedPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("kustomization path does not exist: %s", resolvedPath)
			}
			return fmt.Errorf("failed to access kustomization path %s: %w", resolvedPath, err)
		}

		manifests, err := engine.Render(resolvedPath)
		if err != nil {
			return fmt.Errorf("failed to render kustomization at %s: %w", resolvedPath, err)
		}

		// Categorize and append manifests directly
		for i := range manifests {
			switch manifests[i].GroupVersionKind() {
			case gvk.CustomResourceDefinition:
				e.manifestCategories.CRDs = append(e.manifestCategories.CRDs, manifests[i])
			case gvk.MutatingWebhookConfiguration:
				e.manifestCategories.WebhookConfigs = append(e.manifestCategories.WebhookConfigs, manifests[i])
			case gvk.ValidatingWebhookConfiguration:
				e.manifestCategories.WebhookConfigs = append(e.manifestCategories.WebhookConfigs, manifests[i])
			}
		}
	}

	return nil
}

// loadObjectsToManifests loads client objects into the appropriate manifest categories.
// Objects that are not CRDs or webhook configurations are silently ignored.
// This allows for injecting test objects (e.g., mock CRDs for fault injection) without
// requiring additional kustomization files.
func (e *K3sEnv) loadObjectsToManifests(objects []client.Object) error {
	for _, obj := range objects {
		// Ensure the object has GroupVersionKind set using the scheme
		if err := resources.EnsureGroupVersionKind(e.scheme, obj); err != nil {
			return fmt.Errorf("failed to ensure GVK for object %T: %w", obj, err)
		}

		u, err := resources.ToUnstructured(obj)
		if err != nil {
			return fmt.Errorf("failed to convert object to unstructured: %w", err)
		}

		// Add to appropriate category
		switch u.GroupVersionKind() {
		case gvk.CustomResourceDefinition:
			e.manifestCategories.CRDs = append(e.manifestCategories.CRDs, *u)
		case gvk.MutatingWebhookConfiguration:
			e.manifestCategories.WebhookConfigs = append(e.manifestCategories.WebhookConfigs, *u)
		case gvk.ValidatingWebhookConfiguration:
			e.manifestCategories.WebhookConfigs = append(e.manifestCategories.WebhookConfigs, *u)
		}
	}
	return nil
}

func (e *K3sEnv) patchAndUpdateCRDConversions(
	ctx context.Context,
	convertibleCRDs []unstructured.Unstructured,
	hostPort string,
) error {
	baseURL := fmt.Sprintf("%s://%s", WebhookURLScheme, hostPort)
	caBundle := string(e.certData.CABundle())

	for i := range convertibleCRDs {
		crd := convertibleCRDs[i].DeepCopy()

		if err := e.cli.Get(ctx, client.ObjectKeyFromObject(crd), crd); err != nil {
			return fmt.Errorf("failed to get CRD %s: %w", crd.GetName(), err)
		}

		// Apply conversion webhook configuration using JQ
		err := ApplyJQTransform(
			crd, `
			.spec.conversion = {
				"strategy": "Webhook",
				"webhook": {
					"conversionReviewVersions": ["v1"],
					"clientConfig": {
						"url": "%s",
						"caBundle": "%s"
					}
				}
			}
		`, baseURL+WebhookConvertPath, caBundle)

		if err != nil {
			return fmt.Errorf("failed to patch CRD %s: %w", crd.GetName(), err)
		}

		// Update the CRD
		if err := e.cli.Update(ctx, crd); err != nil {
			return fmt.Errorf("failed to update CRD %s with conversion: %w", crd.GetName(), err)
		}
	}

	return nil
}

func (e *K3sEnv) installCRDs(ctx context.Context) error {
	for i := range e.manifestCategories.CRDs {
		crd := e.manifestCategories.CRDs[i].DeepCopy()

		if err := e.cli.Create(ctx, crd); err != nil {
			return fmt.Errorf("failed to create CRD %s: %w",
				resources.FormatObjectReference(crd),
				err,
			)
		}
	}

	return nil
}

func (e *K3sEnv) waitForCRDsEstablished(ctx context.Context, crdNames []string) error {
	for _, crdName := range crdNames {
		err := wait.PollUntilContextCancel(ctx, CRDEstablishmentPollInterval, true, func(ctx context.Context) (bool, error) {
			obj := &apiextensionsv1.CustomResourceDefinition{}
			err := e.cli.Get(ctx, client.ObjectKey{Name: crdName}, obj)
			switch {
			case k8serr.IsNotFound(err):
				return false, nil
			case err != nil:
				return false, err
			}

			// Check conditions using typed API
			for _, condition := range obj.Status.Conditions {
				if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
					// CRD is established
					return true, nil
				}
			}

			// Not established yet, retry
			return false, nil
		})

		if err != nil {
			return fmt.Errorf("CRD %s not established: %w", crdName, err)
		}
	}

	return nil
}

func (e *K3sEnv) CreateCRD(ctx context.Context, crd client.Object) error {
	// Create the CRD (idempotent - ignore AlreadyExists errors)
	if err := e.cli.Create(ctx, crd); err != nil && !k8serr.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create CRD %s: %w",
			resources.FormatObjectReference(crd),
			err,
		)
	}

	return e.waitForCRDsEstablished(ctx, []string{crd.GetName()})
}

func (e *K3sEnv) patchWebhookConfigurations(hostPort string) error {
	baseURL := fmt.Sprintf("%s://%s", WebhookURLScheme, hostPort)
	caBundle := string(e.certData.CABundle())

	for i := range e.manifestCategories.WebhookConfigs {
		wh := &e.manifestCategories.WebhookConfigs[i]

		err := ApplyJQTransform(wh, `
			.webhooks |= map(
				.clientConfig.url = "%s" + (.clientConfig.service.path // "/") |
				.clientConfig.caBundle = "%s" |
				del(.clientConfig.service)
			)
		`, baseURL, caBundle)

		if err != nil {
			return fmt.Errorf("failed to patch webhook %s: %w", wh.GetName(), err)
		}
	}

	return nil
}

func (e *K3sEnv) waitForWebhooksReady(ctx context.Context, hostPort string) error {
	return wait.PollUntilContextTimeout(ctx, WebhookReadyPollInterval, WebhookReadyTimeout, true, func(ctx context.Context) (bool, error) {
		err := checkWebhookHealth(ctx, hostPort)
		// Continue polling if webhook is not ready yet
		return err == nil, nil
	})
}
