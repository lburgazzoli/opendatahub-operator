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
package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	dsciv2 "github.com/opendatahub-io/opendatahub-operator/v2/api/dscinitialization/v2"
	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
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

// registerController registers the Gateway controller with interceptor functions for fault injection.
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

	handler := &ServiceHandler{}
	return handler.NewReconciler(ctx, mgr, reconciler.WithDirectClient(wrappedCli))
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
	)

	t.Cleanup(teardown)

	tc, err := testf.NewTestContext(
		testf.WithClient(env.Client()),
		testf.WithContext(ctx),
	)
	g.Expect(err).NotTo(HaveOccurred(), "failed to create test context")

	// Create a default DSCI - required for ApplicationsNamespace lookup
	dsci := &dsciv2.DSCInitialization{
		ObjectMeta: metav1.ObjectMeta{Name: "default-dsci"},
		Spec:       dsciv2.DSCInitializationSpec{ApplicationsNamespace: "opendatahub"},
	}
	g.Expect(env.Client().Create(ctx, dsci)).To(Succeed(), "failed to create DSCI")

	// Create the gateway namespace (openshift-ingress)
	gwNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: GatewayNamespace,
		},
	}
	g.Expect(env.Client().Create(ctx, gwNs)).To(Succeed(), "failed to create gateway namespace")

	return tc
}

func TestGatewayController_UpgradeAction_MigratesIngressMode(t *testing.T) {
	tc := setupEnv(t, interceptor.Funcs{})
	g := tc.NewWithT(t)

	// Create the gateway service with LoadBalancer type
	gatewayService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceFullName,
			Namespace: GatewayNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{
				Name: "https",
				Port: 443,
			}},
			Selector: map[string]string{
				"app": "data-science-gateway",
			},
		},
	}
	g.Expect(tc.Client().Create(tc.Context(), gatewayService)).Should(Succeed())

	// Create GatewayConfig with empty ingressMode
	gatewayConfigObj := resources.GvkToUnstructured(gvk.GatewayConfig)
	gatewayConfigObj.SetName(serviceApi.GatewayConfigName)
	g.Create(gatewayConfigObj).Eventually().Should(Succeed())

	// Verify the ingressMode is set to LoadBalancer after reconciliation
	g.Get(gvk.GatewayConfig, client.ObjectKey{Name: serviceApi.GatewayConfigName}).
		Eventually().
		Should(jq.Match(`.spec.ingressMode == "%s"`, serviceApi.IngressModeLoadBalancer))
}

func TestGatewayController_UpgradeAction_SkipsIfAlreadySet(t *testing.T) {
	tc := setupEnv(t, interceptor.Funcs{})
	g := tc.NewWithT(t)

	// Create the gateway service with LoadBalancer type
	gatewayService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceFullName,
			Namespace: GatewayNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{
				Name: "https",
				Port: 443,
			}},
			Selector: map[string]string{
				"app": "data-science-gateway",
			},
		},
	}
	g.Expect(tc.Client().Create(tc.Context(), gatewayService)).Should(Succeed())

	// Create GatewayConfig with ingressMode already set to Route
	gatewayConfigObj := resources.GvkToUnstructured(gvk.GatewayConfig)
	gatewayConfigObj.SetName(serviceApi.GatewayConfigName)
	_ = unstructured.SetNestedField(gatewayConfigObj.Object, string(serviceApi.IngressModeOcpRoute), "spec", "ingressMode")
	g.Create(gatewayConfigObj).Eventually().Should(Succeed())

	// Wait for reconciliation to process by checking that conditions are set
	g.Get(gvk.GatewayConfig, client.ObjectKey{Name: serviceApi.GatewayConfigName}).
		Eventually().
		Should(jq.Match(`.status.conditions | length > 0`))

	// Verify the ingressMode is still OcpRoute (not changed to LoadBalancer)
	g.Get(gvk.GatewayConfig, client.ObjectKey{Name: serviceApi.GatewayConfigName}).
		Eventually().
		Should(jq.Match(`.spec.ingressMode == "%s"`, serviceApi.IngressModeOcpRoute))
}

func TestGatewayController_UpgradeAction_HandlesNoService(t *testing.T) {
	tc := setupEnv(t, interceptor.Funcs{})
	g := tc.NewWithT(t)

	// Create GatewayConfig without any pre-existing gateway service
	gatewayConfigObj := resources.GvkToUnstructured(gvk.GatewayConfig)
	gatewayConfigObj.SetName(serviceApi.GatewayConfigName)
	g.Create(gatewayConfigObj).Eventually().Should(Succeed())

	// Verify the controller processes the CR and sets conditions
	// The upgrade action should complete without errors when no service exists
	g.Get(gvk.GatewayConfig, client.ObjectKeyFromObject(gatewayConfigObj)).
		Eventually().
		Should(jq.Match(`.status.conditions | length > 0`))

	// Verify that there's no UpgradeFailed condition
	g.Get(gvk.GatewayConfig, client.ObjectKeyFromObject(gatewayConfigObj)).
		Eventually().
		ShouldNot(jq.Match(`.status.conditions[] | select(.reason == "%s")`, upgrade.ReasonUpgradeFailed))
}

func TestGatewayController_UpgradeAction_UpdateFailure_SetsCondition(t *testing.T) {
	upgradeFuncs := interceptor.Funcs{
		Update: func(
			ctx context.Context,
			c client.WithWatch,
			obj client.Object,
			opts ...client.UpdateOption,
		) error {
			// Check if this is a GatewayConfig update (typed structs don't have GVK in ObjectKind)
			if _, ok := obj.(*serviceApi.GatewayConfig); ok && obj.GetName() == serviceApi.GatewayConfigName {
				return errors.New("simulated update failure")
			}
			// Delegate to the real client for other updates
			return c.Update(ctx, obj, opts...)
		},
	}

	tc := setupEnv(t, upgradeFuncs)
	g := tc.NewWithT(t)

	// Create the gateway service with LoadBalancer type
	gatewayService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceFullName,
			Namespace: GatewayNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{
				Name: "https",
				Port: 443,
			}},
			Selector: map[string]string{
				"app": "data-science-gateway",
			},
		},
	}
	g.Expect(tc.Client().Create(tc.Context(), gatewayService)).Should(Succeed())

	// Create GatewayConfig with empty ingressMode (needs migration)
	gatewayConfigObj := resources.GvkToUnstructured(gvk.GatewayConfig)
	gatewayConfigObj.SetName(serviceApi.GatewayConfigName)
	g.Create(gatewayConfigObj).Eventually().Should(Succeed())

	// Verify UpgradeFailed condition is set due to update failure
	g.Get(gvk.GatewayConfig, client.ObjectKeyFromObject(gatewayConfigObj)).
		Eventually().
		Should(jq.Match(`[.status.conditions[] | select(.reason == "%s")] | length > 0`, upgrade.ReasonUpgradeFailed))
}
