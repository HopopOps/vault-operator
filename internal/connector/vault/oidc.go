package vault

import authv1beta1 "hopopops/vault-operator/api/auth/v1beta1"

// OIDCConfigPayload is the configuration written to auth/<mount>/config.
// Vault never returns oidc_client_secret, so the state is tracked as a hash of
// this payload rather than by reading it back, see Hash.
func OIDCConfigPayload(s *authv1beta1.OIDCConfigSpec, clientSecret string) map[string]any {
	return map[string]any{
		"oidc_discovery_url":    s.DiscoveryURL,
		"oidc_discovery_ca_pem": s.DiscoveryCAPEM,
		"oidc_client_id":        s.ClientID,
		"oidc_client_secret":    clientSecret,
		"oidc_response_mode":    s.ResponseMode,
		"oidc_response_types":   nonNil(s.ResponseTypes),
		"bound_issuer":          s.BoundIssuer,
		"jwt_supported_algs":    nonNil(s.SupportedAlgs),
		"default_role":          s.DefaultRole,
	}
}

// OIDCRolePayload is the role written to auth/<mount>/role/<name>. Vault
// updates a role field by field and keeps whatever is omitted, so every
// managed field is sent, empty or not, for a removal from the spec to clear it.
func OIDCRolePayload(s *authv1beta1.OIDCRoleSpec) map[string]any {
	return map[string]any{
		"role_type":               "oidc",
		"allowed_redirect_uris":   nonNil(s.AllowedRedirectURIs),
		"user_claim":              s.UserClaim,
		"groups_claim":            s.GroupsClaim,
		"bound_audiences":         nonNil(s.BoundAudiences),
		"bound_subject":           s.BoundSubject,
		"bound_claims":            nonNilMap(s.BoundClaims),
		"claim_mappings":          nonNilMap(s.ClaimMappings),
		"oidc_scopes":             nonNil(s.OIDCScopes),
		"max_age":                 s.MaxAge,
		"token_ttl":               s.TokenTTL,
		"token_max_ttl":           s.TokenMaxTTL,
		"token_policies":          nonNil(s.TokenPolicies),
		"token_bound_cidrs":       nonNil(s.TokenBoundCIDRs),
		"token_explicit_max_ttl":  s.TokenExplicitMaxTTL,
		"token_no_default_policy": s.TokenNoDefaultPolicy,
		"token_num_uses":          s.TokenNumUses,
		"token_period":            s.TokenPeriod,
		"token_type":              orDefault(s.TokenType, "default"),
	}
}

func nonNilMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}
