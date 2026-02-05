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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
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

const testTimeout = 120 * time.Second

// registerController registers the Kueue controller with interceptor functions for fault injection.
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

	handler := &componentHandler{}
	return handler.NewComponentReconciler(ctx, mgr, reconciler.WithDirectClient(wrappedCli))
}

func setupEnv(t *testing.T, upgradeFuncs interceptor.Funcs) *testf.TestContext {
	t.Helper()
	g := NewWithT(t)

	ctx, env, teardown := envtestutil.SetupEnvAndClientWithCRDs(
		t,
		[]envt.RegisterWebhooksFn{},
		[]envt.RegisterControllersFn{func(mgr ctrl.Manager) error {
			return registerController(t.Context(), mgr, upgradeFuncs)
		}},
		testTimeout,
		envtestutil.WithKueueCRDs(),
	)

	t.Cleanup(teardown)

	tc, err := testf.NewTestContext(
		testf.WithClient(env.Client()),
		testf.WithContext(ctx),
	)
	g.Expect(err).NotTo(HaveOccurred(), "failed to create test context")

	// Create a default DSCI - required by the controller to determine ApplicationsNamespace
	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       dsciv2.DSCInitializationSpec{ApplicationsNamespace: "opendatahub"},
	}
	g.Expect(env.Client().Create(ctx, dsci)).To(Succeed(), "failed to create DSCI")

	return tc
}

func TestKueueController_UpgradeAction_CleansUpVAPB(t *testing.T) {
	g := setupEnv(t, interceptor.Funcs{}).NewWithT(t)

	vapbObj := resources.GvkToUnstructured(gvk.ValidatingAdmissionPolicyBinding)
	vapbObj.SetName("kueue-validating-admission-policy-binding")
	_ = unstructured.SetNestedField(vapbObj.Object, "some-policy", "spec", "policyName")
	_ = unstructured.SetNestedStringSlice(vapbObj.Object, []string{"Deny"}, "spec", "validationActions")

	g.Create(vapbObj).Eventually().Should(Succeed())

	kueueObj := resources.GvkToUnstructured(gvk.Kueue)
	kueueObj.SetName(componentApi.KueueInstanceName)
	g.Create(kueueObj).Eventually().Should(Succeed())

	// Verify the VAPB is deleted or marked for deletion (has deletionTimestamp)
	g.Get(gvk.ValidatingAdmissionPolicyBinding, client.ObjectKeyFromObject(vapbObj)).
		Eventually().
		Should(Or(
			BeNil(),
			jq.Match(`.metadata.deletionTimestamp != null`),
		))
}

func TestKueueController_UpgradeAction_HandlesNoVAPB(t *testing.T) {
	g := setupEnv(t, interceptor.Funcs{}).NewWithT(t)

	kueueObj := resources.GvkToUnstructured(gvk.Kueue)
	kueueObj.SetName(componentApi.KueueInstanceName)
	g.Create(kueueObj).Eventually().Should(Succeed())

	// Verify the controller processes the Kueue CR and sets conditions
	// Note: Full provisioning may fail in test environment due to missing manifests,
	// but the upgrade action should complete without errors when no VAPB exists
	// The key is that there should be no "UpgradeFailed" reason since no VAPB cleanup was needed
	g.Get(gvk.Kueue, client.ObjectKeyFromObject(kueueObj)).
		Eventually().
		Should(jq.Match(`.status.conditions | length > 0`))

	// Also verify that the failure (if any) is NOT due to UpgradeFailed
	g.Get(gvk.Kueue, client.ObjectKeyFromObject(kueueObj)).
		Eventually().
		ShouldNot(jq.Match(`.status.conditions[] | select(.reason == "%s")`, upgrade.ReasonUpgradeFailed))
}

func TestKueueController_UpgradeAction_DeleteFailure_SetsCondition(t *testing.T) {
	upgradeFuncs := interceptor.Funcs{
		Delete: func(
			_ context.Context,
			_ client.WithWatch,
			obj client.Object,
			_ ...client.DeleteOption,
		) error {
			if obj.GetName() == "kueue-validating-admission-policy-binding" {
				return errors.New("simulated delete failure")
			}
			// For non-VAPB objects, return nil (allow deletion)
			return nil
		},
	}

	g := setupEnv(t, upgradeFuncs).NewWithT(t)

	vapbObj := resources.GvkToUnstructured(gvk.ValidatingAdmissionPolicyBinding)
	vapbObj.SetName("kueue-validating-admission-policy-binding")
	_ = unstructured.SetNestedField(vapbObj.Object, "some-policy", "spec", "policyName")
	_ = unstructured.SetNestedStringSlice(vapbObj.Object, []string{"Deny"}, "spec", "validationActions")
	g.Create(vapbObj).Eventually().Should(Succeed())

	kueueObj := resources.GvkToUnstructured(gvk.Kueue)
	kueueObj.SetName(componentApi.KueueInstanceName)
	g.Create(kueueObj).Eventually().Should(Succeed())

	// Verify UpgradeFailed condition is set due to delete failure
	g.Get(gvk.Kueue, client.ObjectKeyFromObject(kueueObj)).
		Eventually().
		Should(jq.Match(`[.status.conditions[] | select(.reason == "%s")] | length > 0`, upgrade.ReasonUpgradeFailed))
}
