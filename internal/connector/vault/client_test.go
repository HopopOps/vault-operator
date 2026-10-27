package vault

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	vaultapi "github.com/hashicorp/vault/api"
)

// TestClientRelogin checks that a client whose token is about to expire logs in
// again, and that a long lived one does not.
func TestClientRelogin(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ttl    int
		logins int
	}{
		{name: "expiring token", ttl: 1, logins: 2},
		{name: "long lived token", ttl: 3600, logins: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logins := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				logins++
				_, _ = fmt.Fprintf(w, `{"auth":{"client_token":"token-%d","lease_duration":%d}}`, logins, tc.ttl)
			}))
			defer server.Close()

			tokenPath := filepath.Join(t.TempDir(), "token")
			if err := os.WriteFile(tokenPath, []byte("sa-token"), 0o600); err != nil {
				t.Fatal(err)
			}

			v, err := NewVaultKubernetesClient(context.Background(), &Parameters{
				Address:   server.URL,
				AuthPath:  "kubernetes",
				Role:      "vault-operator",
				TokenPath: tokenPath,
			})
			if err != nil {
				t.Fatal(err)
			}

			c, err := v.Client(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if logins != tc.logins {
				t.Errorf("got %d logins, want %d", logins, tc.logins)
			}
			if want := fmt.Sprintf("token-%d", tc.logins); c.Token() != want {
				t.Errorf("got token %q, want %q", c.Token(), want)
			}
		})
	}
}

// TestIgnoreNotFound replays what Vault answers when the mount of the deleted
// object is gone.
func TestIgnoreNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":["no handler for route \"auth/gone/role/x\". route entry not found."]}`))
	}))
	defer srv.Close()

	c, err := vaultapi.NewClient(&vaultapi.Config{Address: srv.URL})
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.Logical().DeleteWithContext(context.Background(), "auth/gone/role/x")
	if err == nil {
		t.Fatal("the 404 did not surface as an error, the test no longer covers the finalizer case")
	}
	if err := IgnoreNotFound(err); err != nil {
		t.Errorf("a 404 was not ignored: %v", err)
	}

	if err := IgnoreNotFound(&vaultapi.ResponseError{StatusCode: http.StatusForbidden}); err == nil {
		t.Error("a 403 was ignored")
	}
}
