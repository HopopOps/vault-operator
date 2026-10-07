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
	mountFinalizer = "mount.sys.toolkit.vault.hopopops.com/finalizer"
)

// Definitions to manage status conditions
const (
	typeReadyMount = "Ready"
)

// MountReconciler reconciles a Mount object
type MountReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Vault  *vault.Vault
}

// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=mounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=mounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sys.toolkit.vault.hopopops.com,resources=mounts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.25.0/pkg/reconcile
func (r *MountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the Mount instance
	mount := &sysv1beta1.Mount{}
	if err := r.Get(ctx, req.NamespacedName, mount); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Mount resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Mount")
		return ctrl.Result{}, err
	}

	if mount.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(mount, mountFinalizer) {
			// Initialize finalizer
			controllerutil.AddFinalizer(mount, mountFinalizer)
			if err := r.Update(ctx, mount); err != nil {
				log.Error(err, "Failed to add finalizer to Mount")
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(mount, mountFinalizer) {
			// Delete managed resources for this Mount
			if err := r.deleteVaultMount(ctx, mount); err != nil {
				log.Error(err, "Failed to delete Mount")
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(mount, mountFinalizer)
			if err := r.Update(ctx, mount); err != nil {
				log.Error(err, "Failed to remove finalizer from Mount")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	existing, err := r.fetchVaultMount(ctx, mount)
	if err != nil {
		log.Error(err, "Failed to fetch Mount")
		_ = r.setCondition(ctx, mount, metav1.ConditionFalse, "FailedToFetch", "Failed to fetch secret engine from Vault")
		return ctrl.Result{}, err
	}

	// Type and the replication flags cannot be changed once mounted, the spec
	// marks them immutable. Everything else lives in the tune endpoint.
	if existing == nil {
		if err := r.createVaultMount(ctx, mount); err != nil {
			log.Error(err, "Failed to create Mount")
			_ = r.setCondition(ctx, mount, metav1.ConditionFalse, "FailedToCreate", "Failed to create secret engine in Vault")
			return ctrl.Result{}, err
		}

		existing, err = r.fetchVaultMount(ctx, mount)
		if err != nil || existing == nil {
			log.Error(err, "Failed to fetch Mount")
			_ = r.setCondition(ctx, mount, metav1.ConditionFalse, "FailedToFetch", "Failed to fetch secret engine from Vault")
			return ctrl.Result{}, err
		}
	}

	// Tuning is a plain write, pushing it unconditionally is cheaper than
	// reconciling the string durations we send against the seconds Vault reads back.
	if err := r.tuneVaultMount(ctx, mount); err != nil {
		log.Error(err, "Failed to tune Mount")
		_ = r.setCondition(ctx, mount, metav1.ConditionFalse, "FailedToUpdate", "Failed to tune secret engine in Vault")
		return ctrl.Result{}, err
	}

	// Set accessor for reference
	mount.Status.Accessor = existing.Accessor

	return ctrl.Result{}, r.setCondition(ctx, mount, metav1.ConditionTrue, "Configured", "Successfully pushed secret engine to Vault")
}

// setCondition records the reconciliation outcome for the current generation.
// Errors are logged and returned, never masking the error that led here.
func (r *MountReconciler) setCondition(ctx context.Context, mount *sysv1beta1.Mount, status metav1.ConditionStatus, reason, message string) error {
	log := logf.FromContext(ctx)

	mount.Status.ObservedGeneration = mount.Generation
	meta.SetStatusCondition(&mount.Status.Conditions, metav1.Condition{
		Type:               typeReadyMount,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: mount.Generation,
	})

	if err := r.Status().Update(ctx, mount); err != nil {
		log.Error(err, "Failed to update Mount status")
		return err
	}

	return nil
}

// fetchVaultMount returns the mount, or nil when nothing is mounted at that
// path. The list endpoint is used because reading a path that holds no mount is
// an error rather than an empty response.
func (r *MountReconciler) fetchVaultMount(ctx context.Context, mount *sysv1beta1.Mount) (*vaultapi.MountOutput, error) {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return nil, err
	}

	mounts, err := c.Sys().ListMountsWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return mounts[fmt.Sprintf("%s/", mount.Name)], nil
}

func (r *MountReconciler) deleteVaultMount(ctx context.Context, mount *sysv1beta1.Mount) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	return c.Sys().UnmountWithContext(ctx, mount.Name)
}

func (r *MountReconciler) createVaultMount(ctx context.Context, mount *sysv1beta1.Mount) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	// The config is left out, the tune call that follows every create applies it.
	return c.Sys().MountWithContext(ctx, mount.Name, &vaultapi.MountInput{
		Type:                  *mount.Spec.Type,
		Description:           *mount.Spec.Description,
		Local:                 mount.Spec.Local,
		SealWrap:              mount.Spec.SealWrap,
		ExternalEntropyAccess: mount.Spec.ExternalEntropyAccess,
		Options:               mount.Spec.Options,
	})
}

func (r *MountReconciler) tuneVaultMount(ctx context.Context, mount *sysv1beta1.Mount) error {
	c, err := r.Vault.Client(ctx)
	if err != nil {
		return err
	}

	config := mount.Spec.Config
	options := mount.Spec.Options
	if options == nil {
		options = map[string]string{}
	}

	return c.Sys().TuneMountAllowNilWithContext(ctx, mount.Name, vaultapi.TuneMountConfigInput{
		Description:               mount.Spec.Description,
		Options:                   &options,
		DefaultLeaseTTL:           config.DefaultLeaseTTL,
		MaxLeaseTTL:               config.MaxLeaseTTL,
		ForceNoCache:              &config.ForceNoCache,
		ListingVisibility:         config.ListingVisibility,
		PluginVersion:             config.PluginVersion,
		IdentityTokenKey:          config.IdentityTokenKey,
		AuditNonHMACRequestKeys:   stringList(config.AuditNonHMACRequestKeys),
		AuditNonHMACResponseKeys:  stringList(config.AuditNonHMACResponseKeys),
		PassthroughRequestHeaders: stringList(config.PassthroughRequestHeaders),
		AllowedResponseHeaders:    stringList(config.AllowedResponseHeaders),
	})
}

// stringList addresses a list for the tune payload, turning a nil slice into an
// empty one so clearing a list is sent as [] rather than null.
func stringList(s []string) *[]string {
	if s == nil {
		s = []string{}
	}
	return &s
}

// SetupWithManager sets up the controller with the Manager.
func (r *MountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&sysv1beta1.Mount{}).
		Named("sys-mount").
		Complete(r)
}
