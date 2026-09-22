# Deploy Sandbox Operator

The default public image is `hub.kce.ksyun.com/ksyun-public/sandbox-operator:v20260922`; no registry credential is required.

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

## Choose the Operator Namespace

The default namespace is `sandbox-operator-system`. Helm uses the release namespace, so no additional chart value is required:

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-custom \
  --create-namespace
```

Pass the same `NAMESPACE` when deploying and uninstalling raw manifests:

```bash
make deploy NAMESPACE=sandbox-operator-custom
make undeploy NAMESPACE=sandbox-operator-custom
```

CRDs, ClusterRoles, ClusterRoleBindings, and WebhookConfigurations are cluster-scoped and use fixed names. Deploy only one Sandbox Operator instance per cluster.

## Requirements

- The cluster can access the public registry and the configured Sandbox OpenAPI endpoint.
- Helm deployment requires `helm` and `kubectl`.
- Raw-manifest deployment requires `make`, `bash`, `kubectl`, and `openssl`.
- Before deployment, make sure `kubectl get namespaces` succeeds. Installation requires cluster-level permission to create CRDs, ClusterRoles, ClusterRoleBindings, and admission webhooks.

## Deploying from Outside the Cluster

Deployment can run from any machine with `kubectl`; logging into a target cluster node is not required. Place the target cluster kubeconfig in this repository, then set `KUBECONFIG` from the repository root:

```bash
export KUBECONFIG="$PWD/config/kubeconfig.yaml"
kubectl get namespaces
make deploy
```

Replace `config/kubeconfig.yaml` with the actual file name. `kubectl`, `helm`, and `make deploy` in the current terminal automatically use `KUBECONFIG`; the deployment script started by `make deploy` inherits it as well.

To apply it to one deployment only:

```bash
KUBECONFIG="$PWD/config/kubeconfig.yaml" make deploy
```

## Internal OpenAPI Endpoint

To access Sandbox OpenAPI through the internal endpoint, add it to the Helm command:

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set config.openapiBaseURL=http://aicp.cn-beijing-6.inner.api.ksyun.com
```

For raw manifests, override the deployment variable without changing a file:

```bash
make deploy OPENAPI_BASE_URL=http://aicp.cn-beijing-6.inner.api.ksyun.com
```

Repeating this command rolls the operator so the new ConfigMap endpoint takes effect immediately.

## Configure the Polling Interval

`POLL_INTERVAL` uses the Go duration format and defaults to `500ms`. For larger resource sets, increase it as needed, for example to `5s`:

```bash
make deploy POLL_INTERVAL=5s
```

The command writes the value to the operator ConfigMap and rolls the Deployment. Each poll scans namespaces with OpenAPI credentials and reads remote resources. The effective frequency is also limited by the duration of each synchronization pass.

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

The operator then synchronizes templates and sandbox instances in the account into `SandboxTemplate` and `Sandbox` CRs in this namespace. You can also create `SandboxTemplate`, `Sandbox`, or `SandboxClaim` yourself: the first two manage their corresponding platform resources, while `SandboxClaim` is a one-shot batch declaration for sandbox instances. See [CR examples](cr-examples.md) for complete resource and credential examples.

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
OPERATOR_NAMESPACE=sandbox-operator-system
kubectl create namespace "${OPERATOR_NAMESPACE}"
kubectl -n "${OPERATOR_NAMESPACE}" create secret docker-registry sandbox-operator-image-pull \
  --docker-server='<REGISTRY_SERVER>' \
  --docker-username='<REGISTRY_USERNAME>' \
  --docker-password='<REGISTRY_PASSWORD>'
```

Pass the Secret to a raw-manifest deployment:

```bash
make deploy NAMESPACE="${OPERATOR_NAMESPACE}" IMG=my-registry.example.com/sandbox-operator:v0.1.0 IMAGE_PULL_SECRET=sandbox-operator-image-pull
```

For Helm, add `--set imagePullSecrets[0].name=sandbox-operator-image-pull`.

## Upgrade and Uninstall

Do not run `make undeploy` or `helm uninstall` before an upgrade. Uninstalling causes an avoidable service interruption, and raw-manifest uninstall also removes configuration and webhook certificates from the Operator namespace.

For a raw-manifest installation, pull the latest source and repeat the deployment command. `make deploy` updates the CRDs, ConfigMap, RBAC, Deployment, and webhooks, switches to the default `v20260922` image, and rolls the Operator:

```bash
git pull
make deploy
```

If the original installation used a custom namespace, internal OpenAPI endpoint, or other options, pass the same values during the upgrade, for example:

```bash
make deploy \
  NAMESPACE=sandbox-operator-custom \
  OPENAPI_BASE_URL=http://aicp.cn-beijing-6.inner.api.ksyun.com
```

For a Helm installation, explicitly update the CRDs before reusing the install command. Helm does not automatically update CRDs from a chart's `crds/` directory during `helm upgrade`, so the first command is required for the new NFS fields:

```bash
git pull
kubectl apply -f config/crd/bases.yaml
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace
```

If the existing release uses a custom namespace or `--set` values, pass the same values during the upgrade.

Run an uninstall command only when you intend to remove the Operator:

```bash
# Helm
helm uninstall sandbox-operator -n sandbox-operator-system

# Raw manifests
make undeploy
```

`make undeploy` for raw manifests preserves CRDs and business CRs. To remove CRDs, delete all `SandboxTemplate`, `Sandbox`, and `SandboxClaim` resources while the Operator is running and wait for completion, then run `make purge-crds`.

See [raw manifest resources](deploy-manifests.md) for manifest and webhook certificate details.
