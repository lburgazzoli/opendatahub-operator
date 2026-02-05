// Package upgrade provides functions of upgrade ODH from v1 to v2 and vaiours v2 versions.
// It contains both the logic to upgrade the ODH components and the logic to cleanup the deprecated resources.
package upgrade

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1 "github.com/opendatahub-io/opendatahub-operator/v2/api/infrastructure/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
)

const (
	defaultMinMemory                      = "1Mi"
	defaultMinCpu                         = "1"
	odhDashboardConfigPath                = "/dashboard/rhoai/shared/odhdashboardconfig/odhdashboardconfig.yaml"
	odhDashboardConfigName                = "odh-dashboard-config"
	serving                               = "serving"
	notebooks                             = "notebooks"
	customServing                         = "custom-serving"
	acceleratorNameAnnotation             = "opendatahub.io/accelerator-name"
	lastSizeSelectionAnnotation           = "notebooks.opendatahub.io/last-size-selection"
	hardwareProfileNameAnnotation         = "opendatahub.io/hardware-profile-name"
	hardwareProfileNamespaceAnnotation    = "opendatahub.io/hardware-profile-namespace"
	hardwareProfileManagedAnnotation      = "opendatahub.io/managed"
	hardwareProfileVisibilityAnnotation   = "opendatahub.io/dashboard-feature-visibility"
	hardwareProfileModifiedDateAnnotation = "opendatahub.io/modified-date"
	hardwareProfileDisplayNameAnnotation  = "opendatahub.io/display-name"
	hardwareProfileDescriptionAnnotation  = "opendatahub.io/description"
	hardwareProfileDisabledAnnotation     = "opendatahub.io/disabled"
	featureVisibilityModelServing         = `["model-serving"]`
	featureVisibilityWorkbench            = `["workbench"]`
	containerSizeHWPPrefix                = "containersize-"
)

var defaultResourceLimits = map[string]string{
	"maxMemory": "120Gi",
	"minMemory": "8Gi",
	"maxCpu":    "30",
	"minCpu":    "1",
}

// MigrateToInfraHardwareProfiles orchestrates all HardwareProfile migrations including resource creation and annotation updates.
// This is the parent function that gets OdhDashboardConfig once and calls all child migration functions.
func MigrateToInfraHardwareProfiles(ctx context.Context, cli client.Client, applicationNS string) error {
	var multiErr *multierror.Error
	log := logf.FromContext(ctx)
	// If application namespace is empty, it means dsci is not available or not initialized properly with application namespace.
	// In this case, we skip the HardwareProfile migration.
	if applicationNS == "" {
		log.Info("Application namespace is empty, skipping HardwareProfile migrations")
		return nil
	}

	// Get OdhDashboardConfig to extract container sizes
	odhConfig, found, err := getOdhDashboardConfig(ctx, cli, applicationNS)
	if err != nil {
		return fmt.Errorf("failed to get OdhDashboardConfig: %w", err)
	}
	if !found {
		log.Info("OdhDashboardConfig not found, skipping HardwareProfile migrations")
		return nil
	}

	// 1. Create 2 HardwareProfiles for each AcceleratorProfile (notebooks and serving)
	multiErr = multierror.Append(multiErr, MigrateAcceleratorProfilesToHardwareProfiles(ctx, cli, odhConfig))

	// 2. Create 1 HardwareProfile for each container size (notebook and model server sizes)
	multiErr = multierror.Append(multiErr, MigrateContainerSizesToHardwareProfiles(ctx, cli, applicationNS, odhConfig))

	// 3. Attach HardwareProfile annotations to existing Notebooks
	multiErr = multierror.Append(multiErr, AttachHardwareProfileToNotebooks(ctx, cli, applicationNS, odhConfig))

	// 4. Attach HardwareProfile annotations to existing InferenceServices but create custom-serving HWP first.
	multiErr = multierror.Append(multiErr, createCustomServingHardwareProfile(ctx, cli, applicationNS))
	multiErr = multierror.Append(multiErr, AttachHardwareProfileToInferenceServices(ctx, cli, applicationNS, odhConfig))

	return multiErr.ErrorOrNil()
}

