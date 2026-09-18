package v3

import (
	"fmt"

	operatorv1 "github.com/openshift/api/operator/v1"
)

// MaaSV2StateAnnotation records whether the deprecated v2 field was Managed.
// It is conversion-owned metadata, not user configuration.
const MaaSV2StateAnnotation = "conversion.opendatahub.io/maas-v2-state"

const legacyMaaSManaged = "legacy-managed"

// PreserveMaaSV2State derives provenance from the visible v2 field, never from
// incoming annotations. Removed and empty values clear previous provenance.
func (dsc *DataScienceCluster) PreserveMaaSV2State(legacy operatorv1.ManagementState) {
	delete(dsc.Annotations, MaaSV2StateAnnotation)

	if legacy != operatorv1.Managed {
		return
	}

	if dsc.Annotations == nil {
		dsc.Annotations = make(map[string]string)
	}

	dsc.Annotations[MaaSV2StateAnnotation] = legacyMaaSManaged
}

// ReadMaaSV2State reports legacy Managed provenance and rejects unknown markers.
func (dsc *DataScienceCluster) ReadMaaSV2State() (bool, error) {
	value, found := dsc.Annotations[MaaSV2StateAnnotation]
	if !found {
		return false, nil
	}

	if value != legacyMaaSManaged {
		return false, fmt.Errorf("invalid MaaS v2 provenance marker %q", value)
	}

	return true, nil
}

// AdmitMaaSV2State preserves authoritative old provenance across unrelated
// updates. Native creates and MaaS/parent edits retire legacy configuration.
func (dsc *DataScienceCluster) AdmitMaaSV2State(old *DataScienceCluster) error {
	if old == nil {
		dsc.PreserveMaaSV2State(operatorv1.Removed)

		return nil
	}

	managed, err := old.ReadMaaSV2State()
	if err != nil {
		return err
	}

	legacy := operatorv1.Removed
	if managed && old.maasManagementStates() == dsc.maasManagementStates() {
		legacy = operatorv1.Managed
	}

	dsc.PreserveMaaSV2State(legacy)

	return nil
}

func (dsc *DataScienceCluster) maasManagementStates() [3]operatorv1.ManagementState {
	return [3]operatorv1.ManagementState{
		dsc.Spec.Components.Kserve.ManagementState,
		dsc.Spec.Components.AIGateway.ManagementState,
		dsc.Spec.Components.AIGateway.ModelsAsAService.ManagementState,
	}
}
