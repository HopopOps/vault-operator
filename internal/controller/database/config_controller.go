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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	databasev1beta1 "hopopops/vault-operator/api/database/v1beta1"
	"hopopops/vault-operator/internal/connector/vault"
	"hopopops/vault-operator/internal/controller"
)

const (
	configFinalizer = "config.database.toolkit.vault.hopopops.com/finalizer"

	// secretNameIndex indexes Config resources by the Secret they read their
	// root credentials from, so a change to that Secret maps back to them.
	secretNameIndex = "spec.rootCredentials.secretName"
)

// Definitions to manage status conditions
const (
	typeReadyConfig = "Ready"
)

// ConfigReconciler reconciles a Config object
type ConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Vault  *vault.Vault
}

// +kubebuilder:rbac:groups=database.toolkit.vault.hopopops.com,resources=configs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=database.toolkit.vault.hopopops.com,resources=configs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=database.toolkit.vault.hopopops.com,resources=configs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Config instance
	config := &databasev1beta1.Config{}
	if err := r.Get(ctx, req.NamespacedName, config); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Config resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Config")
		return ctrl.Result{}, err
	}

	if config.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(config, configFinalizer) {
			// Initialize finalizer
			controllerutil.AddFinalizer(config, configFinalizer)
			if err := r.Update(ctx, config); err != nil {
				log.Error(err, "Failed to add finalizer to Config")
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(config, configFinalizer) {
			// Delete managed resources for this Config
			if err := r.deleteVaultConfig(ctx, config); err != nil {
				log.Error(err, "Failed to delete Config")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(config, configFinalizer)
			if err := r.Update(ctx, config); err != nil {
				log.Error(err, "Failed to remove finalizer from Config")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	username, password, err := r.rootCredentials(ctx, config)
	if err != nil {
		log.Error(err, "Failed to read root credentials for Config")
		_ = r.setCondition(ctx, config, metav1.ConditionFalse, "MissingCredentials", "Failed to read the root credentials Secret")
		return ctrl.Result{}, err
	}

	payload := vault.DatabaseConfigPayload(&config.Spec, username, password)
	hash, err := vault.Hash(payload)
	if err != nil {
		log.Error(err, "Failed to hash Config payload")
		return ctrl.Result{}, err
	}

	// Vault never hands the password back, so a read tells us whether the
	// connection exists but not whether it is current. The hash of the payload
	// we last pushed is what notices a rotated Secret, which is the whole point
	// of watching it.
	exists, err := r.vaultConfigExists(ctx, config)
	if err != nil {
		log.Error(err, "Failed to fetch Config")
		_ = r.setCondition(ctx, config, metav1.ConditionFalse, "FailedToFetch", "Failed to fetch database connection from Vault")
		return ctrl.Result{}, err
	}

	if !exists || config.Status.CredentialsHash != hash {
		if err := r.updateVaultConfig(ctx, config, payload); err != nil {
			log.Error(err, "Failed to update Config")
			_ = r.setCondition(ctx, config, metav1.ConditionFalse, "FailedToUpdate", "Failed to push database connection to Vault")
			return ctrl.Result{}, err
		}

		log.Info("Pushed database connection to Vault", "path", configPath(config))
		config.Status.CredentialsHash = hash
	}

	// Always record the outcome, the connection may already have been in sync.
	return ctrl.Result{}, r.setCondition(ctx, config, metav1.ConditionTrue, "Configured", "Successfully pushed database connection to Vault")
}

// rootCredentials reads the privileged database account out of the referenced
// Secret. spec.username, when set, wins over the Secret's username key.
func (r *ConfigReconciler) rootCredentials(ctx context.Context, config *databasev1beta1.Config) (string, string, error) {
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: config.Namespace,
		Name:      config.Spec.RootCredentials.SecretName,
	}, secret); err != nil {
		return "", "", err
	}

	// Reads are uncached, so an unlabelled Secret is readable but never watched,
	// and a rotation of it would go unnoticed. Refuse rather than look healthy.
	if secret.Labels[controller.SecretWatchLabel] != "true" {
		return "", "", fmt.Errorf("secret %q is missing the label %s=true, a rotation of it would go unnoticed", secret.Name, controller.SecretWatchLabel)
	}

	username := config.Spec.Username
	if username == "" {
		username = string(secret.Data[config.Spec.RootCredentials.UsernameKey])
	}

	password, ok := secret.Data[config.Spec.RootCredentials.PasswordKey]
	if !ok {
		return "", "", fmt.Errorf("secret %q has no key %q", secret.Name, config.Spec.RootCredentials.PasswordKey)
	}

	return username, string(password), nil
}

// setCondition records the reconciliation outcome for the current generation.
// Errors are logged and returned, never masking the error that led here.
func (r *ConfigReconciler) setCondition(ctx context.Context, config *databasev1beta1.Config, status metav1.ConditionStatus, reason, message string) error {
	log := logf.FromContext(ctx)

	config.Status.ObservedGeneration = config.Generation
	meta.SetStatusCondition(&config.Status.Conditions, metav1.Condition{
		Type:               typeReadyConfig,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: config.Generation,
	})

	if err := r.Status().Update(ctx, config); err != nil {
		log.Error(err, "Failed to update Config status")
		return err
	}

	return nil
}

func configPath(config *databasev1beta1.Config) string {
	return fmt.Sprintf("/%s/config/%s", config.Spec.Path, config.Name)
}

func (r *ConfigReconciler) deleteVaultConfig(ctx context.Context, config *databasev1beta1.Config) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().DeleteWithContext(ctx, configPath(config))
	return err
}

func (r *ConfigReconciler) vaultConfigExists(ctx context.Context, config *databasev1beta1.Config) (bool, error) {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return false, err
	}

	s, err := c.Logical().ReadWithContext(ctx, configPath(config))
	if err != nil {
		return false, err
	}

	return s != nil, nil
}

func (r *ConfigReconciler) updateVaultConfig(ctx context.Context, config *databasev1beta1.Config, payload map[string]any) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().WriteWithContext(ctx, configPath(config), payload)
	return err
}

// configsForSecret lists the Config resources reading their root credentials
// from the given Secret.
func (r *ConfigReconciler) configsForSecret(ctx context.Context, secret client.Object) []reconcile.Request {
	configs := &databasev1beta1.ConfigList{}
	if err := r.List(ctx, configs,
		client.InNamespace(secret.GetNamespace()),
		client.MatchingFields{secretNameIndex: secret.GetName()},
	); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list Configs for Secret", "name", secret.GetName())
		return nil
	}

	requests := make([]reconcile.Request, 0, len(configs.Items))
	for _, config := range configs.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Namespace: config.Namespace, Name: config.Name},
		})
	}

	return requests
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &databasev1beta1.Config{}, secretNameIndex,
		func(o client.Object) []string {
			return []string{o.(*databasev1beta1.Config).Spec.RootCredentials.SecretName}
		},
	); err != nil {
		return err
	}

	// The manager only informs on Secrets carrying controller.SecretWatchLabel,
	// so this watch is already narrow and needs no predicate of its own.
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1beta1.Config{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.configsForSecret)).
		Named("database-config").
		Complete(r)
}
