package vault

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	vaultauth "github.com/hashicorp/vault/api/auth/kubernetes"
)

// expiryMargin is how long before the actual expiration a token is considered
// expired, so a request is never issued with a token about to die.
const expiryMargin = 30 * time.Second

type Vault struct {
	client     *vaultapi.Client
	parameters Parameters

	mu     sync.Mutex
	expiry time.Time // zero value means the token never expires
}

type Parameters struct {
	// connection parameters
	Address  string
	AuthPath string
	Role     string

	// the locations / field names of our two secrets
	TokenPath string
}

func NewVaultKubernetesClient(ctx context.Context, parameters *Parameters) (*Vault, error) {
	log.Printf("connecting to vault @ %s", parameters.Address)

	config := vaultapi.DefaultConfig() // modify for more granular configuration
	config.Address = parameters.Address

	client, err := vaultapi.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize vault client: %w", err)
	}

	vault := &Vault{
		client:     client,
		parameters: *parameters,
	}

	if err := vault.login(ctx); err != nil {
		return nil, fmt.Errorf("vault login error: %w", err)
	}

	log.Println("connecting to vault: success!")

	return vault, nil
}

// Client returns a vault client holding a valid token, logging in again when
// the current token is expired or about to expire.
func (v *Vault) Client(ctx context.Context) (*vaultapi.Client, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.expiry.IsZero() && time.Now().Add(expiryMargin).After(v.expiry) {
		log.Println("vault token expired, logging in again")

		if err := v.login(ctx); err != nil {
			return nil, fmt.Errorf("vault login error: %w", err)
		}
	}

	return v.client, nil
}

// login authenticates against the kubernetes auth method and records when the
// returned token expires. Callers must hold v.mu, except at construction time.
func (v *Vault) login(ctx context.Context) error {
	// The service-account token will be read from the path where the token's
	// Kubernetes Secret is mounted. By default, Kubernetes will mount it to
	// /var/run/secrets/kubernetes.io/serviceaccount/token, but an administrator
	// may have configured it to be mounted elsewhere.
	// In that case, we'll use the option WithServiceAccountTokenPath to look
	// for the token there.
	kubernetesAuth, err := vaultauth.NewKubernetesAuth(
		v.parameters.Role,
		vaultauth.WithServiceAccountTokenPath(v.parameters.TokenPath),
		vaultauth.WithMountPath(v.parameters.AuthPath),
	)
	if err != nil {
		return fmt.Errorf("unable to initialize Kubernetes auth method: %w", err)
	}

	authInfo, err := v.client.Auth().Login(ctx, kubernetesAuth)
	if err != nil {
		return fmt.Errorf("unable to log in with Kubernetes auth: %w", err)
	}
	if authInfo == nil || authInfo.Auth == nil {
		return fmt.Errorf("no auth info was returned after login")
	}

	v.expiry = time.Time{}
	if ttl := authInfo.Auth.LeaseDuration; ttl > 0 {
		v.expiry = time.Now().Add(time.Duration(ttl) * time.Second)
	}

	return nil
}
