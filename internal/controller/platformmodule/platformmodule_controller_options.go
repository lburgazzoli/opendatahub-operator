package platformmodule

import (
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
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
	Registry     *modules.Registry
	ProvisionReg *provision.UnifiedRegistry
	Tracker      *provision.RunlevelTracker
}

func (o Options) applyOption(target *Options) {
	if o.Registry != nil {
		target.Registry = o.Registry
	}
	if o.ProvisionReg != nil {
		target.ProvisionReg = o.ProvisionReg
	}
	if o.Tracker != nil {
		target.Tracker = o.Tracker
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
