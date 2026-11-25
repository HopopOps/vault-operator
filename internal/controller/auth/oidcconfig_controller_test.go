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

	authv1beta1 "hopopops/vault-operator/api/auth/v1beta1"
	"hopopops/vault-operator/internal/connector/vault"
	"hopopops/vault-operator/internal/controller"
)

// TestOIDCConfigSecretRotation checks that a rotated client secret is pushed to
// Vault, that an unchanged resource is not rewritten, and that a mount that
// lost its config (recreated) is configured again.
func TestOIDCConfigSecretRotation(t *testing.T) {
	const namespace = "vault"

	var writes []map[string]any
	exists := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/login"):
			_, _ = w.Write([]byte(`{"auth":{"client_token":"token","lease_duration":3600}}`))

		case r.Method == http.MethodGet && r.URL.Path == "/v1/auth/oidc/config":
			if !exists {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"oidc_client_id":"vault"}}`))

		case r.Method == http.MethodPut && r.URL.Path == "/v1/auth/oidc/config":
			payload := map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("failed to decode the config payload: %v", err)
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
			Name:      "oidc",
			Namespace: namespace,
			Labels:    map[string]string{controller.SecretWatchLabel: "true"},
		},
		Data: map[string][]byte{"client-secret": []byte("first")},
	}
	config := &authv1beta1.OIDCConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "oidc", Namespace: namespace},
		Spec: authv1beta1.OIDCConfigSpec{
			AuthPath:     "oidc",
			DiscoveryURL: "https://accounts.example.com/realms/example",
			ClientID:     "vault",
			ClientSecret: authv1beta1.ClientSecret{SecretName: "oidc", Key: "client-secret"},
			DefaultRole:  "default",
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
		WithObjects(secret, config).
		WithStatusSubresource(&authv1beta1.OIDCConfig{}).
		Build()

	r := &OIDCConfigReconciler{Client: k8s, Scheme: scheme, Vault: v}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: "oidc"}}

	reconcile := func(step string, want int) {
		t.Helper()
		if _, err := r.Reconcile(ctx, req); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		if len(writes) != want {
			t.Fatalf("%s: got %d config writes, want %d", step, len(writes), want)
		}
	}

	reconcile("initial reconciliation", 1)
	if got := writes[0]["oidc_client_secret"]; got != "first" {
		t.Errorf("got client secret %v, want first", got)
	}
	if got := writes[0]["default_role"]; got != "default" {
		t.Errorf("got default role %v, want default", got)
	}

	reconcile("unchanged reconciliation", 1)

	secret.Data["client-secret"] = []byte("second")
	if err := k8s.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	reconcile("reconciliation after rotation", 2)
	if got := writes[1]["oidc_client_secret"]; got != "second" {
		t.Errorf("got client secret %v, want second", got)
	}

	exists = false
	reconcile("reconciliation after the mount lost its config", 3)

	delete(secret.Labels, controller.SecretWatchLabel)
	if err := k8s.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(ctx, req); err == nil {
		t.Error("reconciling against an unlabelled Secret succeeded, want an error")
	}
}
