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

package database

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	databasev1beta1 "hopopops/vault-operator/api/database/v1beta1"
	"hopopops/vault-operator/internal/connector/vault"
	"hopopops/vault-operator/internal/controller"
)

// TestConfigCredentialRotation checks that a rotation of the root credentials
// Secret is pushed to Vault, and that a reconciliation that changes nothing
// does not rewrite the connection. Vault never returns the password, so a
// rotation is only visible through the hash recorded in the status.
func TestConfigCredentialRotation(t *testing.T) {
	const namespace = "vault"

	var writes []map[string]any
	exists := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/login"):
			_, _ = w.Write([]byte(`{"auth":{"client_token":"token","lease_duration":3600}}`))

		case r.Method == http.MethodGet && r.URL.Path == "/v1/database/config/base":
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"plugin_name":"postgresql-database-plugin"}}`))

		case r.Method == http.MethodPut && r.URL.Path == "/v1/database/config/base":
			payload := map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("failed to decode the connection payload: %v", err)
			}
			writes = append(writes, payload)
			exists = true
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

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user",
			Namespace: namespace,
			Labels:    map[string]string{controller.SecretWatchLabel: "true"},
		},
		Data: map[string][]byte{"username": []byte("root"), "password": []byte("first")},
	}
	config := &databasev1beta1.Config{
		ObjectMeta: metav1.ObjectMeta{Name: "base", Namespace: namespace},
		Spec: databasev1beta1.ConfigSpec{
			Path:            "database",
			PluginName:      "postgresql-database-plugin",
			ConnectionURL:   "postgresql://{{username}}:{{password}}@pg:5432/postgres",
			AllowedRoles:    []string{"*"},
			RootCredentials: databasev1beta1.RootCredentials{SecretName: "user", UsernameKey: "username", PasswordKey: "password"},
		},
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := databasev1beta1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	k8s := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret, config).
		WithStatusSubresource(&databasev1beta1.Config{}).
		Build()

	r := &ConfigReconciler{Client: k8s, Scheme: scheme, Vault: v}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "base"}}

	reconcile := func(step string) {
		t.Helper()
		if _, err := r.Reconcile(ctx, req); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}

	reconcile("initial reconciliation")
	if len(writes) != 1 {
		t.Fatalf("got %d connection writes, want 1", len(writes))
	}
	if got := writes[0]["password"]; got != "first" {
		t.Errorf("got password %v, want first", got)
	}

	reconcile("unchanged reconciliation")
	if len(writes) != 1 {
		t.Fatalf("got %d connection writes for an unchanged resource, want 1", len(writes))
	}

	secret.Data["password"] = []byte("second")
	if err := k8s.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}

	reconcile("reconciliation after rotation")
	if len(writes) != 2 {
		t.Fatalf("got %d connection writes after a rotation, want 2", len(writes))
	}
	if got := writes[1]["password"]; got != "second" {
		t.Errorf("got password %v, want second", got)
	}

	// An unlabelled Secret is never informed on, so a rotation of it would go
	// unnoticed. Reconciliation has to fail rather than report a healthy Config.
	delete(secret.Labels, controller.SecretWatchLabel)
	if err := k8s.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err == nil {
		t.Error("reconciling against an unlabelled Secret succeeded, want an error")
	}
}
