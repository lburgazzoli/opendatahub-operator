package base

import "k8s.io/apimachinery/pkg/runtime/schema"

// Handler defines the shared minimal contract implemented by controller
// handlers that are tracked by name and primary operand GVK.
type Handler interface {
	GetName() string
	GetGroupVersionKind() schema.GroupVersionKind
}

// Registry defines the shared lookup/iteration contract exposed by handler
// registries. Ordering semantics remain owned by each concrete registry.
type Registry[H Handler] interface {
	Lookup(name string) H
	ForEach(fn func(handler H) error) error
	ForAll(fn func(handler H, registryEnabled bool) error) error
}
