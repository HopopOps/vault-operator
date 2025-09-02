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

package sys

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	vaultapi "github.com/hashicorp/vault/api"

	sysv1beta1 "hopopops/vault-operator/api/sys/v1beta1"
)

const (
	authFinalizer = "auth.sys.toolkit.vault.hopopops.com/finalizer"
)

// Definitions to manage status conditions
const (
	typeConfiguredAuth = "Configured"
)

// AuthReconciler reconciles a Auth object
type AuthReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Vault  *vaultapi.Client
}

// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=auths,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=auths/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=auths/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Auth object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *AuthReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Auth instance
	auth := &sysv1beta1.Auth{}
	if err := r.Get(ctx, req.NamespacedName, auth); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Auth resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Auth")
		return ctrl.Result{}, err
	}

	if len(auth.Status.Conditions) == 0 {
		meta.SetStatusCondition(&auth.Status.Conditions, metav1.Condition{Type: typeConfiguredAuth, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err := r.Status().Update(ctx, auth); err != nil {
			log.Error(err, "Failed to create Auth status")
			return ctrl.Result{}, err
		}

		if err := r.Get(ctx, req.NamespacedName, auth); err != nil {
			log.Error(err, "Failed to re-fetch Auth")
			return ctrl.Result{}, err
		}
	}

	if auth.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(auth, authFinalizer) {
			// Initialize finalizer
			controllerutil.AddFinalizer(auth, authFinalizer)
			if err := r.Update(ctx, auth); err != nil {
				log.Error(err, "Failed to add finalizer to Auth")
				return ctrl.Result{}, err
			}

			if err := r.Get(ctx, req.NamespacedName, auth); err != nil {
				log.Error(err, "Failed to re-fetch Auth")
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(auth, authFinalizer) {
			// Delete managed resources for this Auth
			if err := r.deleteVaultAuth(ctx, auth); err != nil {
				log.Error(err, "Failed to delete Auth")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(auth, authFinalizer)
			if err := r.Update(ctx, auth); err != nil {
				log.Error(err, "Failed to remove finalizer from Auth")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Create, do not allow update
	if auth.Status.Accessor == "" {
		if err := r.createVaultAuth(ctx, auth); err != nil {
			log.Error(err, "Failed to create Auth")
			meta.SetStatusCondition(&auth.Status.Conditions, metav1.Condition{Type: typeConfiguredAuth, Status: metav1.ConditionFalse, Reason: "FailedToCreate", Message: "Failed to create auth engine in Vault"})
			if err := r.Status().Update(ctx, auth); err != nil {
				log.Error(err, "Failed to update Auth status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		ae, err := r.Vault.Sys().GetAuthWithContext(ctx, auth.Name)
		if err != nil {
			log.Error(err, "Failed to get auth engine from Vault")
			return ctrl.Result{}, err
		}

		// Set accessor for reference
		auth.Status.Accessor = ae.Accessor
		meta.SetStatusCondition(&auth.Status.Conditions, metav1.Condition{Type: typeConfiguredAuth, Status: metav1.ConditionTrue, Reason: "Configured", Message: "Successfully created auth engine in Vault"})
		if err := r.Status().Update(ctx, auth); err != nil {
			log.Error(err, "Failed to update Auth status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *AuthReconciler) deleteVaultAuth(ctx context.Context, auth *sysv1beta1.Auth) error {
	return r.Vault.Sys().DisableAuthWithContext(ctx, auth.Name)
}

func (r *AuthReconciler) createVaultAuth(ctx context.Context, auth *sysv1beta1.Auth) error {
	return r.Vault.Sys().EnableAuthWithOptionsWithContext(ctx, auth.Name, &vaultapi.EnableAuthOptions{
		Type:        *auth.Spec.Type,
		Description: *auth.Spec.Description,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sysv1beta1.Auth{}).
		Named("sys-auth").
		Complete(r)
}
