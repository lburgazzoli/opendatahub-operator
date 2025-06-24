package cleanup_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/rs/xid"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/cleanup"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"

	. "github.com/onsi/gomega"
)

func TestActionOptions_ApplyToAction_Selector(t *testing.T) {
	g := NewWithT(t)

	selector := labels.SelectorFromSet(map[string]string{"app": "test"})
	source := cleanup.ActionOptions{Selector: selector}
	target := cleanup.ActionOptions{}

	source.ApplyToAction(&target)

	g.Expect(target.Selector).To(Equal(selector))
}

func TestActionOptions_ApplyToAction_SelectorNil(t *testing.T) {
	g := NewWithT(t)

	existingSelector := labels.SelectorFromSet(map[string]string{"keep": "existing"})
	source := cleanup.ActionOptions{Selector: nil}
	target := cleanup.ActionOptions{Selector: existingSelector}

	source.ApplyToAction(&target)

	g.Expect(target.Selector).To(Equal(existingSelector))
}

func TestActionOptions_ApplyToAction_ProtectedTypes(t *testing.T) {
	g := NewWithT(t)

	source := cleanup.ActionOptions{
		ProtectedTypes: []schema.GroupVersionKind{gvk.ConfigMap, gvk.Secret},
	}
	target := cleanup.ActionOptions{}

	source.ApplyToAction(&target)

	g.Expect(target.ProtectedTypes).To(Equal([]schema.GroupVersionKind{gvk.ConfigMap, gvk.Secret}))
}

func TestActionOptions_ApplyToAction_ProtectedTypesAppend(t *testing.T) {
	g := NewWithT(t)

	source := cleanup.ActionOptions{
		ProtectedTypes: []schema.GroupVersionKind{gvk.ConfigMap},
	}
	target := cleanup.ActionOptions{
		ProtectedTypes: []schema.GroupVersionKind{gvk.Secret},
	}

	source.ApplyToAction(&target)

	g.Expect(target.ProtectedTypes).To(Equal([]schema.GroupVersionKind{gvk.Secret, gvk.ConfigMap}))
}

func TestActionOptions_ApplyToAction_TypePredicate(t *testing.T) {
	g := NewWithT(t)

	predicate := func(*types.ReconciliationRequest, schema.GroupVersionKind) (bool, error) {
		return true, nil
	}
	source := cleanup.ActionOptions{TypePredicate: predicate}
	target := cleanup.ActionOptions{}

	source.ApplyToAction(&target)

	g.Expect(target.TypePredicate).ToNot(BeNil())
	result, err := target.TypePredicate(nil, gvk.ConfigMap)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(BeTrue())
}

func TestActionOptions_ApplyToAction_TypePredicateNil(t *testing.T) {
	g := NewWithT(t)

	existingPredicate := func(*types.ReconciliationRequest, schema.GroupVersionKind) (bool, error) {
		return false, nil
	}
	source := cleanup.ActionOptions{TypePredicate: nil}
	target := cleanup.ActionOptions{TypePredicate: existingPredicate}

	source.ApplyToAction(&target)

	g.Expect(target.TypePredicate).ToNot(BeNil())
	result, err := target.TypePredicate(nil, gvk.ConfigMap)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(BeFalse())
}

