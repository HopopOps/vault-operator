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

// PasswordPolicySpec defines the desired state of PasswordPolicy
type PasswordPolicySpec struct {
	// policy specifies the password policy document, in HCL.
	// See https://developer.hashicorp.com/vault/docs/concepts/password-policies
	// +required
	Policy *string `json:"policy,omitempty"`
}

// PasswordPolicyStatus defines the observed state of PasswordPolicy.
type PasswordPolicyStatus struct {
	// observedGeneration is the .metadata.generation the status was last reconciled against.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions represent the current state of the PasswordPolicy resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Message",type=string,priority=1,JSONPath=`.status.conditions[?(@.type=="Ready")].message`

// PasswordPolicy is the Schema for the passwordpolicies API
type PasswordPolicy struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of PasswordPolicy
	// +required
	Spec PasswordPolicySpec `json:"spec"`

	// status defines the observed state of PasswordPolicy
	// +optional
	Status PasswordPolicyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// PasswordPolicyList contains a list of PasswordPolicy
type PasswordPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []PasswordPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &PasswordPolicy{}, &PasswordPolicyList{})
		return nil
	})
}
