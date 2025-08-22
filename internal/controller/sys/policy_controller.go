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
	"hopopops/vault-operator/internal/connector/vault"
)

const (
	policyFinalizer = "policy.sys.toolkit.vault.hopopops.com/finalizer"
)

// Definitions to manage status conditions
const (
	typeConfiguredPolicy = "Configured"
)

// PolicyReconciler reconciles a Policy object
type PolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Vault  *vaultapi.Client
}

// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=policies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=policies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=policies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Policy instance
	policy := &sysv1beta1.Policy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Policy resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Policy")
		return ctrl.Result{}, err
	}

	if len(policy.Status.Conditions) == 0 {
		meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{Type: typeConfiguredPolicy, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err := r.Status().Update(ctx, policy); err != nil {
			log.Error(err, "Failed to update Policy status")
			return ctrl.Result{}, err
		}

		if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
			log.Error(err, "Failed to re-fetch Policy")
			return ctrl.Result{}, err
		}
	}

	if policy.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(policy, policyFinalizer) {
			// Initialize finalizer
			controllerutil.AddFinalizer(policy, policyFinalizer)
			if err := r.Update(ctx, policy); err != nil {
				log.Error(err, "Failed to add finalizer to Policy")
				return ctrl.Result{}, err
			}

			if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
				log.Error(err, "Failed to re-fetch Policy")
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(policy, policyFinalizer) {
			// Delete managed resources for this Policy
			if err := r.deleteVaultPolicy(ctx, policy); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(policy, policyFinalizer)
			if err := r.Update(ctx, policy); err != nil {
				log.Error(err, "Failed to remove finalizer from Policy")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Create or update
	if p, err := r.fetchVaultPolicy(ctx, policy); err != nil {
		meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{Type: typeConfiguredPolicy, Status: metav1.ConditionFalse, Reason: "FailedToFetch", Message: "Failed to fetch policy from Vault"})
		if err := r.Status().Update(ctx, policy); err != nil {
			log.Error(err, "Failed to update Policy status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, err
	} else {
		changed := p == nil || p.Name != policy.Name || p.Policy != *policy.Spec.Policy

		if changed {
			if err := r.updateVaultPolicy(ctx, policy); err != nil {
				meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{Type: typeConfiguredPolicy, Status: metav1.ConditionFalse, Reason: "FailedToUpdate", Message: "Failed to push policy to Vault"})
				if err := r.Status().Update(ctx, policy); err != nil {
					log.Error(err, "Failed to update Policy status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{Type: typeConfiguredPolicy, Status: metav1.ConditionTrue, Reason: "Configured", Message: "Successfully pushed policy to Vault"})
			if err := r.Status().Update(ctx, policy); err != nil {
				log.Error(err, "Failed to update Policy status")
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *PolicyReconciler) deleteVaultPolicy(ctx context.Context, policy *sysv1beta1.Policy) error {
	return r.Vault.Sys().DeletePolicyWithContext(ctx, policy.Name)
}

func (r *PolicyReconciler) fetchVaultPolicy(ctx context.Context, policy *sysv1beta1.Policy) (*vault.Policy, error) {
	content, err := r.Vault.Sys().GetPolicyWithContext(ctx, policy.Name)
	if err != nil {
		return nil, err
	}

	// TODO: "not found" should not be an error
	if content == "" {
		return &vault.Policy{Name: "", Policy: ""}, nil
	}

	return &vault.Policy{Name: policy.Name, Policy: content}, nil
}

func (r *PolicyReconciler) updateVaultPolicy(ctx context.Context, policy *sysv1beta1.Policy) error {
	return r.Vault.Sys().PutPolicyWithContext(ctx, policy.Name, *policy.Spec.Policy)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sysv1beta1.Policy{}).
		Named("sys-policy").
		Complete(r)
}
