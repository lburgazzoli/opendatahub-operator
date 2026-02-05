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

package datasciencecluster

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	pkgupgrade "github.com/opendatahub-io/opendatahub-operator/v2/pkg/upgrade"
)

// cleanupDeprecatedRoleBindings removes the deprecated RoleBinding that has the same name as the applications namespace.
func cleanupDeprecatedRoleBindings(ctx context.Context, cli client.Client) error {
	appNs, err := cluster.ApplicationNamespace(ctx, cli)
	switch {
	case k8serr.IsNotFound(err):
		return nil
	case err != nil:
		return err
	}

	rb := &rbacv1.RoleBinding{}
	rb.Name = appNs
	rb.Namespace = appNs

	err = cli.Delete(ctx, rb, client.PropagationPolicy(metav1.DeletePropagationForeground))
	switch {
	case k8serr.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("failed to delete deprecated RoleBinding: %w", err)
	}
	return nil
}

// migrateHardwareProfiles migrates AcceleratorProfiles and container sizes to HardwareProfiles.
func migrateHardwareProfiles(ctx context.Context, cli client.Client) error {
	appNs, err := cluster.ApplicationNamespace(ctx, cli)
	switch {
	case k8serr.IsNotFound(err):
		return nil
	case err != nil:
		return err
	}

	if appNs == "" {
		return nil
	}

	return pkgupgrade.MigrateToInfraHardwareProfiles(ctx, cli, appNs)
}
