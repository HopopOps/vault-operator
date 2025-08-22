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

// KubernetesRoleSpec defines the desired state of KubernetesRole
type KubernetesRoleSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// boundServiceAccountNames defines the list of service account names able to access this role. If set to "*" all names are allowed.
	// +required
	BoundServiceAccountNames []string `json:"boundServiceAccountNames"`

	// boundServiceAccountNamespaces defines the list of namespaces allowed to access this role. If set to "*" all namespaces are allowed.
	// +optional
	BoundServiceAccountNamespaces []string `json:"boundServiceAccountNamespaces,omitempty"`

	// audience defines the audience claim to verify in the JWT. Will be required in Vault 1.21+.
	// +optional
	Audience string `json:"audience,omitempty"`

	// aliasNameSource configures how identity aliases are generated. Valid choices are `serviceaccount_uid` and `serviceaccount_name`.
	// +kubebuilder:validation:Enum=serviceaccount_uid;serviceaccount_name
	// +kubebuilder:default="serviceaccount_uid"
	// +optional
	AliasNameSource string `json:"aliasNameSource,omitempty"`

	// tokenTTL defines the incremental lifetime for generated tokens. This current value of this will be referenced at renewal time.
	// +kubebuilder:validation:Minimum=0
	// +optional
	TokenTTL int `json:"tokenTTL,omitempty"`

	// tokenMaxTTL defines the maximum lifetime for generated tokens. This current value of this will be referenced at renewal time.
	// +kubebuilder:validation:Minimum=0
	// +optional
	TokenMaxTTL int `json:"tokenMaxTTL,omitempty"`

	// tokenPolicies defines the list of token policies to encode onto generated tokens. Depending on the auth method, this list may be supplemented by user/group/other values.
	// +optional
	TokenPolicies []string `json:"tokenPolicies,omitempty"`

	// tokenBoundCIDRs defines the list of CIDR blocks; if set, specifies blocks of IP addresses which can authenticate successfully, and ties the resulting token to these blocks as well.
	// +optional
	TokenBoundCIDRs []string `json:"tokenBoundCIDRs,omitempty"`

	// tokenExplicitMaxTTL if set, will encode an explicit max TTL onto the token. This is a hard cap even if tokenTTL and tokenMaxTTL would otherwise allow a renewal.
	// +kubebuilder:validation:Minimum=0
	// +optional
	TokenExplicitMaxTTL int `json:"tokenExplicitMaxTTL,omitempty"`

	// tokenNoDefaultPolicy if set, the default policy will not be set on generated tokens; otherwise it will be added to the policies set in tokenPolicies.
	// +optional
	TokenNoDefaultPolicy bool `json:"tokenNoDefaultPolicy,omitempty"`

	// tokenNumUses defines the maximum number of times a generated token may be used (within its lifetime); 0 means unlimited. If you require the token to have the ability to create child tokens, you will need to set this value to 0.
	// +kubebuilder:validation:Minimum=0
	// +optional
	TokenNumUses int `json:"tokenNumUses,omitempty"`

	// tokenPeriod defines the maximum allowed period value when a periodic token is requested from this role.
	// +optional
	TokenPeriod int `json:"tokenPeriod,omitempty"`

	// tokenType defines the type of token that should be generated. Can be service, batch, or default to use the mount's tuned default.
	// +kubebuilder:validation:Enum=service;batch;default;default-service;default-batch
	// +optional
	TokenType string `json:"tokenType,omitempty"`

	// authPath defines the remote path in Vault where the auth method is enabled.
	// +kubebuilder:default="kubernetes"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="AuthPath is immutable"
	// +optional
	AuthPath string `json:"authPath,omitempty"`
}

// KubernetesRoleStatus defines the observed state of KubernetesRole.
type KubernetesRoleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// KubernetesRole is the Schema for the kubernetesroles API
type KubernetesRole struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of KubernetesRole
	// +required
	Spec KubernetesRoleSpec `json:"spec"`

	// status defines the observed state of KubernetesRole
	// +optional
	Status KubernetesRoleStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// KubernetesRoleList contains a list of KubernetesRole
type KubernetesRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesRole `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubernetesRole{}, &KubernetesRoleList{})
}
