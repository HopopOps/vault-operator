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
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	databasev1beta1 "hopopops/vault-operator/api/database/v1beta1"
	"hopopops/vault-operator/internal/connector/vault"
)

const (
	roleFinalizer = "role.database.toolkit.vault.hopopops.com/finalizer"
)

// Definitions to manage status conditions
const (
	typeReadyRole = "Ready"
)

// RoleReconciler reconciles a Role object
type RoleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Vault  *vault.Vault
}

// +kubebuilder:rbac:groups=database.toolkit.vault.hopopops.com,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=database.toolkit.vault.hopopops.com,resources=roles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=database.toolkit.vault.hopopops.com,resources=roles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *RoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Role instance
	role := &databasev1beta1.Role{}
	if err := r.Get(ctx, req.NamespacedName, role); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Role resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Role")
		return ctrl.Result{}, err
	}

	if role.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(role, roleFinalizer) {
			// Initialize finalizer
			controllerutil.AddFinalizer(role, roleFinalizer)
			if err := r.Update(ctx, role); err != nil {
				log.Error(err, "Failed to add finalizer to Role")
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(role, roleFinalizer) {
			// Delete managed resources for this Role
			if err := r.deleteVaultRole(ctx, role); err != nil {
				log.Error(err, "Failed to delete Role")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(role, roleFinalizer)
			if err := r.Update(ctx, role); err != nil {
				log.Error(err, "Failed to remove finalizer from Role")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Create or update
	current, err := r.fetchVaultRole(ctx, role)
	if err != nil {
		log.Error(err, "Failed to fetch Role")
		_ = r.setCondition(ctx, role, metav1.ConditionFalse, "FailedToFetch", "Failed to fetch database secret engine role from Vault")
		return ctrl.Result{}, err
	}

	desired := vault.DesiredDatabaseRole(&role.Spec)
	if desired.IsDifferentFrom(current) {
		if err := r.updateVaultRole(ctx, role, desired); err != nil {
			log.Error(err, "Failed to update Role")
			_ = r.setCondition(ctx, role, metav1.ConditionFalse, "FailedToUpdate", "Failed to push database secret engine role to Vault")
			return ctrl.Result{}, err
		}
	}

	// Always record the outcome, the role may already have been in sync.
	return ctrl.Result{}, r.setCondition(ctx, role, metav1.ConditionTrue, "Configured", "Successfully pushed database secret engine role to Vault")
}

// setCondition records the reconciliation outcome for the current generation.
// Errors are logged and returned, never masking the error that led here.
func (r *RoleReconciler) setCondition(ctx context.Context, role *databasev1beta1.Role, status metav1.ConditionStatus, reason, message string) error {
	log := logf.FromContext(ctx)

	role.Status.ObservedGeneration = role.Generation
	meta.SetStatusCondition(&role.Status.Conditions, metav1.Condition{
		Type:               typeReadyRole,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: role.Generation,
	})

	if err := r.Status().Update(ctx, role); err != nil {
		log.Error(err, "Failed to update Role status")
		return err
	}

	return nil
}

func rolePath(role *databasev1beta1.Role) string {
	return fmt.Sprintf("/%s/roles/%s", role.Spec.Path, role.Name)
}

func (r *RoleReconciler) deleteVaultRole(ctx context.Context, role *databasev1beta1.Role) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().DeleteWithContext(ctx, rolePath(role))
	return err
}

// fetchVaultRole returns the role held by Vault, or nil when there is none.
func (r *RoleReconciler) fetchVaultRole(ctx context.Context, role *databasev1beta1.Role) (*vault.DatabaseRole, error) {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return nil, err
	}

	s, err := c.Logical().ReadWithContext(ctx, rolePath(role))
	if err != nil || s == nil {
		return nil, err
	}

	jsonBytes, err := json.Marshal(s.Data)
	if err != nil {
		return nil, err
	}

	var current vault.DatabaseRole
	if err := json.Unmarshal(jsonBytes, &current); err != nil {
		return nil, err
	}

	return &current, nil
}

func (r *RoleReconciler) updateVaultRole(ctx context.Context, role *databasev1beta1.Role, desired *vault.DatabaseRole) error {
	jsonBytes, err := json.Marshal(desired)
	if err != nil {
		return err
	}

	var payload map[string]any
	if err := json.Unmarshal(jsonBytes, &payload); err != nil {
		return err
	}

	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().WriteWithContext(ctx, rolePath(role), payload)
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *RoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&databasev1beta1.Role{}).
		Named("database-role").
		Complete(r)
}
