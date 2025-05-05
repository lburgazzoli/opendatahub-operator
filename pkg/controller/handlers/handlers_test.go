package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/handlers"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/mocks"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/scheme"

	. "github.com/onsi/gomega"
)

func TestTypedAdapter(t *testing.T) {
	t.Parallel()

	s, err := scheme.New()
	require.NoError(t, err)

	tests := []struct {
		name    string
		objType client.Object
		events  []any
		mock    func(*mocks.MockEventHandler)
	}{
		{
			name:    "should forward events for nil objects",
			objType: &appsv1.Deployment{},
			events: []any{
				event.TypedCreateEvent[client.Object]{},
				event.DeleteEvent{},
				event.UpdateEvent{},
				event.GenericEvent{},
			},
			mock: func(m *mocks.MockEventHandler) {
				m.On("Create", mock.Anything, mock.Anything, mock.Anything).Return()
				m.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return()
				m.On("Update", mock.Anything, mock.Anything, mock.Anything).Return()
				m.On("Generic", mock.Anything, mock.Anything, mock.Anything).Return()
			},
		},
		{
			name:    "should not forward events for incompatible types",
			objType: &appsv1.Deployment{},
			events: []any{
				event.TypedCreateEvent[client.Object]{
					Object: &corev1.ConfigMap{},
				},
				event.DeleteEvent{
					Object: &corev1.ConfigMap{},
				},
				event.UpdateEvent{
					ObjectOld: &corev1.ConfigMap{},
					ObjectNew: &corev1.ConfigMap{},
				},
				event.GenericEvent{
					Object: &corev1.ConfigMap{},
				},
			},
			mock: func(m *mocks.MockEventHandler) {
				// No expectations - events should not be forwarded
			},
		},
		{
			name:    "should forward events for matching types",
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
				event.GenericEvent{
					Object: &appsv1.Deployment{},
				},
			},
			mock: func(m *mocks.MockEventHandler) {
				m.On("Create", mock.Anything, mock.Anything, mock.Anything).Return()
				m.On("Delete", mock.Anything, mock.Anything, mock.Anything).Return()
				m.On("Update", mock.Anything, mock.Anything, mock.Anything).Return()
				m.On("Generic", mock.Anything, mock.Anything, mock.Anything).Return()
			},
		},
		{
			name:    "should handle unstructured objects",
			objType: &appsv1.Deployment{},
			events: []any{
				event.TypedCreateEvent[client.Object]{
					Object: resources.GvkToUnstructured(gvk.Deployment),
				},
			},
			mock: func(m *mocks.MockEventHandler) {
				m.On("Create", mock.Anything, mock.Anything, mock.Anything).Return()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockHandler := mocks.NewMockEventHandler(tt.mock)
			adapter := handlers.TypedAdapter(s, tt.objType, mockHandler)

			// Test each event type
			for _, e := range tt.events {
				switch evt := e.(type) {
				case event.CreateEvent:
					adapter.Create(context.Background(), evt, &controllertest.Queue{})
				case event.DeleteEvent:
					adapter.Delete(context.Background(), evt, &controllertest.Queue{})
				case event.UpdateEvent:
					adapter.Update(context.Background(), evt, &controllertest.Queue{})
				case event.GenericEvent:
					adapter.Generic(context.Background(), evt, &controllertest.Queue{})
				}
			}

			// Verify mock expectations
			mockHandler.AssertExpectations(t)
		})
	}
}

func TestNewEventHandlerForGVK(t *testing.T) {
	t.Run("should return empty requests when no resources exist", func(t *testing.T) {
		g := NewWithT(t)
		ctx := context.Background()

		cli, err := fakeclient.New()
		g.Expect(err).ShouldNot(HaveOccurred())

		eh := handlers.NewEventHandlerForGVK(cli, gvk.Namespace)

		q := controllertest.Queue{
			TypedInterface: workqueue.NewTyped[reconcile.Request](),
		}

		eh.Create(
			ctx,
			event.CreateEvent{},
			&q)

		g.Expect(q.Len()).To(BeZero())
	})

	t.Run("should enqueue requests for existing resources", func(t *testing.T) {
		g := NewWithT(t)
		ctx := context.Background()

		// Create test resources using GvkToUnstructured
		obj1 := resources.GvkToUnstructured(gvk.Namespace)
		obj1.SetName("test-ns-1")

		obj2 := resources.GvkToUnstructured(gvk.Namespace)
		obj2.SetName("test-ns-2")

		// Create fake client with objects
		cli, err := fakeclient.New(fakeclient.WithObjects(obj1, obj2))
		g.Expect(err).ShouldNot(HaveOccurred())

		eh := handlers.NewEventHandlerForGVK(cli, gvk.Namespace)

		q := controllertest.Queue{
			TypedInterface: workqueue.NewTyped[reconcile.Request](),
		}

		eh.Create(
			ctx,
			event.CreateEvent{},
			&q)

		// Verify both resources were enqueued
		g.Expect(q.Len()).To(Equal(2))

		// Get and verify each request individually
		g.Expect(q.Get()).To(Equal(reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "test-ns-1"},
		}))

		g.Expect(q.Get()).To(Equal(reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "test-ns-2"},
		}))
	})

	t.Run("should only enqueue requests for matching GVK", func(t *testing.T) {
		g := NewWithT(t)
		ctx := context.Background()

		// Create resources with different GVKs
		nsObj := resources.GvkToUnstructured(gvk.Namespace)
		nsObj.SetName("test-ns")

		secretObj := resources.GvkToUnstructured(gvk.Secret)
		secretObj.SetName("test-secret")

		configMapObj := resources.GvkToUnstructured(gvk.ConfigMap)
		configMapObj.SetName("test-cm")

		// Create fake client with mixed objects
		cli, err := fakeclient.New(fakeclient.WithObjects(nsObj, secretObj, configMapObj))
		g.Expect(err).ShouldNot(HaveOccurred())

		// Create handler that only watches Namespace GVK
		eh := handlers.NewEventHandlerForGVK(cli, gvk.Namespace)

		q := controllertest.Queue{
			TypedInterface: workqueue.NewTyped[reconcile.Request](),
		}

		eh.Create(
			ctx,
			event.CreateEvent{},
			&q)

		// Verify only the namespace resource was enqueued
		g.Expect(q.Len()).To(Equal(1))

		// Get and verify the request
		g.Expect(q.Get()).To(Equal(reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "test-ns"},
		}))
	})
}
