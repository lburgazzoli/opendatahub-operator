package envt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/resources"
)

// groupAnnotations defines annotations to apply to CRDs based on their API group.
// This is useful for protected groups that require specific annotations like
// api-approved.kubernetes.io for Kubernetes API review.
var groupAnnotations = map[string]map[string]string{
	gwapiv1.GroupVersion.Group: {
		"api-approved.kubernetes.io": "https://github.com/kubernetes-sigs/gateway-api/pull/891",
	},
}

// collectYAMLFiles returns all .yaml file paths from the given directories.
func collectYAMLFiles(paths []string) ([]string, error) {
	var files []string

	for _, path := range paths {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory %s: %w", path, err)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
				continue
			}
			files = append(files, filepath.Join(path, entry.Name()))
		}
	}

	return files, nil
}

// LoadCRDsFromPaths loads CRDs from the given paths and patches CRDs in protected
// API groups with required annotations.
func LoadCRDsFromPaths(paths []string) ([]*apiextensionsv1.CustomResourceDefinition, error) {
	crdScheme := runtime.NewScheme()
	if err := apiextensionsv1.AddToScheme(crdScheme); err != nil {
		return nil, fmt.Errorf("failed to add apiextensions to scheme: %w", err)
	}

	files, err := collectYAMLFiles(paths)
	if err != nil {
		return nil, err
	}

	decoder := serializer.NewCodecFactory(crdScheme).UniversalDeserializer()
	var crds []*apiextensionsv1.CustomResourceDefinition

	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CRD file %s: %w", filePath, err)
		}

		obj, _, err := decoder.Decode(content, nil, nil)
		if err != nil {
			continue
		}

		crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
		if !ok {
			continue
		}

		if annotations, ok := groupAnnotations[crd.Spec.Group]; ok {
			resources.SetAnnotations(crd, annotations)
		}

		crds = append(crds, crd)
	}

	return crds, nil
}
