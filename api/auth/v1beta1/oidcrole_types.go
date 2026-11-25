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

// OIDCRoleSpec defines the desired state of OIDCRole
type OIDCRoleSpec struct {
	// authPath defines the remote path in Vault where the oidc auth method is enabled.
	// +kubebuilder:default="oidc"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="AuthPath is immutable"
	// +optional
	AuthPath string `json:"authPath,omitempty"`

	// allowedRedirectURIs defines the redirect URIs allowed for this role, such
	// as https://vault.example.com/ui/vault/auth/oidc/oidc/callback.
	// +kubebuilder:validation:MinItems=1
	// +required
	AllowedRedirectURIs []string `json:"allowedRedirectURIs"`

	// userClaim defines the claim used to uniquely identify the user, and as
	// the name of the identity alias.
	// +required
	UserClaim string `json:"userClaim"`

	// groupsClaim defines the claim holding the groups the user belongs to,
	// used to create identity group aliases.
	// +optional
	GroupsClaim string `json:"groupsClaim,omitempty"`

	// boundAudiences defines the values the "aud" claim must match one of.
	// +optional
	BoundAudiences []string `json:"boundAudiences,omitempty"`

	// boundSubject defines the value the "sub" claim must match.
	// +optional
	BoundSubject string `json:"boundSubject,omitempty"`

	// boundClaims defines claims that must match the given value. A claim
	// holding a list matches when it contains the value.
	// +optional
	BoundClaims map[string]string `json:"boundClaims,omitempty"`

	// claimMappings maps claims to the metadata keys set on the token and alias.
	// +optional
	ClaimMappings map[string]string `json:"claimMappings,omitempty"`

	// oidcScopes defines the scopes requested besides "openid".
	// +optional
	OIDCScopes []string `json:"oidcScopes,omitempty"`

	// maxAge defines, in seconds, the maximum elapsed time since the last
	// authentication of the user. 0 leaves it unchecked.
	// +kubebuilder:validation:Minimum=0
	// +optional
	MaxAge int `json:"maxAge,omitempty"`

	TokenFields `json:",inline"`
}

// OIDCRoleStatus defines the observed state of OIDCRole.
type OIDCRoleStatus struct {
	// observedGeneration is the .metadata.generation the status was last reconciled against.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions represent the current state of the OIDCRole resource.
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
// +kubebuilder:printcolumn:name="Path",type=string,priority=1,JSONPath=`.spec.authPath`
// +kubebuilder:printcolumn:name="Message",type=string,priority=1,JSONPath=`.status.conditions[?(@.type=="Ready")].message`

// OIDCRole is the Schema for the oidcroles API. It manages
// auth/<authPath>/role/<name> of an oidc auth method.
type OIDCRole struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of OIDCRole
	// +required
	Spec OIDCRoleSpec `json:"spec"`

	// status defines the observed state of OIDCRole
	// +optional
	Status OIDCRoleStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// OIDCRoleList contains a list of OIDCRole
type OIDCRoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []OIDCRole `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &OIDCRole{}, &OIDCRoleList{})
		return nil
	})
}
