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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	authv1beta1 "hopopops/vault-operator/api/auth/v1beta1"
	"hopopops/vault-operator/internal/connector/vault"
	"hopopops/vault-operator/internal/controller"
)

// clientSecretNameIndex indexes OIDCConfig resources by the Secret they read
// their client secret from, so a change to that Secret maps back to them.
const clientSecretNameIndex = "spec.clientSecret.secretName"

const typeReadyOIDCConfig = "Ready"

// OIDCConfigReconciler reconciles a OIDCConfig object.
//
// There is no finalizer: the config endpoint has no delete operation, the
// configuration goes away with its mount.
type OIDCConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Vault  *vault.Vault
}

// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=oidcconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=oidcconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *OIDCConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	config := &authv1beta1.OIDCConfig{}
	if err := r.Get(ctx, req.NamespacedName, config); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get OIDCConfig")
		return ctrl.Result{}, err
	}

	if !config.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	secret, err := controller.WatchedSecret(ctx, r, types.NamespacedName{
		Namespace: config.Namespace,
		Name:      config.Spec.ClientSecret.SecretName,
	})
	if err == nil {
		if _, ok := secret.Data[config.Spec.ClientSecret.Key]; !ok {
			err = fmt.Errorf("secret %q has no key %q", secret.Name, config.Spec.ClientSecret.Key)
		}
	}
	if err != nil {
		log.Error(err, "Failed to read client secret for OIDCConfig")
		_ = r.setCondition(ctx, config, metav1.ConditionFalse, "MissingCredentials", "Failed to read the client secret Secret")
		return ctrl.Result{}, err
	}

	payload := vault.OIDCConfigPayload(&config.Spec, string(secret.Data[config.Spec.ClientSecret.Key]))
	hash, err := vault.Hash(payload)
	if err != nil {
		log.Error(err, "Failed to hash OIDCConfig payload")
		return ctrl.Result{}, err
	}

	// Vault never returns the client secret, so a read only tells whether a
	// config exists (a recreated mount comes back empty); the hash tells
	// whether it is current.
	exists, err := r.vaultConfigExists(ctx, config)
	if err != nil {
		log.Error(err, "Failed to fetch OIDCConfig")
		_ = r.setCondition(ctx, config, metav1.ConditionFalse, "FailedToFetch", "Failed to fetch oidc auth config from Vault")
		return ctrl.Result{}, err
	}

	if !exists || config.Status.ConfigHash != hash {
		if err := r.writeVaultConfig(ctx, config, payload); err != nil {
			log.Error(err, "Failed to update OIDCConfig")
			_ = r.setCondition(ctx, config, metav1.ConditionFalse, "FailedToUpdate", "Failed to push oidc auth config to Vault")
			return ctrl.Result{}, err
		}

		log.Info("Pushed oidc auth config to Vault", "path", oidcConfigPath(config))
		config.Status.ConfigHash = hash
	}

	return ctrl.Result{}, r.setCondition(ctx, config, metav1.ConditionTrue, "Configured", "Successfully pushed oidc auth config to Vault")
}

func (r *OIDCConfigReconciler) setCondition(ctx context.Context, config *authv1beta1.OIDCConfig, status metav1.ConditionStatus, reason, message string) error {
	config.Status.ObservedGeneration = config.Generation
	meta.SetStatusCondition(&config.Status.Conditions, metav1.Condition{
		Type:               typeReadyOIDCConfig,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: config.Generation,
	})

	if err := r.Status().Update(ctx, config); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to update OIDCConfig status")
		return err
	}

	return nil
}

func oidcConfigPath(config *authv1beta1.OIDCConfig) string {
	return fmt.Sprintf("/auth/%s/config", config.Spec.AuthPath)
}

// vaultConfigExists reports whether the mount holds a config. An unconfigured
// mount answers the read with an empty response.
func (r *OIDCConfigReconciler) vaultConfigExists(ctx context.Context, config *authv1beta1.OIDCConfig) (bool, error) {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return false, err
	}

	s, err := c.Logical().ReadWithContext(ctx, oidcConfigPath(config))
	if err != nil {
		return false, err
	}

	return s != nil, nil
}

func (r *OIDCConfigReconciler) writeVaultConfig(ctx context.Context, config *authv1beta1.OIDCConfig, payload map[string]any) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().WriteWithContext(ctx, oidcConfigPath(config), payload)
	return err
}

func (r *OIDCConfigReconciler) configsForSecret(ctx context.Context, secret client.Object) []reconcile.Request {
	configs := &authv1beta1.OIDCConfigList{}
	if err := r.List(ctx, configs,
		client.InNamespace(secret.GetNamespace()),
		client.MatchingFields{clientSecretNameIndex: secret.GetName()},
	); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list OIDCConfigs for Secret", "name", secret.GetName())
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
func (r *OIDCConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &authv1beta1.OIDCConfig{}, clientSecretNameIndex,
		func(o client.Object) []string {
			return []string{o.(*authv1beta1.OIDCConfig).Spec.ClientSecret.SecretName}
		},
	); err != nil {
		return err
	}

	// The manager only informs on Secrets carrying controller.SecretWatchLabel.
	return ctrl.NewControllerManagedBy(mgr).
		For(&authv1beta1.OIDCConfig{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.configsForSecret)).
		Named("auth-oidcconfig").
		Complete(r)
}
