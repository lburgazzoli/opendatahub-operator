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

package gateway

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	serviceApi "github.com/opendatahub-io/opendatahub-operator/v2/api/services/v1alpha1"
)

// migrateIngressMode preserves LoadBalancer ingressMode for existing 3.x Gateway deployments.
// If the Gateway service exists with type LoadBalancer but GatewayConfig.spec.ingressMode is empty,
// we set it to LoadBalancer to preserve the existing configuration.
func migrateIngressMode(ctx context.Context, cli client.Client) error {
	gatewayConfig := &serviceApi.GatewayConfig{}
	err := cli.Get(ctx, client.ObjectKey{Name: serviceApi.GatewayConfigName}, gatewayConfig)
	switch {
	case k8serr.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("failed to get GatewayConfig: %w", err)
	}

	if gatewayConfig.Spec.IngressMode != "" {
		return nil
	}

	gatewayService := &corev1.Service{}
	err = cli.Get(ctx, client.ObjectKey{
		Name:      GatewayServiceFullName,
		Namespace: GatewayNamespace,
	}, gatewayService)
	switch {
	case k8serr.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("failed to get gateway service: %w", err)
	}

	if gatewayService.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}

	gatewayConfig.Spec.IngressMode = serviceApi.IngressModeLoadBalancer
	if err := cli.Update(ctx, gatewayConfig); err != nil {
		return fmt.Errorf("failed to update GatewayConfig: %w", err)
	}
	return nil
}
