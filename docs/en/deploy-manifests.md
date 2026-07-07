# Raw Kubernetes Deployment Resources

This page describes the raw Kubernetes manifests used when deploying without Helm.

The manifest directory is `config/deploy/`:

| File | Description |
| --- | --- |
| `00-namespace.yaml` | Operator namespace. |
| `01-crd.yaml` | CRDs for `SandboxTemplate`, `Sandbox`, and `SandboxClaim`. |
| `02-rbac.yaml` | ServiceAccount, ClusterRole, and ClusterRoleBinding. |
| `03-config.yaml` | Operator configuration ConfigMap. |
| `04-manager.yaml` | Operator Deployment. |
| `05-webhook.yaml` | Webhook Service, MutatingWebhookConfiguration, and ValidatingWebhookConfiguration. |

## Apply Order

When applying resources manually, use this order:

```bash
kubectl apply -f config/deploy/00-namespace.yaml
kubectl apply -f config/deploy/01-crd.yaml
kubectl apply -f config/deploy/02-rbac.yaml
kubectl apply -f config/deploy/03-config.yaml

# The webhook TLS Secret and caBundle are generated and patched by scripts/deploy.sh.

kubectl apply -f config/deploy/04-manager.yaml
kubectl apply -f config/deploy/05-webhook.yaml
```

Using the Makefile targets is recommended:

```bash
make docker-build IMG=sandbox-operator:latest
make deploy IMG=sandbox-operator:latest
```

Uninstall:

```bash
make undeploy
```

## Webhook Certificates

Raw manifest deployment does not require cert-manager. `scripts/deploy.sh`:

1. Generates a self-signed CA and serving certificate with `openssl`.
2. Writes the serving certificate into the `sandbox-operator-webhook-server-cert` Secret.
3. Patches the CA bundle into MutatingWebhookConfiguration and ValidatingWebhookConfiguration.

If you manually replace the webhook certificate, patch the webhook `caBundle` again.

## Configuration

`03-config.yaml` contains the default OpenAPI settings:

* `OPENAPI_AUTH_MODE=kop-sigv4`
* `OPENAPI_SERVICE=aicp`
* `OPENAPI_VERSION=2026-04-01`
* `DEFAULT_OPENAPI_CREDENTIAL_SECRET=sandbox-openapi-credentials`

Business namespaces still need their own OpenAPI AK/SK Secret. See:

```text
config/credentials/credentials.example.yaml
```

## Helm Alternative

For regular installation and upgrades, Helm is recommended:

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set image.repository=sandbox-operator \
  --set image.tag=latest
```

The chart generates a self-signed webhook certificate by default. To use cert-manager:

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set certManager.enabled=true \
  --set webhook.selfSigned.enabled=false
```
