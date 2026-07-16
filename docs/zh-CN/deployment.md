# 部署 Sandbox Operator



## 快速部署

在仓库根目录选择以下任一种方式执行。

### Helm 部署

推荐用于安装和后续升级：

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace
```

### 原生 Manifest 部署

适用于不使用 Helm 的环境：

```bash
make deploy
```

该命令会创建 CRD、RBAC、ConfigMap、Deployment 和 webhook 资源，自动生成 webhook TLS 证书并等待 Deployment 就绪。

## 部署条件

- 集群可以通过公网访问公共镜像仓库和 Sandbox OpenAPI。
- Helm 部署需要 `helm` 与 `kubectl`。
- 原生 Manifest 部署需要 `make`、`bash`、`kubectl` 和 `openssl`。

## 验证部署

```bash
kubectl rollout status deployment/sandbox-operator -n sandbox-operator-system
kubectl get pods -n sandbox-operator-system
kubectl get crd sandboxtemplates.sandbox.kce.ksyun.com
kubectl logs -n sandbox-operator-system deploy/sandbox-operator
```

## 创建业务命名空间凭据

operator 只管理包含 OpenAPI 凭据 Secret 的业务命名空间。部署完成后，在需要管理的命名空间创建凭据：

```bash
kubectl create namespace sandbox-demo

kubectl -n sandbox-demo create secret generic sandbox-openapi-credentials \
  --from-literal=accessKeyId='<OPENAPI_ACCESS_KEY_ID>' \
  --from-literal=secretAccessKey='<OPENAPI_SECRET_ACCESS_KEY>' \
  --from-literal=accountId='<ACCOUNT_ID>' \
  --from-literal=region='cn-beijing-6'
```

随后operator会自动将账号下的沙箱、模板资源同步到该命名空间的 `SandboxTemplate`、`Sandbox` 和 `SandboxClaim` CR 中。也可主动创建这些CR，会同步到账号下资源。
完整 CR 和凭据示例见 [CR 示例](cr-examples.md)。

## 使用自建镜像

默认使用的是公共镜像 `hub.kce.ksyun.com/ksyun-public/sandbox-operator:v20260707`，无需镜像仓库凭据。
需要自行构建镜像时，先构建并推送到集群可访问的仓库：

```bash
make docker-build IMG=my-registry.example.com/sandbox-operator:v0.1.0
make docker-push IMG=my-registry.example.com/sandbox-operator:v0.1.0
```

原生 Manifest 部署时覆盖为指定镜像：

```bash
make deploy IMG=my-registry.example.com/sandbox-operator:v0.1.0
```

Helm 部署时覆盖镜像仓库和标签：

```bash
helm upgrade --install sandbox-operator charts/sandbox-operator \
  -n sandbox-operator-system \
  --create-namespace \
  --set image.repository=my-registry.example.com/sandbox-operator \
  --set image.tag=v0.1.0
```

私有镜像仓库需要先在 operator 命名空间创建 image pull Secret：

```bash
kubectl create namespace sandbox-operator-system
kubectl -n sandbox-operator-system create secret docker-registry sandbox-operator-image-pull \
  --docker-server='<REGISTRY_SERVER>' \
  --docker-username='<REGISTRY_USERNAME>' \
  --docker-password='<REGISTRY_PASSWORD>'
```

原生 Manifest 部署时传入该 Secret：

```bash
make deploy IMG=my-registry.example.com/sandbox-operator:v0.1.0 IMAGE_PULL_SECRET=sandbox-operator-image-pull
```

Helm 部署时追加 `--set imagePullSecrets[0].name=sandbox-operator-image-pull`。

## 升级与卸载

Helm 升级复用安装命令；原生 Manifest 升级重新执行 `make deploy`。

```bash
# Helm
helm uninstall sandbox-operator -n sandbox-operator-system

# 原生 Manifest
make undeploy
```

原生 Manifest 文件和 webhook 证书处理细节见 [原生 Kubernetes 部署资源](deploy-manifests.md)。
