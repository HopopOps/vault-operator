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

// Package controller holds what the per-group controller packages share.
package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretWatchLabel opts a Secret into the manager's Secret informer. Only
// labelled Secrets are cached and watched, so a Secret a database Config reads
// its root credentials from has to carry it for a rotation to be noticed.
const SecretWatchLabel = "toolkit.vault.hopopops.com/watch"

// SecretWatchSelector matches the Secrets the informer holds.
func SecretWatchSelector() labels.Selector {
	return labels.SelectorFromSet(labels.Set{SecretWatchLabel: "true"})
}

// WatchedSecret reads a Secret that a controller pushes to Vault. Reads are
// uncached, so an unlabelled Secret is readable but never watched, and a
// rotation of it would go unnoticed. Refuse it rather than look healthy.
func WatchedSecret(ctx context.Context, c client.Reader, key types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, key, secret); err != nil {
		return nil, err
	}

	if secret.Labels[SecretWatchLabel] != "true" {
		return nil, fmt.Errorf("secret %q is missing the label %s=true, a rotation of it would go unnoticed", secret.Name, SecretWatchLabel)
	}

	return secret, nil
}
