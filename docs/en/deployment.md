# Deployment Guide

This guide describes the deployment files, image build process, and supported installation methods for Sandbox Operator.

## 1. Deployment File Layout

The repository provides two deployment paths: Helm and raw Kubernetes manifests.

| Path | Description |
| --- | --- |
| `Dockerfile` | Builds the operator image. |
| `Makefile` | Common build, test, image, deploy, and undeploy targets. |
| `scripts/build-image.sh` | Script used by `make docker-build` to build the image with Docker. |
| `scripts/deploy.sh` | Raw-manifest deployment script used by `make deploy`; it also generates and patches webhook certificates. |
| `scripts/undeploy.sh` | Raw-manifest uninstall script used by `make undeploy`. |
| `charts/sandbox-operator/` | Helm Chart, recommended for regular installation and upgrades. |
| `charts/sandbox-operator/values.yaml` | Helm values for image, replicas, resources, OpenAPI config, webhook TLS, and leader election. |
| `config/deploy/` | Raw Kubernetes manifests used by `scripts/deploy.sh`. |
| `config/credentials/credentials.example.yaml` | Example Secrets for OpenAPI, storage, and image registry credentials. |
| `config/samples/` | Example CRs. |

Raw manifest files under `config/deploy/` are:

| File | Description |
| --- | --- |
| `00-namespace.yaml` | Operator namespace. |
| `01-crd.yaml` | CRDs for `SandboxTemplate`, `Sandbox`, and `SandboxClaim`. |
| `02-rbac.yaml` | ServiceAccount, ClusterRole, and ClusterRoleBinding. |
| `03-config.yaml` | Operator ConfigMap. |
| `04-manager.yaml` | Operator Deployment. |
| `05-webhook.yaml` | Webhook Service, MutatingWebhookConfiguration, and ValidatingWebhookConfiguration. |

## 2. Build the Image

Build and tag the operator image before installing it:

```bash
make docker-build IMG=<IMAGE>
```

Example:

```bash
make docker-build IMG=my-registry.example.com/sandbox/sandbox-operator:v0.1.3
```

The Makefile target calls:

```bash
./scripts/build-image.sh <IMAGE>
```

The script runs `docker build -t <IMAGE> .` using the repository `Dockerfile`.

If the cluster cannot pull from your local Docker daemon, push the image to a registry reachable by the cluster:

```bash
make docker-push IMG=my-registry.example.com/sandbox/sandbox-operator:v0.1.3
```

If the registry is private, create an image pull Secret in the operator namespace before deploying:

```bash
kubectl create namespace sandbox-operator-system

kubectl -n sandbox-operator-system create secret docker-registry sandbox-operator-image-pull \
  --docker-server='<REGISTRY_SERVER>' \
  --docker-username='<REGISTRY_USERNAME>' \
  --docker-password='<REGISTRY_PASSWORD>'
```

## 3. Deploy with Helm

Helm is the recommended installation method.

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set image.repository=my-registry.example.com/sandbox/sandbox-operator \
  --set image.tag=v0.1.3
```

For a private image registry, configure the pull Secret:

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set image.repository=my-registry.example.com/sandbox/sandbox-operator \
  --set image.tag=v0.1.3 \
  --set imagePullSecrets[0].name=sandbox-operator-image-pull
```

By default, the chart generates a self-signed webhook certificate using Helm templates.

If cert-manager is already installed, you can let cert-manager generate the certificate instead:

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set certManager.enabled=true \
  --set webhook.selfSigned.enabled=false
```

To override OpenAPI and poller settings, either edit `charts/sandbox-operator/values.yaml` or pass `--set` values:

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set config.pollInterval=30s \
  --set config.pollPageSize=100 \
  --set config.maxConcurrentNamespaces=5
```

## 4. Deploy with Raw Manifests

Raw-manifest deployment is useful for simple environments or when Helm is unavailable.

```bash
make deploy IMG=my-registry.example.com/sandbox/sandbox-operator:v0.1.3
```

