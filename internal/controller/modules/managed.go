package modules

import (
	"reflect"

	"k8s.io/apimachinery/pkg/util/sets"
)

const moduleTagKey = "module"

// ManagedModuleNames returns the set of module names declared in spec via
// module:"name" struct field tags. This allows DSC and DSCI controllers to
// discover which modules they manage by reflecting on their own spec structs,
// without hardcoding names or tagging modules with controller knowledge.
//
// Each field tagged module:"name" contributes one entry. Untagged fields and
// non-struct fields are ignored. Pass the spec struct (or a pointer to it)
// directly, e.g. ManagedModuleNames(dsc.Spec.Components).
func ManagedModuleNames(spec any) sets.Set[string] {
	result := sets.New[string]()

	t := reflect.TypeOf(spec)
	if t == nil {
		return result
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return result
	}

	for i := range t.NumField() {
		if name := t.Field(i).Tag.Get(moduleTagKey); name != "" {
			result.Insert(name)
		}
	}

	return result
}
