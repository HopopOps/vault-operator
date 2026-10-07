# vault-operator

A small Kubernetes operator managing HashiCorp Vault policies, kubernetes auth roles, mounts,
tokens and database secret engines from CRDs, so that Vault configuration can live next to the
app bundle that uses it in a GitOps pipeline.

If you need broad coverage of the Vault API, use
[redhat-cop/vault-config-operator](https://github.com/redhat-cop/vault-config-operator) instead.
This one deliberately implements a handful of resources only but I built it  to cover a few
things/issues that I felt where lacking in `redhat-cop/vault-config-operator`. 

## Resources

All resources are namespaced. The Vault object name is derived from the CR.

| Kind | Group | Manages |
| --- | --- | --- |
| `Policy` | `sys.toolkit.vault.hopopops.com/v1beta1` | ACL policy (`spec.policy`, HCL) |
| `Auth` | `sys.toolkit.vault.hopopops.com/v1beta1` | auth method mount (`spec.type`, `spec.description`, tuning) |
| `KubernetesRole` | `auth.toolkit.vault.hopopops.com/v1beta1` | kubernetes auth role (`spec.boundServiceAccount*`, `spec.tokenPolicies`, …) |
| `Token` | `auth.toolkit.vault.hopopops.com/v1beta1` | a Vault token, written to the Secret named by `spec.target.name` under key `token` |
| `Mount` | `sys.toolkit.vault.hopopops.com/v1beta1` | secret engine mount (`spec.type`, `spec.options`, tuning) |
| `PasswordPolicy` | `sys.toolkit.vault.hopopops.com/v1beta1` | password policy (`spec.policy`, HCL) |
| `Config` | `database.toolkit.vault.hopopops.com/v1beta1` | database secret engine connection (`database/config/<name>`) |
| `Role` | `database.toolkit.vault.hopopops.com/v1beta1` | database secret engine role (`database/roles/<name>`) |

`Config` watches the Secret named by `spec.rootCredentials.secretName` so a rotated password is
pushed to Vault again. The manager only informs on Secrets labelled
`toolkit.vault.hopopops.com/watch: "true"`, so that Secret must carry the label; reconciliation
fails loudly without it rather than silently missing the next rotation.

Examples live in `config/samples/`.

## Configuration

The manager authenticates to Vault with the kubernetes auth method, using its own service
account token. Flags:

| Flag | Default |
| --- | --- |
| `--vault-addr` | `http://vault.vault-system:8200` |
| `--vault-auth-endpoint` | `kubernetes` |
| `--vault-role` | `vault-operator` |
| `--vault-token-path` | `/var/run/secrets/kubernetes.io/serviceaccount/token` |

Standard controller-runtime flags (`--metrics-bind-address`, `--leader-elect`, …) are also
available; see `--help`.

## Deploy

```sh
make docker-build docker-push IMG=quay.io/hopopops/vault-operator:x.y.z
make install
make deploy IMG=quay.io/hopopops/vault-operator:x.y.z
```

Or consume the generated bundle (`make build-installer` regenerates `dist/install.yaml`; CI
also publishes it from `main` and tags as the OCI artifact
`oci://code.hopopops.com/kubernetes/vault-operator/manifests:latest`, `oci://code.hopopops.com/kubernetes/vault-operator/manifests:x.y.z`):

```yaml
---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - https://code.hopopops.com/kubernetes/vault-operator/raw/tag/x.y.z/dist/install.yaml

patches:
  - patch: |
      - op: add
        path: /spec/template/spec/containers/0/args/-
        value: --vault-addr=https://vault.hopopops.com
    target:
      kind: Deployment
      name: vault-operator-controller-manager
```

Removal: `make undeploy` then `make uninstall`.

## Development

Go 1.26+, `make help` lists the targets.

## License

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
