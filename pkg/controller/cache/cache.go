package cache

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

var (
	DefaultTransformFn = func(in any) (any, error) {
		if obj, err := meta.Accessor(in); err == nil && obj.GetManagedFields() != nil {
			obj.SetManagedFields(nil)
		}

		// Handle specific types that need additional transformations
		if obj, ok := in.(*apiextensionsv1.CustomResourceDefinition); ok {
			obj.Spec = apiextensionsv1.CustomResourceDefinitionSpec{}
		}

		return in, nil
	}
)
