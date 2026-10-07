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