// MigrateAcceleratorProfilesToHardwareProfiles migrates AcceleratorProfiles to HardwareProfiles
// as described in RHOAIENG-33158. This creates 2 HardwareProfiles for each AP (notebooks and serving).
func MigrateAcceleratorProfilesToHardwareProfiles(ctx context.Context, cli client.Client, odhConfig *unstructured.Unstructured) error {
	log := logf.FromContext(ctx)

	apList, err := getAcceleratorProfiles(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to get AcceleratorProfile list: %w", err)
	}
	if len(apList) == 0 {
		log.Info("No AcceleratorProfiles found, skipping migration")
		return nil
	}

	// Get notebooks-only toleration if applicable
	notebooksOnlyToleration, err := getNotebooksOnlyToleration(odhConfig)
	if err != nil {
		return fmt.Errorf("failed to get notebooks-only toleration: %w", err)
	}

	// Calculate container resource limits
	notebookContainerCounts, err := FindContainerCpuMemoryMinMaxCount(odhConfig, "notebookSizes")
	if err != nil {
		return fmt.Errorf("failed to calculate notebook container limits: %w", err)
	}
	// default min limits only for serving HardwareProfile, max limits are not set
	servingContainerCounts := map[string]string{
		"minMemory": "1Gi",
		"minCpu":    "1",
	}

	var multiErr *multierror.Error

	// Create 2 HardwareProfiles for each AcceleratorProfile
	for _, ap := range apList {
		// Create notebooks HardwareProfile
		if err := createHardwareProfileFromAcceleratorProfile(ctx, cli, ap, notebooks, notebookContainerCounts, notebooksOnlyToleration); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to create notebooks HardwareProfile for AP %s: %w", ap.GetName(), err))
			continue
		}

		// Create serving HardwareProfile
		if err := createHardwareProfileFromAcceleratorProfile(ctx, cli, ap, serving, servingContainerCounts, nil); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to create serving HardwareProfile for AP %s: %w", ap.GetName(), err))
			continue
		}
	}

	return multiErr.ErrorOrNil()
}

// MigrateContainerSizesToHardwareProfiles migrates container sizes to HardwareProfiles
// as described in RHOAIENG-33158. This creates 1 HardwareProfile for each container size.
func MigrateContainerSizesToHardwareProfiles(ctx context.Context, cli client.Client, applicationNS string, odhConfig *unstructured.Unstructured) error {
	var multiErr *multierror.Error

	// Get notebooks-only toleration if applicable
	notebooksOnlyToleration, err := getNotebooksOnlyToleration(odhConfig)
	if err != nil {
		return fmt.Errorf("failed to get notebooks-only toleration: %w", err)
	}

	// Create HardwareProfiles for notebook container sizes
	notebookSizes, err := getContainerSizes(odhConfig, "notebookSizes")
	if err != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("failed to get notebook sizes: %w", err))
	}
	if err == nil {
		for _, size := range notebookSizes {
			if err := createHardwareProfileFromContainerSize(ctx, cli, size, notebooks, notebooksOnlyToleration, applicationNS); err != nil {
				multiErr = multierror.Append(multiErr, fmt.Errorf("failed to create HardwareProfile for notebook size %s: %w", size.Name, err))
				continue
			}
		}
	}

	// Create HardwareProfiles for model server container sizes
	modelServerSizes, err := getContainerSizes(odhConfig, "modelServerSizes")
	if err != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("failed to get model server sizes: %w", err))
	}
	if err == nil {
		for _, size := range modelServerSizes {
			if err := createHardwareProfileFromContainerSize(ctx, cli, size, serving, nil, applicationNS); err != nil {
				multiErr = multierror.Append(multiErr, fmt.Errorf("failed to create HardwareProfile for model server size %s: %w", size.Name, err))
				continue
			}
		}
	}

	return multiErr.ErrorOrNil()
}

