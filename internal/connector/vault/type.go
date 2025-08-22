package vault

import (
	"hopopops/vault-operator/api/auth/v1beta1"
	"reflect"
)

type Policy struct {
	Name   string `json:"name"`
	Policy string `json:"policy"`
}

type KubernetesRole struct {
	BoundServiceAccountNames      []string `json:"bound_service_account_names"`
	BoundServiceAccountNamespaces []string `json:"bound_service_account_namespaces,omitempty"`
	Audience                      string   `json:"audience,omitempty"`
	AliasNameSource               string   `json:"alias_name_source,omitempty"`
	TokenTTL                      int      `json:"token_ttl,omitempty"`
	TokenMaxTTL                   int      `json:"token_max_ttl,omitempty"`
	TokenPolicies                 []string `json:"token_policies,omitempty"`
	TokenBoundCIDRs               []string `json:"token_bound_cidrs,omitempty"`
	TokenExplicitMaxTTL           int      `json:"token_explicit_max_ttl,omitempty"`
	TokenNoDefaultPolicy          bool     `json:"token_no_default_policy,omitempty"`
	TokenNumUses                  int      `json:"token_num_uses,omitempty"`
	TokenPeriod                   int      `json:"token_period,omitempty"`
	TokenType                     string   `json:"token_type,omitempty"`
}

func (k *KubernetesRole) IsDifferentFromSpec(s *v1beta1.KubernetesRoleSpec) bool {
	return !reflect.DeepEqual(k.BoundServiceAccountNames, s.BoundServiceAccountNames) ||
		!reflect.DeepEqual(k.BoundServiceAccountNamespaces, s.BoundServiceAccountNamespaces) ||
		!reflect.DeepEqual(k.TokenPolicies, s.TokenPolicies) ||
		!reflect.DeepEqual(k.TokenBoundCIDRs, s.TokenBoundCIDRs) ||
		k.Audience != s.Audience ||
		k.AliasNameSource != s.AliasNameSource ||
		k.TokenTTL != s.TokenTTL ||
		k.TokenMaxTTL != s.TokenMaxTTL ||
		k.TokenExplicitMaxTTL != s.TokenExplicitMaxTTL ||
		k.TokenNoDefaultPolicy != s.TokenNoDefaultPolicy ||
		k.TokenNumUses != s.TokenNumUses ||
		k.TokenPeriod != s.TokenPeriod ||
		k.TokenType != s.TokenType
}
