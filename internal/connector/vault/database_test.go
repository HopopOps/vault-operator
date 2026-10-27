package vault

import (
	"encoding/json"
	"testing"

	databasev1beta1 "hopopops/vault-operator/api/database/v1beta1"
)

func TestHashTracksCredentials(t *testing.T) {
	spec := &databasev1beta1.ConfigSpec{
		Path:          "database",
		PluginName:    "postgresql-database-plugin",
		ConnectionURL: "postgresql://{{username}}:{{password}}@pg:5432/postgres",
		AllowedRoles:  []string{"*"},
	}

	first, err := Hash(DatabaseConfigPayload(spec, "root", "first"))
	if err != nil {
		t.Fatal(err)
	}

	same, err := Hash(DatabaseConfigPayload(spec, "root", "first"))
	if err != nil {
		t.Fatal(err)
	}
	if first != same {
		t.Error("the hash of an unchanged payload is not stable")
	}

	rotated, err := Hash(DatabaseConfigPayload(spec, "root", "second"))
	if err != nil {
		t.Fatal(err)
	}
	if first == rotated {
		t.Error("the hash did not change with the password, a rotation would go unnoticed")
	}
}

// TestDesiredDatabaseRoleMatchesVault checks that a role holding no optional
// statement compares equal to what Vault reads back, which returns empty lists
// rather than nulls.
func TestDesiredDatabaseRoleMatchesVault(t *testing.T) {
	desired := DesiredDatabaseRole(&databasev1beta1.RoleSpec{
		Path:               "database",
		DBName:             "base",
		CreationStatements: []string{"CREATE ROLE \"{{name}}\";"},
		DefaultTTL:         3600,
		MaxTTL:             86400,
	})

	read := &DatabaseRole{}
	if err := json.Unmarshal([]byte(`{
		"db_name": "base",
		"creation_statements": ["CREATE ROLE \"{{name}}\";"],
		"revocation_statements": [],
		"rollback_statements": [],
		"renew_statements": [],
		"default_ttl": 3600,
		"max_ttl": 86400
	}`), read); err != nil {
		t.Fatal(err)
	}

	if desired.IsDifferentFrom(read) {
		t.Errorf("got %+v, want %+v", desired, read)
	}
}

// TestConfigPayloadDefaultsUnchanged pins the payload of a connection using the
// defaults, so existing Configs keep their hash and are not pushed again.
func TestConfigPayloadDefaultsUnchanged(t *testing.T) {
	payload := DatabaseConfigPayload(&databasev1beta1.ConfigSpec{
		Path:          "database",
		PluginName:    "postgresql-database-plugin",
		ConnectionURL: "postgresql://{{username}}:{{password}}@pg:5432/postgres",
	}, "root", "secret")

	for k, want := range map[string]string{
		"username":       "root",
		"password":       "secret",
		"connection_url": "postgresql://{{username}}:{{password}}@pg:5432/postgres",
	} {
		if payload[k] != want {
			t.Errorf("payload[%q] = %v, want %q", k, payload[k], want)
		}
	}
}

func TestConfigPayloadCustomFields(t *testing.T) {
	payload := DatabaseConfigPayload(&databasev1beta1.ConfigSpec{
		Path:       "database",
		PluginName: "mongodbatlas-database-plugin",
		RootCredentials: databasev1beta1.RootCredentials{
			SecretName:    "atlas",
			UsernameField: "public_key",
			PasswordField: "private_key",
		},
		PluginConfig: map[string]string{"project_id": "abc"},
	}, "pub", "priv")

	if payload["public_key"] != "pub" || payload["private_key"] != "priv" || payload["project_id"] != "abc" {
		t.Errorf("credentials or plugin config not mapped: %v", payload)
	}
	for _, k := range []string{"username", "password", "connection_url"} {
		if _, ok := payload[k]; ok {
			t.Errorf("payload carries %q, which the plugin does not expect", k)
		}
	}
}
