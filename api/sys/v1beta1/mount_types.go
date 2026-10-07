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

type MountConfig struct {
	// +optional
	DefaultLeaseTTL *string `json:"defaultLeaseTTL,omitempty"`

	// +optional
	MaxLeaseTTL *string `json:"maxLeaseTTL,omitempty"`

	// +optional
	ForceNoCache bool `json:"forceNoCache,omitempty"`

	// +optional
	AuditNonHMACRequestKeys []string `json:"auditNonHmacRequestKeys,omitempty"`

	// +optional
	AuditNonHMACResponseKeys []string `json:"auditNonHmacResponseKeys,omitempty"`

	// +kubebuilder:validation:Enum=unauth;hidden
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

// MountSpec defines the desired state of Mount
type MountSpec struct {
	// type defines the type of the secret engine, such as "database" or "kv".
	// +required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	Type *string `json:"type,omitempty"`

	// description defines the human-friendly description of the mount.
	// +optional
	// +kubebuilder:default=""
	Description *string `json:"description,omitempty"`

	// config defines the tunable options of the mount. It is pushed to Vault on
	// every reconciliation, unlike the fields above which are set at creation.
	// +optional
	Config MountConfig `json:"config,omitzero"`

	// local when set, the secret engine is a local mount only. Local mounts are
	// not replicated nor, if a secondary, removed by replication.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="local is immutable"
	Local bool `json:"local,omitempty"`

	// sealWrap enables seal wrapping for the mount, causing values stored by the
	// mount to be wrapped by the seal's encryption capability.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="sealWrap is immutable"
	SealWrap bool `json:"sealWrap,omitempty"`

	// externalEntropyAccess allows the secret engine to access Vault's external entropy source.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="externalEntropyAccess is immutable"
	ExternalEntropyAccess bool `json:"externalEntropyAccess,omitempty"`

	// options defines mount type specific options that are passed to the backend.
	// +optional
	Options map[string]string `json:"options,omitempty"`
}

// MountStatus defines the observed state of Mount.
type MountStatus struct {
	// observedGeneration is the .metadata.generation the status was last reconciled against.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions represent the current state of the Mount resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	Accessor   string             `json:"accessor,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Accessor",type=string,priority=1,JSONPath=`.status.accessor`
// +kubebuilder:printcolumn:name="Message",type=string,priority=1,JSONPath=`.status.conditions[?(@.type=="Ready")].message`

// Mount is the Schema for the mounts API
type Mount struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Mount
	// +required
	Spec MountSpec `json:"spec"`

	// status defines the observed state of Mount
	// +optional
	Status MountStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// MountList contains a list of Mount
type MountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Mount `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Mount{}, &MountList{})
		return nil
	})
}
