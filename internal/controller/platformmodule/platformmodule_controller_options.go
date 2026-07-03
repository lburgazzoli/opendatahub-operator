//nolint:ireturn
package platformmodule

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	sr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/services/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
)

// Option is satisfied by both the Options struct literal and the named
// constructor functions (WithRegistry, WithProvisionRegistry, WithTracker).
// Pass any combination to New().
type Option interface {
	applyOption(o *Options)
}

// Options configures the PlatformModule reconciler. Fields left nil default to
// the package-level singletons inside New(), so production callers can write
// Options{} and tests override only what they need.
type Options struct {
	Registry          *modules.Registry
	ServiceRegistry   *sr.Registry
	ProvisionReg      *provision.UnifiedRegistry
	Tracker           *provision.RunlevelTracker
	DeletePropagation metav1.DeletionPropagation
}

func (o Options) applyOption(target *Options) {
	if o.Registry != nil {
		target.Registry = o.Registry
	}
	if o.ServiceRegistry != nil {
		target.ServiceRegistry = o.ServiceRegistry
	}
	if o.ProvisionReg != nil {
		target.ProvisionReg = o.ProvisionReg
	}
	if o.Tracker != nil {
		target.Tracker = o.Tracker
	}
	if o.DeletePropagation != "" {
		target.DeletePropagation = o.DeletePropagation
	}
}

type optionFunc func(*Options)

func (f optionFunc) applyOption(o *Options) { f(o) }

// WithRegistry sets a custom module handler registry.
func WithRegistry(r *modules.Registry) Option {
	return optionFunc(func(o *Options) {
		o.Registry = r
	})
}

// WithServiceRegistry sets a custom service handler registry.
func WithServiceRegistry(r *sr.Registry) Option {
	return optionFunc(func(o *Options) {
		o.ServiceRegistry = r
	})
}

// WithProvisionRegistry sets a custom unified provision registry.
func WithProvisionRegistry(r *provision.UnifiedRegistry) Option {
	return optionFunc(func(o *Options) {
		o.ProvisionReg = r
	})
}

// WithTracker sets a custom RunlevelTracker.
func WithTracker(t *provision.RunlevelTracker) Option {
	return optionFunc(func(o *Options) {
		o.Tracker = t
	})
}

// WithDeletePropagationPolicy sets the propagation policy used when deleting
// stale module operator resources during drift cleanup. Defaults to Foreground.
// Pass Background in tests (envtest has no GC controller).
func WithDeletePropagationPolicy(p metav1.DeletionPropagation) Option {
	return optionFunc(func(o *Options) {
		o.DeletePropagation = p
	})
}
