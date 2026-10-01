# vault-operator

A small Kubernetes operator managing HashiCorp Vault policies, kubernetes auth roles, auth
method mounts and tokens from CRDs, so that Vault configuration can live next to the app bundle
that uses it in a GitOps pipeline.

If you need broad coverage of the Vault API, use
[redhat-cop/vault-config-operator](https://github.com/redhat-cop/vault-config-operator) instead.
This one deliberately implements four resources.

## Resources

All resources are namespaced. The Vault object name is derived from the CR.

| Kind | Group | Manages |
| --- | --- | --- |
| `Policy` | `sys.toolkit.vault.hopopops.com/v1beta1` | ACL policy (`spec.policy`, HCL) |
| `Auth` | `sys.toolkit.vault.hopopops.com/v1beta1` | auth method mount (`spec.type`, `spec.description`, tuning) |
| `KubernetesRole` | `auth.toolkit.vault.hopopops.com/v1beta1` | kubernetes auth role (`spec.boundServiceAccount*`, `spec.tokenPolicies`, …) |
| `Token` | `auth.toolkit.vault.hopopops.com/v1beta1` | a Vault token, written to the Secret named by `spec.target.name` under key `token` |

`Token` renews its lease and rewrites the Secret; `spec.target.deletionPolicy` (`Retain`,
default, or `Delete`) decides what happens to the Secret when the CR goes away.

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
make docker-build docker-push IMG=quay.io/hopopops/vault-operator:tag
make install
make deploy IMG=quay.io/hopopops/vault-operator:tag
```

Or consume the generated bundle (`make build-installer` regenerates `dist/install.yaml`):

```yaml
---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - https://raw.githubusercontent.com/hopopops/vault-operator/main/dist/install.yaml

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

Go 1.24+, `make help` lists the targets.

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
</content>
</invoke>