This calls `scripts/deploy.sh`, which performs the following steps:

1. Applies the CRD, RBAC, ConfigMap, Deployment, and webhook configuration files under `config/deploy`.
2. Generates a self-signed CA and serving certificate with `openssl`.
3. Creates the `sandbox-operator-webhook-server-cert` Secret.
4. Patches the CA bundle into the webhook configurations.
5. Waits for the operator Deployment to become ready.

Uninstall:

```bash
make undeploy
```

For a detailed description of raw manifests, see [Raw Kubernetes deployment resources](deploy-manifests.md).

## 5. What Is Deployed

Sandbox Operator installs the following Kubernetes resources:

* `Namespace`: `sandbox-operator-system` by default.
* CRDs: `SandboxTemplate`, `Sandbox`, `SandboxClaim`.
* RBAC: ServiceAccount, ClusterRole, and ClusterRoleBinding used by the operator.
* ConfigMap: operator configuration.
* Deployment: operator manager.
* Service: webhook service.
* `MutatingWebhookConfiguration`: calls OpenAPI and injects platform IDs on CR creation.
* `ValidatingWebhookConfiguration`: validates CR updates and deletions.
* Webhook TLS Secret: serving certificate for the webhook service.

## 6. Operator Configuration

Configuration is read from the `sandbox-operator-config` ConfigMap. When deploying with Helm, these values can be overridden in `values.yaml`.

| Setting | Default | Description |
| --- | --- | --- |
| `OPENAPI_BASE_URL` | `http://aicp.cn-beijing-6.inner.api.ksyun.com` | Sandbox OpenAPI base URL. |
| `OPENAPI_AUTH_MODE` | `kop-sigv4` | OpenAPI signature mode. |
| `OPENAPI_SERVICE` | `aicp` | KOP service. |
| `OPENAPI_VERSION` | `2026-04-01` | OpenAPI version. |
| `DEFAULT_OPENAPI_CREDENTIAL_SECRET` | `sandbox-openapi-credentials` | Default OpenAPI credential Secret name in business namespaces. |
| `POLL_INTERVAL` | `30s` | OpenAPI polling interval. |
| `POLL_PAGE_SIZE` | `100` | Page size for OpenAPI list calls. |
| `MAX_CONCURRENT_NAMESPACES` | `5` | Number of namespaces synchronized concurrently. |
| `SYNC_NAMESPACES` | *(empty)* | Namespace allowlist; if empty, namespaces containing the default OpenAPI credential Secret are discovered automatically. |
| `LEADER_ELECT` | `true` | Enable leader election. |

## 7. Create Business Namespace Credentials

The operator synchronizes only business namespaces that contain a valid OpenAPI credential Secret.

```bash
kubectl create namespace sandbox-demo

kubectl -n sandbox-demo create secret generic sandbox-openapi-credentials \
  --from-literal=accessKeyId='<OPENAPI_ACCESS_KEY_ID>' \
  --from-literal=secretAccessKey='<OPENAPI_SECRET_ACCESS_KEY>' \
  --from-literal=accountId='<ACCOUNT_ID>' \
  --from-literal=region='cn-beijing-6'
```

Runtime credentials, such as KS3, KPFS, Klog, and image registry credentials, must also be placed in the same namespace as the business CRs.

Full Secret examples are available in [CR examples](cr-examples.md) and [`config/credentials/credentials.example.yaml`](../../config/credentials/credentials.example.yaml).

## 8. Verify the Deployment

```bash
kubectl get pods -n sandbox-operator-system
kubectl get mutatingwebhookconfiguration sandbox-operator-mutating-webhook
kubectl get validatingwebhookconfiguration sandbox-operator-validating-webhook
kubectl get crd sandboxtemplates.sandbox.kce.ksyun.com
kubectl get crd sandboxes.sandbox.kce.ksyun.com
kubectl get crd sandboxclaims.sandbox.kce.ksyun.com
```

View logs:

```bash
kubectl logs -n sandbox-operator-system deploy/sandbox-operator
```
