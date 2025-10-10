package k3senv

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Option is configuration that modifies options for k3senv creation.
type Option interface {
	ApplyToOptions(opts *Options)
}

// Options contains configuration options for creating a K3sEnv.
type Options struct {
	Scheme              *runtime.Scheme
	KustomizationPaths  []string
	Objects             []client.Object
	CertDir             string
	AutoInstallWebhooks bool
	WebhookPort         int
	K3sImage            string
	CertValidity        time.Duration
}

// ApplyOptions applies the given options on these options,
// and then returns itself (for convenient chaining).
func (o *Options) ApplyOptions(opts []Option) *Options {
	for _, opt := range opts {
		opt.ApplyToOptions(o)
	}
	return o
}

// ApplyToOptions implements Option interface.
func (o *Options) ApplyToOptions(target *Options) {
	if o.Scheme != nil {
		target.Scheme = o.Scheme
	}
	if len(o.KustomizationPaths) > 0 {
		target.KustomizationPaths = append(target.KustomizationPaths, o.KustomizationPaths...)
	}
	if len(o.Objects) > 0 {
		target.Objects = append(target.Objects, o.Objects...)
	}
	if o.CertDir != "" {
		target.CertDir = o.CertDir
	}
	if o.AutoInstallWebhooks {
		target.AutoInstallWebhooks = o.AutoInstallWebhooks
	}
	if o.WebhookPort != 0 {
		target.WebhookPort = o.WebhookPort
	}
	if o.K3sImage != "" {
		target.K3sImage = o.K3sImage
	}
	if o.CertValidity != 0 {
		target.CertValidity = o.CertValidity
	}
}

var _ Option = &Options{}

// Scheme sets the runtime scheme for the K3sEnv.
type Scheme struct {
	scheme *runtime.Scheme
}

// WithScheme creates an option that sets the runtime scheme.
func WithScheme(s *runtime.Scheme) Option {
	return &Scheme{scheme: s}
}

func (s *Scheme) ApplyToOptions(o *Options) {
	o.Scheme = s.scheme
}

// Kustomization adds a kustomization path to render.
type Kustomization struct {
	path string
}

// WithKustomization creates an option that adds a kustomization path.
func WithKustomization(p string) Option {
	return &Kustomization{path: p}
}

func (k *Kustomization) ApplyToOptions(o *Options) {
	o.KustomizationPaths = append(o.KustomizationPaths, k.path)
}

// Kustomizations adds multiple kustomization paths to render.
type Kustomizations struct {
	paths []string
}

// WithKustomizations creates an option that adds multiple kustomization paths.
func WithKustomizations(paths ...string) Option {
	return &Kustomizations{paths: paths}
}

func (k *Kustomizations) ApplyToOptions(o *Options) {
	o.KustomizationPaths = append(o.KustomizationPaths, k.paths...)
}

// CertDir sets the directory for certificate storage.
type CertDir struct {
	dir string
}

// WithCertDir creates an option that sets the certificate directory.
func WithCertDir(dir string) Option {
	return &CertDir{dir: dir}
}

func (c *CertDir) ApplyToOptions(o *Options) {
	o.CertDir = c.dir
}

// Objects adds client objects to install.
type Objects struct {
	objects []client.Object
}

// WithObjects creates an option that adds client objects.
func WithObjects(objects ...client.Object) Option {
	return &Objects{objects: objects}
}

func (obj *Objects) ApplyToOptions(o *Options) {
	o.Objects = append(o.Objects, obj.objects...)
}

// AutoInstallWebhooks controls automatic webhook installation during Start().
type AutoInstallWebhooks struct {
	enable bool
}

// WithAutoInstallWebhooks creates an option that enables/disables automatic webhook installation.
func WithAutoInstallWebhooks(enable bool) Option {
	return &AutoInstallWebhooks{enable: enable}
}

func (a *AutoInstallWebhooks) ApplyToOptions(o *Options) {
	o.AutoInstallWebhooks = a.enable
}

// WebhookPort sets the port for the webhook server.
type WebhookPort struct {
	port int
}

// WithWebhookPort creates an option that sets the webhook server port.
func WithWebhookPort(port int) Option {
	return &WebhookPort{port: port}
}

func (w *WebhookPort) ApplyToOptions(o *Options) {
	o.WebhookPort = w.port
}

// K3sImage sets the k3s container image to use.
type K3sImage struct {
	image string
}

// WithK3sImage creates an option that sets the k3s container image.
func WithK3sImage(image string) Option {
	return &K3sImage{image: image}
}

func (k *K3sImage) ApplyToOptions(o *Options) {
	o.K3sImage = k.image
}

// CertValidity sets the certificate validity duration.
type CertValidity struct {
	duration time.Duration
}

// WithCertValidity creates an option that sets the certificate validity duration.
func WithCertValidity(duration time.Duration) Option {
	return &CertValidity{duration: duration}
}

func (c *CertValidity) ApplyToOptions(o *Options) {
	o.CertValidity = c.duration
}
