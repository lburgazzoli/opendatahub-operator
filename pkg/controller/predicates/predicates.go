package predicates

import (
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/generation"
)

var (
	// DefaultPredicate is the default set of predicates associated to
	// resources when there is no specific predicate configured via the
	// builder.
	//
	// It would trigger a reconciliation if either the generation or
	// metadata (labels, annotations) have changed.
	DefaultPredicate = predicate.Or(
		generation.New(),
		predicate.LabelChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
	)
)

// PredicateOptions configures the behavior of predicates.
type PredicateOptions struct {
	// AcceptCreate determines if create events should trigger reconciliation
	AcceptCreate bool
	// AcceptDelete determines if delete events should trigger reconciliation
	AcceptDelete bool
}

// PredicateOption is a functional option for configuring predicates.
type PredicateOption func(*PredicateOptions)

func WithAcceptCreate(value bool) PredicateOption {
	return func(o *PredicateOptions) {
		o.AcceptCreate = value
	}
}

func WithAcceptDelete(value bool) PredicateOption {
	return func(o *PredicateOptions) {
		o.AcceptDelete = value
	}
}
