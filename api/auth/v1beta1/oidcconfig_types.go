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

// ClientSecret points at the Secret holding the OAuth client secret.
type ClientSecret struct {
	// secretName defines the name of a Secret in the same namespace holding the
	// client secret. The Secret is watched: a change to it is pushed to Vault.
	// +required
	SecretName string `json:"secretName"`

	// key defines the key of the Secret holding the client secret.
	// +kubebuilder:default="client-secret"
	// +optional
	Key string `json:"key,omitempty"`
}

// OIDCConfigSpec defines the desired state of OIDCConfig
type OIDCConfigSpec struct {
	// authPath defines the remote path in Vault where the oidc auth method is enabled.
	// +kubebuilder:default="oidc"
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="AuthPath is immutable"
	// +optional
	AuthPath string `json:"authPath,omitempty"`

	// discoveryURL defines the OIDC discovery URL, without the .well-known component.
	// +required
	DiscoveryURL string `json:"discoveryURL"`

	// discoveryCAPEM defines the CA certificate or chain, in PEM, used to validate
	// the discovery URL. The system certificates are used when empty.
	// +optional
	DiscoveryCAPEM string `json:"discoveryCAPEM,omitempty"`

	// clientID defines the OAuth client ID registered with the provider.
	// +required
	ClientID string `json:"clientID"`

	// clientSecret defines where to read the OAuth client secret.
	// +required
	ClientSecret ClientSecret `json:"clientSecret"`

	// responseMode defines how the authorization response is returned.
	// +kubebuilder:validation:Enum=query;form_post
	// +optional
	ResponseMode string `json:"responseMode,omitempty"`

	// responseTypes defines the response types to request. "id_token" requires
	// responseMode "form_post".
	// +optional
	ResponseTypes []string `json:"responseTypes,omitempty"`

	// boundIssuer defines the value the "iss" claim must match.
	// +optional
	BoundIssuer string `json:"boundIssuer,omitempty"`

	// supportedAlgs defines the signing algorithms accepted. Vault defaults to RS256 for OIDC roles.
	// +optional
	SupportedAlgs []string `json:"supportedAlgs,omitempty"`

	// defaultRole defines the role used when a login names none.
	// +optional
	DefaultRole string `json:"defaultRole,omitempty"`
}

// OIDCConfigStatus defines the observed state of OIDCConfig.
type OIDCConfigStatus struct {
	// observedGeneration is the .metadata.generation the status was last reconciled against.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions represent the current state of the OIDCConfig resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// configHash is the hash of the configuration last pushed to Vault, client
	// secret included. Vault never returns the secret, so this is how a rotation
	// of the referenced Secret is noticed.
	// +optional
	ConfigHash string `json:"configHash,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Path",type=string,JSONPath=`.spec.authPath`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Message",type=string,priority=1,JSONPath=`.status.conditions[?(@.type=="Ready")].message`

// OIDCConfig is the Schema for the oidcconfigs API. It manages
// auth/<authPath>/config of an oidc auth method; one per mount.
type OIDCConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of OIDCConfig
	// +required
	Spec OIDCConfigSpec `json:"spec"`

	// status defines the observed state of OIDCConfig
	// +optional
	Status OIDCConfigStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// OIDCConfigList contains a list of OIDCConfig
type OIDCConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []OIDCConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &OIDCConfig{}, &OIDCConfigList{})
		return nil
	})
}
