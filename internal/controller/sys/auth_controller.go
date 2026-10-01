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
	"fmt"

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
	"hopopops/vault-operator/internal/connector/vault"
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
	Vault  *vault.Vault
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
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
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

	// Auth Deletion
	isAuthMarkedToBeDeleted := auth.GetDeletionTimestamp() != nil
	if isAuthMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(auth, authFinalizer) {
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

	// Auth Initialization
	if !controllerutil.ContainsFinalizer(auth, authFinalizer) {
		controllerutil.AddFinalizer(auth, authFinalizer)
		if err := r.Update(ctx, auth); err != nil {
			log.Error(err, "Failed to add finalizer to Auth")
			return ctrl.Result{}, err
		}
	}

	// Create, do not allow update
	if auth.Status.Accessor == "" {
		if err := r.createVaultAuth(ctx, auth); err != nil {
			log.Error(err, "Failed to create Auth")
			_ = r.setCondition(ctx, auth, metav1.ConditionFalse, "FailedToCreate", "Failed to create auth engine in Vault")
			return ctrl.Result{}, err
		}

		c, err := r.Vault.Client(ctx)
		if err != nil {
			log.Error(err, "Failed to get a Vault client")
			_ = r.setCondition(ctx, auth, metav1.ConditionFalse, "FailedToFetch", "Failed to fetch auth engine from Vault")
			return ctrl.Result{}, err
		}

		ae, err := c.Sys().GetAuthWithContext(ctx, auth.Name)
		if err != nil {
			log.Error(err, "Failed to get auth engine from Vault")
			_ = r.setCondition(ctx, auth, metav1.ConditionFalse, "FailedToFetch", "Failed to fetch auth engine from Vault")
			return ctrl.Result{}, err
		}

		// Set accessor for reference
		auth.Status.Accessor = ae.Accessor
	}

	// Always record the outcome, the auth engine may already have been created.
	return ctrl.Result{}, r.setCondition(ctx, auth, metav1.ConditionTrue, "Configured", "Successfully created auth engine in Vault")
}

// setCondition records the reconciliation outcome for the current generation.
// Errors are logged and returned, never masking the error that led here.
func (r *AuthReconciler) setCondition(ctx context.Context, auth *sysv1beta1.Auth, status metav1.ConditionStatus, reason, message string) error {
	log := logf.FromContext(ctx)

	auth.Status.ObservedGeneration = auth.Generation
	meta.SetStatusCondition(&auth.Status.Conditions, metav1.Condition{
		Type:               typeConfiguredAuth,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: auth.Generation,
	})

	if err := r.Status().Update(ctx, auth); err != nil {
		log.Error(err, "Failed to update Auth status")
		return err
	}

	return nil
}

func (r *AuthReconciler) deleteVaultAuth(ctx context.Context, auth *sysv1beta1.Auth) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	return c.Sys().DisableAuthWithContext(ctx, fmt.Sprintf("%s/", auth.Name))
}

func (r *AuthReconciler) createVaultAuth(ctx context.Context, auth *sysv1beta1.Auth) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	return c.Sys().EnableAuthWithOptionsWithContext(ctx, fmt.Sprintf("%s/", auth.Name), &vaultapi.EnableAuthOptions{
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
