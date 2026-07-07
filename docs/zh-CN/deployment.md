# 部署说明

本文说明 Sandbox Operator 的部署文件目录、镜像制作方式，以及 Helm 和原生 Kubernetes manifest 两种部署方式。

## 1. 部署文件目录

项目提供两套部署方式：Helm 和原生 Kubernetes manifest。

| 路径 | 说明 |
| --- | --- |
| `Dockerfile` | operator 镜像构建文件。 |
| `Makefile` | 常用构建、测试、镜像、部署、卸载任务入口。 |
| `scripts/build-image.sh` | `make docker-build` 调用的镜像构建脚本。 |
| `scripts/deploy.sh` | `make deploy` 调用的原生 manifest 部署脚本，同时生成并 patch webhook 证书。 |
| `scripts/undeploy.sh` | `make undeploy` 调用的原生 manifest 卸载脚本。 |
| `charts/sandbox-operator/` | Helm Chart，推荐用于常规安装和升级。 |
| `charts/sandbox-operator/values.yaml` | Helm 配置，包括镜像、副本数、资源、OpenAPI 配置、webhook 证书、leader election 等。 |
| `config/deploy/` | 原生 Kubernetes manifest，由 `scripts/deploy.sh` 使用。 |
| `config/credentials/credentials.example.yaml` | OpenAPI、存储、镜像仓库等 Secret 示例。 |
| `config/samples/` | CR 示例。 |

`config/deploy/` 下的原生 manifest 文件如下：

| 文件 | 说明 |
| --- | --- |
| `00-namespace.yaml` | operator 命名空间。 |
| `01-crd.yaml` | `SandboxTemplate`、`Sandbox`、`SandboxClaim` CRD。 |
| `02-rbac.yaml` | ServiceAccount、ClusterRole、ClusterRoleBinding。 |
| `03-config.yaml` | operator ConfigMap。 |
| `04-manager.yaml` | operator Deployment。 |
| `05-webhook.yaml` | webhook Service、MutatingWebhookConfiguration、ValidatingWebhookConfiguration。 |

## 2. 制作镜像

部署前需要先构建 operator 镜像：

```bash
make docker-build IMG=<IMAGE>
```

示例：

```bash
make docker-build IMG=my-registry.example.com/sandbox/sandbox-operator:v0.1.3
```

该 Makefile 目标会调用：

```bash
./scripts/build-image.sh <IMAGE>
```

脚本会基于仓库根目录的 `Dockerfile` 执行：

```bash
docker build -t <IMAGE> .
```

如果集群节点无法直接拉取本地 Docker 镜像，需要将镜像推送到集群可访问的镜像仓库：

```bash
make docker-push IMG=my-registry.example.com/sandbox/sandbox-operator:v0.1.3
```

如果镜像仓库是私有仓库，需要先在 operator 命名空间创建 image pull Secret：

```bash
kubectl create namespace sandbox-operator-system

kubectl -n sandbox-operator-system create secret docker-registry sandbox-operator-image-pull \
  --docker-server='<REGISTRY_SERVER>' \
  --docker-username='<REGISTRY_USERNAME>' \
  --docker-password='<REGISTRY_PASSWORD>'
```

## 3. 使用 Helm 部署

推荐使用 Helm 部署：

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set image.repository=my-registry.example.com/sandbox/sandbox-operator \
  --set image.tag=v0.1.3
```

如果镜像在私有仓库，请配置 pull secret：

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set image.repository=my-registry.example.com/sandbox/sandbox-operator \
  --set image.tag=v0.1.3 \
  --set imagePullSecrets[0].name=sandbox-operator-image-pull
```

默认情况下，Chart 会使用 Helm 模板生成自签 webhook 证书。

如果集群已安装 cert-manager，可以改用 cert-manager 生成证书：

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set certManager.enabled=true \
  --set webhook.selfSigned.enabled=false
```

如需覆盖 OpenAPI 或 poller 配置，可以修改 `charts/sandbox-operator/values.yaml`，也可以通过 `--set` 传入：

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set config.pollInterval=30s \
  --set config.pollPageSize=100 \
  --set config.maxConcurrentNamespaces=5
```

## 4. 使用原生 Manifest 部署

