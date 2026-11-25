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

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	authv1beta1 "hopopops/vault-operator/api/auth/v1beta1"
	"hopopops/vault-operator/internal/connector/vault"
)

// TestOIDCRoleLifecycle checks that unset fields are sent empty, since Vault
// keeps any field a role update omits, and that deletion removes the role.
func TestOIDCRoleLifecycle(t *testing.T) {
	const namespace = "vault"

	var writes []map[string]any
	deletes := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/login"):
			_, _ = w.Write([]byte(`{"auth":{"client_token":"token","lease_duration":3600}}`))

		case r.Method == http.MethodPut && r.URL.Path == "/v1/auth/oidc/role/user":
			payload := map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("failed to decode the role payload: %v", err)
			}
			writes = append(writes, payload)
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodDelete && r.URL.Path == "/v1/auth/oidc/role/user":
			deletes++
			w.WriteHeader(http.StatusNoContent)

		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("sa-token"), 0o600); err != nil {
		t.Fatal(err)
	}

	v, err := vault.NewVaultKubernetesClient(ctx, &vault.Parameters{
		Address:   server.URL,
		AuthPath:  "kubernetes",
		Role:      "vault-operator",
		TokenPath: tokenPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	role := &authv1beta1.OIDCRole{
		ObjectMeta: metav1.ObjectMeta{Name: "user", Namespace: namespace},
		Spec: authv1beta1.OIDCRoleSpec{
			AuthPath:            "oidc",
			AllowedRedirectURIs: []string{"http://localhost:8250/oidc/callback"},
			UserClaim:           "preferred_username",
			TokenFields:         authv1beta1.TokenFields{TokenPolicies: []string{"default"}},
		},
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := authv1beta1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	k8s := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(role).
		WithStatusSubresource(&authv1beta1.OIDCRole{}).
		Build()

	r := &OIDCRoleReconciler{Client: k8s, Scheme: scheme, Vault: v}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "user"}}

	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 {
		t.Fatalf("got %d role writes, want 1", len(writes))
	}
	for key, want := range map[string]string{
		"role_type":       "oidc",
		"user_claim":      "preferred_username",
		"groups_claim":    "",
		"token_type":      "default",
		"bound_audiences": "[]",
		"bound_claims":    "map[]",
		"token_policies":  "[default]",
	} {
		got, ok := writes[0][key]
		if !ok {
			t.Errorf("%s missing from the payload, Vault would keep its previous value", key)
			continue
		}
		if s := fmt.Sprint(got); s != want {
			t.Errorf("got %s=%s, want %s", key, s, want)
		}
	}

	if err := k8s.Get(ctx, req.NamespacedName, role); err != nil {
		t.Fatal(err)
	}
	if err := k8s.Delete(ctx, role); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if deletes != 1 {
		t.Errorf("got %d role deletions, want 1", deletes)
	}
}
