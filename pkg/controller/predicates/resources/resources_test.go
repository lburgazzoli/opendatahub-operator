package resources_test

import (
	"testing"

	"github.com/onsi/gomega/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/predicates/resources"
	res "github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/mocks"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/scheme"

	. "github.com/onsi/gomega"
)

func TestAnnotationChanged(t *testing.T) {
	t.Parallel()

	const annotationName = "test-annotation"

	tests := []struct {
		name           string
		annotation     string
		oldAnnotations map[string]string
		newAnnotations map[string]string
		want           bool
	}{
		{
			name:           "annotation value changed",
			oldAnnotations: map[string]string{annotationName: "old-value"},
			newAnnotations: map[string]string{annotationName: "new-value"},
			want:           true,
		},
		{
			name:           "annotation value unchanged",
			oldAnnotations: map[string]string{annotationName: "same-value"},
			newAnnotations: map[string]string{annotationName: "same-value"},
			want:           false,
		},
		{
			name:           "annotation added",
			oldAnnotations: map[string]string{},
			newAnnotations: map[string]string{annotationName: "new-value"},
			want:           true,
		},
		{
			name:           "annotation removed",
			oldAnnotations: map[string]string{annotationName: "old-value"},
			newAnnotations: map[string]string{},
			want:           true,
		},
		{
			name:           "different annotation changed",
			oldAnnotations: map[string]string{annotationName: "value", "other-annotation": "old-value"},
			newAnnotations: map[string]string{annotationName: "value", "other-annotation": "new-value"},
			want:           false,
		},
		{
			name:           "nil annotations in both objects",
			oldAnnotations: nil,
			newAnnotations: nil,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			got := resources.AnnotationChanged(annotationName).UpdateFunc(event.UpdateEvent{
				ObjectOld: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-pod",
						Annotations: tt.oldAnnotations,
					},
				},
				ObjectNew: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-pod",
						Annotations: tt.newAnnotations,
					},
				},
			})

			g.Expect(got).To(Equal(tt.want))
		})
	}
}

func TestPathDriftPredicate(t *testing.T) {
	t.Parallel()

	// Helper function to create a test object
	createTestObject := func(name string, value interface{}) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": name,
				},
				"spec": map[string]interface{}{
					"value": value,
				},
			},
		}
	}

	tests := []struct {
		name           string
		paths          []string
		oldObj         client.Object
		newObj         client.Object
		opts           []predicates.PredicateOption
		expectedUpdate bool
	}{
		{
			name:           "should accept create events by default",
			paths:          []string{"spec.value"},
			expectedUpdate: false,
		},
		{
			name:           "should reject create events when disabled",
			paths:          []string{"spec.value"},
			opts:           []predicates.PredicateOption{predicates.WithAcceptCreate(false)},
			expectedUpdate: false,
		},
		{
			name:           "should reject delete events when disabled",
			paths:          []string{"spec.value"},
			opts:           []predicates.PredicateOption{predicates.WithAcceptDelete(false)},
			expectedUpdate: false,
		},
		{
			name:   "should detect drift when path exists in one object but not the other",
			paths:  []string{"spec.value"},
			oldObj: createTestObject("test", "old"),
			newObj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": "test",
					},
				},
			},
			expectedUpdate: true,
		},
		{
			name:           "should detect drift when values are different",
			paths:          []string{"spec.value"},
			oldObj:         createTestObject("test", "old"),
			newObj:         createTestObject("test", "new"),
			expectedUpdate: true,
		},
		{
			name:           "should not detect drift when values are the same",
			paths:          []string{"spec.value"},
			oldObj:         createTestObject("test", "same"),
			newObj:         createTestObject("test", "same"),
			expectedUpdate: false,
		},
		{
			name:           "should handle nil objects",
			paths:          []string{"spec.value"},
			oldObj:         nil,
			newObj:         createTestObject("test", "value"),
			expectedUpdate: true,
		},
		{
			name:           "should handle multiple paths",
			paths:          []string{"spec.value", "metadata.name"},
			oldObj:         createTestObject("old", "value"),
			newObj:         createTestObject("new", "value"),
			expectedUpdate: true,
		},
		{
			name:           "should not detect drift when path doesn't exist in either object",
			paths:          []string{"spec.nonexistent"},
			oldObj:         createTestObject("test", "value"),
			newObj:         createTestObject("test", "value"),
			expectedUpdate: false,
		},
		{
			name:           "should not detect drift when multiple paths don't exist in either object",
			paths:          []string{"spec.nonexistent1", "spec.nonexistent2"},
			oldObj:         createTestObject("test", "value"),
			newObj:         createTestObject("test", "value"),
			expectedUpdate: false,
		},
		{
			name:           "should detect drift when one path exists and others don't",
			paths:          []string{"spec.value", "spec.nonexistent1", "spec.nonexistent2"},
			oldObj:         createTestObject("test", "old"),
			newObj:         createTestObject("test", "new"),
			expectedUpdate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)

			predicate := resources.PathDriftPredicate(tt.paths, tt.opts...)

			// Default options - both create and delete events are accepted by default
			options := predicates.PredicateOptions{
				AcceptCreate: true,
				AcceptDelete: true,
			}

			// Apply provided options
			for _, opt := range tt.opts {
				opt(&options)
			}

			g.Expect(
				predicate.Generic(event.GenericEvent{
					Object: tt.newObj,
				}),
			).To(
				BeFalse(),
			)

			g.Expect(
				predicate.Create(event.CreateEvent{
					Object: tt.newObj,
				}),
			).To(
				Equal(options.AcceptCreate),
			)

			g.Expect(
				predicate.Delete(event.DeleteEvent{
					Object: tt.oldObj,
				}),
			).To(
				Equal(options.AcceptDelete),
			)

			g.Expect(
				predicate.Update(event.UpdateEvent{
					ObjectOld: tt.oldObj,
					ObjectNew: tt.newObj,
				}),
			).To(
				Equal(tt.expectedUpdate),
			)
		})
	}
}