// AttachHardwareProfileToNotebooks migrates AcceleratorProfile and container size annotations
// on Notebooks to HardwareProfile annotations as described in RHOAIENG-33158.
func AttachHardwareProfileToNotebooks(ctx context.Context, cli client.Client, applicationNS string, odhConfig *unstructured.Unstructured) error {
	log := logf.FromContext(ctx)
	var multiErr *multierror.Error

	notebooks, err := getNotebooks(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to get notebooks: %w", err)
	}

	if len(notebooks) == 0 {
		log.Info("No Notebooks found, skipping annotation migration")
		return nil
	}

	// get the size once for all notebooks.
	containerSizes, err := getContainerSizes(odhConfig, "notebookSizes")
	if err != nil {
		return fmt.Errorf("failed to get container sizes: %w", err)
	}

	for _, notebook := range notebooks {
		// Get annotations once for efficiency
		annotations := notebook.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		// Skip if already has HardwareProfile annotation
		if annotations[hardwareProfileNameAnnotation] != "" {
			continue
		}

		var hwpName string
		var migrationSource string

		// Check for AcceleratorProfile annotation first (higher priority)
		if apName := annotations[acceleratorNameAnnotation]; apName != "" {
			// Convert to lowercase and replace spaces with dashes to comply with the hardwareprofile CRD validation
			hwpName = fmt.Sprintf("%s-notebooks", strings.ReplaceAll(strings.ToLower(apName), " ", "-"))
			migrationSource = "AcceleratorProfile annotation"
		} else if sizeSelection := annotations[lastSizeSelectionAnnotation]; sizeSelection != "" && containerSizeExists(containerSizes, sizeSelection) {
			// Handle container size annotation migration
			// If size doesn't exist in OdhDashboardConfig, leave annotation as-is (per requirements)
			hwpName = fmt.Sprintf("%s%s-notebooks", containerSizeHWPPrefix, strings.ReplaceAll(strings.ToLower(sizeSelection), " ", "-"))
			migrationSource = "container size annotation"
		}

		// Set HardwareProfile annotation if we found a migration source
		if hwpName != "" {
			if err := setHardwareProfileAnnotation(ctx, cli, notebook, hwpName, applicationNS); err != nil {
				multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set HardwareProfile annotation for notebook %s: %w", notebook.GetName(), err))
				continue
			}
			log.Info("Migrated annotation to HardwareProfile for Notebook", "notebook", notebook.GetName(), "migrationSource", migrationSource, "hardwareProfile", hwpName)
		}
	}

	return multiErr.ErrorOrNil()
}

func createCustomServingHardwareProfile(ctx context.Context, cli client.Client, namespace string) error {
	log := logf.FromContext(ctx)
	// Check if custom-serving HardwareProfile CR already exists
	_, customServingError := cluster.GetHardwareProfile(ctx, cli, customServing, namespace)
	if client.IgnoreNotFound(customServingError) != nil {
		return fmt.Errorf("failed to check HardwareProfile CR: %s %w", customServing, customServingError)
	}
	if k8serr.IsNotFound(customServingError) {
		// Create custom-serving HardwareProfile programmatically
		annotations := createHardwareProfileAnnotations(serving, customServing, "", false)
		annotations[hardwareProfileManagedAnnotation] = "false"

		hwp := &infrav1.HardwareProfile{
			TypeMeta: metav1.TypeMeta{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "HardwareProfile",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:        customServing,
				Namespace:   namespace,
				Annotations: annotations,
			},
			Spec: infrav1.HardwareProfileSpec{
				Identifiers: []infrav1.HardwareIdentifier{
					{
						Identifier:   "cpu",
						DisplayName:  "cpu",
						ResourceType: "CPU",
						MinCount:     intstr.FromInt(1),
						DefaultCount: intstr.FromInt(1),
					},
					{
						Identifier:   "memory",
						DisplayName:  "memory",
						ResourceType: "Memory",
						MinCount:     intstr.FromString("1Gi"),
						DefaultCount: intstr.FromString("1Gi"),
					},
				},
			},
		}

		if err := cluster.CreateHardwareProfile(ctx, cli, hwp); err != nil {
			return err
		}
		log.Info("Successfully created HardwareProfile", "name", customServing, "namespace", namespace)
	}
	return nil
}

