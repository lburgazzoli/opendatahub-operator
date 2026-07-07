package registry

import (
	"context"

	"github.com/hashicorp/go-multierror"
	operatorv1 "github.com/openshift/api/operator/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/controller/base"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/provision"
)

// ServiceHandler is an interface to manage a service
// Every method should accept ctx since it contains the logger.
type ServiceHandler interface {
	base.Handler
	Init(platform common.Platform) error
	// GetGroupVersionKind returns the GroupVersionKind of the service CR managed
	// by this handler. Returns an empty GVK for services that have no dedicated
	// CR. Used by the Platform controller to register watches dynamically.
	GetManagementState(platform common.Platform, dsci *dsciv2.DSCInitialization) operatorv1.ManagementState
	NewReconciler(ctx context.Context, mgr ctrl.Manager, tracker *provision.RunlevelTracker) error
}

type handlerEntry struct {
	handler ServiceHandler
	enabled bool
}

// Registry is a struct that maintains a set of registered ServiceHandlers.
type Registry struct {
	entries map[string]handlerEntry
}

var _ base.Registry[ServiceHandler] = (*Registry)(nil)

var r = &Registry{}

// Add registers a new ServiceHandler to the registry.
// not thread safe, supposed to be called during program initialization.
func (r *Registry) Add(ch ServiceHandler) {
	if r.entries == nil {
		r.entries = make(map[string]handlerEntry)
	}
	r.entries[ch.GetName()] = handlerEntry{handler: ch, enabled: true}
}

// Enable sets the enabled state for the named handler to true.
func (r *Registry) Enable(name string) {
	r.setEnabled(name, true)
}

// Disable sets the enabled state for the named handler to false.
func (r *Registry) Disable(name string) {
	r.setEnabled(name, false)
}

// setEnabled sets the enabled state for the named handler.
func (r *Registry) setEnabled(name string, enabled bool) {
	if e, ok := r.entries[name]; ok {
		e.enabled = enabled
		r.entries[name] = e
	}
}

// IsEnabled returns the internal enabled state for the named handler.
func (r *Registry) IsEnabled(name string) bool {
	e, ok := r.entries[name]
	return ok && e.enabled
}

// Lookup returns the handler for a named service, or nil if not found.
//
//nolint:ireturn // Registry API intentionally returns the handler interface.
func (r *Registry) Lookup(name string) ServiceHandler {
	if e, ok := r.entries[name]; ok {
		return e.handler
	}
	return nil
}

// ForEach iterates over all registered ServiceHandlers and applies the given function.
// Handlers whose enabled flag is false are skipped.
// If any handler returns an error, that error is collected and returned at the end.
// With go1.23 probably https://go.dev/blog/range-functions can be used.
func (r *Registry) ForEach(f func(ch ServiceHandler) error) error {
	var errs *multierror.Error
	for _, e := range r.entries {
		if !e.enabled {
			continue
		}
		errs = multierror.Append(errs, f(e.handler))
	}

	return errs.ErrorOrNil()
}

// ForAll iterates over every registered service regardless of enabled state.
func (r *Registry) ForAll(f func(handler ServiceHandler, registryEnabled bool) error) error {
	var errs *multierror.Error
	for _, e := range r.entries {
		errs = multierror.Append(errs, f(e.handler, e.enabled))
	}

	return errs.ErrorOrNil()
}

func Add(ch ServiceHandler) {
	r.Add(ch)
}

func Enable(name string) {
	r.setEnabled(name, true)
}

func Disable(name string) {
	r.setEnabled(name, false)
}

func IsEnabled(name string) bool {
	return r.IsEnabled(name)
}

//nolint:ireturn // Registry API intentionally returns the handler interface.
func Lookup(name string) ServiceHandler {
	return r.Lookup(name)
}

func ForEach(f func(ch ServiceHandler) error) error {
	return r.ForEach(f)
}

func ForAll(f func(handler ServiceHandler, registryEnabled bool) error) error {
	return r.ForAll(f)
}

func DefaultRegistry() *Registry {
	return r
}
