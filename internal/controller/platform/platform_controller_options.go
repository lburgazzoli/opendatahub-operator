package platform

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	sr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/services/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/dag"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
)

// Option is satisfied by both the Options struct literal and the named
// constructor functions. Pass any combination to New().
type Option interface {
	applyOption(o *Options)
}

// Options configures the Platform reconciler. Fields left nil default to the
// package-level singletons inside New(), so production callers can write
// Options{} and tests override only what they need.
type Options struct {
	ModuleRegistry    *modules.Registry
	ComponentRegistry *cr.Registry
	ServiceRegistry   *sr.Registry
	StuckTracker      *dag.StuckTracker
	Tracker           *provision.RunlevelTracker
	DeletePropagation metav1.DeletionPropagation
	ProvisionReg      *provision.UnifiedRegistry
}

func (o Options) applyOption(target *Options) {
	if o.ModuleRegistry != nil {
		target.ModuleRegistry = o.ModuleRegistry
	}
	if o.ComponentRegistry != nil {
		target.ComponentRegistry = o.ComponentRegistry
	}
	if o.ServiceRegistry != nil {
		target.ServiceRegistry = o.ServiceRegistry
	}
	if o.StuckTracker != nil {
		target.StuckTracker = o.StuckTracker
	}
	if o.Tracker != nil {
		target.Tracker = o.Tracker
	}
	if o.DeletePropagation != "" {
		target.DeletePropagation = o.DeletePropagation
	}
	if o.ProvisionReg != nil {
		target.ProvisionReg = o.ProvisionReg
	}
}

type optionFunc func(*Options)

func (f optionFunc) applyOption(o *Options) { f(o) }

// WithModuleRegistry sets a custom module handler registry.
//
//nolint:ireturn // Option constructors intentionally return the public Option interface.
func WithModuleRegistry(r *modules.Registry) Option {
	return optionFunc(func(o *Options) {
		o.ModuleRegistry = r
	})
}

// WithComponentRegistry sets a custom component handler registry.
//
//nolint:ireturn // Option constructors intentionally return the public Option interface.
func WithComponentRegistry(r *cr.Registry) Option {
	return optionFunc(func(o *Options) {
		o.ComponentRegistry = r
	})
}

// WithServiceRegistry sets a custom service handler registry.
//
//nolint:ireturn // Option constructors intentionally return the public Option interface.
func WithServiceRegistry(r *sr.Registry) Option {
	return optionFunc(func(o *Options) {
		o.ServiceRegistry = r
	})
}

// WithStuckTracker sets a custom DAG stuck tracker. Useful in tests to get
// hermetic stuck-timeout behavior without sharing a package-level singleton.
//
//nolint:ireturn // Option constructors intentionally return the public Option interface.
func WithStuckTracker(t *dag.StuckTracker) Option {
	return optionFunc(func(o *Options) {
		o.StuckTracker = t
	})
}

// WithTracker sets a custom RunlevelTracker. Defaults to the shared singleton.
//
//nolint:ireturn // Option constructors intentionally return the public Option interface.
func WithTracker(t *provision.RunlevelTracker) Option {
	return optionFunc(func(o *Options) {
		o.Tracker = t
	})
}

// WithProvisionRegistry sets a custom unified provision registry for DAG
// ordering. Defaults to provision.DefaultRegistry(). Override in tests to
// inject isolated runlevel entries.
//
//nolint:ireturn // Option constructors intentionally return the public Option interface.
func WithProvisionRegistry(r *provision.UnifiedRegistry) Option {
	return optionFunc(func(o *Options) { o.ProvisionReg = r })
}

// WithDeletePropagationPolicy sets the propagation policy used when deleting
// disabled PlatformModule CRs. Defaults to Foreground.
// Pass Background in tests (envtest has no GC controller).
//
//nolint:ireturn // Option constructors intentionally return the public Option interface.
func WithDeletePropagationPolicy(p metav1.DeletionPropagation) Option {
	return optionFunc(func(o *Options) {
		o.DeletePropagation = p
	})
}
