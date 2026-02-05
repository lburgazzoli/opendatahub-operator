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
package modelcontroller

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

// registerController registers the ModelController controller with interceptor functions for fault injection.
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

	// Create a default DSCI - required by the controller to determine ApplicationsNamespace
	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       dsciv2.DSCInitializationSpec{ApplicationsNamespace: "opendatahub"},
	}
	g.Expect(env.Client().Create(ctx, dsci)).To(Succeed(), "failed to create DSCI")

	// Create the applications namespace
	appNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "opendatahub",
		},
	}
	g.Expect(env.Client().Create(ctx, appNs)).To(Succeed(), "failed to create applications namespace")

	return tc, "opendatahub"
}

// newModelControllerCR creates a ModelController CR with required spec fields.
func newModelControllerCR() *unstructured.Unstructured {
	obj := resources.GvkToUnstructured(gvk.ModelController)
	obj.SetName(componentApi.ModelControllerInstanceName)
	// Set required spec fields to avoid nil pointer dereference
	_ = unstructured.SetNestedField(obj.Object, "Removed", "spec", "kserve", "managementState")
	_ = unstructured.SetNestedField(obj.Object, "Removed", "spec", "kserve", "nim", "managementState")
	return obj
}

func TestModelControllerController_UpgradeAction_CleansUpLegacyDeployment(t *testing.T) {
	tc, appNs := setupEnv(t, interceptor.Funcs{})
	g := tc.NewWithT(t)

	// Create legacy deployment without the platform label using the raw client
	legacyDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-model-controller",
			Namespace: appNs,
			Labels: map[string]string{
				"app": "odh-model-controller",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "odh-model-controller"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "odh-model-controller"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "controller",
						Image: "quay.io/opendatahub/odh-model-controller:latest",
					}},
				},
			},
		},
	}
	g.Expect(tc.Client().Create(tc.Context(), legacyDeployment)).Should(Succeed())

	// Create ModelController CR to trigger reconciliation
	modelControllerObj := newModelControllerCR()
	g.Create(modelControllerObj).Eventually().Should(Succeed())

	// Verify the legacy deployment is deleted or marked for deletion
	g.Get(gvk.Deployment, client.ObjectKey{Name: "odh-model-controller", Namespace: appNs}).
		Eventually().
		Should(Or(
			BeNil(),
			jq.Match(`.metadata.deletionTimestamp != null`),
		))
}

func TestModelControllerController_UpgradeAction_SkipsNewDeployment(t *testing.T) {
	tc, appNs := setupEnv(t, interceptor.Funcs{})
	g := tc.NewWithT(t)

	// Create deployment WITH the platform label (should not be deleted)
	newDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-model-controller",
			Namespace: appNs,
			Labels: map[string]string{
				"app":                             "odh-model-controller",
				"platform.opendatahub.io/part-of": componentApi.ModelControllerComponentName,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "odh-model-controller"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "odh-model-controller"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "controller",
						Image: "quay.io/opendatahub/odh-model-controller:latest",
					}},
				},
			},
		},
	}
	g.Expect(tc.Client().Create(tc.Context(), newDeployment)).Should(Succeed())

	// Create ModelController CR to trigger reconciliation
	modelControllerObj := newModelControllerCR()
	g.Create(modelControllerObj).Eventually().Should(Succeed())

	// Wait for reconciliation to process by checking that conditions are set
	g.Get(gvk.ModelController, client.ObjectKeyFromObject(modelControllerObj)).
		Eventually().
		Should(jq.Match(`.status.conditions | length > 0`))

	// Verify the deployment still exists and has no deletionTimestamp
	g.Get(gvk.Deployment, client.ObjectKey{Name: "odh-model-controller", Namespace: appNs}).
		Eventually().
		Should(jq.Match(`.metadata.deletionTimestamp == null`))
}

func TestModelControllerController_UpgradeAction_HandlesNoDeployment(t *testing.T) {
	tc, _ := setupEnv(t, interceptor.Funcs{})
	g := tc.NewWithT(t)

	// Create ModelController CR without any pre-existing deployment
	modelControllerObj := newModelControllerCR()
	g.Create(modelControllerObj).Eventually().Should(Succeed())

	// Verify the controller processes the CR and sets conditions
	// The upgrade action should complete without errors when no deployment exists
	g.Get(gvk.ModelController, client.ObjectKeyFromObject(modelControllerObj)).
		Eventually().
		Should(jq.Match(`.status.conditions | length > 0`))

	// Verify that there's no UpgradeFailed condition
	g.Get(gvk.ModelController, client.ObjectKeyFromObject(modelControllerObj)).
		Eventually().
		ShouldNot(jq.Match(`.status.conditions[] | select(.reason == "%s")`, upgrade.ReasonUpgradeFailed))
}

func TestModelControllerController_UpgradeAction_DeleteFailure_SetsCondition(t *testing.T) {
	upgradeFuncs := interceptor.Funcs{
		Delete: func(
			_ context.Context,
			_ client.WithWatch,
			obj client.Object,
			_ ...client.DeleteOption,
		) error {
			if obj.GetName() == "odh-model-controller" {
				return errors.New("simulated delete failure")
			}
			return nil
		},
	}

	tc, appNs := setupEnv(t, upgradeFuncs)
	g := tc.NewWithT(t)

	// Create legacy deployment without the platform label
	legacyDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odh-model-controller",
			Namespace: appNs,
			Labels: map[string]string{
				"app": "odh-model-controller",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "odh-model-controller"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "odh-model-controller"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "controller",
						Image: "quay.io/opendatahub/odh-model-controller:latest",
					}},
				},
			},
		},
	}
	g.Expect(tc.Client().Create(tc.Context(), legacyDeployment)).Should(Succeed())

	// Create ModelController CR
	modelControllerObj := resources.GvkToUnstructured(gvk.ModelController)
	modelControllerObj.SetName(componentApi.ModelControllerInstanceName)
	g.Create(modelControllerObj).Eventually().Should(Succeed())

	// Verify UpgradeFailed condition is set due to delete failure
	g.Get(gvk.ModelController, client.ObjectKeyFromObject(modelControllerObj)).
		Eventually().
		Should(jq.Match(`[.status.conditions[] | select(.reason == "%s")] | length > 0`, upgrade.ReasonUpgradeFailed))
}
