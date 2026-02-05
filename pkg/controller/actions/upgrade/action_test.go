/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package upgrade_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	dscv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v2"
	odherrors "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/errors"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/upgrade"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/mocks"

	. "github.com/onsi/gomega"
)

func newMockManager(cli client.Client) *mocks.Manager {
	return mocks.NewMockManager(func(m *mocks.Manager) {
		m.On("GetConfig").Return(nil)
		m.On("GetClient").Return(cli)
	})
}

func newMockManagerWithRecorder(cli client.Client) (*mocks.Manager, *record.FakeRecorder) {
	recorder := record.NewFakeRecorder(10)
	mgr := mocks.NewMockManager(func(m *mocks.Manager) {
		m.On("GetConfig").Return(nil)
		m.On("GetClient").Return(cli)
		m.On("GetEventRecorderFor").Return(recorder)
	})
	return mgr, recorder
}

func newMockController(cli client.Client) *mocks.MockController {
	return mocks.NewMockController(func(m *mocks.MockController) {
		m.On("GetClient").Return(cli)
		m.On("GetDirectClient").Return(cli)
	})
}

func TestAction_RunsOnce(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	callCount := 0
	mgr := newMockManager(cli)
	action := upgrade.NewAction(mgr, upgrade.WithFn(func(_ context.Context, _ client.Client) error {
		callCount++
		return nil
	}))

	rr := &types.ReconciliationRequest{Client: cli, Controller: newMockController(cli)}

	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(callCount).To(Equal(1))

	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(callCount).To(Equal(1))

	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(callCount).To(Equal(1))
}

func TestAction_RetriesOnError(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	callCount := 0
	mgr, _ := newMockManagerWithRecorder(cli)
	action := upgrade.NewAction(mgr, upgrade.WithFn(func(_ context.Context, _ client.Client) error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}))

	rr := &types.ReconciliationRequest{
		Client:     cli,
		Controller: newMockController(cli),
		Instance: &dscv2.DataScienceCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-dsc"},
		},
	}

	err = action.Run(ctx, rr)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(callCount).To(Equal(1))

	err = action.Run(ctx, rr)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(callCount).To(Equal(2))

	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(callCount).To(Equal(3))

	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(callCount).To(Equal(3))
}

func TestAction_ErrorsConvertToStopErrors(t *testing.T) {
	tests := []struct {
		name        string
		inputErr    error
		expectedMsg string
	}{
		{
			name:        "wraps standard error to StopError",
			inputErr:    errors.New("some error"),
			expectedMsg: "some error",
		},
		{
			name:        "preserves existing StopError",
			inputErr:    odherrors.NewStopError("already a stop error"),
			expectedMsg: "already a stop error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := t.Context()

			cli, err := fakeclient.New()
			g.Expect(err).ShouldNot(HaveOccurred())

			mgr, _ := newMockManagerWithRecorder(cli)
			action := upgrade.NewAction(mgr, upgrade.WithFn(func(_ context.Context, _ client.Client) error {
				return tt.inputErr
			}))

			rr := &types.ReconciliationRequest{
				Client:     cli,
				Controller: newMockController(cli),
				Instance: &dscv2.DataScienceCluster{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dsc"},
				},
			}

			err = action.Run(ctx, rr)
			g.Expect(err).Should(HaveOccurred())

			var stopErr odherrors.StopError
			g.Expect(errors.As(err, &stopErr)).To(BeTrue())
			g.Expect(stopErr.Error()).To(Equal(tt.expectedMsg))
		})
	}
}

func TestAction_ReceivesClient(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	var receivedClient client.Client
	mgr := newMockManager(cli)
	action := upgrade.NewAction(mgr, upgrade.WithFn(func(_ context.Context, c client.Client) error {
		receivedClient = c
		return nil
	}))

	rr := &types.ReconciliationRequest{Client: cli, Controller: newMockController(cli)}

	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(receivedClient).To(Equal(cli))
}

func TestAction_ThreadSafe(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	var callCount atomic.Int32
	mgr := newMockManager(cli)
	action := upgrade.NewAction(mgr, upgrade.WithFn(func(_ context.Context, _ client.Client) error {
		callCount.Add(1)
		return nil
	}))

	rr := &types.ReconciliationRequest{Client: cli, Controller: newMockController(cli)}

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = action.Run(ctx, rr)
		}()
	}
	wg.Wait()

	g.Expect(callCount.Load()).To(Equal(int32(1)))
}

func TestAction_MultipleFunctions(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	var order []int
	fn1 := func(_ context.Context, _ client.Client) error {
		order = append(order, 1)
		return nil
	}
	fn2 := func(_ context.Context, _ client.Client) error {
		order = append(order, 2)
		return nil
	}
	fn3 := func(_ context.Context, _ client.Client) error {
		order = append(order, 3)
		return nil
	}

	mgr := newMockManager(cli)
	action := upgrade.NewAction(mgr,
		upgrade.WithFn(fn1),
		upgrade.WithFn(fn2),
		upgrade.WithFn(fn3),
	)
	rr := &types.ReconciliationRequest{Client: cli, Controller: newMockController(cli)}

	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(order).To(Equal([]int{1, 2, 3}))

	// Verify run-once semantics
	order = nil
	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(order).To(BeNil())
}

