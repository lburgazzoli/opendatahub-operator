package datasciencecluster

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cr "github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/components/registry"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/modules"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
)

// Option configures the DataScienceCluster reconciler.
type Option interface {
	applyOption(o *Options)
}

// Options holds the registries used by the DataScienceCluster reconciler.
// Fields default to the package-level singletons; override in tests via
// WithComponentRegistry / WithModuleRegistry.
type Options struct {
	ComponentRegistry *cr.Registry
	ModuleRegistry    *modules.Registry
	ProvisionRegistry *provision.UnifiedRegistry
	DeletePropagation metav1.DeletionPropagation
}

func (o Options) applyOption(target *Options) {
	if o.ComponentRegistry != nil {
		target.ComponentRegistry = o.ComponentRegistry
	}
	if o.ModuleRegistry != nil {
		target.ModuleRegistry = o.ModuleRegistry
	}
	if o.ProvisionRegistry != nil {
		target.ProvisionRegistry = o.ProvisionRegistry
	}
	if o.DeletePropagation != "" {
		target.DeletePropagation = o.DeletePropagation
	}
}

type optionFunc func(*Options)

func (f optionFunc) applyOption(o *Options) { f(o) }

// WithComponentRegistry sets a custom component handler registry.
func WithComponentRegistry(r *cr.Registry) Option { //nolint:ireturn // Public option constructors intentionally return the Option interface.
	return optionFunc(func(o *Options) { o.ComponentRegistry = r })
}

// WithModuleRegistry sets a custom module handler registry.
func WithModuleRegistry(r *modules.Registry) Option { //nolint:ireturn // Public option constructors intentionally return the Option interface.
	return optionFunc(func(o *Options) { o.ModuleRegistry = r })
}

// WithProvisionRegistry sets a custom unified DAG registry.
func WithProvisionRegistry(r *provision.UnifiedRegistry) Option { //nolint:ireturn // Public option constructors intentionally return the Option interface.
	return optionFunc(func(o *Options) { o.ProvisionRegistry = r })
}

// WithDeletePropagationPolicy sets the propagation policy used when deleting
// disabled component and module CRs. Defaults to Foreground.
// Pass Background in tests (envtest has no GC controller).
//
//nolint:ireturn // Public option constructors intentionally return the Option interface.
func WithDeletePropagationPolicy(p metav1.DeletionPropagation) Option {
	return optionFunc(func(o *Options) { o.DeletePropagation = p })
}
