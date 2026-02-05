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

package envtestutil

import (
	"context"
	"fmt"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/envt"
)

// WithKueueCRDs enables mock Kueue CRDs for controller integration tests.
// Prometheus and OLM CRDs are loaded from config/crd/external by envtest.
func WithKueueCRDs() CRDSetupOption {
	return func(ctx context.Context, t *testing.T, env *envt.EnvT) error {
		t.Helper()

		// Kueue-specific types (CRDs created at runtime)
		env.Scheme().AddKnownTypeWithName(gvk.KueueConfigV1, &unstructured.Unstructured{})
		env.Scheme().AddKnownTypeWithName(gvk.LocalQueue, &unstructured.Unstructured{})
		env.Scheme().AddKnownTypeWithName(gvk.ClusterQueue, &unstructured.Unstructured{})
		env.Scheme().AddKnownTypeWithName(gvk.ResourceFlavor, &unstructured.Unstructured{})

		// Only create Kueue-specific CRDs (Prometheus/OLM are loaded from files)
		crds := []*apiextensionsv1.CustomResourceDefinition{
			MockKueueConfigCRD(),
			MockLocalQueueCRD(),
			MockClusterQueueCRD(),
			MockResourceFlavorCRD(),
		}

		for _, crd := range crds {
			if err := createAndWaitForCRD(ctx, env, crd); err != nil {
				return fmt.Errorf("failed to create and wait for %s CRD: %w", crd.Name, err)
			}
		}
		return nil
	}
}

// MockKueueConfigCRD creates a mock Kueue config CRD (kueues.kueue.openshift.io).
func MockKueueConfigCRD() *apiextensionsv1.CustomResourceDefinition {
	preserveUnknownFields := true
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "kueues.kueue.openshift.io"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "kueue.openshift.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "kueues",
				Singular: "kueue",
				Kind:     "Kueue",
			},
			Scope: apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type:                   "object",
						XPreserveUnknownFields: &preserveUnknownFields,
					},
				},
				Subresources: &apiextensionsv1.CustomResourceSubresources{
					Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
				},
			}},
		},
	}
}

// MockLocalQueueCRD creates a mock LocalQueue CRD.
func MockLocalQueueCRD() *apiextensionsv1.CustomResourceDefinition {
	preserveUnknownFields := true
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "localqueues.kueue.x-k8s.io"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "kueue.x-k8s.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "localqueues",
				Singular: "localqueue",
				Kind:     "LocalQueue",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1beta1",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type:                   "object",
						XPreserveUnknownFields: &preserveUnknownFields,
					},
				},
			}},
		},
	}
}

// MockClusterQueueCRD creates a mock ClusterQueue CRD.
func MockClusterQueueCRD() *apiextensionsv1.CustomResourceDefinition {
	preserveUnknownFields := true
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "clusterqueues.kueue.x-k8s.io"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "kueue.x-k8s.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "clusterqueues",
				Singular: "clusterqueue",
				Kind:     "ClusterQueue",
			},
			Scope: apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1beta1",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type:                   "object",
						XPreserveUnknownFields: &preserveUnknownFields,
					},
				},
			}},
		},
	}
}

// MockResourceFlavorCRD creates a mock ResourceFlavor CRD.
func MockResourceFlavorCRD() *apiextensionsv1.CustomResourceDefinition {
	preserveUnknownFields := true
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "resourceflavors.kueue.x-k8s.io"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "kueue.x-k8s.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "resourceflavors",
				Singular: "resourceflavor",
				Kind:     "ResourceFlavor",
			},
			Scope: apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1beta1",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type:                   "object",
						XPreserveUnknownFields: &preserveUnknownFields,
					},
				},
			}},
		},
	}
}