func TestTypedPredicateAdapter(t *testing.T) {
	t.Parallel()

	s, err := scheme.New()
	require.NoError(t, err)

	tests := []struct {
		name    string
		objType client.Object
		events  []any
		mp      *mocks.MockPredicate
		matcher types.GomegaMatcher
	}{
		{
			name:    "event should be forwarded for nil object",
			objType: &appsv1.Deployment{},
			events: []any{
				event.CreateEvent{},
				event.DeleteEvent{},
				event.UpdateEvent{},
			},
			mp: mocks.NewMockTypedPredicate(
				func(m *mocks.MockPredicate) {
					m.On("Create", mock.Anything).Return(true).Once()
					m.On("Delete", mock.Anything).Return(true).Once()
					m.On("Update", mock.Anything).Return(true).Once()
				},
			),
			matcher: Equal(true),
		},
		{
			name:    "event should not be forwarded for non compatible object",
			objType: &appsv1.Deployment{},
			events: []any{
				event.CreateEvent{
					Object: res.GvkToUnstructured(gvk.ConfigMap),
				},
				event.DeleteEvent{
					Object: res.GvkToUnstructured(gvk.ConfigMap),
				},
				event.UpdateEvent{
					ObjectOld: res.GvkToUnstructured(gvk.ConfigMap),
					ObjectNew: res.GvkToUnstructured(gvk.ConfigMap),
				},
			},
			mp: mocks.NewMockTypedPredicate(
				func(m *mocks.MockPredicate) {
				},
			),
			matcher: Equal(false),
		},
		{
			name:    "event should be forwarded for matching type",
			objType: &appsv1.Deployment{},
			events: []any{
				event.CreateEvent{
					Object: &appsv1.Deployment{},
				},
				event.DeleteEvent{
					Object: &appsv1.Deployment{},
				},
				event.UpdateEvent{
					ObjectOld: &appsv1.Deployment{},
					ObjectNew: &appsv1.Deployment{},
				},
			},
			mp: mocks.NewMockTypedPredicate(
				func(m *mocks.MockPredicate) {
					m.On("Create", mock.Anything).Return(true).Once()
					m.On("Delete", mock.Anything).Return(true).Once()
					m.On("Update", mock.Anything).Return(true).Once()
				},
			),
			matcher: Equal(true),
		},
		{
			name:    "event should be forwarded for matching unstructured type",
			objType: &appsv1.Deployment{},
			events: []any{
				event.CreateEvent{
					Object: res.GvkToUnstructured(gvk.Deployment),
				},
				event.DeleteEvent{
					Object: res.GvkToUnstructured(gvk.Deployment),
				},
				event.UpdateEvent{
					ObjectOld: res.GvkToUnstructured(gvk.Deployment),
					ObjectNew: res.GvkToUnstructured(gvk.Deployment),
				},
			},
			mp: mocks.NewMockTypedPredicate(
				func(m *mocks.MockPredicate) {
					m.On("Create", mock.Anything).Return(true).Once()
					m.On("Delete", mock.Anything).Return(true).Once()
					m.On("Update", mock.Anything).Return(true).Once()
				},
			),
			matcher: Equal(true),
		},
		{
			name:    "event should be forwarded for unstructured",
			objType: res.GvkToUnstructured(gvk.Deployment),
			events: []any{
				event.CreateEvent{
					Object: res.GvkToUnstructured(gvk.Deployment),
				},
				event.DeleteEvent{
					Object: res.GvkToUnstructured(gvk.Deployment),
				},
				event.UpdateEvent{
					ObjectOld: res.GvkToUnstructured(gvk.Deployment),
					ObjectNew: res.GvkToUnstructured(gvk.Deployment),
				},
			},
			mp: mocks.NewMockTypedPredicate(
				func(m *mocks.MockPredicate) {
					m.On("Create", mock.Anything).Return(true).Once()
					m.On("Delete", mock.Anything).Return(true).Once()
					m.On("Update", mock.Anything).Return(true).Once()
				},
			),
			matcher: Equal(true),
		},
		{
			name:    "event should be forwarded for partial",
			objType: res.GvkToPartial(gvk.Deployment),
			events: []any{
				event.CreateEvent{
					Object: res.GvkToPartial(gvk.Deployment),
				},
				event.DeleteEvent{
					Object: res.GvkToPartial(gvk.Deployment),
				},
				event.UpdateEvent{
					ObjectOld: res.GvkToPartial(gvk.Deployment),
					ObjectNew: res.GvkToPartial(gvk.Deployment),
				},
			},
			mp: mocks.NewMockTypedPredicate(
				func(m *mocks.MockPredicate) {
					m.On("Create", mock.Anything).Return(true).Once()
					m.On("Delete", mock.Anything).Return(true).Once()
					m.On("Update", mock.Anything).Return(true).Once()
				},
			),
			matcher: Equal(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			p := resources.TypedPredicateAdapter(s, tt.objType, tt.mp)

			for _, ev := range tt.events {
				switch e := ev.(type) {
				case event.CreateEvent:
					g.Expect(p.Create(e)).To(tt.matcher)
				case event.DeleteEvent:
					g.Expect(p.Delete(e)).To(tt.matcher)
				case event.UpdateEvent:
					g.Expect(p.Update(e)).To(tt.matcher)
				}
			}

			tt.mp.AssertExpectations(t)
		})
	}
}
