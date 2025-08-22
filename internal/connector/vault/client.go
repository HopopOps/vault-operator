package vault

import (
	"context"
	"fmt"
	"log"

	vaultapi "github.com/hashicorp/vault/api"
	vaultauth "github.com/hashicorp/vault/api/auth/kubernetes"
)

type Vault struct {
	Client     *vaultapi.Client
	parameters Parameters
}

type Parameters struct {
	// connection parameters
	Address  string
	AuthPath string
	Role     string

	// the locations / field names of our two secrets
	TokenPath string
}

func NewVaultKubernetesClient(ctx context.Context, parameters *Parameters) (*Vault, *vaultapi.Secret, error) {
	log.Printf("connecting to vault @ %s", parameters.Address)

	config := vaultapi.DefaultConfig() // modify for more granular configuration
	config.Address = parameters.Address

	client, err := vaultapi.NewClient(config)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to initialize vault client: %w", err)
	}

	vault := &Vault{
		Client:     client,
		parameters: *parameters,
	}

	token, err := vault.login(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("vault login error: %w", err)
	}

	log.Println("connecting to vault: success!")

	return vault, token, nil
}

func (v *Vault) PeriodicallyRenewLeases(
	ctx context.Context,
	authToken *vaultapi.Secret,
) {
	/* */ log.Println("renew / recreate secrets loop: begin")
	defer log.Println("renew / recreate secrets loop: end")

	currentAuthToken := authToken

	for {
		renewed, err := v.renewLeases(ctx, currentAuthToken)
		if err != nil {
			log.Fatalf("renew error: %v", err) // simplified error handling
		}

		if renewed&exitRequested != 0 {
			return
		}

		if renewed&expiringAuthToken != 0 {
			log.Printf("auth token: can no longer be renewed; will log in again")

			authToken, err := v.login(ctx)
			if err != nil {
				log.Fatalf("login authentication error: %v", err) // simplified error handling
			}

			currentAuthToken = authToken
		}
	}
}

// renewResult is a bitmask which could contain one or more of the values below
type renewResult uint8

const (
	renewError renewResult = 1 << iota
	exitRequested
	expiringAuthToken // will be revoked soon
)

// renewLeases is a blocking helper function that uses LifetimeWatcher
// instances to periodically renew the given secrets when they are close to
// their 'token_ttl' expiration times until one of the secrets is close to its
// 'token_max_ttl' lease expiration time.
func (v *Vault) renewLeases(ctx context.Context, authToken *vaultapi.Secret) (renewResult, error) {
	/* */ log.Println("renew cycle: begin")
	defer log.Println("renew cycle: end")

	// auth token
	authTokenWatcher, err := v.Client.NewLifetimeWatcher(&vaultapi.LifetimeWatcherInput{
		Secret: authToken,
	})
	if err != nil {
		return renewError, fmt.Errorf("unable to initialize auth token lifetime watcher: %w", err)
	}

	go authTokenWatcher.Start()
	defer authTokenWatcher.Stop()

	// monitor events from both watchers
	for {
		select {
		case <-ctx.Done():
			return exitRequested, nil

		// DoneCh will return if renewal fails, or if the remaining lease
		// duration is under a built-in threshold and either renewing is not
		// extending it or renewing is disabled.  In both cases, the caller
		// should attempt a re-read of the secret. Clients should check the
		// return value of the channel to see if renewal was successful.
		case err := <-authTokenWatcher.DoneCh():
			// Leases created by a token get revoked when the token is revoked.
			return expiringAuthToken, err

		// RenewCh is a channel that receives a message when a successful
		// renewal takes place and includes metadata about the renewal.
		case info := <-authTokenWatcher.RenewCh():
			log.Printf("auth token: successfully renewed; remaining duration: %ds", info.Secret.Auth.LeaseDuration)
		}
	}
}

func (v *Vault) login(ctx context.Context) (*vaultapi.Secret, error) {
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
		return nil, fmt.Errorf("unable to initialize Kubernetes auth method: %w", err)
	}

	authInfo, err := v.Client.Auth().Login(ctx, kubernetesAuth)
	if err != nil {
		return nil, fmt.Errorf("unable to log in with Kubernetes auth: %w", err)
	}
	if authInfo == nil {
		return nil, fmt.Errorf("no auth info was returned after login")
	}

	return authInfo, nil
}
