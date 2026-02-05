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
package datasciencecluster

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	"github.com/opendatahub-io/opendatahub-operator/v2/internal/webhook/envtestutil"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/upgrade"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/reconciler"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/testf"

	. "github.com/onsi/gomega"
)

//nolint:gochecknoinits
func init() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
}

const (
	testTimeout      = 120 * time.Second
	testAppNamespace = "opendatahub"
)

// registerController registers the DSC controller with interceptor functions for fault injection.
// Pass empty interceptor.Funcs{} for normal operation.
// faultFuncs applies to the DirectClient for fault injection in upgrade actions.
func registerController(
	ctx context.Context,
	mgr ctrl.Manager,
	faultFuncs interceptor.Funcs,
) error {
	directCli, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme()})
	if err != nil {
		return err
	}
	wrappedCli := envtestutil.WrapClientWithInterceptor(directCli, faultFuncs)

	return NewDataScienceClusterReconciler(ctx, mgr, reconciler.WithDirectClient(wrappedCli))
}

func setupEnv(t *testing.T, upgradeFuncs interceptor.Funcs) (*testf.TestContext, string) {
	t.Helper()
	g := NewWithT(t)

	ctx, env, teardown := envtestutil.SetupEnvAndClientWithCRDs(
		t,
		[]envt.RegisterWebhooksFn{},
		[]envt.RegisterControllersFn{func(mgr ctrl.Manager) error {
			return registerController(t.Context(), mgr, upgradeFuncs)
		}},
		testTimeout,
	)

	t.Cleanup(teardown)

	tc, err := testf.NewTestContext(
		testf.WithClient(env.Client()),
		testf.WithContext(ctx),
	)
	g.Expect(err).NotTo(HaveOccurred(), "failed to create test context")

	appNs := testAppNamespace

	// Create a default DSCI - required by the controller to determine ApplicationsNamespace
	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       dsciv2.DSCInitializationSpec{ApplicationsNamespace: appNs},
	}
	g.Expect(env.Client().Create(ctx, dsci)).To(Succeed(), "failed to create DSCI")

	// Create the applications namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: appNs,
		},
	}
	g.Expect(env.Client().Create(ctx, ns)).To(Succeed(), "failed to create applications namespace")

	return tc, appNs
}

func TestDSCController_UpgradeAction_CleansUpRoleBindings(t *testing.T) {
	tc, appNs := setupEnv(t, interceptor.Funcs{})
	g := tc.NewWithT(t)

	// Create a RoleBinding with the same name as the applications namespace
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appNs,
			Namespace: appNs,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "admin",
		},
	}
	g.Expect(tc.Client().Create(tc.Context(), rb)).Should(Succeed())

	// Create DataScienceCluster CR to trigger reconciliation
	dscObj := resources.GvkToUnstructured(gvk.DataScienceCluster)
	dscObj.SetName("default-dsc")
	g.Create(dscObj).Eventually().Should(Succeed())

	// Verify the RoleBinding is deleted or marked for deletion
	g.Get(gvk.RoleBinding, client.ObjectKey{Name: appNs, Namespace: appNs}).
		Eventually().
		Should(Or(
			BeNil(),
			jq.Match(`.metadata.deletionTimestamp != null`),
		))
}

func TestDSCController_UpgradeAction_HandlesNoRoleBinding(t *testing.T) {
	tc, _ := setupEnv(t, interceptor.Funcs{})
	g := tc.NewWithT(t)

	// Create DataScienceCluster CR without any pre-existing RoleBinding
	dscObj := resources.GvkToUnstructured(gvk.DataScienceCluster)
	dscObj.SetName("default-dsc")
	g.Create(dscObj).Eventually().Should(Succeed())

	// Verify the controller processes the CR and sets conditions
	// The upgrade action should complete without errors when no RoleBinding exists
	g.Get(gvk.DataScienceCluster, client.ObjectKeyFromObject(dscObj)).
		Eventually().
		Should(jq.Match(`.status.conditions | length > 0`))

	// Verify that there's no UpgradeFailed condition
	g.Get(gvk.DataScienceCluster, client.ObjectKeyFromObject(dscObj)).
		Eventually().
		ShouldNot(jq.Match(`.status.conditions[] | select(.reason == "%s")`, upgrade.ReasonUpgradeFailed))
}

func TestDSCController_UpgradeAction_DeleteFailure_SetsCondition(t *testing.T) {
	upgradeFuncs := interceptor.Funcs{
		Delete: func(
			_ context.Context,
			_ client.WithWatch,
			obj client.Object,
			_ ...client.DeleteOption,
		) error {
			// Fail deletion of RoleBinding with the matching name
			if _, ok := obj.(*rbacv1.RoleBinding); ok {
				if obj.GetName() == "opendatahub" && obj.GetNamespace() == "opendatahub" {
					return errors.New("simulated delete failure")
				}
			}
			return nil
		},
	}

	tc, appNs := setupEnv(t, upgradeFuncs)
	g := tc.NewWithT(t)

	// Create a RoleBinding with the same name as the applications namespace
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appNs,
			Namespace: appNs,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "admin",
		},
	}
	g.Expect(tc.Client().Create(tc.Context(), rb)).Should(Succeed())

	// Create DataScienceCluster CR
	dscObj := resources.GvkToUnstructured(gvk.DataScienceCluster)
	dscObj.SetName("default-dsc")
	g.Create(dscObj).Eventually().Should(Succeed())

	// Verify UpgradeFailed condition is set due to delete failure
	// Note: DSC uses NonBlocking() so the condition won't block reconciliation
	g.Get(gvk.DataScienceCluster, client.ObjectKeyFromObject(dscObj)).
		Eventually().
		Should(jq.Match(`[.status.conditions[] | select(.reason == "%s")] | length > 0`, upgrade.ReasonUpgradeFailed))
}

func TestDSCController_UpgradeAction_NonBlockingContinuesReconciliation(t *testing.T) {
	upgradeFuncs := interceptor.Funcs{
		Delete: func(
			_ context.Context,
			_ client.WithWatch,
			obj client.Object,
			_ ...client.DeleteOption,
		) error {
			// Fail deletion of RoleBinding
			if _, ok := obj.(*rbacv1.RoleBinding); ok {
				return errors.New("simulated delete failure")
			}
			return nil
		},
	}

	tc, appNs := setupEnv(t, upgradeFuncs)
	g := tc.NewWithT(t)

	// Create a RoleBinding with the same name as the applications namespace
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appNs,
			Namespace: appNs,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "admin",
		},
	}
	g.Expect(tc.Client().Create(tc.Context(), rb)).Should(Succeed())

	// Create DataScienceCluster CR
	dscObj := resources.GvkToUnstructured(gvk.DataScienceCluster)
	dscObj.SetName("default-dsc")
	g.Create(dscObj).Eventually().Should(Succeed())

	// Since DSC uses NonBlocking(), the reconciliation should continue despite upgrade failure
	// Verify that other conditions are set (controller continues past upgrade action)
	g.Get(gvk.DataScienceCluster, client.ObjectKeyFromObject(dscObj)).
		Eventually().
		Should(And(
			jq.Match(`[.status.conditions[] | select(.reason == "%s")] | length > 0`, upgrade.ReasonUpgradeFailed),
			// Should have more than just the UpgradeFailed condition
			jq.Match(`.status.conditions | length > 1`),
		))
}
