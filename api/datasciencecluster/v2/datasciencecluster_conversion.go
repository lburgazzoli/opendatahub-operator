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

package v2

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	dscv3 "github.com/opendatahub-io/opendatahub-operator/v2/api/datasciencecluster/v3"
)

// ConvertTo copies the complete DSC object into the other served version.
// The initial v2 and v3 wire contracts are identical; fields are mapped explicitly
// so later accepted API differences have a reviewable conversion point.
func (c *DataScienceCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*dscv3.DataScienceCluster)
	src := c.DeepCopy()

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = dscv3.DataScienceClusterSpec{
		Components: dscv3.Components{
			Dashboard:            src.Spec.Components.Dashboard,
			Workbenches:          src.Spec.Components.Workbenches,
			AIPipelines:          src.Spec.Components.AIPipelines,
			Kserve:               src.Spec.Components.Kserve,
			Kueue:                src.Spec.Components.Kueue,
			Ray:                  src.Spec.Components.Ray,
			TrustyAI:             src.Spec.Components.TrustyAI,
			ModelRegistry:        src.Spec.Components.ModelRegistry,
			TrainingOperator:     src.Spec.Components.TrainingOperator,
			FeastOperator:        src.Spec.Components.FeastOperator,
			LlamaStackOperator:   src.Spec.Components.LlamaStackOperator,
			OGX:                  src.Spec.Components.OGX,
			MLflowOperator:       src.Spec.Components.MLflowOperator,
			Trainer:              src.Spec.Components.Trainer,
			SparkOperator:        src.Spec.Components.SparkOperator,
			AIGateway:            src.Spec.Components.AIGateway,
			MCPLifecycleOperator: src.Spec.Components.MCPLifecycleOperator,
		},
	}
	dst.Status = dscv3.DataScienceClusterStatus{
		Status:         src.Status.Status,
		RelatedObjects: src.Status.RelatedObjects,
		ErrorMessage:   src.Status.ErrorMessage,
		Components: dscv3.ComponentsStatus{
			Dashboard:            src.Status.Components.Dashboard,
			MaaSConsumerPortal:   src.Status.Components.MaaSConsumerPortal,
			Workbenches:          src.Status.Components.Workbenches,
			WorkbenchesV2:        src.Status.Components.WorkbenchesV2,
			AIPipelines:          src.Status.Components.AIPipelines,
			Kserve:               src.Status.Components.Kserve,
			Kueue:                src.Status.Components.Kueue,
			Ray:                  src.Status.Components.Ray,
			TrustyAI:             src.Status.Components.TrustyAI,
			ModelRegistry:        src.Status.Components.ModelRegistry,
			TrainingOperator:     src.Status.Components.TrainingOperator,
			FeastOperator:        src.Status.Components.FeastOperator,
			LlamaStackOperator:   src.Status.Components.LlamaStackOperator,
			OGX:                  src.Status.Components.OGX,
			MLflowOperator:       src.Status.Components.MLflowOperator,
			Trainer:              src.Status.Components.Trainer,
			SparkOperator:        src.Status.Components.SparkOperator,
			AIGateway:            src.Status.Components.AIGateway,
			ModelsAsAService:     src.Status.Components.ModelsAsAService,
			BatchGateway:         src.Status.Components.BatchGateway,
			MCPLifecycleOperator: src.Status.Components.MCPLifecycleOperator,
		},
		Release: src.Status.Release,
	}

	return nil
}

// ConvertFrom copies the complete DSC object into the other served version.
// The initial v2 and v3 wire contracts are identical; fields are mapped explicitly
// so later accepted API differences have a reviewable conversion point.
func (c *DataScienceCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*dscv3.DataScienceCluster).DeepCopy()
	dst := c

	dst.ObjectMeta = src.ObjectMeta
	dst.Spec = DataScienceClusterSpec{
		Components: Components{
			Dashboard:            src.Spec.Components.Dashboard,
			Workbenches:          src.Spec.Components.Workbenches,
			AIPipelines:          src.Spec.Components.AIPipelines,
			Kserve:               src.Spec.Components.Kserve,
			Kueue:                src.Spec.Components.Kueue,
			Ray:                  src.Spec.Components.Ray,
			TrustyAI:             src.Spec.Components.TrustyAI,
			ModelRegistry:        src.Spec.Components.ModelRegistry,
			TrainingOperator:     src.Spec.Components.TrainingOperator,
			FeastOperator:        src.Spec.Components.FeastOperator,
			LlamaStackOperator:   src.Spec.Components.LlamaStackOperator,
			OGX:                  src.Spec.Components.OGX,
			MLflowOperator:       src.Spec.Components.MLflowOperator,
			Trainer:              src.Spec.Components.Trainer,
			SparkOperator:        src.Spec.Components.SparkOperator,
			AIGateway:            src.Spec.Components.AIGateway,
			MCPLifecycleOperator: src.Spec.Components.MCPLifecycleOperator,
		},
	}
	dst.Status = DataScienceClusterStatus{
		Status:         src.Status.Status,
		RelatedObjects: src.Status.RelatedObjects,
		ErrorMessage:   src.Status.ErrorMessage,
		Components: ComponentsStatus{
			Dashboard:            src.Status.Components.Dashboard,
			MaaSConsumerPortal:   src.Status.Components.MaaSConsumerPortal,
			Workbenches:          src.Status.Components.Workbenches,
			WorkbenchesV2:        src.Status.Components.WorkbenchesV2,
			AIPipelines:          src.Status.Components.AIPipelines,
			Kserve:               src.Status.Components.Kserve,
			Kueue:                src.Status.Components.Kueue,
			Ray:                  src.Status.Components.Ray,
			TrustyAI:             src.Status.Components.TrustyAI,
			ModelRegistry:        src.Status.Components.ModelRegistry,
			TrainingOperator:     src.Status.Components.TrainingOperator,
			FeastOperator:        src.Status.Components.FeastOperator,
			LlamaStackOperator:   src.Status.Components.LlamaStackOperator,
			OGX:                  src.Status.Components.OGX,
			MLflowOperator:       src.Status.Components.MLflowOperator,
			Trainer:              src.Status.Components.Trainer,
			SparkOperator:        src.Status.Components.SparkOperator,
			AIGateway:            src.Status.Components.AIGateway,
			ModelsAsAService:     src.Status.Components.ModelsAsAService,
			BatchGateway:         src.Status.Components.BatchGateway,
			MCPLifecycleOperator: src.Status.Components.MCPLifecycleOperator,
		},
		Release: src.Status.Release,
	}

	return nil
}
