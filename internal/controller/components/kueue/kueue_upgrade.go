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

package kueue

import (
	"context"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cleanupDeprecatedVAPB deletes the deprecated ValidatingAdmissionPolicyBinding
// that was used in earlier versions of Kueue.
func cleanupDeprecatedVAPB(ctx context.Context, cli client.Client) error {
	vapb := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}
	key := client.ObjectKey{Name: "kueue-validating-admission-policy-binding"}

	err := cli.Get(ctx, key, vapb)
	switch {
	case k8serr.IsNotFound(err), meta.IsNoMatchError(err):
		return nil
	case err != nil:
		return fmt.Errorf("failed to get VAPB: %w", err)
	}

	err = cli.Delete(ctx, vapb, client.PropagationPolicy(metav1.DeletePropagationForeground))
	switch {
	case k8serr.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("failed to delete VAPB: %w", err)
	}
	return nil
}
