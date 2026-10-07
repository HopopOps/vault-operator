/*
Copyright 2025 HopopOps, Inc..

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RoleSpec defines the desired state of Role
type RoleSpec struct {
	// path defines the mount path of the database secret engine.
	// +required
	Path string `json:"path"`

	// dbName defines the name of the database connection to use for this role.
	// +required
	DBName string `json:"dbName"`

	// creationStatements defines the database statements executed to create and
	// configure a user.
	// +required
	CreationStatements []string `json:"creationStatements"`

	// revocationStatements defines the database statements executed to revoke a user.
	// +optional
	RevocationStatements []string `json:"revocationStatements,omitempty"`

	// rollbackStatements defines the database statements executed to roll back a
	// create operation that failed halfway.
	// +optional
	RollbackStatements []string `json:"rollbackStatements,omitempty"`

	// renewStatements defines the database statements executed to renew a user.
	// +optional
	RenewStatements []string `json:"renewStatements,omitempty"`

	// defaultTTL defines, in seconds, the TTL of the leases associated with this
	// role. Defaults to the system or mount default TTL.
	// +kubebuilder:validation:Minimum=0
	// +optional
	DefaultTTL int `json:"defaultTTL,omitempty"`

	// maxTTL defines, in seconds, the maximum TTL of the leases associated with
	// this role. It may be shorter than the mount max TTL, never longer.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxTTL int `json:"maxTTL,omitempty"`
}

// RoleStatus defines the observed state of Role.
type RoleStatus struct {
	// observedGeneration is the .metadata.generation the status was last reconciled against.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions represent the current state of the Role resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Path",type=string,JSONPath=`.spec.path`
// +kubebuilder:printcolumn:name="Connection",type=string,JSONPath=`.spec.dbName`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Message",type=string,priority=1,JSONPath=`.status.conditions[?(@.type=="Ready")].message`

// Role is the Schema for the roles API
type Role struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Role
	// +required
	Spec RoleSpec `json:"spec"`

	// status defines the observed state of Role
	// +optional
	Status RoleStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// RoleList contains a list of Role
type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Role `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Role{}, &RoleList{})
		return nil
	})
}
