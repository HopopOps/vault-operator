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
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type AuthConfig struct {
	// +optional
	DefaultLeaseTTL *string `json:"defaultLeaseTTL,omitempty"`

	// +optional
	MaxLeaseTTL *string `json:"maxLeaseTTL,omitempty"`

	// +optional
	AuditNonHMACRequestKeys []string `json:"auditNonHmacRequestKeys,omitempty"`

	// +optional
	AuditNonHMACResponseKeys []string `json:"auditNonHmacResponseKeys,omitempty"`

	// +kubebuilder:default="hidden"
	// +optional
	ListingVisibility *string `json:"listingVisibility,omitempty"`

	// +optional
	PassthroughRequestHeaders []string `json:"passthroughRequestHeaders,omitempty"`

	// +optional
	AllowedResponseHeaders []string `json:"allowedResponseHeaders,omitempty"`

	// +optional
	PluginVersion *string `json:"pluginVersion,omitempty"`

	// +optional
	IdentityTokenKey *string `json:"identityTokenKey,omitempty"`
}

// AuthSpec defines the desired state of Auth
type AuthSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// +optional
	// +kubebuilder:default=""
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Description is immutable"
	Description *string `json:"description,omitempty"`

	// +kubebuilder:default="kubernetes"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Type is immutable"
	Type *string `json:"type,omitempty"`

	//// +kubebuilder:default=false
	//// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Local is immutable"
	//// +optional
	//Local bool `json:"local,omitempty"`
	//
	//// +kubebuilder:default=false
	//// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="SealWrap is immutable"
	//// +optional
	//SealWrap bool `json:"sealWrap,omitempty"`
	//
	//// +optional
	//Config AuthConfig `json:"config,omitempty"`
}

// AuthStatus defines the observed state of Auth.
type AuthStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Auth is the Schema for the auths API
type Auth struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Auth
	// +required
	Spec AuthSpec `json:"spec"`

	// status defines the observed state of Auth
	// +optional
	Status AuthStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// AuthList contains a list of Auth
type AuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Auth `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Auth{}, &AuthList{})
}
