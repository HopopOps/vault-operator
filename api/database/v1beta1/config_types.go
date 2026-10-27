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

// RootCredentials points at the Secret holding the credentials of the privileged
// database account Vault uses to manage the users it hands out.
type RootCredentials struct {
	// secretName defines the name of a Secret in the same namespace holding the
	// root credentials. The Secret is watched: a change to it is pushed to Vault.
	// +required
	SecretName string `json:"secretName"`

	// usernameKey defines the key of the Secret holding the username. Ignored
	// when spec.username is set.
	// +kubebuilder:default="username"
	// +optional
	UsernameKey string `json:"usernameKey,omitempty"`

	// passwordKey defines the key of the Secret holding the password.
	// +kubebuilder:default="password"
	// +optional
	PasswordKey string `json:"passwordKey,omitempty"`

	// usernameField defines the connection parameter receiving the username,
	// for plugins not using "username" (e.g. "public_key" for mongodbatlas).
	// +kubebuilder:default="username"
	// +optional
	UsernameField string `json:"usernameField,omitempty"`

	// passwordField defines the connection parameter receiving the password,
	// for plugins not using "password" (e.g. "private_key" for mongodbatlas).
	// +kubebuilder:default="password"
	// +optional
	PasswordField string `json:"passwordField,omitempty"`
}

// ConfigSpec defines the desired state of Config
type ConfigSpec struct {
	// path defines the mount path of the database secret engine.
	// +required
	Path string `json:"path"`

	// rootCredentials defines where to read the credentials of the privileged database account.
	// +required
	RootCredentials RootCredentials `json:"rootCredentials"`

	// pluginName defines the name of the plugin to use for this connection, such
	// as "postgresql-database-plugin".
	// +required
	PluginName string `json:"pluginName"`

	// pluginVersion defines the semantic version of the plugin to use for this connection.
	// +optional
	PluginVersion string `json:"pluginVersion,omitempty"`

	// connectionURL defines the connection string used to connect to the database.
	// The "{{username}}" and "{{password}}" templating directives are replaced
	// with the root credentials. Left out of the payload when empty, for plugins
	// without a connection URL (e.g. mongodbatlas).
	// +optional
	ConnectionURL string `json:"connectionURL,omitempty"`

	// username defines the name of the privileged database account. It takes
	// precedence over the username read from the referenced Secret.
	// +optional
	Username string `json:"username,omitempty"`

	// verifyConnection defines whether the connection is verified when it is configured.
	// +kubebuilder:default=true
	// +optional
	VerifyConnection bool `json:"verifyConnection,omitempty"`

	// allowedRoles defines the list of roles allowed to use this connection. A
	// "*" entry allows every role.
	// +kubebuilder:default={"*"}
	// +optional
	AllowedRoles []string `json:"allowedRoles,omitempty"`

	// rootRotationStatements defines the database statements executed to rotate
	// the root credentials.
	// +optional
	RootRotationStatements []string `json:"rootRotationStatements,omitempty"`

	// passwordPolicy defines the name of the Vault password policy used to
	// generate the passwords of the users handed out by this connection.
	// +optional
	PasswordPolicy string `json:"passwordPolicy,omitempty"`

	// passwordAuthentication, for the PostgreSQL plugin, defines how passwords
	// reach the server. With "scram-sha-256" they are hashed by Vault, with
	// "password" they are sent in plaintext.
	// +kubebuilder:validation:Enum=password;scram-sha-256
	// +optional
	PasswordAuthentication string `json:"passwordAuthentication,omitempty"`

	// disableEscaping defines whether special characters in the username and
	// password are left unescaped. Useful for alternate connection string
	// formats such as ADO.
	// +optional
	DisableEscaping bool `json:"disableEscaping,omitempty"`

	// pluginConfig defines the configuration specific to the database plugin in
	// use, merged into the payload as is.
	// +optional
	PluginConfig map[string]string `json:"pluginConfig,omitempty"`
}

// ConfigStatus defines the observed state of Config.
type ConfigStatus struct {
	// observedGeneration is the .metadata.generation the status was last reconciled against.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// conditions represent the current state of the Config resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// credentialsHash is the hash of the root credentials last pushed to Vault.
	// Vault never returns the password, so this is the only way to notice that
	// the referenced Secret was rotated and the connection needs pushing again.
	// +optional
	CredentialsHash string `json:"credentialsHash,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Path",type=string,JSONPath=`.spec.path`
// +kubebuilder:printcolumn:name="Plugin",type=string,JSONPath=`.spec.pluginName`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].reason`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Message",type=string,priority=1,JSONPath=`.status.conditions[?(@.type=="Ready")].message`

// Config is the Schema for the configs API
type Config struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Config
	// +required
	Spec ConfigSpec `json:"spec"`

	// status defines the observed state of Config
	// +optional
	Status ConfigStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ConfigList contains a list of Config
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Config `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Config{}, &ConfigList{})
		return nil
	})
}
