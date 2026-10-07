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

	sysv1beta1 "hopopops/vault-operator/api/sys/v1beta1"
	"hopopops/vault-operator/internal/connector/vault"
)

const (
	passwordPolicyFinalizer = "passwordpolicy.sys.toolkit.vault.hopopops.com/finalizer"
)

// Definitions to manage status conditions
const (
	typeReadyPasswordPolicy = "Ready"
)

// PasswordPolicyReconciler reconciles a PasswordPolicy object
type PasswordPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Vault  *vault.Vault
}

// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=passwordpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=passwordpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=passwordpolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *PasswordPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the PasswordPolicy instance
	policy := &sysv1beta1.PasswordPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("PasswordPolicy resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PasswordPolicy")
		return ctrl.Result{}, err
	}

	if policy.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(policy, passwordPolicyFinalizer) {
			// Initialize finalizer
			controllerutil.AddFinalizer(policy, passwordPolicyFinalizer)
			if err := r.Update(ctx, policy); err != nil {
				log.Error(err, "Failed to add finalizer to PasswordPolicy")
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(policy, passwordPolicyFinalizer) {
			// Delete managed resources for this PasswordPolicy
			if err := r.deleteVaultPasswordPolicy(ctx, policy); err != nil {
				log.Error(err, "Failed to delete PasswordPolicy")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(policy, passwordPolicyFinalizer)
			if err := r.Update(ctx, policy); err != nil {
				log.Error(err, "Failed to remove finalizer from PasswordPolicy")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Create or update
	current, err := r.fetchVaultPasswordPolicy(ctx, policy)
	if err != nil {
		log.Error(err, "Failed to fetch PasswordPolicy")
		_ = r.setCondition(ctx, policy, metav1.ConditionFalse, "FailedToFetch", "Failed to fetch password policy from Vault")
		return ctrl.Result{}, err
	}

	if current != *policy.Spec.Policy {
		if err := r.updateVaultPasswordPolicy(ctx, policy); err != nil {
			log.Error(err, "Failed to update PasswordPolicy")
			_ = r.setCondition(ctx, policy, metav1.ConditionFalse, "FailedToUpdate", "Failed to push password policy to Vault")
			return ctrl.Result{}, err
		}
	}

	// Always record the outcome, the policy may already have been in sync.
	return ctrl.Result{}, r.setCondition(ctx, policy, metav1.ConditionTrue, "Configured", "Successfully pushed password policy to Vault")
}

// setCondition records the reconciliation outcome for the current generation.
// Errors are logged and returned, never masking the error that led here.
func (r *PasswordPolicyReconciler) setCondition(ctx context.Context, policy *sysv1beta1.PasswordPolicy, status metav1.ConditionStatus, reason, message string) error {
	log := logf.FromContext(ctx)

	policy.Status.ObservedGeneration = policy.Generation
	meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
		Type:               typeReadyPasswordPolicy,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: policy.Generation,
	})

	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update PasswordPolicy status")
		return err
	}

	return nil
}

func passwordPolicyPath(policy *sysv1beta1.PasswordPolicy) string {
	return fmt.Sprintf("/sys/policies/password/%s", policy.Name)
}

func (r *PasswordPolicyReconciler) deleteVaultPasswordPolicy(ctx context.Context, policy *sysv1beta1.PasswordPolicy) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().DeleteWithContext(ctx, passwordPolicyPath(policy))
	return err
}

// fetchVaultPasswordPolicy returns the policy document held by Vault, or an
// empty string when there is none.
func (r *PasswordPolicyReconciler) fetchVaultPasswordPolicy(ctx context.Context, policy *sysv1beta1.PasswordPolicy) (string, error) {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return "", err
	}

	s, err := c.Logical().ReadWithContext(ctx, passwordPolicyPath(policy))
	if err != nil || s == nil {
		return "", err
	}

	current, _ := s.Data["policy"].(string)
	return current, nil
}

func (r *PasswordPolicyReconciler) updateVaultPasswordPolicy(ctx context.Context, policy *sysv1beta1.PasswordPolicy) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	_, err = c.Logical().WriteWithContext(ctx, passwordPolicyPath(policy), map[string]any{
		"policy": *policy.Spec.Policy,
	})
	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *PasswordPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sysv1beta1.PasswordPolicy{}).
		Named("sys-passwordpolicy").
		Complete(r)
}