func TestAction_MultipleFunctions_CombinesErrors(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	var order []int
	fn1 := func(_ context.Context, _ client.Client) error {
		order = append(order, 1)
		return nil
	}
	fn2 := func(_ context.Context, _ client.Client) error {
		order = append(order, 2)
		return errors.New("fn2 failed")
	}
	fn3 := func(_ context.Context, _ client.Client) error {
		order = append(order, 3)
		return errors.New("fn3 failed")
	}

	mgr, _ := newMockManagerWithRecorder(cli)
	action := upgrade.NewAction(mgr,
		upgrade.WithFn(fn1),
		upgrade.WithFn(fn2),
		upgrade.WithFn(fn3),
	)
	rr := &types.ReconciliationRequest{
		Client:     cli,
		Controller: newMockController(cli),
		Instance: &dscv2.DataScienceCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-dsc"},
		},
	}

	err = action.Run(ctx, rr)
	g.Expect(err).Should(HaveOccurred())
	// All functions should run
	g.Expect(order).To(Equal([]int{1, 2, 3}))
	// Errors should be combined
	g.Expect(err.Error()).To(ContainSubstring("fn2 failed"))
	g.Expect(err.Error()).To(ContainSubstring("fn3 failed"))
}

func TestAction_NonBlocking_ReturnsStandardError(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	upgradeErr := errors.New("upgrade failed")
	mgr, _ := newMockManagerWithRecorder(cli)
	action := upgrade.NewAction(mgr,
		upgrade.WithFn(func(_ context.Context, _ client.Client) error {
			return upgradeErr
		}),
		upgrade.NonBlocking(),
	)

	rr := &types.ReconciliationRequest{
		Client:     cli,
		Controller: newMockController(cli),
		Instance: &dscv2.DataScienceCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-dsc"},
		},
	}

	err = action.Run(ctx, rr)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err.Error()).To(Equal("upgrade failed"))

	// Should NOT be a StopError
	var stopErr odherrors.StopError
	g.Expect(errors.As(err, &stopErr)).To(BeFalse())
}

func TestAction_NonBlocking_CombinesErrors(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	mgr, _ := newMockManagerWithRecorder(cli)
	action := upgrade.NewAction(mgr,
		upgrade.WithFn(func(_ context.Context, _ client.Client) error {
			return errors.New("error1")
		}),
		upgrade.WithFn(func(_ context.Context, _ client.Client) error {
			return errors.New("error2")
		}),
		upgrade.NonBlocking(),
	)

	rr := &types.ReconciliationRequest{
		Client:     cli,
		Controller: newMockController(cli),
		Instance: &dscv2.DataScienceCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-dsc"},
		},
	}

	err = action.Run(ctx, rr)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("error1"))
	g.Expect(err.Error()).To(ContainSubstring("error2"))

	// Should NOT be a StopError
	var stopErr odherrors.StopError
	g.Expect(errors.As(err, &stopErr)).To(BeFalse())
}

func TestAction_NonBlocking_MarksDoneOnSuccess(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	callCount := 0
	mgr, _ := newMockManagerWithRecorder(cli)
	action := upgrade.NewAction(mgr,
		upgrade.WithFn(func(_ context.Context, _ client.Client) error {
			callCount++
			return nil
		}),
		upgrade.NonBlocking(),
	)

	rr := &types.ReconciliationRequest{
		Client:     cli,
		Controller: newMockController(cli),
		Instance: &dscv2.DataScienceCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-dsc"},
		},
	}

	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(callCount).To(Equal(1))

	// Should not run again
	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(callCount).To(Equal(1))
}

func TestAction_NonBlocking_EmitsEvent(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	mgr, recorder := newMockManagerWithRecorder(cli)
	action := upgrade.NewAction(mgr,
		upgrade.WithFn(func(_ context.Context, _ client.Client) error {
			return errors.New("upgrade failed")
		}),
		upgrade.NonBlocking(),
	)

	rr := &types.ReconciliationRequest{
		Client:     cli,
		Controller: newMockController(cli),
		Instance: &dscv2.DataScienceCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-dsc"},
		},
	}

	err = action.Run(ctx, rr)
	g.Expect(err).Should(HaveOccurred())

	// Verify event was emitted
	g.Expect(recorder.Events).Should(Receive(ContainSubstring("UpgradeFailed")))
}