func TestActionOptions_ApplyToAction_Handler(t *testing.T) {
	g := NewWithT(t)

	handler := func(context.Context, *types.ReconciliationRequest, unstructured.Unstructured) (bool, error) {
		return true, nil
	}
	source := cleanup.ActionOptions{Handler: handler}
	target := cleanup.ActionOptions{}

	source.ApplyToAction(&target)

	g.Expect(target.Handler).ToNot(BeNil())
	result, err := target.Handler(t.Context(), nil, unstructured.Unstructured{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(BeTrue())
}

func TestActionOptions_ApplyToAction_HandlerNil(t *testing.T) {
	g := NewWithT(t)

	existingHandler := func(context.Context, *types.ReconciliationRequest, unstructured.Unstructured) (bool, error) {
		return false, nil
	}
	source := cleanup.ActionOptions{Handler: nil}
	target := cleanup.ActionOptions{Handler: existingHandler}

	source.ApplyToAction(&target)

	g.Expect(target.Handler).ToNot(BeNil())
	result, err := target.Handler(t.Context(), nil, unstructured.Unstructured{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(BeFalse())
}

func TestActionOptions_ApplyToAction_NamespaceFunc(t *testing.T) {
	g := NewWithT(t)

	ns := xid.New().String()

	namespaceFunc := func(context.Context, *types.ReconciliationRequest) (string, error) {
		return ns, nil
	}
	source := cleanup.ActionOptions{NamespaceFunc: namespaceFunc}
	target := cleanup.ActionOptions{}

	source.ApplyToAction(&target)

	g.Expect(target.NamespaceFunc).ToNot(BeNil())
	result, err := target.NamespaceFunc(t.Context(), nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(ns))
}

func TestActionOptions_ApplyToAction_NamespaceFuncNil(t *testing.T) {
	g := NewWithT(t)

	ns := xid.New().String()

	existingFunc := func(context.Context, *types.ReconciliationRequest) (string, error) {
		return ns, nil
	}
	source := cleanup.ActionOptions{NamespaceFunc: nil}
	target := cleanup.ActionOptions{NamespaceFunc: existingFunc}

	source.ApplyToAction(&target)

	g.Expect(target.NamespaceFunc).ToNot(BeNil())
	result, err := target.NamespaceFunc(t.Context(), nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(ns))
}

func TestActionOptions_ApplyToAction_AllFields(t *testing.T) {
	g := NewWithT(t)

	ns := xid.New().String()

	// Store function references in variables with correct types
	typePredicate := cleanup.TypePredicateFn(func(*types.ReconciliationRequest, schema.GroupVersionKind) (bool, error) {
		return true, nil
	})
	handler := func(context.Context, *types.ReconciliationRequest, unstructured.Unstructured) (bool, error) {
		return true, nil
	}
	namespaceFunc := func(context.Context, *types.ReconciliationRequest) (string, error) {
		return ns, nil
	}

	source := cleanup.ActionOptions{
		Selector: labels.SelectorFromSet(map[string]string{"app": "test"}),
		ProtectedTypes: []schema.GroupVersionKind{
			gvk.ConfigMap,
			gvk.Secret,
		},
		TypePredicate: typePredicate,
		Handler:       handler,
		NamespaceFunc: namespaceFunc,
	}
	target := cleanup.ActionOptions{}

	source.ApplyToAction(&target)

	g.Expect(target.Selector).Should(Equal(source.Selector))
	g.Expect(target.ProtectedTypes).Should(Equal(source.ProtectedTypes))
	g.Expect(reflect.ValueOf(target.TypePredicate).Pointer()).Should(Equal(reflect.ValueOf(source.TypePredicate).Pointer()))
	g.Expect(reflect.ValueOf(target.Handler).Pointer()).Should(Equal(reflect.ValueOf(source.Handler).Pointer()))
	g.Expect(reflect.ValueOf(target.NamespaceFunc).Pointer()).Should(Equal(reflect.ValueOf(source.NamespaceFunc).Pointer()))

	// Test function behavior to ensure they work correctly
	predicateResult, err := target.TypePredicate(nil, gvk.ConfigMap)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(predicateResult).To(BeTrue())

	handlerResult, err := target.Handler(t.Context(), nil, unstructured.Unstructured{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(handlerResult).To(BeTrue())

	namespaceResult, err := target.NamespaceFunc(t.Context(), nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(namespaceResult).To(Equal(ns))
}

func TestWithSelector(t *testing.T) {
	g := NewWithT(t)

	selector := labels.SelectorFromSet(map[string]string{"app": "test"})
	option := cleanup.WithSelector(selector)

	config := cleanup.ActionOptions{}
	option.ApplyToAction(&config)

	g.Expect(config.Selector).To(Equal(selector))
}

func TestWithLabels(t *testing.T) {
	g := NewWithT(t)

	values := map[string]string{"app": "test", "version": "v1"}
	option := cleanup.WithLabels(values)

	config := cleanup.ActionOptions{}
	option.ApplyToAction(&config)

	expectedSelector := labels.SelectorFromSet(values)
	g.Expect(config.Selector).To(Equal(expectedSelector))
}

func TestWithProtectedTypes(t *testing.T) {
	tests := []struct {
		name          string
		firstCall     []schema.GroupVersionKind
		secondCall    []schema.GroupVersionKind
		expectedTypes []schema.GroupVersionKind
	}{
		{
			name:          "should add single type",
			firstCall:     []schema.GroupVersionKind{gvk.ConfigMap},
			expectedTypes: []schema.GroupVersionKind{gvk.ConfigMap},
		},
		{
			name:          "should add multiple types",
			firstCall:     []schema.GroupVersionKind{gvk.ConfigMap, gvk.Secret},
			expectedTypes: []schema.GroupVersionKind{gvk.ConfigMap, gvk.Secret},
		},
		{
			name:       "should be incremental - multiple calls",
			firstCall:  []schema.GroupVersionKind{gvk.ConfigMap},
			secondCall: []schema.GroupVersionKind{gvk.Secret, gvk.Pod},
			expectedTypes: []schema.GroupVersionKind{
				gvk.ConfigMap,
				gvk.Secret,
				gvk.Pod,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			config := cleanup.ActionOptions{}

			// Apply first option
			firstOption := cleanup.WithProtectedTypes(tt.firstCall...)
			firstOption.ApplyToAction(&config)

			// Apply second option if provided
			if tt.secondCall != nil {
				secondOption := cleanup.WithProtectedTypes(tt.secondCall...)
				secondOption.ApplyToAction(&config)
			}

			g.Expect(config.ProtectedTypes).To(Equal(tt.expectedTypes))
		})
	}
}

func TestWithTypePredicate(t *testing.T) {
	g := NewWithT(t)

	predicate := func(*types.ReconciliationRequest, schema.GroupVersionKind) (bool, error) {
		return true, nil
	}
	option := cleanup.WithTypePredicate(predicate)

	config := cleanup.ActionOptions{}
	option.ApplyToAction(&config)

	g.Expect(config.TypePredicate).ToNot(BeNil())

	// Test that the predicate works
	result, err := config.TypePredicate(nil, gvk.ConfigMap)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(BeTrue())
}

func TestWithHandler(t *testing.T) {
	g := NewWithT(t)

	handler := func(context.Context, *types.ReconciliationRequest, unstructured.Unstructured) (bool, error) {
		return true, nil
	}
	option := cleanup.WithHandler(handler)

	config := cleanup.ActionOptions{}
	option.ApplyToAction(&config)

	g.Expect(config.Handler).ToNot(BeNil())

	// Test that the handler works
	result, err := config.Handler(t.Context(), nil, unstructured.Unstructured{})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(BeTrue())
}

func TestInNamespace(t *testing.T) {
	g := NewWithT(t)

	namespace := "test-namespace"
	option := cleanup.InNamespace(namespace)

	config := cleanup.ActionOptions{}
	option.ApplyToAction(&config)

	g.Expect(config.NamespaceFunc).ToNot(BeNil())

	// Test that the namespace function works
	result, err := config.NamespaceFunc(t.Context(), nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(namespace))
}

func TestInNamespaceFn(t *testing.T) {
	g := NewWithT(t)

	expectedNamespace := "dynamic-namespace"
	namespaceFn := func(context.Context, *types.ReconciliationRequest) (string, error) {
		return expectedNamespace, nil
	}
	option := cleanup.InNamespaceFn(namespaceFn)

	config := cleanup.ActionOptions{}
	option.ApplyToAction(&config)

	g.Expect(config.NamespaceFunc).ToNot(BeNil())

	// Test that the namespace function works
	result, err := config.NamespaceFunc(t.Context(), nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(Equal(expectedNamespace))
}

func TestActionOptionsInterface(t *testing.T) {
	g := NewWithT(t)

	// Test that ActionOptions implements ActionOption interface
	var option cleanup.ActionOption = &cleanup.ActionOptions{}
	g.Expect(option).ToNot(BeNil())

	// Test that it can be used as an option
	config := cleanup.ActionOptions{
		Selector: labels.SelectorFromSet(map[string]string{"test": "value"}),
	}

	var target cleanup.ActionOptions
	config.ApplyToAction(&target)

	g.Expect(target.Selector).To(Equal(config.Selector))
}

func TestOptionChaining(t *testing.T) {
	g := NewWithT(t)

	// Test that multiple options can be chained together
	config := cleanup.ActionOptions{}

	// Apply multiple options
	cleanup.WithLabels(map[string]string{"app": "test"}).ApplyToAction(&config)
	cleanup.WithProtectedTypes(gvk.ConfigMap).ApplyToAction(&config)
	cleanup.WithProtectedTypes(gvk.Secret).ApplyToAction(&config)
	cleanup.InNamespace("test-ns").ApplyToAction(&config)

	// Verify all options were applied
	g.Expect(config.Selector).To(Equal(labels.SelectorFromSet(map[string]string{"app": "test"})))
	g.Expect(config.ProtectedTypes).To(ContainElements(gvk.ConfigMap, gvk.Secret))
	g.Expect(config.NamespaceFunc).ToNot(BeNil())

	// Test namespace function
	ns, err := config.NamespaceFunc(t.Context(), nil)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(ns).To(Equal("test-ns"))
}

func TestOptionTypes(t *testing.T) {
	g := NewWithT(t)

	// Store function references in variables with correct types
	typePredicate := cleanup.TypePredicateFn(func(*types.ReconciliationRequest, schema.GroupVersionKind) (bool, error) {
		return true, nil
	})
	handler := func(context.Context, *types.ReconciliationRequest, unstructured.Unstructured) (bool, error) {
		return true, nil
	}

	// Test that all option types implement ActionOption interface
	options := []cleanup.ActionOption{
		cleanup.WithSelector(labels.Everything()),
		cleanup.WithLabels(map[string]string{"test": "value"}),
		cleanup.WithProtectedTypes(gvk.Pod),
		cleanup.WithTypePredicate(typePredicate),
		cleanup.WithHandler(handler),
		cleanup.InNamespace("test"),
		cleanup.InNamespaceFn(actions.OperatorNamespace),
	}

	// Verify all options can be applied
	config := cleanup.ActionOptions{}
	for _, option := range options {
		g.Expect(option).ToNot(BeNil())
		option.ApplyToAction(&config)
	}

	g.Expect(config.Selector).Should(Not(BeNil()))
	g.Expect(config.ProtectedTypes).Should(HaveLen(1))
	g.Expect(reflect.ValueOf(config.TypePredicate).Pointer()).Should(Equal(reflect.ValueOf(typePredicate).Pointer()))
	g.Expect(reflect.ValueOf(config.Handler).Pointer()).Should(Equal(reflect.ValueOf(handler).Pointer()))
	g.Expect(reflect.ValueOf(config.NamespaceFunc).Pointer()).Should(Equal(reflect.ValueOf(actions.OperatorNamespace).Pointer()))
}
