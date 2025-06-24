//nolint:ireturn
package cleanup

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	odhTypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

type ActionOption interface {
	ApplyToAction(opts *ActionOptions)
}

type ActionOptions struct {
	Selector       labels.Selector
	ProtectedTypes []schema.GroupVersionKind
	TypePredicate  TypePredicateFn
	Handler        HandlerFn
	NamespaceFunc  actions.StringGetter
}

func (o *ActionOptions) ApplyToAction(config *ActionOptions) {
	if o.Selector != nil {
		config.Selector = o.Selector
	}
	if o.ProtectedTypes != nil {
		config.ProtectedTypes = append(config.ProtectedTypes, o.ProtectedTypes...)
	}
	if o.TypePredicate != nil {
		config.TypePredicate = o.TypePredicate
	}
	if o.Handler != nil {
		config.Handler = o.Handler
	}
	if o.NamespaceFunc != nil {
		config.NamespaceFunc = o.NamespaceFunc
	}
}

type SelectorOption struct {
	selector labels.Selector
}

func (o SelectorOption) ApplyToAction(config *ActionOptions) {
	config.Selector = o.selector
}

type ProtectedTypesOption struct {
	types []schema.GroupVersionKind
}

func (o ProtectedTypesOption) ApplyToAction(config *ActionOptions) {
	config.ProtectedTypes = append(config.ProtectedTypes, o.types...)
}

type TypePredicateOption struct {
	predicate TypePredicateFn
}

func (o TypePredicateOption) ApplyToAction(config *ActionOptions) {
	config.TypePredicate = o.predicate
}

type HandlerOption struct {
	handler HandlerFn
}

func (o HandlerOption) ApplyToAction(config *ActionOptions) {
	config.Handler = o.handler
}

type NamespaceFuncOption struct {
	fn actions.StringGetter
}

func (o NamespaceFuncOption) ApplyToAction(config *ActionOptions) {
	config.NamespaceFunc = o.fn
}

func WithSelector(selector labels.Selector) ActionOption {
	return SelectorOption{selector: selector}
}

func WithLabels(values map[string]string) ActionOption {
	return WithSelector(labels.SelectorFromSet(values))
}

func WithProtectedTypes(types ...schema.GroupVersionKind) ActionOption {
	return ProtectedTypesOption{types: types}
}

func WithTypePredicate(predicate TypePredicateFn) ActionOption {
	return TypePredicateOption{predicate: predicate}
}

func WithHandler(handler HandlerFn) ActionOption {
	return HandlerOption{handler: handler}
}

func InNamespace(namespace string) ActionOption {
	return NamespaceFuncOption{
		fn: func(_ context.Context, _ *odhTypes.ReconciliationRequest) (string, error) {
			return namespace, nil
		},
	}
}

func InNamespaceFn(fn actions.StringGetter) ActionOption {
	return NamespaceFuncOption{fn: fn}
}
