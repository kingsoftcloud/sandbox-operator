# Deploy Sandbox Operator

The default public image is `hub.kce.ksyun.com/ksyun-public/sandbox-operator:v20260707`; no registry credential is required.

## Quick Install

From the repository root, choose one of the following installation methods.

### Helm

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace
```

### Raw Manifests

For environments that do not use Helm:

```bash
make deploy
```

This creates the CRDs, RBAC, ConfigMap, Deployment, and webhook resources; it also creates the webhook TLS certificate and waits for the Deployment.

## Requirements

- The cluster can access the public registry and Sandbox OpenAPI.
- Helm deployment requires `helm` and `kubectl`.
- Raw-manifest deployment requires `make`, `bash`, `kubectl`, and `openssl`.

## Internal OpenAPI Endpoint

For Ksyun internal accounts, add the internal OpenAPI endpoint to the Helm command:

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set config.openapiBaseURL=http://aicp.cn-beijing-6.inner.api.ksyun.com
```

For raw manifests, change `OPENAPI_BASE_URL` in [03-config.yaml](../../config/deploy/03-config.yaml) to `http://aicp.cn-beijing-6.inner.api.ksyun.com`, then run `make deploy` again.

## Webhook Certificates

Helm generates a self-signed webhook certificate by default. To use an existing cert-manager installation:

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set certManager.enabled=true \
  --set webhook.selfSigned.enabled=false
```

## Verify

```bash
kubectl rollout status deployment/sandbox-operator -n sandbox-operator-system
kubectl get pods -n sandbox-operator-system
kubectl get crd sandboxtemplates.sandbox.kce.ksyun.com
kubectl logs -n sandbox-operator-system deploy/sandbox-operator
```

## Business Namespace Credentials

The operator manages only namespaces containing an OpenAPI credential Secret:

```bash
kubectl create namespace sandbox-demo

kubectl -n sandbox-demo create secret generic sandbox-openapi-credentials \
  --from-literal=accessKeyId='<OPENAPI_ACCESS_KEY_ID>' \
  --from-literal=secretAccessKey='<OPENAPI_SECRET_ACCESS_KEY>' \
  --from-literal=accountId='<ACCOUNT_ID>' \
  --from-literal=region='cn-beijing-6'
```

See [CR examples](cr-examples.md) for complete resource and credential examples.

## Custom Image

Build and push a custom image:

```bash
make docker-build IMG=my-registry.example.com/sandbox-operator:v0.1.0
make docker-push IMG=my-registry.example.com/sandbox-operator:v0.1.0
```

Deploy it with raw manifests:

```bash
make deploy IMG=my-registry.example.com/sandbox-operator:v0.1.0
```

Or override Helm values:

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set image.repository=my-registry.example.com/sandbox-operator \
  --set image.tag=v0.1.0
```

For a private registry, first create an image pull Secret in the operator namespace:

```bash
kubectl create namespace sandbox-operator-system
kubectl -n sandbox-operator-system create secret docker-registry sandbox-operator-image-pull \
  --docker-server='<REGISTRY_SERVER>' \
  --docker-username='<REGISTRY_USERNAME>' \
  --docker-password='<REGISTRY_PASSWORD>'
```

Pass the Secret to a raw-manifest deployment:

```bash
make deploy IMG=my-registry.example.com/sandbox-operator:v0.1.0 IMAGE_PULL_SECRET=sandbox-operator-image-pull
```

For Helm, add `--set imagePullSecrets[0].name=sandbox-operator-image-pull`.

## Upgrade and Uninstall

Repeat the install command to upgrade. To remove the operator:

```bash
# Helm
helm uninstall sandbox-operator -n sandbox-operator-system

# Raw manifests
make undeploy
```

See [raw manifest resources](deploy-manifests.md) for manifest and webhook certificate details.
