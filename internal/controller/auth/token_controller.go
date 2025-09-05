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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	vaultapi "github.com/hashicorp/vault/api"

	authv1beta1 "hopopops/vault-operator/api/auth/v1beta1"
)

const (
	tokenFinalizer = "token.auth.toolkit.vault.hopopops.com/finalizer"
)

// Definitions to manage status conditions
const (
	typeConfiguredToken = "Configured"
)

// TokenReconciler reconciles a Token object
type TokenReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Vault  *vaultapi.Client
}

// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=tokens,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=tokens/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=tokens/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;create;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Token object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *TokenReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Token instance
	token := &authv1beta1.Token{}
	if err := r.Get(ctx, req.NamespacedName, token); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Token resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Token")
		return ctrl.Result{}, err
	}

	// Token Deletion
	isTokenMarkedToBeDeleted := token.GetDeletionTimestamp() != nil
	if isTokenMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(token, tokenFinalizer) {
			if token.Spec.Target.DeletionPolicy == "Delete" {
				if err := r.deleteK8sSecret(ctx, token); err != nil {
					log.Error(err, "Failed to delete Secret")
					return ctrl.Result{}, err
				}

				if err := r.Vault.Auth().Token().RevokeAccessorWithContext(ctx, token.Status.Accessor); err != nil {
					log.Error(err, "Failed to delete accessor")
					return ctrl.Result{}, err
				}
			}

			controllerutil.RemoveFinalizer(token, tokenFinalizer)
			if err := r.Update(ctx, token); err != nil {
				log.Error(err, "Failed to remove finalizer from Token")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Token Initialization
	if !controllerutil.ContainsFinalizer(token, tokenFinalizer) {
		controllerutil.AddFinalizer(token, tokenFinalizer)
		meta.SetStatusCondition(&token.Status.Conditions, metav1.Condition{Type: typeConfiguredToken, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err := r.Update(ctx, token); err != nil {
			log.Error(err, "Failed to initialize Token status")
			return ctrl.Result{}, err
		}
	}

	if token.Status.Accessor == "" {
		if t, err := r.Vault.Auth().Token().Create(&vaultapi.TokenCreateRequest{
			Policies:        token.Spec.Policies,
			Metadata:        token.Spec.Meta,
			TTL:             token.Spec.TTL,
			ExplicitMaxTTL:  token.Spec.ExplicitMaxTTL,
			Period:          token.Spec.Period,
			NoParent:        token.Spec.NoParent,
			NoDefaultPolicy: token.Spec.NoDefaultPolicy,
			DisplayName:     fmt.Sprintf("%s/%s", token.Namespace, token.Name),
			NumUses:         token.Spec.NumUses,
			Renewable:       &token.Spec.Renewable,
			Type:            token.Spec.Type,
			EntityAlias:     token.Spec.EntityAlias,
		}); err != nil {
			log.Error(err, "Failed to create Token")
			meta.SetStatusCondition(&token.Status.Conditions, metav1.Condition{Type: typeConfiguredToken, Status: metav1.ConditionFalse, Reason: "FailedToCreate", Message: "Failed to create token engine in Vault"})
			if err := r.Status().Update(ctx, token); err != nil {
				log.Error(err, "Failed to update Token status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		} else {
			if err := r.createK8sSecret(ctx, token, t.Auth.ClientToken); err != nil {
				log.Error(err, "Failed to create k8s secret")
				meta.SetStatusCondition(&token.Status.Conditions, metav1.Condition{Type: typeConfiguredToken, Status: metav1.ConditionFalse, Reason: "FailedToCreate", Message: fmt.Sprintf("Failed to create k8s secret %s", token.Spec.Target.Name)})
				if err := r.Status().Update(ctx, token); err != nil {
					log.Error(err, "Failed to update Token status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, err
			}

			token.Status.Accessor = t.Auth.Accessor
			meta.SetStatusCondition(&token.Status.Conditions, metav1.Condition{Type: typeConfiguredToken, Status: metav1.ConditionTrue, Reason: "Configured", Message: "Successfully created token engine in Vault"})
			if err := r.Status().Update(ctx, token); err != nil {
				log.Error(err, "Failed to update Token status")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *TokenReconciler) deleteK8sSecret(ctx context.Context, token *authv1beta1.Token) error {
	secret := &corev1.Secret{}

	err := r.Get(ctx, types.NamespacedName{
		Name:      token.Spec.Target.Name,
		Namespace: token.Namespace,
	}, secret)

	if err != nil && apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	for _, owner := range secret.GetOwnerReferences() {
		if owner.UID == token.UID {
			if err := r.Delete(ctx, secret); err != nil {
				return fmt.Errorf("failed to delete secret: %w", err)
			}
			return nil
		}
	}

	ctrl.Log.Info("Secret exists but CR is not owner, skipping deletion", "secret", token.Spec.Target.Name)
	return nil
}

func (r *TokenReconciler) createK8sSecret(ctx context.Context, token *authv1beta1.Token, data string) error {
	log := logf.FromContext(ctx)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      token.Spec.Target.Name,
			Namespace: token.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"token": []byte(data),
		},
	}

	if err := controllerutil.SetControllerReference(token, secret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	existingSecret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      secret.Name,
		Namespace: secret.Namespace,
	}, existingSecret)

	if err != nil && apierrors.IsNotFound(err) {
		if err := r.Create(ctx, secret); err != nil {
			return err
		}
		log.Info("Created secret", "secret", secret.Name)
		return nil
	}

	if err != nil {
		return err
	}

	return apierrors.NewAlreadyExists(corev1.Resource("secrets"), secret.Name)
}

func (r *TokenReconciler) deleteVaultToken(ctx context.Context, token *authv1beta1.Token) error {
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TokenReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&authv1beta1.Token{}).
		Named("auth-token").
		Complete(r)
}
