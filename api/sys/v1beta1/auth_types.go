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
	Description *string `json:"description,omitempty"`

	// +kubebuilder:default="kubernetes"
	Type *string `json:"type,omitempty"`
}

// AuthStatus defines the observed state of Auth.
type AuthStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// observedGeneration is the .metadata.generation the status was last reconciled against.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Auth resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	Accessor   string             `json:"accessor,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Auth is the Schema for the auths API
type Auth struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Auth
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="AuthSpec is immutable"
	// +required
	Spec AuthSpec `json:"spec"`

	// status defines the observed state of Auth
	// +optional
	Status AuthStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AuthList contains a list of Auth
type AuthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Auth `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Auth{}, &AuthList{})
		return nil
	})
}
