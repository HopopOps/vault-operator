package sys

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	sysv1beta1 "hopopops/vault-operator/api/sys/v1beta1"
	"hopopops/vault-operator/internal/connector/vault"
)

const policyName = "test"

// TestPolicyStatusWhenAlreadyInSync is the regression check for a policy whose
// content already matches in Vault: nothing is written to Vault, yet the
// resource must still report Ready for the current generation.
func TestPolicyStatusWhenAlreadyInSync(t *testing.T) {
	const document = "path \"secret/*\" { capabilities = [\"read\"] }"

	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/auth/kubernetes/login":
			_, _ = w.Write([]byte(`{"auth":{"client_token":"token","lease_duration":3600}}`))
		case r.Method == http.MethodPut:
			puts++
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = w.Write([]byte(`{"data":{"name":"test","policy":"path \"secret/*\" { capabilities = [\"read\"] }"}}`))
		}
	}))
	defer server.Close()

	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte("sa-token"), 0o600); err != nil {
		t.Fatal(err)
	}

	v, err := vault.NewVaultKubernetesClient(context.Background(), &vault.Parameters{
		Address:   server.URL,
		AuthPath:  "kubernetes",
		Role:      "vault-operator",
		TokenPath: tokenPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	policy := &sysv1beta1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:       policyName,
			Generation: 4,
			Finalizers: []string{policyFinalizer},
		},
		Spec: sysv1beta1.PolicySpec{Policy: new(document)},
	}

	sch := runtime.NewScheme()
	if err := sysv1beta1.AddToScheme(sch); err != nil {
		t.Fatal(err)
	}

	c := fake.NewClientBuilder().
		WithScheme(sch).
		WithObjects(policy).
		WithStatusSubresource(&sysv1beta1.Policy{}).
		Build()

	r := &PolicyReconciler{Client: c, Scheme: c.Scheme(), Vault: v}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: policyName},
	}); err != nil {
		t.Fatal(err)
	}

	if puts != 0 {
		t.Errorf("policy was pushed to Vault %d times, want 0", puts)
	}

	got := &sysv1beta1.Policy{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: policyName}, got); err != nil {
		t.Fatal(err)
	}

	cond := meta.FindStatusCondition(got.Status.Conditions, typeReadyPolicy)
	if cond == nil {
		t.Fatalf("no %q condition, status is %+v", typeReadyPolicy, got.Status)
	}
	if cond.Status != metav1.ConditionTrue {
		t.Errorf("condition is %s/%s, want True", cond.Status, cond.Reason)
	}
	if cond.ObservedGeneration != 4 || got.Status.ObservedGeneration != 4 {
		t.Errorf("observedGeneration is %d/%d, want 4/4", cond.ObservedGeneration, got.Status.ObservedGeneration)
	}
}

//go:fix inline
