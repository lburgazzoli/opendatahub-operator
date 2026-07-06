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
	"sort"

	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/opendatahub-io/opendatahub-operator/v2/api/common"
)

const (
	PlatformKind         = "Platform"
	PlatformInstanceName = "default"
)

var _ common.PlatformObject = (*Platform)(nil)

// PlatformSpec defines the desired state of Platform.
type PlatformSpec struct {
	// Modules declares the low-level desired inventory managed by this Platform
	// instance.
	// +optional
	Modules PlatformModules `json:"modules,omitempty"`
}

// PlatformModuleConfig describes one desired Platform inventory entry.
// +kubebuilder:object:generate=true
type PlatformModuleConfig struct {
	// Name is the canonical inventory entry name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// ManagementState declares whether this entry is actively managed.
	// +kubebuilder:default=Removed
	// +kubebuilder:validation:Enum=Managed;Removed
	ManagementState operatorv1.ManagementState `json:"managementState,omitempty"`

	// Config carries optional low-level opaque configuration reserved for
	// future direct Platform consumers.
	// +optional
	Config *runtime.RawExtension `json:"config,omitempty"`
}

func (c PlatformModuleConfig) IsManaged() bool {
	return c.ManagementState == operatorv1.Managed
}

// PlatformModules is the keyed list of desired Platform inventory entries.
// +listType=map
// +listMapKey=name
// +kubebuilder:object:generate=true
type PlatformModules []PlatformModuleConfig

// PlatformModuleConditionSummary is the compact status payload reported per
// inventory entry.
// +kubebuilder:object:generate=true
type PlatformModuleConditionSummary struct {
	Ready   bool   `json:"ready"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// PlatformModuleSummary reports the aggregated observed state of one inventory
// entry.
// +kubebuilder:object:generate=true
type PlatformModuleSummary struct {
	Name string `json:"name"`
	// +optional
	Runlevel int32 `json:"runlevel,omitempty"`
	// +optional
	Version string                         `json:"version,omitempty"`
	Status  PlatformModuleConditionSummary `json:"status"`
}

// PlatformModuleSummaries is the keyed list of aggregated inventory statuses.
// +listType=map
// +listMapKey=name
// +kubebuilder:object:generate=true
type PlatformModuleSummaries []PlatformModuleSummary

// PlatformStatus defines the observed state of Platform.
type PlatformStatus struct {
	common.Status `json:",inline"`
	// Modules reports a compact summary for each declared Platform inventory
	// entry.
	// +optional
	Modules PlatformModuleSummaries `json:"modules,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
// +kubebuilder:resource:scope=Cluster,shortName=odhp
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'default'",message="Platform name must be default"
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`,description="Ready"
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`,description="Reason"

// Platform is the Schema for the platforms API. It serves as the primary
// reconcile trigger for the module reconciler on clusters where
// DataScienceCluster is not installed (xKS / vanilla Kubernetes).
type Platform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlatformSpec   `json:"spec,omitempty"`
	Status PlatformStatus `json:"status,omitempty"`
}

func (p *Platform) GetStatus() common.Status {
	if copied := p.Status.Status.DeepCopy(); copied != nil {
		return *copied
	}
	return common.Status{}
}

func (p *Platform) SetStatus(status common.Status) {
	if copied := status.DeepCopy(); copied != nil {
		p.Status.Status = *copied
		return
	}
	p.Status.Status = common.Status{}
}

func (p *Platform) GetConditions() []common.Condition {
	return p.Status.GetConditions()
}

func (p *Platform) SetConditions(conditions []common.Condition) {
	p.Status.SetConditions(conditions)
}

//+kubebuilder:object:root=true

// PlatformList contains a list of Platform.
type PlatformList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Platform `json:"items"`
}

// EnabledModules returns the names of entries whose ManagementState is Managed.
func (m PlatformModules) EnabledModules() []string {
	var enabled []string
	for _, entry := range m {
		if entry.ManagementState == operatorv1.Managed {
			enabled = append(enabled, entry.Name)
		}
	}
	sort.Strings(enabled)
	return enabled
}

// AllModuleNames returns all declared entry names in sorted order.
func (m PlatformModules) AllModuleNames() []string {
	names := make([]string, 0, len(m))
	for _, entry := range m {
		if entry.Name == "" {
			continue
		}
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return names
}

// Lookup returns the declared entry for the provided name.
func (m PlatformModules) Lookup(name string) (PlatformModuleConfig, bool) {
	for _, entry := range m {
		if entry.Name == name {
			return entry, true
		}
	}
	return PlatformModuleConfig{}, false
}

// Set inserts or replaces the entry keyed by name and keeps deterministic order.
func (m *PlatformModules) Set(entry PlatformModuleConfig) {
	if m == nil || entry.Name == "" {
		return
	}
	for i := range *m {
		if (*m)[i].Name == entry.Name {
			(*m)[i] = entry
			sort.Slice(*m, func(i int, j int) bool {
				return (*m)[i].Name < (*m)[j].Name
			})
			return
		}
	}
	*m = append(*m, entry)
	sort.Slice(*m, func(i int, j int) bool {
		return (*m)[i].Name < (*m)[j].Name
	})
}

func init() {
	SchemeBuilder.Register(&Platform{}, &PlatformList{})
}
