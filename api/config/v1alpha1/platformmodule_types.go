/*
Copyright 2026.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
)

const (
	PlatformModuleKind = "PlatformModule"
)

var _ common.PlatformObject = (*PlatformModule)(nil)

// PlatformModuleSpec is intentionally empty. The CR name (metadata.name)
// is the module name — it matches the handler's GetName() and the
// PlatformModules struct field.
type PlatformModuleSpec struct{}

// ResourceRef identifies a Kubernetes resource deployed by the module operator.
// Used to track installed resources for per-reconcile drift cleanup.
// The Group/Version/Kind fields mirror schema.GroupVersionKind but carry
// explicit JSON tags required by controller-gen for CRD schema generation.
// +kubebuilder:object:generate=true
type ResourceRef struct {
	// +optional
	Group string `json:"group,omitempty"`
	// +required
	Version string `json:"version"`
	// +required
	Kind string `json:"kind"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +required
	Name string `json:"name"`
}

// GroupVersionKind returns the schema.GroupVersionKind for this ResourceRef.
func (r ResourceRef) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

// PlatformModuleStatus defines the observed state of a PlatformModule.
type PlatformModuleStatus struct {
	common.Status `json:",inline"`

	// Release records the platform version that was last successfully deployed
	// by this reconciler. It is set only after the operator deployment completes
	// so the DAG readiness checker can verify the version handshake — mirroring
	// the mechanism used by module operators in their own CR status.
	// +optional
	Release common.Release `json:"release,omitempty"`

	// Resources lists every resource deployed by the PlatformModule reconciler.
	// Used for drift cleanup: resources present here but absent from the current
	// render are deleted on the next reconcile.
	// +optional
	Resources []ResourceRef `json:"resources,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:resource:scope=Cluster,shortName=odpm
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`,description="Ready"
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`,description="Reason"

// PlatformModule is the tracker CR for a single module operator managed by
// the Platform controller. One PlatformModule CR exists per enabled module;
// its name matches the module handler name (e.g. "aigateway", "monitoring").
// The Platform controller creates and deletes PlatformModule CRs based on
// Platform.Spec.Modules; the PlatformModule reconciler deploys the operator.
type PlatformModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlatformModuleSpec   `json:"spec,omitempty"`
	Status PlatformModuleStatus `json:"status,omitempty"`
}

func (p *PlatformModule) GetStatus() *common.Status {
	return &p.Status.Status
}

func (p *PlatformModule) GetConditions() []common.Condition {
	return p.Status.GetConditions()
}

func (p *PlatformModule) SetConditions(conditions []common.Condition) {
	p.Status.SetConditions(conditions)
}

// +kubebuilder:object:root=true

// PlatformModuleList contains a list of PlatformModule.
type PlatformModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlatformModule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PlatformModule{}, &PlatformModuleList{})
}