func TestAction_NoFunctions(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	mgr := newMockManager(cli)
	action := upgrade.NewAction(mgr)

	rr := &types.ReconciliationRequest{Client: cli, Controller: newMockController(cli)}

	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())

	// Should be marked done even with no functions
	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestAction_WithFaultyClient_Blocking(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	faultyInterceptor := interceptor.Funcs{
		Get: func(
			ctx context.Context,
			c client.WithWatch,
			key client.ObjectKey,
			obj client.Object,
			opts ...client.GetOption,
		) error {
			return errors.New("simulated server error")
		},
	}

	cli, err := fakeclient.New(fakeclient.WithInterceptorFuncs(faultyInterceptor))
	g.Expect(err).ShouldNot(HaveOccurred())

	callCount := 0
	mgr, _ := newMockManagerWithRecorder(cli)
	action := upgrade.NewAction(mgr, upgrade.WithFn(func(ctx context.Context, c client.Client) error {
		callCount++
		obj := &corev1.ConfigMap{}
		return c.Get(ctx, client.ObjectKey{Name: "test", Namespace: "default"}, obj)
	}))

	rr := &types.ReconciliationRequest{
		Client:     cli,
		Controller: newMockController(cli),
		Instance: &dscv2.DataScienceCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-dsc"},
		},
	}

	err = action.Run(ctx, rr)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(callCount).To(Equal(1))

	var stopErr odherrors.StopError
	g.Expect(errors.As(err, &stopErr)).To(BeTrue())
}

func TestAction_WithFaultyClient_NonBlocking(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New(fakeclient.WithInterceptorFuncs(interceptor.Funcs{
		Get: func(
			ctx context.Context,
			c client.WithWatch,
			key client.ObjectKey,
			obj client.Object,
			opts ...client.GetOption,
		) error {
			return errors.New("simulated server error")
		},
	}))
	g.Expect(err).ShouldNot(HaveOccurred())

	mgr, recorder := newMockManagerWithRecorder(cli)

	action := upgrade.NewAction(mgr,
		upgrade.WithFn(func(ctx context.Context, c client.Client) error {
			obj := &corev1.ConfigMap{}
			return c.Get(ctx, client.ObjectKey{Name: "test", Namespace: "default"}, obj)
		}),
		upgrade.NonBlocking(),
	)

	rr := &types.ReconciliationRequest{
		Client:     cli,
		Controller: newMockController(cli),
		Instance: &dscv2.DataScienceCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-dsc"},
		},
	}

	err = action.Run(ctx, rr)
	g.Expect(err).Should(HaveOccurred())

	var stopErr odherrors.StopError
	g.Expect(errors.As(err, &stopErr)).To(BeFalse())

	g.Expect(recorder.Events).Should(Receive(ContainSubstring("UpgradeFailed")))
}

func TestAction_MultipleFunctions_OneFails_WithFaultyClient(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New(fakeclient.WithInterceptorFuncs(interceptor.Funcs{
		Get: func(
			ctx context.Context,
			c client.WithWatch,
			key client.ObjectKey,
			obj client.Object,
			opts ...client.GetOption,
		) error {
			return errors.New("simulated server error")
		},
	}))
	g.Expect(err).ShouldNot(HaveOccurred())

	var order []int
	fn1 := func(_ context.Context, _ client.Client) error {
		order = append(order, 1)
		return nil
	}
	fn2 := func(ctx context.Context, c client.Client) error {
		order = append(order, 2)
		obj := &corev1.ConfigMap{}
		return c.Get(ctx, client.ObjectKey{Name: "test", Namespace: "default"}, obj)
	}
	fn3 := func(_ context.Context, _ client.Client) error {
		order = append(order, 3)
		return nil
	}

	mgr, _ := newMockManagerWithRecorder(cli)
	action := upgrade.NewAction(mgr,
		upgrade.WithFn(fn1),
		upgrade.WithFn(fn2),
		upgrade.WithFn(fn3),
	)
	rr := &types.ReconciliationRequest{
		Client:     cli,
		Controller: newMockController(cli),
		Instance: &dscv2.DataScienceCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-dsc"},
		},
	}

	err = action.Run(ctx, rr)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(order).To(Equal([]int{1, 2, 3}))
	g.Expect(err.Error()).To(ContainSubstring("simulated server error"))
}

func TestAction_UsesDirectClient(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	getCalled := false
	interceptorFuncs := interceptor.Funcs{
		Get: func(
			ctx context.Context,
			c client.WithWatch,
			key client.ObjectKey,
			obj client.Object,
			opts ...client.GetOption,
		) error {
			getCalled = true
			return nil
		},
	}

	cli, err := fakeclient.New(fakeclient.WithInterceptorFuncs(interceptorFuncs))
	g.Expect(err).ShouldNot(HaveOccurred())

	mgr := newMockManager(cli)
	action := upgrade.NewAction(mgr,
		upgrade.WithFn(func(ctx context.Context, c client.Client) error {
			return c.Get(ctx, client.ObjectKey{Name: "test"}, &corev1.ConfigMap{})
		}),
	)

	// DirectClient is the one that should be used by upgrade functions
	rr := &types.ReconciliationRequest{Client: cli, Controller: newMockController(cli)}
	err = action.Run(ctx, rr)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(getCalled).To(BeTrue(), "DirectClient should be used by upgrade functions")
}
