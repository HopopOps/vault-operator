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

	authv1beta1 "hopopops/vault-operator/api/auth/v1beta1"
	"hopopops/vault-operator/internal/connector/vault"
)

const (
	roleFinalizer = "kubernetesrole.auth.toolkit.vault.hopopops.com/finalizer"
)

// Definitions to manage status conditions
const (
	typeConfiguredRole = "Configured"
)

// KubernetesRoleReconciler reconciles a KubernetesRole object
type KubernetesRoleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Vault  *vault.Vault
}

// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=kubernetesroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=kubernetesroles/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=auth.toolkit.vault.hopopops.com,resources=kubernetesroles/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *KubernetesRoleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the KubernetesRole instance
	role := &authv1beta1.KubernetesRole{}
	if err := r.Get(ctx, req.NamespacedName, role); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("KubernetesRole resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get KubernetesRole")
		return ctrl.Result{}, err
	}

	if role.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(role, roleFinalizer) {
			// Initialize finalizer
			controllerutil.AddFinalizer(role, roleFinalizer)
			if err := r.Update(ctx, role); err != nil {
				log.Error(err, "Failed to add finalizer to KubernetesRole")
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(role, roleFinalizer) {
			// Delete managed resources for this KubernetesRole
			if err := r.deleteVaultKubernetesRole(ctx, role); err != nil {
				log.Error(err, "Failed to delete KubernetesRole")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(role, roleFinalizer)
			if err := r.Update(ctx, role); err != nil {
				log.Error(err, "Failed to remove finalizer from KubernetesRole")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Create or update
	kr, err := r.fetchVaultKubernetesRole(ctx, role)
	if err != nil {
		log.Error(err, "Failed to fetch KubernetesRole")
		_ = r.setCondition(ctx, role, metav1.ConditionFalse, "FailedToFetch", "Failed to fetch kubernetes auth engine role from Vault")
		return ctrl.Result{}, err
	}

	if kr == nil || kr.IsDifferentFromSpec(&role.Spec) {
		if err := r.updateVaultKubernetesRole(ctx, role); err != nil {
			log.Error(err, "Failed to update KubernetesRole")
			_ = r.setCondition(ctx, role, metav1.ConditionFalse, "FailedToUpdate", "Failed to push kubernetes auth engine role to Vault")
			return ctrl.Result{}, err
		}
	}

	// Always record the outcome, the role may already have been in sync.
	return ctrl.Result{}, r.setCondition(ctx, role, metav1.ConditionTrue, "Configured", "Successfully pushed kubernetes auth engine role to Vault")
}

// setCondition records the reconciliation outcome for the current generation.
// Errors are logged and returned, never masking the error that led here.
func (r *KubernetesRoleReconciler) setCondition(ctx context.Context, role *authv1beta1.KubernetesRole, status metav1.ConditionStatus, reason, message string) error {
	log := logf.FromContext(ctx)

	role.Status.ObservedGeneration = role.Generation
	meta.SetStatusCondition(&role.Status.Conditions, metav1.Condition{
		Type:               typeConfiguredRole,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: role.Generation,
	})

	if err := r.Status().Update(ctx, role); err != nil {
		log.Error(err, "Failed to update KubernetesRole status")
		return err
	}

	return nil
}

func (r *KubernetesRoleReconciler) deleteVaultKubernetesRole(ctx context.Context, role *authv1beta1.KubernetesRole) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().DeleteWithContext(ctx, fmt.Sprintf("/auth/%s/role/%s", role.Spec.AuthPath, role.Name))
	return err
}

func (r *KubernetesRoleReconciler) fetchVaultKubernetesRole(ctx context.Context, role *authv1beta1.KubernetesRole) (*vault.KubernetesRole, error) {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return nil, err
	}

	s, err := c.Logical().ReadWithContext(ctx, fmt.Sprintf("/auth/%s/role/%s", role.Spec.AuthPath, role.Name))
	if err != nil {
		// TODO: "not found" should not be an error
		return nil, err
	}

	if s == nil {
		return nil, nil
	}

	jsonBytes, err := json.Marshal(s.Data)
	if err != nil {
		return nil, err
	}

	var kr vault.KubernetesRole
	if err := json.Unmarshal(jsonBytes, &kr); err != nil {
		return nil, err
	}

	return &kr, nil
}

func (r *KubernetesRoleReconciler) updateVaultKubernetesRole(ctx context.Context, role *authv1beta1.KubernetesRole) error {
	jsonBytes, err := json.Marshal(&vault.KubernetesRole{
		BoundServiceAccountNames:      role.Spec.BoundServiceAccountNames,
		BoundServiceAccountNamespaces: role.Spec.BoundServiceAccountNamespaces,
		Audience:                      role.Spec.Audience,
		AliasNameSource:               role.Spec.AliasNameSource,
		TokenTTL:                      role.Spec.TokenTTL,
		TokenMaxTTL:                   role.Spec.TokenMaxTTL,
		TokenPolicies:                 role.Spec.TokenPolicies,
		TokenBoundCIDRs:               role.Spec.TokenBoundCIDRs,
		TokenExplicitMaxTTL:           role.Spec.TokenExplicitMaxTTL,
		TokenNoDefaultPolicy:          role.Spec.TokenNoDefaultPolicy,
		TokenNumUses:                  role.Spec.TokenNumUses,
		TokenPeriod:                   role.Spec.TokenPeriod,
		TokenType:                     role.Spec.TokenType,
	})
	if err != nil {
		return err
	}

	var m map[string]any
	if err = json.Unmarshal(jsonBytes, &m); err != nil {
		return err
	}

	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().WriteWithContext(ctx, fmt.Sprintf("/auth/%s/role/%s", role.Spec.AuthPath, role.Name), m)
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *KubernetesRoleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&authv1beta1.KubernetesRole{}).
		Named("auth-kubernetesrole").
		Complete(r)
}
