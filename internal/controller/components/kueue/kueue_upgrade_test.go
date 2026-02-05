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

//nolint:testpackage
package kueue

import (
	"context"
	"errors"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"

	. "github.com/onsi/gomega"
)

func TestCleanupDeprecatedVAPB_DeletesExisting(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	vapb := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kueue-validating-admission-policy-binding",
		},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(vapb))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = cleanupDeprecatedVAPB(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())

	var deletedVAPB admissionregistrationv1.ValidatingAdmissionPolicyBinding
	err = cli.Get(ctx, client.ObjectKey{Name: "kueue-validating-admission-policy-binding"}, &deletedVAPB)
	g.Expect(k8serr.IsNotFound(err)).To(BeTrue())
}

func TestCleanupDeprecatedVAPB_HandlesNotFound(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	err = cleanupDeprecatedVAPB(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestCleanupDeprecatedVAPB_GetFails(t *testing.T) {
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
			if _, ok := obj.(*admissionregistrationv1.ValidatingAdmissionPolicyBinding); ok {
				return errors.New("simulated API server error")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = cleanupDeprecatedVAPB(ctx, cli)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to get VAPB"))
}

func TestCleanupDeprecatedVAPB_DeleteFails(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	vapb := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "kueue-validating-admission-policy-binding"},
	}

	cli, err := fakeclient.New(
		fakeclient.WithObjects(vapb),
		fakeclient.WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.DeleteOption,
			) error {
				if _, ok := obj.(*admissionregistrationv1.ValidatingAdmissionPolicyBinding); ok {
					return errors.New("simulated delete failure")
				}
				return c.Delete(ctx, obj, opts...)
			},
		}),
	)
	g.Expect(err).ShouldNot(HaveOccurred())

	err = cleanupDeprecatedVAPB(ctx, cli)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to delete VAPB"))
}

func TestCleanupDeprecatedVAPB_TransientFailureThenSuccess(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	vapb := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "kueue-validating-admission-policy-binding"},
	}

	callCount := 0
	cli, err := fakeclient.New(
		fakeclient.WithObjects(vapb),
		fakeclient.WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(
				ctx context.Context,
				c client.WithWatch,
				obj client.Object,
				opts ...client.DeleteOption,
			) error {
				if _, ok := obj.(*admissionregistrationv1.ValidatingAdmissionPolicyBinding); ok {
					callCount++
					if callCount < 2 {
						return errors.New("transient failure")
					}
				}
				return c.Delete(ctx, obj, opts...)
			},
		}),
	)
	g.Expect(err).ShouldNot(HaveOccurred())

	// First call fails
	err = cleanupDeprecatedVAPB(ctx, cli)
	g.Expect(err).Should(HaveOccurred())

	// Second call succeeds (simulating retry)
	err = cleanupDeprecatedVAPB(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())
}
