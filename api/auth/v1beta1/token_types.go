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

type TokenTarget struct {
	// +required
	Name string `json:"name"`

	// +optional
	// +kubebuilder:default="Retain"
	// +kubebuilder:validation:Enum=Retain;Delete
	DeletionPolicy string `json:"deletionPolicy,omitempty"`
}

// TokenSpec defines the desired state of Token
type TokenSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// +required
	Target TokenTarget `json:"target"`

	// roleName defines the name of the token role.
	// +optional
	RoleName string `json:"roleName,omitempty"`

	// policies defines a list of policies for the token. This must be a subset of the policies belonging to the token making the request, unless the calling token is root or contains sudo capabilities.
	// +optional
	Policies []string `json:"policies,omitempty"`

	// meta defines a map of string to string valued metadata. This is passed through to the audit devices.
	// +optional
	Meta map[string]string `json:"meta,omitempty"`

	// noParent when set to true, the token created will not have a parent. This argument only has effect if used by a root or sudo caller.
	// +optional
	NoParent bool `json:"noParent,omitempty"`

	// noDefaultPolicy if true the default policy will not be contained in this token's policy set.
	// +optional
	NoDefaultPolicy bool `json:"noDefaultPolicy,omitempty"`

	// renewable set to false to disable the ability of the token to be renewed past its initial TTL. Setting the value to true will allow the token to be renewable up to the system/mount maximum TTL.
	// +kubebuilder:default=true
	// +optional
	Renewable bool `json:"renewable,omitempty"`

	// ttl defines the TTL period of the token, provided as "1h", where hour is the largest suffix. If not provided, the token is valid for the default lease TTL, or indefinitely if the root policy is used.
	// +optional
	TTL string `json:"ttl,omitempty"`

	// type defines the token type. Can be "batch" or "service". Defaults to the type specified by the role configuration named by roleName.
	// +kubebuilder:validation:Enum=batch;service
	// +optional
	Type string `json:"type,omitempty"`

	// explicitMaxTTL if set, the token will have an explicit max TTL set upon it. This maximum token TTL cannot be changed later, and unlike with normal tokens, updates to the system/mount max TTL value will have no effect at renewal time.
	// +optional
	ExplicitMaxTTL string `json:"explicitMaxTTL,omitempty"`

	// numUses defines the maximum uses for the given token. This can be used to create a one-time-token or limited use token. The value of 0 has no limit to the number of uses.
	// +kubebuilder:validation:Minimum=0
	// +optional
	NumUses int `json:"numUses,omitempty"`

	// period if specified, the token will be periodic; it will have no maximum TTL (unless an "explicitMaxTTL" is also set) but every renewal will use the given period. Requires a root token or one with the sudo capability.
	// +optional
	Period string `json:"period,omitempty"`

	// entityAlias defines the name of the entity alias to associate with during token creation. Only works in combination with roleName argument and used entity alias must be listed in allowed_entity_aliases.
	// +optional
	EntityAlias string `json:"entityAlias,omitempty"`
}

// TokenStatus defines the observed state of Token.
type TokenStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	Accessor   string             `json:"accessor,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Token is the Schema for the tokens API
type Token struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Token
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="TokenSpec is immutable"
	// +required
	Spec TokenSpec `json:"spec"`

	// status defines the observed state of Token
	// +optional
	Status TokenStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// TokenList contains a list of Token
type TokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Token `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Token{}, &TokenList{})
}
