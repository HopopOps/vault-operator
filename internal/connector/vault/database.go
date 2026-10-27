package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"

	databasev1beta1 "hopopops/vault-operator/api/database/v1beta1"
)

// DatabaseRole is the payload of a database secret engine role, as written to
// and read back from <mount>/roles/<name>.
type DatabaseRole struct {
	DBName               string   `json:"db_name"`
	CreationStatements   []string `json:"creation_statements"`
	RevocationStatements []string `json:"revocation_statements"`
	RollbackStatements   []string `json:"rollback_statements"`
	RenewStatements      []string `json:"renew_statements"`
	DefaultTTL           int      `json:"default_ttl"`
	MaxTTL               int      `json:"max_ttl"`
}

// DesiredDatabaseRole is the role Vault should hold for this spec. Empty lists
// rather than nil ones, so it compares equal to what Vault reads back.
func DesiredDatabaseRole(s *databasev1beta1.RoleSpec) *DatabaseRole {
	return &DatabaseRole{
		DBName:               s.DBName,
		CreationStatements:   nonNil(s.CreationStatements),
		RevocationStatements: nonNil(s.RevocationStatements),
		RollbackStatements:   nonNil(s.RollbackStatements),
		RenewStatements:      nonNil(s.RenewStatements),
		DefaultTTL:           s.DefaultTTL,
		MaxTTL:               s.MaxTTL,
	}
}

func (r *DatabaseRole) IsDifferentFrom(other *DatabaseRole) bool {
	return !reflect.DeepEqual(r, other)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// DatabaseConfigPayload is the connection configuration written to
// <mount>/config/<name>. Vault never returns the password, so the state of a
// connection is tracked as a hash of this payload rather than by reading it
// back, see Hash.
func DatabaseConfigPayload(s *databasev1beta1.ConfigSpec, username, password string) map[string]any {
	payload := map[string]any{
		"plugin_name":                        s.PluginName,
		"verify_connection":                  s.VerifyConnection,
		"allowed_roles":                      s.AllowedRoles,
		"root_credentials_rotate_statements": s.RootRotationStatements,
		"disable_escaping":                   s.DisableEscaping,
		orDefault(s.RootCredentials.UsernameField, "username"): username,
		orDefault(s.RootCredentials.PasswordField, "password"): password,
	}

	if s.ConnectionURL != "" {
		payload["connection_url"] = s.ConnectionURL
	}

	if s.PluginVersion != "" {
		payload["plugin_version"] = s.PluginVersion
	}
	if s.PasswordPolicy != "" {
		payload["password_policy"] = s.PasswordPolicy
	}
	if s.PasswordAuthentication != "" {
		payload["password_authentication"] = s.PasswordAuthentication
	}
	for k, v := range s.PluginConfig {
		payload[k] = v
	}

	return payload
}

// Hash fingerprints a connection payload, credentials included, so a rotation
// of the root password is noticed even though Vault will not hand it back.
func Hash(payload map[string]any) (string, error) {
	// Marshalling a map sorts the keys, so the hash is stable.
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
