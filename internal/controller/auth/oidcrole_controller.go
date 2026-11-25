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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	authv1beta1 "hopopops/vault-operator/api/auth/v1beta1"
	"hopopops/vault-operator/internal/connector/vault"
)

const (
	oidcRoleFinalizer = "oidcrole.auth.toolkit.vault.hopopops.com/finalizer"

	typeReadyOIDCRole = "Ready"
)

// OIDCRoleReconciler reconciles a OIDCRole object
type OIDCRoleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Vault  *vault.Vault
}

// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=oidcroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=oidcroles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=oidcroles/finalizers,verbs=update

func (r *OIDCRoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	role := &authv1beta1.OIDCRole{}
	if err := r.Get(ctx, req.NamespacedName, role); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get OIDCRole")
		return ctrl.Result{}, err
	}

	if !role.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(role, oidcRoleFinalizer) {
			if err := r.deleteVaultRole(ctx, role); err != nil {
				log.Error(err, "Failed to delete OIDCRole")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(role, oidcRoleFinalizer)
			if err := r.Update(ctx, role); err != nil {
				log.Error(err, "Failed to remove finalizer from OIDCRole")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(role, oidcRoleFinalizer) {
		controllerutil.AddFinalizer(role, oidcRoleFinalizer)
		if err := r.Update(ctx, role); err != nil {
			log.Error(err, "Failed to add finalizer to OIDCRole")
			return ctrl.Result{}, err
		}
	}

	// A plain write, pushed unconditionally: comparing against Vault's read-back
	// would mean normalising every default it fills in.
	if err := r.writeVaultRole(ctx, role); err != nil {
		log.Error(err, "Failed to update OIDCRole")
		_ = r.setCondition(ctx, role, metav1.ConditionFalse, "FailedToUpdate", "Failed to push oidc auth role to Vault")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.setCondition(ctx, role, metav1.ConditionTrue, "Configured", "Successfully pushed oidc auth role to Vault")
}

func (r *OIDCRoleReconciler) setCondition(ctx context.Context, role *authv1beta1.OIDCRole, status metav1.ConditionStatus, reason, message string) error {
	role.Status.ObservedGeneration = role.Generation
	meta.SetStatusCondition(&role.Status.Conditions, metav1.Condition{
		Type:               typeReadyOIDCRole,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: role.Generation,
	})

	if err := r.Status().Update(ctx, role); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to update OIDCRole status")
		return err
	}

	return nil
}

func oidcRolePath(role *authv1beta1.OIDCRole) string {
	return fmt.Sprintf("/auth/%s/role/%s", role.Spec.AuthPath, role.Name)
}

func (r *OIDCRoleReconciler) deleteVaultRole(ctx context.Context, role *authv1beta1.OIDCRole) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().DeleteWithContext(ctx, oidcRolePath(role))
	return vault.IgnoreNotFound(err)
}

func (r *OIDCRoleReconciler) writeVaultRole(ctx context.Context, role *authv1beta1.OIDCRole) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().WriteWithContext(ctx, oidcRolePath(role), vault.OIDCRolePayload(&role.Spec))
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *OIDCRoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&authv1beta1.OIDCRole{}).
		Named("auth-oidcrole").
		Complete(r)
}
