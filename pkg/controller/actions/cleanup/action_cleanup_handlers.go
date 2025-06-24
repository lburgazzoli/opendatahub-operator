package cleanup

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	odhTypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

type ObjectPredicateFn func(*odhTypes.ReconciliationRequest, unstructured.Unstructured) (bool, error)

type ManagedAnnotationOption struct {
	enabled bool
}

func (o ManagedAnnotationOption) ApplyToDeleteHandler(config *DeleteHandlerOptions) {
	config.CheckManagedAnnotation = &o.enabled
}

func (o ManagedAnnotationOption) ApplyToDeOwnHandler(config *DeOwnHandlerOptions) {
	config.CheckManagedAnnotation = &o.enabled
}

type ObjectPredicateOption struct {
	predicate ObjectPredicateFn
}

func (o ObjectPredicateOption) ApplyToDeleteHandler(config *DeleteHandlerOptions) {
	config.ObjectPredicate = o.predicate
}

func (o ObjectPredicateOption) ApplyToDeOwnHandler(config *DeOwnHandlerOptions) {
	config.ObjectPredicate = o.predicate
}

type OwnershipCheckOption struct {
	enabled bool
}

func (o OwnershipCheckOption) ApplyToDeleteHandler(config *DeleteHandlerOptions) {
	config.CheckOwnership = &o.enabled
}

func WithManagedAnnotationCheck(enabled bool) ManagedAnnotationOption {
	return ManagedAnnotationOption{enabled: enabled}
}

func WithObjectPredicate(predicate ObjectPredicateFn) ObjectPredicateOption {
	return ObjectPredicateOption{predicate: predicate}
}

func WithOwnershipCheck(enabled bool) OwnershipCheckOption {
	return OwnershipCheckOption{enabled: enabled}
}