原生 manifest 部署适合简单环境，或不能使用 Helm 的场景。

```bash
make deploy IMG=my-registry.example.com/sandbox/sandbox-operator:v0.1.3
```

该目标会调用 `scripts/deploy.sh`，脚本会：

1. 应用 `config/deploy` 下的 CRD、RBAC、ConfigMap、Deployment 和 webhook 配置。
2. 使用 `openssl` 生成 webhook 自签 CA 和服务端证书。
3. 创建 `sandbox-operator-webhook-server-cert` Secret。
4. 将 CA bundle 写入 webhook configuration。
5. 等待 operator Deployment 就绪。

卸载：

```bash
make undeploy
```

原生 manifest 文件说明见 [原生 Kubernetes 部署资源](deploy-manifests.md)。

## 5. 部署内容

operator 部署后包含以下 Kubernetes 资源：

- `Namespace`：默认 `sandbox-operator-system`。
- CRD：`SandboxTemplate`、`Sandbox`、`SandboxClaim`。
- RBAC：operator 使用的 ServiceAccount、ClusterRole、ClusterRoleBinding。
- ConfigMap：operator 配置。
- Deployment：operator manager。
- Service：webhook 服务。
- MutatingWebhookConfiguration：创建 CR 时调用 OpenAPI 并注入平台 ID。
- ValidatingWebhookConfiguration：校验 CR 更新、删除等操作。
- Webhook TLS Secret：webhook 服务证书。

## 6. Operator 配置

配置来自 `sandbox-operator-config` ConfigMap，Helm 部署时可通过 `values.yaml` 覆盖。

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `OPENAPI_BASE_URL` | `http://aicp.cn-beijing-6.inner.api.ksyun.com` | Sandbox OpenAPI 地址。 |
| `OPENAPI_AUTH_MODE` | `kop-sigv4` | OpenAPI 签名模式。 |
| `OPENAPI_SERVICE` | `aicp` | KOP Service。 |
| `OPENAPI_VERSION` | `2026-04-01` | OpenAPI 版本。 |
| `DEFAULT_OPENAPI_CREDENTIAL_SECRET` | `sandbox-openapi-credentials` | 业务命名空间中的默认 OpenAPI 凭据 Secret 名称。 |
| `POLL_INTERVAL` | `30s` | OpenAPI 轮询周期。 |
| `POLL_PAGE_SIZE` | `100` | OpenAPI 列表分页大小。 |
| `MAX_CONCURRENT_NAMESPACES` | `5` | 并发同步命名空间数量。 |
| `SYNC_NAMESPACES` | 空 | 命名空间白名单；为空时扫描存在默认 OpenAPI Secret 的命名空间。 |
| `LEADER_ELECT` | `true` | 是否启用 leader election。 |

## 7. 创建业务命名空间凭据

operator 只会同步有 OpenAPI 凭据的业务命名空间。

```bash
kubectl create namespace sandbox-demo

kubectl -n sandbox-demo create secret generic sandbox-openapi-credentials \
  --from-literal=accessKeyId='<OPENAPI_ACCESS_KEY_ID>' \
  --from-literal=secretAccessKey='<OPENAPI_SECRET_ACCESS_KEY>' \
  --from-literal=accountId='<ACCOUNT_ID>' \
  --from-literal=region='cn-beijing-6'
```

运行时凭据，例如 KS3、KPFS、Klog、镜像仓库凭据，也需要放在业务 CR 所在命名空间。

完整 Secret 示例见 [CR 示例](cr-examples.md) 和 [`config/credentials/credentials.example.yaml`](../../config/credentials/credentials.example.yaml)。

## 8. 检查部署状态

```bash
kubectl get pods -n sandbox-operator-system
kubectl get mutatingwebhookconfiguration sandbox-operator-mutating-webhook
kubectl get validatingwebhookconfiguration sandbox-operator-validating-webhook
kubectl get crd sandboxtemplates.sandbox.kce.ksyun.com
kubectl get crd sandboxes.sandbox.kce.ksyun.com
kubectl get crd sandboxclaims.sandbox.kce.ksyun.com
```

查看日志：

```bash
kubectl logs -n sandbox-operator-system deploy/sandbox-operator
```
