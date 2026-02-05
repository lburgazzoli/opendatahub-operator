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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/fakeclient"

	. "github.com/onsi/gomega"
)

func TestMigrateIngressMode_SetsLoadBalancer(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: GatewayNamespace},
	}

	gatewayConfig := &serviceApi.GatewayConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceApi.GatewayConfigName,
		},
		Spec: serviceApi.GatewayConfigSpec{},
	}

	gatewayService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceFullName,
			Namespace: GatewayNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(ns, gatewayConfig, gatewayService))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = migrateIngressMode(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())

	var updatedConfig serviceApi.GatewayConfig
	err = cli.Get(ctx, client.ObjectKey{Name: serviceApi.GatewayConfigName}, &updatedConfig)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(updatedConfig.Spec.IngressMode).To(Equal(serviceApi.IngressModeLoadBalancer))
}

func TestMigrateIngressMode_SkipsIfAlreadySet(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: GatewayNamespace},
	}

	gatewayConfig := &serviceApi.GatewayConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceApi.GatewayConfigName,
		},
		Spec: serviceApi.GatewayConfigSpec{
			IngressMode: serviceApi.IngressModeOcpRoute,
		},
	}

	gatewayService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceFullName,
			Namespace: GatewayNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(ns, gatewayConfig, gatewayService))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = migrateIngressMode(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())

	var updatedConfig serviceApi.GatewayConfig
	err = cli.Get(ctx, client.ObjectKey{Name: serviceApi.GatewayConfigName}, &updatedConfig)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(updatedConfig.Spec.IngressMode).To(Equal(serviceApi.IngressModeOcpRoute))
}

func TestMigrateIngressMode_SkipsIfServiceNotFound(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: GatewayNamespace},
	}

	gatewayConfig := &serviceApi.GatewayConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceApi.GatewayConfigName,
		},
		Spec: serviceApi.GatewayConfigSpec{},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(ns, gatewayConfig))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = migrateIngressMode(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())

	var updatedConfig serviceApi.GatewayConfig
	err = cli.Get(ctx, client.ObjectKey{Name: serviceApi.GatewayConfigName}, &updatedConfig)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(updatedConfig.Spec.IngressMode).To(BeEmpty())
}

func TestMigrateIngressMode_SkipsIfNotLoadBalancer(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: GatewayNamespace},
	}

	gatewayConfig := &serviceApi.GatewayConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceApi.GatewayConfigName,
		},
		Spec: serviceApi.GatewayConfigSpec{},
	}

	gatewayService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceFullName,
			Namespace: GatewayNamespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	cli, err := fakeclient.New(fakeclient.WithObjects(ns, gatewayConfig, gatewayService))
	g.Expect(err).ShouldNot(HaveOccurred())

	err = migrateIngressMode(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())

	var updatedConfig serviceApi.GatewayConfig
	err = cli.Get(ctx, client.ObjectKey{Name: serviceApi.GatewayConfigName}, &updatedConfig)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(updatedConfig.Spec.IngressMode).To(BeEmpty())
}

func TestMigrateIngressMode_SkipsIfGatewayConfigNotFound(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	cli, err := fakeclient.New()
	g.Expect(err).ShouldNot(HaveOccurred())

	err = migrateIngressMode(ctx, cli)
	g.Expect(err).ShouldNot(HaveOccurred())
}
