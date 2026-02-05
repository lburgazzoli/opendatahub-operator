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

package modelcontroller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	componentApi "github.com/opendatahub-io/opendatahub-operator/v2/api/components/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

// cleanupLegacyDeployment deletes the old "odh-model-controller" deployment if it
// doesn't have the new platform label. This is necessary because older versions
// used different label selectors, preventing SSA from updating the deployment.
func cleanupLegacyDeployment(ctx context.Context, cli client.Client) error {
	appNs, err := cluster.ApplicationNamespace(ctx, cli)
	switch {
	case k8serr.IsNotFound(err):
		return nil
	case err != nil:
		return err
	}

	d := appsv1.Deployment{}
	d.Name = "odh-model-controller"
	d.Namespace = appNs

	err = cli.Get(ctx, client.ObjectKeyFromObject(&d), &d)
	switch {
	case k8serr.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("failed to get legacy deployment: %w", err)
	}

	if resources.HasLabel(&d, labels.PlatformPartOf, componentApi.ModelControllerComponentName) {
		return nil
	}

	err = cli.Delete(ctx, &d, client.PropagationPolicy(metav1.DeletePropagationForeground))
	switch {
	case k8serr.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("failed to delete legacy deployment: %w", err)
	}
	return nil
}