// AttachHardwareProfileToInferenceServices migrates AcceleratorProfile annotations from ServingRuntimes
// and matches container sizes on InferenceServices to HardwareProfile annotations as described in RHOAIENG-33158.
func AttachHardwareProfileToInferenceServices(ctx context.Context, cli client.Client, applicationNamespace string, odhConfig *unstructured.Unstructured) error {
	log := logf.FromContext(ctx)
	var multiErr *multierror.Error

	inferenceServices, err := getInferenceServices(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to get InferenceServices: %w", err)
	}

	if len(inferenceServices) == 0 {
		log.Info("No InferenceServices found, skipping annotation migration")
		return nil
	}

	// get the size once for all inference services.
	containerSizes, err := getContainerSizes(odhConfig, "modelServerSizes")
	if err != nil {
		return fmt.Errorf("failed to get model server sizes: %w", err)
	}

	for _, isvc := range inferenceServices {
		// Get annotations once for efficiency
		isvcAnnotations := isvc.GetAnnotations()
		if isvcAnnotations == nil {
			isvcAnnotations = map[string]string{}
		}

		// Skip if already has HardwareProfile annotation
		if isvcAnnotations[hardwareProfileNameAnnotation] != "" {
			continue
		}

		// Check ServingRuntime for AcceleratorProfile annotation and apply to InferenceService
		servingRuntime, err := getSRFromISVC(ctx, cli, isvc)
		if err == nil {
			runtimeAnnotations := servingRuntime.GetAnnotations()
			if runtimeAnnotations == nil {
				runtimeAnnotations = map[string]string{}
			}
			if apName := runtimeAnnotations[acceleratorNameAnnotation]; apName != "" {
				hwpName := fmt.Sprintf("%s-serving", strings.ReplaceAll(strings.ToLower(apName), " ", "-"))
				if err := setHardwareProfileAnnotation(ctx, cli, isvc, hwpName, applicationNamespace); err != nil {
					multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set HardwareProfile annotation for InferenceService %s: %w", isvc.GetName(), err))
					continue
				}
				log.Info("Migrated ServingRuntime AP annotation to HardwareProfile annotation for InferenceService",
					"isvc", isvc.GetName(), "runtime", servingRuntime.GetName(), "hwp", hwpName)
				continue
			}
		}

		// No AP found, try container size matching
		// Default usign HWProfile CR "custom-serving", update only if we find a matching size
		hwpName := customServing
		var matchedSize string

		resources, err := getInferenceServiceResources(isvc)
		if err == nil {
			// Try to match resources to a container size
			matchedSize = findContainerSizeByResources(containerSizes, resources)
			if matchedSize != "" {
				hwpName = fmt.Sprintf("%s%s-serving", containerSizeHWPPrefix, strings.ReplaceAll(strings.ToLower(matchedSize), " ", "-"))
			}
		}

		if err := setHardwareProfileAnnotation(ctx, cli, isvc, hwpName, applicationNamespace); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set HardwareProfile annotation for InferenceService %s: %w", isvc.GetName(), err))
		} else {
			// Log after successful annotation setting
			if matchedSize != "" {
				log.Info("Set HardwareProfile annotation for InferenceService based on container size match", "isvc", isvc.GetName(), "size", matchedSize, "hardwareProfile", hwpName)
			} else {
				log.Info("Set HardwareProfile annotation for InferenceService with "+customServing+" HardwareProfile", "isvc", isvc.GetName(), "hardwareProfile", hwpName)
			}
		}
	}

	return multiErr.ErrorOrNil()
}
